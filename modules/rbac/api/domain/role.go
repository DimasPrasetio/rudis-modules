// Package domain holds roles and the rules for granting permissions.
package domain

import (
	"strings"
	"time"

	"__GO_MODULE__/internal/shared/validator"
)

const (
	// AdminRole is every company's system role: it holds Wildcard and can be neither changed nor
	// deleted — only its members can. Company 1's is created by the migration; another
	// company's when the company is (AppointFounder).
	AdminRole = "Administrator"
	// AdminDescription is the Administrator role's description, as migration 002 words it.
	AdminDescription = "Semua izin, termasuk yang ditambahkan kemudian"
	// Wildcard grants every permission, including ones modules declare later.
	Wildcard = "*"
	// PermManage is the permission to manage roles and who holds them.
	PermManage = "rbac.manage"

	MaxNameLength        = 80
	MaxDescriptionLength = 255
)

// Role is a named set of permissions given to users. It belongs to one company: its members
// hold it there, across the company or in one of its branches.
type Role struct {
	ID          int64
	CompanyID   int64
	Name        string
	Description string
	IsSystem    bool
	Permissions []string
	MemberCount int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateRole checks a role's name and description.
func ValidateRole(v *validator.Validator, name, description string) {
	v.Required("name", name)
	v.MaxLen("name", name, MaxNameLength)
	v.MaxLen("description", description, MaxDescriptionLength)
	v.Check(!strings.EqualFold(strings.TrimSpace(name), AdminRole), "name", "sudah dipakai sistem")
}

// Grants reports whether a set of granted permissions covers permission.
func Grants(granted map[string]bool, permission string) bool {
	return granted[Wildcard] || granted[permission]
}
