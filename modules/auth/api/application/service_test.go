package application_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/auth/application"
	"__GO_MODULE__/internal/modules/auth/ports"
	"__GO_MODULE__/internal/shared/apperror"
)

// fakeStore satisfies the UserStore port — the module is tested with no users module at all.
type fakeStore struct {
	users map[string]ports.Credentials
	byID  map[int64]ports.User
	// clock stamps password changes; a full service reads the same one (newServiceWith).
	clock *testClock
}

// testClock stands still until a test moves it. Windows' clock moves in steps of about a
// millisecond and Linux's every nanosecond, so two steps of a test can land on the same instant
// on one and not on the other; on this clock they do so only when the test says.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func newTestClock() *testClock {
	return &testClock{t: time.Now().UTC().Truncate(time.Microsecond)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newFakeStore(t *testing.T, email, password string, active bool) *fakeStore {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeStore{
		users: map[string]ports.Credentials{email: {UserID: 1, PasswordHash: string(hash), IsActive: active}},
		byID:  map[int64]ports.User{1: {ID: 1, Name: "Ana", Email: email, IsActive: active}},
	}
}

func (f *fakeStore) FindCredentials(_ context.Context, email string) (ports.Credentials, error) {
	c, ok := f.users[email]
	if !ok {
		return ports.Credentials{}, ports.ErrUserNotFound
	}
	return c, nil
}

func (f *fakeStore) FindUser(_ context.Context, id int64) (ports.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return ports.User{}, ports.ErrUserNotFound
	}
	return u, nil
}

// fakeSessions keeps sessions in memory.
type fakeSessions struct {
	mu      sync.Mutex
	rows    map[string]application.SessionRecord
	touches int
}

func (f *fakeSessions) Create(_ context.Context, r application.SessionRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[r.ID] = r
	return nil
}

func (f *fakeSessions) Get(_ context.Context, id string) (application.SessionRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok {
		return application.SessionRecord{}, application.ErrNoSession
	}
	return r, nil
}

func (f *fakeSessions) Revoke(_ context.Context, id string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.rows[id]
	r.Revoked = true
	f.rows[id] = r
	return nil
}

func (f *fakeSessions) Touch(_ context.Context, id string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.rows[id]
	r.LastSeenAt = at
	f.rows[id] = r
	f.touches++
	return nil
}

func (f *fakeSessions) DeleteExpired(context.Context, time.Time) error { return nil }

const secret = "test-secret-test-secret-test-secret"

// fakeResets keeps reset requests in memory.
type fakeResets struct {
	mu   sync.Mutex
	rows map[string]*application.ResetRecord
	next int64
}

func (f *fakeResets) Create(_ context.Context, rec application.ResetRecord, hash, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	rec.ID = f.next
	f.rows[hash] = &rec
	return nil
}

func (f *fakeResets) Release(_ context.Context, id int64, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			r.Used = false
		}
	}
	return nil
}

func (f *fakeResets) Find(_ context.Context, hash string) (application.ResetRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[hash]
	if !ok {
		return application.ResetRecord{}, application.ErrNoReset
	}
	return *r, nil
}

func (f *fakeResets) Use(_ context.Context, id int64, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id && !r.Used {
			r.Used = true
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeResets) VoidUser(_ context.Context, userID int64, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.UserID == userID {
			r.Used = true
		}
	}
	return nil
}

func (f *fakeResets) DeleteOld(context.Context, time.Time) error { return nil }

// fakePasswords is the PasswordWriter port: it rewrites the fake store.
type fakePasswords struct{ store *fakeStore }

