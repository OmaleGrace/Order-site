package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeGemini answers each request with the next canned reply and records what it was sent.
type fakeGemini struct {
	mu      sync.Mutex
	replies []string // JSON bodies
	bodies  []map[string]any
}

func (f *fakeGemini) server(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.bodies = append(f.bodies, body)
		i := len(f.bodies) - 1
		if i >= len(f.replies) {
			i = len(f.replies) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, f.replies[i])
	}))
	t.Cleanup(srv.Close)
	old := geminiBaseURL
	geminiBaseURL = srv.URL + "/models/"
	t.Cleanup(func() { geminiBaseURL = old })
	return srv.URL
}

func geminiReply(text, finish string) string {
	b, _ := json.Marshal(map[string]any{"candidates": []map[string]any{{
		"content":      map[string]any{"parts": []map[string]string{{"text": text}}},
		"finishReason": finish,
	}}})
	return string(b)
}

func genConfig(body map[string]any) map[string]any {
	cfg, _ := body["generationConfig"].(map[string]any)
	return cfg
}

const fullBriefing = "**Daily Business Briefing: Mama Put**\n\nYesterday you took 12 orders worth ₦48,500. Jollof rice was your best seller, so keep it well stocked. Tomorrow looks busy."

func TestGeminiRetriesWithBiggerBudgetWhenCutOff(t *testing.T) {
	f := &fakeGemini{replies: []string{
		geminiReply("**Daily Business Briefing: Mama Put**", "MAX_TOKENS"), // thinking used the budget up
		geminiReply(fullBriefing, "STOP"),
	}}
	f.server(t)

	got, err := generateGeminiDraft(context.Background(), "key", "gemini-2.5-flash", "sys", "user", 350)
	if err != nil {
		t.Fatal(err)
	}
	if got != fullBriefing {
		t.Fatalf("got %q, want the complete briefing", got)
	}
	if len(f.bodies) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(f.bodies))
	}
	first, second := genConfig(f.bodies[0])["maxOutputTokens"].(float64), genConfig(f.bodies[1])["maxOutputTokens"].(float64)
	if first != 350 || second <= first {
		t.Fatalf("budgets = %v then %v, want 350 then something larger", first, second)
	}
}

func TestGeminiFlashRunsWithoutHiddenThinking(t *testing.T) {
	f := &fakeGemini{replies: []string{geminiReply(fullBriefing, "STOP")}}
	f.server(t)
	if _, err := generateGeminiDraft(context.Background(), "key", "gemini-2.5-flash", "s", "u", 1200); err != nil {
		t.Fatal(err)
	}
	tc, ok := genConfig(f.bodies[0])["thinkingConfig"].(map[string]any)
	if !ok || tc["thinkingBudget"] != float64(0) {
		t.Fatalf("expected thinkingBudget 0 for a 2.5 Flash model, got %v", genConfig(f.bodies[0]))
	}
}

func TestGeminiOtherModelsAreNotSentThinkingConfig(t *testing.T) {
	for _, model := range []string{"gemini-2.5-pro", "gemini-2.0-flash", "gemini-1.5-flash"} {
		f := &fakeGemini{replies: []string{geminiReply(fullBriefing, "STOP")}}
		f.server(t)
		if _, err := generateGeminiDraft(context.Background(), "key", model, "s", "u", 1200); err != nil {
			t.Fatal(err)
		}
		if _, present := genConfig(f.bodies[0])["thinkingConfig"]; present {
			t.Errorf("%s: thinkingConfig must not be sent (the API rejects it)", model)
		}
	}
}

func TestStillCutOffKeepsOnlyCompleteSentences(t *testing.T) {
	cut := "Yesterday you took 12 orders worth ₦48,500 and Jollof rice sold best. Tomorrow you should prepare extra stock because the week"
	f := &fakeGemini{replies: []string{geminiReply(cut, "MAX_TOKENS")}}
	f.server(t)

	got, err := generateGeminiDraft(context.Background(), "key", "gemini-2.5-flash", "s", "u", 350)
	if err != nil {
		t.Fatal(err)
	}
	want := "Yesterday you took 12 orders worth ₦48,500 and Jollof rice sold best."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOnlyAHeadingIsAnErrorNotAnAnswer(t *testing.T) {
	f := &fakeGemini{replies: []string{geminiReply("**Daily Business Briefing: Mama Put**", "MAX_TOKENS")}}
	f.server(t)

	_, err := generateGeminiDraft(context.Background(), "key", "gemini-2.5-flash", "s", "u", 350)
	if err == nil {
		t.Fatal("a title with nothing after it must not be shown as a finished answer")
	}
	if classifyAIError(err) != aiFailBusy {
		t.Fatalf("a cut-off answer should ask the person to try again, got class %v", classifyAIError(err))
	}
}

func TestOpenAIRetriesWhenFinishReasonIsLength(t *testing.T) {
	var budgets []float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		budgets = append(budgets, body["max_tokens"].(float64))
		finish, text := "length", "**Daily Business Briefing: Mama Put**"
		if len(budgets) == 2 {
			finish, text = "stop", fullBriefing
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{
			"message": map[string]string{"role": "assistant", "content": text}, "finish_reason": finish,
		}}})
	}))
	defer srv.Close()
	old := openAIEndpoint
	openAIEndpoint = srv.URL
	defer func() { openAIEndpoint = old }()

	got, err := generateOpenAIDraft(context.Background(), "key", "gpt-4o-mini", "s", "u", 350)
	if err != nil || !strings.Contains(got, "Tomorrow looks busy.") {
		t.Fatalf("got %q, %v", got, err)
	}
	if len(budgets) != 2 || budgets[1] <= budgets[0] {
		t.Fatalf("budgets = %v", budgets)
	}
}

func TestTrimToLastSentence(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"No punctuation at all": "",
		"Short. Ok":             "", // too short to be useful
		"A complete and useful first sentence that is clearly long enough to keep. Then a half":      "A complete and useful first sentence that is clearly long enough to keep.",
		"A complete and useful first sentence that is clearly long enough to keep! And then more of": "A complete and useful first sentence that is clearly long enough to keep!",
	}
	for in, want := range cases {
		if got := trimToLastSentence(in); got != want {
			t.Errorf("trimToLastSentence(%q) = %q, want %q", in, got, want)
		}
	}
}
