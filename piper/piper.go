package piper

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var home, _ = os.UserHomeDir()
var voicesDir = filepath.Join(home, ".piper-voices")

func saveToFile(data []byte, filename string) error {
	err := os.MkdirAll(voicesDir, 0755)
	fmt.Println(err)

	_, err = os.OpenFile(filepath.Join(voicesDir, "test.wav"), os.O_RDWR|os.O_CREATE, 0666)
	fmt.Println(err)

	return nil
}

func ListLanguages() error {
	return nil
}

func ListVoices(language string) error {
	return nil
}


func DownloadVoice(language string, voice string) error {
	return nil
}

type PiperVoice struct {
	Language string
	Model string
}


type PiperOption func(*PiperVoice)

func WithLanguage(language string) PiperOption {
	return func(pv *PiperVoice) {
		pv.Language = language
	}
}

func WithModel(model string) PiperOption {
	return func(pv *PiperVoice) {
		pv.Model = model
	}
}

func NewPiperVoice(options ...PiperOption) PiperVoice {
	pv := PiperVoice{
		Language: "de",
		Model: "karlsson-low",
	}

	for _, option := range options {
		option(&pv)
	}
	return pv
}

// speakWithPiper generates speech using Piper TTS and plays it
func (p PiperVoice) Speak(text string) error {
	modelFile := filepath.Join(voicesDir, p.Model)
	_, err := os.Stat(modelFile)
	if err != nil {
		fmt.Println("Model not found, downloading...")
		err = DownloadVoice("de", "karlsson-low")
		if err != nil {
			return err
		}
	}

	// Create piper command
	// Piper reads from stdin and outputs WAV to stdout
	piperCmd := exec.Command("piper-tts", "--model", modelFile, "--output_file", "-")
	piperCmd.Stdin = bytes.NewBufferString(text)

	// Pipe piper output to aplay (or use paplay for PulseAudio)
	aplayCmd := exec.Command("aplay", "-r", "22050", "-f", "S16_LE", "-t", "wav", "-")

	// Connect piper stdout to aplay stdin
	pipe, err := piperCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	aplayCmd.Stdin = pipe

	// Capture stderr for debugging
	var piperStderr, aplayStderr bytes.Buffer
	piperCmd.Stderr = &piperStderr
	aplayCmd.Stderr = &aplayStderr

	// Start both commands
	err = piperCmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start piper: %w", err)
	}

	err = aplayCmd.Start()
	if err != nil {
		piperCmd.Process.Kill()
		return fmt.Errorf("failed to start aplay: %w", err)
	}

	// Wait for both commands to finish
	piperErr := piperCmd.Wait()
	aplayErr := aplayCmd.Wait()

	if piperErr != nil {
		return fmt.Errorf("piper error: %w, stderr: %s", piperErr, piperStderr.String())
	}
	if aplayErr != nil {
		return fmt.Errorf("aplay error: %w, stderr: %s", aplayErr, aplayStderr.String())
	}

	return nil
}

