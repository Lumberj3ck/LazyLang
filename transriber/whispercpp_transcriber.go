package transriber

import (
	"strings"
	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type WhispercppTranscriber struct {
	modelpath string // Path to the model
}

func NewWhispercppTranscriber(modelpath string) *WhispercppTranscriber {
	return &WhispercppTranscriber{
		modelpath: modelpath,
	}
}

func int16ToFloat32(input []int16) []float32 {
    output := make([]float32, len(input))
    for i, v := range input {
        output[i] = float32(v)
    }
    return output
}

func (m WhispercppTranscriber) Transcribe(audioData []int16) (string, error) {
	samples := int16ToFloat32(audioData)

	// Load the model
	model, err := whisper.New(m.modelpath)
	if err != nil {
		panic(err)
	}
	defer model.Close()

	// Process samples
	context, err := model.NewContext()
	if err != nil {
		panic(err)
	}
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
