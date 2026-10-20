package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type anthropicProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func newAnthropicProvider(cfg ProviderConfig) (*anthropicProvider, error) {
	baseURL := trimBase(cfg.BaseURL)
	if baseURL == "" {
		baseURL = trimBase(envOr("ANTHROPIC_BASE_URL", "https://api.anthropic.com"))
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(envOr("ANTHROPIC_API_KEY", ""))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}
	model := cfg.Model
	if model == "" {
		model = "claude-3-5-sonnet-latest"
	}
	return &anthropicProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: timeoutOr(cfg.RequestTimeout, 2*time.Minute)},
	}, nil
}

func (p *anthropicProvider) Name() string { return "anthropic" }

type anthropicRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
}

type anthropicResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (p *anthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	var system string
	messages := make([]Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if system != "" {
				system += "\n\n"
			}
			system += msg.Content
			continue
		}
		messages = append(messages, msg)
	}
	if req.JSON {
		system += "\n\nReturn only a valid JSON object, with no markdown fence or commentary."
	}
	body := anthropicRequest{
		Model:       model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		System:      system,
		Messages:    messages,
	}
	var out anthropicResponse
	headers := map[string]string{
		"x-api-key":         p.apiKey,
		"anthropic-version": "2023-06-01",
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/v1/messages", headers, body, &out); err != nil {
		return CompletionResponse{}, err
	}
	var text strings.Builder
	for _, block := range out.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	result := strings.TrimSpace(text.String())
	if result == "" {
		return CompletionResponse{}, fmt.Errorf("anthropic response content is empty")
	}
	respModel := out.Model
	if respModel == "" {
		respModel = model
	}
	return CompletionResponse{
		Text:  result,
		Model: respModel,
		Usage: Usage{InputTokens: out.Usage.InputTokens, OutputTokens: out.Usage.OutputTokens},
	}, nil
}
