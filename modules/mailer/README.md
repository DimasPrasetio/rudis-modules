# mailer

Sends the app's email. Fullstack (Go + React), owns `mail_outbox`.

**How it works.** A module calls `MailClient.Send(ctx, Message{To, ToName, Subject, Text, HTML})`
through its own port (an adapter in the composition root — modules never import the mailer).
The message is validated (one address, no line breaks in headers, a plain-text body always) and
**stored** with its first delivery task (`d.Tasks`) — inside the caller's `d.Tx` when there is
one, so an email goes out only if what it tells about was committed; the request does not wait
for a mail server. The kernel runs each attempt once across containers (even during a deploy)
and reclaims one whose container died. A failure is retried after 1 min, 5 min, 30 min, 2 h, and
6 h; a permanent refusal of the recipient or the message (SMTP 5xx at RCPT or DATA) or the sixth
failure marks it **failed**. A refusal of the app's own setup — authentication (530/535) or a
sender refused at MAIL FROM — is a configuration error: retried on the same schedule and logged
as an error naming SMTP_USERNAME, SMTP_PASSWORD, MAIL_FROM, never settled as the recipient's
permanent refusal. Once the server has accepted a message (its reply to the end of DATA), an
error at QUIT or a dropped connection afterwards no longer counts as a failure, so it is not
sent twice; the Message-ID is the same on every attempt, so a receiver can recognise a repeat.
Delivery is at least once: a crash after the server accepted a message but before it was
recorded sends it again, and so may a message an administrator resends after its task failed mid-delivery. Every
10 minutes a rescue job queues the next attempt of any message long due with no delivery under
way (left by 0.1's own worker, or by a delivery task that failed for good before recording the
outcome); one whose task still waits, or whose attempt has started, is untouched. A send cut short by
shutdown is not counted: that attempt is made again. A sent message's bodies are cleared (a reset link does not stay in the database); its
row — recipient, subject, times — is kept 30 days, failed ones 90 (a daily job).

`HTML` is the inner body — paragraphs, a link, a button. The mailer wraps it in a plain,
client-safe layout with the app's name; email cannot follow the app's theme, so the layout is
neutral. Write the message in the sending module (it knows what to say).

**Sending for real** needs a transactional provider over SMTP (587 STARTTLS, or 465 TLS): a VPS's
own port 25 is usually blocked and its IP distrusted. Put SPF, DKIM, and DMARC for the provider on
the `MAIL_FROM` domain, or mail lands in spam. Without `SMTP_HOST`, development writes each email
to the log (the text body included — a reset link can be copied from there); **production refuses
to start** without it, so email is never dropped silently. `SMTP_TLS=none` (local test servers)
is refused in production too.

**Each company's outbox.** A message belongs to the company it was sent for: the one the request
works in, or — sent without a scope — the default company of an app with one company. In an app with
several companies, email sent before any company is chosen (a password reset by token) is
**installation-level** (company 0). The outbox shows the active company's messages to whoever
holds `mailer.read` across it (`mailer.manage` to resend); `?scope=system` opens the
installation-level outbox, only for `mailer.system` across the home company (company 1).
Recipients and subjects of one company are never shown to another.

**Sender.** Always `MAIL_FROM` — the address SPF, DKIM, and DMARC are set up for. With the
optional `SenderIdentity` port wired (adapter: `settings`), a company's email is named after it
and replies go to its own email address (Reply-To); without it, `MAIL_FROM_NAME`.

**Kernel 7.** Uses `d.Tx`, `d.Tasks` (delivery; the company travels in the task), `d.Jobs` (the
daily prune, the rescue every 10 minutes), and `d.Audit` (a resend is recorded). Unscoped email
takes its company from `kernel.CompanyOf`: the default company in an app with one company,
installation level (company 0) in an app with several.

**Offers** `contracts.MailClient`. **Needs** nothing; optional late port `SenderIdentity`.

**Web:** `/mailer/outbox` — "Kotak keluar": search, status filter, failed messages with the
server's answer, and "Kirim ulang" (`mailer.manage`); "Tingkat instalasi" for `mailer.system`.
Nav entry in "Pengaturan" (`mailer.read`).

**Not in this version:** attachments, inbound mail, bulk sending, per-provider HTTP APIs.

## Changes

- **0.3.1** — requires kernel 7: the company a message is sent for comes from
  `kernel.CompanyOf` instead of recovering from `ScopeFrom`'s panic; behavior is unchanged.
- **0.3.0** — scoped (`"scoped": true`): each message keeps its company (migration 002;
  messages from before the upgrade are installation-level, company 0, seen on the
  installation-level outbox), and the outbox and resend work in the active company only; new
  permission `mailer.system` for the installation-level outbox (`?scope=system`). New optional
  late port `SenderIdentity` (adapter: `settings`) names email after its company, with the
  company's address as Reply-To. Delivery: an error at QUIT after the server accepted the
  message is no longer a failure (it sent the message twice); authentication failures and a
  sender refused at MAIL FROM are configuration errors, retried and logged instead of failing
  every message at once (RFC 5321 §4.2, RFC 4954 §6); the Message-ID is stable per message. The
  outbox search matches `%` and `_` literally. Upgrading from 0.2: `mailer.read` and
  `mailer.manage` now need to be held across the company. Grounds: transactional outbox —
  https://microservices.io/patterns/data/transactional-outbox.html.

- **0.2.0** — needs kernel 6. Delivery is a kernel task per attempt instead of the module's own
  polling job and lease: queued in the sender's transaction, run once across containers,
  reclaimed after a crash. Same retry schedule and outbox page. Messages pending at the upgrade
  are picked up by the rescue job within 10 minutes.
- **0.1.0** — first version.
