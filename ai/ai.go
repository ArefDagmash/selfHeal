package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// FailureContext is the slice of information the model needs to suggest a fix.
// Keeping it as a plain struct (rather than importing the runner) avoids a
// dependency from the ai package back into the runner.
type FailureContext struct {
	Name           string
	Type           string // "api" | "ui"
	ErrorType      string // timeout | assertion | network | selector_not_found
	ErrorMessage   string
	URL            string
	ExpectedStatus int
	Selector       string
}

// Client turns a failure into a one-line suggested root cause + fix using either
// Anthropic's API or a local Ollama server (OpenAI-compatible). Provider is
// chosen at construction from environment variables; if neither is configured
// the client is disabled and Suggest returns an error so callers can skip AI.
type Client struct {
	provider string // "anthropic" | "ollama" | ""
	apiKey   string
	model    string
	baseURL  string
	httpc    *http.Client
}

// New builds a client based on the environment:
//   - ANTHROPIC_API_KEY set  -> Anthropic Messages API (model from AI_MODEL)
//   - OLLAMA_MODEL (or OLLAMA_BASE_URL) set -> local Ollama, OpenAI-compatible
//     (model from OLLAMA_MODEL, default qwen2.5-coder:7b)
//
// With neither, the client is disabled.
func New() *Client {
	c := &Client{httpc: &http.Client{Timeout: 60 * time.Second}}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		c.provider = "anthropic"
		c.apiKey = key
		c.model = envOr("AI_MODEL", "claude-sonnet-4-5")
		c.baseURL = "https://api.anthropic.com/v1/messages"
		return c
	}
	if model := os.Getenv("OLLAMA_MODEL"); model != "" || os.Getenv("OLLAMA_BASE_URL") != "" {
		c.provider = "ollama"
		c.model = envOr("OLLAMA_MODEL", "qwen2.5-coder:7b")
		c.baseURL = strings.TrimRight(envOr("OLLAMA_BASE_URL", "http://localhost:11434/v1"), "/")
		return c
	}
	return c
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// Enabled reports whether a usable provider is configured.
func (c *Client) Enabled() bool { return c.provider != "" }

// Provider returns the resolved provider name ("anthropic", "ollama", or "").
func (c *Client) Provider() string { return c.provider }

// Suggest asks the configured model for a single-sentence root-cause + fix for
// the failure. It returns the trimmed suggestion text, or an error if the call
// fails or no provider is configured.
func (c *Client) Suggest(ctx context.Context, f FailureContext) (string, error) {
	if !c.Enabled() {
		return "", errors.New("ai disabled: set ANTHROPIC_API_KEY or OLLAMA_MODEL")
	}
	return c.complete(ctx, buildPrompt(f), 120)
}

// RunResult is one test's outcome, as fed into Summarize. It's a plain struct
// (rather than importing the runner/events packages) for the same reason as
// FailureContext: keeps the ai package dependency-free in that direction.
type RunResult struct {
	Name         string
	Status       string // passed | failed | healed
	ErrorType    string
	ErrorMessage string
	OldSelector  string // healed only
	NewSelector  string // healed only
}

// Summarize asks the configured model to recap a full suite run in a short
// paragraph — what broke, what self-healed, what still needs a human — rather
// than a one-line-per-failure suggestion. Same provider selection as Suggest.
func (c *Client) Summarize(ctx context.Context, results []RunResult) (string, error) {
	if !c.Enabled() {
		return "", errors.New("ai disabled: set ANTHROPIC_API_KEY or OLLAMA_MODEL")
	}
	if len(results) == 0 {
		return "", errors.New("no results to summarize")
	}
	return c.complete(ctx, buildSummaryPrompt(results), 220)
}

// complete routes a prompt to the configured provider and returns the trimmed
// reply text.
func (c *Client) complete(ctx context.Context, prompt string, maxTokens int) (string, error) {
	switch c.provider {
	case "anthropic":
		return c.completeAnthropic(ctx, prompt, maxTokens)
	case "ollama":
		return c.completeOllama(ctx, prompt, maxTokens)
	default:
		return "", errors.New("no ai provider configured")
	}
}

// --- Anthropic path ---

func (c *Client) completeAnthropic(ctx context.Context, prompt string, maxTokens int) (string, error) {
	body := map[string]any{
		"model":      c.model,
		"max_tokens": maxTokens,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	}
	return c.postJSON(ctx, c.baseURL, body, func(raw []byte) (string, error) {
		var out struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return "", err
		}
		for _, b := range out.Content {
			if b.Type == "text" && b.Text != "" {
				return trimOneLine(b.Text), nil
			}
		}
		return "", errors.New("empty suggestion from model")
	}, map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": "2023-06-01",
	})
}

