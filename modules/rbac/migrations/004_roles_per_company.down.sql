-- Back to one set of roles for the app: company 1's. Members of other companies are re-pointed
-- to company 1's role of the same name. Lost: a role that exists only in another company (and
-- its memberships), and another company's own description and permissions of a role that
-- shares its name with company 1's. (Names are unique per company, so no membership lands on a
-- row that already exists.)
UPDATE user_roles ur
JOIN roles n ON n.id = ur.role_id AND n.company_id <> 1
JOIN roles o ON o.company_id = 1 AND o.name = n.name
SET ur.role_id = o.id;

-- Cascades to role_permissions and to the memberships of roles company 1 does not have.
DELETE FROM roles WHERE company_id <> 1;

ALTER TABLE role_permissions
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (role_id, permission);
ALTER TABLE role_permissions DROP COLUMN company_id;

ALTER TABLE roles
  DROP KEY roles_company_name_unique,
  ADD UNIQUE KEY roles_name_unique (name);
ALTER TABLE roles DROP COLUMN company_id;
