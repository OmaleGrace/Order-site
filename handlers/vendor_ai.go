package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"Order-site/errors"
	"Order-site/render"
)

const defaultGeminiModel = "gemini-2.5-flash"

var vendorAIHTTPClient = &http.Client{
	Timeout: 35 * time.Second,
}

type VendorAIPage struct {
	VendorName      string
	InsightSummary  string
	Tool            string
	DishName        string
	Details         string
	CustomerMessage string
	Tone            string
	Result          string
	Notice          string
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

// geminiThinkingConfig: Gemini 2.5 models "think" before answering and that hidden thinking is
// counted against maxOutputTokens, so a small limit can be used up before any visible answer.
// ThinkingBudget 0 switches thinking off (it must be sent even though it is zero).
type geminiThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int                   `json:"maxOutputTokens,omitempty"`
	ThinkingConfig  *geminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

// geminiCanDisableThinking is true for the 2.5 Flash family. Other models either do not support
// the setting (2.0) or cannot run without thinking (2.5 Pro), and would reject the request.
func geminiCanDisableThinking(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "gemini-2.5") && strings.Contains(m, "flash")
}

type geminiRequest struct {
	SystemInstruction *geminiContent         `json:"system_instruction,omitempty"`
	Contents          []geminiContent        `json:"contents"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`

	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func geminiAPIKey() string {
	return strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
}

func geminiModel() string {
	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		return defaultGeminiModel
	}
	return model
}

func openAIKey() string { return strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) }

func openAIModel() string {
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		return "gpt-4o-mini"
	}
	return model
}

func aiConfigured() bool { return openAIKey() != "" || geminiAPIKey() != "" }

