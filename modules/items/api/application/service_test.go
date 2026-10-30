package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/kernel/kerneltest"
	"__GO_MODULE__/internal/modules/items/application"
	"__GO_MODULE__/internal/modules/items/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/validator"
)

// memRepo keeps every company's rows in memory, keyed like the tables.
type memRepo struct {
	items      map[int64]domain.Item
	units      map[int64]domain.Unit
	categories map[int64]domain.Category
	itemUnits  map[int64]domain.ItemUnit
	settings   map[int64]application.Settings
	nextID     int64

	lastFilter  application.ListFilter
	lastPattern string
	lastLimit   int
}

func newMemRepo() *memRepo {
	return &memRepo{items: map[int64]domain.Item{}, units: map[int64]domain.Unit{}, categories: map[int64]domain.Category{},
		itemUnits: map[int64]domain.ItemUnit{}, settings: map[int64]application.Settings{}}
}

func (m *memRepo) id() int64 { m.nextID++; return m.nextID }

// unlike reads a LIKE pattern the way MySQL does with its default escape character: what it
// matches literally.
func unlike(p string) string {
	return strings.NewReplacer(`\\`, `\`, `\%`, `%`, `\_`, `_`).Replace(p)
}

func (m *memRepo) List(_ context.Context, co int64, f application.ListFilter) ([]domain.Item, int64, error) {
	m.lastFilter = f
	var out []domain.Item
	for _, it := range m.items {
		if it.CompanyID != co || (f.Status == "active" && !it.IsActive) || (f.Status == "inactive" && it.IsActive) ||
			(f.Kind != "" && string(it.Kind) != f.Kind) || (f.CategoryID != 0 && it.CategoryID != f.CategoryID) {
			continue
		}
		lit := unlike(f.Pattern)
		if f.Pattern != "" && !strings.Contains(it.Code, lit) && !strings.Contains(it.Name, lit) && (it.Barcode == "" || it.Barcode != f.Search) {
			continue
		}
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, int64(len(out)), nil
}

func (m *memRepo) Search(_ context.Context, co int64, search, pattern string, limit int) ([]domain.Item, error) {
	m.lastPattern, m.lastLimit = pattern, limit
	lit := unlike(pattern)
	var out []domain.Item
	for _, it := range m.items {
		if it.CompanyID == co && it.IsActive && (strings.HasPrefix(it.Code, lit) || strings.Contains(it.Name, lit) || (it.Barcode != "" && it.Barcode == search)) {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out[:min(limit, len(out))], nil
}

func (m *memRepo) Get(_ context.Context, co, id int64) (domain.Item, error) {
	it, ok := m.items[id]
	if !ok || it.CompanyID != co {
		return domain.Item{}, application.ErrNotFound
	}
	return it, nil
}

func (m *memRepo) GetShared(ctx context.Context, co, id int64) (domain.Item, error) {
	return m.Get(ctx, co, id)
}

func (m *memRepo) GetForUpdate(ctx context.Context, co, id int64) (domain.Item, error) {
	return m.Get(ctx, co, id)
}

func (m *memRepo) CodeTaken(_ context.Context, co int64, code string) (bool, error) {
	for _, it := range m.items {
		if it.CompanyID == co && it.Code == code {
			return true, nil
		}
	}
	return false, nil
}

func (m *memRepo) BarcodeOwner(_ context.Context, co int64, barcode string, except int64) (string, error) {
	for _, it := range m.items {
		if it.CompanyID == co && it.Barcode == barcode && it.ID != except {
			return it.Code, nil
		}
	}
	return "", nil
}

// stored is an item as the table keeps it: the display fields are not columns.
func stored(it domain.Item) domain.Item {
	it.BaseUnitCode, it.BaseUnitName, it.CategoryName = "", "", ""
	return it
}

func (m *memRepo) unique(it domain.Item) error {
	for _, o := range m.items {
		if o.CompanyID != it.CompanyID || o.ID == it.ID {
			continue
		}
		if o.Code == it.Code {
			return application.ErrCodeTaken
		}
		if it.Barcode != "" && o.Barcode == it.Barcode {
			return application.ErrBarcodeTaken
		}
	}
	return nil
}

func (m *memRepo) Insert(_ context.Context, it domain.Item) (int64, error) {
	if err := m.unique(it); err != nil {
		return 0, err
	}
	it.ID = m.id()
	m.items[it.ID] = stored(it)
	return it.ID, nil
}

func (m *memRepo) Update(_ context.Context, it domain.Item) error {
	if err := m.unique(it); err != nil {
		return err
	}
	before := m.items[it.ID]
	// Kind, is_stocked, and base_unit_id are not written by the UPDATE.
	it.Kind, it.IsStocked, it.BaseUnitID = before.Kind, before.IsStocked, before.BaseUnitID
	m.items[it.ID] = stored(it)
	return nil
}

func (m *memRepo) SetActive(_ context.Context, _, id int64, active bool, by int64, at time.Time) error {
	it := m.items[id]
	it.IsActive, it.UpdatedBy, it.UpdatedAt = active, by, at
	m.items[id] = it
	return nil
}

func (m *memRepo) Units(_ context.Context, co int64) ([]domain.Unit, error) {
	var out []domain.Unit
	for _, u := range m.units {
		if u.CompanyID == co {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (m *memRepo) Unit(_ context.Context, co, id int64) (domain.Unit, error) {
	u, ok := m.units[id]
	if !ok || u.CompanyID != co {
		return domain.Unit{}, application.ErrNotFound
	}
	return u, nil
}

func (m *memRepo) UnitForUpdate(ctx context.Context, co, id int64) (domain.Unit, error) {
	return m.Unit(ctx, co, id)
}

func (m *memRepo) UnitByCode(_ context.Context, co int64, code string) (domain.Unit, error) {
	for _, u := range m.units {
		if u.CompanyID == co && u.Code == code {
			return u, nil
		}
	}
	return domain.Unit{}, application.ErrNotFound
}

func (m *memRepo) InsertUnit(ctx context.Context, u domain.Unit) (int64, error) {
	if _, err := m.UnitByCode(ctx, u.CompanyID, u.Code); err == nil {
		return 0, application.ErrCodeTaken
	}
	u.ID = m.id()
	m.units[u.ID] = u
	return u.ID, nil
}

func (m *memRepo) InsertUnitIfMissing(ctx context.Context, u domain.Unit) (bool, error) {
	_, err := m.InsertUnit(ctx, u)
	if errors.Is(err, application.ErrCodeTaken) {
		return false, nil
	}
	return err == nil, err
}

func (m *memRepo) UpdateUnit(_ context.Context, u domain.Unit) error {
	m.units[u.ID] = u
	return nil
}

func (m *memRepo) Categories(_ context.Context, co int64) ([]domain.Category, error) {
	var out []domain.Category
	for _, c := range m.categories {
		if c.CompanyID == co {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *memRepo) Category(_ context.Context, co, id int64) (domain.Category, error) {
	c, ok := m.categories[id]
	if !ok || c.CompanyID != co {
		return domain.Category{}, application.ErrNotFound
	}
	return c, nil
}

func (m *memRepo) CategoryForUpdate(ctx context.Context, co, id int64) (domain.Category, error) {
	return m.Category(ctx, co, id)
}

func (m *memRepo) nameTaken(c domain.Category) bool {
	for _, o := range m.categories {
		if o.CompanyID == c.CompanyID && o.Name == c.Name && o.ID != c.ID {
			return true
		}
	}
	return false
}

func (m *memRepo) InsertCategory(_ context.Context, c domain.Category) (int64, error) {
	if m.nameTaken(c) {
		return 0, application.ErrNameTaken
	}
	c.ID = m.id()
	m.categories[c.ID] = c
	return c.ID, nil
}

func (m *memRepo) UpdateCategory(_ context.Context, c domain.Category) error {
	if m.nameTaken(c) {
		return application.ErrNameTaken
	}
	m.categories[c.ID] = c
	return nil
}

func (m *memRepo) withUnit(iu domain.ItemUnit) domain.ItemUnit {
	u := m.units[iu.UnitID]
	iu.UnitCode, iu.UnitName = u.Code, u.Name
	return iu
}

func (m *memRepo) ItemUnits(_ context.Context, co, item int64) ([]domain.ItemUnit, error) {
	var out []domain.ItemUnit
	for _, iu := range m.itemUnits {
		if iu.CompanyID == co && iu.ItemID == item {
			out = append(out, m.withUnit(iu))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsActive != out[j].IsActive {
			return out[i].IsActive
		}
		return out[i].Factor.Cmp(out[j].Factor) < 0
	})
	return out, nil
}

func (m *memRepo) ItemUnit(_ context.Context, co, item, id int64) (domain.ItemUnit, error) {
	iu, ok := m.itemUnits[id]
	if !ok || iu.CompanyID != co || iu.ItemID != item {
		return domain.ItemUnit{}, application.ErrNotFound
	}
	return m.withUnit(iu), nil
}

func (m *memRepo) ItemUnitOf(_ context.Context, co, item, unit int64) (domain.ItemUnit, error) {
	for _, iu := range m.itemUnits {
		if iu.CompanyID == co && iu.ItemID == item && iu.UnitID == unit {
			return iu, nil
		}
	}
	return domain.ItemUnit{}, application.ErrNotFound
}

func (m *memRepo) InsertItemUnit(_ context.Context, iu domain.ItemUnit) (int64, error) {
	iu.ID = m.id()
	iu.UnitCode, iu.UnitName = "", ""
	m.itemUnits[iu.ID] = iu
	return iu.ID, nil
}

func (m *memRepo) SetItemUnitActive(_ context.Context, _, _, id int64, active bool) error {
	iu := m.itemUnits[id]
	iu.IsActive = active
	m.itemUnits[id] = iu
	return nil
}

func (m *memRepo) Settings(_ context.Context, co int64) (application.Settings, error) {
	if s, ok := m.settings[co]; ok {
		return s, nil
	}
	return application.Settings{CodePrefix: application.DefaultPrefix}, nil
}

func (m *memRepo) LockSettings(ctx context.Context, co int64, _ time.Time) (application.Settings, error) {
	s, _ := m.Settings(ctx, co)
	m.settings[co] = s
	return s, nil
}

func (m *memRepo) SetLastNumber(_ context.Context, co, n int64) error {
	s := m.settings[co]
	s.LastNumber = n
	m.settings[co] = s
	return nil
}

func (m *memRepo) SetPrefix(_ context.Context, co int64, prefix string, _ time.Time) error {
	s := m.settings[co]
	s.CodePrefix = prefix
	m.settings[co] = s
	return nil
}

type authz map[string]kernel.Units

func (a authz) Units(_ context.Context, p string) (kernel.Units, error) { return a[p], nil }

type signedIn struct{}

func (signedIn) UserID(context.Context) (int64, bool) { return 7, true }

var all = kernel.Units{AllBranches: true}

func setup(t *testing.T, perms authz) (*application.Service, *memRepo, *kerneltest.Kit, context.Context) {
	t.Helper()
	k := kerneltest.Deps(t)
	repo := newMemRepo()
	svc := application.NewService(repo, k.Deps.Tx, k.Deps.Events, k.Deps.Audit, perms, signedIn{}, k.Clock)
	return svc, repo, k, kerneltest.In(context.Background(), 1, 0)
}

func manager() authz { return authz{application.PermManage: all} }

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func fields(t *testing.T, err error) map[string]string {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok || e.Status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %v", err)
	}
	out := map[string]string{}
	for _, f := range e.Details.([]validator.FieldError) {
		out[f.Field] = f.Message
	}
	return out
}

func unit(t *testing.T, svc *application.Service, ctx context.Context, code string) domain.Unit {
	t.Helper()
	u, err := svc.CreateUnit(ctx, domain.Unit{Code: code, Name: code})
	if err != nil {
		t.Fatalf("CreateUnit(%s): %v", code, err)
	}
	return u
}

func goods(name string, base domain.Unit) application.Input {
	return application.Input{Item: domain.Item{Name: name, Kind: domain.Goods, BaseUnitID: base.ID}}
}

func create(t *testing.T, svc *application.Service, ctx context.Context, in application.Input) domain.Item {
	t.Helper()
	it, err := svc.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create(%s): %v", in.Item.Name, err)
	}
	return it
}

func ptr[T any](v T) *T { return &v }

func TestCreateGeneratesCodesAndSkipsTypedOnes(t *testing.T) {
	svc, _, k, ctx := setup(t, manager())
	pcs := unit(t, svc, ctx, "PCS")
	if it := create(t, svc, ctx, goods("Pensil", pcs)); it.Code != "BRG-00001" || it.BaseUnitCode != "PCS" {
		t.Errorf("first item = %s, base %q", it.Code, it.BaseUnitCode)
	}
	typed := goods("Pulpen", pcs)
	typed.Item.Code = " brg-00002 "
	if it := create(t, svc, ctx, typed); it.Code != "BRG-00002" {
		t.Errorf("typed code = %q; want it trimmed, in capitals", it.Code)
	}
	if it := create(t, svc, ctx, goods("Penghapus", pcs)); it.Code != "BRG-00003" {
		t.Errorf("a generated code passes over a typed one: %s", it.Code)
	}
	if _, err := svc.Create(ctx, typed); fields(t, err)["code"] == "" {
		t.Errorf("a code in use: %v", err)
	}
	bad := goods("Spidol", pcs)
	bad.Item.Code = "A B"
	if _, err := svc.Create(ctx, bad); fields(t, err)["code"] == "" {
		t.Errorf("a code with a space: %v", err)
	}
	if err := svc.SetPrefix(ctx, "atk/"); err != nil {
		t.Fatal(err)
	}
	if it := create(t, svc, ctx, goods("Spidol", pcs)); it.Code != "ATK/00004" {
		t.Errorf("after a prefix change = %s", it.Code)
	}
	if st, _ := svc.Settings(ctx); st.CodePrefix != "ATK/" || st.LastNumber != 4 {
		t.Errorf("settings = %+v", st)
	}
	// Another company has its own counter and units.
	co2 := kerneltest.In(context.Background(), 2, 0)
	if it := create(t, svc, co2, goods("Kertas", unit(t, svc, co2, "PCS"))); it.Code != "BRG-00001" {
		t.Errorf("company 2 = %s", it.Code)
	}
	if n := len(k.Audit.Records()); n == 0 {
		t.Error("no audit records")
	}
}

func TestThePrefixNeedsManageAcrossTheCompany(t *testing.T) {
	svc, _, _, ctx := setup(t, authz{application.PermManage: {BranchIDs: []int64{3}}})
	if err := svc.SetPrefix(ctx, "X-"); err == nil || status(t, err) != http.StatusForbidden {
		t.Errorf("a branch-scoped manager changed the prefix: %v", err)
	}
	if err := svc.SetPrefix(ctx, "TOO-LONG-PREFIX"); fields(t, err)["code_prefix"] == "" {
		t.Errorf("a long prefix: %v", err)
	}
}

func TestCreateChecksKindStockUnitsAndCodes(t *testing.T) {
	svc, _, _, ctx := setup(t, manager())
	pcs := unit(t, svc, ctx, "PCS")
	jam := unit(t, svc, ctx, "JAM")
	if it := create(t, svc, ctx, goods("Pensil", pcs)); !it.IsStocked || it.TaxCode != domain.NoTaxCode {
		t.Errorf("goods default: stocked %v, tax code %q", it.IsStocked, it.TaxCode)
	}
	notTracked := goods("Kantong plastik", pcs)
	notTracked.IsStocked = ptr(false)
	if it := create(t, svc, ctx, notTracked); it.IsStocked {
		t.Error("goods asked not to be tracked are tracked")
	}
	svcIn := application.Input{Item: domain.Item{Name: "Servis AC", Kind: domain.Service, BaseUnitID: jam.ID, TaxCode: "250100"}}
	if it := create(t, svc, ctx, svcIn); it.IsStocked || it.Kind != domain.Service || it.TaxCode != "250100" {
		t.Errorf("a service = %+v", it)
	}
	svcIn.IsStocked = ptr(true)
	if _, err := svc.Create(ctx, svcIn); fields(t, err)["is_stocked"] == "" {
		t.Errorf("a stocked service: %v", err)
	}

	off, err := svc.UpdateUnit(ctx, unit(t, svc, ctx, "LAMA").ID, application.UnitPatch{IsActive: ptr(false)})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := svc.CreateCategory(ctx, domain.Category{Name: "Lama"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateCategory(ctx, cat.ID, application.CategoryPatch{IsActive: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	other := unit(t, svc, kerneltest.In(context.Background(), 2, 0), "KG")
	for name, tc := range map[string]struct {
		in    application.Input
		field string
	}{
		"no kind":             {application.Input{Item: domain.Item{Name: "X", BaseUnitID: pcs.ID}}, "kind"},
		"no name":             {application.Input{Item: domain.Item{Kind: domain.Goods, BaseUnitID: pcs.ID}}, "name"},
		"no base unit":        {application.Input{Item: domain.Item{Name: "X", Kind: domain.Goods}}, "base_unit_id"},
		"inactive base unit":  {goods("X", off), "base_unit_id"},
		"another's unit":      {goods("X", other), "base_unit_id"},
		"inactive category":   {application.Input{Item: domain.Item{Name: "X", Kind: domain.Goods, BaseUnitID: pcs.ID, CategoryID: cat.ID}}, "category_id"},
		"unknown category":    {application.Input{Item: domain.Item{Name: "X", Kind: domain.Goods, BaseUnitID: pcs.ID, CategoryID: 999}}, "category_id"},
		"tax code of 5":       {application.Input{Item: domain.Item{Name: "X", Kind: domain.Goods, BaseUnitID: pcs.ID, TaxCode: "12345"}}, "tax_code"},
		"short barcode":       {application.Input{Item: domain.Item{Name: "X", Kind: domain.Goods, BaseUnitID: pcs.ID, Barcode: "1234567"}}, "barcode"},
		"barcode with spaces": {application.Input{Item: domain.Item{Name: "X", Kind: domain.Goods, BaseUnitID: pcs.ID, Barcode: "8991 2345 678"}}, "barcode"},
	} {
		if _, err := svc.Create(ctx, tc.in); err == nil || fields(t, err)[tc.field] == "" {
			t.Errorf("%s: want a %s error, got %v", name, tc.field, err)
		}
	}
}

func TestFixedFieldsAreRefused(t *testing.T) {
	svc, _, k, ctx := setup(t, manager())
	pcs := unit(t, svc, ctx, "PCS")
	kg := unit(t, svc, ctx, "KG")
	it := create(t, svc, ctx, goods("Beras", kg))
	n := len(k.Published())
	for name, tc := range map[string]struct {
		p     application.Patch
		field string
	}{
		"kind":       {application.Patch{Kind: ptr("service")}, "kind"},
		"is_stocked": {application.Patch{IsStocked: ptr(false)}, "is_stocked"},
		"base unit":  {application.Patch{BaseUnitID: ptr(pcs.ID)}, "base_unit_id"},
	} {
		msg := ""
		if _, err := svc.Update(ctx, it.ID, tc.p); err != nil {
			msg = fields(t, err)[tc.field]
		}
		if !strings.Contains(msg, "tidak bisa diubah") {
			t.Errorf("changing %s: message %q", name, msg)
		}
	}
	// Sending them unchanged — a form sends every field — is fine.
	got, err := svc.Update(ctx, it.ID, application.Patch{Kind: ptr("goods"), IsStocked: ptr(true), BaseUnitID: ptr(kg.ID), Name: ptr("Beras Pandan")})
	if err != nil || got.Name != "Beras Pandan" || got.BaseUnitCode != "KG" {
		t.Errorf("unchanged fixed fields: %+v %v", got, err)
	}
	if got := len(k.Published()) - n; got != 1 {
		t.Errorf("refused changes and one real one published %d events", got)
	}
}

func TestUpdateChangesOnlyWhatIsSent(t *testing.T) {
	svc, repo, k, ctx := setup(t, manager())
	pcs := unit(t, svc, ctx, "PCS")
	cat, err := svc.CreateCategory(ctx, domain.Category{Name: "ATK"})
	if err != nil {
		t.Fatal(err)
	}
	in := goods("Pensil 2B", pcs)
	in.Item.CategoryID, in.Item.Barcode, in.Item.TaxCode, in.Item.Description = cat.ID, "8991234567890", "480200", "Kayu"
	it := create(t, svc, ctx, in)

	got, err := svc.Update(ctx, it.ID, application.Patch{Name: ptr("Pensil 2B Faber")})
	if err != nil {
		t.Fatal(err)
	}
	want := stored(it)
	want.Name, want.UpdatedAt = "Pensil 2B Faber", got.UpdatedAt
	if s := repo.items[it.ID]; s != want {
		t.Errorf("a name-only PUT changed more:\n got  %+v\n want %+v", s, want)
	}
	if got.CategoryName != "ATK" {
		t.Errorf("category name = %q", got.CategoryName)
	}

	// Clearing: barcode "" and category 0; tax code "" is 000000.
	got, err = svc.Update(ctx, it.ID, application.Patch{Barcode: ptr(""), CategoryID: ptr(int64(0)), TaxCode: ptr("")})
	if err != nil || got.Barcode != "" || got.CategoryID != 0 || got.TaxCode != domain.NoTaxCode || got.Description != "Kayu" {
		t.Errorf("cleared = %+v %v", got, err)
	}

	// A PUT that changes nothing records and announces nothing.
	audits, events := len(k.Audit.Records()), len(k.Published())
	if _, err := svc.Update(ctx, it.ID, application.Patch{Name: ptr(" Pensil 2B Faber ")}); err != nil {
		t.Fatal(err)
	}
	if len(k.Audit.Records()) != audits || len(k.Published()) != events {
		t.Error("an update that changed nothing was recorded or announced")
	}
	if _, err := svc.Update(ctx, it.ID, application.Patch{Code: ptr("")}); fields(t, err)["code"] == "" {
		t.Errorf("an empty code on update: %v", err)
	}
	if _, err := svc.Update(kerneltest.In(context.Background(), 2, 0), it.ID, application.Patch{Name: ptr("X")}); err == nil || status(t, err) != http.StatusNotFound {
		t.Errorf("another company's item: %v", err)
	}
}

func TestBarcodesAreUniquePerCompany(t *testing.T) {
	svc, _, _, ctx := setup(t, manager())
	pcs := unit(t, svc, ctx, "PCS")
	a := goods("Teh botol", pcs)
	a.Item.Barcode = "8991234567890"
	first := create(t, svc, ctx, a)
	b := goods("Teh kotak", pcs)
	b.Item.Barcode = "8991234567890"
	if _, err := svc.Create(ctx, b); !strings.Contains(fields(t, err)["barcode"], first.Code) {
		t.Errorf("a barcode in use: %v", err)
	}
	b.Item.Barcode = ""
	second := create(t, svc, ctx, b)
	if _, err := svc.Update(ctx, second.ID, application.Patch{Barcode: ptr("8991234567890")}); fields(t, err)["barcode"] == "" {
		t.Errorf("changing to a barcode in use: %v", err)
	}
	// Two items without a barcode, and the same barcode in another company, are fine.
	create(t, svc, ctx, goods("Teh celup", pcs))
	co2 := kerneltest.In(context.Background(), 2, 0)
	a.Item.BaseUnitID = unit(t, svc, co2, "PCS").ID
	create(t, svc, co2, a)
}

func TestAlternativeUnitsAndTheirFactors(t *testing.T) {
	svc, repo, _, ctx := setup(t, manager())
	pcs := unit(t, svc, ctx, "PCS")
	dus := unit(t, svc, ctx, "DUS")
	lsn := unit(t, svc, ctx, "LSN")
	krt := unit(t, svc, ctx, "KRT")
	it := create(t, svc, ctx, goods("Mi instan", pcs))

	iu, err := svc.AddUnit(ctx, it.ID, dus.ID, money.MustRate("40"))
	if err != nil || iu.UnitCode != "DUS" || !iu.IsActive {
		t.Fatalf("AddUnit DUS = %+v %v", iu, err)
	}
	if _, err := svc.AddUnit(ctx, it.ID, lsn.ID, money.MustRate("12")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddUnit(ctx, it.ID, pcs.ID, money.MustRate("1")); fields(t, err)["unit_id"] == "" {
		t.Errorf("the base unit as an alternative: %v", err)
	}
	if _, err := svc.AddUnit(ctx, it.ID, krt.ID, money.Rate{}); fields(t, err)["factor"] == "" {
		t.Errorf("a zero factor: %v", err)
	}
	if _, err := svc.AddUnit(ctx, it.ID, krt.ID, money.MustRate("-2")); fields(t, err)["factor"] == "" {
		t.Errorf("a negative factor: %v", err)
	}
	if _, err := svc.AddUnit(ctx, it.ID, dus.ID, money.MustRate("40")); err == nil || status(t, err) != http.StatusConflict {
		t.Errorf("DUS twice: %v", err)
	}
	if _, err := svc.UpdateUnit(ctx, krt.ID, application.UnitPatch{IsActive: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddUnit(ctx, it.ID, krt.ID, money.MustRate("400")); fields(t, err)["unit_id"] == "" {
		t.Errorf("an inactive unit: %v", err)
	}

	// The contract's view: base first (factor 1), then active alternatives by factor.
	units, err := svc.Measures(ctx, it.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, u := range units {
		got = append(got, u.Code+"="+u.Factor.String())
	}
	if strings.Join(got, " ") != "PCS=1 LSN=12 DUS=40" || !units[0].IsBase || units[1].IsBase {
		t.Errorf("Measures = %v", got)
	}
	for code, want := range map[string]string{"": "PCS=1", "pcs": "PCS=1", " dus ": "DUS=40", "LSN": "LSN=12"} {
		m, err := svc.Measure(ctx, it.ID, code)
		if err != nil || m.Code+"="+m.Factor.String() != want || m.IsBase != (want == "PCS=1") {
			t.Errorf("Measure(%q) = %+v %v; want %s", code, m, err, want)
		}
	}
	for _, code := range []string{"KRT", "KG", "X"} {
		if _, err := svc.Measure(ctx, it.ID, code); !errors.Is(err, application.ErrUnknownUnit) {
			t.Errorf("Measure(%q): %v", code, err)
		}
	}

	// A factor never changes: deactivate, then only the same factor brings the unit back.
	if err := svc.DeactivateUnit(ctx, it.ID, iu.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Measure(ctx, it.ID, "DUS"); !errors.Is(err, application.ErrUnknownUnit) {
		t.Errorf("a deactivated unit: %v", err)
	}
	if _, err := svc.AddUnit(ctx, it.ID, dus.ID, money.MustRate("24")); err == nil || status(t, err) != http.StatusConflict {
		t.Errorf("DUS again with another factor: %v", err)
	}
	back, err := svc.AddUnit(ctx, it.ID, dus.ID, money.MustRate("40.000"))
	if err != nil || back.ID != iu.ID || !back.IsActive {
		t.Errorf("DUS again with its factor = %+v %v", back, err)
	}
	if n := len(repo.itemUnits); n != 2 {
		t.Errorf("%d item units stored; a reactivation adds none", n)
	}
	if err := svc.DeactivateUnit(ctx, it.ID, 999); err == nil || status(t, err) != http.StatusNotFound {
		t.Errorf("an unknown item unit: %v", err)
	}

	co2 := kerneltest.In(context.Background(), 2, 0)
	if _, err := svc.Measure(co2, it.ID, ""); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("Measure from another company: %v", err)
	}
	if _, err := svc.Measures(co2, it.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("Measures from another company: %v", err)
	}
}

func TestGetAndUsableForDocuments(t *testing.T) {
	svc, _, _, ctx := setup(t, manager())
	it := create(t, svc, ctx, goods("Gula", unit(t, svc, ctx, "KG")))
	got, err := svc.Get(ctx, it.ID)
	if err != nil || got.Code != it.Code || got.BaseUnitCode != "KG" || !got.IsStocked {
		t.Errorf("Get = %+v %v", got, err)
	}
	if err := svc.Usable(ctx, it.ID); err != nil {
		t.Errorf("Usable(active) = %v", err)
	}
	if err := svc.SetActive(ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Usable(ctx, it.ID); !errors.Is(err, application.ErrInactive) {
		t.Errorf("Usable(inactive) = %v", err)
	}
	if got, err := svc.Get(ctx, it.ID); err != nil || got.IsActive {
		t.Errorf("Get(inactive) = %+v %v", got, err)
	}
	co2 := kerneltest.In(context.Background(), 2, 0)
	if err := svc.Usable(co2, it.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("Usable from another company = %v", err)
	}
	if _, err := svc.Get(co2, it.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("Get from another company = %v", err)
	}
}

func TestSearchEscapesLikeAndBoundsTheLimit(t *testing.T) {
	svc, repo, _, ctx := setup(t, manager())
	pcs := unit(t, svc, ctx, "PCS")
	create(t, svc, ctx, goods("Diskon 50%", pcs))
	create(t, svc, ctx, goods("Diskon 500", pcs))
	create(t, svc, ctx, goods("Kabel_USB", pcs))
	create(t, svc, ctx, goods("KabelXUSB", pcs))

	rows, _, err := svc.List(ctx, application.ListFilter{Search: " 50% "})
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastFilter.Pattern != `50\%` || repo.lastFilter.Search != "50%" {
		t.Errorf("filter = %+v", repo.lastFilter)
	}
	if len(rows) != 1 || rows[0].Name != "Diskon 50%" {
		t.Errorf(`"50%%" matched %v`, rows)
	}
	if _, _, err := svc.List(ctx, application.ListFilter{Search: `a_b\c`}); err != nil || repo.lastFilter.Pattern != `a\_b\\c` {
		t.Errorf(`pattern of a_b\c = %q %v`, repo.lastFilter.Pattern, err)
	}
	found, err := svc.Search(ctx, "_", 10)
	if err != nil || repo.lastPattern != `\_` || len(found) != 1 || found[0].Name != "Kabel_USB" {
		t.Errorf(`Search("_") = %v %v (pattern %q)`, found, err, repo.lastPattern)
	}
	if found[0].BaseUnitCode != "PCS" {
		t.Errorf("Search fills the base unit: %+v", found[0])
	}
	for limit, want := range map[int]int{0: 1, -5: 1, 20: 20, 500: 50} {
		if _, err := svc.Search(ctx, "", limit); err != nil || repo.lastLimit != want {
			t.Errorf("Search limit %d → %d; want %d", limit, repo.lastLimit, want)
		}
	}
	if _, _, err := svc.List(ctx, application.ListFilter{Status: "gone"}); fields(t, err)["status"] == "" {
		t.Errorf("an unknown status: %v", err)
	}
	if _, _, err := svc.List(ctx, application.ListFilter{Kind: "food"}); fields(t, err)["kind"] == "" {
		t.Errorf("an unknown kind: %v", err)
	}
}

func TestUnitsCategoriesAndTheCommonUnits(t *testing.T) {
	svc, _, k, ctx := setup(t, manager())
	u, err := svc.CreateUnit(ctx, domain.Unit{Code: " pcs ", Name: "Buah", TaxUnitCode: "um.0021"})
	if err != nil || u.Code != "PCS" || u.TaxUnitCode != "UM.0021" {
		t.Fatalf("CreateUnit = %+v %v", u, err)
	}
	for name, tc := range map[string]struct {
		u     domain.Unit
		field string
	}{
		"code in use":     {domain.Unit{Code: "PCS", Name: "Piece"}, "code"},
		"code with space": {domain.Unit{Code: "PC S", Name: "X"}, "code"},
		"code with dash":  {domain.Unit{Code: "M-2", Name: "X"}, "code"},
		"long code":       {domain.Unit{Code: "ABCDEFGHIJK", Name: "X"}, "code"},
		"no name":         {domain.Unit{Code: "X"}, "name"},
		"tax unit code":   {domain.Unit{Code: "X", Name: "X", TaxUnitCode: "UM.21"}, "tax_unit_code"},
	} {
		if _, err := svc.CreateUnit(ctx, tc.u); err == nil || fields(t, err)[tc.field] == "" {
			t.Errorf("%s: want a %s error, got %v", name, tc.field, err)
		}
	}
	renamed, err := svc.UpdateUnit(ctx, u.ID, application.UnitPatch{Name: ptr("Piece")})
	if err != nil || renamed.Name != "Piece" || renamed.Code != "PCS" || renamed.TaxUnitCode != "UM.0021" || !renamed.IsActive {
		t.Errorf("UpdateUnit(name) = %+v %v", renamed, err)
	}

	added, err := svc.AddCommonUnits(ctx)
	if err != nil || len(added) != len(domain.CommonUnits)-1 || strings.Contains(strings.Join(added, " "), "PCS") {
		t.Fatalf("AddCommonUnits = %v %v", added, err)
	}
	units, _ := svc.Units(ctx)
	byCode := map[string]domain.Unit{}
	for _, u := range units {
		byCode[u.Code] = u
	}
	if byCode["PCS"].Name != "Piece" || byCode["KRT"].TaxUnitCode != "UM.0037" || byCode["ROL"].TaxUnitCode != "UM.0039" || byCode["KG"].Name != "Kilogram" {
		t.Errorf("units after the defaults = %+v", byCode)
	}
	audits := len(k.Audit.Records())
	if again, err := svc.AddCommonUnits(ctx); err != nil || len(again) != 0 || len(k.Audit.Records()) != audits {
		t.Errorf("AddCommonUnits again = %v %v", again, err)
	}

	cat, err := svc.CreateCategory(ctx, domain.Category{Name: " Minuman "})
	if err != nil || cat.Name != "Minuman" {
		t.Fatalf("CreateCategory = %+v %v", cat, err)
	}
	if _, err := svc.CreateCategory(ctx, domain.Category{Name: "Minuman"}); fields(t, err)["name"] == "" {
		t.Errorf("a category name in use: %v", err)
	}
	if _, err := svc.CreateCategory(kerneltest.In(context.Background(), 2, 0), domain.Category{Name: "Minuman"}); err != nil {
		t.Errorf("the same name in another company: %v", err)
	}
	if _, err := svc.UpdateCategory(ctx, cat.ID, application.CategoryPatch{Name: ptr("")}); fields(t, err)["name"] == "" {
		t.Errorf("an empty category name: %v", err)
	}
	if _, err := svc.UpdateUnit(kerneltest.In(context.Background(), 2, 0), u.ID, application.UnitPatch{Name: ptr("X")}); err == nil || status(t, err) != http.StatusNotFound {
		t.Errorf("another company's unit: %v", err)
	}
}

func decode[T any](t *testing.T, ev kernel.Event) T {
	t.Helper()
	var p T
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatalf("%s payload: %v", ev.Name, err)
	}
	return p
}

// eventNames is every published event as "<verb>" or "<verb>:<part>", in order.
func eventNames(t *testing.T, k *kerneltest.Kit) []string {
	t.Helper()
	var out []string
	for _, ev := range k.Published() {
		if ev.Version != 1 || !strings.HasPrefix(ev.Name, "items.item.") {
			t.Fatalf("event %s v%d", ev.Name, ev.Version)
		}
		name := strings.TrimPrefix(ev.Name, "items.item.")
		if name == "changed" {
			name += ":" + decode[application.ItemChanged](t, ev).Part
		}
		out = append(out, name)
	}
	return out
}

func TestEventsAndAuditFollowTheLifecycle(t *testing.T) {
	svc, _, k, ctx := setup(t, manager())
	pcs := unit(t, svc, ctx, "PCS")
	dus := unit(t, svc, ctx, "DUS")
	it := create(t, svc, ctx, goods("Air mineral", pcs))
	pub := k.Published()
	if len(pub) != 1 {
		t.Fatalf("after Create, published = %+v", pub)
	}
	if c := decode[application.ItemCreated](t, pub[0]); c.ID != it.ID || c.CompanyID != 1 || c.Code != "BRG-00001" || c.Name != "Air mineral" || c.Kind != "goods" {
		t.Errorf("created payload = %+v", c)
	}

	if _, err := svc.Update(ctx, it.ID, application.Patch{Name: ptr("Air mineral 600 ml")}); err != nil {
		t.Fatal(err)
	}
	iu, err := svc.AddUnit(ctx, it.ID, dus.ID, money.MustRate("24"))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeactivateUnit(ctx, it.ID, iu.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeactivateUnit(ctx, it.ID, iu.ID); err != nil { // already inactive: nothing
		t.Fatal(err)
	}
	if err := svc.SetActive(ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActive(ctx, it.ID, false); err != nil { // already inactive: nothing
		t.Fatal(err)
	}
	if err := svc.SetActive(ctx, it.ID, true); err != nil {
		t.Fatal(err)
	}
	want := "created changed:item changed:unit changed:unit deactivated activated"
	if got := strings.Join(eventNames(t, k), " "); got != want {
		t.Fatalf("events = %s\nwant     %s", got, want)
	}
	pub = k.Published()
	if ch := decode[application.ItemChanged](t, pub[1]); ch.ID != it.ID || ch.Name != "Air mineral 600 ml" || ch.Part != "item" {
		t.Errorf("changed payload = %+v", ch)
	}
	if off := decode[application.ItemEvent](t, pub[4]); off.ID != it.ID || off.CompanyID != 1 || off.Code != "BRG-00001" {
		t.Errorf("deactivated payload = %+v", off)
	}

	var actions []string
	for _, r := range k.Audit.Records() {
		if r.Module != "items" || r.Summary == "" {
			t.Errorf("audit record %+v", r)
		}
		actions = append(actions, r.Action)
	}
	wantAudit := "unit.created unit.created item.created item.updated item.unit_added item.unit_deactivated item.deactivated item.activated"
	if got := strings.Join(actions, " "); got != wantAudit {
		t.Errorf("audit = %s\nwant    %s", got, wantAudit)
	}
}

func TestNoEventOfAFailedTransaction(t *testing.T) {
	svc, _, k, ctx := setup(t, manager())
	pcs := unit(t, svc, ctx, "PCS")
	typed := goods("Sabun", pcs)
	typed.Item.Code = "SB-1"
	it := create(t, svc, ctx, typed)
	n := len(k.Published())
	// Refused changes: a code in use, a fixed field, another company's item.
	if _, err := svc.Create(ctx, typed); err == nil {
		t.Fatal("a duplicate code saved")
	}
	if _, err := svc.Update(ctx, it.ID, application.Patch{Kind: ptr("service"), Name: ptr("Sabun cair")}); err == nil {
		t.Fatal("a kind change saved")
	}
	if err := svc.SetActive(kerneltest.In(context.Background(), 2, 0), it.ID, false); err == nil {
		t.Fatal("deactivated from another company")
	}
	if got := len(k.Published()); got != n {
		t.Fatalf("refused changes published %d events", got-n)
	}
	// The event is published inside the transaction: a caller's rollback takes it with it.
	err := k.Deps.Tx.Do(ctx, func(ctx context.Context) error {
		create(t, svc, ctx, goods("Sampo", pcs))
		if err := svc.SetActive(ctx, it.ID, false); err != nil {
			return err
		}
		return errors.New("the caller rolls back")
	})
	if err == nil {
		t.Fatal("the outer transaction did not fail")
	}
	if got := len(k.Published()); got != n {
		t.Fatalf("a rolled-back transaction published %d events", got-n)
	}
}
