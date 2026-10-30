// Package infrastructure stores items, their alternative units, the company's units and
// categories, and code settings in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/items/application"
	"__GO_MODULE__/internal/modules/items/domain"
	"__GO_MODULE__/internal/modules/items/infrastructure/sqlc"
	"__GO_MODULE__/internal/shared/money"
)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository { return &Repository{tx: tx} }

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func nullID(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: id != 0} }

// duplicate maps MySQL's duplicate-key error to the business key it hit: def for every key but
// an item's barcode.
func duplicate(err error, def error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		if strings.Contains(me.Message, "itm_items_barcode") {
			return application.ErrBarcodeTaken
		}
		return def
	}
	return err
}

func noRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

func toItem(row sqlc.ItmItem) domain.Item {
	return domain.Item{
		ID: row.ID, CompanyID: row.CompanyID, Code: row.Code, Name: row.Name, Kind: domain.Kind(row.Kind), IsStocked: row.IsStocked,
		CategoryID: row.CategoryID.Int64, BaseUnitID: row.BaseUnitID, Barcode: row.Barcode.String, TaxCode: row.TaxCode,
		Description: row.Description.String, IsActive: row.IsActive,
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt,
	}
}

func toItems(rows []sqlc.ItmItem) []domain.Item {
	out := make([]domain.Item, len(rows))
	for i, row := range rows {
		out[i] = toItem(row)
	}
	return out
}

func one(row sqlc.ItmItem, err error) (domain.Item, error) {
	if err != nil {
		return domain.Item{}, noRows(err)
	}
	return toItem(row), nil
}

func (r *Repository) List(ctx context.Context, company int64, f application.ListFilter) ([]domain.Item, int64, error) {
	rows, err := r.q(ctx).ListItems(ctx, sqlc.ListItemsParams{CompanyID: company, Status: f.Status, Kind: f.Kind, AnyCategory: f.CategoryID == 0,
		CategoryID: nullID(f.CategoryID), Pattern: f.Pattern, Search: nullString(f.Search), Limit: int32(f.Limit), Offset: int32(f.Offset)})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountItems(ctx, sqlc.CountItemsParams{CompanyID: company, Status: f.Status, Kind: f.Kind, AnyCategory: f.CategoryID == 0,
		CategoryID: nullID(f.CategoryID), Pattern: f.Pattern, Search: nullString(f.Search)})
	if err != nil {
		return nil, 0, err
	}
	return toItems(rows), total, nil
}

