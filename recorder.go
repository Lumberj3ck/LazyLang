package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

type Recorder struct {
	recording bool
	Content   []int16
	done      chan struct{}
	finished  chan struct{}
	Stopped   time.Time
	mu        sync.RWMutex
}

func NewRecorder() *Recorder {
	return &Recorder{
		recording: false,
		done:      make(chan struct{}),
		finished:  make(chan struct{}),
	}
}

func (r *Recorder) IsRecording() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.recording
}

// recordAudio captures audio from the microphone until Ctrl+B is pressed
func (r *Recorder) Start() ([]int16, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audio context: %w", err)
	}
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = uint32(channels)
	deviceConfig.SampleRate = uint32(sampleRate)

	var capturedBytes []byte

	onRecvFrames := func(pOutputSample, pInputSamples []byte, framecount uint32) {
		r.mu.Lock()
		capturedBytes = append(capturedBytes, pInputSamples...)
		r.mu.Unlock()
	}

	callbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize capture device: %w", err)
	}

	err = device.Start()
	if err != nil {
		device.Uninit()
		return nil, fmt.Errorf("failed to start capture device: %w", err)
	}

	r.mu.Lock()
	r.recording = true
	r.mu.Unlock()

	log.Println("Recording")

	// Wait until stopped
	<-r.done

	device.Stop()
	device.Uninit()

	// Convert raw PCM bytes to []int16
	r.mu.Lock()
	raw := capturedBytes
	r.mu.Unlock()

	allSamples := make([]int16, len(raw)/2)
	for i := range allSamples {
		allSamples[i] = int16(raw[2*i]) | int16(raw[2*i+1])<<8
	}

	// Convert to WAV format
	r.Content = allSamples
	r.mu.Lock()
	r.recording = false
	r.mu.Unlock()

	r.Stopped = time.Now()
	close(r.finished)
	return allSamples, nil
}

func (r *Recorder) Stop() {
	if !r.recording {
		return
	}
	r.done <- struct{}{}
	<-r.finished
	// Reset channels for next recording
	r.done = make(chan struct{})
	r.finished = make(chan struct{})
}

