// Package costing values stock: what goods cost when they come in and what they take with them
// when they go out, by moving average or FIFO — the two methods Indonesian accounting (PSAK 202)
// and tax (UU PPh Pasal 10(6)) accept; LIFO is not offered.
//
// Everything is in the item's base unit: quantities are money.Qty, values money.Amount. Nothing
// is rounded twice and nothing is left behind: the value a take carries out and the value that
// stays always add up to what was there (money.Allocate), so a position with no quantity left has
// no value left either. The kit keeps no state and touches no database — a stock module loads a
// Position, calls one function, and stores what it returns.
package costing

import (
	"errors"
	"fmt"

	"__GO_MODULE__/internal/shared/money"
)

// Method is how goods going out are valued.
type Method string

const (
	// Average is the moving (perpetual weighted) average: every take leaves at the position's
	// average cost.
	Average Method = "average"
	// FIFO takes from the oldest receipt still holding goods first.
	FIFO Method = "fifo"
)

var (
	// ErrMethod is a method other than Average or FIFO.
	ErrMethod = errors.New("costing: unknown method")
	// ErrQty is a quantity that is zero or negative where a positive one is needed.
	ErrQty = errors.New("costing: quantity must be positive")
	// ErrValue is a negative value.
	ErrValue = errors.New("costing: value must not be negative")
	// ErrInsufficient is a take of more than the position holds.
	ErrInsufficient = errors.New("costing: not enough stock")
	// ErrConsumed is a receipt that cannot be undone because goods from it already went out.
	ErrConsumed = errors.New("costing: goods from this receipt already went out")
	// ErrTooSmall is a quantity that converts to zero in the base unit.
	ErrTooSmall = errors.New("costing: quantity is too small for the base unit")
	// ErrInconsistent is a FIFO position whose layers do not hold its quantity and value.
	ErrInconsistent = errors.New("costing: layers do not match the position")
)

// ParseMethod reads "average" or "fifo".
func ParseMethod(s string) (Method, error) {
	switch m := Method(s); m {
	case Average, FIFO:
		return m, nil
	}
	return "", fmt.Errorf("%w: %q", ErrMethod, s)
}

// Layer is what is left of one FIFO receipt. ID is the caller's (its row); a layer Receive
// creates has ID 0 until the caller stores it.
type Layer struct {
	ID             int64
	Remaining      money.Qty
	RemainingValue money.Amount
}

// Position is the stock of one item in one place: its quantity and value, and under FIFO the
// layers that hold them, oldest first. Layers with nothing left may be omitted.
type Position struct {
	Qty    money.Qty
	Value  money.Amount
	Layers []Layer
}

// Take is the part of one FIFO layer a take carried out — kept by the caller so that undoing the
// take (Unissue) puts the goods back where they came from.
type Take struct {
	LayerID int64
	Qty     money.Qty
	Value   money.Amount
}

func positive(q money.Qty) error {
	if q.Sign() <= 0 {
		return ErrQty
	}
	return nil
}

// Receive adds qty at value (the whole receipt's value, not a unit cost). Under FIFO the new
// layer is appended with ID 0 and also returned, for the caller to store.
func Receive(p Position, m Method, qty money.Qty, value money.Amount) (Position, Layer, error) {
	if _, err := ParseMethod(string(m)); err != nil {
		return p, Layer{}, err
	}
	if err := positive(qty); err != nil {
		return p, Layer{}, err
	}
	if value.Sign() < 0 {
		return p, Layer{}, ErrValue
	}
	var err error
	if p.Qty, err = p.Qty.Add(qty); err != nil {
		return p, Layer{}, err
	}
	if p.Value, err = p.Value.Add(value); err != nil {
		return p, Layer{}, err
	}
	l := Layer{Remaining: qty, RemainingValue: value}
	if m == FIFO {
		p.Layers = append(append([]Layer(nil), p.Layers...), l)
	}
	return p, l, nil
}

// part is the value qty carries out of a holding of have worth value: all of it when qty is all
// of have, else its share — the part and what stays add up to value exactly.
func part(value money.Amount, have, qty money.Qty) (money.Amount, error) {
	if qty.Equal(have) {
		return value, nil
	}
	rest, err := have.Sub(qty)
	if err != nil {
		return money.Amount{}, err
	}
	parts, err := money.Allocate(value, []money.Qty{qty, rest})
	if err != nil {
		return money.Amount{}, err
	}
	return parts[0], nil
}

