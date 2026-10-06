-- +goose Up

-- A role short of administrator. Empty is an ordinary member; "helpdesk"
-- can look around the administration pages and help people back in
-- (unlocking accounts, resetting two-factor, sending password links,
-- disconnecting and revoking devices) without changing any settings.
-- Administrators are still marked by is_admin, which the Django release
-- reads; it ignores this column, and its default lets it insert users.

ALTER TABLE "users" ADD COLUMN "role" varchar(16) NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE "users" DROP COLUMN "role";
