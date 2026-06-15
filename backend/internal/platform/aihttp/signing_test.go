package aihttp

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
)

func TestSignAddsTimestampAndSignature(t *testing.T) {
	body := []byte(`{"task":"demo"}`)
	req, err := http.NewRequest(http.MethodPost, "http://ai-service/tasks/demo", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	Sign(req, body, "secret")

	timestamp := req.Header.Get("X-ZBT-Timestamp")
	if timestamp == "" {
		t.Fatal("expected timestamp header")
	}
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if req.Header.Get("X-ZBT-Signature") != expected {
		t.Fatalf("signature mismatch")
	}
}

func TestSignSkipsEmptySecret(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://ai-service/tasks/demo", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	Sign(req, nil, "")

	if req.Header.Get("X-ZBT-Timestamp") != "" || req.Header.Get("X-ZBT-Signature") != "" {
		t.Fatal("expected no signature headers for empty secret")
	}
}
