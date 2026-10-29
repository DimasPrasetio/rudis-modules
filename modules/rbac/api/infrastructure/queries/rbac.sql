-- Roles belong to one company (roles.company_id); so do their permissions and their members.
-- Every query works in one company, except the one the kernel's authorizer needs across them.

-- name: ListRoles :many
SELECT r.id, r.company_id, r.name, r.description, r.is_system, r.created_at, r.updated_at,
       (SELECT COUNT(*) FROM user_roles ur WHERE ur.role_id = r.id AND ur.company_id = r.company_id) AS member_count
FROM roles r
WHERE r.company_id = ?
ORDER BY r.is_system DESC, r.name;

-- name: GetRole :one
SELECT r.id, r.company_id, r.name, r.description, r.is_system, r.created_at, r.updated_at,
       (SELECT COUNT(*) FROM user_roles ur WHERE ur.role_id = r.id AND ur.company_id = r.company_id) AS member_count
FROM roles r
WHERE r.company_id = ? AND r.id = ?;

-- name: GetRoleByName :one
SELECT id FROM roles WHERE company_id = ? AND name = ?;

-- name: ListRolePermissions :many
SELECT role_id, permission FROM role_permissions WHERE company_id = ? ORDER BY role_id, permission;

-- name: ListPermissionsOfRole :many
SELECT permission FROM role_permissions WHERE company_id = ? AND role_id = ? ORDER BY permission;

-- name: CreateRole :execlastid
INSERT INTO roles (company_id, name, description, is_system) VALUES (?, ?, ?, ?);

-- name: UpdateRole :exec
UPDATE roles SET name = ?, description = ? WHERE company_id = ? AND id = ?;

-- name: DeleteRole :execrows
DELETE FROM roles WHERE company_id = ? AND id = ?;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE company_id = ? AND role_id = ?;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (company_id, role_id, permission) VALUES (?, ?, ?);

-- name: ListMembers :many
SELECT user_id, branch_id FROM user_roles WHERE company_id = ? AND role_id = ? ORDER BY created_at, user_id, branch_id;

-- name: AddMember :exec
INSERT IGNORE INTO user_roles (company_id, user_id, role_id, branch_id) VALUES (?, ?, ?, ?);

-- name: RemoveMember :execrows
DELETE FROM user_roles WHERE company_id = ? AND user_id = ? AND role_id = ? AND branch_id = ?;

-- name: LockMembers :many
SELECT user_id, branch_id FROM user_roles WHERE company_id = ? AND role_id = ? ORDER BY user_id, branch_id FOR UPDATE;

-- name: RolesOfUser :many
SELECT r.id, r.name, r.is_system, ur.branch_id
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id AND r.company_id = ur.company_id
WHERE ur.company_id = ? AND ur.user_id = ?
ORDER BY r.name, ur.branch_id;

-- name: SystemGrantsOfUser :many
-- all companies: what the user holds everywhere — the kernel's authorizer answers for the
-- company a request works in, and the home company's Administrator for every company.
SELECT DISTINCT ur.company_id, ur.branch_id, rp.permission
FROM user_roles ur
JOIN role_permissions rp ON rp.role_id = ur.role_id AND rp.company_id = ur.company_id
WHERE ur.user_id = ?;

-- name: LockRoleForUpdate :one
SELECT id FROM roles WHERE company_id = ? AND id = ? FOR UPDATE;

-- name: LockRoleForShare :one
SELECT id FROM roles WHERE company_id = ? AND id = ? FOR SHARE;
