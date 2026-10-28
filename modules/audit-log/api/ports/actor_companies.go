package ports

import "context"

// ActorCompanies tells which companies a person belongs to (an organization module). Optional:
// with it a record made before any company was chosen (a sign-in, a password reset, in an app
// with several companies) is also filed in each company its actor is placed in, so that
// company's auditors see their own people's sign-ins. Without it such records stay
// installation-level only. Wired once every module is built (late).
type ActorCompanies interface {
	// CompaniesOf lists the company of each of the user's placements (a company may repeat).
	CompaniesOf(ctx context.Context, userID int64) ([]int64, error)
}
