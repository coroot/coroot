// Package llm implements a minimal OpenAI-compatible chat completion client used for
// self-hosted root cause analysis. It intentionally depends only on the standard library
// so airgapped builds don't need a vendor-specific SDK.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultSystemPrompt = `You are a senior Site Reliability Engineer performing a root cause analysis of a production incident.

You will receive a JSON document describing one application during the incident window. It contains the
application status, failing health checks produced by Coroot's auditor, summarized vitals (requests per
second, error rate, latency percentiles, CPU and memory), top log patterns, upstream dependency health,
recent deployments, Kubernetes events, and sample traces that violated the SLO.

Rules:
- Base every claim on the supplied evidence. Never invent metric values, log lines, or component names.
- Prefer the most specific cause the evidence supports. If the evidence is inconclusive, say so and list
  what to check next instead of guessing.
- Pay attention to timing: a deployment or Kubernetes event just before the incident started is a strong
  signal. Failing upstream dependencies usually indicate the cause is downstream of this application.
- Be concise and concrete. An on-call engineer will read this while the incident is ongoing.

Respond with a single JSON object and nothing else, using exactly these keys:
{
  "short_summary": "One sentence, max 100 characters, describing the incident.",
  "root_cause": "One short markdown paragraph naming the most likely root cause and the evidence for it.",
  "immediate_fixes": "Markdown bullet list of concrete actions to mitigate the incident now.",
  "detailed_root_cause_analysis": "Markdown explaining the failure chain: symptoms, evidence, reasoning, and what to verify."
}`

// Result mirrors the text fields of model.RCA so the caller can copy it straight through.
type Result struct {
	ShortSummary      string `json:"short_summary"`
	RootCause         string `json:"root_cause"`
	ImmediateFixes    string `json:"immediate_fixes"`
	DetailedRootCause string `json:"detailed_root_cause_analysis"`
}

type Config struct {
	BaseUrl      string
	ApiKey       string
	Model        string
	SystemPrompt string
	Timeout      time.Duration
}

type Client struct {
	cfg        Config
	httpClient *http.Client
}

func NewClient(cfg Config) *Client {
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = DefaultSystemPrompt
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	cfg.BaseUrl = strings.TrimSuffix(cfg.BaseUrl, "/")
	return &Client{cfg: cfg, httpClient: &http.Client{Timeout: cfg.Timeout}}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Temperature    float32         `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Analyze sends the evidence document to the model and returns the parsed analysis.
// Models that ignore the JSON response format get one corrective retry before giving up.
func (c *Client) Analyze(ctx context.Context, evidence any) (*Result, error) {
	payload, err := json.Marshal(evidence)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal evidence: %w", err)
	}

	messages := []message{
		{Role: "system", Content: c.cfg.SystemPrompt},
		{Role: "user", Content: "Incident evidence:\n" + string(payload)},
	}

	for attempt := 0; attempt < 2; attempt++ {
		content, err := c.complete(ctx, messages)
		if err != nil {
			return nil, err
		}
		result, parseErr := parseResult(content)
		if parseErr == nil {
			return result, nil
		}
		if attempt == 1 {
			return nil, parseErr
		}
		messages = append(messages,
			message{Role: "assistant", Content: content},
			message{Role: "user", Content: fmt.Sprintf(
				"That response was rejected: %s. Reply with only a JSON object containing the keys short_summary, root_cause, immediate_fixes, and detailed_root_cause_analysis.", parseErr)},
		)
	}
	return nil, fmt.Errorf("unreachable")
}

func (c *Client) complete(ctx context.Context, messages []message) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:          c.cfg.Model,
		Messages:       messages,
		ResponseFormat: &responseFormat{Type: "json_object"},
		Temperature:    0.1,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseUrl+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.ApiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	// Cap the response so a misconfigured base URL can't stream an unbounded body into memory.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read llm response: %w", err)
	}

	var parsed chatResponse
	unmarshalErr := json.Unmarshal(raw, &parsed)

	if resp.StatusCode != http.StatusOK {
		if unmarshalErr == nil && parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("llm returned %s: %s", resp.Status, parsed.Error.Message)
		}
		return "", fmt.Errorf("llm returned %s: %s", resp.Status, truncate(string(raw), 300))
	}
	if unmarshalErr != nil {
		return "", fmt.Errorf("failed to parse llm response: %w", unmarshalErr)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("llm returned an error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func parseResult(content string) (*Result, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("the model returned an empty message")
	}
	var res Result
	if err := json.Unmarshal([]byte(stripCodeFence(content)), &res); err != nil {
		return nil, fmt.Errorf("the model did not return valid JSON: %w", err)
	}
	if res.RootCause == "" {
		return nil, fmt.Errorf("the model returned no root_cause")
	}
	if res.ShortSummary == "" {
		res.ShortSummary = firstSentence(res.RootCause)
	}
	return &res, nil
}

// stripCodeFence unwraps ```json ... ``` blocks, which models emit even when asked for raw JSON.
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func firstSentence(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if i := strings.Index(s, ". "); i > 0 {
		s = s[:i+1]
	}
	return truncate(s, 200)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