// generateAIDraft follows the provider documented in .env.example (OpenAI) and
// retains Gemini support for deployments that already configured GEMINI_API_KEY.
func generateAIDraft(ctx context.Context, systemPrompt, userPrompt string, maxOutputTokens int) (string, error) {
	var lastErr error
	if key := openAIKey(); key != "" {
		out, err := callWithRetry(ctx, func() (string, error) {
			return generateOpenAIDraft(ctx, key, openAIModel(), systemPrompt, userPrompt, maxOutputTokens)
		})
		if err == nil {
			return out, nil
		}
		lastErr = err
		if geminiAPIKey() != "" {
			log.Printf("AI: OpenAI failed, trying Gemini: %v", err)
		}
	}
	if key := geminiAPIKey(); key != "" {
		out, err := callWithRetry(ctx, func() (string, error) {
			return generateGeminiDraft(ctx, key, geminiModel(), systemPrompt, userPrompt, maxOutputTokens)
		})
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("AI is not configured: set OPENAI_API_KEY or GEMINI_API_KEY")
}

// Where the providers are reached. Variables so tests can point them at a local server.
var (
	openAIEndpoint = "https://api.openai.com/v1/chat/completions"
	geminiBaseURL  = "https://generativelanguage.googleapis.com/v1beta/models/"
)

// maxAIRetryTokens caps the larger budget used when a first answer is cut off.
const maxAIRetryTokens = 4096

// cutOffAIError is returned when an answer is still unfinished after a retry. It is reported as
// a temporary (5xx) failure, so the person is simply asked to try again.
func cutOffAIError(provider string) error {
	return &aiProviderError{Provider: provider, Status: 503, Message: "the answer was cut off before it finished"}
}

// trimToLastSentence drops a trailing half-sentence from a cut-off answer. It returns "" when
// what is left would be too short to be useful (for example only a heading).
func trimToLastSentence(text string) string {
	text = strings.TrimSpace(text)
	cut := strings.LastIndexAny(text, ".!?")
	if cut < 0 {
		return ""
	}
	end := cut + 1
	// keep a closing quote or bracket that belongs to the sentence
	for end < len(text) && strings.ContainsRune(`"')”’*)`, rune(text[end])) {
		end++
	}
	out := strings.TrimSpace(text[:end])
	if len([]rune(out)) < 60 {
		return ""
	}
	return out
}

// generateWithRetry runs one provider call. If the provider stopped because it hit the output
// limit, it asks again once with a larger limit; if the answer is still unfinished it keeps only
// the complete sentences, and fails (so the person can retry) rather than show half a sentence.
func generateWithRetry(
	provider string,
	maxOutputTokens int,
	once func(budget int) (text string, truncated bool, err error),
) (string, error) {
	budget := maxOutputTokens
	last := ""
	for attempt := 0; attempt < 2; attempt++ {
		text, truncated, err := once(budget)
		if err != nil {
			return "", err
		}
		if !truncated {
			return text, nil
		}
		last = text
		if budget = budget * 3; budget > maxAIRetryTokens {
			budget = maxAIRetryTokens
		}
	}
	if trimmed := trimToLastSentence(last); trimmed != "" {
		return trimmed, nil
	}
	return "", cutOffAIError(provider)
}

func generateOpenAIDraft(ctx context.Context, apiKey, model, systemPrompt, userPrompt string, maxOutputTokens int) (string, error) {
	return generateWithRetry("OpenAI", maxOutputTokens, func(budget int) (string, bool, error) {
		return openAIOnce(ctx, apiKey, model, systemPrompt, userPrompt, budget)
	})
}

func generateGeminiDraft(ctx context.Context, apiKey, model, systemPrompt, userPrompt string, maxOutputTokens int) (string, error) {
	return generateWithRetry("Gemini", maxOutputTokens, func(budget int) (string, bool, error) {
		return geminiOnce(ctx, apiKey, model, systemPrompt, userPrompt, budget)
	})
}

func openAIOnce(ctx context.Context, apiKey, model, systemPrompt, userPrompt string, maxOutputTokens int) (string, bool, error) {
	payload := openAIRequest{
		Model: model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   maxOutputTokens,
		Temperature: 0.7,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", false, fmt.Errorf("marshal OpenAI request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", false, fmt.Errorf("create OpenAI request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	resp, err := vendorAIHTTPClient.Do(req)
	if err != nil {
		return "", false, &aiProviderError{Provider: "OpenAI", Cause: err}
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", false, fmt.Errorf("read OpenAI response: %w", err)
	}
	var decoded openAIResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", false, fmt.Errorf("decode OpenAI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decoded.Error != nil && decoded.Error.Message != "" {
			return "", false, &aiProviderError{Provider: "OpenAI", Status: resp.StatusCode, Message: decoded.Error.Message}
		}
		return "", false, &aiProviderError{Provider: "OpenAI", Status: resp.StatusCode}
	}
	if len(decoded.Choices) == 0 {
		return "", false, fmt.Errorf("OpenAI returned no choices")
	}
	result := strings.TrimSpace(decoded.Choices[0].Message.Content)
	truncated := decoded.Choices[0].FinishReason == "length"
	if result == "" && !truncated {
		return "", false, fmt.Errorf("OpenAI returned an empty response")
	}
	return result, truncated, nil
}

func geminiOnce(
	ctx context.Context,
	apiKey string,
	model string,
	systemPrompt string,
	userPrompt string,
	maxOutputTokens int,
) (string, bool, error) {
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)

	if apiKey == "" {
		return "", false, fmt.Errorf("GEMINI_API_KEY is not configured")
	}

	if model == "" {
		model = defaultGeminiModel
	}

	endpoint := geminiBaseURL +
		url.PathEscape(model) +
		":generateContent"

	payload := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{
				{Text: systemPrompt},
			},
		},
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: userPrompt},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			MaxOutputTokens: maxOutputTokens,
		},
	}
	if geminiCanDisableThinking(model) {
		payload.GenerationConfig.ThinkingConfig = &geminiThinkingConfig{ThinkingBudget: 0}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", false, fmt.Errorf("marshal Gemini request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", false, fmt.Errorf("create Gemini request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := vendorAIHTTPClient.Do(req)
	if err != nil {
		return "", false, &aiProviderError{Provider: "Gemini", Cause: err}
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", false, fmt.Errorf("read Gemini response: %w", err)
	}

	var decoded geminiResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", false, fmt.Errorf("decode Gemini response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decoded.Error != nil && decoded.Error.Message != "" {
			return "", false, &aiProviderError{Provider: "Gemini", Status: resp.StatusCode, Message: decoded.Error.Message}
		}

		return "", false, &aiProviderError{Provider: "Gemini", Status: resp.StatusCode}
	}

	if len(decoded.Candidates) == 0 {
		return "", false, fmt.Errorf("Gemini returned no candidates")
	}

	var parts []string

	for _, part := range decoded.Candidates[0].Content.Parts {
		text := strings.TrimSpace(part.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}

	result := strings.TrimSpace(strings.Join(parts, "\n"))
	truncated := decoded.Candidates[0].FinishReason == "MAX_TOKENS"

	if result == "" && !truncated {
		return "", false, fmt.Errorf("Gemini returned an empty response")
	}

	return result, truncated, nil
}

