package validator

import (
	"fmt"
	"net/url"
	"strings"
)

const MaxURLLength = 2048

// Reserved aliases that conflict with server routes.
var reservedAliases = map[string]bool{
	"shorten": true,
	"static":  true,
	"health":  true,
	"api":     true,
}

// ValidateURL checks that rawURL is a valid HTTP or HTTPS URL.
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("url is required")
	}
	if len(rawURL) > MaxURLLength {
		return fmt.Errorf("url exceeds maximum length of %d characters", MaxURLLength)
	}
	if strings.ContainsAny(rawURL, " \t\n\r") {
		return fmt.Errorf("url must not contain whitespace")
	}
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("Invalid url. Url should start from http or https")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("Invalid url. Url should start from http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("Invalid url. Url should start from http or https")
	}
	return nil
}

// ValidateAlias checks that alias is 3-30 alphanumeric/hyphen characters and not reserved.
func ValidateAlias(alias string) error {
	if len(alias) < 3 || len(alias) > 30 {
		return fmt.Errorf("custom alias must be 3-30 characters")
	}
	for _, c := range alias {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-') {
			return fmt.Errorf("custom alias must contain only alphanumeric characters and hyphens")
		}
	}
	if reservedAliases[alias] {
		return fmt.Errorf("alias %q is reserved", alias)
	}
	return nil
}
