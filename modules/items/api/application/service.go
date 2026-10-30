// Package application holds the items module's use cases: keeping items, their alternative
// units, the company's units and categories, and answering other modules what a document needs
// to know about an item.
package application

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/items/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/validator"
)

// Permissions this module declares (items.module.go) and checks.
const (
	PermRead   = "items.read"
	PermManage = "items.manage"
)

var (
	// ErrNotFound is no row with that ID (or code) in the company.
	ErrNotFound = errors.New("items: not found")
	// ErrCodeTaken is an item code, or a unit code, the company already has.
	ErrCodeTaken = errors.New("items: code taken")
	// ErrBarcodeTaken is a barcode another item of the company has.
	ErrBarcodeTaken = errors.New("items: barcode taken")
	// ErrNameTaken is a category name the company already has.
	ErrNameTaken = errors.New("items: name taken")
)

// ListFilter is a page of the item list.
type ListFilter struct {
	// Search is what was typed, trimmed: a barcode matches it whole.
	Search string
	// Pattern is Search escaped for LIKE (likeEscape): a code or name containing it matches, and
	// a "%" or "_" typed is a character, not a wildcard.
	Pattern    string
	Kind       string // "", "goods", "service"
	CategoryID int64  // 0: any
	Status     string // "", "active", "inactive"
	Limit      int
	Offset     int
}

// Settings are a company's code prefix and counter.
type Settings struct {
	CodePrefix string
	LastNumber int64
}

// DefaultPrefix is the prefix of a company that never chose one.
const DefaultPrefix = "BRG-"

// Repository is the itm_* tables.
type Repository interface {
	List(ctx context.Context, company int64, f ListFilter) ([]domain.Item, int64, error)
	// Search is active items whose code starts with pattern, whose name contains it, or whose
	// barcode is search.
	Search(ctx context.Context, company int64, search, pattern string, limit int) ([]domain.Item, error)
	Get(ctx context.Context, company, id int64) (domain.Item, error)
	// GetShared is Get holding the row shared until the transaction ends.
	GetShared(ctx context.Context, company, id int64) (domain.Item, error)
	// GetForUpdate is Get holding the row exclusively until the transaction ends.
	GetForUpdate(ctx context.Context, company, id int64) (domain.Item, error)
	CodeTaken(ctx context.Context, company int64, code string) (bool, error)
	// BarcodeOwner is the code of the other item (not except) with the barcode, or "".
	BarcodeOwner(ctx context.Context, company int64, barcode string, except int64) (string, error)
	Insert(ctx context.Context, it domain.Item) (int64, error)
	Update(ctx context.Context, it domain.Item) error
	SetActive(ctx context.Context, company, id int64, active bool, by int64, at time.Time) error

	Units(ctx context.Context, company int64) ([]domain.Unit, error)
	Unit(ctx context.Context, company, id int64) (domain.Unit, error)
	UnitForUpdate(ctx context.Context, company, id int64) (domain.Unit, error)
	UnitByCode(ctx context.Context, company int64, code string) (domain.Unit, error)
	InsertUnit(ctx context.Context, u domain.Unit) (int64, error)
	// InsertUnitIfMissing adds u unless the company has a unit with its code; reports whether it did.
	InsertUnitIfMissing(ctx context.Context, u domain.Unit) (bool, error)
	UpdateUnit(ctx context.Context, u domain.Unit) error

	Categories(ctx context.Context, company int64) ([]domain.Category, error)
	Category(ctx context.Context, company, id int64) (domain.Category, error)
	CategoryForUpdate(ctx context.Context, company, id int64) (domain.Category, error)
	InsertCategory(ctx context.Context, c domain.Category) (int64, error)
	UpdateCategory(ctx context.Context, c domain.Category) error

	// ItemUnits are an item's alternative units with their unit's code and name: active first,
	// then by factor.
	ItemUnits(ctx context.Context, company, item int64) ([]domain.ItemUnit, error)
	ItemUnit(ctx context.Context, company, item, id int64) (domain.ItemUnit, error)
	// ItemUnitOf is the item's alternative unit in unit, active or not.
	ItemUnitOf(ctx context.Context, company, item, unit int64) (domain.ItemUnit, error)
	InsertItemUnit(ctx context.Context, iu domain.ItemUnit) (int64, error)
	SetItemUnitActive(ctx context.Context, company, item, id int64, active bool) error

	// Settings are the company's, or the defaults when it has none yet.
	Settings(ctx context.Context, company int64) (Settings, error)
	// LockSettings creates the company's row when missing and holds it until the transaction ends.
	LockSettings(ctx context.Context, company int64, at time.Time) (Settings, error)
	SetLastNumber(ctx context.Context, company, n int64) error
	SetPrefix(ctx context.Context, company int64, prefix string, at time.Time) error
}