// VendorAI shows the optional AI writing assistant to the owner of a shop.
func (h *Handlers) VendorAI(w http.ResponseWriter, r *http.Request) {
	_, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	p := VendorAIPage{
		VendorName: vendorName,
		Tool:       "description",
		Tone:       "warm and appetising",
		Notice:     r.URL.Query().Get("msg"),
	}

	h.renderVendorAI(w, p, http.StatusOK)
}

// VendorAIGenerate creates draft marketing copy.
// It never changes a live menu or post automatically.
func (h *Handlers) VendorAIGenerate(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Invalid form submission", http.StatusBadRequest)
		return
	}

	p := VendorAIPage{
		VendorName:      vendorName,
		Tool:            strings.TrimSpace(r.FormValue("tool")),
		DishName:        strings.TrimSpace(r.FormValue("dish_name")),
		Details:         strings.TrimSpace(r.FormValue("details")),
		CustomerMessage: strings.TrimSpace(r.FormValue("customer_message")),
		Tone:            strings.TrimSpace(r.FormValue("tone")),
	}

	if validationMessage := validateVendorAIInput(p); validationMessage != "" {
		p.Notice = validationMessage
		h.renderVendorAI(w, p, http.StatusBadRequest)
		return
	}

	if p.Tool == "business_brief" {
		insight, err := h.vendorAIDataSummary(r.Context(), vendorID)
		if err != nil {
			log.Printf("vendor AI: analytics summary failed: %v", err)
			p.Notice = "I could not load your shop data right now. Please try again or open Analytics directly."
			h.renderVendorAI(w, p, http.StatusInternalServerError)
			return
		}
		p.InsightSummary = insight
	}

	if !aiConfigured() {
		log.Printf("vendor AI: requested but no OPENAI_API_KEY or GEMINI_API_KEY is set (operator hint: add one to the server environment)")
		p.Notice = aiUnavailableMessage
		h.renderVendorAI(w, p, http.StatusOK)
		return
	}

	systemPrompt := `You are ENIE's practical Nigerian food-business copywriting assistant.

Write clear, appetising, honest copy.

Do not invent:
- ingredients
- health claims
- discounts
- delivery promises
- awards
- availability
- prices

Use only facts provided by the vendor.

Return only the requested draft, with no preamble.`

	requestPrompt := "Shop: " + vendorName +
		"\nDish/product or topic: " + p.DishName +
		"\nVendor details: " + p.Details +
		"\nTone: " + p.Tone +
		"\n"

	if p.Tool == "business_brief" {
		requestPrompt += "\nVerified shop data (last 7 days):\n" + p.InsightSummary + "\n"
	}

	switch p.Tool {
	case "description":
		requestPrompt += "Write one appealing menu description of 2-4 sentences, suitable for a Nigerian food marketplace. Keep it concise and do not add unprovided ingredients."

	case "social":
		requestPrompt += "Write one engaging social media promotional caption for this food business, with a clear call to action and 3-5 relevant hashtags. Do not invent prices or offers."

	case "menu_ideas":
		requestPrompt += "Suggest 5 practical menu item ideas that fit the food/product and vendor details. For each, give a short name and a one-sentence description. Clearly mark these as ideas, not confirmed menu items. Do not invent the vendor's existing ingredients or stock."

	case "whatsapp":
		requestPrompt += "Write a concise WhatsApp customer broadcast message, friendly and respectful, with a clear call to action. Do not claim a discount, price, or availability unless explicitly provided."

	case "campaign":
		requestPrompt += "Create a simple 3-post promotional content plan for this food business. For each post provide a hook, caption, and call to action. Do not invent prices, discounts, awards, ingredients, or availability."

	case "review_reply":
		requestPrompt += "Draft a polite, professional response to a customer review or feedback. Acknowledge the feedback, avoid arguing or revealing personal data, and do not promise compensation unless stated in the supplied details."

	case "business_brief":
		requestPrompt += "Create a short daily business briefing using only the verified shop data above. Include: what is going well, one meaningful trend, one practical action for today, and one thing to monitor. If the data is insufficient, say so. Do not invent causes, customer demographics, forecasts, prices, or targets. Keep it under 300 words."

	case "pricing":
		requestPrompt += "Create a pricing worksheet for the vendor to fill in: ingredients cost, packaging, labour, overhead, desired margin, suggested price formula, and questions to consider. Do not invent costs or claim a definitive price without cost data."

	case "customer_chat":
		requestPrompt += `You are helping the shop owner draft a reply to a customer chat message.

Customer message: "` + p.CustomerMessage + `"

Draft a warm, concise, helpful response that sounds natural for a Nigerian food business.

Answer only using the vendor details provided.

Never invent:
- prices
- stock
- ingredients
- opening hours
- delivery times
- discounts
- order status
- refund promises

If important information is missing, politely ask a clarifying question or say the vendor will confirm.

Do not include or request sensitive payment credentials, passwords, PINs, or OTPs.

Return only the message the vendor can review and send.`
	}

	result, err := generateAIDraft(
		r.Context(),
		systemPrompt,
		requestPrompt,
		1200,
	)

	if err != nil {
		logAIFailure("vendor AI", err)
		p.Notice = aiUserMessage(err)
		h.renderVendorAI(w, p, http.StatusBadGateway)
		return
	}

	p.Result = strings.TrimSpace(result)

	if p.Result == "" {
		p.Notice = "The AI service returned an empty draft. Please try again."
	}

	h.renderVendorAI(w, p, http.StatusOK)
}