// --- Ollama path (OpenAI-compatible /v1/chat/completions) ---

func (c *Client) completeOllama(ctx context.Context, prompt string, maxTokens int) (string, error) {
	body := map[string]any{
		"model":      c.model,
		"stream":     false,
		"max_tokens": maxTokens,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	}
	url := c.baseURL + "/chat/completions"
	return c.postJSON(ctx, url, body, func(raw []byte) (string, error) {
		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return "", err
		}
		if len(out.Choices) > 0 && out.Choices[0].Message.Content != "" {
			return trimOneLine(out.Choices[0].Message.Content), nil
		}
		return "", errors.New("empty suggestion from model")
	}, nil)
}

// postJSON marshals body, POSTs it to url, and hands the raw response to parse
// for extraction. Extra headers (e.g. auth) may be supplied.
func (c *Client) postJSON(ctx context.Context, url string, body any, parse func([]byte) (string, error), headers map[string]string) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai %d: %s", resp.StatusCode, string(raw))
	}
	return parse(raw)
}

// buildPrompt assembles a compact, role-focused prompt. We ask for exactly one
// sentence so the result fits the dashboard's single detail line.
func buildPrompt(f FailureContext) string {
	p := "You are a test-automation failure analyst. Given one failing automated test, " +
		"reply with EXACTLY ONE short sentence (under 25 words): state the likely root cause and the fix. " +
		"Be concrete. No bullets, no preamble.\n\n"
	p += fmt.Sprintf("Test name: %s\n", f.Name)
	p += fmt.Sprintf("Test type: %s\n", f.Type)
	p += fmt.Sprintf("Error type: %s\n", f.ErrorType)
	if f.ErrorMessage != "" {
		p += fmt.Sprintf("Error message: %s\n", f.ErrorMessage)
	}
	if f.URL != "" {
		p += fmt.Sprintf("Target URL: %s\n", f.URL)
	}
	if f.Type == "api" && f.ExpectedStatus != 0 {
		p += fmt.Sprintf("Expected HTTP status: %d\n", f.ExpectedStatus)
	}
	if f.Type == "ui" && f.Selector != "" {
		p += fmt.Sprintf("UI selector: %s\n", f.Selector)
	}
	return p
}

// buildSummaryPrompt assembles a compact recap request over every result in
// the run. Unlike buildPrompt (one failure, one sentence), this asks for a
// short paragraph covering the whole run, written for a non-technical reader.
func buildSummaryPrompt(results []RunResult) string {
	p := "You are explaining a test run to someone non-technical. Write a SHORT report " +
		"(2-3 sentences, plain everyday English) covering: what broke, what fixed itself, and " +
		"what still needs a person to check. Use normal, simple words — no jargon, no code terms, " +
		"no bullets, no headers. Never write a raw identifier with underscores or dots (like " +
		"broken_api_dead_host or .submit-btn) — always describe it in plain words instead (e.g. " +
		"\"the dead-host check\" or \"the submit button\"). Keep it short.\n\n"
	for _, r := range results {
		line := fmt.Sprintf("- %s: %s", humanize(r.Name), r.Status)
		switch {
		case r.Status == "healed":
			line += fmt.Sprintf(" (its selector changed from %s to %s)", r.OldSelector, r.NewSelector)
		case r.Status == "failed" && r.ErrorMessage != "":
			line += fmt.Sprintf(" (%s: %s)", r.ErrorType, r.ErrorMessage)
		case r.Status == "failed" && r.ErrorType != "":
			line += fmt.Sprintf(" (%s)", r.ErrorType)
		}
		p += line + "\n"
	}
	return p
}

// humanize turns a snake_case test name into space-separated words, so the
// data fed to the model already looks like plain language rather than a code
// identifier the model might just echo back verbatim.
func humanize(name string) string {
	return strings.ReplaceAll(name, "_", " ")
}

// trimOneLine collapses all whitespace runs into single spaces and trims the
// ends, so the suggestion reads cleanly on one dashboard line.
func trimOneLine(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			if len(out) == 0 || out[len(out)-1] == ' ' {
				continue // collapse consecutive whitespace
			}
			out = append(out, ' ')
		} else {
			out = append(out, r)
		}
	}
	for len(out) > 0 && out[len(out)-1] == ' ' {
		out = out[:len(out)-1]
	}
	return string(out)
}
