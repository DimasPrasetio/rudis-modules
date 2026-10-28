package application_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/kernel/kerneltest"
	"__GO_MODULE__/internal/modules/mailer/application"
	"__GO_MODULE__/internal/modules/mailer/contracts"
	"__GO_MODULE__/internal/modules/mailer/domain"
	"__GO_MODULE__/internal/modules/mailer/ports"
	"__GO_MODULE__/internal/shared/pagination"
)

// memRepo is the outbox in memory: the module is tested without MySQL.
type memRepo struct {
	mu     sync.Mutex
	rows   map[int64]*domain.Message
	locked map[int64]time.Time // locked_until
	next   int64
}

func newRepo() *memRepo {
	return &memRepo{rows: map[int64]*domain.Message{}, locked: map[int64]time.Time{}}
}

func (r *memRepo) Enqueue(_ context.Context, m domain.Message) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	m.ID, m.Status = r.next, domain.StatusPending
	r.rows[m.ID] = &m
	return m.ID, nil
}

func (r *memRepo) Get(_ context.Context, _, id int64) (domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.rows[id]
	if !ok {
		return domain.Message{}, application.ErrNotFound
	}
	return *m, nil
}

func (r *memRepo) Postpone(_ context.Context, _, id int64, attempt int, until, staleBefore time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.rows[id]
	if (m.Status != domain.StatusPending && m.Status != domain.StatusSending) || m.Attempts != attempt-1 || !m.NextAttemptAt.Before(staleBefore) {
		return false, nil
	}
	m.NextAttemptAt = until
	return true, nil
}

func (r *memRepo) ReleaseAttempt(_ context.Context, _, id int64, _ int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.locked, id)
	return nil
}

func (r *memRepo) ClaimAttempt(_ context.Context, _, id int64, attempt int, now, until time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.rows[id]
	if (m.Status != domain.StatusPending && m.Status != domain.StatusSending) || m.Attempts != attempt-1 || r.locked[id].After(now) {
		return false, nil
	}
	m.Status, m.NextAttemptAt = domain.StatusSending, now
	r.locked[id] = until
	return true, nil
}

func (r *memRepo) MarkSent(_ context.Context, _, id int64, attempts int, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.rows[id]
	m.Status, m.SentAt, m.Attempts, m.Text, m.HTML = domain.StatusSent, at, attempts, "", ""
	delete(r.locked, id)
	return nil
}

func (r *memRepo) MarkFailedAttempt(_ context.Context, _, id int64, status string, attempts int, next time.Time, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.rows[id]
	m.Status, m.Attempts, m.NextAttemptAt, m.LastError = status, attempts, next, lastErr
	delete(r.locked, id)
	return nil
}

func (r *memRepo) Requeue(_ context.Context, _, id int64, now time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.rows[id]
	if !ok || m.Status != domain.StatusFailed {
		return false, nil
	}
	m.Status, m.Attempts, m.NextAttemptAt = domain.StatusPending, 0, now
	return true, nil
}

