package modules

import (
	"context"

	mailerports "__GO_MODULE__/internal/modules/mailer/ports"
	settingscontracts "__GO_MODULE__/internal/modules/settings/contracts"
)

var _ mailerports.SenderIdentity = mailerSenderIdentity{}

// mailerSenderIdentity satisfies the mailer's SenderIdentity port with the settings module's
// client: the company's name, and its email as the reply address. Mapping only.
type mailerSenderIdentity struct {
	settings settingscontracts.SettingsClient
}

func (a mailerSenderIdentity) Sender(ctx context.Context) (mailerports.Identity, error) {
	c, err := a.settings.Company(ctx)
	if err != nil {
		return mailerports.Identity{}, err
	}
	return mailerports.Identity{Name: c.Name, ReplyTo: c.Email}, nil
}