// Transactor runs fn in one transaction — the kernel's d.Tx.
type Transactor interface {
	Do(ctx context.Context, fn func(ctx context.Context) error, opts ...kernel.TxOption) error
}

// Publisher announces events — the kernel's d.Events.
type Publisher interface {
	Publish(ctx context.Context, name string, version int, payload any) error
}

// Auditor records changes — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Authorizer answers how far the signed-in user's permissions reach — the kernel's d.Authz.
type Authorizer interface {
	Units(ctx context.Context, permission string) (kernel.Units, error)
}

// Identity is who is signed in — the kernel's d.Auth.
type Identity interface {
	UserID(ctx context.Context) (int64, bool)
}

// Service is the items module's use cases.
type Service struct {
	repo   Repository
	tx     Transactor
	events Publisher
	audit  Auditor
	authz  Authorizer
	auth   Identity
	clock  kernel.Clock
}

func NewService(repo Repository, tx Transactor, events Publisher, audit Auditor, authz Authorizer, auth Identity, clock kernel.Clock) *Service {
	return &Service{repo: repo, tx: tx, events: events, audit: audit, authz: authz, auth: auth, clock: clock}
}

// Event payloads — what a subscriber receives (README › Events). Primitive IDs and plain fields
// only. Published inside the transaction that makes the change, so an event exists only if the
// change committed.

