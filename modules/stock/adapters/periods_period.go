package modules

import (
	"context"
	"errors"

	"__GO_MODULE__/internal/kernel"
	periodcontracts "__GO_MODULE__/internal/modules/period/contracts"
	stockports "__GO_MODULE__/internal/modules/stock/ports"
)

// stockPeriods satisfies stock's Periods port with period's client. Mapping only.
type stockPeriods struct{ period periodcontracts.PeriodClient }

func (a stockPeriods) Declare(area, label string) { a.period.Declare(area, label) }

func (a stockPeriods) EnsureOpen(ctx context.Context, area string, date kernel.Date) error {
	err := a.period.EnsureOpen(ctx, area, date)
	if errors.Is(err, periodcontracts.ErrClosed) {
		return &stockports.PeriodClosedError{Message: err.Error()}
	}
	return err
}
