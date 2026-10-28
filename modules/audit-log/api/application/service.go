// Package application holds the audit trail's use cases: storing what the kernel hands over,
// and reading it back.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/audit-log/domain"
	"__GO_MODULE__/internal/modules/audit-log/ports"
	"__GO_MODULE__/internal/shared/pagination"
)

// Filter narrows the trail. From and To are a half-open interval.
type Filter struct {
	Company int64 // set by List: the request's company, or 0 for the installation-level trail
	// Branches, when set, limits the trail to records made in these branches — for someone who
	// may read it only there. Nil: the whole company.
	Branches []int64
	Kind     string
	Module   string
	Actor    int64
	Target   string
	From     time.Time
	To       time.Time
}

// ErrSystemDenied means the installation-level trail was asked for by someone who may not read
// it: it needs audit-log.system across the home company (company 1), where the installation is
// administered.
var ErrSystemDenied = errors.New("installation-level trail not allowed")

// ErrNoBranches means a branch-only reader holds the permission in no branch at all.
var ErrNoBranches = errors.New("no branch in reach")

// Reach is how far the reader may see: what the presentation layer learns from d.Authz.
type Reach struct {
	// System asks for the installation-level trail (company 0): records made before any company
	// was chosen, with several companies.
	System bool
	// SystemAllowed: audit-log.system across the home company.
	SystemAllowed bool
	// AllBranches: audit-log.read across the active company; else Branches lists where it holds.
	AllBranches bool
	Branches    []int64
}

// Repository is the audit_log table.
type Repository interface {
	Insert(ctx context.Context, e domain.Entry) error
	List(ctx context.Context, f Filter, p pagination.Params) ([]domain.Entry, int64, error)
	Modules(ctx context.Context, company int64) ([]string, error)
	SystemDeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// Service is the audit trail.
type Service struct {
	repo      Repository
	users     ports.UserDirectory  // nil: actors shown by ID
	companies ports.ActorCompanies // nil: installation-level records stay installation-level only
	retention time.Duration
	now       func() time.Time
}

func NewService(repo Repository, users ports.UserDirectory, retention time.Duration) *Service {
	return &Service{repo: repo, users: users, retention: retention, now: func() time.Time { return time.Now().UTC() }}
}

// UseActorCompanies wires the optional ActorCompanies port — from register.go.late, once the
// provider (an organization module) is built.
func (s *Service) UseActorCompanies(c ports.ActorCompanies) { s.companies = c }

// Write stores a record — the kernel's AuditSink. Over-long values are cut, not refused.
//
// A record the kernel files without a company (company 0: a sign-in or a reset by token in an
// app with several companies, made before any company is chosen) is installation-level: it is
// stored as company 0 and shown only on the installation-level trail. When it names a person
// and ActorCompanies is wired, a copy is also filed in each company that person is placed in,
// so those companies' auditors see their own people's sign-ins — and no other company's.
func (s *Service) Write(ctx context.Context, r kernel.AuditRecord) error {
	data := json.RawMessage("null")
	if len(r.Data) > 0 {
		b, err := json.Marshal(r.Data)
		if err != nil || len(b) > domain.MaxData {
			b, _ = json.Marshal(map[string]any{"note": "data too large or not JSON"})
		}
		data = b
	}
	kind := string(r.Kind)
	if kind == "" {
		kind = string(kernel.AuditChange)
	}
	e := domain.Entry{
		OccurredAt: r.At, ActorID: r.ActorID, Kind: domain.Cut(kind, 10), CompanyID: r.CompanyID, BranchID: r.BranchID,
		Module: domain.Cut(r.Module, domain.MaxModule), Action: domain.Cut(r.Action, domain.MaxAction),
		Target: domain.Cut(r.Target, domain.MaxTarget), Summary: domain.Cut(r.Summary, domain.MaxSummary),
		Data: data, IP: domain.Cut(r.IP, 45), RequestID: domain.Cut(r.RequestID, 64),
	}
	if err := s.repo.Insert(ctx, e); err != nil {
		return err
	}
	if e.CompanyID != 0 || e.ActorID == 0 || s.companies == nil {
		return nil
	}
	ids, err := s.companies.CompaniesOf(ctx, e.ActorID)
	if err != nil {
		return fmt.Errorf("installation-level record stored, its company copies not: %w", err) // the kernel logs it
	}
	slices.Sort(ids)
	for _, c := range slices.Compact(ids) {
		if c <= 0 {
			continue
		}
		copied := e
		copied.CompanyID, copied.BranchID = c, 0
		if err := s.repo.Insert(ctx, copied); err != nil {
			return err
		}
	}
	return nil
}

// Named is an entry with its actor's display name.
type Named struct {
	domain.Entry
	ActorName string
}

// target decides whose trail a reader sees: the installation's (company 0) for an installation
// auditor who asks for it, else the active company's — all of it, or only the branches the
// reader's permission reaches.
func target(ctx context.Context, r Reach) (company int64, branches []int64, err error) {
	if r.System {
		if !r.SystemAllowed {
			return 0, nil, ErrSystemDenied
		}
		return 0, nil, nil
	}
	company = kernel.ScopeFrom(ctx).CompanyID
	if r.AllBranches {
		return company, nil, nil
	}
	if len(r.Branches) == 0 {
		return 0, nil, ErrNoBranches
	}
	return company, r.Branches, nil
}

// List is the trail the reader may see, newest first, with each actor's name when the
// directory knows it.
func (s *Service) List(ctx context.Context, r Reach, f Filter, p pagination.Params) ([]Named, int64, error) {
	var err error
	if f.Company, f.Branches, err = target(ctx, r); err != nil {
		return nil, 0, err
	}
	// An open end is a real bound: MySQL compares a TIMESTAMP with Go's zero time ("0000-00-00")
	// as no match at all.
	if f.From.IsZero() {
		f.From = time.Date(1970, 1, 2, 0, 0, 0, 0, time.UTC)
	}
	if f.To.IsZero() {
		f.To = s.now().Add(time.Minute)
	}
	entries, total, err := s.repo.List(ctx, f, p)
	if err != nil {
		return nil, 0, err
	}
	names := map[int64]string{}
	if s.users != nil {
		var ids []int64
		seen := map[int64]bool{}
		for _, e := range entries {
			if e.ActorID != 0 && !seen[e.ActorID] {
				seen[e.ActorID] = true
				ids = append(ids, e.ActorID)
			}
		}
		if len(ids) > 0 {
			if got, err := s.users.Names(ctx, ids); err == nil {
				names = got
			}
		}
	}
	out := make([]Named, len(entries))
	for i, e := range entries {
		out[i] = Named{Entry: e, ActorName: names[e.ActorID]}
	}
	return out, total, nil
}

// Modules lists the modules that appear in the trail the reader may see.
func (s *Service) Modules(ctx context.Context, r Reach) ([]string, error) {
	company, _, err := target(ctx, r)
	if err != nil {
		return nil, err
	}
	return s.repo.Modules(ctx, company)
}

// Prune removes entries older than the retention period — the job the module schedules.
func (s *Service) Prune(ctx context.Context) error {
	before := s.now().Add(-s.retention)
	for range 20 { // at most 100 000 rows a run, in short statements
		n, err := s.repo.SystemDeleteOlderThan(ctx, before)
		if err != nil || n < 5000 || ctx.Err() != nil {
			return err
		}
	}
	return nil
}