func (f fakePasswords) SetPassword(_ context.Context, id int64, password string) error {
	if len(password) < 8 {
		return fmt.Errorf("%w: password: minimal 8 karakter", ports.ErrWeakPassword)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	u := f.store.byID[id]
	for email, c := range f.store.users {
		if c.UserID == id {
			c.PasswordHash = string(hash)
			f.store.users[email] = c
		}
	}
	u.PasswordChangedAt = f.store.clock.Now()
	f.store.byID[id] = u
	return nil
}

// outbox is the Mailer port: it keeps what was sent.
type outbox struct{ sent []string }

func (o *outbox) Send(_ context.Context, to, _, _, text, _ string) error {
	o.sent = append(o.sent, to+"\n"+text)
	return nil
}

type noAudit struct{}

func (noAudit) Record(context.Context, kernel.AuditEntry) {}

// auditLog is the Auditor: it keeps what was recorded.
type auditLog struct {
	mu      sync.Mutex
	entries []kernel.AuditEntry
}

func (a *auditLog) Record(_ context.Context, e kernel.AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, e)
}

func newService(store ports.UserStore) *application.Service {
	return application.NewService(application.Deps{
		Users: store, Sessions: &fakeSessions{rows: map[string]application.SessionRecord{}}, Resets: &fakeResets{rows: map[string]*application.ResetRecord{}},
		Tokens: application.NewTokens(secret, time.Hour, "app"), Limiter: application.NewLimiter(time.Minute, 1000), Audit: noAudit{},
	})
}

// newFullService has every optional port wired, and does its after-the-answer work at once. It
// and the store read store.clock: time passes only when the test moves it.
func newFullService(store *fakeStore) (*application.Service, *outbox) {
	return newServiceWith(store, fakePasswords{store})
}

func newServiceWith(store *fakeStore, passwords ports.PasswordWriter) (*application.Service, *outbox) {
	store.clock = newTestClock()
	mail := &outbox{}
	svc := application.NewService(application.Deps{
		Users: store, Passwords: passwords, Mailer: mail,
		Sessions: &fakeSessions{rows: map[string]application.SessionRecord{}}, Resets: &fakeResets{rows: map[string]*application.ResetRecord{}},
		Tokens: application.NewTokens(secret, time.Hour, "app"), Limiter: application.NewLimiter(time.Minute, 1000), Audit: noAudit{},
		AppURL: "https://app.example.com/", AppName: "App",
	})
	application.RunNow(svc)
	application.SetClock(svc, store.clock.Now)
	return svc, mail
}

var office = application.Client{IP: "203.0.113.7"}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func login(t *testing.T, svc *application.Service, email, password string) application.Session {
	t.Helper()
	s, err := svc.Login(context.Background(), email, password, office)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLoginAndResolve(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	s := login(t, svc, " Ana@Example.com ", "secret123")
	u, err := svc.Resolve(context.Background(), s.Token)
	if err != nil || u.ID != 1 {
		t.Fatalf("resolve: user=%+v err=%v", u, err)
	}
}

func TestWrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	_, wrong := svc.Login(context.Background(), "ana@example.com", "nope-nope", office)
	_, unknown := svc.Login(context.Background(), "who@example.com", "nope-nope", office)
	if status(t, wrong) != http.StatusUnauthorized || status(t, unknown) != http.StatusUnauthorized {
		t.Fatalf("want 401 for both, got %v / %v", wrong, unknown)
	}
	if wrong.Error() != unknown.Error() {
		t.Fatalf("messages differ: %q vs %q", wrong, unknown)
	}
}

func TestDeactivatedUserCannotSignInAndLearnsNothing(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", false))
	_, right := svc.Login(context.Background(), "ana@example.com", "secret123", office)
	_, wrong := svc.Login(context.Background(), "ana@example.com", "nope-nope", office)
	if status(t, right) != http.StatusUnauthorized || right.Error() != wrong.Error() {
		t.Fatalf("the right password on a deactivated account must answer like a wrong one: %v / %v", right, wrong)
	}
}

func TestDeactivationEndsAnOpenSession(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc := newService(store)
	s := login(t, svc, "ana@example.com", "secret123")
	u := store.byID[1]
	u.IsActive = false
	store.byID[1] = u
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("want 401 after deactivation, got %v", err)
	}
}

