package validator

import (
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com", false},
		{"valid http", "http://example.com/path?q=1", false},
		{"valid with port", "https://example.com:8080/path", false},
		{"valid with fragment", "https://example.com/page#section", false},
		{"empty string", "", true},
		{"no scheme", "example.com", true},
		{"ftp scheme", "ftp://example.com", true},
		{"no host", "https://", true},
		{"just scheme", "http://", true},
		{"too long", "https://example.com/" + strings.Repeat("a", 2048), true},
		{"spaces in url", "https://exam ple.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{"valid lowercase", "my-link", false},
		{"valid alphanumeric", "abc123", false},
		{"valid with hyphens", "my-cool-link", false},
		{"valid 3 chars", "abc", false},
		{"valid 30 chars", strings.Repeat("a", 30), false},
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", 31), true},
		{"underscore not allowed", "my_link", true},
		{"special chars", "my-link!", true},
		{"spaces", "my link", true},
		{"reserved shorten", "shorten", true},
		{"reserved static", "static", true},
		{"reserved health", "health", true},
		{"reserved api", "api", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAlias(tt.alias)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAlias(%q) error = %v, wantErr %v", tt.alias, err, tt.wantErr)
			}
		})
	}
}
