package transriber

type Transcriber interface {
	Transcribe(audio []int16) (string, error)
}