func (h *Handlers) vendorAIDataSummary(ctx context.Context, vendorID int) (string, error) {
	var salesKobo, commissionKobo int
	var orders, activeOrders int
	if err := h.DB.QueryRowContext(ctx, `
        SELECT COUNT(*), COALESCE(SUM(total_kobo),0), COALESCE(SUM(commission_kobo),0)
        FROM orders
        WHERE vendor_id=$1 AND status NOT IN ('pending','cancelled')
          AND created_at >= CURRENT_TIMESTAMP - INTERVAL '7 days'
    `, vendorID).Scan(&orders, &salesKobo, &commissionKobo); err != nil {
		return "", err
	}
	if err := h.DB.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM orders
        WHERE vendor_id=$1 AND status IN ('preparing','ready')
    `, vendorID).Scan(&activeOrders); err != nil {
		return "", err
	}

	rows, err := h.DB.QueryContext(ctx, `
        SELECT m.name, SUM(oi.quantity), SUM(oi.quantity * oi.price_kobo)
        FROM order_items oi
        JOIN orders o ON o.id=oi.order_id
        JOIN menu_items m ON m.id=oi.menu_item_id
        WHERE o.vendor_id=$1 AND o.status NOT IN ('pending','cancelled')
          AND o.created_at >= CURRENT_TIMESTAMP - INTERVAL '7 days'
        GROUP BY m.id, m.name
        ORDER BY SUM(oi.quantity) DESC, SUM(oi.quantity * oi.price_kobo) DESC
        LIMIT 5
    `, vendorID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	fmt.Fprintf(&b, "Orders: %d\\nSales: ₦%.2f\\nCommission: ₦%.2f\\nActive orders now: %d\\nTop items:\\n", orders, float64(salesKobo)/100, float64(commissionKobo)/100, activeOrders)
	found := false
	for rows.Next() {
		var name string
		var quantity, itemSales int
		if err := rows.Scan(&name, &quantity, &itemSales); err != nil {
			return "", err
		}
		found = true
		fmt.Fprintf(&b, "- %s: %d sold, ₦%.2f revenue\\n", name, quantity, float64(itemSales)/100)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if !found {
		b.WriteString("- No item-level sales recorded in this period.\\n")
	}
	return b.String(), nil
}

func (h *Handlers) renderVendorAI(
	w http.ResponseWriter,
	p VendorAIPage,
	status int,
) {
	tmpl, err := render.Page("vendor-ai.html")
	if err != nil {
		log.Printf("vendor AI: template failed: %v", err)
		errors.Render(
			w,
			"Could not display the AI writing assistant",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := tmpl.Execute(w, p); err != nil {
		log.Printf("vendor AI: template execution failed: %v", err)
	}
}

func validateVendorAIInput(p VendorAIPage) string {
	switch p.Tool {
	case "description",
		"social",
		"menu_ideas",
		"whatsapp",
		"campaign",
		"review_reply",
		"business_brief",
		"pricing",
		"customer_chat":
	default:
		return "Choose one of the available AI writing tools."
	}

	if p.Tool == "customer_chat" {
		if strings.TrimSpace(p.CustomerMessage) == "" ||
			len([]rune(p.CustomerMessage)) > 1200 {
			return "Paste the customer message (up to 1,200 characters) so the assistant can draft a reply."
		}
	} else if p.Tool != "business_brief" && strings.TrimSpace(p.DishName) == "" {
		return "Enter a dish or product name."
	}

	if len([]rune(p.DishName)) > 100 ||
		len([]rune(p.Details)) > 700 ||
		len([]rune(p.Tone)) > 80 {
		return "Keep the dish/topic to 100 characters, details to 700 characters, and tone to 80 characters."
	}

	return ""
}
