// Package contracts is what the rbac module offers. Another module never imports it: it
// declares its own port (an approvals module: "who may approve this?"), and an adapter in the
// composition root connects the two.
package contracts

import (
	"context"
	"errors"
)

// ErrRoleNotFound means the company ctx works in has no role of that name.
var ErrRoleNotFound = errors.New("rbac: role not found")

// Holder is a user holding a role in the company ctx works in: across it (BranchID 0) or in one
// branch.
type Holder struct {
	UserID   int64
	BranchID int64
}

// RoleHolders answers who holds which role in the company ctx works in (kernel.ScopeFrom).
// Roles belong to one company: the same name in another company is another role.
type RoleHolders interface {
	// Holders lists who holds the role named name in the company of ctx's scope, in every
	// branch. Deactivated accounts are not filtered here — the caller asks its user directory.
	// ErrRoleNotFound when the company has no such role.
	Holders(ctx context.Context, name string) ([]Holder, error)
	// RolesOf lists the names of the roles userID holds where ctx works — branch-aware like
	// permissions: in a branch, the company-wide roles and that branch's; at the whole
	// company, company-wide roles only.
	RolesOf(ctx context.Context, userID int64) ([]string, error)
	// Roles lists the company's role names (for a picker), the Administrator role first.
	Roles(ctx context.Context) ([]string, error)
}
