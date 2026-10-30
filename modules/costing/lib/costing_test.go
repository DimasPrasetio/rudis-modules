package costing

import (
	"errors"
	"testing"

	"__GO_MODULE__/internal/shared/money"
)

func q(s string) money.Qty    { return money.MustQty(s) }
func a(s string) money.Amount { return money.MustAmount(s) }

func receive(t *testing.T, p Position, m Method, qty, value string, id int64) Position {
	t.Helper()
	p, l, err := Receive(p, m, q(qty), a(value))
	if err != nil {
		t.Fatalf("Receive %s @ %s: %v", qty, value, err)
	}
	if m == FIFO {
		p.Layers[len(p.Layers)-1].ID = id
		if !l.Remaining.Equal(q(qty)) || !l.RemainingValue.Equal(a(value)) {
			t.Fatalf("new layer = %+v", l)
		}
	}
	return p
}

func wantPos(t *testing.T, p Position, qty, value string) {
	t.Helper()
	if !p.Qty.Equal(q(qty)) || !p.Value.Equal(a(value)) {
		t.Fatalf("position = %s / %s, want %s / %s", p.Qty, p.Value, qty, value)
	}
}

func TestAverage(t *testing.T) {
	var p Position
	p = receive(t, p, Average, "10", "1000.00", 0) // 100 each
	p = receive(t, p, Average, "10", "2000.00", 0) // 200 each → average 150
	if c, ok := UnitCost(p); !ok || !c.Equal(a("150")) {
		t.Fatalf("unit cost = %s %v", c, ok)
	}
	p, v, takes, err := Issue(p, Average, q("4"))
	if err != nil || !v.Equal(a("600")) || takes != nil {
		t.Fatalf("issue 4 = %s %v %v", v, takes, err)
	}
	wantPos(t, p, "16", "2400")
	if len(p.Layers) != 0 {
		t.Fatalf("average keeps no layers: %+v", p.Layers)
	}
}

func TestAverageLeavesNoSen(t *testing.T) {
	var p Position
	p = receive(t, p, Average, "3", "100.00", 0) // 33.333… each
	var out money.Amount
	for i := 0; i < 3; i++ {
		var v money.Amount
		var err error
		p, v, _, err = Issue(p, Average, q("1"))
		if err != nil {
			t.Fatal(err)
		}
		out, _ = out.Add(v)
	}
	wantPos(t, p, "0", "0")
	if !out.Equal(a("100")) {
		t.Fatalf("three takes carried %s, want all 100.00", out)
	}
}

func TestFIFOAcrossLayers(t *testing.T) {
	var p Position
	p = receive(t, p, FIFO, "10", "1000.00", 1)
	p = receive(t, p, FIFO, "10", "2000.00", 2)
	p, v, takes, err := Issue(p, FIFO, q("15"))
	if err != nil || !v.Equal(a("2000")) { // 10 × 100 + 5 × 200
		t.Fatalf("issue 15 = %s %v", v, err)
	}
	if len(takes) != 2 || takes[0].LayerID != 1 || !takes[0].Qty.Equal(q("10")) || !takes[0].Value.Equal(a("1000")) ||
		takes[1].LayerID != 2 || !takes[1].Qty.Equal(q("5")) || !takes[1].Value.Equal(a("1000")) {
		t.Fatalf("takes = %+v", takes)
	}
	wantPos(t, p, "5", "1000")
	if !p.Layers[0].Remaining.IsZero() || !p.Layers[1].Remaining.Equal(q("5")) || !p.Layers[1].RemainingValue.Equal(a("1000")) {
		t.Fatalf("layers = %+v", p.Layers)
	}
	p, err = Unissue(p, FIFO, q("15"), v, takes)
	if err != nil {
		t.Fatal(err)
	}
	wantPos(t, p, "20", "3000")
	if !p.Layers[0].Remaining.Equal(q("10")) || !p.Layers[0].RemainingValue.Equal(a("1000")) {
		t.Fatalf("restored layers = %+v", p.Layers)
	}
}

func TestFIFOPartialLayerLeavesNoSen(t *testing.T) {
	var p Position
	p = receive(t, p, FIFO, "3", "10.00", 7)
	p, v1, _, _ := Issue(p, FIFO, q("1"))
	p, v2, _, _ := Issue(p, FIFO, q("1"))
	p, v3, _, err := Issue(p, FIFO, q("1"))
	if err != nil {
		t.Fatal(err)
	}
	sum, _ := money.Sum(v1, v2, v3)
	if !sum.Equal(a("10")) {
		t.Fatalf("takes %s + %s + %s = %s, want 10.00", v1, v2, v3, sum)
	}
	wantPos(t, p, "0", "0")
}

