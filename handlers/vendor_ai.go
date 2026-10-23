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

	"Order-site/brand"
	"Order-site/errors"
)

const defaultGeminiModel = "gemini-3.8-flash"

var vendorAIHTTPClient = &http.Client{
	Timeout: 35 * time.Second,
}

type VendorAIPage struct {
	VendorName      string
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

type geminiGenerationConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
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
	} `json:"candidates"`

	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
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

func generateGeminiDraft(
	ctx context.Context,
	apiKey string,
	model string,
	systemPrompt string,
	userPrompt string,
	maxOutputTokens int,
) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)

	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not configured")
	}

	if model == "" {
		model = defaultGeminiModel
	}

	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/" +
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

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal Gemini request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create Gemini request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := vendorAIHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read Gemini response: %w", err)
	}

	var decoded geminiResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", fmt.Errorf("decode Gemini response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decoded.Error != nil && decoded.Error.Message != "" {
			return "", fmt.Errorf("Gemini API error: %s", decoded.Error.Message)
		}

		return "", fmt.Errorf("Gemini API returned HTTP %d", resp.StatusCode)
	}

	if len(decoded.Candidates) == 0 {
		return "", fmt.Errorf("Gemini returned no candidates")
	}

	var parts []string

	for _, part := range decoded.Candidates[0].Content.Parts {
		text := strings.TrimSpace(part.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}

	result := strings.TrimSpace(strings.Join(parts, "\n"))

	if result == "" {
		return "", fmt.Errorf("Gemini returned an empty response")
	}

	return result, nil
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
	_, vendorName, ok := h.ownedVendor(w, r)
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

	apiKey := geminiAPIKey()

	if apiKey == "" {
		p.Notice = "AI writing is not configured yet. Add GEMINI_API_KEY to your local .env or hosting environment to enable it."
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

	result, err := generateGeminiDraft(
		r.Context(),
		apiKey,
		geminiModel(),
		systemPrompt,
		requestPrompt,
		350,
	)

	if err != nil {
		log.Printf("vendor AI: Gemini request failed: %v", err)
		p.Notice = "AI generation failed. Check your Gemini API configuration or try again later."
		h.renderVendorAI(w, p, http.StatusBadGateway)
		return
	}

	p.Result = strings.TrimSpace(result)

	if p.Result == "" {
		p.Notice = "The AI service returned an empty draft. Please try again."
	}

	h.renderVendorAI(w, p, http.StatusOK)
}

func (h *Handlers) renderVendorAI(
	w http.ResponseWriter,
	p VendorAIPage,
	status int,
) {
	tmpl, err := brand.ParseE("templates/vendor-ai.html")
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
	} else if strings.TrimSpace(p.DishName) == "" {
		return "Enter a dish or product name."
	}

	if len([]rune(p.DishName)) > 100 ||
		len([]rune(p.Details)) > 700 ||
		len([]rune(p.Tone)) > 80 {
		return "Keep the dish/topic to 100 characters, details to 700 characters, and tone to 80 characters."
	}

	return ""
}
