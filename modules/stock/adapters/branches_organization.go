package modules

import (
	"context"

	organizationcontracts "__GO_MODULE__/internal/modules/organization/contracts"
	stockports "__GO_MODULE__/internal/modules/stock/ports"
)

// stockBranches satisfies stock's Branches port with the organization module's client. Mapping
// only.
type stockBranches struct {
	organization organizationcontracts.OrganizationClient
}

func (a stockBranches) Branches(ctx context.Context, companyID int64) ([]stockports.Branch, error) {
	bs, err := a.organization.Branches(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]stockports.Branch, len(bs))
	for i, b := range bs {
		out[i] = stockports.Branch{ID: b.ID, Code: b.Code, Name: b.Name, IsActive: b.IsActive}
	}
	return out, nil
}
