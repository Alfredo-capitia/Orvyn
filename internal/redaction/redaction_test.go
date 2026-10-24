package redaction

import (
	"strings"
	"testing"
)

func TestRedactAWSKey(t *testing.T) {
	s := "key=AKIAIOSFODNN7EXAMPLE"
	out := Redact(s)
	if strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("aws key not redacted: %s", out)
	}
}

func TestRedactPassword(t *testing.T) {
	s := "password=supersecret123"
	out := Redact(s)
	if strings.Contains(out, "supersecret123") {
		t.Fatalf("password not redacted: %s", out)
	}
}

func TestRedactPrivateKey(t *testing.T) {
	s := "-----BEGIN RSA PRIVATE KEY-----\nABC\n-----END RSA PRIVATE KEY-----"
	out := Redact(s)
	if strings.Contains(out, "BEGIN RSA PRIVATE KEY") && !strings.Contains(out, "REDACTED") {
		t.Fatalf("private key not redacted: %s", out)
	}
}