// ItemEvent is the payload of items.item.deactivated and .activated.
type ItemEvent struct {
	ID        int64  `json:"id"`
	CompanyID int64  `json:"company_id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
}

// ItemCreated is the payload of items.item.created.
type ItemCreated struct {
	ItemEvent
	Kind string `json:"kind"`
}

// ItemChanged is the payload of items.item.changed: Part names what changed — "item" (its own
// fields) or "unit" (an alternative unit).
type ItemChanged struct {
	ItemEvent
	Part string `json:"part"`
}

func eventOf(it domain.Item) ItemEvent {
	return ItemEvent{ID: it.ID, CompanyID: it.CompanyID, Code: it.Code, Name: it.Name}
}

// changed announces that part of the item changed, inside the caller's transaction.
func (s *Service) changed(ctx context.Context, it domain.Item, part string) error {
	return s.events.Publish(ctx, "items.item.changed", 1, ItemChanged{ItemEvent: eventOf(it), Part: part})
}

func company(ctx context.Context) int64 { return kernel.ScopeFrom(ctx).CompanyID }

func target(kind string, id int64) string { return kind + ":" + strconv.FormatInt(id, 10) }

func fieldError(field, message string) error {
	return apperror.Validation([]validator.FieldError{{Field: field, Message: message}})
}

// likeEscape makes a search term match literally inside LIKE: "%" and "_" typed by a user are
// characters, not wildcards (MySQL's default escape character is the backslash).
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// companyWide refuses unless the signed-in user holds permission across the whole company.
func (s *Service) companyWide(ctx context.Context, permission, deny string) error {
	u, err := s.authz.Units(ctx, permission)
	if err != nil {
		return err
	}
	if !u.AllBranches {
		return apperror.Forbidden(deny)
	}
	return nil
}

// CanCompanyWide reports whether the signed-in user holds permission across the whole company —
// for the web to show what the API will allow.
func (s *Service) CanCompanyWide(ctx context.Context, permission string) bool {
	u, err := s.authz.Units(ctx, permission)
	return err == nil && u.AllBranches
}

func notFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("Barang tidak ditemukan")
	}
	return err
}

// describe fills the base unit's code and name and the category's name of every item.
func (s *Service) describe(ctx context.Context, co int64, items []domain.Item) error {
	if len(items) == 0 {
		return nil
	}
	units, err := s.repo.Units(ctx, co)
	if err != nil {
		return err
	}
	cats, err := s.repo.Categories(ctx, co)
	if err != nil {
		return err
	}
	unitByID := make(map[int64]domain.Unit, len(units))
	for _, u := range units {
		unitByID[u.ID] = u
	}
	catByID := make(map[int64]string, len(cats))
	for _, c := range cats {
		catByID[c.ID] = c.Name
	}
	for i := range items {
		u := unitByID[items[i].BaseUnitID]
		items[i].BaseUnitCode, items[i].BaseUnitName = u.Code, u.Name
		items[i].CategoryName = catByID[items[i].CategoryID]
	}
	return nil
}

// List is a page of the company's items.
func (s *Service) List(ctx context.Context, f ListFilter) ([]domain.Item, int64, error) {
	f.Search = strings.TrimSpace(f.Search)
	f.Pattern = likeEscape(f.Search)
	var v validator.Validator
	v.Check(f.Status == "" || f.Status == "active" || f.Status == "inactive", "status", "pilih active atau inactive")
	v.Check(f.Kind == "" || domain.Kind(f.Kind).Valid(), "kind", "pilih goods atau service")
	if err := v.Err(); err != nil {
		return nil, 0, err
	}
	co := company(ctx)
	rows, total, err := s.repo.List(ctx, co, f)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, s.describe(ctx, co, rows)
}

// Detail is an item with its alternative units.
type Detail struct {
	Item  domain.Item
	Units []domain.ItemUnit
}

// Detail returns an item, active or not, with its alternative units, active first.
func (s *Service) Detail(ctx context.Context, id int64) (Detail, error) {
	co := company(ctx)
	it, err := s.repo.Get(ctx, co, id)
	if err != nil {
		return Detail{}, notFound(err)
	}
	one := []domain.Item{it}
	if err := s.describe(ctx, co, one); err != nil {
		return Detail{}, err
	}
	units, err := s.repo.ItemUnits(ctx, co, id)
	if err != nil {
		return Detail{}, err
	}
	return Detail{Item: one[0], Units: units}, nil
}

// Input is what a person sends to create an item. Item.Code "" takes the next generated code;
// IsStocked nil tracks the stock of goods and not of a service.
type Input struct {
	Item      domain.Item
	IsStocked *bool
}

// Create adds an item to the company.
func (s *Service) Create(ctx context.Context, in Input) (domain.Item, error) {
	it := in.Item
	it.Code = domain.NormalizeCode(it.Code)
	if in.IsStocked != nil {
		it.IsStocked = *in.IsStocked
	} else {
		it.IsStocked = it.Kind == domain.Goods
	}
	var v validator.Validator
	it.Clean(&v)
	if it.Code != "" {
		v.Check(domain.ValidCode(it.Code), "code", "huruf besar, angka, . / -; maksimal 30")
	}
	if err := v.Err(); err != nil {
		return domain.Item{}, err
	}
	actor, _ := s.auth.UserID(ctx)
	now := s.clock.Now()
	co := company(ctx)
	it.CompanyID, it.IsActive, it.CreatedBy, it.CreatedAt, it.UpdatedBy, it.UpdatedAt = co, true, actor, now, actor, now
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.checkRefs(ctx, co, it, domain.Item{}); err != nil {
			return err
		}
		if it.Code == "" {
			code, err := s.nextCode(ctx, co, now)
			if err != nil {
				return err
			}
			it.Code = code
		}
		id, err := s.repo.Insert(ctx, it)
		if err != nil {
			return taken(err)
		}
		it.ID = id
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "item.created", Target: target("item", id),
			Summary: "Menambah " + kindLabel(it.Kind) + " " + it.Code + " " + it.Name,
			Data:    map[string]any{"code": it.Code, "name": it.Name, "kind": string(it.Kind), "is_stocked": it.IsStocked, "base_unit_id": it.BaseUnitID}})
		return s.events.Publish(ctx, "items.item.created", 1, ItemCreated{ItemEvent: eventOf(it), Kind: string(it.Kind)})
	})
	if err != nil {
		return domain.Item{}, err
	}
	one := []domain.Item{it}
	return one[0], s.describe(ctx, co, one)
}

func kindLabel(k domain.Kind) string {
	if k == domain.Service {
		return "jasa"
	}
	return "barang"
}

// taken maps a duplicate code or barcode found by the database to its field.
func taken(err error) error {
	switch {
	case errors.Is(err, ErrCodeTaken):
		return fieldError("code", "sudah dipakai barang lain")
	case errors.Is(err, ErrBarcodeTaken):
		return fieldError("barcode", "sudah dipakai barang lain")
	}
	return err
}

// checkRefs checks, inside the caller's transaction, what it points at that before did not: a
// base unit and a category the company has and still uses, and a barcode no other item has.
func (s *Service) checkRefs(ctx context.Context, co int64, it, before domain.Item) error {
	var v validator.Validator
	if it.BaseUnitID != before.BaseUnitID {
		u, err := s.repo.Unit(ctx, co, it.BaseUnitID)
		switch {
		case errors.Is(err, ErrNotFound):
			v.Check(false, "base_unit_id", "satuan tidak ditemukan")
		case err != nil:
			return err
		default:
			v.Check(u.IsActive, "base_unit_id", "satuan "+u.Code+" nonaktif — pilih satuan aktif")
		}
	}
	if it.CategoryID != 0 && it.CategoryID != before.CategoryID {
		c, err := s.repo.Category(ctx, co, it.CategoryID)
		switch {
		case errors.Is(err, ErrNotFound):
			v.Check(false, "category_id", "kategori tidak ditemukan")
		case err != nil:
			return err
		default:
			v.Check(c.IsActive, "category_id", "kategori "+c.Name+" nonaktif — pilih kategori aktif")
		}
	}
	if it.Barcode != "" && it.Barcode != before.Barcode {
		owner, err := s.repo.BarcodeOwner(ctx, co, it.Barcode, it.ID)
		if err != nil {
			return err
		}
		v.Check(owner == "", "barcode", "sudah dipakai barang "+owner)
	}
	return v.Err()
}

// maxCodeSkips bounds how many typed codes one generated code passes over.
const maxCodeSkips = 1000

// nextCode takes the company's next generated code under its settings row lock, passing over
// codes someone typed by hand.
func (s *Service) nextCode(ctx context.Context, co int64, now time.Time) (string, error) {
	st, err := s.repo.LockSettings(ctx, co, now)
	if err != nil {
		return "", err
	}
	for n := st.LastNumber + 1; n <= st.LastNumber+maxCodeSkips; n++ {
		code := domain.GeneratedCode(st.CodePrefix, n)
		taken, err := s.repo.CodeTaken(ctx, co, code)
		if err != nil {
			return "", err
		}
		if !taken {
			return code, s.repo.SetLastNumber(ctx, co, n)
		}
	}
	return "", apperror.Conflict("Kode otomatis berikutnya sudah dipakai semua — ganti awalan kode di pengaturan barang")
}

// Patch is a partial change of an item: a nil field is left as it is. Kind, IsStocked, and
// BaseUnitID are fixed — sending one that differs is refused.
type Patch struct {
	Code        *string
	Name        *string
	Kind        *string
	IsStocked   *bool
	CategoryID  *int64 // 0 clears it
	BaseUnitID  *int64
	Barcode     *string // "" clears it
	TaxCode     *string // "" is 000000
	Description *string
}

// sameStored reports whether a and b store the same fields Update writes.
func sameStored(a, b domain.Item) bool {
	return a.Code == b.Code && a.Name == b.Name && a.CategoryID == b.CategoryID && a.Barcode == b.Barcode &&
		a.TaxCode == b.TaxCode && a.Description == b.Description
}

// Update changes the fields p sends. A change that changes nothing writes, records, and
// announces nothing.
func (s *Service) Update(ctx context.Context, id int64, p Patch) (domain.Item, error) {
	co := company(ctx)
	actor, _ := s.auth.UserID(ctx)
	var saved domain.Item
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		before, err := s.repo.GetForUpdate(ctx, co, id)
		if err != nil {
			return notFound(err)
		}
		var v validator.Validator
		v.Check(p.Kind == nil || domain.Kind(*p.Kind) == before.Kind, "kind", "Jenis tidak bisa diubah setelah dibuat — buat barang atau jasa baru")
		v.Check(p.IsStocked == nil || *p.IsStocked == before.IsStocked, "is_stocked", "Pelacakan stok tidak bisa diubah setelah barang dibuat")
		v.Check(p.BaseUnitID == nil || *p.BaseUnitID == before.BaseUnitID, "base_unit_id",
			"Satuan dasar tidak bisa diubah setelah barang dibuat — tambahkan satuan alternatif, atau buat barang baru")
		it := before
		if p.Code != nil {
			it.Code = domain.NormalizeCode(*p.Code)
			v.Check(domain.ValidCode(it.Code), "code", "wajib diisi: huruf besar, angka, . / -; maksimal 30")
		}
		for _, f := range []struct {
			to   *string
			from *string
		}{{&it.Name, p.Name}, {&it.Barcode, p.Barcode}, {&it.TaxCode, p.TaxCode}, {&it.Description, p.Description}} {
			if f.from != nil {
				*f.to = *f.from
			}
		}
		if p.CategoryID != nil {
			it.CategoryID = max(*p.CategoryID, 0)
		}
		it.Clean(&v)
		if err := v.Err(); err != nil {
			return err
		}
		if sameStored(it, before) {
			saved = it
			return nil
		}
		if err := s.checkRefs(ctx, co, it, before); err != nil {
			return err
		}
		it.UpdatedBy, it.UpdatedAt = actor, s.clock.Now()
		if err := s.repo.Update(ctx, it); err != nil {
			return taken(err)
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "item.updated", Target: target("item", id),
			Summary: "Mengubah " + kindLabel(it.Kind) + " " + it.Code + " " + it.Name, Data: changes(before, it)})
		saved = it
		return s.changed(ctx, it, "item")
	})
	if err != nil {
		return domain.Item{}, err
	}
	one := []domain.Item{saved}
	return one[0], s.describe(ctx, co, one)
}

// changes is what Update changed, before and after, for the audit record.
func changes(a, b domain.Item) map[string]any {
	out := map[string]any{}
	add := func(field string, before, after any) {
		if before != after {
			out[field] = map[string]any{"before": before, "after": after}
		}
	}
	add("code", a.Code, b.Code)
	add("name", a.Name, b.Name)
	add("category_id", a.CategoryID, b.CategoryID)
	add("barcode", a.Barcode, b.Barcode)
	add("tax_code", a.TaxCode, b.TaxCode)
	if a.Description != b.Description {
		out["description"] = "diubah"
	}
	return out
}

// SetActive deactivates or reactivates an item. Its documents stay; new ones may not use it.
func (s *Service) SetActive(ctx context.Context, id int64, active bool) error {
	co := company(ctx)
	actor, _ := s.auth.UserID(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		it, err := s.repo.GetForUpdate(ctx, co, id)
		if err != nil {
			return notFound(err)
		}
		if it.IsActive == active {
			return nil
		}
		if err := s.repo.SetActive(ctx, co, id, active, actor, s.clock.Now()); err != nil {
			return err
		}
		action, summary := "item.deactivated", "Menonaktifkan "
		if active {
			action, summary = "item.activated", "Mengaktifkan "
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: action, Target: target("item", id), Summary: summary + kindLabel(it.Kind) + " " + it.Code + " " + it.Name})
		if active {
			return s.events.Publish(ctx, "items.item.activated", 1, eventOf(it))
		}
		return s.events.Publish(ctx, "items.item.deactivated", 1, eventOf(it))
	})
}

// Settings are the company's code prefix and last generated number.
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	return s.repo.Settings(ctx, company(ctx))
}

// SetPrefix changes the prefix of generated codes: items.manage across the company. Codes
// already given keep theirs.
func (s *Service) SetPrefix(ctx context.Context, prefix string) error {
	prefix = domain.NormalizeCode(prefix)
	if !domain.ValidPrefix(prefix) {
		return fieldError("code_prefix", "huruf besar, angka, . / -; maksimal 10")
	}
	if err := s.companyWide(ctx, PermManage, "Mengubah awalan kode butuh izin items.manage di seluruh perusahaan"); err != nil {
		return err
	}
	co := company(ctx)
	now := s.clock.Now()
	return s.tx.Do(ctx, func(ctx context.Context) error {
		st, err := s.repo.LockSettings(ctx, co, now)
		if err != nil || st.CodePrefix == prefix {
			return err
		}
		if err := s.repo.SetPrefix(ctx, co, prefix, now); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "settings.changed", Target: "items:settings",
			Summary: "Mengubah awalan kode barang menjadi " + prefix, Data: map[string]any{"before": st.CodePrefix, "after": prefix}})
		return nil
	})
}