func TestLogoutEndsTheTokenEverywhere(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	s := login(t, svc, "ana@example.com", "secret123")
	other := login(t, svc, "ana@example.com", "secret123") // another device
	if err := svc.Logout(context.Background(), s.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("a signed-out token (e.g. copied before) still works: %v", err)
	}
	if _, err := svc.Resolve(context.Background(), other.Token); err != nil {
		t.Fatalf("signing out one device ended another: %v", err)
	}
}

func TestPasswordChangeEndsEarlierSessions(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc := newService(store)
	s := login(t, svc, "ana@example.com", "secret123")
	u := store.byID[1]
	u.PasswordChangedAt = time.Now().Add(time.Minute)
	store.byID[1] = u
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("a session from before the password change still works: %v", err)
	}
}

func TestPasswordChangeRightAfterSignInStillEndsTheSession(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc := newService(store)
	s := login(t, svc, "ana@example.com", "secret123")
	u := store.byID[1]
	u.PasswordChangedAt = time.Now() // the same second as the sign-in
	store.byID[1] = u
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("a session opened just before the password change survived: %v", err)
	}
}

func TestTooManyFailuresAreRefused(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	for i := 0; i < 5; i++ {
		_, _ = svc.Login(context.Background(), "ana@example.com", "wrong-pass", office)
	}
	_, err := svc.Login(context.Background(), "ana@example.com", "secret123", office)
	if got := status(t, err); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
	// The same person from another address is not locked out by someone else's guesses here.
	if _, err := svc.Login(context.Background(), "ana@example.com", "secret123", application.Client{IP: "198.51.100.9"}); err != nil {
		t.Fatalf("another address was locked out: %v", err)
	}
}

