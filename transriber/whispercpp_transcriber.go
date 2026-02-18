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
	"errors"
	"fmt"
	"lazylang/utils"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func init() {
	C.whisper_disable_logging()
}

var whisperModelsDir = filepath.Join(utils.GetProjectPath(), "whisper-models")
type WhispercppTranscriber struct {
	model string // Path to the model
	language string
}

func NewWhispercppTranscriber(model string, language string) *WhispercppTranscriber {
	return &WhispercppTranscriber{
		model: model,
		language: language,
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

const(
	srcExt  = ".bin"                                                       // Filename extension
	srcUrl  = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/" // The location of the models
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

var(
bufSize = 1024 * 64                                                    // Size of the buffer used for downloading the model
)
func (m WhispercppTranscriber) DownloadModel(model string, report chan int64) error {
	// Start download plus progress reporting
	return errors.New("Not implemented")
}

func (m WhispercppTranscriber) Transcribe(audioData []int16) (string, error) {
	samples := int16ToFloat32(audioData)
	modelName := strings.TrimRight(m.model, ".bin")
	modelName = modelName + ".bin"
	modelPath := filepath.Join(whisperModelsDir, modelName)

	// Load the model
	model, err := whisper.New(modelPath)

	if errors.Is(err, os.ErrNotExist) {
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

