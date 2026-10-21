package mail

import (
	"context"
	"fmt"
	"strings"
)

// SendUpdateSubscribeConfirm asks the recipient to confirm a page subscription.
func (c *Client) SendUpdateSubscribeConfirm(ctx context.Context, email, link, pageTitle, product string) error {
	if product == "" {
		product = "Canvasbin"
	}
	page := oneLine(pageTitle)
	if page == "" {
		page = "this page"
	}
	text := fmt.Sprintf("Confirm your subscription to updates for %s on %s:\n\n%s\n\nIf you did not request this, ignore this email.\n",
		page, product, link)
	html, err := htmlEmail{
		Preheader: "Confirm your email to receive page updates.",
		Heading:   "Confirm your subscription",
		Intro: []string{
			fmt.Sprintf("You asked to receive email updates for %s on %s.", page, product),
			"Confirm your email address to start receiving new posts.",
		},
		Button: &emailButton{Label: "Confirm subscription", URL: link},
		Footer: fmt.Sprintf("You're receiving this because someone entered this address to subscribe to %s.", page),
		Base:   linkBase(link),
	}.render()
	if err != nil {
		return err
	}
	return c.sendHTML(ctx, email, "Confirm your subscription on "+product, text, html)
}

// SendUpdateNotification emails subscribers about a new post.
func (c *Client) SendUpdateNotification(ctx context.Context, email, product, pageTitle, updateTitle, summary, viewURL, unsubURL string) error {
	if product == "" {
		product = "Canvasbin"
	}
	page := oneLine(pageTitle)
	if page == "" {
		page = "this page"
	}
	updateTitle = oneLine(updateTitle)
	summary = oneLine(summary)
	text := updateTitle + "\n\n"
	if summary != "" {
		text += summary + "\n\n"
	}
	text += "View update: " + viewURL + "\n\nUnsubscribe: " + unsubURL + "\n"
	html, err := htmlEmail{
		Preheader: summary,
		Heading:   updateTitle,
		Intro:     introLines(summary),
		Button:    &emailButton{Label: "View update", URL: viewURL},
		Footer:    fmt.Sprintf("You're receiving this email because you subscribed to updates for %s on %s.", page, product),
		FooterLink: &emailButton{Label: "Unsubscribe", URL: unsubURL},
		Base:       viewBase(viewURL),
	}.render()
	if err != nil {
		return err
	}
	subject := updateTitle
	if subject == "" {
		subject = "New update on " + page
	}
	return c.sendHTML(ctx, email, subject, text, html)
}

func introLines(summary string) []string {
	if summary == "" {
		return nil
	}
	return []string{summary}
}

func linkBase(link string) string {
	if i := strings.Index(link, "/api/"); i > 0 {
		return link[:i]
	}
	if i := strings.Index(link, "://"); i >= 0 {
		if j := strings.Index(link[i+3:], "/"); j >= 0 {
			return link[:i+3+j]
		}
	}
	return link
}

func viewBase(viewURL string) string { return linkBase(viewURL) }
