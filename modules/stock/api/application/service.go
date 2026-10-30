// Package application holds the stock module's use cases: warehouses and the costing method,
// documents that move stock, the ledger that records every movement with its value, and what
// is in stock.
package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/stock/domain"
	"__GO_MODULE__/internal/modules/stock/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/costing"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/validator"
)

// Permissions this module declares (stock.module.go) and checks.
const (
	PermRead   = "stock.read"
	PermDraft  = "stock.draft"
	PermPost   = "stock.post"
	PermCancel = "stock.cancel"
	PermManage = "stock.manage"
)

// Area is what stock declares to the period module; DocTypes what it declares to numbering.
const Area = "inventory"

// DocType is the numbering series of a kind of document.
func DocType(k domain.Kind) string { return "stock." + string(k) }

var (
	// ErrNotFound is no row with that ID in the company.
	ErrNotFound = errors.New("stock: not found")
	// ErrCodeTaken is a warehouse code the company already uses.
	ErrCodeTaken = errors.New("stock: code taken")

	// What a movement can be refused for; each comes inside a *MoveError with a message.
	ErrInsufficient = errors.New("stock: not enough stock")
	ErrBackdated    = errors.New("stock: dated before the last movement")
	ErrNotLast      = errors.New("stock: a later movement exists")
	ErrNoCost       = errors.New("stock: no cost to value goods coming in")
	ErrNotStocked   = errors.New("stock: item is not stocked")
	ErrItemInactive = errors.New("stock: item inactive")
	ErrWhInactive   = errors.New("stock: warehouse inactive")
	ErrPeriodClosed = errors.New("stock: period closed")
)

// MoveError is a movement refused, with what the person needs to know to fix it.
type MoveError struct {
	Err     error
	Message string
}

func (e *MoveError) Error() string { return e.Message }
func (e *MoveError) Unwrap() error { return e.Err }

func refuse(err error, format string, args ...any) error {
	return &MoveError{Err: err, Message: fmt.Sprintf(format, args...)}
}

// Settings are a company's costing method, and whether stock has moved under it.
type Settings struct {
	Method costing.Method
	Locked bool
}

// Balance is an item's stock in a warehouse.
type Balance struct {
	WarehouseID int64
	ItemID      int64
	Qty         money.Qty
	Value       money.Amount
	LastMoveID  int64        // the latest movement still counted; 0: none
	LastDate    *kernel.Date // its date: nothing may be dated earlier
}

// Move is one row of the ledger.
type Move struct {
	ID            int64
	WarehouseID   int64
	WarehouseCode string // filled by reads that show it
	ItemID        int64
	Date          kernel.Date
	Qty           money.Qty    // + in, − out (base unit)
	Value         money.Amount // + in, − out
	QtyAfter      money.Qty
	ValueAfter    money.Amount
	DocumentID    int64
	LineID        int64
	Source        string
	Reverses      int64 // the row this one undoes
	ReversedBy    int64 // the row that undid this one
	CreatedAt     time.Time
}

// DocRef names a document a ledger row came from.
type DocRef struct {
	ID     int64
	Kind   domain.Kind
	Number string
}

// DocFilter is a page of the document list.
type DocFilter struct {
	Kind        string
	Status      string
	WarehouseID int64
	From, To    kernel.Date
	Search      string // a LIKE pattern, escaped; "" for none
	Limit       int
	Offset      int
}

// DocRow is a document as the list shows it.
type DocRow struct {
	domain.Document
	WarehouseCode string
	WarehouseName string
	TotalValue    money.Amount // what its lines added or took when posted; 0 for a draft
}

// BalanceFilter is a page of the stock list.
type BalanceFilter struct {
	WarehouseID int64
	ItemID      int64
	WithStock   bool
	Search      string // a LIKE pattern, escaped; "" for none
	Limit       int
	Offset      int
}

// BalanceRow is an item's stock in a warehouse as the stock list shows it.
type BalanceRow struct {
	WarehouseID   int64
	WarehouseCode string
	WarehouseName string
	ItemID        int64
	ItemCode      string
	ItemName      string
	BaseUnit      string
	Qty           money.Qty
	Value         money.Amount
	LastDate      *kernel.Date
}

// ReportFilter is the movement report's period and filters.
type ReportFilter struct {
	WarehouseID int64
	From, To    kernel.Date
	Search      string // a LIKE pattern, escaped; "" for none
}

