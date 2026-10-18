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
	return c.send(ctx, email, "Verify your Canvasbin email",
		"Welcome to Canvasbin!\n\nConfirm your email address to finish setting up your account:\n\n"+link+
			"\n\nThis link expires in 24 hours. If you didn't sign up, you can ignore this email.\n")
}

// SendWelcome is sent once, when an account's email is first verified. base is
// the site origin, e.g. https://canvasbin.dev.
func (c *Client) SendWelcome(ctx context.Context, email, base string) error {
	return c.send(ctx, email, "Your Canvasbin account is ready",
		"Your email is confirmed and your Canvasbin account is ready.\n\n"+
			"Canvasbin puts HTML and Markdown pages at a private link, with a PDF and an edit link. "+
			"The quickest way to use it is from your AI assistant. Connect Claude, ChatGPT or Cursor here:\n\n"+
			base+"/connect\n\n"+
			"Then try asking it something like:\n\n"+
			"  \"Turn this week's sales figures into a dashboard and publish it to Canvasbin.\"\n"+
			"  \"Write up our meeting notes as a Canvasbin page I can share with the team.\"\n"+
			"  \"Draw a diagram of how our services connect and give me a Canvasbin link and a PDF.\"\n\n"+
			"Your assistant can update the same page later, so the link stays the same.\n\n"+
			"Prefer to write it yourself? Paste HTML or Markdown into the editor:\n\n"+
			base+"/new\n\n"+
			"Questions or feedback: "+base+"/contact\n")
}

func (c *Client) SendReset(ctx context.Context, email, link string) error {
	return c.send(ctx, email, "Reset your Canvasbin password",
		"We received a request to reset your Canvasbin password. Choose a new one here:\n\n"+link+
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
	return c.send(ctx, email, who+" added you to "+name+" on Canvasbin",
		who+" added you to the team \""+name+"\" on Canvasbin, where you can view, edit and comment on the team's pages.\n\n"+
			"Sign up or log in with this email address to join:\n\n"+link+"\n")
}

func (c *Client) SendInvite(ctx context.Context, email, link, inviter string) error {
	who := oneLine(inviter)
	if who == "" {
		who = "Someone"
	}
	return c.send(ctx, email, who+" shared a Canvasbin page with you",
		who+" shared a page with you on Canvasbin.\n\nSign up or log in with this email address to open it:\n\n"+link+"\n")
}
