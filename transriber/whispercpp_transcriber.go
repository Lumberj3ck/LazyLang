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
	"lazylang/utils"
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

func (m WhispercppTranscriber) Transcribe(audioData []int16) (string, error) {
	samples := int16ToFloat32(audioData)
	modelName := strings.TrimRight(m.model, ".bin")
	modelName = modelName + ".bin"
	modelPath := filepath.Join(whisperModelsDir, modelName)

	// Load the model
	model, err := whisper.New(modelPath)
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
