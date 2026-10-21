package render

import (
	"strings"
	"testing"
	"time"

	"github.com/stewartmckee/canvasbin/internal/store"
)

const blk = "<section id=\"canvasbin-updates\"></section>"

func TestWithUpdatesPlacement(t *testing.T) {
	cases := []struct {
		name, doc, before, after string
	}{
		{"hero class", `<body><nav>n</nav><section class="page-hero"><div><h1>T</h1></div></section><main>m</main></body>`, `</section>`, `<main>`},
		{"header with h1", `<body><header><h1>T</h1><p>l</p></header><main>m</main></body>`, `</header>`, `<main>`},
		{"h1 and lede", `<body><header><nav>n</nav></header><h1>T</h1>
<!-- x --> <p>lede</p><p>more</p></body>`, `<p>lede</p>`, `<p>more</p>`},
		{"h1 without lede", `<body><h1>T</h1><ul><li>a</li></ul></body>`, `</h1>`, `<ul>`},
		{"header only", `<body><header>h</header><main>m</main></body>`, `</header>`, `<main>`},
		{"no hero", `<body><main>m</main></body>`, `<body>`, `<main>`},
		{"marker", `<body><h1>T</h1><!-- canvasbin-updates --></body>`, `</h1>`, `</body>`},
		{"nested same tag", `<body><div class="hero"><div>a</div><div>b</div></div><p>after</p></body>`, `<div>b</div></div>`, `<p>after</p>`},
	}
	for _, c := range cases {
		got := WithUpdates(c.doc, blk)
		i := strings.Index(got, blk)
		if i < 0 || !strings.Contains(got[:i], c.before) || !strings.Contains(got[i:], c.after) {
			t.Errorf("%s: block misplaced:\n%s", c.name, got)
		}
		if strings.Count(got, blk) != 1 {
			t.Errorf("%s: block inserted %d times", c.name, strings.Count(got, blk))
		}
	}
}

func TestWithUpdatesNoBody(t *testing.T) {
	got := WithUpdates("plain", blk)
	if !strings.Contains(got, blk) {
		t.Fatalf("block missing: %q", got)
	}
}

func TestUpdatesBlockFoldsOlder(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	out, err := UpdatesBlock("k", []store.PageUpdate{
		{ID: 2, Title: "New", Body: "b", CreatedAt: at, PublishedAt: at},
		{ID: 1, Title: "Old", Body: "a", CreatedAt: at, PublishedAt: at},
	}, "Canvasbin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "21 Oct 2026") {
		t.Error("date not formatted")
	}
	d := strings.Index(out, "<details")
	if d < 0 || strings.Index(out, `id="update-2"`) > d || strings.Index(out, `id="update-1"`) < d {
		t.Error("newest update should be shown and older ones folded")
	}
}
