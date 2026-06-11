package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientDefaultBaseURLRequiresAPIKey(t *testing.T) {
	if _, err := NewClient("treasury-id"); err == nil {
		t.Fatal("expected missing api key error")
	}
}

func TestNewClientAcceptsDefaultBaseURLWithAPIKey(t *testing.T) {
	c, err := NewClient("treasury-id", WithAPIKey("raw-key"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL != "https://treasury.cordialapis.com" {
		t.Fatalf("BaseURL = %q", c.BaseURL)
	}
	if c.APIKey != "cmF3LWtleQ==" {
		t.Fatalf("APIKey = %q, want base64-encoded raw key", c.APIKey)
	}
}

func TestGetJSONAppliesTreasuryAndAuthorizationHeaders(t *testing.T) {
	var gotTreasury string
	var gotAuthorization string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTreasury = r.Header.Get("Treasury")
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c, err := NewClient("treasury-id", WithBaseURL(srv.URL), WithAPIKey("raw-key"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.GetJSON("treasury"); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if gotTreasury != "treasury-id" {
		t.Fatalf("Treasury header = %q", gotTreasury)
	}
	if gotAuthorization != "Bearer cmF3LWtleQ==" {
		t.Fatalf("Authorization header = %q", gotAuthorization)
	}
}

func TestExecuteAppliesTreasuryAuthorizationAndSignatureHeaders(t *testing.T) {
	var gotTreasury string
	var gotAuthorization string
	var gotSignature string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTreasury = r.Header.Get("Treasury")
		gotAuthorization = r.Header.Get("Authorization")
		gotSignature = r.Header.Get("Signature")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`"operations/op-1"`))
	}))
	defer srv.Close()

	identity, err := GenerateEd25519Identity("tester")
	if err != nil {
		t.Fatalf("GenerateEd25519Identity: %v", err)
	}
	c, err := NewClient("treasury-id", WithBaseURL(srv.URL), WithAPIKey("raw-key"), WithSigner(identity))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	op, err := c.Execute(http.MethodPost, "/v1/accounts", map[string]string{"variant": "internal"}, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if op != "operations/op-1" {
		t.Fatalf("operation = %q", op)
	}
	if gotTreasury != "treasury-id" {
		t.Fatalf("Treasury header = %q", gotTreasury)
	}
	if gotAuthorization != "Bearer cmF3LWtleQ==" {
		t.Fatalf("Authorization header = %q", gotAuthorization)
	}
	if gotSignature == "" {
		t.Fatal("expected Signature header")
	}
}
