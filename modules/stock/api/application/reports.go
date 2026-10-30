package application

import (
	"context"
	"errors"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/stock/domain"
	"__GO_MODULE__/internal/modules/stock/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/costing"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/validator"
)

// Stock is a page of what is in stock, and the value of everything the filter's warehouse holds.
type Stock struct {
	Rows  []BalanceRow
	Total int64
	Value money.Amount
}

// Balances is what each item holds in each warehouse.
func (s *Service) Balances(ctx context.Context, f BalanceFilter) (Stock, error) {
	co := company(ctx)
	rows, total, err := s.repo.Balances(ctx, co, f)
	if err != nil {
		return Stock{}, err
	}
	value, err := s.repo.TotalValue(ctx, co, f.WarehouseID)
	return Stock{Rows: rows, Total: total, Value: value}, err
}

// UnitCostOf is a balance's average cost of one base unit; false when it holds nothing.
func UnitCostOf(qty money.Qty, value money.Amount) (money.Amount, bool) {
	return costing.UnitCost(costing.Position{Qty: qty, Value: value})
}

// Card is an item's stock card for a period: its balance per warehouse before the first day,
// then every movement in date order, each with the document it came from.
type Card struct {
	Item     ports.Item
	From, To kernel.Date
	Openings []Opening
	Moves    []Move
	Docs     map[int64]DocRef
}

// maxDays bounds a card or report period.
const maxDays = 3660

func checkPeriod(from, to kernel.Date) error {
	if from.Year == 0 || to.Year == 0 {
		return apperror.Validation([]validator.FieldError{{Field: "from", Message: "isi periode"}})
	}
	if domain.DateBefore(to, from) {
		return apperror.Validation([]validator.FieldError{{Field: "to", Message: "tanggal akhir sebelum tanggal awal"}})
	}
	if to.Start(time.UTC).Sub(from.Start(time.UTC)).Hours()/24 > maxDays {
		return apperror.Validation([]validator.FieldError{{Field: "to", Message: "periode paling panjang 10 tahun"}})
	}
	return nil
}

// Card returns an item's stock card; warehouse 0 is every warehouse.
func (s *Service) Card(ctx context.Context, item, warehouse int64, from, to kernel.Date) (Card, error) {
	if err := checkPeriod(from, to); err != nil {
		return Card{}, err
	}
	co := company(ctx)
	it, err := s.items.Get(ctx, item)
	if err != nil {
		if errors.Is(err, ports.ErrItemNotFound) {
			return Card{}, apperror.NotFound("Barang tidak ditemukan")
		}
		return Card{}, err
	}
	openings, err := s.repo.Openings(ctx, co, item, warehouse, from)
	if err != nil {
		return Card{}, err
	}
	moves, err := s.repo.CardMoves(ctx, co, item, warehouse, from, to)
	if err != nil {
		return Card{}, err
	}
	var ids []int64
	seen := map[int64]bool{}
	for _, m := range moves {
		if m.DocumentID != 0 && !seen[m.DocumentID] {
			seen[m.DocumentID] = true
			ids = append(ids, m.DocumentID)
		}
	}
	docs := map[int64]DocRef{}
	if len(ids) > 0 {
		refs, err := s.repo.DocumentsByID(ctx, co, ids)
		if err != nil {
			return Card{}, err
		}
		for _, r := range refs {
			docs[r.ID] = r
		}
	}
	return Card{Item: it, From: from, To: to, Openings: openings, Moves: moves, Docs: docs}, nil
}

// Report is the movement report: opening, in, out, and closing per item and warehouse.
func (s *Service) Report(ctx context.Context, f ReportFilter) ([]ReportRow, error) {
	if err := checkPeriod(f.From, f.To); err != nil {
		return nil, err
	}
	return s.repo.MovementReport(ctx, company(ctx), f)
}
