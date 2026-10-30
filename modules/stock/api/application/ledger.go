package application

import (
	"context"
	"errors"
	"strconv"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/stock/domain"
	"__GO_MODULE__/internal/modules/stock/ports"
	"__GO_MODULE__/internal/shared/costing"
	"__GO_MODULE__/internal/shared/money"
)

// moveIn is a movement to record: goods in (Qty > 0) at Value, or out (Qty < 0) valued by the
// costing method.
type moveIn struct {
	WarehouseID int64
	ItemID      int64
	Date        kernel.Date
	Qty         money.Qty     // base unit, signed
	Value       *money.Amount // in: the whole value; nil takes the current average
	DocumentID  int64
	LineID      int64
	Source      string
	// Item and Warehouse name them in messages.
	Item      string
	Warehouse string
}

// apply records one movement inside the caller's transaction: it locks the item's balance in
// the warehouse, refuses a date before its last movement (no history is revalued) and a take of
// more than is there, values it, and writes the ledger row, the FIFO layers, and the balance.
func (s *Service) apply(ctx context.Context, co int64, method costing.Method, in moveIn) (Move, error) {
	now := s.clock.Now()
	b, err := s.repo.LockBalance(ctx, co, in.WarehouseID, in.ItemID, now)
	if err != nil {
		return Move{}, err
	}
	if b.LastDate != nil && domain.DateBefore(in.Date, *b.LastDate) {
		return Move{}, refuse(ErrBackdated, "Tanggal %s lebih awal dari mutasi terakhir %s di gudang %s (%s) — stok tidak dicatat mundur. Pakai tanggal %s atau sesudahnya.",
			domain.FormatDate(in.Date), in.Item, in.Warehouse, domain.FormatDate(*b.LastDate), domain.FormatDate(*b.LastDate))
	}
	pos := costing.Position{Qty: b.Qty, Value: b.Value}
	if method == costing.FIFO {
		if pos.Layers, err = s.repo.OpenLayers(ctx, co, in.WarehouseID, in.ItemID); err != nil {
			return Move{}, err
		}
	}
	mv := Move{WarehouseID: in.WarehouseID, ItemID: in.ItemID, Date: in.Date, Qty: in.Qty, DocumentID: in.DocumentID, LineID: in.LineID, Source: in.Source}
	var (
		layer costing.Layer
		takes []costing.Take
	)
	if in.Qty.Sign() > 0 {
		var value money.Amount
		if in.Value != nil {
			value = *in.Value
		} else {
			c, ok := costing.UnitCost(pos)
			if !ok {
				return Move{}, refuse(ErrNoCost, "Isi biaya %s: belum ada stok di gudang %s untuk dijadikan biaya rata-rata", in.Item, in.Warehouse)
			}
			if value, err = c.MulQty(in.Qty, money.HalfUp); err != nil {
				return Move{}, err
			}
		}
		if pos, layer, err = costing.Receive(pos, method, in.Qty, value); err != nil {
			return Move{}, err
		}
		mv.Value = value
	} else {
		var out money.Amount
		pos, out, takes, err = costing.Issue(pos, method, in.Qty.Neg())
		if errors.Is(err, costing.ErrInsufficient) {
			return Move{}, refuse(ErrInsufficient, "Stok %s di gudang %s tidak cukup: tersedia %s, diminta %s (satuan dasar)",
				in.Item, in.Warehouse, money.FormatQty(b.Qty), money.FormatQty(in.Qty.Neg()))
		}
		if err != nil {
			return Move{}, err
		}
		mv.Value = out.Neg()
	}
	mv.QtyAfter, mv.ValueAfter = pos.Qty, pos.Value
	if mv.ID, err = s.repo.InsertMove(ctx, co, mv, now); err != nil {
		return Move{}, err
	}
	if method == costing.FIFO {
		if in.Qty.Sign() > 0 {
			if _, err := s.repo.InsertLayer(ctx, co, in.WarehouseID, in.ItemID, mv.ID, layer); err != nil {
				return Move{}, err
			}
		} else {
			if err := s.saveTouched(ctx, co, pos.Layers, takes); err != nil {
				return Move{}, err
			}
			if err := s.repo.InsertTakes(ctx, co, mv.ID, takes); err != nil {
				return Move{}, err
			}
		}
	}
	date := in.Date
	err = s.repo.SaveBalance(ctx, co, Balance{WarehouseID: in.WarehouseID, ItemID: in.ItemID, Qty: pos.Qty, Value: pos.Value, LastMoveID: mv.ID, LastDate: &date}, now)
	return mv, err
}

