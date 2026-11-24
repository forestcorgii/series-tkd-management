package services

import (
	"errors"
	"strings"
	"testing"
)

func TestTelegramTokenSanitizationAndRedaction(t *testing.T) {
	if err := sanitizeTelegramToken("123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"); err != nil {
		t.Fatalf("expected valid token to pass, got %v", err)
	}

	for _, bad := range []string{"123/sendMessage", "123?foo=bar", "123#frag", "123 456"} {
		if err := sanitizeTelegramToken(bad); err == nil {
			t.Errorf("expected error for malformed token %q", bad)
		}
	}

	token := "999999:SECRET_TOKEN_VALUE"
	redacted := redactTelegramToken(errors.New("Post https://api.telegram.org/bot"+token+"/getMe: dial tcp"), token)
	if strings.Contains(redacted.Error(), token) {
		t.Fatalf("expected token to be redacted, got %q", redacted.Error())
	}
	if !strings.Contains(redacted.Error(), "[REDACTED]") {
		t.Fatalf("expected [REDACTED] placeholder, got %q", redacted.Error())
	}
}
