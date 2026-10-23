package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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

const vendorAIEndpointBase = "https://generativelanguage.googleapis.com/v1beta/models/"

var vendorAIHTTPClient = &http.Client{Timeout: 25 * time.Second}

type VendorAIPage struct {
	VendorName string
	Tool       string
	DishName   string
	Details    string
	Tone       string
	Result     string
	Notice     string
}

type vendorAIText struct {
	Text string `json:"text"`
}
type vendorAIContent struct {
	Role  string         `json:"role,omitempty"`
	Parts []vendorAIText `json:"parts"`
}
type vendorAIRequest struct {
	SystemInstruction vendorAIContent   `json:"systemInstruction"`
	Contents          []vendorAIContent `json:"contents"`
	GenerationConfig  struct {
		MaxOutputTokens int `json:"maxOutputTokens"`
	} `json:"generationConfig"`
}
type vendorAIResponse struct {
	Candidates []struct {
		Content vendorAIContent `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// VendorAI shows the optional AI writing assistant to the owner of a shop.
func (h *Handlers) VendorAI(w http.ResponseWriter, r *http.Request) {
	_, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	p := VendorAIPage{VendorName: vendorName, Tool: "description", Tone: "warm and appetising", Notice: r.URL.Query().Get("msg")}
	h.renderVendorAI(w, p, http.StatusOK)
}

// VendorAIGenerate creates draft marketing copy. It never changes a live menu or post automatically.
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
		VendorName: vendorName,
		Tool:       strings.TrimSpace(r.FormValue("tool")),
		DishName:   strings.TrimSpace(r.FormValue("dish_name")),
		Details:    strings.TrimSpace(r.FormValue("details")),
		Tone:       strings.TrimSpace(r.FormValue("tone")),
	}
	if validationMessage := validateVendorAIInput(p); validationMessage != "" {
		p.Notice = validationMessage
		h.renderVendorAI(w, p, http.StatusBadRequest)
		return
	}
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		p.Notice = "AI writing is not configured yet. Add GEMINI_API_KEY to your local .env or hosting environment to enable it."
		h.renderVendorAI(w, p, http.StatusOK)
		return
	}

	systemPrompt := "You are ENIE's practical Nigerian food-business copywriting assistant. Write clear, appetising, honest copy. Do not invent ingredients, health claims, discounts, delivery promises, awards, or availability. Use only facts provided by the vendor. Return only the requested draft, with no preamble."
	requestPrompt := "Shop: " + vendorName + "\nDish/product: " + p.DishName + "\nVendor details: " + p.Details + "\nTone: " + p.Tone + "\n"
	if p.Tool == "description" {
		requestPrompt += "Write one appealing menu description of 2-4 sentences, suitable for a Nigerian food marketplace. Keep it concise and do not add unprovided ingredients."
	} else {
		requestPrompt += "Write one engaging social media promotional caption for this food business, with a clear call to action and 3-5 relevant hashtags. Do not invent prices or offers."
	}
	payload := vendorAIRequest{
		SystemInstruction: vendorAIContent{Parts: []vendorAIText{{Text: systemPrompt}}},
		Contents:          []vendorAIContent{{Role: "user", Parts: []vendorAIText{{Text: requestPrompt}}}},
	}
	// Newer Gemini models spend part of this budget on internal thinking, so keep it generous.
	payload.GenerationConfig.MaxOutputTokens = 1024
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("vendor AI: request encoding failed: %v", err)
		errors.Render(w, "Could not prepare the AI request", http.StatusInternalServerError)
		return
	}

	// Try the chosen model first, then a fallback if it is busy or slow.
	models := vendorAIModels()
	var lastNotice string
	for _, model := range models {
		for attempt := 1; attempt <= 2; attempt++ {
			text, retry, notice := callGemini(r.Context(), apiKey, model, body)
			if text != "" {
				p.Result = text
				h.renderVendorAI(w, p, http.StatusOK)
				return
			}
			lastNotice = notice
			if !retry {
				break // retrying the same model will not help (bad key, bad model name, ...)
			}
			time.Sleep(time.Duration(attempt) * 700 * time.Millisecond)
		}
		if r.Context().Err() != nil {
			break
		}
	}
	p.Notice = lastNotice
	h.renderVendorAI(w, p, http.StatusBadGateway)
}

// vendorAIModels returns the configured model followed by fallback models,
// without duplicates. Set GEMINI_FALLBACK_MODEL to choose the first fallback.
func vendorAIModels() []string {
	fallback := strings.TrimSpace(os.Getenv("GEMINI_FALLBACK_MODEL"))
	if fallback == "" {
		fallback = "gemini-3.5-flash-lite"
	}
	candidates := []string{strings.TrimSpace(os.Getenv("GEMINI_MODEL")), fallback, "gemini-3.6-flash"}
	var out []string
	seen := map[string]bool{}
	for _, m := range candidates {
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// callGemini makes one request. It returns the text on success; otherwise an
// empty text, whether another try may help, and a message safe to show the vendor.
func callGemini(parent context.Context, apiKey, model string, body []byte) (text string, retry bool, notice string) {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, vendorAIEndpointBase+url.PathEscape(model)+":generateContent", bytes.NewReader(body))
	if err != nil {
		return "", false, "Could not prepare the AI request."
	}
	req.Header.Set("x-goog-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := vendorAIHTTPClient.Do(req)
	if err != nil {
		log.Printf("vendor AI: request failed (model %s): %v", model, err)
		return "", true, "The AI service could not be reached. Please try again shortly."
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		log.Printf("vendor AI: reading response failed (model %s): %v", model, err)
		return "", true, "The AI service response could not be read. Please try again."
	}
	var decoded vendorAIResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		log.Printf("vendor AI: invalid response (status %d, model %s): %v: %.300s", resp.StatusCode, model, err, raw)
		return "", true, "The AI service returned an unexpected response. Please try again."
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || len(decoded.Candidates) == 0 {
		msg := string(raw)
		if decoded.Error != nil {
			msg = decoded.Error.Message
		}
		log.Printf("vendor AI: provider rejected request (status %d, model %s): %.300s", resp.StatusCode, model, msg)
		busy := resp.StatusCode == 429 || resp.StatusCode >= 500
		if busy {
			return "", true, "The AI service is busy right now. Please try again in a minute."
		}
		return "", false, "AI generation failed. Check the AI API configuration or try again later."
	}
	var sb strings.Builder
	for _, part := range decoded.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", true, "The AI service returned an empty draft. Please try again."
	}
	return out, false, ""
}

func (h *Handlers) renderVendorAI(w http.ResponseWriter, p VendorAIPage, status int) {
	tmpl, err := brand.ParseE("templates/vendor-ai.html")
	if err != nil {
		log.Printf("vendor AI: template failed: %v", err)
		errors.Render(w, "Could not display the AI writing assistant", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.Execute(w, p); err != nil {
		log.Printf("vendor AI: template execution failed: %v", err)
	}
}

func validateVendorAIInput(p VendorAIPage) string {
	if p.Tool != "description" && p.Tool != "social" {
		return "Choose a description or promotional post."
	}
	if strings.TrimSpace(p.DishName) == "" || len([]rune(p.DishName)) > 100 || len([]rune(p.Details)) > 700 || len([]rune(p.Tone)) > 80 {
		return "Enter a dish or product name (up to 100 characters), with details up to 700 characters."
	}
	return ""
}