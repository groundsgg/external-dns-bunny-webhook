package bunny

import (
	"net/http"
	"testing"
)

func TestRedactAuthHeaders(t *testing.T) {
	in := http.Header{}
	in.Set("AccessKey", "secret-token")
	in.Set("Authorization", "Bearer secret")
	in.Set("X-Trace-Id", "abc123")

	out := redactAuthHeaders(in)
	if out.Get("AccessKey") != "[REDACTED]" {
		t.Errorf("AccessKey not redacted: %q", out.Get("AccessKey"))
	}
	if out.Get("Authorization") != "[REDACTED]" {
		t.Errorf("Authorization not redacted: %q", out.Get("Authorization"))
	}
	if out.Get("X-Trace-Id") != "abc123" {
		t.Errorf("non-auth header should pass through: %q", out.Get("X-Trace-Id"))
	}
	if in.Get("AccessKey") != "secret-token" {
		t.Error("input header was mutated")
	}
}
