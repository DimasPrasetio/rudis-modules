package application

import (
	"context"
	"errors"
	"strings"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/items/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/validator"
)

// --- an item's alternative units ---

// AddUnit gives an item an alternative unit: one unit is factor base units. A unit the item had
// and lost comes back only with the same factor — a factor never changes, since documents
// copied it.
func (s *Service) AddUnit(ctx context.Context, itemID, unitID int64, factor money.Rate) (domain.ItemUnit, error) {
	if !domain.ValidFactor(factor) {
		return domain.ItemUnit{}, fieldError("factor", "lebih dari 0")
	}
	co := company(ctx)
	now := s.clock.Now()
	var saved domain.ItemUnit
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		it, err := s.repo.GetForUpdate(ctx, co, itemID)
		if err != nil {
			return notFound(err)
		}
		if unitID == it.BaseUnitID {
			return fieldError("unit_id", "ini satuan dasar barang — pilih satuan lain")
		}
		u, err := s.repo.Unit(ctx, co, unitID)
		switch {
		case errors.Is(err, ErrNotFound):
			return fieldError("unit_id", "satuan tidak ditemukan")
		case err != nil:
			return err
		case !u.IsActive:
			return fieldError("unit_id", "satuan "+u.Code+" nonaktif — aktifkan dulu di daftar satuan")
		}
		iu, err := s.repo.ItemUnitOf(ctx, co, itemID, unitID)
		switch {
		case errors.Is(err, ErrNotFound):
			iu = domain.ItemUnit{CompanyID: co, ItemID: itemID, UnitID: unitID, Factor: factor, IsActive: true, CreatedAt: now}
			if iu.ID, err = s.repo.InsertItemUnit(ctx, iu); err != nil {
				return err
			}
		case err != nil:
			return err
		case iu.IsActive:
			return apperror.Conflict("Satuan " + u.Code + " sudah ada untuk barang ini")
		case !iu.Factor.Equal(factor):
			return apperror.Conflict("Satuan " + u.Code + " pernah dipakai barang ini dengan faktor " + iu.Factor.String() +
				". Faktor tidak bisa diubah: aktifkan lagi dengan faktor yang sama, atau pakai satuan lain.")
		default:
			if err := s.repo.SetItemUnitActive(ctx, co, itemID, iu.ID, true); err != nil {
				return err
			}
			iu.IsActive = true
		}
		iu.UnitCode, iu.UnitName = u.Code, u.Name
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "item.unit_added", Target: target("item", itemID),
			Summary: "Satuan " + u.Code + " (1 " + u.Code + " = " + factor.String() + " " + s.baseCode(ctx, co, it) + ") untuk " + it.Code,
			Data:    map[string]any{"item_unit_id": iu.ID, "unit": u.Code, "factor": factor.String()}})
		saved = iu
		return s.changed(ctx, it, "unit")
	})
	return saved, err
}

// baseCode is the code of the item's base unit, or "" when it cannot be read (for summaries).
func (s *Service) baseCode(ctx context.Context, co int64, it domain.Item) string {
	u, err := s.repo.Unit(ctx, co, it.BaseUnitID)
	if err != nil {
		return ""
	}
	return u.Code
}

// DeactivateUnit takes an alternative unit from an item: new documents may not count the item
// in it; documents that did keep their copy of the factor.
func (s *Service) DeactivateUnit(ctx context.Context, itemID, id int64) error {
	co := company(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		it, err := s.repo.GetForUpdate(ctx, co, itemID)
		if err != nil {
			return notFound(err)
		}
		iu, err := s.repo.ItemUnit(ctx, co, itemID, id)
		if errors.Is(err, ErrNotFound) {
			return apperror.NotFound("Satuan barang tidak ditemukan")
		}
		if err != nil || !iu.IsActive {
			return err
		}
		if err := s.repo.SetItemUnitActive(ctx, co, itemID, id, false); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "item.unit_deactivated", Target: target("item", itemID),
			Summary: "Menonaktifkan satuan " + iu.UnitCode + " barang " + it.Code, Data: map[string]any{"item_unit_id": id, "unit": iu.UnitCode}})
		return s.changed(ctx, it, "unit")
	})
}

// --- the company's units ---

// Units are the company's units, active or not, by code.
func (s *Service) Units(ctx context.Context) ([]domain.Unit, error) {
	return s.repo.Units(ctx, company(ctx))
}

func unitTaken(err error) error {
	if errors.Is(err, ErrCodeTaken) {
		return fieldError("code", "satuan dengan kode ini sudah ada")
	}
	return err
}

