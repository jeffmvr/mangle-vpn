-- +goose Up

-- State that sign in has to share between the web application and the
-- OpenVPN hooks, which run as separate processes:
--
--   mfa_last_step  the time step of the last two-factor code accepted, so a
--                  code cannot be used twice, on the web or on the VPN;
--   failed_logins  consecutive failed passwords and codes since the last
--                  successful sign in;
--   locked_until   when a lockout after too many failures ends.
--
-- Each has a default, so the Django release can still insert users without
-- knowing the columns exist.

ALTER TABLE "users" ADD COLUMN "mfa_last_step" integer NOT NULL DEFAULT 0;
ALTER TABLE "users" ADD COLUMN "failed_logins" integer NOT NULL DEFAULT 0;
ALTER TABLE "users" ADD COLUMN "locked_until" datetime NULL;

-- +goose Down

ALTER TABLE "users" DROP COLUMN "locked_until";
ALTER TABLE "users" DROP COLUMN "failed_logins";
ALTER TABLE "users" DROP COLUMN "mfa_last_step";
