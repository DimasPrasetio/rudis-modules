package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/audit-log/application"
	"__GO_MODULE__/internal/modules/audit-log/domain"
	"__GO_MODULE__/internal/shared/pagination"
)

type memRepo struct{ rows []domain.Entry }

func (m *memRepo) Insert(_ context.Context, e domain.Entry) error {
	e.ID = int64(len(m.rows) + 1)
	m.rows = append(m.rows, e)
	return nil
}

func (m *memRepo) List(_ context.Context, f application.Filter, _ pagination.Params) ([]domain.Entry, int64, error) {
	var out []domain.Entry
	for _, e := range m.rows {
		inBranch := f.Branches == nil || slices.Contains(f.Branches, e.BranchID)
		if e.CompanyID == f.Company && inBranch && (f.Kind == "" || e.Kind == f.Kind) && (f.Module == "" || e.Module == f.Module) &&
			!e.OccurredAt.Before(f.From) && e.OccurredAt.Before(f.To) {
			out = append(out, e)
		}
	}
	return out, int64(len(out)), nil
}

func (m *memRepo) Modules(context.Context, int64) ([]string, error) { return nil, nil }

func (m *memRepo) SystemDeleteOlderThan(_ context.Context, before time.Time) (int64, error) {
	kept := m.rows[:0]
	var n int64
	for _, e := range m.rows {
		if e.OccurredAt.Before(before) {
			n++
			continue
		}
		kept = append(kept, e)
	}
	m.rows = kept
	return n, nil
}

// everywhere is a reader holding audit-log.read across the active company.
var everywhere = application.Reach{AllBranches: true}

type directory map[int64]string