// Issue takes qty out and returns the position after it and the value that went out; under FIFO
// also the layers it took from, oldest first. More than the position holds is ErrInsufficient —
// a position never goes negative.
func Issue(p Position, m Method, qty money.Qty) (Position, money.Amount, []Take, error) {
	if _, err := ParseMethod(string(m)); err != nil {
		return p, money.Amount{}, nil, err
	}
	if err := positive(qty); err != nil {
		return p, money.Amount{}, nil, err
	}
	if qty.Cmp(p.Qty) > 0 {
		return p, money.Amount{}, nil, ErrInsufficient
	}
	var (
		value money.Amount
		takes []Take
		err   error
	)
	if m == Average {
		if value, err = part(p.Value, p.Qty, qty); err != nil {
			return p, money.Amount{}, nil, err
		}
	} else {
		layers := append([]Layer(nil), p.Layers...)
		need := qty
		for i := range layers {
			if need.IsZero() {
				break
			}
			l := &layers[i]
			if l.Remaining.Sign() <= 0 {
				continue
			}
			q := need
			if q.Cmp(l.Remaining) > 0 {
				q = l.Remaining
			}
			v, err := part(l.RemainingValue, l.Remaining, q)
			if err != nil {
				return p, money.Amount{}, nil, err
			}
			if l.Remaining, err = l.Remaining.Sub(q); err != nil {
				return p, money.Amount{}, nil, err
			}
			if l.RemainingValue, err = l.RemainingValue.Sub(v); err != nil {
				return p, money.Amount{}, nil, err
			}
			if need, err = need.Sub(q); err != nil {
				return p, money.Amount{}, nil, err
			}
			if value, err = value.Add(v); err != nil {
				return p, money.Amount{}, nil, err
			}
			takes = append(takes, Take{LayerID: l.ID, Qty: q, Value: v})
		}
		if !need.IsZero() {
			return p, money.Amount{}, nil, ErrInconsistent
		}
		p.Layers = layers
	}
	if p.Qty, err = p.Qty.Sub(qty); err != nil {
		return p, money.Amount{}, nil, err
	}
	if p.Value, err = p.Value.Sub(value); err != nil {
		return p, money.Amount{}, nil, err
	}
	if p.Qty.IsZero() && !p.Value.IsZero() {
		return p, money.Amount{}, nil, ErrInconsistent
	}
	return p, value, takes, nil
}

// Unreceive undoes a receipt of qty at value — cancelling it. Under FIFO its layer (layerID)
// must still hold all of it, else ErrConsumed; under Average the position must still hold qty,
// else ErrInsufficient.
func Unreceive(p Position, m Method, layerID int64, qty money.Qty, value money.Amount) (Position, error) {
	if _, err := ParseMethod(string(m)); err != nil {
		return p, err
	}
	if err := positive(qty); err != nil {
		return p, err
	}
	if m == FIFO {
		layers := make([]Layer, 0, len(p.Layers))
		found := false
		for _, l := range p.Layers {
			if l.ID == layerID {
				if !l.Remaining.Equal(qty) || !l.RemainingValue.Equal(value) {
					return p, ErrConsumed
				}
				found = true
				continue
			}
			layers = append(layers, l)
		}
		if !found {
			return p, ErrConsumed
		}
		p.Layers = layers
	}
	if qty.Cmp(p.Qty) > 0 || value.Cmp(p.Value) > 0 {
		return p, ErrInsufficient
	}
	var err error
	if p.Qty, err = p.Qty.Sub(qty); err != nil {
		return p, err
	}
	if p.Value, err = p.Value.Sub(value); err != nil {
		return p, err
	}
	if p.Qty.IsZero() && !p.Value.IsZero() {
		return p, ErrInconsistent
	}
	return p, nil
}

// Unissue undoes a take of qty worth value — cancelling it: the goods come back at the value they
// left with, and under FIFO into the layers they came from (takes, as Issue returned them).
func Unissue(p Position, m Method, qty money.Qty, value money.Amount, takes []Take) (Position, error) {
	if _, err := ParseMethod(string(m)); err != nil {
		return p, err
	}
	if err := positive(qty); err != nil {
		return p, err
	}
	if value.Sign() < 0 {
		return p, ErrValue
	}
	var err error
	if m == FIFO {
		layers := append([]Layer(nil), p.Layers...)
		var sumQ money.Qty
		var sumV money.Amount
		for _, t := range takes {
			i := -1
			for j := range layers {
				if layers[j].ID == t.LayerID {
					i = j
					break
				}
			}
			if i < 0 {
				return p, ErrInconsistent
			}
			if layers[i].Remaining, err = layers[i].Remaining.Add(t.Qty); err != nil {
				return p, err
			}
			if layers[i].RemainingValue, err = layers[i].RemainingValue.Add(t.Value); err != nil {
				return p, err
			}
			if sumQ, err = sumQ.Add(t.Qty); err != nil {
				return p, err
			}
			if sumV, err = sumV.Add(t.Value); err != nil {
				return p, err
			}
		}
		if !sumQ.Equal(qty) || !sumV.Equal(value) {
			return p, ErrInconsistent
		}
		p.Layers = layers
	}
	if p.Qty, err = p.Qty.Add(qty); err != nil {
		return p, err
	}
	if p.Value, err = p.Value.Add(value); err != nil {
		return p, err
	}
	return p, nil
}

// UnitCost is the position's average cost of one base unit, to the sen — for showing, and as the
// default cost of goods found in an adjustment. false when the position holds nothing.
func UnitCost(p Position) (money.Amount, bool) {
	if p.Qty.Sign() <= 0 {
		return money.Amount{}, false
	}
	c, err := p.Value.QuoQty(p.Qty, money.HalfUp)
	if err != nil {
		return money.Amount{}, false
	}
	return c, true
}

// Convert is qty of a unit worth factor base units, in base units (1 DUS × 12 = 12 PCS), rounded
// half-up to the quantity's 4 decimals. A non-zero quantity that rounds to zero is ErrTooSmall.
func Convert(qty money.Qty, factor money.Rate) (money.Qty, error) {
	if factor.Cmp(money.Rate{}) <= 0 {
		return money.Qty{}, fmt.Errorf("costing: unit factor must be positive")
	}
	b, err := qty.MulRate(factor, money.HalfUp)
	if err != nil {
		return money.Qty{}, err
	}
	if b.IsZero() && !qty.IsZero() {
		return money.Qty{}, ErrTooSmall
	}
	return b, nil
}
