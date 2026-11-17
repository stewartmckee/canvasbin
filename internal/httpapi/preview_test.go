package httpapi

import "testing"

func TestLinkPreviewCrawler(t *testing.T) {
	for _, ua := range []string{
		"WhatsApp/2.23.20.0 A",
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
		"Slackbot-LinkExpanding 1.0",
	} {
		if !linkPreviewCrawler(ua) {
			t.Errorf("%s should be allowed to read the title", ua)
		}
	}
	if linkPreviewCrawler("Mozilla/5.0 (compatible; Googlebot/2.1)") {
		t.Fatal("Googlebot should stay noindex")
	}
}