func TestParallelGuessesCannotSlipPastTheLimit(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	var wg sync.WaitGroup
	var mu sync.Mutex
	tried := 0
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Login(context.Background(), "ana@example.com", "wrong-pass", office)
			if e, ok := apperror.As(err); ok && e.Status == http.StatusUnauthorized {
				mu.Lock()
				tried++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if tried > 5 {
		t.Fatalf("%d passwords were checked in parallel, want at most 5", tried)
	}
}

func TestSprayingFromOneAddressIsLimited(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	refused := false
	for i := 0; i < 30; i++ {
		_, err := svc.Login(context.Background(), string(rune('a'+i%26))+"x@example.com", "wrong-pass", office)
		if e, ok := apperror.As(err); ok && e.Status == http.StatusTooManyRequests {
			refused = true
		}
	}
	if !refused {
		t.Fatal("30 accounts guessed from one address without being refused")
	}
}

func TestAFloodOfKeysNeitherGrowsMemoryNorLocksEveryoneOut(t *testing.T) {
	l := application.NewLimiter(time.Minute, 10)
	if _, ok := l.Reserve(application.Rule{Key: "busy", Max: 2}); !ok {
		t.Fatal("first attempt refused")
	}
	for i := 0; i < 100; i++ {
		l.Reserve(application.Rule{Key: fmt.Sprint("flood-", i), Max: 5}, application.Rule{Key: "busy", Max: 1000})
	}
	if _, ok := l.Reserve(application.Rule{Key: "fresh", Max: 5}); !ok {
		t.Fatal("a full limiter locked a new person out")
	}
	// "busy" was used all along: it is still counted, not dropped by the flood.
	if _, ok := l.Reserve(application.Rule{Key: "busy", Max: 101}); ok {
		t.Fatal("a key in use was dropped")
	}
}

func TestAFloodOfAddressesCannotPushOutAnAccountsCounter(t *testing.T) {
	l := application.NewLimiter(time.Minute, 10)
	for i := 0; i < 3; i++ {
		l.Reserve(application.Rule{Key: "e:ana@example.com", Max: 3})
	}
	for i := 0; i < 10_000; i++ { // a large IPv6 range, one /64 per attempt
		l.Reserve(application.Rule{Key: fmt.Sprintf("ip:2001:db8:%x::/64", i), Max: 5})
	}
	if _, ok := l.Reserve(application.Rule{Key: "e:ana@example.com", Max: 3}); ok {
		t.Fatal("a flood of addresses evicted the account's counter")
	}
}

func TestSuccessfulSignInsFromOneAddressAreNeverLimited(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc := newService(store)
	// An office behind one NAT: far more sign-ins than the per-address budget, all good ones.
	for i := 0; i < 3*20+5; i++ {
		if _, err := svc.Login(context.Background(), "ana@example.com", "secret123", office); err != nil {
			t.Fatalf("sign-in %d from the shared address refused: %v", i+1, err)
		}
	}
}

func TestFailuresSlowDownThenRecover(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	now := time.Now()
	application.SetClock(svc, func() time.Time { return now })
	for i := 0; i < 5; i++ {
		_, _ = svc.Login(context.Background(), "ana@example.com", "wrong-pass", office)
	}
	_, err := svc.Login(context.Background(), "ana@example.com", "secret123", office)
	var tm *application.TooManyError
	if !errors.As(err, &tm) || status(t, err) != http.StatusTooManyRequests || tm.RetryAfter < time.Second || tm.RetryAfter > 5*time.Minute {
		t.Fatalf("want 429 with a short Retry-After, got %v", err)
	}
	if !strings.Contains(err.Error(), "Coba lagi dalam") {
		t.Fatalf("the message must say when: %q", err)
	}
	now = now.Add(tm.RetryAfter)
	if _, err := svc.Login(context.Background(), "ana@example.com", "secret123", office); err != nil {
		t.Fatalf("after waiting, the right password must work: %v", err)
	}
}

func TestSomeoneGuessingCannotLockTheOwnerOutForLong(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	now := time.Now()
	application.SetClock(svc, func() time.Time { return now })
	// Many addresses guessing Ana's password, each attempt let through by its own counters.
	for i := 0; i < 60; i++ {
		_, _ = svc.Login(context.Background(), "ana@example.com", "wrong-pass", application.Client{IP: fmt.Sprintf("198.51.100.%d", i)})
		now = now.Add(2 * time.Second)
	}
	// Ana, from her own address, waits at most the account's longest delay.
	now = now.Add(30 * time.Second)
	if _, err := svc.Login(context.Background(), "ana@example.com", "secret123", office); err != nil {
		t.Fatalf("the owner was locked out by someone else's guesses: %v", err)
	}
}

func TestAnIdleSessionEndsAndAnUsedOneLives(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	sessions := &fakeSessions{rows: map[string]application.SessionRecord{}}
	svc := application.NewService(application.Deps{Users: store, Sessions: sessions, Resets: &fakeResets{rows: map[string]*application.ResetRecord{}},
		Tokens: application.NewTokens(secret, 24*time.Hour, "app"), Limiter: application.NewLimiter(time.Minute, 1000), Audit: noAudit{},
		IdleTimeout: time.Hour})
	now := time.Now()
	application.SetClock(svc, func() time.Time { return now })
	s := login(t, svc, "ana@example.com", "secret123")
	for i := 0; i < 5; i++ { // used every 50 minutes: never idle for an hour
		now = now.Add(50 * time.Minute)
		if _, err := svc.Resolve(context.Background(), s.Token); err != nil {
			t.Fatalf("a session in use ended after %d×50m: %v", i+1, err)
		}
	}
	before := sessions.touches
	if _, err := svc.Resolve(context.Background(), s.Token); err != nil || sessions.touches != before {
		t.Fatalf("a request within the minute must not write again (touches %d → %d): %v", before, sessions.touches, err)
	}
	now = now.Add(61 * time.Minute)
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("a session unused for over an hour still works: %v", err)
	}
}

func TestSigningInAgainEndsTheBrowsersOldSession(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	old := login(t, svc, "ana@example.com", "secret123")
	from := office
	from.PreviousToken = old.Token
	if _, err := svc.Login(context.Background(), "ana@example.com", "secret123", from); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Resolve(context.Background(), old.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("the browser's old session survived a new sign-in: %v", err)
	}
}

