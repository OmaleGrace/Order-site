package handlers

import (
	"strings"
	"testing"
)

func TestValidateVendorAIInput(t *testing.T) {
	cases := []struct {
		name      string
		page      VendorAIPage
		wantError bool
	}{
		{"description valid", VendorAIPage{Tool: "description", DishName: "Jollof rice", Details: "Smoky party-style rice"}, false},
		{"social valid", VendorAIPage{Tool: "social", DishName: "Meat pie"}, false},
		{"menu ideas valid", VendorAIPage{Tool: "menu_ideas", DishName: "Ofada rice"}, false},
		{"WhatsApp valid", VendorAIPage{Tool: "whatsapp", DishName: "Puff puff"}, false},
		{"campaign valid", VendorAIPage{Tool: "campaign", DishName: "Small chops"}, false},
		{"review reply valid", VendorAIPage{Tool: "review_reply", DishName: "Customer feedback"}, false},
		{"pricing worksheet valid", VendorAIPage{Tool: "pricing", DishName: "Meat pie"}, false},
		{"customer chat valid", VendorAIPage{Tool: "customer_chat", CustomerMessage: "Do you deliver to Ikeja?"}, false},
		{"customer chat missing message", VendorAIPage{Tool: "customer_chat"}, true},
		{"customer chat message too long", VendorAIPage{Tool: "customer_chat", CustomerMessage: strings.Repeat("x", 1201)}, true},
		{"unsupported tool", VendorAIPage{Tool: "system", DishName: "Rice"}, true},
		{"empty name", VendorAIPage{Tool: "description", DishName: "   "}, true},
		{"name too long", VendorAIPage{Tool: "description", DishName: strings.Repeat("x", 101)}, true},
		{"details too long", VendorAIPage{Tool: "social", DishName: "Rice", Details: strings.Repeat("x", 701)}, true},
		{"tone too long", VendorAIPage{Tool: "social", DishName: "Rice", Tone: strings.Repeat("x", 81)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateVendorAIInput(tc.page)
			if (got != "") != tc.wantError {
				t.Fatalf("validateVendorAIInput() error = %q, wantError %v", got, tc.wantError)
			}
		})
	}
}

func TestNigerianAccountNumberValidation(t *testing.T) {
	valid := []string{"0123456789", "1234567890"}
	for _, value := range valid {
		if !nigerianAccountNumber.MatchString(value) {
			t.Errorf("expected %q to be valid", value)
		}
	}
	invalid := []string{"123456789", "12345678901", "12345abc90", "123 456789"}
	for _, value := range invalid {
		if nigerianAccountNumber.MatchString(value) {
			t.Errorf("expected %q to be invalid", value)
		}
	}
}
