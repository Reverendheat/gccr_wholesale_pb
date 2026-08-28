package square

import (
	"testing"
)

func TestNew_SandboxURL(t *testing.T) {
	c := New(Config{AccessToken: "tok", Sandbox: true})
	if c.SDK == nil {
		t.Fatal("expected SDK client to be non-nil")
	}
}

func TestNew_ProductionURL(t *testing.T) {
	c := New(Config{AccessToken: "tok", Sandbox: false})
	if c.SDK == nil {
		t.Fatal("expected SDK client to be non-nil")
	}
}

func TestNew_ConfiguresWholesaleCatalog(t *testing.T) {
	c := New(Config{
		AccessToken:                  "tok",
		WholesaleCategoryID:          "category-id",
		WholesaleGrindModifierListID: "grind-list-id",
		WholesaleDripModifierID:      "drip-id",
	})
	if c.WholesaleCategoryID != "category-id" {
		t.Fatalf("expected configured wholesale category ID, got %q", c.WholesaleCategoryID)
	}
	if c.WholesaleGrindModifierListID != "grind-list-id" {
		t.Fatalf("expected configured grind modifier list ID, got %q", c.WholesaleGrindModifierListID)
	}
	if c.WholesaleDripModifierID != "drip-id" {
		t.Fatalf("expected configured drip modifier ID, got %q", c.WholesaleDripModifierID)
	}
}
