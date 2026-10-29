-- A role belongs to one company. Every role so far was defined once for the app: it becomes
-- company 1's, and a company whose members hold one of them gets its own copy.
ALTER TABLE roles
  ADD COLUMN company_id BIGINT NOT NULL DEFAULT 1 AFTER id,
  DROP KEY roles_name_unique,
  ADD UNIQUE KEY roles_company_name_unique (company_id, name);

-- role_id keeps leading the primary key: the foreign key to roles still has its index.
ALTER TABLE role_permissions
  ADD COLUMN company_id BIGINT NOT NULL DEFAULT 1,
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (role_id, company_id, permission);

-- Every other company whose members hold a role (company 1's, until now) gets its own copy of
-- it — the Administrator held by its founder (the old AppointFounder gave company 1's) included.
INSERT INTO roles (company_id, name, description, is_system, created_at, updated_at)
SELECT DISTINCT ur.company_id, r.name, r.description, r.is_system, r.created_at, r.updated_at
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.company_id <> 1 AND r.company_id = 1;

-- …with the permissions of the original.
INSERT INTO role_permissions (role_id, company_id, permission)
SELECT n.id, n.company_id, rp.permission
FROM roles n
JOIN roles o ON o.company_id = 1 AND o.name = n.name
JOIN role_permissions rp ON rp.role_id = o.id AND rp.company_id = 1
WHERE n.company_id <> 1;

-- Members of other companies now hold their own company's copy.
UPDATE user_roles ur
JOIN roles o ON o.id = ur.role_id AND o.company_id = 1
JOIN roles n ON n.company_id = ur.company_id AND n.name = o.name
SET ur.role_id = n.id
WHERE ur.company_id <> 1;
