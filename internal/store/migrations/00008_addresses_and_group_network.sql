-- +goose Up

-- A device may be given a fixed VPN address, from the upper half of the
-- client subnet, which OpenVPN never hands out on its own. No two devices
-- may share one.
--
-- A group may push routes and DNS servers of its own to its members, on top
-- of the server-wide ones, and may remove its members' devices once they
-- have gone unused for device_idle_days (0 keeps them).
--
-- Every column has a default, so the Django release can still insert rows
-- without knowing they exist.

ALTER TABLE "devices" ADD COLUMN "static_ip" varchar(15) NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS "idx_devices_static_ip" ON "devices" ("static_ip") WHERE "static_ip" != '';

ALTER TABLE "groups" ADD COLUMN "routes" text NOT NULL DEFAULT '';
ALTER TABLE "groups" ADD COLUMN "nameservers" text NOT NULL DEFAULT '';
ALTER TABLE "groups" ADD COLUMN "device_idle_days" integer NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE "groups" DROP COLUMN "device_idle_days";
ALTER TABLE "groups" DROP COLUMN "nameservers";
ALTER TABLE "groups" DROP COLUMN "routes";
DROP INDEX IF EXISTS "idx_devices_static_ip";
ALTER TABLE "devices" DROP COLUMN "static_ip";
