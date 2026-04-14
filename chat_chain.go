package main

import (
	"context"
	"errors"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/prompts"
)

type ChatLLMChain struct {
	*chains.LLMChain
}

func NewChatLLMChain(llm llms.Model, prompt prompts.FormatPrompter, opts ...chains.ChainCallOption) *ChatLLMChain {
	return &ChatLLMChain{LLMChain: chains.NewLLMChain(llm, prompt, opts...)}
}

func (c *ChatLLMChain) Call(ctx context.Context, values map[string]any, options ...chains.ChainCallOption) (map[string]any, error) {
	promptValue, err := c.Prompt.FormatPrompt(values)
	if err != nil {
		return nil, err
	}

	chatPrompt, ok := promptValue.(interface{ Messages() []llms.ChatMessage })
	if !ok {
		return c.LLMChain.Call(ctx, values, options...)
	}

	messageContents := chatMessagesToContent(chatPrompt.Messages())
	result, err := c.LLM.GenerateContent(ctx, messageContents, chains.GetLLMCallOptions(options...)...)
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.Choices) == 0 || result.Choices[0] == nil {
		return nil, errors.New("llm returned no content")
	}

	finalOutput, err := c.OutputParser.ParseWithPrompt(result.Choices[0].Content, promptValue)
	if err != nil {
		return nil, err
	}

	return map[string]any{c.OutputKey: finalOutput}, nil
}

func chatMessagesToContent(messages []llms.ChatMessage) []llms.MessageContent {
	mcList := make([]llms.MessageContent, len(messages))
	for i, msg := range messages {
		mcList[i] = chatMessageToContent(msg)
	}
	return mcList
}

func chatMessageToContent(msg llms.ChatMessage) llms.MessageContent {
	role := msg.GetType()
	switch m := msg.(type) {
	case llms.ToolChatMessage:
		return llms.MessageContent{
			Role: role,
			Parts: []llms.ContentPart{llms.ToolCallResponse{
				ToolCallID: m.ID,
				Content:    m.Content,
			}},
		}
	case llms.FunctionChatMessage:
		return llms.MessageContent{
			Role: role,
			Parts: []llms.ContentPart{llms.ToolCallResponse{
				Name:    m.Name,
				Content: m.Content,
			}},
		}
	case llms.AIChatMessage:
		if len(m.ToolCalls) > 0 {
			parts := make([]llms.ContentPart, 0, len(m.ToolCalls))
			for _, call := range m.ToolCalls {
				parts = append(parts, llms.ToolCall{
					ID:           call.ID,
					Type:         call.Type,
					FunctionCall: call.FunctionCall,
				})
			}
			return llms.MessageContent{Role: role, Parts: parts}
		}
	}

	return llms.MessageContent{
		Role:  role,
		Parts: []llms.ContentPart{llms.TextContent{Text: msg.GetContent()}},
	}
}
