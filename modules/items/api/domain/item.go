// Package domain holds the items module's types and rules: what an item, a unit, a category, and
// an item's alternative unit must be to be saved. No I/O.
package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/validator"
)

// Kind is whether an item is goods or a service — Coretax's A and B. Fixed once created.
type Kind string

const (
	Goods   Kind = "goods"
	Service Kind = "service"
)

// Valid reports whether k is a kind this module knows.
func (k Kind) Valid() bool { return k == Goods || k == Service }

// NoTaxCode is the Coretax goods/service code of an item that names none: DJP accepts 000000.
const NoTaxCode = "000000"

// Item is a good or a service the company buys, keeps, or sells.
type Item struct {
	ID          int64
	CompanyID   int64
	Code        string
	Name        string
	Kind        Kind
	IsStocked   bool  // its stock is tracked; fixed, and never for a service
	CategoryID  int64 // 0: none
	BaseUnitID  int64 // fixed: stock and documents count in it
	Barcode     string
	TaxCode     string // six digits; NoTaxCode when none was given
	Description string
	IsActive    bool
	CreatedBy   int64
	CreatedAt   time.Time
	UpdatedBy   int64
	UpdatedAt   time.Time

	// Filled for display from the company's units and categories; not stored with the item.
	BaseUnitCode string
	BaseUnitName string
	CategoryName string
}

var (
	codeRe     = regexp.MustCompile(`^[A-Z0-9][A-Z0-9./-]{0,29}$`)
	prefixRe   = regexp.MustCompile(`^[A-Z0-9./-]{0,10}$`)
	barcodeRe  = regexp.MustCompile(`^[\x21-\x7E]{8,64}$`)
	taxCodeRe  = regexp.MustCompile(`^[0-9]{6}$`)
	unitCodeRe = regexp.MustCompile(`^[A-Z0-9]{1,10}$`)
	taxUnitRe  = regexp.MustCompile(`^UM\.[0-9]{4}$`)
)

// NormalizeCode is a code as stored: trimmed, in capitals.
func NormalizeCode(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// ValidCode reports whether a typed item code may be stored: capitals, digits, . / -, up to 30.
func ValidCode(s string) bool { return codeRe.MatchString(s) }

// GeneratedCode is the code taken from a company's counter: prefix and n in five digits.
func GeneratedCode(prefix string, n int64) string { return fmt.Sprintf("%s%05d", prefix, n) }

// ValidPrefix reports whether a code prefix may be stored.
func ValidPrefix(s string) bool { return prefixRe.MatchString(s) }

// Clean trims and normalizes the item's own fields, then records every field error in v. The
// code is checked by the caller: "" means "generate one" only when the item is created.
func (it *Item) Clean(v *validator.Validator) {
	for _, f := range []*string{&it.Name, &it.Barcode, &it.TaxCode, &it.Description} {
		*f = strings.TrimSpace(*f)
	}
	if it.TaxCode == "" {
		it.TaxCode = NoTaxCode
	}
	v.Required("name", it.Name)
	v.MaxLen("name", it.Name, 150)
	v.Check(it.Kind.Valid(), "kind", "pilih barang atau jasa")
	v.Check(it.Kind != Service || !it.IsStocked, "is_stocked", "jasa tidak dilacak stoknya")
	v.Check(it.BaseUnitID > 0, "base_unit_id", "pilih satuan dasar")
	v.Check(it.Barcode == "" || barcodeRe.MatchString(it.Barcode), "barcode", "8–64 huruf, angka, atau tanda baca, tanpa spasi")
	v.Check(taxCodeRe.MatchString(it.TaxCode), "tax_code", "6 angka; 000000 bila tidak diisi")
	v.MaxLen("description", it.Description, 2000)
}

// Unit is a unit of measure of the company. Its code is fixed once created: documents copy it.
type Unit struct {
	ID          int64
	CompanyID   int64
	Code        string
	Name        string
	TaxUnitCode string // Coretax "UM.0021", or ""
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidUnitCode reports whether a unit code may be stored: capitals and digits, up to 10.
func ValidUnitCode(s string) bool { return unitCodeRe.MatchString(s) }

// Clean trims u, puts its code in capitals, and records every field error in v.
func (u *Unit) Clean(v *validator.Validator) {
	u.Code = NormalizeCode(u.Code)
	u.Name = strings.TrimSpace(u.Name)
	u.TaxUnitCode = strings.ToUpper(strings.TrimSpace(u.TaxUnitCode))
	v.Check(ValidUnitCode(u.Code), "code", "huruf besar dan angka, maksimal 10, mis. PCS")
	v.Required("name", u.Name)
	v.MaxLen("name", u.Name, 60)
	v.Check(u.TaxUnitCode == "" || taxUnitRe.MatchString(u.TaxUnitCode), "tax_unit_code", "kosong, atau UM. diikuti 4 angka, mis. UM.0021")
}

// CommonUnits are the units most companies start with, each with its Coretax unit code (the
// Coretax unit list as in Odoo's l10n_id_efaktur_coretax, data/uom.code.csv).
var CommonUnits = []Unit{
	{Code: "PCS", Name: "Piece", TaxUnitCode: "UM.0021"},
	{Code: "UNIT", Name: "Unit", TaxUnitCode: "UM.0018"},
	{Code: "KG", Name: "Kilogram", TaxUnitCode: "UM.0003"},
	{Code: "G", Name: "Gram", TaxUnitCode: "UM.0004"},
	{Code: "L", Name: "Liter", TaxUnitCode: "UM.0007"},
	{Code: "M", Name: "Meter", TaxUnitCode: "UM.0013"},
	{Code: "DUS", Name: "Dus", TaxUnitCode: "UM.0022"},
	{Code: "LSN", Name: "Lusin", TaxUnitCode: "UM.0017"},
	{Code: "SET", Name: "Set", TaxUnitCode: "UM.0019"},
	{Code: "LBR", Name: "Lembar", TaxUnitCode: "UM.0020"},
	{Code: "ROL", Name: "Roll", TaxUnitCode: "UM.0039"},
	{Code: "KRT", Name: "Karton", TaxUnitCode: "UM.0037"},
}

// Category groups items for lists and reports.
type Category struct {
	ID        int64
	CompanyID int64
	Name      string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Clean trims c and records every field error in v.
func (c *Category) Clean(v *validator.Validator) {
	c.Name = strings.TrimSpace(c.Name)
	v.Required("name", c.Name)
	v.MaxLen("name", c.Name, 100)
}

// ItemUnit is an alternative unit of an item: one of it is Factor base units. The factor is
// fixed — a wrong one is deactivated and another unit added.
type ItemUnit struct {
	ID        int64
	CompanyID int64
	ItemID    int64
	UnitID    int64
	Factor    money.Rate
	IsActive  bool
	CreatedAt time.Time

	// The unit's, for display.
	UnitCode string
	UnitName string
}

// ValidFactor reports whether f may be an alternative unit's factor: more than zero. Its digits
// (at most 9 whole, 10 decimals) are what money.ParseRate already refuses beyond.
func ValidFactor(f money.Rate) bool { return f.Cmp(money.Rate{}) > 0 }