func TestIPv6IsLimitedPerSubnet(t *testing.T) {
	a, b := application.ClientKey("2001:db8:1:2::1"), application.ClientKey("2001:db8:1:2:ffff::9")
	if a != b || a != "2001:db8:1:2::/64" {
		t.Fatalf("one /64 must be one key: %q, %q", a, b)
	}
	if application.ClientKey("2001:db8:1:3::1") == a {
		t.Fatal("another /64 shares the key")
	}
	if k := application.ClientKey("203.0.113.7"); k != "203.0.113.7" {
		t.Fatalf("IPv4: %q", k)
	}
}

func TestAFailedSignInNeverRecordsWhatWasTyped(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	audit := &auditLog{}
	svc := application.NewService(application.Deps{Users: store, Sessions: &fakeSessions{rows: map[string]application.SessionRecord{}},
		Resets: &fakeResets{rows: map[string]*application.ResetRecord{}}, Tokens: application.NewTokens(secret, time.Hour, "app"),
		Limiter: application.NewLimiter(time.Minute, 1000), Audit: audit})
	for _, typed := range []string{"Hunter2-password", "budi.santoso@example.com"} {
		_, _ = svc.Login(context.Background(), typed, "whatever1", office)
	}
	for _, e := range audit.entries {
		if strings.Contains(strings.ToLower(e.Summary), "hunter2") || strings.Contains(e.Summary, "budi.santoso") {
			t.Fatalf("recorded as typed: %q", e.Summary)
		}
	}
	if len(audit.entries) != 2 || !strings.Contains(audit.entries[1].Summary, "b***@example.com") {
		t.Fatalf("entries: %+v", audit.entries)
	}
}

// racingStore changes the password while a sign-in is between checking it and opening the session.
type racingStore struct{ *fakeStore }

func (r racingStore) FindUser(ctx context.Context, id int64) (ports.User, error) {
	u, err := r.fakeStore.FindUser(ctx, id)
	u.PasswordChangedAt = time.Now()
	return u, err
}

func TestASignInRacingAPasswordChangeGetsNoSession(t *testing.T) {
	svc := newService(racingStore{newFakeStore(t, "ana@example.com", "secret123", true)})
	if _, err := svc.Login(context.Background(), "ana@example.com", "secret123", office); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("the old password opened a session after the change: %v", err)
	}
}

func TestOversizedEmailIsRefusedBeforeTheLimiter(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := svc.Login(context.Background(), string(long)+"@example.com", "x", office); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for an oversized email, got %v", err)
	}
}

func TestTokenFromAnotherAppOrSecretIsRejected(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	s := login(t, svc, "ana@example.com", "secret123")
	for name, tokens := range map[string]*application.Tokens{
		"other secret": application.NewTokens("another-secret-another-secret-xx", time.Hour, "app"),
		"other app":    application.NewTokens(secret, time.Hour, "another-app"),
	} {
		other := application.NewService(application.Deps{Users: newFakeStore(t, "ana@example.com", "secret123", true),
			Sessions: &fakeSessions{rows: map[string]application.SessionRecord{}}, Resets: &fakeResets{rows: map[string]*application.ResetRecord{}},
			Tokens: tokens, Limiter: application.NewLimiter(time.Minute, 1000), Audit: noAudit{}})
		if _, err := other.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
			t.Fatalf("%s: token was accepted: %v", name, err)
		}
	}
}

func TestPasswordFeaturesFollowTheWiredPorts(t *testing.T) {
	alone := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	if o := alone.Options(); o.PasswordChange || o.PasswordReset {
		t.Fatalf("without ports: %+v", o)
	}
	if err := alone.RequestReset(context.Background(), "ana@example.com", office); status(t, err) != http.StatusNotImplemented {
		t.Fatalf("forgot without a mailer: want 501, got %v", err)
	}
	full, _ := newFullService(newFakeStore(t, "ana@example.com", "secret123", true))
	if o := full.Options(); !o.PasswordChange || !o.PasswordReset {
		t.Fatalf("with ports: %+v", o)
	}
}