func (d directory) Names(_ context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	for _, id := range ids {
		if n, ok := d[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func TestWriteKeepsEveryRecordAndCutsOverlongValues(t *testing.T) {
	repo := &memRepo{}
	svc := application.NewService(repo, nil, 365*24*time.Hour)
	err := svc.Write(context.Background(), kernel.AuditRecord{
		AuditEntry: kernel.AuditEntry{Module: "users", Action: "user.updated", Target: "user:2", Summary: strings.Repeat("x", 1000), Data: map[string]any{"name": map[string]any{"before": "A", "after": "B"}}},
		At:         time.Now().UTC(), ActorID: 1, IP: "203.0.113.7", RequestID: "req-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	e := repo.rows[0]
	if len([]rune(e.Summary)) != domain.MaxSummary || !strings.Contains(string(e.Data), `"before":"A"`) || e.RequestID != "req-1" {
		t.Fatalf("entry = %+v", e)
	}
}

func TestListNamesActorsWhenItCan(t *testing.T) {
	repo := &memRepo{}
	now := time.Now().UTC()
	for _, actor := range []int64{1, 2, 0} {
		_ = repo.Insert(context.Background(), domain.Entry{OccurredAt: now, ActorID: actor, CompanyID: 1, Module: "users", Action: "user.updated"})
	}
	named, _, err := application.NewService(repo, directory{1: "Ana"}, time.Hour).List(context.Background(), everywhere, application.Filter{}, pagination.Params{Page: 1, Limit: 10})
	if err != nil || named[0].ActorName != "Ana" || named[1].ActorName != "" {
		t.Fatalf("named = %+v err=%v", named, err)
	}
	if _, _, err := application.NewService(repo, nil, time.Hour).List(context.Background(), everywhere, application.Filter{}, pagination.Params{Page: 1, Limit: 10}); err != nil {
		t.Fatal("without a directory the trail must still list")
	}
}

func TestPruneDropsOnlyWhatIsPastRetention(t *testing.T) {
	repo := &memRepo{}
	now := time.Now().UTC()
	_ = repo.Insert(context.Background(), domain.Entry{OccurredAt: now.AddDate(0, 0, -400)})
	_ = repo.Insert(context.Background(), domain.Entry{OccurredAt: now.AddDate(0, 0, -10)})
	if err := application.NewService(repo, nil, 365*24*time.Hour).Prune(context.Background()); err != nil || len(repo.rows) != 1 {
		t.Fatalf("rows left %d, err %v", len(repo.rows), err)
	}
}

func TestEachCompanySeesItsOwnTrailAndKinds(t *testing.T) {
	repo := &memRepo{}
	svc := application.NewService(repo, nil, time.Hour)
	at := time.Now().UTC()
	for _, r := range []kernel.AuditRecord{
		{AuditEntry: kernel.AuditEntry{Module: "users", Action: "user.updated"}, At: at, CompanyID: 1},
		{AuditEntry: kernel.AuditEntry{Kind: kernel.AuditAccess, Module: "hr", Action: "salary.viewed"}, At: at, CompanyID: 2, BranchID: 5},
	} {
		if err := svc.Write(context.Background(), r); err != nil {
			t.Fatal(err)
		}
	}
	if repo.rows[0].Kind != "change" || repo.rows[1].Kind != "access" || repo.rows[1].BranchID != 5 {
		t.Fatalf("stored: %+v", repo.rows)
	}
	in2 := kernel.WithScope(context.Background(), kernel.Scope{CompanyID: 2})
	got, total, err := svc.List(in2, everywhere, application.Filter{}, pagination.Params{Page: 1, Limit: 10})
	if err != nil || total != 1 || got[0].Module != "hr" {
		t.Fatalf("company 2 sees %d: %+v %v", total, got, err)
	}
	if _, total, _ := svc.List(in2, everywhere, application.Filter{Kind: "change"}, pagination.Params{Page: 1, Limit: 10}); total != 0 {
		t.Fatal("the kind filter let a change through")
	}
	if _, total, _ := svc.List(context.Background(), everywhere, application.Filter{}, pagination.Params{Page: 1, Limit: 10}); total != 1 {
		t.Fatal("company 1 sees another company's trail")
	}
}

type placements map[int64][]int64

func (p placements) CompaniesOf(_ context.Context, id int64) ([]int64, error) { return p[id], nil }

func TestInstallationLevelRecordsStayThereAndReachTheActorsCompanies(t *testing.T) {
	repo := &memRepo{}
	svc := application.NewService(repo, nil, time.Hour)
	svc.UseActorCompanies(placements{7: {2, 3, 2}})
	at := time.Now().UTC()
	signIn := func(actor int64) kernel.AuditRecord {
		return kernel.AuditRecord{AuditEntry: kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "auth", Action: "session.created"}, At: at, ActorID: actor}
	}
	for _, r := range []kernel.AuditRecord{signIn(7), signIn(0)} { // a known person, and a failed sign-in for an unknown email
		if err := svc.Write(context.Background(), r); err != nil {
			t.Fatal(err)
		}
	}
	page := pagination.Params{Page: 1, Limit: 10}
	count := func(ctx context.Context, r application.Reach) int64 {
		_, total, err := svc.List(ctx, r, application.Filter{}, page)
		if err != nil {
			t.Fatal(err)
		}
		return total
	}
	in := func(c int64) context.Context {
		return kernel.WithScope(context.Background(), kernel.Scope{CompanyID: c})
	}
	if n := count(in(1), everywhere); n != 0 {
		t.Fatalf("company 1 sees %d installation-level records — they are not company 1's", n)
	}
	if count(in(2), everywhere) != 1 || count(in(3), everywhere) != 1 {
		t.Fatal("each company the person is placed in should see their sign-in once")
	}
	if n := count(in(1), application.Reach{System: true, SystemAllowed: true}); n != 2 {
		t.Fatalf("the installation-level trail has %d records, want 2", n)
	}
	if _, _, err := svc.List(in(1), application.Reach{System: true}, application.Filter{}, page); !errors.Is(err, application.ErrSystemDenied) {
		t.Fatalf("an auditor without audit-log.system read the installation-level trail: %v", err)
	}
}

func TestABranchOnlyReaderSeesOnlyItsBranches(t *testing.T) {
	repo := &memRepo{}
	svc := application.NewService(repo, nil, time.Hour)
	at := time.Now().UTC()
	for _, b := range []int64{0, 4, 5} {
		_ = svc.Write(context.Background(), kernel.AuditRecord{AuditEntry: kernel.AuditEntry{Module: "sales", Action: "order.posted"}, At: at, CompanyID: 2, BranchID: b})
	}
	in2 := kernel.WithScope(context.Background(), kernel.Scope{CompanyID: 2})
	got, total, err := svc.List(in2, application.Reach{Branches: []int64{4}}, application.Filter{}, pagination.Params{Page: 1, Limit: 10})
	if err != nil || total != 1 || got[0].BranchID != 4 {
		t.Fatalf("branch 4 reader sees %d: %+v %v", total, got, err)
	}
	if _, _, err := svc.List(in2, application.Reach{}, application.Filter{}, pagination.Params{Page: 1, Limit: 10}); !errors.Is(err, application.ErrNoBranches) {
		t.Fatalf("a reader in no branch got the trail: %v", err)
	}
}
