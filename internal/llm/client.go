package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 是最小化的 OpenAI 兼容聊天 HTTP 客户端（429/5xx 时自动重试）
type Client struct {
	BaseURL    string
	APIKey     string
	HTTP       *http.Client
	MaxRetries int
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
	Temp     float64       `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ChatCompletion 调用 POST /v1/chat/completions 并支持重试
func (c *Client) ChatCompletion(ctx context.Context, model, userPrompt string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("missing API key")
	}
	body, _ := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: userPrompt},
		},
		Temp: 0.1,
	})
	url := c.BaseURL
	if url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	url += "/chat/completions"

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	retries := c.MaxRetries
	if retries <= 0 {
		retries = 3
	}
	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * 300 * time.Millisecond):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("upstream %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
		}
		var out chatResponse
		if err := json.Unmarshal(b, &out); err != nil {
			return "", err
		}
		if out.Error != nil {
			return "", fmt.Errorf("api error: %s", out.Error.Message)
		}
		if len(out.Choices) == 0 {
			return "", fmt.Errorf("empty choices")
		}
		return out.Choices[0].Message.Content, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("exhausted retries")
}