// CreateUnit adds a unit to the company. Its code is fixed from now on.
func (s *Service) CreateUnit(ctx context.Context, u domain.Unit) (domain.Unit, error) {
	var v validator.Validator
	u.Clean(&v)
	if err := v.Err(); err != nil {
		return domain.Unit{}, err
	}
	now := s.clock.Now()
	u.CompanyID, u.IsActive, u.CreatedAt, u.UpdatedAt = company(ctx), true, now, now
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		id, err := s.repo.InsertUnit(ctx, u)
		if err != nil {
			return unitTaken(err)
		}
		u.ID = id
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "unit.created", Target: target("unit", id),
			Summary: "Menambah satuan " + u.Code + " " + u.Name, Data: map[string]any{"code": u.Code, "tax_unit_code": u.TaxUnitCode}})
		return nil
	})
	return u, err
}

// UnitPatch is a partial change of a unit; its code is fixed.
type UnitPatch struct {
	Name        *string
	TaxUnitCode *string
	IsActive    *bool
}

// UpdateUnit changes a unit's name, Coretax code, or whether new items and alternative units may
// pick it. Items that have it keep it.
func (s *Service) UpdateUnit(ctx context.Context, id int64, p UnitPatch) (domain.Unit, error) {
	co := company(ctx)
	var saved domain.Unit
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		before, err := s.repo.UnitForUpdate(ctx, co, id)
		if errors.Is(err, ErrNotFound) {
			return apperror.NotFound("Satuan tidak ditemukan")
		}
		if err != nil {
			return err
		}
		u := before
		if p.Name != nil {
			u.Name = *p.Name
		}
		if p.TaxUnitCode != nil {
			u.TaxUnitCode = *p.TaxUnitCode
		}
		if p.IsActive != nil {
			u.IsActive = *p.IsActive
		}
		var v validator.Validator
		u.Clean(&v)
		if err := v.Err(); err != nil {
			return err
		}
		saved = u
		if u.Name == before.Name && u.TaxUnitCode == before.TaxUnitCode && u.IsActive == before.IsActive {
			return nil
		}
		u.UpdatedAt = s.clock.Now()
		if err := s.repo.UpdateUnit(ctx, u); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "unit.updated", Target: target("unit", id),
			Summary: "Mengubah satuan " + u.Code + " " + u.Name,
			Data: map[string]any{"before": map[string]any{"name": before.Name, "tax_unit_code": before.TaxUnitCode, "is_active": before.IsActive},
				"after": map[string]any{"name": u.Name, "tax_unit_code": u.TaxUnitCode, "is_active": u.IsActive}}})
		saved = u
		return nil
	})
	return saved, err
}

// AddCommonUnits adds the common units (domain.CommonUnits) the company does not have yet — a
// code it has is passed over, whatever its name. Returns the codes added.
func (s *Service) AddCommonUnits(ctx context.Context) ([]string, error) {
	co := company(ctx)
	now := s.clock.Now()
	added := []string{}
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		added = added[:0] // d.Tx may run fn again
		for _, u := range domain.CommonUnits {
			u.CompanyID, u.IsActive, u.CreatedAt, u.UpdatedAt = co, true, now, now
			ok, err := s.repo.InsertUnitIfMissing(ctx, u)
			if err != nil {
				return err
			}
			if ok {
				added = append(added, u.Code)
			}
		}
		if len(added) > 0 {
			s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "unit.defaults_added", Target: "items:units",
				Summary: "Menambahkan satuan umum " + strings.Join(added, ", "), Data: map[string]any{"codes": added}})
		}
		return nil
	})
	return added, err
}

// --- the company's categories ---

// Categories are the company's categories, active or not, by name.
func (s *Service) Categories(ctx context.Context) ([]domain.Category, error) {
	return s.repo.Categories(ctx, company(ctx))
}

func nameTaken(err error) error {
	if errors.Is(err, ErrNameTaken) {
		return fieldError("name", "kategori dengan nama ini sudah ada")
	}
	return err
}

// CreateCategory adds a category to the company.
func (s *Service) CreateCategory(ctx context.Context, c domain.Category) (domain.Category, error) {
	var v validator.Validator
	c.Clean(&v)
	if err := v.Err(); err != nil {
		return domain.Category{}, err
	}
	now := s.clock.Now()
	c.CompanyID, c.IsActive, c.CreatedAt, c.UpdatedAt = company(ctx), true, now, now
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		id, err := s.repo.InsertCategory(ctx, c)
		if err != nil {
			return nameTaken(err)
		}
		c.ID = id
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "category.created", Target: target("category", id), Summary: "Menambah kategori " + c.Name})
		return nil
	})
	return c, err
}

// CategoryPatch is a partial change of a category.
type CategoryPatch struct {
	Name     *string
	IsActive *bool
}

