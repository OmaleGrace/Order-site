package handlers

import (
	"context"
	stderrors "errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// aiFailure groups provider failures by what the person using the site can do.
type aiFailure int

const (
	// aiFailBusy: rate limits, overload and 5xx responses. Usually clears on its own.
	aiFailBusy aiFailure = iota
	// aiFailTimeout: the provider did not answer in time.
	aiFailTimeout
	// aiFailUnavailable: bad key, exhausted quota, wrong model, not configured.
	// Only the site operator can fix these, so vendors are never shown the details.
	aiFailUnavailable
)

// aiProviderError keeps the provider's raw details for server logs only.
type aiProviderError struct {
	Provider string
	Status   int // HTTP status, 0 when the request never got a response
	Message  string
	Cause    error
}

func (e *aiProviderError) Error() string {
	switch {
	case e.Cause != nil:
		return fmt.Sprintf("%s request failed: %v", e.Provider, e.Cause)
	case e.Message != "":
		return fmt.Sprintf("%s API error (HTTP %d): %s", e.Provider, e.Status, e.Message)
	default:
		return fmt.Sprintf("%s API returned HTTP %d", e.Provider, e.Status)
	}
}

func (e *aiProviderError) Unwrap() error { return e.Cause }

// classifyAIError decides whether a failure is worth retrying and what to tell the user.
func classifyAIError(err error) aiFailure {
	if err == nil {
		return aiFailUnavailable
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return aiFailTimeout
	}
	var netErr net.Error
	if stderrors.As(err, &netErr) && netErr.Timeout() {
		return aiFailTimeout
	}
	var pe *aiProviderError
	if !stderrors.As(err, &pe) {
		return aiFailUnavailable
	}
	msg := strings.ToLower(pe.Message)
	// Billing and quota problems do not clear by waiting a few seconds.
	for _, hint := range []string{"quota", "billing", "insufficient", "exceeded your current"} {
		if strings.Contains(msg, hint) {
			return aiFailUnavailable
		}
	}
	switch pe.Status {
	case 0:
		return aiFailBusy // connection dropped before any response
	case 429, 500, 502, 503, 504:
		return aiFailBusy
	}
	for _, hint := range []string{"high demand", "overloaded", "try again later", "temporarily"} {
		if strings.Contains(msg, hint) {
			return aiFailBusy
		}
	}
	return aiFailUnavailable
}

const aiUnavailableMessage = "The AI assistant isn't available right now. Please try again later, or write this yourself for now."

// aiUserMessage is the only AI error text vendors see: no keys, models or providers.
func aiUserMessage(err error) string {
	switch classifyAIError(err) {
	case aiFailBusy:
		return "The AI assistant is very busy right now. Please wait a minute and try again."
	case aiFailTimeout:
		return "The AI assistant took too long to respond. Please try again."
	default:
		return aiUnavailableMessage
	}
}

// logAIFailure writes the technical detail and a fix suggestion for the site operator.
func logAIFailure(feature string, err error) {
	var hint string
	switch classifyAIError(err) {
	case aiFailBusy:
		hint = "provider is overloaded or rate limiting; retries were already attempted"
	case aiFailTimeout:
		hint = "provider did not answer in time; check network access and provider status"
	default:
		hint = "check the API key, model name, billing and quota for your AI provider"
	}
	log.Printf("%s: AI request failed: %v (operator hint: %s)", feature, err, hint)
}

// aiRetryDelays are the waits before retry 1 and retry 2. A variable so tests can shorten it.
var aiRetryDelays = []time.Duration{700 * time.Millisecond, 2 * time.Second}

// callWithRetry retries only failures that are likely to be temporary.
func callWithRetry(ctx context.Context, call func() (string, error)) (string, error) {
	for attempt := 0; ; attempt++ {
		out, err := call()
		if err == nil {
			return out, nil
		}
		if classifyAIError(err) != aiFailBusy || attempt >= len(aiRetryDelays) {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", err
		case <-time.After(aiRetryDelays[attempt]):
		}
	}
}
