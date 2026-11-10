// Package mail sends transactional email through the Mailgun EU HTTP API.
package mail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.eu.mailgun.net"

// Sender sends messages. Server code depends on this so tests can fake it.
type Sender interface {
	SendVerify(ctx context.Context, email, link string) error
	SendReset(ctx context.Context, email, link string) error
	SendInvite(ctx context.Context, email, link, inviter string) error
	SendTeamInvite(ctx context.Context, email, link, inviter, team string) error
	SendWelcome(ctx context.Context, email, base string) error
	SendVerifyNudge(ctx context.Context, email, link, base string) error
	SendUpdateSubscribeConfirm(ctx context.Context, email, link, pageTitle, product string) error
	SendUpdateNotification(ctx context.Context, email, product, pageTitle, updateTitle, summary, viewURL, unsubURL string) error
}

// Client is a Mailgun client. The API key is never logged or included in errors.
type Client struct {
	Key     string
	Domain  string
	From    string
	BaseURL string // override for tests; defaults to the Mailgun EU endpoint
	HTTP    *http.Client
}

func New(key, domain, from string) *Client {
	return &Client{
		Key: key, Domain: domain, From: from,
		BaseURL: defaultBaseURL,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) send(ctx context.Context, to, subject, text string) error {
	return c.sendHTML(ctx, to, subject, text, "")
}

// sendHTML sends a multipart message; text is the plain-text alternative and
// html may be empty for a text-only message.
func (c *Client) sendHTML(ctx context.Context, to, subject, text, html string) error {
	if c.Key == "" {
		return errors.New("mail: MAILGUN_SENDING_KEY not configured")
	}
	if strings.ContainsAny(to, "\r\n") {
		return errors.New("mail: invalid recipient")
	}
	form := url.Values{}
	form.Set("from", c.From)
	form.Set("to", to)
	form.Set("subject", subject)
	form.Set("text", text)
	if html != "" {
		form.Set("html", html)
	}

	base := c.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	endpoint := strings.TrimRight(base, "/") + "/v3/" + url.PathEscape(c.Domain) + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth("api", c.Key)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		// url.Error never contains the Authorization header, only the URL.
		return fmt.Errorf("mail: send failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mail: mailgun returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *Client) SendVerify(ctx context.Context, email, link string) error {
	base := link
	if i := strings.Index(link, "/verify?"); i > 0 {
		base = link[:i]
	}
	confirm := withEmailUTM(link, campaignVerifySignup, "confirm")
	examples := campaignPath(base, "/use-cases/news-apps-games", campaignVerifySignup, "use_cases_news_apps_games")

	text := "Welcome to Canvasbin!\n\nConfirm your email address to finish setting up your account:\n\n" + confirm +
		"\n\nAfter you verify you can connect Claude, ChatGPT or Cursor and publish pages from a chat. See examples: " +
		examples + "\n\nThis link expires in 24 hours. If you didn't sign up, you can ignore this email.\n"

	html, err := htmlEmail{
		Preheader: "Confirm your email, then connect your AI assistant and publish your first page.",
		Heading:   "Confirm your Canvasbin email",
		Intro: []string{
			"Thanks for signing up. Confirm your email address to finish creating your account.",
			"Once you're verified you can connect your AI assistant and publish HTML or Markdown pages at a private link, with a PDF and an edit link.",
		},
		Button:       &emailButton{Label: "Confirm email address", URL: confirm},
		PromptsIntro: "Popular first asks once you're connected:",
		Prompts:      starterPrompts[:3],
		Secondary:    absButtonsCampaign(base, useCaseLinks[:2], campaignVerifySignup),
		Footer:       "This link expires in 24 hours. If you didn't sign up, you can ignore this email.",
		Base:         withEmailUTM(base, campaignVerifySignup, "brand"),
	}.render()
	if err != nil {
		return err
	}
	return c.sendHTML(ctx, email, "Verify your Canvasbin email", text, html)
}

// SendVerifyNudge goes to accounts that are still unverified after the initial email.
func (c *Client) SendVerifyNudge(ctx context.Context, email, link, base string) error {
	confirm := withEmailUTM(link, campaignVerifyNudge, "confirm")
	connect := campaignPath(base, "/connect", campaignVerifyNudge, "connect")
	examples := campaignPath(base, "/use-cases/news-apps-games", campaignVerifyNudge, "use_cases_news_apps_games")
	allCases := campaignPath(base, "/use-cases", campaignVerifyNudge, "use_cases")

	text := "You signed up for Canvasbin but haven't confirmed your email yet.\n\n" +
		"Verify to connect your AI assistant and publish pages:\n\n" + confirm +
		"\n\nAfter you verify, connect here:\n" + connect + "\n\n" +
		"Try asking your assistant:\n"
	for _, p := range starterPrompts {
		text += "  \"" + p + "\"\n"
	}
	text += "\nExample pages: " + examples + "\n" +
		"More ideas: " + allCases + "\n\n" +
		"This link expires in 24 hours.\n"

	secondary := append(
		[]emailButton{{Label: "Connect your AI assistant", URL: connect}},
		absButtons(base, useCaseLinks)...,
	)

	html, err := htmlEmail{
		Preheader: "Confirm your email to connect Claude and publish your first page.",
		Heading:   "You're almost set up on Canvasbin",
		Intro: []string{
			"You created a Canvasbin account but haven't confirmed your email yet. Verification unlocks connecting Claude, ChatGPT or Cursor and publishing from a chat.",
			"Canvasbin gives you a shareable link and PDF for every page, and your assistant can update the same link later.",
		},
		Button:       &emailButton{Label: "Confirm email address", URL: confirm},
		PromptsIntro: "Paste one of these into your assistant after you connect:",
		Prompts:      starterPrompts,
		After: []string{
			"Browse real examples people published with an assistant: newsroom-style apps, Sunday briefings, games, and team dashboards.",
		},
		Secondary: tagButtons(secondary, campaignVerifyNudge),
		Footer:    "You're receiving this because you signed up and haven't verified yet. This is a one-time reminder.",
		Base:      withEmailUTM(strings.TrimRight(base, "/"), campaignVerifyNudge, "brand"),
	}.render()
	if err != nil {
		return err
	}
	return c.sendHTML(ctx, email, "Confirm your email to publish with Canvasbin", text, html)
}

// SendWelcome is sent once, when an account's email is first verified. base is
// the site origin, e.g. https://canvasbin.dev.
func (c *Client) SendWelcome(ctx context.Context, email, base string) error {
	connect := campaignPath(base, "/connect", campaignWelcome, "connect")
	examples := campaignPath(base, "/use-cases/news-apps-games", campaignWelcome, "use_cases_news_apps_games")
	editor := campaignPath(base, "/new", campaignWelcome, "new")
	contact := campaignPath(base, "/contact", campaignWelcome, "contact")

	text := "Your email is confirmed and your Canvasbin account is ready.\n\n" +
		"Canvasbin puts HTML and Markdown pages at a private link, with a PDF and an edit link. " +
		"The quickest way to use it is from your AI assistant. Connect Claude, ChatGPT or Cursor here:\n\n" +
		connect + "\n\n" +
		"Then try asking it something like:\n\n"
	for _, p := range starterPrompts[:3] {
		text += "  \"" + p + "\"\n"
	}
	text += "\nSee pages made this way: " + examples + "\n\n" +
		"Your assistant can update the same page later, so the link stays the same.\n\n" +
		"Prefer to write it yourself? Paste HTML or Markdown into the editor:\n\n" +
		editor + "\n\n" +
		"Questions or feedback: " + contact + "\n"

	html, err := htmlEmail{
		Preheader: "Connect your AI assistant and publish your first page in a minute.",
		Heading:   "Your Canvasbin account is ready",
		Intro: []string{
			"Your email is confirmed. Canvasbin puts HTML and Markdown pages at a private link, with a PDF and an edit link.",
			"The quickest way to use it is from your AI assistant. Connect Claude, ChatGPT or Cursor once, and it can publish and update pages for you.",
		},
		Button:       &emailButton{Label: "Connect your AI assistant", URL: connect},
		PromptsIntro: "Then paste one of these into your assistant:",
		Prompts:      starterPrompts[:3],
		After: []string{
			"Your assistant can update the same page later, so the link never changes. Prefer to write it yourself? Paste HTML or Markdown into the editor.",
		},
		Secondary: tagButtons([]emailButton{
			{Label: "See example pages", URL: examples},
			{Label: "Open the editor", URL: editor},
		}, campaignWelcome),
		Footer:     "You're receiving this because you created a Canvasbin account. Questions or feedback?",
		FooterLink: &emailButton{Label: "Contact us", URL: contact},
		Base:       withEmailUTM(strings.TrimRight(base, "/"), campaignWelcome, "brand"),
	}.render()
	if err != nil {
		return err
	}
	return c.sendHTML(ctx, email, "Your Canvasbin account is ready", text, html)
}

func (c *Client) SendReset(ctx context.Context, email, link string) error {
	reset := withEmailUTM(link, campaignPasswordReset, "reset")
	return c.send(ctx, email, "Reset your Canvasbin password",
		"We received a request to reset your Canvasbin password. Choose a new one here:\n\n"+reset+
			"\n\nThis link expires in 1 hour. If you didn't ask for this, you can ignore this email.\n")
}

func oneLine(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(strings.TrimSpace(s))
}

func (c *Client) SendTeamInvite(ctx context.Context, email, link, inviter, team string) error {
	who := oneLine(inviter)
	if who == "" {
		who = "Someone"
	}
	name := oneLine(team)
	join := withEmailUTM(link, campaignTeamInvite, "join")
	return c.send(ctx, email, who+" added you to "+name+" on Canvasbin",
		who+" added you to the team \""+name+"\" on Canvasbin, where you can view, edit and comment on the team's pages.\n\n"+
			"Sign up or log in with this email address to join:\n\n"+join+"\n")
}

func (c *Client) SendInvite(ctx context.Context, email, link, inviter string) error {
	who := oneLine(inviter)
	if who == "" {
		who = "Someone"
	}
	open := withEmailUTM(link, campaignPageShare, "open_page")
	return c.send(ctx, email, who+" shared a Canvasbin page with you",
		who+" shared a page with you on Canvasbin.\n\nSign up or log in with this email address to open it:\n\n"+open+"\n")
}
