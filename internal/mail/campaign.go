package mail

import (
	"net/url"
	"strings"
)

// Transactional email campaigns (utm_campaign). All use utm_source=email and
// utm_medium=transactional; utm_content identifies the link target.
const (
	campaignVerifySignup          = "verify_signup"
	campaignVerifyNudge           = "verify_nudge"
	campaignWelcome               = "welcome"
	campaignPasswordReset         = "password_reset"
	campaignPageShare             = "page_share"
	campaignTeamInvite            = "team_invite"
	campaignUpdatesSubscribe      = "updates_subscribe"
	campaignUpdatesNotification   = "updates_notification"
)

// withEmailUTM adds standard transactional campaign query params for Umami/GA.
// Existing utm_* on the URL are left unchanged.
func withEmailUTM(rawURL, campaign, content string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	if q.Get("utm_source") != "" {
		return rawURL
	}
	q.Set("utm_source", "email")
	q.Set("utm_medium", "transactional")
	q.Set("utm_campaign", campaign)
	if content != "" {
		q.Set("utm_content", content)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func campaignPath(base, path, campaign, content string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if content == "" {
		content = utmContentFromPath(path)
	}
	return withEmailUTM(base+path, campaign, content)
}

func utmContentFromPath(path string) string {
	path = strings.Trim(strings.TrimPrefix(path, "/"), "/")
	if path == "" {
		return "home"
	}
	return strings.NewReplacer("/", "_", "-", "_").Replace(path)
}

func utmContentFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "link"
	}
	return utmContentFromPath(u.Path)
}

func tagButtons(btns []emailButton, campaign string) []emailButton {
	out := make([]emailButton, len(btns))
	for i, b := range btns {
		out[i] = emailButton{
			Label: b.Label,
			URL:   withEmailUTM(b.URL, campaign, utmContentFromURL(b.URL)),
		}
	}
	return out
}

func absButtonsCampaign(base string, btns []emailButton, campaign string) []emailButton {
	return tagButtons(absButtons(base, btns), campaign)
}
