// Package mailadpt sends the mails of the invites (ADR 0029 step 2.5): it
// words a structured authsvc.Notice from the template files embedded here,
// one set per language, and hands the finished Message to a Mailer (Brevo's
// HTTPS API, the server log of a local run, or a recorder in tests). This is
// the one place the backend composes text for a person, because a mail is
// worded where it is sent (ADR 0029 step 2.5, ADR 0023).
package mailadpt

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	texttemplate "text/template"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

// Message is a finished mail.
type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// Mailer sends a finished Message.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

//go:embed templates
var templateFiles embed.FS

// templateData is what the templates read. The mails carry only the inviter's
// name, the invitee's address and the application's address: no token, no
// secret.
type templateData struct {
	Inviter string
	Invitee string
	AppURL  string
}

type kindLanguage struct {
	kind authsvc.NoticeKind
	lang authsvc.Language
}

type compiled struct {
	subject *texttemplate.Template
	text    *texttemplate.Template
	html    *htmltemplate.Template
}

// Notifier implements authsvc.Notifier over a Mailer.
type Notifier struct {
	mailer    Mailer
	appURL    string
	templates map[kindLanguage]compiled
}

var _ authsvc.Notifier = (*Notifier)(nil)

// New builds the notifier: appURL is the application's address the mails
// link to. It parses every template, so a broken one stops the start.
func New(m Mailer, appURL string) (*Notifier, error) {
	n := &Notifier{mailer: m, appURL: appURL, templates: map[kindLanguage]compiled{}}
	for _, kind := range []authsvc.NoticeKind{authsvc.NoticeInvite, authsvc.NoticeAccepted} {
		for _, lang := range []authsvc.Language{authsvc.LanguageEnglish, authsvc.LanguageFrench} {
			c, err := compile(kind, lang)
			if err != nil {
				return nil, err
			}
			n.templates[kindLanguage{kind, lang}] = c
		}
	}
	return n, nil
}

func compile(kind authsvc.NoticeKind, lang authsvc.Language) (compiled, error) {
	name := fmt.Sprintf("templates/%s.%s.", kind, lang)
	read := func(ext string) (string, error) {
		b, err := fs.ReadFile(templateFiles, name+ext)
		if err != nil {
			return "", fmt.Errorf("mailadpt: %w", err)
		}
		return string(b), nil
	}
	var c compiled
	subject, err := read("subject")
	if err != nil {
		return c, err
	}
	text, err := read("txt")
	if err != nil {
		return c, err
	}
	html, err := read("html")
	if err != nil {
		return c, err
	}
	if c.subject, err = texttemplate.New(name + "subject").Option("missingkey=error").Parse(subject); err != nil {
		return c, fmt.Errorf("mailadpt: %w", err)
	}
	if c.text, err = texttemplate.New(name + "txt").Option("missingkey=error").Parse(text); err != nil {
		return c, fmt.Errorf("mailadpt: %w", err)
	}
	if c.html, err = htmltemplate.New(name + "html").Option("missingkey=error").Parse(html); err != nil {
		return c, fmt.Errorf("mailadpt: %w", err)
	}
	return c, nil
}

// Notify implements authsvc.Notifier: it words n in its language and sends it.
func (n *Notifier) Notify(ctx context.Context, notice authsvc.Notice) error {
	c, ok := n.templates[kindLanguage{notice.Kind, notice.Language}]
	if !ok {
		return fmt.Errorf("mailadpt: no template for %s in %q", notice.Kind, notice.Language)
	}
	data := templateData{Inviter: notice.InviterEmail, Invitee: notice.InviteeEmail, AppURL: n.appURL}
	var subject, text, html bytes.Buffer
	if err := c.subject.Execute(&subject, data); err != nil {
		return fmt.Errorf("mailadpt: %w", err)
	}
	if err := c.text.Execute(&text, data); err != nil {
		return fmt.Errorf("mailadpt: %w", err)
	}
	if err := c.html.Execute(&html, data); err != nil {
		return fmt.Errorf("mailadpt: %w", err)
	}
	return n.mailer.Send(ctx, Message{To: notice.To, Subject: subject.String(), TextBody: text.String(), HTMLBody: html.String()})
}

// Unconfigured is the notifier of a server without mail settings: every mail
// is authsvc.ErrMailNotConfigured.
type Unconfigured struct{}

var _ authsvc.Notifier = Unconfigured{}

// Notify implements authsvc.Notifier.
func (Unconfigured) Notify(context.Context, authsvc.Notice) error {
	return authsvc.ErrMailNotConfigured
}
