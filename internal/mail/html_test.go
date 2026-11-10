package mail

import (
	"strings"
	"testing"
)

func TestSecondaryButtonRows(t *testing.T) {
	btns := []emailButton{{Label: "a"}, {Label: "b"}, {Label: "c"}, {Label: "d"}, {Label: "e"}}
	rows := secondaryButtonRows(btns, 3)
	if len(rows) != 2 || len(rows[0]) != 3 || len(rows[1]) != 3 {
		t.Fatalf("rows: %+v", rows)
	}
	if rows[1][2].Label != "" {
		t.Fatalf("expected padded empty cell")
	}
}

func TestSecondaryButtonsThreePerRowInHTML(t *testing.T) {
	html, err := htmlEmail{
		Heading: "Test",
		Base:    "https://canvasbin.test",
		Secondary: []emailButton{
			{Label: "One", URL: "https://canvasbin.test/1"},
			{Label: "Two", URL: "https://canvasbin.test/2"},
			{Label: "Three", URL: "https://canvasbin.test/3"},
			{Label: "Four", URL: "https://canvasbin.test/4"},
			{Label: "Five", URL: "https://canvasbin.test/5"},
		},
	}.render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(html, "display:inline-block;width:32%") != 5 {
		t.Fatalf("expected 5 inline-block cells at 32%% width, got %d", strings.Count(html, "display:inline-block;width:32%"))
	}
	msoRows := strings.Count(html, "<tr>")
	if msoRows < 2 {
		t.Fatalf("expected MSO table rows for 5 buttons, tr count %d", msoRows)
	}
}
