-- Filters the trail is read by: kind (security, access) and person, within a company — and the
-- installation-level trail, company 0 (from this version, records made before any company was
-- chosen are kept there instead of in company 1). Rows written before this version under
-- company 1 cannot be told apart from company 1's own and stay where they are.
ALTER TABLE audit_log
  ADD KEY audit_log_company_kind (company_id, kind, occurred_at),
  ADD KEY audit_log_company_actor (company_id, actor_id, occurred_at);
