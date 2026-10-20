package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type openAIProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func newOpenAIProvider(cfg ProviderConfig) (*openAIProvider, error) {
	baseURL := trimBase(cfg.BaseURL)
	if baseURL == "" {
		baseURL = trimBase(envOr("OPENAI_BASE_URL", "https://api.openai.com/v1"))
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(envOr("OPENAI_API_KEY", ""))
	}
	if apiKey == "" && baseURL == "https://api.openai.com/v1" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set (or pass --base-url for a local OpenAI-compatible server)")
	}
	model := cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &openAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: timeoutOr(cfg.RequestTimeout, 2*time.Minute)},
	}, nil
}

func (p *openAIProvider) Name() string { return "openai" }

type openAIRequest struct {
	Model          string    `json:"model"`
	Messages       []Message `json:"messages"`
	MaxTokens      int       `json:"max_tokens,omitempty"`
	Temperature    float64   `json:"temperature"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
}

type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (p *openAIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	body := openAIRequest{
		Model:       model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	if req.JSON {
		body.ResponseFormat = &struct {
			Type string `json:"type"`
		}{Type: "json_object"}
	}
	var out openAIResponse
	headers := map[string]string{"Authorization": "Bearer " + p.apiKey}
	if p.apiKey == "" {
		headers["Authorization"] = ""
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/chat/completions", headers, body, &out); err != nil {
		return CompletionResponse{}, err
	}
	if len(out.Choices) == 0 {
		return CompletionResponse{}, fmt.Errorf("openai response has no choices")
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return CompletionResponse{}, fmt.Errorf("openai response content is empty")
	}
	respModel := out.Model
	if respModel == "" {
		respModel = model
	}
	return CompletionResponse{
		Text:  text,
		Model: respModel,
		Usage: Usage{InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens},
	}, nil
}
