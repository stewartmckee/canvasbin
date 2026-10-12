package keys

import "testing"

func TestReadKeyIsDerivedAndDistinct(t *testing.T) {
	w, err := NewWrite()
	if err != nil {
		t.Fatal(err)
	}
	r := ReadFromWrite(w)
	if len(w) != Length || len(r) != Length {
		t.Fatalf("unexpected lengths %d %d", len(w), len(r))
	}
	if r == w || ReadFromWrite(w) != r {
		t.Fatal("read key must be stable and differ from write key")
	}
}

func TestParse(t *testing.T) {
	w, _ := NewWrite()
	for _, in := range []string{
		w,
		"  " + w + "\n",
		"https://canvasbin.mckee.family/e/" + w,
		"https://canvasbin.mckee.family/" + w,
		"https://canvasbin.mckee.family/" + w + ".html",
		"https://canvasbin.mckee.family/" + w + ".md",
		w + ".html",
		"https://canvasbin.mckee.family/v/" + w + "/",
		"/raw/" + w,
	} {
		got, err := Parse(in)
		if err != nil || got != w {
			t.Errorf("Parse(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "short", w + "x", "https://x/e/" + w[:40] + "!!!"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}