func (r *memRepo) SystemStale(_ context.Context, before time.Time) ([]domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Message
	for _, m := range r.rows {
		if (m.Status == domain.StatusPending || m.Status == domain.StatusSending) && m.NextAttemptAt.Before(before) {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (r *memRepo) SystemPrune(context.Context, time.Time, time.Time) (int64, error) { return 0, nil }

func (r *memRepo) List(_ context.Context, company int64, _, _ string, _ pagination.Params) ([]domain.Message, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Message
	for _, m := range r.rows {
		if m.CompanyID == company {
			out = append(out, *m)
		}
	}
	return out, int64(len(out)), nil
}

// transport records what it was asked to send and fails as told.
type transport struct {
	sent []application.Envelope
	fail error
}

func (t *transport) Send(_ context.Context, e application.Envelope) error {
	if t.fail != nil {
		return t.fail
	}
	t.sent = append(t.sent, e)
	return nil
}

type noAudit struct{}

func (noAudit) Record(context.Context, kernel.AuditEntry) {}

func setup(t *testing.T) (*application.Service, *memRepo, *transport, *kerneltest.Kit) {
	k := kerneltest.Deps(t)
	repo, tr := newRepo(), &transport{}
	svc := application.NewService(repo, tr, application.Sender{Address: "noreply@app.test", Name: "App", AppName: "App"}, noAudit{}, k.Deps.Tx, k.Deps.Tasks, nil)
	svc.RegisterTasks()
	return svc, repo, tr, k
}

func TestSendQueuesAndATaskDelivers(t *testing.T) {
	svc, repo, tr, k := setup(t)
	id, err := svc.Send(context.Background(), contracts.Message{To: "ana@example.com", Subject: "Halo", Text: "Isi", HTML: "<p>Isi</p>"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.sent) != 0 || len(k.Enqueued()) != 1 {
		t.Fatalf("Send must queue a task, not wait for the mail server (sent %d, tasks %d)", len(tr.sent), len(k.Enqueued()))
	}
	k.RunTasks()
	if len(tr.sent) != 1 || repo.rows[id].Status != domain.StatusSent || repo.rows[id].Attempts != 1 {
		t.Fatalf("sent=%d status=%s", len(tr.sent), repo.rows[id].Status)
	}
	if !strings.Contains(tr.sent[0].HTML, "<p>Isi</p>") || !strings.Contains(tr.sent[0].HTML, "<html") {
		t.Fatal("the inner HTML must be wrapped in the layout")
	}
	if repo.rows[id].Text != "" {
		t.Fatal("a sent message keeps its body")
	}
	_ = k.Deps.Tasks.Enqueue(context.Background(), application.TaskDeliver, map[string]int64{"id": id, "attempt": 1})
	k.RunTasks() // an attempt repeated after a crash
	if len(tr.sent) != 1 {
		t.Fatal("a settled message was sent again")
	}
}

func TestInvalidMessagesAreRefused(t *testing.T) {
	svc, _, _, _ := setup(t)
	for name, m := range map[string]contracts.Message{
		"no recipient":     {Subject: "s", Text: "t"},
		"bad address":      {To: "not-an-email", Subject: "s", Text: "t"},
		"header injection": {To: "a@example.com", Subject: "s\r\nBcc: x@evil.example", Text: "t"},
		"no text":          {To: "a@example.com", Subject: "s"},
	} {
		if _, err := svc.Send(context.Background(), m); !errors.Is(err, contracts.ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
}

func TestTemporaryFailuresRetryThenGiveUp(t *testing.T) {
	svc, repo, tr, k := setup(t)
	id, _ := svc.Send(context.Background(), contracts.Message{To: "ana@example.com", Subject: "s", Text: "t"})
	tr.fail = errors.New("421 try later")
	// Each failed attempt queues the next one (at 1 min, 5 min, …); RunTasks runs them all.
	if n := k.RunTasks(); n != domain.MaxAttempts {
		t.Fatalf("ran %d attempts, want %d", n, domain.MaxAttempts)
	}
	if repo.rows[id].Status != domain.StatusFailed || repo.rows[id].Attempts != domain.MaxAttempts {
		t.Fatalf("after %d attempts: status=%s attempts=%d", domain.MaxAttempts, repo.rows[id].Status, repo.rows[id].Attempts)
	}
	tr.fail = nil
	if err := svc.Retry(context.Background(), application.Reach{AllBranches: true}, id); err != nil || repo.rows[id].Status != domain.StatusPending {
		t.Fatalf("retry: %v %s", err, repo.rows[id].Status)
	}
	k.RunTasks()
	if repo.rows[id].Status != domain.StatusSent {
		t.Fatalf("after retry: %s", repo.rows[id].Status)
	}
}

func TestPermanentFailureGivesUpAtOnce(t *testing.T) {
	svc, repo, tr, k := setup(t)
	id, _ := svc.Send(context.Background(), contracts.Message{To: "nobody@example.com", Subject: "s", Text: "t"})
	tr.fail = application.PermanentError{Err: errors.New("550 no such user")}
	if n := k.RunTasks(); n != 1 {
		t.Fatalf("a permanent refusal was retried (%d attempts)", n)
	}
	if repo.rows[id].Status != domain.StatusFailed || !strings.Contains(repo.rows[id].LastError, "550") {
		t.Fatalf("status=%s error=%q", repo.rows[id].Status, repo.rows[id].LastError)
	}
}

func TestRescueQueuesMessagesLeftWithoutADelivery(t *testing.T) {
	svc, repo, tr, k := setup(t)
	ctx := context.Background()
	// Left by the mailer's own worker before 0.2: pending, no task.
	old := time.Now().UTC().Add(-time.Hour)
	id, _ := repo.Enqueue(ctx, domain.Message{To: "a@example.com", Subject: "Hi", Text: "x", NextAttemptAt: old, CreatedAt: old})
	if _, err := svc.Send(ctx, contracts.Message{To: "b@example.com", Subject: "Hi", Text: "y"}); err != nil {
		t.Fatal(err) // queued normally: its task waits, so Rescue leaves it alone
	}
	if err := svc.Rescue(ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(k.Enqueued()); n != 2 {
		t.Fatalf("tasks = %d, want the normal one and one rescue", n)
	}
	k.RunTasks()
	if len(tr.sent) != 2 || repo.rows[id].Status != domain.StatusSent {
		t.Fatalf("sent %d, rescued message %s", len(tr.sent), repo.rows[id].Status)
	}
}

func TestOneAttemptIsDeliveredOnce(t *testing.T) {
	svc, repo, tr, k := setup(t)
	ctx := context.Background()
	id, err := svc.Send(ctx, contracts.Message{To: "a@example.com", Subject: "Hi", Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	// Another delivery of attempt 1 (a rescue, a retried task) is sending it right now.
	if ok, _ := repo.ClaimAttempt(ctx, 1, id, 1, time.Now(), time.Now().Add(time.Minute)); !ok {
		t.Fatal("claim")
	}
	k.RunTasks()
	if len(tr.sent) != 0 {
		t.Fatalf("attempt 1 was sent twice at once (%d)", len(tr.sent))
	}
	if err := svc.Rescue(ctx); err != nil || len(k.Enqueued()) != 0 {
		t.Fatalf("a message being sent was rescued: %v %d", err, len(k.Enqueued()))
	}
}

type severalCompanies struct{}

func (severalCompanies) Resolve(context.Context, int64, string) (kernel.Scope, error) {
	return kernel.Scope{}, kernel.ErrScopeDenied
}

func (severalCompanies) Companies(context.Context) ([]kernel.Scope, error) {
	return []kernel.Scope{{CompanyID: 1}, {CompanyID: 2}}, nil
}

func TestAMessageBelongsToTheCompanyItWasSentFor(t *testing.T) {
	svc, repo, _, k := setup(t)
	send := func(ctx context.Context) int64 {
		id, err := svc.Send(ctx, contracts.Message{To: "ana@example.com", Subject: "Halo", Text: "Isi"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	in2 := kernel.WithScope(context.Background(), kernel.Scope{CompanyID: 2})
	a, b := send(in2), send(context.Background())
	if repo.rows[a].CompanyID != 2 {
		t.Fatalf("sent in company 2, filed under %d", repo.rows[a].CompanyID)
	}
	if repo.rows[b].CompanyID != 1 {
		t.Fatalf("an app with one company files unscoped email under it, got %d", repo.rows[b].CompanyID)
	}
	k.Deps.Scope.Provide(severalCompanies{}) // an organization module: several companies, no default
	c := send(context.Background())
	if repo.rows[c].CompanyID != 0 {
		t.Fatalf("with several companies, unscoped email (a reset by token) is installation-level, got company %d", repo.rows[c].CompanyID)
	}
	k.Deps.Scope.Provide(nil)
	everywhere := application.Reach{AllBranches: true}
	if got, n, _ := svc.List(in2, everywhere, "", "", pagination.Params{Page: 1, Limit: 10}); n != 1 || got[0].ID != a {
		t.Fatalf("company 2's outbox shows %d messages: %+v", n, got)
	}
	if _, _, err := svc.List(in2, application.Reach{System: true}, "", "", pagination.Params{Page: 1, Limit: 10}); !errors.Is(err, application.ErrSystemDenied) {
		t.Fatalf("the installation-level outbox opened without mailer.system: %v", err)
	}
	if _, _, err := svc.List(in2, application.Reach{}, "", "", pagination.Params{Page: 1, Limit: 10}); !errors.Is(err, application.ErrNotEverywhere) {
		t.Fatalf("a branch-only reader saw the company's outbox: %v", err)
	}
	if err := svc.Retry(in2, everywhere, b); err == nil {
		t.Fatal("company 2 requeued company 1's message") // memRepo ignores the company; the error is "not failed"
	}
}

func TestAConfigurationRefusalIsRetriedNotFailed(t *testing.T) {
	svc, repo, tr, k := setup(t)
	id, err := svc.Send(context.Background(), contracts.Message{To: "ana@example.com", Subject: "Halo", Text: "Isi"})
	if err != nil {
		t.Fatal(err)
	}
	tr.fail = application.ConfigError{Err: errors.New("535 5.7.8 authentication failed")}
	k.RunTasks()
	m := repo.rows[id]
	if m.Status == domain.StatusFailed && m.Attempts < domain.MaxAttempts {
		t.Fatalf("a configuration error settled the message after %d attempt(s)", m.Attempts)
	}
	if !strings.Contains(m.LastError, "konfigurasi SMTP") {
		t.Fatalf("last error does not say it is the configuration: %q", m.LastError)
	}
}

type identity struct{}

func (i identity) Sender(ctx context.Context) (ports.Identity, error) {
	if kernel.ScopeFrom(ctx).CompanyID != 2 {
		return ports.Identity{}, errors.New("asked for the wrong company")
	}
	return ports.Identity{Name: "Toko Dua", ReplyTo: "halo@tokodua.test"}, nil
}

func TestAMessageIsNamedAfterItsCompany(t *testing.T) {
	svc, _, tr, k := setup(t)
	svc.UseSenderIdentity(identity{})
	in2 := kernel.WithScope(context.Background(), kernel.Scope{CompanyID: 2})
	if _, err := svc.Send(in2, contracts.Message{To: "ana@example.com", Subject: "Halo", Text: "Isi", HTML: "<p>Isi</p>"}); err != nil {
		t.Fatal(err)
	}
	k.RunTasks()
	if len(tr.sent) != 1 || tr.sent[0].FromName != "Toko Dua" || tr.sent[0].ReplyTo != "halo@tokodua.test" || tr.sent[0].From != "noreply@app.test" {
		t.Fatalf("sent %+v", tr.sent)
	}
}
