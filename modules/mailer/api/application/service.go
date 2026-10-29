// Package application holds the mailer's use cases: queueing a message, delivering what is due,
// and keeping the outbox tidy.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/mailer/contracts"
	"__GO_MODULE__/internal/modules/mailer/domain"
	"__GO_MODULE__/internal/modules/mailer/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
)

// ErrNotFound means no message has that ID.
var ErrNotFound = errors.New("message not found")

// ErrSystemDenied means the installation-level outbox was asked for by someone who may not see
// it: it needs mailer.system across the home company (company 1).
var ErrSystemDenied = errors.New("installation-level outbox not allowed")

// ErrNotEverywhere means the reader holds the permission in some branches only: the outbox is
// the company's, not a branch's.
var ErrNotEverywhere = errors.New("permission not held across the company")

// Reach is whose outbox a request works on — what the presentation layer learns from d.Authz.
type Reach struct {
	System        bool // the installation-level outbox (company 0)
	SystemAllowed bool // mailer.system across the home company
	AllBranches   bool // the permission across the active company
}

// TaskDeliver is the task that sends one message (kernel d.Tasks); one per attempt.
const TaskDeliver = "mailer.deliver"

// How delivery behaves.
const (
	sendTimeout = time.Minute
	markTimeout = 10 * time.Second
	keepSent    = 30 * 24 * time.Hour
	keepFailed  = 90 * 24 * time.Hour
	// staleAfter is how long a message may sit due without a delivery before Rescue queues one.
	staleAfter = 10 * time.Minute
)

// Repository is the outbox table — the history of every email and its outcome. Delivery
// itself is a kernel task per attempt.
// Every call but the System ones works in one company's outbox (company 0: installation-level).
type Repository interface {
	Enqueue(ctx context.Context, m domain.Message) (int64, error)
	// Get returns the message, or ErrNotFound.
	Get(ctx context.Context, company, id int64) (domain.Message, error)
	// ClaimAttempt marks attempt as under way until until — only while it is the message's next
	// attempt and no other delivery of it holds the message; false otherwise.
	ClaimAttempt(ctx context.Context, company, id int64, attempt int, now, until time.Time) (bool, error)
	// ReleaseAttempt gives up the claim on attempt without counting it.
	ReleaseAttempt(ctx context.Context, company, id int64, attempt int) error
	// Postpone moves the due time of a message still waiting for attempt and due before
	// staleBefore — Rescue's claim on queuing that attempt; false when it is not.
	Postpone(ctx context.Context, company, id int64, attempt int, until, staleBefore time.Time) (bool, error)
	// MarkSent records the delivery and clears the bodies: a sent message is never sent again,
	// and what it said (a reset link, say) does not stay in the database.
	MarkSent(ctx context.Context, company, id int64, attempts int, at time.Time) error
	MarkFailedAttempt(ctx context.Context, company, id int64, status string, attempts int, next time.Time, lastErr string) error
	Requeue(ctx context.Context, company, id int64, now time.Time) (bool, error)
	SystemPrune(ctx context.Context, sentBefore, failedBefore time.Time) (int64, error)
	// SystemStale lists up to 100 unsettled messages (pending or sending) of any company due
	// before before.
	SystemStale(ctx context.Context, before time.Time) ([]domain.Message, error)
	List(ctx context.Context, company int64, status, search string, p pagination.Params) ([]domain.Message, int64, error)
}

// Envelope is a message as a transport sends it.
type Envelope struct {
	ID        int64
	CreatedAt time.Time // with ID, makes the Message-ID: the same for every attempt at a message
	From      string
	FromName  string
	ReplyTo   string // optional: the company's own address, when the sender identity gives one
	To        string
	ToName    string
	Subject   string
	Text      string
	HTML      string // the complete document, layout included; empty for plain text
}

// Transport hands a message to a mail server (or, in development, to the log).
type Transport interface {
	Send(ctx context.Context, e Envelope) error
}

// PermanentError is a failure retrying will not fix — the mail server refused the recipient
// or the message for good (SMTP 5xx).
type PermanentError struct{ Err error }

func (e PermanentError) Error() string { return e.Err.Error() }
func (e PermanentError) Unwrap() error { return e.Err }

// ConfigError is a refusal of the app's own setup, not of the message — SMTP authentication
// failed (535) or the server will not take mail from MAIL_FROM. Every message would fail the
// same way until someone fixes the configuration, so it is retried like a temporary failure
// and logged as an error, never counted as the recipient's permanent refusal.
type ConfigError struct{ Err error }

func (e ConfigError) Error() string { return "konfigurasi SMTP: " + e.Err.Error() }
func (e ConfigError) Unwrap() error { return e.Err }

// Auditor records actions on the outbox — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Sender is who the app's email comes from.
type Sender struct {
	Address string
	Name    string
	AppName string // shown in the layout's header
}

// Service is the mailer's use cases.
type Service struct {
	repo      Repository
	transport Transport
	from      Sender
	audit     Auditor
	tx        *kernel.TxSlot
	tasks     *kernel.TasksSlot
	identity  ports.SenderIdentity // nil: every company sends as MAIL_FROM_NAME
	log       *slog.Logger
	now       func() time.Time
}

