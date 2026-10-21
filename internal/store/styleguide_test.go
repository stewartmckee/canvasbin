package store

import (
	"errors"
	"strings"
	"testing"
)

func TestCleanStyleGuide(t *testing.T) {
	got, err := cleanStyleGuide("  Short sentences.\r\nBritish English.  ")
	if err != nil || got != "Short sentences.\nBritish English." {
		t.Fatalf("clean = %q %v", got, err)
	}
	if got, err := cleanStyleGuide("   \n  "); err != nil || got != "" {
		t.Fatalf("blank = %q %v", got, err)
	}
	if _, err := cleanStyleGuide(strings.Repeat("å", MaxStyleGuideRunes+1)); !errors.Is(err, ErrStyleGuideTooLong) {
		t.Fatalf("too long: %v", err)
	}
	if _, err := cleanStyleGuide(strings.Repeat("a", MaxStyleGuideRunes)); err != nil {
		t.Fatalf("at the limit: %v", err)
	}
}
