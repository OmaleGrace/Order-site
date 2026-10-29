package brand

import (
	"html/template"
	"os"
	"regexp"
	"strings"
)

// Name returns the business name for this copy of the app.
func Name() string {
	if n := strings.TrimSpace(os.Getenv("BUSINESS_NAME")); n != "" {
		// Keep an old deployment environment value from silently restoring the
		// previous brand after the codebase has been rebranded.
		legacy := strings.EqualFold(n, "Grace's Kitchen") || strings.EqualFold(n, "Grace’s Kitchen") || strings.EqualFold(n, "Grace Ordering Site")
		if !legacy {
			return n
		}
	}
	return "ENIE"
}

var adsensePublisherPattern = regexp.MustCompile(`^pub-[0-9]{10,20}$`)

// AdSensePublisherID returns a validated publisher ID, or an empty string
// until the owner configures a real ID in the deployment environment.
func AdSensePublisherID() string {
	id := strings.TrimSpace(os.Getenv("ADSENSE_PUBLISHER_ID"))
	if !adsensePublisherPattern.MatchString(id) {
		return ""
	}
	return id
}

// SiteURL returns the configured public base URL without a trailing slash.
func SiteURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
}

// Funcs makes shared site configuration available inside templates.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"brandName":          Name,
		"adsensePublisherID": AdSensePublisherID,
		"siteURL":            SiteURL,
	}
}
