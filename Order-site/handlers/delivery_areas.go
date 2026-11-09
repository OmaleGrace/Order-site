package handlers

import (
	"fmt"
	"strings"
)

// canonicalState returns the canonical Nigerian state name for a case-insensitive input.
func canonicalState(input string) (string, bool) {
	input = strings.TrimSpace(input)
	for _, state := range nigerianStates {
		if strings.EqualFold(input, state) || (state == "FCT (Abuja)" && strings.EqualFold(input, "Abuja")) {
			return state, true
		}
	}
	return "", false
}

// parseDeliveryAreas reads one "State | LGA" pair per line, validates it, and
// removes duplicate state/LGA pairs without treating different LGAs as the same.
func parseDeliveryAreas(raw string) ([][2]string, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	areas := make([][2]string, 0, len(lines))
	seen := make(map[string]bool)
	for lineNo, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: use State | LGA", lineNo+1)
		}
		state, ok := canonicalState(parts[0])
		if !ok {
			return nil, fmt.Errorf("line %d: enter a valid Nigerian state", lineNo+1)
		}
		lga := strings.TrimSpace(parts[1])
		if n := len([]rune(lga)); n < 3 || n > 60 {
			return nil, fmt.Errorf("line %d: LGA must be between 3 and 60 characters", lineNo+1)
		}
		key := strings.ToLower(state) + "|" + strings.ToLower(lga)
		if seen[key] {
			continue
		}
		seen[key] = true
		areas = append(areas, [2]string{state, lga})
		if len(areas) > 100 {
			return nil, fmt.Errorf("you can add at most 100 delivery areas")
		}
	}
	return areas, nil
}