// ReportRow is one item in one warehouse over the report's period.
type ReportRow struct {
	WarehouseID   int64
	WarehouseCode string
	ItemID        int64
	ItemCode      string
	ItemName      string
	BaseUnit      string
	OpeningQty    money.Qty
	OpeningValue  money.Amount
	InQty         money.Qty
	InValue       money.Amount
	OutQty        money.Qty
	OutValue      money.Amount
	ClosingQty    money.Qty
	ClosingValue  money.Amount
}

// Opening is an item's balance in a warehouse before the stock card's first day.
type Opening struct {
	WarehouseID int64
	Qty         money.Qty
	Value       money.Amount
}

// Repository is the stk_* tables.
type Repository interface {
	// Settings are the company's, or the defaults when it has none yet.
	Settings(ctx context.Context, company int64) (Settings, error)
	// SettingsShared creates the company's row when missing and holds it shared until the
	// transaction ends; SettingsForUpdate holds it exclusively.
	SettingsShared(ctx context.Context, company int64, at time.Time) (Settings, error)
	SettingsForUpdate(ctx context.Context, company int64, at time.Time) (Settings, error)
	SetCostMethod(ctx context.Context, company int64, m costing.Method, at time.Time) error
	LockCostMethod(ctx context.Context, company int64, at time.Time) error

	Warehouses(ctx context.Context, company int64) ([]domain.Warehouse, error)
	Warehouse(ctx context.Context, company, id int64) (domain.Warehouse, error)
	WarehouseShared(ctx context.Context, company, id int64) (domain.Warehouse, error)
	WarehouseForUpdate(ctx context.Context, company, id int64) (domain.Warehouse, error)
	InsertWarehouse(ctx context.Context, w domain.Warehouse) (int64, error)
	UpdateWarehouse(ctx context.Context, w domain.Warehouse) error
	SetWarehouseActive(ctx context.Context, company, id int64, active bool, at time.Time) error
	WarehouseHasStock(ctx context.Context, company, id int64) (bool, error)

	UpsertItem(ctx context.Context, company int64, it ports.Item, at time.Time) error
	RenameItem(ctx context.Context, company, id int64, code, name string, at time.Time) error

	Documents(ctx context.Context, company int64, f DocFilter) ([]DocRow, int64, error)
	// Document returns a document with its lines; forUpdate holds it until the transaction ends.
	Document(ctx context.Context, company, id int64, forUpdate bool) (domain.Document, error)
	InsertDocument(ctx context.Context, d domain.Document) (int64, error)
	// UpdateDraft replaces a draft's header and lines; 0 when it is no longer a draft.
	UpdateDraft(ctx context.Context, d domain.Document) (int64, error)
	DeleteDraft(ctx context.Context, company, id int64) (int64, error)
	SetLinePosted(ctx context.Context, company, lineID int64, code, name string, value money.Amount) error
	MarkPosted(ctx context.Context, company, id int64, number string, at time.Time, by int64) (int64, error)
	MarkCancelled(ctx context.Context, company, id int64, reason string, at time.Time, by int64) (int64, error)
	Amended(ctx context.Context, company, id int64) (bool, error)
	DocumentsByID(ctx context.Context, company int64, ids []int64) ([]DocRef, error)

	// LockBalance creates the balance row when missing and holds it until the transaction ends.
	LockBalance(ctx context.Context, company, warehouse, item int64, at time.Time) (Balance, error)
	// Balance is the item's balance in the warehouse; zero when it never moved there.
	Balance(ctx context.Context, company, warehouse, item int64) (Balance, error)
	SaveBalance(ctx context.Context, company int64, b Balance, at time.Time) error
	InsertMove(ctx context.Context, company int64, m Move, at time.Time) (int64, error)
	MarkReversed(ctx context.Context, company, id, by int64) error
	// OpenMovesBySource are the source's movements still counted, newest first.
	OpenMovesBySource(ctx context.Context, company int64, source string) ([]Move, error)
	// LastOpenMove is the latest movement of the item in the warehouse still counted.
	LastOpenMove(ctx context.Context, company, warehouse, item int64) (Move, bool, error)
	OpenLayers(ctx context.Context, company, warehouse, item int64) ([]costing.Layer, error)
	LayerOfMove(ctx context.Context, company, moveID int64) (costing.Layer, error)
	LayersOfTakes(ctx context.Context, company, moveID int64) ([]costing.Layer, error)
	InsertLayer(ctx context.Context, company, warehouse, item, moveID int64, l costing.Layer) (int64, error)
	SetLayer(ctx context.Context, company int64, l costing.Layer) error
	DeleteLayer(ctx context.Context, company, id int64) error
	InsertTakes(ctx context.Context, company, moveID int64, takes []costing.Take) error
	Takes(ctx context.Context, company, moveID int64) ([]costing.Take, error)

	Balances(ctx context.Context, company int64, f BalanceFilter) ([]BalanceRow, int64, error)
	TotalValue(ctx context.Context, company, warehouse int64) (money.Amount, error)
	Openings(ctx context.Context, company, item, warehouse int64, from kernel.Date) ([]Opening, error)
	CardMoves(ctx context.Context, company, item, warehouse int64, from, to kernel.Date) ([]Move, error)
	MovementReport(ctx context.Context, company int64, f ReportFilter) ([]ReportRow, error)
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

// Service is the stock module's use cases.
type Service struct {
	repo     Repository
	tx       Transactor
	items    ports.Items
	numbers  ports.Numbers
	periods  ports.Periods  // nil: no period lock
	branches ports.Branches // nil: warehouses belong to the company as a whole
	events   Publisher
	audit    Auditor
	authz    Authorizer
	auth     Identity
	clock    kernel.Clock
}

func NewService(repo Repository, tx Transactor, items ports.Items, numbers ports.Numbers, periods ports.Periods, branches ports.Branches,
	events Publisher, audit Auditor, authz Authorizer, auth Identity, clock kernel.Clock) *Service {
	for _, k := range []domain.Kind{domain.Receipt, domain.Issue, domain.Adjustment} {
		prefix := map[domain.Kind]string{domain.Receipt: "BM", domain.Issue: "BK", domain.Adjustment: "PS"}[k]
		numbers.Declare(DocType(k), k.Label(), ports.NumberFormat{Pattern: prefix + "/{YYYY}/{MM}/{SEQ:4}", Reset: "monthly"})
	}
	if periods != nil {
		periods.Declare(Area, "Persediaan")
	}
	return &Service{repo: repo, tx: tx, items: items, numbers: numbers, periods: periods, branches: branches, events: events, audit: audit,
		authz: authz, auth: auth, clock: clock}
}

func company(ctx context.Context) int64 { return kernel.ScopeFrom(ctx).CompanyID }

func (s *Service) actor(ctx context.Context) int64 {
	id, _ := s.auth.UserID(ctx)
	return id
}

// allowed refuses unless the signed-in user holds permission for branch (0: the whole company).
func (s *Service) allowed(ctx context.Context, permission string, branch int64) error {
	u, err := s.authz.Units(ctx, permission)
	if err != nil {
		return err
	}
	if !u.Includes(branch) {
		if branch == 0 {
			return apperror.Forbidden("Butuh izin " + permission + " di seluruh perusahaan untuk gudang ini")
		}
		return apperror.Forbidden("Butuh izin " + permission + " di cabang gudang ini")
	}
	return nil
}

// Can reports whether the signed-in user holds permission for branch — for the web to show what
// the API will allow.
func (s *Service) Can(ctx context.Context, permission string, branch int64) bool {
	u, err := s.authz.Units(ctx, permission)
	return err == nil && u.Includes(branch)
}

// LikePattern is q as a LIKE '%q%' pattern that matches it literally ("" for an empty q): "%",
// "_", and "\" a person typed are characters, not wildcards.
func LikePattern(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

func notFound(err error, what string) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound(what + " tidak ditemukan")
	}
	return err
}