func NewService(repo Repository, transport Transport, from Sender, audit Auditor, tx *kernel.TxSlot, tasks *kernel.TasksSlot, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, transport: transport, from: from, audit: audit, tx: tx, tasks: tasks, log: log, now: func() time.Time { return time.Now().UTC() }}
}

// UseSenderIdentity wires the optional SenderIdentity port — from register.go.late.
func (s *Service) UseSenderIdentity(i ports.SenderIdentity) { s.identity = i }

// RegisterTasks declares the delivery task — called in New. It runs outside a transaction (it
// talks to a mail server) and records each outcome itself; a crash between the server's
// acceptance and that record sends the message again (at least once).
func (s *Service) RegisterTasks() {
	s.tasks.Register(TaskDeliver, s.Deliver, kernel.TaskKind{Lane: kernel.LaneShort, Timeout: 2 * sendTimeout, MaxAttempts: 10, NonTransactional: true})
}

// delivery is one attempt at one message. The company travels in the payload: a message sent
// before any company was chosen (company 0) is delivered by a task that has no scope either.
type delivery struct {
	ID      int64 `json:"id"`
	Attempt int   `json:"attempt"`
	Company int64 `json:"company"`
}

func (s *Service) next(ctx context.Context, company, id int64, attempt int, at time.Time) error {
	return s.tasks.Enqueue(ctx, TaskDeliver, delivery{ID: id, Attempt: attempt, Company: company}, kernel.At(at), kernel.Dedupe(fmt.Sprintf("%d:%d", id, attempt)))
}

// target is the outbox a reader works on, from what they hold.
func target(ctx context.Context, r Reach) (int64, error) {
	if r.System {
		if !r.SystemAllowed {
			return 0, ErrSystemDenied
		}
		return 0, nil
	}
	if !r.AllBranches {
		return 0, ErrNotEverywhere
	}
	return kernel.ScopeFrom(ctx).CompanyID, nil
}

// Send validates and queues a message: the outbox row and its first delivery commit together —
// inside the caller's d.Tx when there is one, so an email is sent only if what it tells about
// happened. The request never waits for a mail server.
func (s *Service) Send(ctx context.Context, m contracts.Message) (int64, error) {
	if err := domain.Validate(m.To, m.ToName, m.Subject, m.Text, m.HTML); err != nil {
		return 0, fmt.Errorf("%w: %v", contracts.ErrInvalid, err)
	}
	// The company it is sent for: the one ctx works in; without a scope, the default company of
	// an app with one company, else none (0, installation level) — the email of a reset by
	// token, before any company is chosen.
	company, _ := kernel.CompanyOf(ctx)
	now := s.now()
	var id int64
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		if id, err = s.repo.Enqueue(ctx, domain.Message{CompanyID: company, To: m.To, ToName: m.ToName, Subject: m.Subject, Text: m.Text, HTML: m.HTML, NextAttemptAt: now, CreatedAt: now}); err != nil {
			return err
		}
		return s.next(ctx, company, id, 1, now)
	})
	return id, err
}

// sender is who a company's email comes from: always MAIL_FROM (the address SPF, DKIM, and
// DMARC are set up for), named after the company and answered to its own address when the
// sender identity knows them.
func (s *Service) sender(ctx context.Context, company int64) (name, replyTo, appName string) {
	name, appName = s.from.Name, s.from.AppName
	if s.identity == nil || company == 0 {
		return name, "", appName
	}
	id, err := s.identity.Sender(kernel.WithScope(ctx, kernel.Scope{CompanyID: company}))
	if err != nil {
		s.log.Warn("mailer: company sender identity unavailable, sending as MAIL_FROM_NAME", "company", company, "err", err)
		return name, "", appName
	}
	if id.Name != "" {
		name, appName = id.Name, id.Name
	}
	if a, err := mail.ParseAddress(id.ReplyTo); err == nil && a.Address == id.ReplyTo {
		replyTo = id.ReplyTo
	}
	return name, replyTo, appName
}

