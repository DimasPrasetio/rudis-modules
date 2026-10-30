package modules

import (
	"context"

	"__GO_MODULE__/internal/kernel"
	numberingcontracts "__GO_MODULE__/internal/modules/numbering/contracts"
	stockports "__GO_MODULE__/internal/modules/stock/ports"
)

// stockNumbers satisfies stock's Numbers port with numbering's client. Mapping only.
type stockNumbers struct {
	numbering numberingcontracts.NumberingClient
}

func (a stockNumbers) Declare(docType, label string, def stockports.NumberFormat) {
	a.numbering.Declare(docType, label, numberingcontracts.Format{Pattern: def.Pattern, Reset: def.Reset, PerBranch: def.PerBranch})
}

func (a stockNumbers) Next(ctx context.Context, docType string, date kernel.Date) (string, error) {
	return a.numbering.Next(ctx, docType, date)
}
