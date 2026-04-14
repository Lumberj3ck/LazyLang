package transriber

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// WAV header constants
const (
	wavHeaderSize = 44
	SampleRate    = 16000
	Channels      = 1
)

type openAITranscriptionResponse struct {
	Text string `json:"text"`
}

type OpenAITranscriber struct {
	token    string
	language string
	model    string
	baseURL  string
}

func NewOpenAITranscriber(baseURL string, token string, model string, language string) *OpenAITranscriber {
	return &OpenAITranscriber{
		token:    token,
		language: language,
		model:    model,
		baseURL:  strings.TrimRight(baseURL, "/"),
	}
}

// Transcribe sends audio to an OpenAI-compatible API for transcription
func (m OpenAITranscriber) Transcribe(audioData []int16) (string, error) {
	audioWav := samplesToWAV(audioData, SampleRate, Channels)
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add audio file
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = part.Write(audioWav)
	if err != nil {
		return "", fmt.Errorf("failed to write audio data: %w", err)
	}

	// Add model field
	err = writer.WriteField("model", m.model)
	if err != nil {
		return "", fmt.Errorf("failed to write model field: %w", err)
	}

	// Add Language field
	err = writer.WriteField("language", m.language)
	if err != nil {
		return "", fmt.Errorf("failed to write language field: %w", err)
	}

	// Add response format
	err = writer.WriteField("response_format", "json")
	if err != nil {
		return "", fmt.Errorf("failed to write response_format field: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/audio/transcriptions", m.baseURL)
	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if m.token != "" {
		req.Header.Set("Authorization", "Bearer "+m.token)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var transcriptionResp openAITranscriptionResponse
	err = json.Unmarshal(body, &transcriptionResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return transcriptionResp.Text, nil
}

// samplesToWAV converts raw audio samples to WAV format
func samplesToWAV(samples []int16, sampleRate, channels int) []byte {
	var buf bytes.Buffer

	dataSize := len(samples) * 2 // 2 bytes per sample (16-bit)
	fileSize := wavHeaderSize + dataSize - 8

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, int32(fileSize))
	buf.WriteString("WAVE")

	// fmt subchunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, int32(16))         // Subchunk1Size (16 for PCM)
	binary.Write(&buf, binary.LittleEndian, int16(1))          // AudioFormat (1 for PCM)
	binary.Write(&buf, binary.LittleEndian, int16(channels))   // NumChannels
	binary.Write(&buf, binary.LittleEndian, int32(sampleRate)) // SampleRate
	byteRate := sampleRate * channels * 2                      // ByteRate
	binary.Write(&buf, binary.LittleEndian, int32(byteRate))
	blockAlign := channels * 2 // BlockAlign
	binary.Write(&buf, binary.LittleEndian, int16(blockAlign))
	binary.Write(&buf, binary.LittleEndian, int16(16)) // BitsPerSample

	// data subchunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, int32(dataSize))

	// Write audio data
	for _, sample := range samples {
		binary.Write(&buf, binary.LittleEndian, sample)
	}

	return buf.Bytes()
}
