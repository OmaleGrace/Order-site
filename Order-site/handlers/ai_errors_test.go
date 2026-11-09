package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestClassifyAIError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want aiFailure
	}{
		{"gemini high demand", &aiProviderError{Provider: "Gemini", Status: 503, Message: "This model is currently experiencing high demand."}, aiFailBusy},
		{"rate limited", &aiProviderError{Provider: "OpenAI", Status: 429, Message: "Rate limit reached"}, aiFailBusy},
		{"server error", &aiProviderError{Provider: "OpenAI", Status: 500}, aiFailBusy},
		{"connection dropped", &aiProviderError{Provider: "Gemini", Cause: fmt.Errorf("connection reset")}, aiFailBusy},
		{"quota exhausted", &aiProviderError{Provider: "OpenAI", Status: 429, Message: "You exceeded your current quota"}, aiFailUnavailable},
		{"bad key", &aiProviderError{Provider: "OpenAI", Status: 401, Message: "Incorrect API key provided"}, aiFailUnavailable},
		{"unknown model", &aiProviderError{Provider: "Gemini", Status: 404, Message: "model not found"}, aiFailUnavailable},
		{"deadline", context.DeadlineExceeded, aiFailTimeout},
		{"plain error", fmt.Errorf("something else"), aiFailUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyAIError(tc.err); got != tc.want {
				t.Fatalf("classifyAIError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAIUserMessageHidesTechnicalDetails(t *testing.T) {
	errs := []error{
		&aiProviderError{Provider: "Gemini", Status: 503, Message: "This model is currently experiencing high demand."},
		&aiProviderError{Provider: "OpenAI", Status: 401, Message: "Incorrect API key provided: sk-secret"},
		&aiProviderError{Provider: "OpenAI", Status: 429, Message: "You exceeded your current quota"},
		context.DeadlineExceeded,
	}
	banned := []string{"api key", "apikey", "quota", "gemini", "openai", "model", "sk-", "_key", "env"}
	for _, err := range errs {
		msg := strings.ToLower(aiUserMessage(err))
		if msg == "" {
			t.Fatalf("empty message for %v", err)
		}
		for _, word := range banned {
			if strings.Contains(msg, word) {
				t.Errorf("user message %q must not contain %q", msg, word)
			}
		}
	}
}

func TestCallWithRetry(t *testing.T) {
	old := aiRetryDelays
	aiRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { aiRetryDelays = old }()

	busy := &aiProviderError{Provider: "Gemini", Status: 503, Message: "high demand"}

	t.Run("recovers after temporary failures", func(t *testing.T) {
		calls := 0
		got, err := callWithRetry(context.Background(), func() (string, error) {
			calls++
			if calls < 3 {
				return "", busy
			}
			return "draft", nil
		})
		if err != nil || got != "draft" || calls != 3 {
			t.Fatalf("got %q, err %v after %d calls", got, err, calls)
		}
	})

	t.Run("gives up after retries run out", func(t *testing.T) {
		calls := 0
		_, err := callWithRetry(context.Background(), func() (string, error) {
			calls++
			return "", busy
		})
		if err == nil || calls != 3 {
			t.Fatalf("err %v after %d calls, want error after 3", err, calls)
		}
	})

	t.Run("does not retry unfixable errors", func(t *testing.T) {
		calls := 0
		_, err := callWithRetry(context.Background(), func() (string, error) {
			calls++
			return "", &aiProviderError{Provider: "OpenAI", Status: 401, Message: "bad key"}
		})
		if err == nil || calls != 1 {
			t.Fatalf("err %v after %d calls, want 1 call", err, calls)
		}
	})
}
