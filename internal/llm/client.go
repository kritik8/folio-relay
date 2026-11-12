package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const DefaultGroqURL = "https://api.groq.com/openai/v1/chat/completions"
const DefaultGroqModel = "llama-3.1-8b-instant"

var GroqModelFallbacks = []string{
	"llama-3.3-70b-versatile",
	"llama-3.1-8b-instant",
	"openai/gpt-oss-20b",
	"qwen/qwen3-32b",
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model         string         `json:"model"`
	Messages      []ChatMessage  `json:"messages"`
	Stream        bool           `json:"stream"`
	Temperature   float64        `json:"temperature,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type ChatCompletion struct {
	Content        string
	Usage          map[string]any
	RateLimit      map[string]string
	HTTPStatusCode int
	RetryAfter     string
	FirstTokenMS   int
	TotalMS        int
	Attempts       int
}

type Client struct {
	APIKey  string
	BaseURL string
	Model   string
	Real    bool
}

func NewClient() *Client {
	model := normalizeValue(os.Getenv("GROQ_MODEL"))
	if model == "" {
		model = DefaultGroqModel
	}
	apiKey := normalizeValue(os.Getenv("GROQ_API_KEY"))
	switch strings.ToLower(apiKey) {
	case "", "your_key_here", "gsk_your_actual_key_here", "replace_me":
		apiKey = ""
	}
	return &Client{
		APIKey:  apiKey,
		BaseURL: DefaultGroqURL,
		Model:   model,
		Real:    apiKey != "",
	}
}

func normalizeValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func (c *Client) ChatWithFallback(req ChatRequest) (*ChatCompletion, error) {
	candidates := make([]string, 0, len(GroqModelFallbacks)+1)
	candidates = append(candidates, normalizeValue(req.Model))
	for _, model := range GroqModelFallbacks {
		if model != candidates[0] {
			candidates = append(candidates, model)
		}
	}
	var lastCompletion *ChatCompletion
	var lastErr error
	for attempt, model := range candidates {
		req.Model = model
		completion, err := c.Chat(req)
		if completion != nil {
			completion.Attempts = attempt + 1
			lastCompletion = completion
		}
		if err == nil {
			c.Model = model
			return completion, nil
		}
		lastErr = err
		if completion == nil || completion.HTTPStatusCode != http.StatusNotFound || !strings.Contains(strings.ToLower(err.Error()), "model") {
			return completion, err
		}
	}
	return lastCompletion, lastErr
}

func (c *Client) Chat(req ChatRequest) (*ChatCompletion, error) {
	if c.BaseURL != DefaultGroqURL {
		return nil, fmt.Errorf("Groq endpoint must be %s", DefaultGroqURL)
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY not set")
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}
	httpClient := &http.Client{Timeout: 20 * time.Second}
	started := time.Now()
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	result := &ChatCompletion{
		HTTPStatusCode: resp.StatusCode,
		RateLimit:      map[string]string{},
	}
	for _, h := range []string{
		"x-ratelimit-limit-requests",
		"x-ratelimit-limit-tokens",
		"x-ratelimit-remaining-requests",
		"x-ratelimit-remaining-tokens",
		"x-ratelimit-reset-requests",
		"x-ratelimit-reset-tokens",
		"retry-after",
	} {
		if v := resp.Header.Get(h); v != "" {
			result.RateLimit[h] = v
		}
	}
	result.RetryAfter = resp.Header.Get("retry-after")
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return result, fmt.Errorf("read Groq error response: %w", readErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return result, fmt.Errorf("429 rate limited: %s", string(body))
		}
		return result, fmt.Errorf("groq http %d: %s", resp.StatusCode, string(body))
	}
	if req.Stream {
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break
			}
			var event struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Usage map[string]any `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return result, fmt.Errorf("decode Groq stream event: %w", err)
			}
			if len(event.Choices) > 0 && event.Choices[0].Delta.Content != "" {
				if result.FirstTokenMS == 0 {
					result.FirstTokenMS = int(time.Since(started).Milliseconds())
				}
				result.Content += event.Choices[0].Delta.Content
			}
			if event.Usage != nil {
				result.Usage = event.Usage
			}
		}
		if err := scanner.Err(); err != nil {
			return result, fmt.Errorf("read Groq stream: %w", err)
		}
		result.TotalMS = int(time.Since(started).Milliseconds())
		if result.FirstTokenMS == 0 {
			result.FirstTokenMS = result.TotalMS
		}
		return result, nil
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return result, fmt.Errorf("read Groq response: %w", readErr)
	}
	result.FirstTokenMS = int(time.Since(started).Milliseconds())
	result.TotalMS = result.FirstTokenMS
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return result, err
	}
	if len(parsed.Choices) > 0 {
		result.Content = parsed.Choices[0].Message.Content
	}
	result.Usage = parsed.Usage
	return result, nil
}

func (c *Client) SimulateStream(prompt string) <-chan string {
	out := make(chan string, 32)
	go func() {
		defer close(out)
		tokens := strings.Fields(prompt)
		for _, token := range tokens {
			time.Sleep(25 * time.Millisecond)
			out <- token + " "
		}
	}()
	return out
}
