package modules

import (
	"context"

	auditports "__GO_MODULE__/internal/modules/audit-log/ports"
	organizationcontracts "__GO_MODULE__/internal/modules/organization/contracts"
)

var _ auditports.ActorCompanies = auditActorCompanies{}

// auditActorCompanies satisfies audit-log's ActorCompanies port with the organization module's
// client. Mapping only: the company of each placement (the port's caller removes repeats).
type auditActorCompanies struct {
	organization organizationcontracts.OrganizationClient
}

func (a auditActorCompanies) CompaniesOf(ctx context.Context, userID int64) ([]int64, error) {
	ws, err := a.organization.PlacesOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(ws))
	for i, w := range ws {
		out[i] = w.CompanyID
	}
	return out, nil
}
