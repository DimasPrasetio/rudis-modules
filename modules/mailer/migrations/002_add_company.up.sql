-- Each message belongs to the company it was sent for (kernel 6 scope): the outbox shows a
-- company only its own recipients and subjects. Company 0 is installation-level — email sent
-- before any company is chosen, in an app with several companies (a password reset by token).
-- Existing rows: unknown, so installation-level; they are seen on the installation-level outbox.
ALTER TABLE mail_outbox
  ADD COLUMN company_id BIGINT NOT NULL DEFAULT 0,
  ADD KEY mail_outbox_company (company_id, id);
