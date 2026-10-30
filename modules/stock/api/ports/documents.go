package ports

import (
	"context"

	"__GO_MODULE__/internal/kernel"
)

// NumberFormat is a document type's default number format.
type NumberFormat struct {
	Pattern   string
	Reset     string
	PerBranch bool
}

// Numbers gives posted documents their numbers (the numbering module). Required.
type Numbers interface {
	Declare(docType, label string, def NumberFormat)
	// Next must run inside d.Tx: a rollback gives the number back, so there are no gaps.
	Next(ctx context.Context, docType string, date kernel.Date) (string, error)
}

// PeriodClosedError is a date in a closed period; Message says until when.
type PeriodClosedError struct{ Message string }

func (e *PeriodClosedError) Error() string { return e.Message }

// Periods refuses dates in closed periods (the period module). Optional: without it every date
// is open.
type Periods interface {
	Declare(area, label string)
	// EnsureOpen returns a *PeriodClosedError for a date in a closed period of area.
	EnsureOpen(ctx context.Context, area string, date kernel.Date) error
}

// Branch is a branch of the company a warehouse may belong to.
type Branch struct {
	ID       int64
	Code     string
	Name     string
	IsActive bool
}

// Branches lists the company's branches (the organization module). Optional: without it every
// warehouse belongs to the company as a whole.
type Branches interface {
	Branches(ctx context.Context, companyID int64) ([]Branch, error)
}
