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
