-- +goose Up

-- A single-use code that lets OpenVPN Connect fetch a new device's profile
-- itself, through an openvpn://import-profile/ link, without the user's
-- session. Only a SHA-256 hash of the code is kept, and it lapses at
-- import_expires. The defaults let the Django release insert devices
-- without knowing the columns exist.

ALTER TABLE "devices" ADD COLUMN "import_token" varchar(64) NOT NULL DEFAULT '';
ALTER TABLE "devices" ADD COLUMN "import_expires" datetime NULL;

-- +goose Down

ALTER TABLE "devices" DROP COLUMN "import_expires";
ALTER TABLE "devices" DROP COLUMN "import_token";
