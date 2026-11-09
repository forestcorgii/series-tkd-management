package services

import (
	"testing"
)

func TestIsTelegramValidButtonURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"http://localhost:8080/dashboard", false},
		{"http://localhost/sessions", false},
		{"http://127.0.0.1:8080", false},
		{"http://0.0.0.0:8080", false},
		{"http://localonly/path", false},
		{"", false},
		{"   ", false},
		{"invalid://url-scheme", false},
		{"ftp://example.com", false},
		{"https://seriestkd.com/dashboard", true},
		{"https://stms-production.up.railway.app/sessions", true},
		{"http://subdomain.example.com", true},
		{"tg://resolve?domain=seriestkd", true},
	}

	for _, tt := range tests {
		result := isTelegramValidButtonURL(tt.url)
		if result != tt.expected {
			t.Errorf("isTelegramValidButtonURL(%q) = %v; want %v", tt.url, result, tt.expected)
		}
	}
}

func TestSanitizeTelegramButtons(t *testing.T) {
	// Case 1: All buttons are localhost -> should return nil
	allLocalhost := [][]TelegramInlineButton{
		{
			{Text: "Local Portal", URL: "http://localhost:8080/dashboard"},
			{Text: "Local Schedule", URL: "http://127.0.0.1:8080/sessions"},
		},
	}
	res1 := sanitizeTelegramButtons(allLocalhost)
	if len(res1) != 0 {
		t.Errorf("expected empty sanitized buttons for localhost URLs, got %v", res1)
	}

	// Case 2: Mixed buttons -> only valid public URLs kept
	mixed := [][]TelegramInlineButton{
		{
			{Text: "Local", URL: "http://localhost:8080/dashboard"},
			{Text: "Public", URL: "https://seriestkd.com/sessions"},
		},
		{
			{Text: "Another Local", URL: "http://localhost:3000"},
		},
	}
	res2 := sanitizeTelegramButtons(mixed)
	if len(res2) != 1 {
		t.Fatalf("expected 1 row after sanitization, got %d", len(res2))
	}
	if len(res2[0]) != 1 || res2[0][0].Text != "Public" {
		t.Errorf("expected only 'Public' button to remain, got %v", res2[0])
	}
}

func TestSendTelegramMessage_SanitizationWithMock(t *testing.T) {
	buttons := [][]TelegramInlineButton{
		{
			{Text: "Local Portal", URL: "http://localhost:8080/dashboard"},
		},
	}
	// Mock token should succeed and not panic or fail
	err := SendTelegramMessage("mock", "@testchat", "<b>Hello World</b>", buttons)
	if err != nil {
		t.Fatalf("SendTelegramMessage with mock failed: %v", err)
	}
}