func TestRefusals(t *testing.T) {
	var p Position
	p = receive(t, p, Average, "2", "20", 0)
	if _, _, _, err := Issue(p, Average, q("2.0001")); !errors.Is(err, ErrInsufficient) {
		t.Fatalf("over-issue: %v", err)
	}
	if _, _, _, err := Issue(p, Average, q("0")); !errors.Is(err, ErrQty) {
		t.Fatalf("zero issue: %v", err)
	}
	if _, _, err := Receive(p, Average, q("-1"), a("1")); !errors.Is(err, ErrQty) {
		t.Fatalf("negative receipt: %v", err)
	}
	if _, _, err := Receive(p, Average, q("1"), a("-1")); !errors.Is(err, ErrValue) {
		t.Fatalf("negative value: %v", err)
	}
	if _, _, err := Receive(p, "lifo", q("1"), a("1")); !errors.Is(err, ErrMethod) {
		t.Fatalf("LIFO: %v", err)
	}
	if _, err := ParseMethod("lifo"); !errors.Is(err, ErrMethod) {
		t.Fatalf("ParseMethod lifo: %v", err)
	}
	if m, err := ParseMethod("fifo"); err != nil || m != FIFO {
		t.Fatalf("ParseMethod fifo: %v %v", m, err)
	}
	// Layers that do not hold the position: refused rather than valued from nothing.
	broken := Position{Qty: q("5"), Value: a("50"), Layers: []Layer{{ID: 1, Remaining: q("2"), RemainingValue: a("20")}}}
	if _, _, _, err := Issue(broken, FIFO, q("3")); !errors.Is(err, ErrInconsistent) {
		t.Fatalf("broken layers: %v", err)
	}
	if _, ok := UnitCost(Position{}); ok {
		t.Fatal("unit cost of nothing")
	}
}

func TestUnreceive(t *testing.T) {
	var p Position
	p = receive(t, p, FIFO, "10", "1000", 1)
	p = receive(t, p, FIFO, "5", "750", 2)
	back, err := Unreceive(p, FIFO, 2, q("5"), a("750"))
	if err != nil || len(back.Layers) != 1 {
		t.Fatalf("unreceive intact layer: %+v %v", back, err)
	}
	wantPos(t, back, "10", "1000")
	p, _, _, _ = Issue(p, FIFO, q("12")) // eats into layer 2
	if _, err := Unreceive(p, FIFO, 2, q("5"), a("750")); !errors.Is(err, ErrConsumed) {
		t.Fatalf("unreceive consumed layer: %v", err)
	}
	var avg Position
	avg = receive(t, avg, Average, "10", "1000", 0)
	avg = receive(t, avg, Average, "10", "2000", 0)
	avg, err = Unreceive(avg, Average, 0, q("10"), a("2000"))
	if err != nil {
		t.Fatal(err)
	}
	wantPos(t, avg, "10", "1000")
	if _, err := Unreceive(avg, Average, 0, q("11"), a("1000")); !errors.Is(err, ErrInsufficient) {
		t.Fatalf("unreceive more than held: %v", err)
	}
}

func TestUnissueAverage(t *testing.T) {
	var p Position
	p = receive(t, p, Average, "3", "100", 0)
	after, v, _, _ := Issue(p, Average, q("1"))
	back, err := Unissue(after, Average, q("1"), v, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantPos(t, back, "3", "100")
}

func TestConvert(t *testing.T) {
	b, err := Convert(q("2"), money.MustRate("12"))
	if err != nil || !b.Equal(q("24")) {
		t.Fatalf("2 DUS = %s %v", b, err)
	}
	b, err = Convert(q("1.5"), money.MustRate("0.001")) // 1.5 g in kg
	if err != nil || !b.Equal(q("0.0015")) {
		t.Fatalf("1.5 g = %s kg %v", b, err)
	}
	if _, err := Convert(q("0.0001"), money.MustRate("0.001")); !errors.Is(err, ErrTooSmall) {
		t.Fatalf("too small: %v", err)
	}
	if _, err := Convert(q("1"), money.Rate{}); err == nil {
		t.Fatal("zero factor accepted")
	}
}
