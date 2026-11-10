package oauth

import (
	"strings"
	"testing"
)

func TestBuildClientRedirect(t *testing.T) {
	u, err := buildClientRedirect("https://claude.ai/callback", "https://canvasbin.dev", "st", map[string]string{
		"code": "abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"code=abc", "state=st", "iss=https"} {
		if !strings.Contains(u, part) {
			t.Fatalf("url %q missing %q", u, part)
		}
	}
}
