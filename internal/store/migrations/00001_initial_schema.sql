-- +goose Up

-- The table and column definitions below match those created by the Django
-- release of this application, so an existing mangle.db keeps working after
-- the upgrade. UUIDs are stored as 32 unseparated hex characters, timestamps
-- as naive UTC text, and booleans as integers.

CREATE TABLE IF NOT EXISTS "groups" (
    "id"           char(32)     NOT NULL PRIMARY KEY,
    "created_at"   datetime     NOT NULL,
    "updated_at"   datetime     NOT NULL,
    "description"  text         NOT NULL,
    "is_enabled"   bool         NOT NULL,
    "max_devices"  integer      NOT NULL,
    "name"         varchar(32)  NOT NULL UNIQUE,
    "mfa_enforced" bool         NOT NULL
);

CREATE TABLE IF NOT EXISTS "users" (
    "password"        varchar(128) NOT NULL,
    "last_login"      datetime     NULL,
    "id"              char(32)     NOT NULL PRIMARY KEY,
    "created_at"      datetime     NOT NULL,
    "updated_at"      datetime     NOT NULL,
    "email"           varchar(255) NOT NULL UNIQUE,
    "is_admin"        bool         NOT NULL,
    "is_enabled"      bool         NOT NULL,
    "name"            varchar(255) NOT NULL,
    "mfa_enabled"     bool         NOT NULL,
    "mfa_secret"      varchar(255) NOT NULL,
    "group_id"        char(32)     NOT NULL REFERENCES "groups" ("id") DEFERRABLE INITIALLY DEFERRED,
    "mfa_enforced"    bool         NULL,
    "password_change" bool         NOT NULL,
    "password_expire" datetime     NULL
);

CREATE TABLE IF NOT EXISTS "devices" (
    "id"          char(32)     NOT NULL PRIMARY KEY,
    "created_at"  datetime     NOT NULL,
    "updated_at"  datetime     NOT NULL,
    "fingerprint" varchar(255) NOT NULL,
    "last_login"  datetime     NULL,
    "name"        varchar(32)  NOT NULL,
    "serial"      varchar(255) NOT NULL,
    "user_id"     char(32)     NOT NULL REFERENCES "users" ("id") DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE IF NOT EXISTS "clients" (
    "id"          char(32)     NOT NULL PRIMARY KEY,
    "created_at"  datetime     NOT NULL,
    "updated_at"  datetime     NOT NULL,
    "common_name" varchar(255) NOT NULL UNIQUE,
    "platform"    varchar(32)  NOT NULL,
    "remote_ip"   varchar(32)  NOT NULL,
    "virtual_ip"  varchar(32)  NOT NULL UNIQUE,
    "device_id"   char(32)     NOT NULL UNIQUE REFERENCES "devices" ("id") DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE IF NOT EXISTS "firewall_rules" (
    "id"          char(32)     NOT NULL PRIMARY KEY,
    "created_at"  datetime     NOT NULL,
    "updated_at"  datetime     NOT NULL,
    "action"      varchar(255) NOT NULL,
    "destination" varchar(255) NOT NULL,
    "is_enabled"  bool         NOT NULL,
    "port"        varchar(255) NOT NULL,
    "protocol"    varchar(255) NOT NULL,
    "group_id"    char(32)     NOT NULL REFERENCES "groups" ("id") DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE IF NOT EXISTS "events" (
    "id"         char(32)     NOT NULL PRIMARY KEY,
    "created_at" datetime     NOT NULL,
    "updated_at" datetime     NOT NULL,
    "detail"     text         NOT NULL,
    "name"       varchar(255) NOT NULL,
    "user_id"    char(32)     NOT NULL REFERENCES "users" ("id") DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE IF NOT EXISTS "revoked_devices" (
    "id"         char(32)     NOT NULL PRIMARY KEY,
    "created_at" datetime     NOT NULL,
    "updated_at" datetime     NOT NULL,
    "serial"     varchar(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS "setting" (
    "id"         char(32)     NOT NULL PRIMARY KEY,
    "created_at" datetime     NOT NULL,
    "updated_at" datetime     NOT NULL,
    "name"       varchar(255) NOT NULL UNIQUE,
    "value"      text         NOT NULL
);

-- The job queue replaces the Redis backed task queue of the Django release.
-- Keeping it in SQLite means the web, task, and VPN hook processes share one
-- durable queue without a second daemon to install.
CREATE TABLE IF NOT EXISTS "jobs" (
    "id"         char(32)     NOT NULL PRIMARY KEY,
    "created_at" datetime     NOT NULL,
    "run_at"     datetime     NOT NULL,
    "kind"       varchar(64)  NOT NULL,
    "payload"    text         NOT NULL,
    "attempts"   integer      NOT NULL DEFAULT 0,
    "claimed_at" datetime     NULL
);

-- Web sessions. The Django release kept these in django_session; the layout
-- here is the one the session manager expects.
CREATE TABLE IF NOT EXISTS "sessions" (
    "token"   text     NOT NULL PRIMARY KEY,
    "data"    blob     NOT NULL,
    "expires" datetime NOT NULL
);

CREATE INDEX IF NOT EXISTS "idx_sessions_expires" ON "sessions" ("expires");

CREATE INDEX IF NOT EXISTS "idx_users_email"        ON "users" ("email");
CREATE INDEX IF NOT EXISTS "idx_users_group_id"     ON "users" ("group_id");
CREATE INDEX IF NOT EXISTS "idx_devices_user_id"    ON "devices" ("user_id");
CREATE INDEX IF NOT EXISTS "idx_devices_finger"     ON "devices" ("fingerprint");
CREATE INDEX IF NOT EXISTS "idx_clients_common"     ON "clients" ("common_name");
CREATE INDEX IF NOT EXISTS "idx_clients_virtual"    ON "clients" ("virtual_ip");
CREATE INDEX IF NOT EXISTS "idx_clients_device_id"  ON "clients" ("device_id");
CREATE INDEX IF NOT EXISTS "idx_firewall_group_id"  ON "firewall_rules" ("group_id");
CREATE INDEX IF NOT EXISTS "idx_events_user_id"     ON "events" ("user_id");
CREATE INDEX IF NOT EXISTS "idx_events_created_at"  ON "events" ("created_at");
CREATE INDEX IF NOT EXISTS "idx_groups_name"        ON "groups" ("name");
CREATE INDEX IF NOT EXISTS "idx_setting_name"       ON "setting" ("name");
CREATE INDEX IF NOT EXISTS "idx_jobs_run_at"        ON "jobs" ("run_at");

-- +goose Down

-- Deliberately empty.
--
-- This migration is idempotent and may be applied to a database that
-- already held these tables, because the Django release created them
-- before goose existed here. Rolling it back would therefore destroy data
-- this migration never created. Drop the file if you genuinely want a
-- clean database.

