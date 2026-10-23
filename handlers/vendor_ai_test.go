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
