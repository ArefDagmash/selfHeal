package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSuggestDisabled confirms we fail fast (not panic, not hang) with no key
// and no Ollama configured.
func TestSuggestDisabled(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OLLAMA_MODEL", "")
	t.Setenv("OLLAMA_BASE_URL", "")
	c := New()
	if c.Enabled() {
		t.Fatal("client should be disabled without any provider")
	}
	_, err := c.Suggest(t.Context(), FailureContext{Name: "x"})
	if err == nil {
		t.Fatal("expected error when disabled")
	}
}

// TestSuggestOllamaViaMock confirms the OpenAI-compatible Ollama path parses a
// chat/completions response correctly, using a fake server.
func TestSuggestOllamaViaMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"The endpoint returns 500; check the server logs for the panic."}}]}`))
	}))
	defer srv.Close()

	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OLLAMA_MODEL", "test-model")
	t.Setenv("OLLAMA_BASE_URL", srv.URL+"/v1")

	c := New()
	if c.Provider() != "ollama" {
		t.Fatalf("expected ollama provider, got %q", c.Provider())
	}
	got, err := c.Suggest(t.Context(), FailureContext{Name: "checkout", ErrorType: "assertion"})
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if !strings.Contains(got, "endpoint returns 500") {
		t.Fatalf("unexpected suggestion: %q", got)
	}
}

// TestTrimOneLine collapses whitespace to a single clean line.
func TestTrimOneLine(t *testing.T) {
	got := trimOneLine("  the\n\tendpoint\n is down  ")
	if got != "the endpoint is down" {
		t.Fatalf("unexpected trim: %q", got)
	}
}

// TestBuildPromptIncludesContext checks the prompt carries the failure details
// the model needs.
func TestBuildPromptIncludesContext(t *testing.T) {
	p := buildPrompt(FailureContext{
		Name:           "checkout_api",
		Type:           "api",
		ErrorType:      "assertion",
		ErrorMessage:   "expected 200 got 500",
		URL:            "https://shop/checkout",
		ExpectedStatus: 200,
	})
	for _, want := range []string{"checkout_api", "assertion", "expected 200 got 500", "https://shop/checkout", "200"} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt missing %q\n%s", want, p)
		}
	}
}

// TestSummarizeDisabled mirrors TestSuggestDisabled for the run-report path.
func TestSummarizeDisabled(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OLLAMA_MODEL", "")
	t.Setenv("OLLAMA_BASE_URL", "")
	c := New()
	_, err := c.Summarize(t.Context(), []RunResult{{Name: "x", Status: "failed"}})
	if err == nil {
		t.Fatal("expected error when disabled")
	}
}

// TestSummarizeEmptyResults refuses to call the model with nothing to report on.
func TestSummarizeEmptyResults(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OLLAMA_MODEL", "test-model")
	t.Setenv("OLLAMA_BASE_URL", "http://unused.invalid")
	c := New()
	_, err := c.Summarize(t.Context(), nil)
	if err == nil {
		t.Fatal("expected error for empty results")
	}
}

// TestSummarizeOllamaViaMock confirms Summarize reuses the same Ollama path as
// Suggest, just with a different prompt and a fake multi-result run.
func TestSummarizeOllamaViaMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"2 tests self-healed after a selector rename, and checkout_api failed on a dead host."}}]}`))
	}))
	defer srv.Close()

	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OLLAMA_MODEL", "test-model")
	t.Setenv("OLLAMA_BASE_URL", srv.URL+"/v1")

	c := New()
	got, err := c.Summarize(t.Context(), []RunResult{
		{Name: "shop_submit", Status: "healed", OldSelector: ".submit", NewSelector: ".submit-btn"},
		{Name: "checkout_api", Status: "failed", ErrorType: "network", ErrorMessage: "dial tcp: connection refused"},
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if !strings.Contains(got, "self-healed") {
		t.Fatalf("unexpected summary: %q", got)
	}
}

// TestBuildSummaryPromptIncludesEachResult checks every result's (humanized)
// name and outcome detail makes it into the prompt.
func TestBuildSummaryPromptIncludesEachResult(t *testing.T) {
	p := buildSummaryPrompt([]RunResult{
		{Name: "shop_submit", Status: "healed", OldSelector: ".submit", NewSelector: ".submit-btn"},
		{Name: "checkout_api", Status: "failed", ErrorType: "network", ErrorMessage: "dial tcp: connection refused"},
		{Name: "home_h1", Status: "passed"},
	})
	for _, want := range []string{"shop submit", ".submit to .submit-btn", "checkout api", "dial tcp: connection refused", "home h1: passed"} {
		if !strings.Contains(p, want) {
			t.Fatalf("summary prompt missing %q\n%s", want, p)
		}
	}
	if strings.Contains(p, "shop_submit") || strings.Contains(p, "checkout_api") || strings.Contains(p, "home_h1") {
		t.Fatalf("summary prompt should not contain raw underscored identifiers:\n%s", p)
	}
}

// TestHumanize turns snake_case into space-separated words.
func TestHumanize(t *testing.T) {
	got := humanize("broken_api_dead_host")
	if got != "broken api dead host" {
		t.Fatalf("unexpected humanize result: %q", got)
	}
}