// Deliver makes one attempt at one message — the mailer.deliver task. A failure schedules the
// next attempt after 1 min, 5 min, 30 min, 2 h, then 6 h; a permanent refusal (SMTP 5xx) or the
// sixth failure marks the message failed for an administrator to retry.
func (s *Service) Deliver(ctx context.Context, t kernel.Task) error {
	var d delivery
	if err := json.Unmarshal(t.Payload, &d); err != nil {
		return fmt.Errorf("mailer: task %d: %w", t.ID, err)
	}
	m, err := s.repo.Get(ctx, d.Company, d.ID)
	if errors.Is(err, ErrNotFound) {
		return nil // pruned
	}
	if err != nil {
		return err
	}
	if m.Status == domain.StatusSent || m.Status == domain.StatusFailed {
		return nil // already settled — an attempt repeated after a crash
	}
	// One delivery per attempt: another task for it (a rescue, a retry after a lost record) or a
	// later attempt already queued finds the message taken, and stops.
	if ok, err := s.repo.ClaimAttempt(ctx, d.Company, m.ID, d.Attempt, s.now(), s.now().Add(2*sendTimeout+markTimeout)); err != nil || !ok {
		return err
	}
	fromName, replyTo, appName := s.sender(ctx, d.Company)
	html := ""
	if m.HTML != "" {
		html = Layout(appName, m.Subject, m.HTML)
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	sendErr := s.transport.Send(sendCtx, Envelope{ID: m.ID, CreatedAt: m.CreatedAt, From: s.from.Address, FromName: fromName, ReplyTo: replyTo,
		To: m.To, ToName: m.ToName, Subject: m.Subject, Text: m.Text, HTML: html})
	cancel()
	// The outcome is recorded even when the app is shutting down: an unrecorded delivery would
	// be sent again.
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), markTimeout)
	defer cancel()
	now := s.now()
	if sendErr != nil && ctx.Err() != nil {
		// Cut short by shutdown: not the message's failure. The task goes back in the queue and
		// this attempt is made again.
		if err := s.repo.ReleaseAttempt(markCtx, d.Company, m.ID, d.Attempt); err != nil {
			return errors.Join(ctx.Err(), err)
		}
		return ctx.Err()
	}
	if sendErr == nil {
		if err := s.repo.MarkSent(markCtx, d.Company, m.ID, d.Attempt, now); err != nil {
			return fmt.Errorf("mail %d was sent but not recorded, and may be sent again: %w", m.ID, err)
		}
		return nil
	}
	if errors.As(sendErr, new(ConfigError)) {
		s.log.Error("mailer: the mail server refused the app's SMTP configuration — every message will fail until it is fixed (SMTP_USERNAME, SMTP_PASSWORD, MAIL_FROM)",
			"mail", m.ID, "attempt", d.Attempt, "err", sendErr)
	}
	status, next := domain.NextAttempt(d.Attempt, errors.As(sendErr, new(PermanentError)), now)
	msg := sendErr.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return s.tx.Do(markCtx, func(ctx context.Context) error {
		if err := s.repo.MarkFailedAttempt(ctx, d.Company, m.ID, status, d.Attempt, next, msg); err != nil {
			return err
		}
		if status == domain.StatusFailed {
			return nil
		}
		return s.next(ctx, d.Company, m.ID, d.Attempt+1, next)
	})
}

// Rescue queues the next attempt of every message long due with no delivery under way — left
// by an upgrade from the mailer's own worker (0.1), or by a delivery task that failed for good
// before it could record the outcome. A message whose delivery task still waits is untouched
// (the task's dedupe key, "<id>:<attempt>", is the same); one being sent was marked when the
// attempt started. Each rescued message is postponed, so the next run reaches the ones after it.
func (s *Service) Rescue(ctx context.Context) error {
	stale, err := s.repo.SystemStale(ctx, s.now().Add(-staleAfter))
	if err != nil {
		return err
	}
	for _, m := range stale {
		err := s.tx.Do(ctx, func(ctx context.Context) error {
			ok, err := s.repo.Postpone(ctx, m.CompanyID, m.ID, m.Attempts+1, s.now(), s.now().Add(-staleAfter))
			if err != nil || !ok {
				return err // settled, attempted, or rescued since it was listed
			}
			return s.next(ctx, m.CompanyID, m.ID, m.Attempts+1, s.now())
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Prune drops sent messages after 30 days and failed ones after 90.
func (s *Service) Prune(ctx context.Context) error {
	now := s.now()
	_, err := s.repo.SystemPrune(ctx, now.Add(-keepSent), now.Add(-keepFailed))
	return err
}

// List is the outbox the reader may see — the active company's, or the installation-level one —
// newest first, optionally one status and a search on recipient/subject.
func (s *Service) List(ctx context.Context, r Reach, status, search string, p pagination.Params) ([]domain.Message, int64, error) {
	switch status {
	case "", domain.StatusPending, domain.StatusSending, domain.StatusSent, domain.StatusFailed:
	default:
		return nil, 0, apperror.BadRequest("Status tidak dikenal: " + status)
	}
	company, err := target(ctx, r)
	if err != nil {
		return nil, 0, err
	}
	return s.repo.List(ctx, company, status, search, p)
}

// Retry queues a failed message of the reader's outbox again, from its first attempt.
func (s *Service) Retry(ctx context.Context, r Reach, id int64) error {
	company, err := target(ctx, r)
	if err != nil {
		return err
	}
	now := s.now()
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		ok, err := s.repo.Requeue(ctx, company, id, now)
		if err != nil {
			return err
		}
		if !ok {
			return apperror.Conflict("Hanya email yang gagal yang dapat dikirim ulang")
		}
		return s.next(ctx, company, id, 1, now)
	})
	if err != nil {
		return err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "mailer", Action: "mail.requeued", Target: fmt.Sprintf("mail:%d", id), Summary: "Mengirim ulang email yang gagal"})
	return nil
}
