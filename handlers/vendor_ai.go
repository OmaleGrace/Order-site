package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"Order-site/brand"
	"Order-site/errors"
)

const vendorAIEndpoint = "https://api.openai.com/v1/chat/completions"

var vendorAIHTTPClient = &http.Client{Timeout: 35 * time.Second}

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

type vendorAIRequest struct {
	Model     string            `json:"model"`
	Messages  []vendorAIMessage `json:"messages"`
	MaxTokens int               `json:"max_tokens"`
}
type vendorAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type vendorAIResponse struct {
	Choices []struct {
		Message vendorAIMessage `json:"message"`
	} `json:"choices"`
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
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		p.Notice = "AI writing is not configured yet. Add OPENAI_API_KEY to your local .env or hosting environment to enable it."
		h.renderVendorAI(w, p, http.StatusOK)
		return
	}

	systemPrompt := "You are ENIE's practical Nigerian food-business copywriting assistant. Write clear, appetising, honest copy. Do not invent ingredients, health claims, discounts, delivery promises, awards, or availability. Use only facts provided by the vendor. Return only the requested draft, with no preamble."
	requestPrompt := "Shop: " + vendorName + "\nDish/product or topic: " + p.DishName + "\nVendor details: " + p.Details + "\nTone: " + p.Tone + "\n"
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
		requestPrompt += "You are helping the shop owner draft a reply to a customer chat message. Customer message: \"" + p.CustomerMessage + "\"\nDraft a warm, concise, helpful response that sounds natural for a Nigerian food business. Answer only using the vendor details provided. Never invent prices, stock, ingredients, opening hours, delivery times, discounts, order status, or refund promises. If important information is missing, politely ask a clarifying question or say the vendor will confirm. Do not include or request sensitive payment credentials, passwords, PINs, or OTPs. Return only the message the vendor can review and send."
	}
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-4o-mini"
	}
	payload := vendorAIRequest{Model: model, MaxTokens: 350, Messages: []vendorAIMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: requestPrompt}}}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("vendor AI: request encoding failed: %v", err)
		errors.Render(w, "Could not prepare the AI request", http.StatusInternalServerError)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, vendorAIEndpoint, bytes.NewReader(body))
	if err != nil {
		errors.Render(w, "Could not prepare the AI request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := vendorAIHTTPClient.Do(req)
	if err != nil {
		log.Printf("vendor AI: request failed: %v", err)
		p.Notice = "The AI service could not be reached. Please try again shortly."
		h.renderVendorAI(w, p, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	limited, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		p.Notice = "The AI service response could not be read. Please try again."
		h.renderVendorAI(w, p, http.StatusBadGateway)
		return
	}
	var decoded vendorAIResponse
	if err := json.Unmarshal(limited, &decoded); err != nil {
		log.Printf("vendor AI: invalid response (status %d): %v", resp.StatusCode, err)
		p.Notice = "The AI service returned an unexpected response. Please try again."
		h.renderVendorAI(w, p, http.StatusBadGateway)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || len(decoded.Choices) == 0 {
		if decoded.Error != nil {
			log.Printf("vendor AI: provider rejected request: %s", decoded.Error.Message)
		}
		p.Notice = "AI generation failed. Check the AI API configuration or try again later."
		h.renderVendorAI(w, p, http.StatusBadGateway)
		return
	}
	p.Result = strings.TrimSpace(decoded.Choices[0].Message.Content)
	if p.Result == "" {
		p.Notice = "The AI service returned an empty draft. Please try again."
	}
	h.renderVendorAI(w, p, http.StatusOK)
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
	switch p.Tool {
	case "description", "social", "menu_ideas", "whatsapp", "campaign", "review_reply", "pricing", "customer_chat":
	default:
		return "Choose one of the available AI writing tools."
	}
	if p.Tool == "customer_chat" {
		if strings.TrimSpace(p.CustomerMessage) == "" || len([]rune(p.CustomerMessage)) > 1200 {
			return "Paste the customer message (up to 1,200 characters) so the assistant can draft a reply."
		}
	} else if strings.TrimSpace(p.DishName) == "" {
		return "Enter a dish or product name."
	}
	if len([]rune(p.DishName)) > 100 || len([]rune(p.Details)) > 700 || len([]rune(p.Tone)) > 80 {
		return "Keep the dish/topic to 100 characters, details to 700 characters, and tone to 80 characters."
	}
	return ""
}
