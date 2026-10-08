package secrets

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

const redacted = "[REDACTED]"

type classifiedPattern struct {
	category string
	re       *regexp.Regexp
}

type Redactor struct {
	patterns      []classifiedPattern
	credentialURL *regexp.Regexp
	secretFields  map[string]struct{}
}

func NewRedactor() Redactor {
	return Redactor{
		patterns: []classifiedPattern{
			{category: "private_key", re: regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)},
			{category: "openai_key", re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)},
			{category: "github_token", re: regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{20,}\b`)},
			{category: "authorization", re: regexp.MustCompile(`(?i)\bBearer[ \t]+[A-Za-z0-9._~+/=-]{16,}`)},
		},
		credentialURL: regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/@\s:]+:[^/@\s]+@`),
		secretFields: map[string]struct{}{
			"api_key": {}, "apikey": {}, "token": {}, "access_token": {},
			"auth_token": {}, "authorization": {}, "password": {}, "secret": {},
			"client_secret": {}, "private_key": {}, "github_token": {},
		},
	}
}

func (r Redactor) ScrubText(input string) string {
	result := r.credentialURL.ReplaceAllString(input, `${1}`+redacted+`@`)
	for _, pattern := range r.patterns {
		result = pattern.re.ReplaceAllString(result, redacted)
	}
	return result
}

func (r Redactor) RedactField(path string, value any) model.SafeValue {
	display := scalarDisplay(value)
	safe := model.SafeValue{
		Kind:    scalarKind(value),
		Display: display,
		Present: value != nil,
	}
	if value == nil {
		return safe
	}
	if r.isSecretField(path) {
		safe.Display = redacted
		safe.SecretCategory = "credential_field"
		return safe
	}
	for _, pattern := range r.patterns {
		if pattern.re.MatchString(display) {
			safe.Display = redacted
			safe.SecretCategory = pattern.category
			return safe
		}
	}
	if r.credentialURL.MatchString(display) {
		safe.Display = r.credentialURL.ReplaceAllString(display, `${1}`+redacted+`@`)
		safe.SecretCategory = "url_credentials"
		return safe
	}
	if looksHighEntropy(display) {
		safe.Display = redacted
		safe.SecretCategory = "high_entropy_literal"
	}
	return safe
}

func (r Redactor) RedactTree(value any, fieldPath string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = r.RedactTree(child, joinPath(fieldPath, key))
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = r.RedactTree(child, fmt.Sprintf("%s[%d]", fieldPath, index))
		}
		return result
	default:
		return r.RedactField(fieldPath, value)
	}
}

func (r Redactor) isSecretField(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	if index := strings.LastIndex(path, "."); index >= 0 {
		path = path[index+1:]
	}
	path = strings.TrimSuffix(path, "]")
	if index := strings.LastIndex(path, "["); index >= 0 {
		path = path[:index]
	}
	_, ok := r.secretFields[path]
	return ok
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func scalarDisplay(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func scalarKind(value any) string {
	if value == nil {
		return "null"
	}
	kind := reflect.TypeOf(value).Kind()
	switch kind {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	default:
		return "string"
	}
}

func looksHighEntropy(value string) bool {
	if len(value) < 24 || strings.ContainsAny(value, `/\\ \t\r\n`) {
		return false
	}
	var upper, lower, digit, symbol bool
	for _, char := range value {
		switch {
		case unicode.IsUpper(char):
			upper = true
		case unicode.IsLower(char):
			lower = true
		case unicode.IsDigit(char):
			digit = true
		default:
			symbol = true
		}
	}
	classes := 0
	for _, present := range []bool{upper, lower, digit, symbol} {
		if present {
			classes++
		}
	}
	return classes >= 3
}