func TestChangePasswordKeepsThisDeviceAndEndsTheOthers(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc, _ := newFullService(store)
	here := login(t, svc, "ana@example.com", "secret123")
	there := login(t, svc, "ana@example.com", "secret123")
	store.clock.Advance(time.Second)
	if _, err := svc.ChangePassword(context.Background(), 1, "wrong-current", "new-secret-1", office); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("a wrong current password must be a 422, got %v", err)
	}
	fresh, err := svc.ChangePassword(context.Background(), 1, "secret123", "new-secret-1", office)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Resolve(context.Background(), fresh.Token); err != nil {
		t.Fatalf("the new session for this device must work: %v", err)
	}
	for name, tok := range map[string]string{"this device's old token": here.Token, "another device": there.Token} {
		if _, err := svc.Resolve(context.Background(), tok); status(t, err) != http.StatusUnauthorized {
			t.Errorf("%s still works after the change", name)
		}
	}
	store.clock.Advance(time.Second)
	if _, err := svc.Login(context.Background(), "ana@example.com", "new-secret-1", office); err != nil {
		t.Fatalf("the new password does not sign in: %v", err)
	}
}

// A session opened at the very instant the password changed may have been opened with the old
// password: it ends, on every OS.
func TestASessionOpenedAtTheInstantOfAPasswordChangeEnds(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc, _ := newFullService(store)
	s := login(t, svc, "ana@example.com", "secret123")
	u := store.byID[1]
	u.PasswordChangedAt = store.clock.Now()
	store.byID[1] = u
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("a session opened at the instant of the change survived: %v", err)
	}
}

// A sign-in checked at the very instant the password changed cannot tell whether the password it
// checked was the old or the new one, so it opens no session — even with the new password, and
// on every OS. A moment later the new password signs in.
func TestASignInAtTheInstantOfAPasswordChangeGetsNoSession(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc, _ := newFullService(store)
	if _, err := svc.ChangePassword(context.Background(), 1, "secret123", "new-secret-1", office); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(context.Background(), "ana@example.com", "new-secret-1", office); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("a sign-in at the instant of the change opened a session: %v", err)
	}
	store.clock.Advance(time.Microsecond)
	if _, err := svc.Login(context.Background(), "ana@example.com", "new-secret-1", office); err != nil {
		t.Fatalf("the new password does not sign in after the change: %v", err)
	}
}

func resetToken(t *testing.T, mail *outbox) string {
	t.Helper()
	last := mail.sent[len(mail.sent)-1]
	i := strings.Index(last, "token=")
	if i < 0 {
		t.Fatalf("no link in:\n%s", last)
	}
	tok, err := url.QueryUnescape(strings.Fields(last[i+len("token="):])[0])
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestForgotPasswordByEmail(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc, mail := newFullService(store)
	old := login(t, svc, "ana@example.com", "secret123")
	if err := svc.RequestReset(context.Background(), "ANA@example.com ", office); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 1 || !strings.Contains(mail.sent[0], "https://app.example.com/reset-password#token=") {
		t.Fatalf("mail: %v", mail.sent)
	}
	tok := resetToken(t, mail)
	store.clock.Advance(time.Second)
	if err := svc.ResetPassword(context.Background(), tok, "short", office); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("a short password: want 422, got %v", err)
	}
	if err := svc.ResetPassword(context.Background(), tok, "reset-secret-1", office); err != nil {
		t.Fatalf("a short password must not spend the token: %v", err)
	}
	if err := svc.ResetPassword(context.Background(), tok, "reset-secret-2", office); status(t, err) != http.StatusBadRequest {
		t.Fatalf("a used token: want 400, got %v", err)
	}
	if _, err := svc.Resolve(context.Background(), old.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatal("a session from before the reset still works")
	}
	store.clock.Advance(time.Second)
	if _, err := svc.Login(context.Background(), "ana@example.com", "reset-secret-1", office); err != nil {
		t.Fatalf("the reset password does not sign in: %v", err)
	}
}