func (r *Repository) Search(ctx context.Context, company int64, search, pattern string, limit int) ([]domain.Item, error) {
	rows, err := r.q(ctx).SearchActiveItems(ctx, sqlc.SearchActiveItemsParams{CompanyID: company, Pattern: pattern, Search: nullString(search), Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	return toItems(rows), nil
}

func (r *Repository) Get(ctx context.Context, company, id int64) (domain.Item, error) {
	return one(r.q(ctx).GetItem(ctx, sqlc.GetItemParams{CompanyID: company, ID: id}))
}

func (r *Repository) GetShared(ctx context.Context, company, id int64) (domain.Item, error) {
	return one(r.q(ctx).GetItemShared(ctx, sqlc.GetItemSharedParams{CompanyID: company, ID: id}))
}

func (r *Repository) GetForUpdate(ctx context.Context, company, id int64) (domain.Item, error) {
	return one(r.q(ctx).GetItemForUpdate(ctx, sqlc.GetItemForUpdateParams{CompanyID: company, ID: id}))
}

func (r *Repository) CodeTaken(ctx context.Context, company int64, code string) (bool, error) {
	n, err := r.q(ctx).ItemCodeTaken(ctx, sqlc.ItemCodeTakenParams{CompanyID: company, Code: code})
	return n > 0, err
}

func (r *Repository) BarcodeOwner(ctx context.Context, company int64, barcode string, except int64) (string, error) {
	codes, err := r.q(ctx).BarcodeOwner(ctx, sqlc.BarcodeOwnerParams{CompanyID: company, Barcode: nullString(barcode), ID: except})
	if err != nil || len(codes) == 0 {
		return "", err
	}
	return codes[0], nil
}

func (r *Repository) Insert(ctx context.Context, it domain.Item) (int64, error) {
	id, err := r.q(ctx).InsertItem(ctx, sqlc.InsertItemParams{
		CompanyID: it.CompanyID, Code: it.Code, Name: it.Name, Kind: string(it.Kind), IsStocked: it.IsStocked,
		CategoryID: nullID(it.CategoryID), BaseUnitID: it.BaseUnitID, Barcode: nullString(it.Barcode), TaxCode: it.TaxCode,
		Description: nullString(it.Description), CreatedBy: it.CreatedBy, CreatedAt: it.CreatedAt, UpdatedBy: it.UpdatedBy, UpdatedAt: it.UpdatedAt,
	})
	return id, duplicate(err, application.ErrCodeTaken)
}

func (r *Repository) Update(ctx context.Context, it domain.Item) error {
	return duplicate(r.q(ctx).UpdateItem(ctx, sqlc.UpdateItemParams{
		Code: it.Code, Name: it.Name, CategoryID: nullID(it.CategoryID), Barcode: nullString(it.Barcode), TaxCode: it.TaxCode,
		Description: nullString(it.Description), UpdatedBy: it.UpdatedBy, UpdatedAt: it.UpdatedAt, CompanyID: it.CompanyID, ID: it.ID,
	}), application.ErrCodeTaken)
}

func (r *Repository) SetActive(ctx context.Context, company, id int64, active bool, by int64, at time.Time) error {
	return r.q(ctx).SetItemActive(ctx, sqlc.SetItemActiveParams{IsActive: active, UpdatedBy: by, UpdatedAt: at, CompanyID: company, ID: id})
}

func toUnit(row sqlc.ItmUnit) domain.Unit {
	return domain.Unit{ID: row.ID, CompanyID: row.CompanyID, Code: row.Code, Name: row.Name, TaxUnitCode: row.TaxUnitCode,
		IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func oneUnit(row sqlc.ItmUnit, err error) (domain.Unit, error) {
	if err != nil {
		return domain.Unit{}, noRows(err)
	}
	return toUnit(row), nil
}

func (r *Repository) Units(ctx context.Context, company int64) ([]domain.Unit, error) {
	rows, err := r.q(ctx).ListUnits(ctx, company)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Unit, len(rows))
	for i, row := range rows {
		out[i] = toUnit(row)
	}
	return out, nil
}

func (r *Repository) Unit(ctx context.Context, company, id int64) (domain.Unit, error) {
	return oneUnit(r.q(ctx).GetUnit(ctx, sqlc.GetUnitParams{CompanyID: company, ID: id}))
}

func (r *Repository) UnitForUpdate(ctx context.Context, company, id int64) (domain.Unit, error) {
	return oneUnit(r.q(ctx).GetUnitForUpdate(ctx, sqlc.GetUnitForUpdateParams{CompanyID: company, ID: id}))
}

func (r *Repository) UnitByCode(ctx context.Context, company int64, code string) (domain.Unit, error) {
	return oneUnit(r.q(ctx).GetUnitByCode(ctx, sqlc.GetUnitByCodeParams{CompanyID: company, Code: code}))
}

func (r *Repository) InsertUnit(ctx context.Context, u domain.Unit) (int64, error) {
	id, err := r.q(ctx).InsertUnit(ctx, sqlc.InsertUnitParams{CompanyID: u.CompanyID, Code: u.Code, Name: u.Name, TaxUnitCode: u.TaxUnitCode,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt})
	return id, duplicate(err, application.ErrCodeTaken)
}

func (r *Repository) InsertUnitIfMissing(ctx context.Context, u domain.Unit) (bool, error) {
	n, err := r.q(ctx).InsertUnitIfMissing(ctx, sqlc.InsertUnitIfMissingParams{CompanyID: u.CompanyID, Code: u.Code, Name: u.Name,
		TaxUnitCode: u.TaxUnitCode, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt})
	return n > 0, err
}

func (r *Repository) UpdateUnit(ctx context.Context, u domain.Unit) error {
	return r.q(ctx).UpdateUnit(ctx, sqlc.UpdateUnitParams{Name: u.Name, TaxUnitCode: u.TaxUnitCode, IsActive: u.IsActive, UpdatedAt: u.UpdatedAt,
		CompanyID: u.CompanyID, ID: u.ID})
}

func toCategory(row sqlc.ItmCategory) domain.Category {
	return domain.Category{ID: row.ID, CompanyID: row.CompanyID, Name: row.Name, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func oneCategory(row sqlc.ItmCategory, err error) (domain.Category, error) {
	if err != nil {
		return domain.Category{}, noRows(err)
	}
	return toCategory(row), nil
}

func (r *Repository) Categories(ctx context.Context, company int64) ([]domain.Category, error) {
	rows, err := r.q(ctx).ListCategories(ctx, company)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Category, len(rows))
	for i, row := range rows {
		out[i] = toCategory(row)
	}
	return out, nil
}

func (r *Repository) Category(ctx context.Context, company, id int64) (domain.Category, error) {
	return oneCategory(r.q(ctx).GetCategory(ctx, sqlc.GetCategoryParams{CompanyID: company, ID: id}))
}

func (r *Repository) CategoryForUpdate(ctx context.Context, company, id int64) (domain.Category, error) {
	return oneCategory(r.q(ctx).GetCategoryForUpdate(ctx, sqlc.GetCategoryForUpdateParams{CompanyID: company, ID: id}))
}

func (r *Repository) InsertCategory(ctx context.Context, c domain.Category) (int64, error) {
	id, err := r.q(ctx).InsertCategory(ctx, sqlc.InsertCategoryParams{CompanyID: c.CompanyID, Name: c.Name, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt})
	return id, duplicate(err, application.ErrNameTaken)
}

func (r *Repository) UpdateCategory(ctx context.Context, c domain.Category) error {
	return duplicate(r.q(ctx).UpdateCategory(ctx, sqlc.UpdateCategoryParams{Name: c.Name, IsActive: c.IsActive, UpdatedAt: c.UpdatedAt,
		CompanyID: c.CompanyID, ID: c.ID}), application.ErrNameTaken)
}

// itemUnit builds an alternative unit from a row; company is the caller's, as the query filtered.
func itemUnit(company, id, item, unit int64, factor string, active bool, at time.Time, code, name string) (domain.ItemUnit, error) {
	f, err := money.ParseRate(factor)
	if err != nil {
		return domain.ItemUnit{}, err
	}
	return domain.ItemUnit{ID: id, CompanyID: company, ItemID: item, UnitID: unit, Factor: f, IsActive: active, CreatedAt: at,
		UnitCode: code, UnitName: name}, nil
}

func (r *Repository) ItemUnits(ctx context.Context, company, item int64) ([]domain.ItemUnit, error) {
	rows, err := r.q(ctx).ListItemUnits(ctx, sqlc.ListItemUnitsParams{CompanyID: company, ItemID: item})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ItemUnit, len(rows))
	for i, row := range rows {
		if out[i], err = itemUnit(company, row.ID, row.ItemID, row.UnitID, row.Factor, row.IsActive, row.CreatedAt, row.Code, row.Name); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repository) ItemUnit(ctx context.Context, company, item, id int64) (domain.ItemUnit, error) {
	row, err := r.q(ctx).GetItemUnit(ctx, sqlc.GetItemUnitParams{CompanyID: company, ItemID: item, ID: id})
	if err != nil {
		return domain.ItemUnit{}, noRows(err)
	}
	return itemUnit(company, row.ID, row.ItemID, row.UnitID, row.Factor, row.IsActive, row.CreatedAt, row.Code, row.Name)
}

func (r *Repository) ItemUnitOf(ctx context.Context, company, item, unit int64) (domain.ItemUnit, error) {
	row, err := r.q(ctx).GetItemUnitOf(ctx, sqlc.GetItemUnitOfParams{CompanyID: company, ItemID: item, UnitID: unit})
	if err != nil {
		return domain.ItemUnit{}, noRows(err)
	}
	return itemUnit(company, row.ID, row.ItemID, row.UnitID, row.Factor, row.IsActive, row.CreatedAt, "", "")
}

func (r *Repository) InsertItemUnit(ctx context.Context, iu domain.ItemUnit) (int64, error) {
	return r.q(ctx).InsertItemUnit(ctx, sqlc.InsertItemUnitParams{CompanyID: iu.CompanyID, ItemID: iu.ItemID, UnitID: iu.UnitID,
		Factor: iu.Factor.String(), CreatedAt: iu.CreatedAt})
}

func (r *Repository) SetItemUnitActive(ctx context.Context, company, item, id int64, active bool) error {
	return r.q(ctx).SetItemUnitActive(ctx, sqlc.SetItemUnitActiveParams{IsActive: active, CompanyID: company, ItemID: item, ID: id})
}

func (r *Repository) Settings(ctx context.Context, company int64) (application.Settings, error) {
	row, err := r.q(ctx).GetSettings(ctx, company)
	if errors.Is(err, sql.ErrNoRows) {
		return application.Settings{CodePrefix: application.DefaultPrefix}, nil
	}
	return application.Settings{CodePrefix: row.CodePrefix, LastNumber: row.LastNumber}, err
}

func (r *Repository) LockSettings(ctx context.Context, company int64, at time.Time) (application.Settings, error) {
	if err := r.q(ctx).EnsureSettings(ctx, sqlc.EnsureSettingsParams{CompanyID: company, UpdatedAt: at}); err != nil {
		return application.Settings{}, err
	}
	row, err := r.q(ctx).LockSettings(ctx, company)
	return application.Settings{CodePrefix: row.CodePrefix, LastNumber: row.LastNumber}, err
}

func (r *Repository) SetLastNumber(ctx context.Context, company, n int64) error {
	return r.q(ctx).SetLastNumber(ctx, sqlc.SetLastNumberParams{LastNumber: n, CompanyID: company})
}

func (r *Repository) SetPrefix(ctx context.Context, company int64, prefix string, at time.Time) error {
	return r.q(ctx).SetPrefix(ctx, sqlc.SetPrefixParams{CodePrefix: prefix, UpdatedAt: at, CompanyID: company})
}