func target(kind string, id int64) string { return kind + ":" + strconv.FormatInt(id, 10) }

// --- costing method --------------------------------------------------------------------------

// Settings are the company's costing method.
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	return s.repo.Settings(ctx, company(ctx))
}

// SetCostMethod changes the company's costing method — only before stock has moved, and only by
// whoever holds stock.manage across the company. A security record: it decides every value.
func (s *Service) SetCostMethod(ctx context.Context, method string) (Settings, error) {
	m, err := costing.ParseMethod(method)
	if err != nil {
		return Settings{}, apperror.Validation([]validator.FieldError{{Field: "cost_method", Message: "pilih rata-rata atau FIFO"}})
	}
	if err := s.allowed(ctx, PermManage, 0); err != nil {
		return Settings{}, err
	}
	co, now := company(ctx), s.clock.Now()
	var out Settings
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		st, err := s.repo.SettingsForUpdate(ctx, co, now)
		if err != nil {
			return err
		}
		if st.Method == m {
			out = st
			return nil
		}
		if st.Locked {
			return apperror.Conflict("Metode biaya tidak bisa diubah setelah ada mutasi stok (PSAK 202, UU PPh Pasal 10 ayat 6: metode dipakai secara konsisten)")
		}
		if err := s.repo.SetCostMethod(ctx, co, m, now); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "stock", Action: "cost-method.changed", Target: target("company", co),
			Summary: "Mengubah metode biaya persediaan menjadi " + methodLabel(m), Data: map[string]any{"before": string(st.Method), "after": string(m)}})
		out = Settings{Method: m}
		return nil
	})
	return out, err
}

