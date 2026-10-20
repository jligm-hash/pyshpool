package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ProviderConfig configures a provider constructed from CLI flags/env.
type ProviderConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	RequestTimeout time.Duration
}

// NewProvider builds a provider by name. Empty means mock.
func NewProvider(name string, cfg ProviderConfig) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "mock":
		return &MockProvider{}, nil
	case "openai", "openai-compatible", "compatible":
		return newOpenAIProvider(cfg)
	case "anthropic", "claude":
		return newAnthropicProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown provider %q (use mock, openai, or anthropic)", name)
	}
}

func timeoutOr(d time.Duration, fallback time.Duration) time.Duration {
	if d <= 0 {
		return fallback
	}
	return d
}

func trimBase(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string {
	body := strings.TrimSpace(e.Body)
	if len(body) > 500 {
		body = body[:500] + "..."
	}
	if body == "" {
		return fmt.Sprintf("HTTP %d", e.Status)
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, body)
}

func postJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpError{Status: resp.StatusCode, Body: string(payload)}
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