// saveTouched writes the layers takes went through.
func (s *Service) saveTouched(ctx context.Context, co int64, layers []costing.Layer, takes []costing.Take) error {
	for _, t := range takes {
		for _, l := range layers {
			if l.ID == t.LayerID {
				if err := s.repo.SetLayer(ctx, co, l); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// reverse undoes one movement inside the caller's transaction, exactly: it must still be its
// item's last movement in the warehouse, so the balance and the FIFO layers go back to what they
// were. A reversing row is added; nothing is deleted from the ledger.
func (s *Service) reverse(ctx context.Context, co int64, method costing.Method, mv Move, item, warehouse string) error {
	now := s.clock.Now()
	b, err := s.repo.LockBalance(ctx, co, mv.WarehouseID, mv.ItemID, now)
	if err != nil {
		return err
	}
	notLast := func() error {
		return refuse(ErrNotLast, "%s di gudang %s sudah punya mutasi sesudah dokumen ini — batalkan dokumen yang lebih baru dulu, atau koreksi dengan dokumen penyesuaian",
			item, warehouse)
	}
	if b.LastMoveID != mv.ID {
		return notLast()
	}
	pos := costing.Position{Qty: b.Qty, Value: b.Value}
	if method == costing.FIFO {
		if pos.Layers, err = s.repo.OpenLayers(ctx, co, mv.WarehouseID, mv.ItemID); err != nil {
			return err
		}
	}
	if mv.Qty.Sign() > 0 {
		var layerID int64
		if method == costing.FIFO {
			l, err := s.repo.LayerOfMove(ctx, co, mv.ID)
			if err != nil {
				return err
			}
			layerID = l.ID
		}
		pos, err = costing.Unreceive(pos, method, layerID, mv.Qty, mv.Value)
		if errors.Is(err, costing.ErrConsumed) || errors.Is(err, costing.ErrInsufficient) {
			return notLast()
		}
		if err != nil {
			return err
		}
		if method == costing.FIFO {
			if err := s.repo.DeleteLayer(ctx, co, layerID); err != nil {
				return err
			}
		}
	} else {
		var takes []costing.Take
		if method == costing.FIFO {
			if takes, err = s.repo.Takes(ctx, co, mv.ID); err != nil {
				return err
			}
			touched, err := s.repo.LayersOfTakes(ctx, co, mv.ID)
			if err != nil {
				return err
			}
			pos.Layers = mergeLayers(pos.Layers, touched)
		}
		if pos, err = costing.Unissue(pos, method, mv.Qty.Neg(), mv.Value.Neg(), takes); err != nil {
			return err
		}
		if err := s.saveTouched(ctx, co, pos.Layers, takes); err != nil {
			return err
		}
	}
	rev := Move{WarehouseID: mv.WarehouseID, ItemID: mv.ItemID, Date: mv.Date, Qty: mv.Qty.Neg(), Value: mv.Value.Neg(),
		QtyAfter: pos.Qty, ValueAfter: pos.Value, DocumentID: mv.DocumentID, LineID: mv.LineID, Source: mv.Source, Reverses: mv.ID}
	if rev.ID, err = s.repo.InsertMove(ctx, co, rev, now); err != nil {
		return err
	}
	if err := s.repo.MarkReversed(ctx, co, mv.ID, rev.ID); err != nil {
		return err
	}
	nb := Balance{WarehouseID: mv.WarehouseID, ItemID: mv.ItemID, Qty: pos.Qty, Value: pos.Value}
	last, ok, err := s.repo.LastOpenMove(ctx, co, mv.WarehouseID, mv.ItemID)
	if err != nil {
		return err
	}
	if ok {
		d := last.Date
		nb.LastMoveID, nb.LastDate = last.ID, &d
	}
	return s.repo.SaveBalance(ctx, co, nb, now)
}

// mergeLayers is open plus the layers of touched it does not hold yet.
func mergeLayers(open, touched []costing.Layer) []costing.Layer {
	out := append([]costing.Layer(nil), open...)
	for _, t := range touched {
		found := false
		for _, o := range open {
			if o.ID == t.ID {
				found = true
				break
			}
		}
		if !found {
			out = append(out, t)
		}
	}
	return out
}

// lockMethod is the company's costing method, held shared for the transaction; the first
// movement locks it for good.
func (s *Service) lockMethod(ctx context.Context, co int64) (costing.Method, error) {
	st, err := s.repo.SettingsShared(ctx, co, s.clock.Now())
	if err != nil {
		return "", err
	}
	if !st.Locked {
		if err := s.repo.LockCostMethod(ctx, co, s.clock.Now()); err != nil {
			return "", err
		}
	}
	return st.Method, nil
}

// periodOpen refuses a date in a closed inventory period, when a period module is wired.
func (s *Service) periodOpen(ctx context.Context, date kernel.Date) error {
	if s.periods == nil {
		return nil
	}
	err := s.periods.EnsureOpen(ctx, Area, date)
	var closed *ports.PeriodClosedError
	if errors.As(err, &closed) {
		return refuse(ErrPeriodClosed, "%s", closed.Message)
	}
	return err
}

// levelChanged is the payload of stock.level.changed: an item's quantity in a warehouse after a
// commit. Subscribers (a marketplace sync, a reorder alert) read it, not the value.
type levelChanged struct {
	CompanyID   int64     `json:"company_id"`
	WarehouseID int64     `json:"warehouse_id"`
	ItemID      int64     `json:"item_id"`
	Qty         money.Qty `json:"qty"`
}

func (s *Service) publishLevel(ctx context.Context, co int64, mv Move) error {
	return s.events.Publish(ctx, "stock.level.changed", 1, levelChanged{CompanyID: co, WarehouseID: mv.WarehouseID, ItemID: mv.ItemID, Qty: mv.QtyAfter})
}

// --- the StockLedger contract ----------------------------------------------------------------

// OnHand is an item's quantity in a warehouse.
func (s *Service) OnHand(ctx context.Context, warehouse, item int64) (money.Qty, error) {
	b, err := s.repo.Balance(ctx, company(ctx), warehouse, item)
	return b.Qty, err
}

// PostMovement records a movement another module made (contracts.StockLedger.Post), inside its
// transaction. unitCost is per base unit; nil takes the current average.
func (s *Service) PostMovement(ctx context.Context, warehouse, itemID int64, date kernel.Date, qty money.Qty, unitCost *money.Amount, source string) (Move, error) {
	co := company(ctx)
	var mv Move
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if qty.IsZero() {
			return refuse(ErrInsufficient, "Jumlah mutasi tidak boleh 0")
		}
		if err := s.periodOpen(ctx, date); err != nil {
			return err
		}
		wh, err := s.repo.WarehouseShared(ctx, co, warehouse)
		if err != nil {
			return err
		}
		if !wh.IsActive {
			return refuse(ErrWhInactive, "Gudang %s nonaktif", wh.Code)
		}
		it, err := s.item(ctx, co, itemID)
		if err != nil {
			return err
		}
		method, err := s.lockMethod(ctx, co)
		if err != nil {
			return err
		}
		in := moveIn{WarehouseID: warehouse, ItemID: itemID, Date: date, Qty: qty, Source: source, Item: it.Code + " " + it.Name, Warehouse: wh.Code}
		if qty.Sign() > 0 && unitCost != nil {
			v, err := unitCost.MulQty(qty, money.HalfUp)
			if err != nil {
				return err
			}
			in.Value = &v
		}
		if mv, err = s.apply(ctx, co, method, in); err != nil {
			return err
		}
		return s.publishLevel(ctx, co, mv)
	})
	return mv, err
}

// ReverseSource undoes every movement still counted under source, newest first
// (contracts.StockLedger.Reverse).
func (s *Service) ReverseSource(ctx context.Context, source string) error {
	co := company(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		moves, err := s.repo.OpenMovesBySource(ctx, co, source)
		if err != nil {
			return err
		}
		if len(moves) == 0 {
			return nil
		}
		method, err := s.lockMethod(ctx, co)
		if err != nil {
			return err
		}
		for _, mv := range moves {
			if err := s.reverse(ctx, co, method, mv, "Barang #"+itoa(mv.ItemID), "#"+itoa(mv.WarehouseID)); err != nil {
				return err
			}
			b, err := s.repo.Balance(ctx, co, mv.WarehouseID, mv.ItemID)
			if err != nil {
				return err
			}
			mv.QtyAfter = b.Qty
			if err := s.publishLevel(ctx, co, mv); err != nil {
				return err
			}
		}
		return nil
	})
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
