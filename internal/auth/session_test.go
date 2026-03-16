package auth

import "testing"

func TestSignAndParseSessionCookie(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	sessionID := "550e8400-e29b-41d4-a716-446655440000"

	cookie := SignSessionCookie(sessionID, secret)
	if cookie == sessionID {
		t.Fatal("cookie should not equal raw session ID")
	}

	parsed, err := ParseSessionCookie(cookie, secret)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if parsed != sessionID {
		t.Fatalf("expected %s, got %s", sessionID, parsed)
	}
}

func TestParseSessionCookieInvalidSignature(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	cookie := "550e8400-e29b-41d4-a716-446655440000.invalidsignature"

	_, err := ParseSessionCookie(cookie, secret)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestParseSessionCookieMalformed(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"

	// No dot separator
	_, err := ParseSessionCookie("noseparator", secret)
	if err == nil {
		t.Fatal("expected error for malformed cookie")
	}
}