func methodLabel(m costing.Method) string {
	if m == costing.FIFO {
		return "FIFO"
	}
	return "rata-rata bergerak"
}

// --- warehouses ------------------------------------------------------------------------------

// Warehouses are the company's warehouses, active first.
func (s *Service) Warehouses(ctx context.Context) ([]domain.Warehouse, error) {
	return s.repo.Warehouses(ctx, company(ctx))
}

func codeTaken(err error) error {
	if errors.Is(err, ErrCodeTaken) {
		return apperror.Validation([]validator.FieldError{{Field: "code", Message: "sudah dipakai gudang lain"}})
	}
	return err
}

// SaveWarehouse adds a warehouse (id 0) or changes one. stock.manage across the company.
func (s *Service) SaveWarehouse(ctx context.Context, id int64, w domain.Warehouse) (domain.Warehouse, error) {
	var v validator.Validator
	w.Clean(&v)
	if err := v.Err(); err != nil {
		return domain.Warehouse{}, err
	}
	if err := s.allowed(ctx, PermManage, 0); err != nil {
		return domain.Warehouse{}, err
	}
	co, now := company(ctx), s.clock.Now()
	w.CompanyID, w.UpdatedAt = co, now
	if err := s.checkBranch(ctx, co, w.BranchID); err != nil {
		return domain.Warehouse{}, err
	}
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if id == 0 {
			w.CreatedAt, w.IsActive = now, true
			newID, err := s.repo.InsertWarehouse(ctx, w)
			if err != nil {
				return codeTaken(err)
			}
			w.ID = newID
			s.audit.Record(ctx, kernel.AuditEntry{Module: "stock", Action: "warehouse.created", Target: target("warehouse", w.ID),
				Summary: "Menambah gudang " + w.Code + " " + w.Name})
			return nil
		}
		before, err := s.repo.WarehouseForUpdate(ctx, co, id)
		if err != nil {
			return notFound(err, "Gudang")
		}
		if before.BranchID != w.BranchID {
			has, err := s.repo.WarehouseHasStock(ctx, co, id)
			if err != nil {
				return err
			}
			if has {
				return apperror.Validation([]validator.FieldError{{Field: "branch_id", Message: "gudang yang masih berisi stok tidak bisa dipindah cabang"}})
			}
		}
		w.ID, w.IsActive, w.CreatedAt = id, before.IsActive, before.CreatedAt
		if err := s.repo.UpdateWarehouse(ctx, w); err != nil {
			return codeTaken(err)
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "stock", Action: "warehouse.changed", Target: target("warehouse", id),
			Summary: "Mengubah gudang " + w.Code + " " + w.Name})
		return nil
	})
	return w, err
}

// Branches are the company's active branches a warehouse may belong to; none without an
// organization module.
func (s *Service) Branches(ctx context.Context) ([]ports.Branch, error) {
	if s.branches == nil {
		return nil, nil
	}
	all, err := s.branches.Branches(ctx, company(ctx))
	if err != nil {
		return nil, err
	}
	var out []ports.Branch
	for _, b := range all {
		if b.IsActive {
			out = append(out, b)
		}
	}
	return out, nil
}

