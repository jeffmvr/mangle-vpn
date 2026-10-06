-- +goose Up

-- The operating system a device's profile was generated for, so that the
-- device list can say what each device is. It is the first column the Django
-- release does not have; the default keeps that release able to insert rows
-- without naming it, and rows created before this migration read as unknown.

ALTER TABLE "devices" ADD COLUMN "os" varchar(16) NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE "devices" DROP COLUMN "os";
