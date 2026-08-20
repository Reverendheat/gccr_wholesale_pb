package routes

import (
	"strings"
	"testing"
)

func TestNormalizePlacementNote(t *testing.T) {
	got, err := normalizePlacementNote("  Phone order  ")
	if err != nil || got != "Phone order" {
		t.Fatalf("got %q, err %v", got, err)
	}
	if _, err := normalizePlacementNote("   "); err == nil {
		t.Fatal("expected empty note error")
	}
	if _, err := normalizePlacementNote(strings.Repeat("x", 501)); err == nil {
		t.Fatal("expected long note error")
	}
}