// checkBranch refuses a branch that is not an active branch of the company (0 is always fine).
func (s *Service) checkBranch(ctx context.Context, co, branch int64) error {
	if branch == 0 {
		return nil
	}
	bad := apperror.Validation([]validator.FieldError{{Field: "branch_id", Message: "cabang tidak dikenal atau nonaktif"}})
	if s.branches == nil {
		return bad
	}
	all, err := s.branches.Branches(ctx, co)
	if err != nil {
		return err
	}
	for _, b := range all {
		if b.ID == branch && b.IsActive {
			return nil
		}
	}
	return bad
}

// SetWarehouseActive deactivates or activates a warehouse. A warehouse still holding stock is
// not deactivated: move or adjust it out first.
func (s *Service) SetWarehouseActive(ctx context.Context, id int64, active bool) (domain.Warehouse, error) {
	if err := s.allowed(ctx, PermManage, 0); err != nil {
		return domain.Warehouse{}, err
	}
	co, now := company(ctx), s.clock.Now()
	var w domain.Warehouse
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		if w, err = s.repo.WarehouseForUpdate(ctx, co, id); err != nil {
			return notFound(err, "Gudang")
		}
		if w.IsActive == active {
			return nil
		}
		if !active {
			has, err := s.repo.WarehouseHasStock(ctx, co, id)
			if err != nil {
				return err
			}
			if has {
				return apperror.Conflict("Gudang " + w.Code + " masih berisi stok — kosongkan dengan pengeluaran atau penyesuaian dulu")
			}
		}
		if err := s.repo.SetWarehouseActive(ctx, co, id, active, now); err != nil {
			return err
		}
		w.IsActive, w.UpdatedAt = active, now
		verb, action := "Mengaktifkan", "warehouse.activated"
		if !active {
			verb, action = "Menonaktifkan", "warehouse.deactivated"
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "stock", Action: action, Target: target("warehouse", id), Summary: verb + " gudang " + w.Code})
		return nil
	})
	return w, err
}

// --- items -----------------------------------------------------------------------------------

// SearchItems are the stocked, active items matching q, for a line's picker.
func (s *Service) SearchItems(ctx context.Context, q string) ([]ports.Item, error) {
	rows, err := s.items.Search(ctx, strings.TrimSpace(q), 20)
	if err != nil {
		return nil, err
	}
	out := rows[:0]
	for _, it := range rows {
		if it.IsStocked {
			out = append(out, it)
		}
	}
	return out, nil
}

// ItemUnits are an item's units, base first.
func (s *Service) ItemUnits(ctx context.Context, id int64) ([]ports.Unit, error) {
	units, err := s.items.Units(ctx, id)
	if errors.Is(err, ports.ErrItemNotFound) {
		return nil, apperror.NotFound("Barang tidak ditemukan")
	}
	return units, err
}

// OnItemChanged keeps the stock module's copy of an item's code and name current — the
// subscriber of items.item.changed, wired in registry.go. The payload is decoded by the module
// root into id/code/name: no import of the items module.
func (s *Service) OnItemChanged(ctx context.Context, id int64, code, name string) error {
	return s.repo.RenameItem(ctx, company(ctx), id, code, name, s.clock.Now())
}

// item checks an item a movement may use, inside the transaction, and refreshes stock's copy of
// it. The item is held shared until the commit (ports.Items.Usable).
func (s *Service) item(ctx context.Context, co, id int64) (ports.Item, error) {
	if err := s.items.Usable(ctx, id); err != nil {
		switch {
		case errors.Is(err, ports.ErrItemNotFound):
			return ports.Item{}, refuse(ErrNotFound, "Barang #%d tidak ditemukan", id)
		case errors.Is(err, ports.ErrItemInactive):
			it, _ := s.items.Get(ctx, id)
			return ports.Item{}, refuse(ErrItemInactive, "Barang %s %s sudah nonaktif", it.Code, it.Name)
		}
		return ports.Item{}, err
	}
	it, err := s.items.Get(ctx, id)
	if err != nil {
		return ports.Item{}, err
	}
	if !it.IsStocked {
		return ports.Item{}, refuse(ErrNotStocked, "%s %s tidak dilacak stoknya", it.Code, it.Name)
	}
	return it, s.repo.UpsertItem(ctx, co, it, s.clock.Now())
}
