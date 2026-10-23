package handlers

import "strings"

var nigerianStates = []string{
	"Abia", "Adamawa", "Akwa Ibom", "Anambra", "Bauchi", "Bayelsa", "Benue", "Borno",
	"Cross River", "Delta", "Ebonyi", "Edo", "Ekiti", "Enugu", "FCT (Abuja)", "Gombe",
	"Imo", "Jigawa", "Kaduna", "Kano", "Katsina", "Kebbi", "Kogi", "Kwara", "Lagos",
	"Nasarawa", "Niger", "Ogun", "Ondo", "Osun", "Oyo", "Plateau", "Rivers", "Sokoto",
	"Taraba", "Yobe", "Zamfara",
}

func validState(s string) bool {
	_, ok := canonicalState(s)
	return ok
}

// normalizeNigerianPhone accepts 0XXXXXXXXXX, 234XXXXXXXXXX and +234XXXXXXXXXX
// (spaces and dashes allowed) and returns the 11-digit local form, or "" if the
// number is not a valid Nigerian mobile number.
func normalizeNigerianPhone(raw string) string {
	var b strings.Builder
	for i, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
		case r == ' ' || r == '-':
		default:
			return ""
		}
	}

	d := b.String()
	switch {
	case len(d) == 13 && strings.HasPrefix(d, "234"):
		d = "0" + d[3:]
	case len(d) == 11 && d[0] == '0':
	default:
		return ""
	}

	if d[1] != '7' && d[1] != '8' && d[1] != '9' {
		return ""
	}
	return d
}
