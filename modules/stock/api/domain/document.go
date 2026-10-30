// Package domain holds the stock module's types and the rules a document must meet before it is
// saved. No I/O.
package domain

import (
	"fmt"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/shared/docstate"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/validator"
)

// Kind is what a document does to stock.
type Kind string

const (
	Receipt    Kind = "receipt"    // barang masuk: every line adds
	Issue      Kind = "issue"      // barang keluar: every line takes
	Adjustment Kind = "adjustment" // penyesuaian: a line adds (found, + qty) or takes (lost, − qty)
)

// Valid reports whether k is one of the three kinds.
func (k Kind) Valid() bool { return k == Receipt || k == Issue || k == Adjustment }

// Label is the kind as people read it.
func (k Kind) Label() string {
	switch k {
	case Receipt:
		return "Penerimaan barang"
	case Issue:
		return "Pengeluaran barang"
	case Adjustment:
		return "Penyesuaian stok"
	}
	return string(k)
}

// MaxLines bounds one document.
const MaxLines = 200

// Document is a receipt, an issue, or an adjustment of one warehouse.
type Document struct {
	ID           int64
	CompanyID    int64
	BranchID     int64 // the warehouse's
	Kind         Kind
	WarehouseID  int64
	Date         kernel.Date
	Number       string
	Counterparty string
	Reference    string
	Notes        string
	docstate.Fields
	CreatedBy int64
	CreatedAt time.Time
	UpdatedBy int64
	UpdatedAt time.Time
	Lines     []Line
}

// Line is one item of a document.
type Line struct {
	ID       int64
	LineNo   int
	ItemID   int64
	ItemCode string
	ItemName string
	UnitCode string
	Factor   money.Rate // base units per UnitCode
	Qty      money.Qty  // in UnitCode; negative on an adjustment line that takes
	BaseQty  money.Qty  // Qty × Factor, signed as the ledger moves it: negative on an issue
	// UnitCost is the cost of one UnitCode coming in. On a receipt it is required; on an
	// adjustment that adds, nil takes the current average; going out it is always nil.
	UnitCost *money.Amount
	ZeroCost bool          // received at no cost on purpose
	Value    *money.Amount // set when posted: + added, − taken
}

// Clean trims the document's text, checks what can be checked without the database, and drops
// what the kind does not use (a cost on a line going out).
func (d *Document) Clean(v *validator.Validator) {
	d.Counterparty = strings.TrimSpace(d.Counterparty)
	d.Reference = strings.TrimSpace(d.Reference)
	d.Notes = strings.TrimSpace(d.Notes)
	v.Check(d.Kind.Valid(), "kind", "pilih penerimaan, pengeluaran, atau penyesuaian")
	v.Check(d.WarehouseID > 0, "warehouse_id", "pilih gudang")
	v.Check(d.Date.Year > 0, "date", "isi tanggal dokumen")
	v.MaxLen("counterparty", d.Counterparty, 150)
	v.MaxLen("reference", d.Reference, 60)
	v.MaxLen("notes", d.Notes, 2000)
	switch {
	case len(d.Lines) == 0:
		v.Check(false, "lines", "isi paling sedikit satu barang")
	case len(d.Lines) > MaxLines:
		v.Check(false, "lines", fmt.Sprintf("paling banyak %d baris dalam satu dokumen", MaxLines))
	}
	for i := range d.Lines {
		l := &d.Lines[i]
		l.LineNo = i + 1
		l.UnitCode = strings.ToUpper(strings.TrimSpace(l.UnitCode))
		f := func(name string) string { return fmt.Sprintf("lines.%d.%s", i, name) }
		v.Check(l.ItemID > 0, f("item_id"), "pilih barang")
		switch d.Kind {
		case Receipt:
			v.Check(l.Qty.Sign() > 0, f("qty"), "jumlah harus lebih dari 0")
			if l.ZeroCost {
				zero := money.Amount{}
				l.UnitCost = &zero
			}
			switch {
			case l.UnitCost == nil:
				v.Check(false, f("unit_cost"), "isi biaya per satuan")
			case l.UnitCost.Sign() < 0:
				v.Check(false, f("unit_cost"), "biaya tidak boleh negatif")
			case l.UnitCost.IsZero() && !l.ZeroCost:
				v.Check(false, f("unit_cost"), "biaya 0 hanya untuk barang tanpa biaya — centang \"Tanpa biaya\"")
			}
		case Issue:
			v.Check(l.Qty.Sign() > 0, f("qty"), "jumlah harus lebih dari 0")
			l.UnitCost, l.ZeroCost = nil, false
		case Adjustment:
			v.Check(!l.Qty.IsZero(), f("qty"), "isi tambahan (+) atau pengurangan (−), bukan 0")
			if l.Qty.Sign() < 0 {
				l.UnitCost, l.ZeroCost = nil, false
			} else if l.UnitCost != nil {
				v.Check(l.UnitCost.Sign() >= 0, f("unit_cost"), "biaya tidak boleh negatif")
				l.ZeroCost = l.UnitCost.IsZero()
			}
		}
	}
}

// Warehouse is a place stock is kept.
type Warehouse struct {
	ID        int64
	CompanyID int64
	BranchID  int64 // 0: the company as a whole
	Code      string
	Name      string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Clean normalizes a warehouse and checks it.
func (w *Warehouse) Clean(v *validator.Validator) {
	w.Code = strings.ToUpper(strings.TrimSpace(w.Code))
	w.Name = strings.TrimSpace(w.Name)
	v.Required("code", w.Code)
	v.Check(validCode(w.Code), "code", "huruf besar, angka, . / -; maksimal 20")
	v.Required("name", w.Name)
	v.MaxLen("name", w.Name, 100)
	v.Check(w.BranchID >= 0, "branch_id", "cabang tidak dikenal")
}

func validCode(s string) bool {
	if s == "" || len(s) > 20 {
		return s == ""
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '/' || r == '-') {
			return false
		}
	}
	return true
}

// DateBefore reports whether a is an earlier day than b.
func DateBefore(a, b kernel.Date) bool { return a.String() < b.String() }

// FormatDate is d as people read it: 30/10/2026.
func FormatDate(d kernel.Date) string { return fmt.Sprintf("%02d/%02d/%04d", d.Day, d.Month, d.Year) }
