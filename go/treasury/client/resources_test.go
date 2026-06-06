package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cordialsys/sdk-go/treasury/types"
)

func TestCreateAccountRejectsInvalidVariant(t *testing.T) {
	c, err := NewClient("test", WithBaseURL("http://example.test"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.CreateAccount("acct", CreateAccountRequest{
		Variant: types.AccountVariant("unknown"),
	})
	if err == nil {
		t.Fatal("expected invalid variant error")
	}
}

func TestCreateAddressUsesTypedPayloadAndNestedPath(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`"operations/op-1"`))
	}))
	defer srv.Close()

	identity, err := GenerateEd25519Identity("tester")
	if err != nil {
		t.Fatalf("GenerateEd25519Identity: %v", err)
	}

	c, err := NewClient("test", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetIdentity(identity)

	account := types.AccountName("accounts/main")
	op, err := c.CreateAddress("SOL", "addr-1", CreateAddressRequest{
		AddressData: types.AddressData{
			Account: &account,
		},
		Variant: types.AddressVariantInternal,
	})
	if err != nil {
		t.Fatalf("CreateAddress: %v", err)
	}
	if op != "operations/op-1" {
		t.Fatalf("operation = %q, want operations/op-1", op)
	}
	if gotPath != "/v1/chains/SOL/addresses/addr-1" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["variant"] != "internal" {
		t.Fatalf("variant = %v", gotBody["variant"])
	}
	if gotBody["account"] != "accounts/main" {
		t.Fatalf("account = %v", gotBody["account"])
	}
}

func TestCreateTransferRuleRejectsInvalidVariant(t *testing.T) {
	c, err := NewClient("test", WithBaseURL("http://example.test"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.CreateTransferRule("rule", CreateTransferRuleRequest{
		Variant: types.RuleVariant("maybe"),
	})
	if err == nil {
		t.Fatal("expected invalid variant error")
	}
}

func TestCreateTransferUsesGeneratedUnionPayload(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`"operations/op-transfer"`))
	}))
	defer srv.Close()

	identity, err := GenerateEd25519Identity("tester")
	if err != nil {
		t.Fatalf("GenerateEd25519Identity: %v", err)
	}

	var from types.TransferCreateData_From
	if err := from.FromAddressChoice("chains/SOL/addresses/source"); err != nil {
		t.Fatalf("FromAddressChoice: %v", err)
	}
	var to types.TransferCreateData_To
	if err := to.FromAddressChoice("chains/SOL/addresses/dest"); err != nil {
		t.Fatalf("FromAddressChoice(to): %v", err)
	}

	c, err := NewClient("test", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetIdentity(identity)

	op, err := c.CreateTransfer("tx-1", CreateTransferRequest{
		TransferCreateData: types.TransferCreateData{
			Asset: "chains/SOL/assets/SOL",
			From:  from,
			To:    to,
		},
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}
	if op != "operations/op-transfer" {
		t.Fatalf("operation = %q, want operations/op-transfer", op)
	}
	if gotPath != "/v1/transfers/tx-1" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["asset"] != "chains/SOL/assets/SOL" {
		t.Fatalf("asset = %v", gotBody["asset"])
	}
	if gotBody["from"] != "chains/SOL/addresses/source" {
		t.Fatalf("from = %v", gotBody["from"])
	}
	if gotBody["to"] != "chains/SOL/addresses/dest" {
		t.Fatalf("to = %v", gotBody["to"])
	}
}
