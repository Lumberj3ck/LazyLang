package transriber

/*
#include <whisper.h>

// no-op log callback to silence whisper.cpp output
static void whisper_log_noop(enum ggml_log_level level, const char * text, void * user_data) {
    (void)level;
    (void)text;
    (void)user_data;
}

static void whisper_disable_logging() {
    whisper_log_set(whisper_log_noop, NULL);
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"lazylang/utils"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func init() {
	C.whisper_disable_logging()
}

var whisperModelsDir = filepath.Join(utils.GetProjectPath(), "whisper-models")

type WhispercppTranscriber struct {
	model    string // Path to the model
	language string
	cancel   context.CancelFunc
	mu       sync.Mutex
	DownloadReport   chan int64
}

func NewWhispercppTranscriber(model string, language string) *WhispercppTranscriber {
	return &WhispercppTranscriber{
		model:    model,
		language: language,
		DownloadReport: make(chan int64, 10),
	}
}

func int16ToFloat32(input []int16) []float32 {
	output := make([]float32, len(input))
	for i, v := range input {
		// normalise to [-1, 1] for whispercpp
		output[i] = float32(v) / 32768.0
	}
	return output
}

const (
	srcExt = ".bin"                                                       // Filename extension
	srcUrl = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/" // The location of the models
)

// URLForModel returns the URL for the given model on huggingface.co
func URLForModel(model string) (string, error) {
	// Ensure "ggml-" prefix is added only once
	if !strings.HasPrefix(model, "ggml-") {
		model = "ggml-" + model
	}

	// Ensure ".bin" extension is added only once
	if filepath.Ext(model) != srcExt {
		model += srcExt
	}

	// Parse the base URL
	url, err := url.Parse(srcUrl)
	if err != nil {
		return "", err
	}

	// Ensure no trailing slash in the base URL
	url.Path = fmt.Sprintf("%s/%s", strings.TrimSuffix(url.Path, "/"), model)
	return url.String(), nil
}

var ErrNoModel = errors.New("No model found")

var (
	bufSize = 1024 * 64 // Size of the buffer used for downloading the model
)

func (m *WhispercppTranscriber) DownloadModel(model string) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.mu.Unlock()

	// Create HTTP client
	client := http.Client{
		Timeout: time.Minute * 20,
	}

	// Initiate the download
	url, err := URLForModel(model)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", model, resp.Status)
	}

	// If output file exists and is the same size as the model, skip
	path := filepath.Join(whisperModelsDir, filepath.Base(url))
	if info, err := os.Stat(path); err == nil && info.Size() == resp.ContentLength {
		log.Printf("Skipping %s as it already exists", url)
		return nil
	}

	// Create file
	w, err := os.Create(path)
	if err != nil {
		return err
	}
	defer w.Close()

	// Progressively download the model
	data := make([]byte, bufSize)
	count := int64(0)
	ticker := time.NewTicker(time.Millisecond * 500)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if len(m.DownloadReport) == cap(m.DownloadReport) {
				<-m.DownloadReport
			}
			m.DownloadReport <- count * 100 / resp.ContentLength
		default:
			// Read body
			n, readErr := resp.Body.Read(data)
			if n > 0 {
				k, err := w.Write(data[:n])
				if err != nil {
					return err
				}
				count += int64(k)
			}

			log.Println("New data")
			if readErr != nil {
				return err
			} 
		}
	}
}

func (m *WhispercppTranscriber) Transcribe(audioData []int16) (string, error) {
	samples := int16ToFloat32(audioData)
	modelName := strings.TrimRight(m.model, ".bin")
	modelName = modelName + ".bin"
	modelPath := filepath.Join(whisperModelsDir, modelName)

	url, err := URLForModel(m.model)
	if err != nil {
		return "", err
	}
	resp, err := http.Head(url)
	if err != nil {
		return "", err
	}

	path := filepath.Join(whisperModelsDir, filepath.Base(url))
	if info, err := os.Stat(path); err != nil || info.Size() != resp.ContentLength {
		log.Printf("Skipping %s as it already exists", url)
		return "", ErrNoModel
	}

	// Load the model
	model, err := whisper.New(modelPath)
	if errors.Is(err, os.ErrNotExist){
		return "", ErrNoModel
	}

	if err != nil {
		return "", err
	}
	defer model.Close()

	// Process samples
	context, err := model.NewContext()
	if err != nil {
		return "", err
	}
	context.SetLanguage(m.language)

	if err := context.Process(samples, nil, nil, nil); err != nil {
		return "", err
	}

	log.Println("Processing complete")
	var res strings.Builder
	for {
		segment, err := context.NextSegment()
		if err != nil {
			break
		}
		res.WriteString(segment.Text)
	}
	return res.String(), nil
}

