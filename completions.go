package main

import (
	"fmt"
	"os"

	"github.com/tmc/langchaingo/llms/openai"
)

type ChatCompletion struct {
	url   string
	model string
	token string
}
type Option func(*ChatCompletion)

func WithBaseURL(url string) Option {
	return func(cc *ChatCompletion) {
		cc.url = url
	}
}

func WithToken(token string) Option {
	return func(cc *ChatCompletion) {
		cc.token = token
	}
}

func WithModel(model string) Option {
	return func(cc *ChatCompletion) {
		cc.model = model
	}
}

func NewLLM(options ...Option) (*openai.LLM, error) {
	cc := ChatCompletion{
		token: os.Getenv(completionTokenEnvVar),
	}
	for _, option := range options {
		option(&cc)
	}

	if cc.url == "" {
		return nil, fmt.Errorf("missing completion provider base URL")
	}
	if cc.model == "" {
		return nil, fmt.Errorf("missing completion provider model")
	}
	if completionTokenRequired(cc.url, cc.token) {
		return nil, fmt.Errorf("missing completion provider token")
	}

	opts := []openai.Option{
		openai.WithBaseURL(cc.url),
		openai.WithModel(cc.model),
	}
	if cc.token != "" {
		opts = append(opts, openai.WithToken(cc.token))
	}

	llm, err := openai.New(opts...)

	if err != nil {
		return llm, err
	}
	return llm, nil
}
