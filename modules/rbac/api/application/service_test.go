package application_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/kernel/kerneltest"
	"__GO_MODULE__/internal/modules/rbac/application"
	"__GO_MODULE__/internal/modules/rbac/domain"
	"__GO_MODULE__/internal/modules/rbac/ports"
	"__GO_MODULE__/internal/shared/apperror"
)

// memRepo is an in-memory Repository seeded like the migrations: company 1's Administrator
// holds "*". Roles belong to one company (Role.CompanyID).
type memRepo struct {
	roles   map[int64]domain.Role
	members map[int64]map[unit]bool // role → assignments
	nextID  int64
}

type unit struct{ user, company, branch int64 }

func newMemRepo() *memRepo {
	return &memRepo{
		roles:   map[int64]domain.Role{1: {ID: 1, CompanyID: 1, Name: domain.AdminRole, IsSystem: true, Permissions: []string{domain.Wildcard}}},
		members: map[int64]map[unit]bool{1: {}},
		nextID:  1,
	}
}

func (m *memRepo) ListRoles(_ context.Context, company int64) ([]domain.Role, error) {
	var out []domain.Role
	for id, r := range m.roles {
		if r.CompanyID != company {
			continue
		}
		r, _ := m.GetRole(context.Background(), company, id)
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsSystem != out[j].IsSystem {
			return out[i].IsSystem
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (m *memRepo) GetRole(_ context.Context, company, id int64) (domain.Role, error) {
	r, ok := m.roles[id]
	if !ok || r.CompanyID != company {
		return domain.Role{}, application.ErrNotFound
	}
	r.MemberCount = int64(len(m.members[id]))
	return r, nil
}

func (m *memRepo) RoleIDByName(_ context.Context, company int64, name string) (int64, error) {
	for id, r := range m.roles {
		if r.CompanyID == company && r.Name == name {
			return id, nil
		}
	}
	return 0, application.ErrNotFound
}

func (m *memRepo) CreateRole(_ context.Context, company int64, name, desc string, isSystem bool, perms []string) (int64, error) {
	for _, r := range m.roles {
		if r.CompanyID == company && r.Name == name {
			return 0, application.ErrNameTaken
		}
	}
	m.nextID++
	m.roles[m.nextID] = domain.Role{ID: m.nextID, CompanyID: company, Name: name, Description: desc, IsSystem: isSystem, Permissions: perms}
	m.members[m.nextID] = map[unit]bool{}
	return m.nextID, nil
}

func (m *memRepo) UpdateRole(_ context.Context, company, id int64, name, desc string, perms []string) error {
	r, ok := m.roles[id]
	if !ok || r.CompanyID != company {
		return application.ErrNotFound
	}
	r.Name, r.Description, r.Permissions = name, desc, perms
	m.roles[id] = r
	return nil
}

func (m *memRepo) DeleteRole(_ context.Context, company, id int64) error {
	if r, ok := m.roles[id]; !ok || r.CompanyID != company {
		return application.ErrNotFound
	}
	delete(m.roles, id)
	delete(m.members, id)
	return nil
}

func (m *memRepo) Members(_ context.Context, company, roleID int64) ([]application.Assignment, error) {
	var out []application.Assignment
	for u := range m.members[roleID] {
		if u.company == company {
			out = append(out, application.Assignment{UserID: u.user, BranchID: u.branch})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out, nil
}

// AddMember, as the table: a membership names a role of its own company.
func (m *memRepo) AddMember(_ context.Context, company, branch, roleID, userID int64) error {
	if r, ok := m.roles[roleID]; !ok || r.CompanyID != company {
		return application.ErrNotFound
	}
	m.members[roleID][unit{userID, company, branch}] = true
	return nil
}

func (m *memRepo) RemoveMember(_ context.Context, company, branch, roleID, userID int64) error {
	k := unit{userID, company, branch}
	if !m.members[roleID][k] {
		return application.ErrNotFound
	}
	delete(m.members[roleID], k)
	return nil
}

func (m *memRepo) LockMembers(ctx context.Context, company, roleID int64) ([]application.Assignment, error) {
	return m.Members(ctx, company, roleID)
}

func (m *memRepo) LockRole(_ context.Context, company, id int64, _ bool) error {
	if r, ok := m.roles[id]; !ok || r.CompanyID != company {
		return application.ErrNotFound
	}
	return nil
}

func (m *memRepo) RolesOfUser(_ context.Context, company, userID int64) ([]application.HeldRole, error) {
	var out []application.HeldRole
	for roleID, units := range m.members {
		for u := range units {
			if u.user == userID && u.company == company {
				r := m.roles[roleID]
				out = append(out, application.HeldRole{RoleID: roleID, Name: r.Name, IsSystem: r.IsSystem, BranchID: u.branch})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name || (out[i].Name == out[j].Name && out[i].BranchID < out[j].BranchID)
	})
	return out, nil
}

func (m *memRepo) SystemGrantsOfUser(_ context.Context, userID int64) ([]application.Grant, error) {
	var out []application.Grant
	for roleID, units := range m.members {
		for u := range units {
			if u.user == userID {
				for _, p := range m.roles[roleID].Permissions {
					out = append(out, application.Grant{CompanyID: u.company, BranchID: u.branch, Permission: p})
				}
			}
		}
	}
	return out, nil
}

// adminOf is the company's Administrator role, made like migration 004 / AppointFounder do.
func adminOf(t *testing.T, repo *memRepo, company int64) int64 {
	t.Helper()
	if id, err := repo.RoleIDByName(context.Background(), company, domain.AdminRole); err == nil {
		return id
	}
	id, err := repo.CreateRole(context.Background(), company, domain.AdminRole, domain.AdminDescription, true, []string{domain.Wildcard})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// directory is a fake UserDirectory: the module is tested with no users module at all.
type directory map[int64]ports.User

func (d directory) Get(_ context.Context, id int64) (ports.User, error) {
	u, ok := d[id]
	if !ok {
		return ports.User{}, ports.ErrUserNotFound
	}
	return u, nil
}

func (d directory) FindByEmail(_ context.Context, email string) (ports.User, error) {
	for _, u := range d {
		if u.Email == email {
			return u, nil
		}
	}
	return ports.User{}, ports.ErrUserNotFound
}

func (d directory) Search(_ context.Context, q string, _ int) ([]ports.User, error) {
	var out []ports.User
	for _, u := range d {
		if strings.Contains(u.Email, q) || strings.Contains(u.Name, q) {
			out = append(out, u)
		}
	}
	return out, nil
}

var catalog = func() []kernel.Permission {
	return []kernel.Permission{{Name: "rbac.manage"}, {Name: "users.manage"}, {Name: "users.read"}}
}

var people = directory{
	1: {ID: 1, Email: "admin@example.com", IsActive: true},
	2: {ID: 2, Email: "ana@example.com", IsActive: true},
	3: {ID: 3, Email: "budi@example.com", IsActive: true},
}

func setup() (*application.Service, *memRepo) {
	repo := newMemRepo()
	return application.NewService(repo, tx, everywhere{}, people, catalog, noAudit{}), repo
}

// fresh is a new service over repo — grants are cached, and these tests change repo directly.
func fresh(repo *memRepo) *application.Service {
	return application.NewService(repo, tx, everywhere{}, people, catalog, noAudit{})
}

// everywhere places every user in every company and branch (an organization module's answer).
type everywhere struct{}

func (everywhere) Resolve(_ context.Context, _ int64, requested string) (kernel.Scope, error) {
	var sc kernel.Scope
	c, b, _ := strings.Cut(requested, ":")
	sc.CompanyID, _ = strconv.ParseInt(c, 10, 64)
	sc.BranchID, _ = strconv.ParseInt(b, 10, 64)
	return sc, nil
}

// tx is the kernel's d.Tx without a database: fn runs, hooks follow.
var tx = kernel.NewTxSlot(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

// admin is the bootstrapped Administrator; tests act as them unless they say otherwise.
const admin = int64(1)

func setupWithAdmin(t *testing.T) (*application.Service, *memRepo) {
	t.Helper()
	svc, repo := setup()
	if ok, err := svc.Bootstrap(context.Background(), "admin@example.com"); err != nil || !ok {
		t.Fatalf("bootstrap: %v %v", ok, err)
	}
	return svc, repo
}

type noAudit struct{}

func (noAudit) Record(context.Context, kernel.AuditEntry) {}

// auditSpy hands records to the kernel's recorder (kerneltest) and counts the ones written
// outside the transaction — there must be none.
type auditSpy struct {
	kit     *kerneltest.Kit
	outside int
}

func (a *auditSpy) Record(ctx context.Context, e kernel.AuditEntry) {
	if !a.kit.Deps.Tx.InTx(ctx) {
		a.outside++
	}
	a.kit.Deps.Audit.Record(ctx, e)
}

// audited is a service on a kerneltest kernel: its d.Tx and its audit trail.
func audited(t *testing.T) (*application.Service, *memRepo, *kerneltest.Kit, *auditSpy) {
	t.Helper()
	k := kerneltest.Deps(t)
	repo := newMemRepo()
	spy := &auditSpy{kit: k}
	svc := application.NewService(repo, k.Deps.Tx, everywhere{}, people, catalog, spy)
	return svc, repo, k, spy
}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func TestBootstrapThenAdministratorCanEverything(t *testing.T) {
	svc, _ := setup()
	ctx := context.Background()
	if ok, err := svc.Bootstrap(ctx, "admin@example.com"); err != nil || !ok {
		t.Fatalf("bootstrap: %v %v", ok, err)
	}
	if ok, _ := svc.Bootstrap(ctx, "ana@example.com"); ok {
		t.Fatal("second bootstrap must be a no-op once someone is Administrator")
	}
	if !svc.Can(ctx, 1, "users.manage") || !svc.Can(ctx, 1, "anything.declared.later") {
		t.Fatal("Administrator's * must grant every permission")
	}
	if svc.Can(ctx, 2, "users.read") {
		t.Fatal("a user without roles must hold nothing")
	}
}

func TestBootstrapIsASecurityRecordInTheHomeCompanyInsideTheTransaction(t *testing.T) {
	svc, _, k, spy := audited(t)
	if ok, err := svc.Bootstrap(context.Background(), "admin@example.com"); err != nil || !ok {
		t.Fatalf("bootstrap: %v %v", ok, err)
	}
	recs := k.Audit.Records()
	if len(recs) != 1 || recs[0].Kind != kernel.AuditSecurity || recs[0].Action != "role.member_added" || recs[0].CompanyID != kernel.DefaultCompany {
		t.Fatalf("bootstrap audit = %+v", recs)
	}
	if recs[0].Summary != "Memberi peran Administrator saat mulai (bootstrap)" || recs[0].Data["email"] != "admin@example.com" || recs[0].ActorID != 0 {
		t.Fatalf("bootstrap record = %+v", recs[0])
	}
	if spy.outside != 0 {
		t.Fatal("the bootstrap record was written outside the transaction")
	}
	if _, _ = svc.Bootstrap(context.Background(), "ana@example.com"); len(k.Audit.Records()) != 1 {
		t.Fatal("a bootstrap that gives nothing records nothing")
	}
}

func TestEveryRoleAndMemberChangeIsASecurityRecordInsideTheTransaction(t *testing.T) {
	svc, _, k, spy := audited(t)
	ctx := context.Background()
	if ok, _ := svc.Bootstrap(ctx, "admin@example.com"); !ok {
		t.Fatal("bootstrap")
	}
	role, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateRole(ctx, admin, role.ID, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read", "users.manage"}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, admin, role.ID, 2, 5); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveMember(ctx, admin, role.ID, 2, 5); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRole(ctx, admin, role.ID); err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, r := range k.Audit.Records() {
		if r.Kind != kernel.AuditSecurity || r.Module != "rbac" {
			t.Fatalf("not a security record of rbac: %+v", r)
		}
		actions = append(actions, r.Action)
	}
	want := "role.member_added role.created role.updated role.member_added role.member_removed role.deleted"
	if got := strings.Join(actions, " "); got != want {
		t.Fatalf("actions = %q, want %q", got, want)
	}
	if spy.outside != 0 {
		t.Fatalf("%d records written outside their transaction", spy.outside)
	}
	upd := k.Audit.Records()[2]
	if upd.Data["permissions"].(map[string]any)["before"].([]string)[0] != "users.read" {
		t.Fatalf("role.updated keeps the before/after permissions: %+v", upd.Data)
	}
}

func TestRoleGrantsItsPermissionsOnly(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	role, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Viewer", Permissions: []string{"users.read", "users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(role.Permissions) != 1 {
		t.Fatalf("duplicates must collapse: %v", role.Permissions)
	}
	if err := svc.AddMember(ctx, admin, role.ID, 2, 0); err != nil {
		t.Fatal(err)
	}
	if !svc.Can(ctx, 2, "users.read") || svc.Can(ctx, 2, "users.manage") {
		t.Fatal("Viewer must read users and nothing more")
	}
	got, _ := svc.Effective(ctx, 2)
	if len(got) != 1 || got[0] != "users.read" {
		t.Fatalf("effective = %v", got)
	}
}

func TestUnknownPermissionAndReservedNameAreRejected(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	_, err := svc.CreateRole(context.Background(), admin, application.RoleInput{Name: "X", Permissions: []string{"nope.never"}})
	if status(t, err) != http.StatusUnprocessableEntity {
		t.Fatal("unknown permission must be a 422")
	}
	_, err = svc.CreateRole(context.Background(), admin, application.RoleInput{Name: "administrator"})
	if status(t, err) != http.StatusUnprocessableEntity {
		t.Fatal("the Administrator name is reserved")
	}
}

func TestAdministratorRoleIsProtected(t *testing.T) {
	svc, _ := setup()
	ctx := context.Background()
	_, _ = svc.Bootstrap(ctx, "admin@example.com")
	if _, err := svc.UpdateRole(ctx, admin, 1, application.RoleInput{Name: "Root"}); status(t, err) != http.StatusConflict {
		t.Error("Administrator must not change")
	}
	if err := svc.DeleteRole(ctx, admin, 1); status(t, err) != http.StatusConflict {
		t.Error("Administrator must not be deleted")
	}
	if err := svc.RemoveMember(ctx, admin, 1, 1, 0); status(t, err) != http.StatusConflict {
		t.Error("the last Administrator must keep the role")
	}
}

func TestChangesApplyImmediately(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	role, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Editor", Permissions: []string{"users.manage"}})
	_ = svc.AddMember(ctx, admin, role.ID, 2, 0)
	if !svc.Can(ctx, 2, "users.manage") {
		t.Fatal("granted")
	}
	if err := svc.RemoveMember(ctx, admin, role.ID, 2, 0); err != nil {
		t.Fatal(err)
	}
	if svc.Can(ctx, 2, "users.manage") {
		t.Fatal("the cache must not outlive a change")
	}
}

func TestUnknownUserCannotGetARole(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	if err := svc.AddMember(context.Background(), admin, 1, 99, 0); status(t, err) != http.StatusNotFound {
		t.Fatal("want 404 for a user the directory does not know")
	}
}

// A role manager holds rbac.manage and users.read — but not users.manage, and is no administrator.
func roleManager(t *testing.T, svc *application.Service) int64 {
	t.Helper()
	ctx := context.Background()
	role, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Role manager", Permissions: []string{"rbac.manage", "users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, admin, role.ID, 2, 0); err != nil {
		t.Fatal(err)
	}
	return 2
}

func TestRbacManageIsNoPathToMorePermissions(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	mgr := roleManager(t, svc)
	if err := svc.AddMember(ctx, mgr, 1, mgr, 0); status(t, err) != http.StatusForbidden {
		t.Errorf("making yourself Administrator: want 403, got %v", err)
	}
	if _, err := svc.CreateRole(ctx, mgr, application.RoleInput{Name: "Mine", Permissions: []string{"users.manage"}}); status(t, err) != http.StatusForbidden {
		t.Errorf("a role with a permission you lack: want 403, got %v", err)
	}
	editor, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Editor", Permissions: []string{"users.manage"}})
	if err := svc.AddMember(ctx, mgr, editor.ID, mgr, 0); status(t, err) != http.StatusForbidden {
		t.Errorf("joining a stronger role: want 403, got %v", err)
	}
	if _, err := svc.UpdateRole(ctx, mgr, editor.ID, application.RoleInput{Name: "Editor", Permissions: []string{"users.read"}}); status(t, err) != http.StatusForbidden {
		t.Errorf("editing a stronger role: want 403, got %v", err)
	}
	_ = svc.AddMember(ctx, admin, 1, 3, 0) // a second administrator
	if err := svc.RemoveMember(ctx, mgr, 1, 3, 0); status(t, err) != http.StatusForbidden {
		t.Errorf("demoting an administrator: want 403, got %v", err)
	}
	if svc.Can(ctx, mgr, "users.manage") {
		t.Fatal("the role manager gained users.manage")
	}
}

func TestCovers(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	mgr := roleManager(t, svc)
	if svc.Covers(ctx, mgr, admin) {
		t.Error("a role manager must not cover the administrator")
	}
	if !svc.Covers(ctx, admin, mgr) {
		t.Error("the administrator covers everyone")
	}
	if !svc.Covers(ctx, mgr, 3) {
		t.Error("someone with a company-wide role covers a user with no permissions")
	}
}

func TestABranchRoleReachesNoAccount(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	if err := svc.AddMember(ctx, admin, 1, 2, 5); err != nil { // Administrator in branch 5 only
		t.Fatal(err)
	}
	if svc.Covers(in(1, 5), 2, 3) {
		t.Fatal("a branch administrator may act on an account — accounts are app-wide")
	}
}

// nowhere places nobody beyond company 1 itself (an app without an organization module).
type nowhere struct{}

func (nowhere) Resolve(_ context.Context, _ int64, requested string) (kernel.Scope, error) {
	if requested == "1" {
		return kernel.Scope{CompanyID: 1}, nil
	}
	return kernel.Scope{}, kernel.ErrScopeDenied
}

func TestARoleGoesOnlyWhereTheUserIsPlaced(t *testing.T) {
	repo := newMemRepo()
	svc := application.NewService(repo, tx, nowhere{}, people, catalog, noAudit{})
	ctx := context.Background()
	if ok, _ := svc.Bootstrap(ctx, "admin@example.com"); !ok {
		t.Fatal("bootstrap")
	}
	if err := svc.AddMember(ctx, admin, 1, 2, 5); status(t, err) != http.StatusBadRequest {
		t.Fatalf("a role in a branch the user is not placed in: %v", err)
	}
	if err := svc.AddMember(ctx, admin, 1, 2, 0); err != nil {
		t.Fatalf("in the company they are placed in: %v", err)
	}
}

func TestBootstrapReturnsWhenNoActiveAdministratorIsLeft(t *testing.T) {
	repo := newMemRepo()
	users := directory{1: {ID: 1, Email: "admin@example.com", IsActive: true}}
	svc := application.NewService(repo, tx, everywhere{}, users, catalog, noAudit{})
	ctx := context.Background()
	if ok, _ := svc.Bootstrap(ctx, "admin@example.com"); !ok {
		t.Fatal("first bootstrap")
	}
	delete(users, 1) // the only administrator is gone (removed from the user store)
	users[2] = ports.User{ID: 2, Email: "new@example.com", IsActive: true}
	if ok, err := svc.Bootstrap(ctx, "new@example.com"); err != nil || !ok {
		t.Fatalf("bootstrap must give the role again when no active administrator is left: %v %v", ok, err)
	}
}

func in(company, branch int64) context.Context {
	return kernel.WithScope(context.Background(), kernel.Scope{CompanyID: company, BranchID: branch})
}

func TestBranchRoleReachesOnlyItsBranch(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	reader, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, admin, reader.ID, 2, 5); err != nil {
		t.Fatal(err)
	}
	if svc.Can(in(1, 0), 2, "users.read") || !svc.Can(in(1, 5), 2, "users.read") || svc.Can(in(1, 6), 2, "users.read") {
		t.Fatal("a branch role must reach its branch only — not the whole company, not another branch")
	}
	if u, _ := svc.Units(in(1, 0), 2, "users.read"); u.AllBranches || len(u.BranchIDs) != 1 || u.BranchIDs[0] != 5 {
		t.Fatalf("units at the company: %+v", u)
	}
	if u, _ := svc.Units(in(1, 0), admin, "users.read"); !u.AllBranches {
		t.Fatalf("a company-wide administrator reaches every branch: %+v", u)
	}
	if !svc.Can(in(2, 0), admin, "users.read") || svc.Can(in(2, 0), 2, "users.read") {
		t.Fatal("the home company's Administrator administers every company; a role in company 1 reaches only company 1")
	}
}

func TestBranchAdministratorIsNoCompanyAdministrator(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	if err := svc.AddMember(ctx, admin, 1, 2, 5); err != nil { // Administrator in branch 5
		t.Fatal(err)
	}
	if svc.Covers(ctx, 2, admin) {
		t.Fatal("a branch administrator may not act on a company-wide administrator")
	}
	if !svc.Covers(ctx, admin, 2) {
		t.Fatal("the company-wide administrator covers a branch administrator")
	}
	if err := svc.AddMember(ctx, 2, 1, 3, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("a branch administrator made someone company-wide Administrator: %v", err)
	}
	if err := svc.AddMember(ctx, 2, 1, 3, 5); err != nil {
		t.Fatalf("within their branch they may: %v", err)
	}
	if err := svc.RemoveMember(ctx, admin, 1, admin, 0); status(t, err) != http.StatusConflict {
		t.Fatalf("the last company-wide Administrator was removed: %v", err)
	}
}

func TestAnAccountIsCoveredInEveryCompany(t *testing.T) {
	svc, repo := setupWithAdmin(t) // user 1: Administrator of company 1 (home)
	ctx := context.Background()
	admin2, admin3 := adminOf(t, repo, 2), adminOf(t, repo, 3)
	_ = repo.AddMember(ctx, 2, 0, admin2, 2) // user 2: Administrator of company 2 only
	_ = repo.AddMember(ctx, 3, 0, admin3, 3) // user 3: Administrator of company 3 only
	svc = fresh(repo)
	if svc.Covers(in(2, 0), 2, 3) || svc.Covers(in(3, 0), 3, 2) {
		t.Fatal("one company's administrator may act on the account of another's")
	}
	if svc.Covers(in(2, 0), 2, admin) {
		t.Fatal("company 2's administrator may act on the account of the home company's")
	}
	if !svc.Covers(in(1, 0), admin, 2) || !svc.Covers(in(1, 0), admin, 3) {
		t.Fatal("the home company's Administrator administers the installation")
	}
	_ = repo.AddMember(ctx, 3, 0, admin3, 2)
	if !fresh(repo).Covers(in(2, 0), 2, 3) {
		t.Fatal("an administrator of both companies covers the other's administrator")
	}
}

func TestRolesBelongToOneCompany(t *testing.T) {
	svc, repo := setupWithAdmin(t)
	ctx := context.Background()
	reader, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = repo.AddMember(ctx, 2, 0, adminOf(t, repo, 2), 2) // Ana: Administrator of company 2 only
	svc = fresh(repo)
	inTwo := in(2, 0)
	if _, err := svc.GetRole(inTwo, reader.ID); status(t, err) != http.StatusNotFound {
		t.Fatalf("company 1's role seen from company 2: %v", err)
	}
	if roles, _ := svc.ListRoles(inTwo); len(roles) != 1 || !roles[0].IsSystem {
		t.Fatalf("company 2 lists %v, want only its own Administrator", roles)
	}
	if _, err := svc.UpdateRole(inTwo, 2, reader.ID, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.manage"}}); status(t, err) != http.StatusNotFound {
		t.Fatalf("changing company 1's role from company 2: %v", err)
	}
	if err := svc.DeleteRole(inTwo, 2, reader.ID); status(t, err) != http.StatusNotFound {
		t.Fatalf("deleting company 1's role from company 2: %v", err)
	}
	if err := svc.AddMember(inTwo, 2, reader.ID, 3, 0); status(t, err) != http.StatusNotFound {
		t.Fatalf("giving company 1's role in company 2: %v", err)
	}
	own, err := svc.CreateRole(inTwo, 2, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read"}})
	if err != nil || own.ID == reader.ID || own.CompanyID != 2 {
		t.Fatalf("company 2 defines its own Pembaca: %+v %v", own, err)
	}
	if _, err := svc.CreateRole(inTwo, 2, application.RoleInput{Name: "Pembaca"}); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("a name is unique within a company: %v", err)
	}
	if err := svc.AddMember(inTwo, 2, own.ID, 3, 0); err != nil {
		t.Fatal(err)
	}
	if !svc.Can(in(2, 0), 3, "users.read") || svc.Can(in(1, 0), 3, "users.read") {
		t.Fatal("company 2's Pembaca reads users in company 2 only")
	}
	if r, _ := svc.GetRole(in(1, 0), reader.ID); r.MemberCount != 0 {
		t.Fatal("company 1's Pembaca gained no member")
	}
}

func TestBranchManagersManageMembersOnlyInTheirBranch(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	mgr, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Pengelola", Permissions: []string{"rbac.manage", "users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	reader, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read"}})
	if err := svc.AddMember(ctx, admin, mgr.ID, 2, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRole(in(1, 5), 2, application.RoleInput{Name: "Baru", Permissions: []string{"users.read"}}); status(t, err) != http.StatusForbidden {
		t.Fatalf("a branch manager defined a role for the whole company: %v", err)
	}
	if err := svc.AddMember(in(1, 5), 2, reader.ID, 3, 5); err != nil {
		t.Fatalf("in their branch they may: %v", err)
	}
	if err := svc.AddMember(in(1, 5), 2, reader.ID, 3, 6); status(t, err) != http.StatusForbidden {
		t.Fatalf("a branch manager gave a role in another branch: %v", err)
	}
	if err := svc.AddMember(in(1, 5), 2, reader.ID, 3, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("a branch manager gave a role across the company: %v", err)
	}
}

func TestTheLastActiveAdministratorStays(t *testing.T) {
	repo := newMemRepo()
	users := directory{
		1: {ID: 1, Email: "admin@example.com", IsActive: true},
		2: {ID: 2, Email: "gone@example.com", IsActive: false},
	}
	svc := application.NewService(repo, tx, everywhere{}, users, catalog, noAudit{})
	ctx := context.Background()
	if ok, _ := svc.Bootstrap(ctx, "admin@example.com"); !ok {
		t.Fatal("bootstrap")
	}
	_ = repo.AddMember(ctx, 1, 0, 1, 2) // a deactivated administrator does not count
	if err := svc.RemoveMember(ctx, admin, 1, admin, 0); status(t, err) != http.StatusConflict {
		t.Fatalf("the last active administrator was removed: %v", err)
	}
	if err := svc.RemoveMember(ctx, admin, 1, 2, 0); err != nil {
		t.Fatalf("removing the deactivated one: %v", err)
	}
}

// placements is a fake organization module: user → companies.
type placements map[int64][]int64

func (p placements) PlacesOf(_ context.Context, user int64) ([]ports.Workplace, error) {
	var out []ports.Workplace
	for _, c := range p[user] {
		out = append(out, ports.Workplace{CompanyID: c})
	}
	return out, nil
}

func (p placements) PlacedUserIDs(_ context.Context, company int64) ([]int64, error) {
	var out []int64
	for u, cs := range p {
		for _, c := range cs {
			if c == company {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

func TestWithPlacementsARolelessAccountBelongsWhereItIsPlaced(t *testing.T) {
	svc, repo := setupWithAdmin(t)                                         // user 1: Administrator of company 1
	_ = repo.AddMember(context.Background(), 2, 0, adminOf(t, repo, 2), 2) // user 2: Administrator of company 2 only
	svc = fresh(repo)
	svc.UsePlacements(placements{1: {1, 2}, 2: {2}, 3: {1}})
	if svc.Covers(in(2, 0), 2, 3) {
		t.Fatal("company 2's administrator may act on an account placed only in company 1")
	}
	if !svc.Covers(in(1, 0), admin, 3) {
		t.Fatal("company 1's administrator covers its own roleless account")
	}
	if got, _ := svc.SearchUsers(in(2, 0), "example.com"); len(got) != 2 {
		t.Fatalf("company 2's member search lists %d users, want the 2 placed there", len(got))
	}
	svc.UsePlacements(placements{1: {1, 2}, 2: {2}, 3: {1, 2}})
	if svc.Covers(in(2, 0), 2, 3) {
		t.Fatal("an account placed in two companies is covered only with rights in both")
	}
}

func TestTheFounderOfACompanyGetsItsOwnAdministratorRole(t *testing.T) {
	svc, repo, k, spy := audited(t)
	ctx := context.Background()
	if ok, _ := svc.Bootstrap(ctx, "admin@example.com"); !ok {
		t.Fatal("bootstrap")
	}
	if err := svc.AppointFounder(ctx, 7, 3); err != nil {
		t.Fatal(err)
	}
	if err := svc.AppointFounder(ctx, 7, 3); err != nil {
		t.Fatalf("repeated: %v", err)
	}
	if !svc.Can(in(7, 0), 3, "users.manage") || svc.Can(in(1, 0), 3, "users.manage") || svc.Can(in(8, 0), 3, "users.manage") {
		t.Fatal("the founder is Administrator of their company — and only there")
	}
	roles, _ := svc.ListRoles(in(7, 0))
	if len(roles) != 1 || !roles[0].IsSystem || roles[0].CompanyID != 7 || roles[0].Name != domain.AdminRole || roles[0].MemberCount != 1 {
		t.Fatalf("company 7's roles = %+v, want its own Administrator with one member", roles)
	}
	if roles[0].ID == 1 {
		t.Fatal("company 7 was given company 1's Administrator role")
	}
	if ones, _ := svc.ListRoles(in(1, 0)); len(ones) != 1 || ones[0].MemberCount != 1 {
		t.Fatalf("company 1's roles changed: %+v", ones)
	}
	// Once: the role made, the founder appointed — a repeated delivery records nothing more.
	var got []string
	for _, r := range k.Audit.Records()[1:] {
		if r.Kind != kernel.AuditSecurity || r.CompanyID != 7 {
			t.Fatalf("recorded outside company 7 or not as security: %+v", r)
		}
		got = append(got, r.Action)
	}
	if strings.Join(got, " ") != "role.created role.member_added" || spy.outside != 0 {
		t.Fatalf("audit = %v (outside tx: %d)", got, spy.outside)
	}
	// A company whose Administrator role already exists (migration 004 made it) keeps it.
	existing := adminOf(t, repo, 9)
	if err := svc.AppointFounder(ctx, 9, 2); err != nil {
		t.Fatal(err)
	}
	if id, _ := repo.RoleIDByName(ctx, 9, domain.AdminRole); id != existing {
		t.Fatal("the existing Administrator role of company 9 was not reused")
	}
	if m, _ := repo.Members(ctx, 9, existing); len(m) != 1 || m[0].UserID != 2 {
		t.Fatalf("company 9's administrators = %+v", m)
	}
}

func TestARoleElsewhereNeverHandsAnAccountToThatCompany(t *testing.T) {
	svc, repo := setupWithAdmin(t) // admin: Administrator of company 1
	ctx := context.Background()
	_ = repo.AddMember(ctx, 2, 0, adminOf(t, repo, 2), 2) // Ana: Administrator of company 2 only
	viewer, _ := repo.CreateRole(ctx, 2, "Pembaca", "", false, []string{"users.read"})
	_ = repo.AddMember(ctx, 2, 0, viewer, 3) // Budi: a role in company 2 …
	svc = fresh(repo)
	svc.UsePlacements(placements{1: {1}, 2: {2}, 3: {1, 2}}) // … and works in both companies
	if svc.Covers(in(2, 0), 2, 3) {
		t.Fatal("a role in company 2 handed company 2 an account that also works in company 1")
	}
	if !svc.Covers(in(1, 0), admin, 3) {
		t.Fatal("the home company's Administrator covers anyone")
	}
}

func TestACompanysFounderStaysWithinTheHomeAdministratorsReach(t *testing.T) {
	svc, repo := setupWithAdmin(t)
	if err := svc.AppointFounder(context.Background(), 7, 2); err != nil { // Ana created company 7
		t.Fatal(err)
	}
	svc = fresh(repo)
	svc.UsePlacements(placements{2: {1, 7}})
	if !svc.Covers(in(1, 0), admin, 2) {
		t.Fatal("creating a company made its founder untouchable by the home company's Administrator")
	}
}

func TestABranchAdministratorReachesAccountsThatLiveInTheBranch(t *testing.T) {
	svc, repo := setupWithAdmin(t)
	_ = repo.AddMember(context.Background(), 1, 5, 1, 2) // Ana: Administrator of branch 5
	svc = fresh(repo)
	svc.UsePlacements(placementsAt{3: {{CompanyID: 1, BranchID: 5}}, 4: {{CompanyID: 1}}})
	if !svc.Covers(in(1, 5), 2, 3) {
		t.Fatal("a branch administrator covers someone who works only in that branch")
	}
	if svc.Covers(in(1, 5), 2, 4) {
		t.Fatal("a branch administrator covers someone working across the company")
	}
}

// placementsAt is a fake organization module with branches: user → workplaces.
type placementsAt map[int64][]ports.Workplace

func (p placementsAt) PlacesOf(_ context.Context, user int64) ([]ports.Workplace, error) {
	return p[user], nil
}

func (p placementsAt) PlacedUserIDs(context.Context, int64) ([]int64, error) { return nil, nil }

func TestRoleHoldersAnswerForTheCompanyOfTheScope(t *testing.T) {
	svc, repo := setupWithAdmin(t)
	ctx := context.Background()
	kasir, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Kasir", Permissions: []string{"users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = svc.AddMember(ctx, admin, kasir.ID, 2, 0) // Ana: Kasir across company 1
	_ = svc.AddMember(ctx, admin, kasir.ID, 3, 5) // Budi: Kasir in branch 5
	_ = repo.AddMember(ctx, 2, 0, adminOf(t, repo, 2), 3)
	svc = fresh(repo)
	holders, err := svc.Holders(in(1, 0), "Kasir")
	if err != nil || len(holders) != 2 || holders[0].UserID != 2 || holders[0].BranchID != 0 || holders[1].UserID != 3 || holders[1].BranchID != 5 {
		t.Fatalf("holders of Kasir = %+v %v", holders, err)
	}
	if _, err := svc.Holders(in(2, 0), "Kasir"); err != application.ErrNotFound {
		t.Fatalf("company 2 has no Kasir: %v", err)
	}
	if names, _ := svc.RoleNamesOf(in(1, 0), 3); len(names) != 0 {
		t.Fatalf("at the whole company Budi holds nothing: %v", names)
	}
	if names, _ := svc.RoleNamesOf(in(1, 5), 3); len(names) != 1 || names[0] != "Kasir" {
		t.Fatalf("in branch 5 Budi is Kasir: %v", names)
	}
	if names, _ := svc.RoleNamesOf(in(1, 6), 2); len(names) != 1 || names[0] != "Kasir" {
		t.Fatalf("a company-wide role applies in every branch: %v", names)
	}
	if names, _ := svc.RoleNamesOf(in(2, 0), 3); len(names) != 1 || names[0] != domain.AdminRole {
		t.Fatalf("Budi in company 2: %v", names)
	}
	if names, _ := svc.RoleNames(in(1, 0)); strings.Join(names, ",") != "Administrator,Kasir" {
		t.Fatalf("company 1's roles = %v", names)
	}
	if names, _ := svc.RoleNames(in(2, 0)); strings.Join(names, ",") != "Administrator" {
		t.Fatalf("company 2's roles = %v", names)
	}
}

func TestUnplacingRevokesTheRolesHeldInThatUnitOnly(t *testing.T) {
	svc, repo, k, spy := audited(t)
	ctx := context.Background()
	if ok, _ := svc.Bootstrap(ctx, "admin@example.com"); !ok {
		t.Fatal("bootstrap")
	}
	kasir, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Kasir", Permissions: []string{"users.read"}})
	gudang, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Gudang", Permissions: []string{"users.read"}})
	_ = svc.AddMember(ctx, admin, kasir.ID, 3, 5)  // Budi: Kasir in branch 5 …
	_ = svc.AddMember(ctx, admin, gudang.ID, 3, 5) // … and Gudang there
	_ = svc.AddMember(ctx, admin, kasir.ID, 3, 6)  // Kasir in branch 6
	_ = svc.AddMember(ctx, admin, gudang.ID, 3, 0) // Gudang across the company
	admin2 := adminOf(t, repo, 2)
	_ = repo.AddMember(ctx, 2, 0, admin2, 3) // and Administrator of company 2
	before := len(k.Audit.Records())
	if err := svc.RevokeAt(ctx, 3, 1, 5); err != nil {
		t.Fatal(err)
	}
	held, _ := repo.RolesOfUser(ctx, 1, 3)
	if len(held) != 2 || held[0].Name != "Gudang" || held[0].BranchID != 0 || held[1].Name != "Kasir" || held[1].BranchID != 6 {
		t.Fatalf("after unplacing from branch 5 Budi holds %+v", held)
	}
	if !svc.Can(in(2, 0), 3, "users.manage") {
		t.Fatal("company 2's Administrator role was touched")
	}
	recs := k.Audit.Records()[before:]
	if len(recs) != 2 || spy.outside != 0 {
		t.Fatalf("records = %+v (outside tx: %d)", recs, spy.outside)
	}
	for _, r := range recs {
		if r.Kind != kernel.AuditSecurity || r.Action != "role.member_removed" || r.CompanyID != 1 || r.BranchID != 5 || !strings.Contains(r.Summary, "karena penempatan di cabang #5 dicabut") {
			t.Fatalf("record = %+v", r)
		}
	}
	// Nothing held there: nothing happens, nothing is recorded.
	if err := svc.RevokeAt(ctx, 3, 1, 5); err != nil || len(k.Audit.Records()) != before+2 {
		t.Fatalf("a repeated unplacing: %v, %d records", err, len(k.Audit.Records()))
	}
	// Across the company: the company-wide roles go, the branches' stay.
	if err := svc.RevokeAt(ctx, 3, 1, 0); err != nil {
		t.Fatal(err)
	}
	if held, _ := repo.RolesOfUser(ctx, 1, 3); len(held) != 1 || held[0].BranchID != 6 {
		t.Fatalf("after unplacing from the company Budi holds %+v", held)
	}
	if svc.Can(in(1, 0), 3, "users.read") || !svc.Can(in(1, 6), 3, "users.read") {
		t.Fatal("the cache outlived the revocation")
	}
	// Even the last Administrator: the person can no longer work there.
	if err := svc.RevokeAt(ctx, 3, 2, 0); err != nil {
		t.Fatal(err)
	}
	if m, _ := repo.Members(ctx, 2, admin2); len(m) != 0 {
		t.Fatalf("company 2's administrators = %+v", m)
	}
}