func TestForgotPasswordNeverTellsWhoHasAnAccount(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc, mail := newFullService(store)
	if err := svc.RequestReset(context.Background(), "nobody@example.com", office); err != nil {
		t.Fatalf("an unknown email must answer like a known one, got %v", err)
	}
	inactive := newFakeStore(t, "off@example.com", "secret123", false)
	svcOff, mailOff := newFullService(inactive)
	_ = svcOff.RequestReset(context.Background(), "off@example.com", office)
	if len(mail.sent)+len(mailOff.sent) != 0 {
		t.Fatal("an email went to an unknown or deactivated account")
	}
	// Flooding one address: three emails, then silence — with the same answer.
	for i := 0; i < 6; i++ {
		if err := svc.RequestReset(context.Background(), "ana@example.com", office); err != nil {
			t.Fatal(err)
		}
	}
	if len(mail.sent) != 3 {
		t.Fatalf("%d reset emails to one address, want 3", len(mail.sent))
	}
}

func TestANewerResetVoidsTheOlderLink(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc, mail := newFullService(store)
	_ = svc.RequestReset(context.Background(), "ana@example.com", office)
	first := resetToken(t, mail)
	_ = svc.RequestReset(context.Background(), "ana@example.com", office)
	if err := svc.ResetPassword(context.Background(), first, "reset-secret-1", office); status(t, err) != http.StatusBadRequest {
		t.Fatalf("the older link must stop working, got %v", err)
	}
	if err := svc.ResetPassword(context.Background(), resetToken(t, mail), "reset-secret-1", office); err != nil {
		t.Fatal(err)
	}
}

func TestAResetLinkDiesWhenThePasswordChanges(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc, mail := newFullService(store)
	_ = svc.RequestReset(context.Background(), "ana@example.com", office)
	link := resetToken(t, mail)
	store.clock.Advance(time.Second)
	if _, err := svc.ChangePassword(context.Background(), 1, "secret123", "new-secret-1", office); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(context.Background(), link, "reset-secret-1", office); status(t, err) != http.StatusBadRequest {
		t.Fatalf("a link from before the change still works: %v", err)
	}
}

func TestAResetLinkDiesWhenTheEmailChanges(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc, mail := newFullService(store)
	_ = svc.RequestReset(context.Background(), "ana@example.com", office)
	u := store.byID[1]
	u.Email = "ana.baru@example.com"
	store.byID[1] = u
	if err := svc.ResetPassword(context.Background(), resetToken(t, mail), "reset-secret-1", office); status(t, err) != http.StatusBadRequest {
		t.Fatalf("a link sent to the old address still works: %v", err)
	}
}

// flakyPasswords fails the first save, as a database blip would.
type flakyPasswords struct {
	fakePasswords
	failed *bool
}

func (f flakyPasswords) SetPassword(ctx context.Context, id int64, password string) error {
	if !*f.failed {
		*f.failed = true
		return fmt.Errorf("database unavailable")
	}
	return f.fakePasswords.SetPassword(ctx, id, password)
}

func TestAResetThatCouldNotBeSavedKeepsTheLink(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc, mail := newServiceWith(store, flakyPasswords{fakePasswords{store}, new(bool)})
	_ = svc.RequestReset(context.Background(), "ana@example.com", office)
	link := resetToken(t, mail)
	if err := svc.ResetPassword(context.Background(), link, "reset-secret-1", office); err == nil {
		t.Fatal("want the save error")
	}
	if err := svc.ResetPassword(context.Background(), link, "reset-secret-1", office); err != nil {
		t.Fatalf("the link was spent by a failed save: %v", err)
	}
}
