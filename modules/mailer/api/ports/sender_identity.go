// Package ports is what the mailer module needs from outside, in its own words.
package ports

import "context"

// Identity is who a company's email is from, as people read it.
type Identity struct {
	Name    string // shown as the sender ("Toko Maju"); empty: MAIL_FROM_NAME
	ReplyTo string // where replies go (the company's own address); empty: none
}

// SenderIdentity names the sender after the company the email is sent for. Optional: without it
// every company's email comes from MAIL_FROM_NAME. The address stays MAIL_FROM either way — the
// one domain SPF, DKIM, and DMARC are set up for. Called with ctx working in that company.
type SenderIdentity interface {
	Sender(ctx context.Context) (Identity, error)
}