// UpdateCategory renames a category or changes whether new items may pick it. Items in it stay.
func (s *Service) UpdateCategory(ctx context.Context, id int64, p CategoryPatch) (domain.Category, error) {
	co := company(ctx)
	var saved domain.Category
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		before, err := s.repo.CategoryForUpdate(ctx, co, id)
		if errors.Is(err, ErrNotFound) {
			return apperror.NotFound("Kategori tidak ditemukan")
		}
		if err != nil {
			return err
		}
		c := before
		if p.Name != nil {
			c.Name = *p.Name
		}
		if p.IsActive != nil {
			c.IsActive = *p.IsActive
		}
		var v validator.Validator
		c.Clean(&v)
		if err := v.Err(); err != nil {
			return err
		}
		saved = c
		if c.Name == before.Name && c.IsActive == before.IsActive {
			return nil
		}
		c.UpdatedAt = s.clock.Now()
		if err := s.repo.UpdateCategory(ctx, c); err != nil {
			return nameTaken(err)
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "items", Action: "category.updated", Target: target("category", id),
			Summary: "Mengubah kategori " + c.Name, Data: map[string]any{"before": map[string]any{"name": before.Name, "is_active": before.IsActive},
				"after": map[string]any{"name": c.Name, "is_active": c.IsActive}}})
		saved = c
		return nil
	})
	return saved, err
}

// --- for other modules (contracts.ItemCatalog) ---

// Get returns an item of the company, active or not, with its base unit; ErrNotFound when there
// is none.
func (s *Service) Get(ctx context.Context, id int64) (domain.Item, error) {
	co := company(ctx)
	it, err := s.repo.Get(ctx, co, id)
	if err != nil {
		return domain.Item{}, err
	}
	u, err := s.repo.Unit(ctx, co, it.BaseUnitID)
	if err != nil {
		return domain.Item{}, err
	}
	it.BaseUnitCode, it.BaseUnitName = u.Code, u.Name
	return it, nil
}

// Search finds up to limit (1–50) active items: the code starting with q, the name containing
// it, or the barcode equal to it.
func (s *Service) Search(ctx context.Context, q string, limit int) ([]domain.Item, error) {
	limit = max(1, min(limit, 50))
	q = strings.TrimSpace(q)
	co := company(ctx)
	rows, err := s.repo.Search(ctx, co, q, likeEscape(q), limit)
	if err != nil {
		return nil, err
	}
	return rows, s.describe(ctx, co, rows)
}

// ErrInactive and ErrUnknownUnit are what Usable and Measure refuse with.
var (
	ErrInactive    = errors.New("items: inactive")
	ErrUnknownUnit = errors.New("items: unknown unit")
)

// Usable checks, holding the row shared until the caller's transaction ends, that a document
// may use the item now.
func (s *Service) Usable(ctx context.Context, id int64) error {
	it, err := s.repo.GetShared(ctx, company(ctx), id)
	switch {
	case err != nil:
		return err
	case !it.IsActive:
		return ErrInactive
	}
	return nil
}

// Measure is a unit an item is counted in, and what one of it is in the base unit.
type Measure struct {
	Code   string
	Name   string
	Factor money.Rate
	IsBase bool
}

var baseFactor = money.MustRate("1")

func (s *Service) base(ctx context.Context, co, id int64) (Measure, error) {
	it, err := s.repo.Get(ctx, co, id)
	if err != nil {
		return Measure{}, err
	}
	u, err := s.repo.Unit(ctx, co, it.BaseUnitID)
	if err != nil {
		return Measure{}, err
	}
	return Measure{Code: u.Code, Name: u.Name, Factor: baseFactor, IsBase: true}, nil
}

// Measure is the item's unit code: "" or the base unit's code is the base unit (factor 1), an
// active alternative unit its factor; anything else ErrUnknownUnit.
func (s *Service) Measure(ctx context.Context, id int64, code string) (Measure, error) {
	co := company(ctx)
	b, err := s.base(ctx, co, id)
	if err != nil {
		return Measure{}, err
	}
	code = domain.NormalizeCode(code)
	if code == "" || code == b.Code {
		return b, nil
	}
	u, err := s.repo.UnitByCode(ctx, co, code)
	if errors.Is(err, ErrNotFound) {
		return Measure{}, ErrUnknownUnit
	}
	if err != nil {
		return Measure{}, err
	}
	iu, err := s.repo.ItemUnitOf(ctx, co, id, u.ID)
	if errors.Is(err, ErrNotFound) || (err == nil && !iu.IsActive) {
		return Measure{}, ErrUnknownUnit
	}
	if err != nil {
		return Measure{}, err
	}
	return Measure{Code: u.Code, Name: u.Name, Factor: iu.Factor}, nil
}

// Measures are the units the item may be counted in: the base unit, then its active alternative
// units by factor.
func (s *Service) Measures(ctx context.Context, id int64) ([]Measure, error) {
	co := company(ctx)
	b, err := s.base(ctx, co, id)
	if err != nil {
		return nil, err
	}
	ius, err := s.repo.ItemUnits(ctx, co, id)
	if err != nil {
		return nil, err
	}
	out := []Measure{b}
	for _, iu := range ius {
		if iu.IsActive {
			out = append(out, Measure{Code: iu.UnitCode, Name: iu.UnitName, Factor: iu.Factor})
		}
	}
	return out, nil
}
