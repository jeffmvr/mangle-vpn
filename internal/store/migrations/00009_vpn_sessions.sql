-- +goose Up

-- The history of VPN connections, one row written as each one ends: which
-- device, from where, for how long, and how much data it moved. Byte counts
-- are as OpenVPN reports them, from the server's side: received from the
-- device, and sent to it. Rows go with their device or user, and age out
-- with the audit log.
--
-- The Django release neither reads nor writes this table.

CREATE TABLE IF NOT EXISTS "vpn_sessions" (
    "id"             char(32)    NOT NULL PRIMARY KEY,
    "device_id"      char(32)    NOT NULL REFERENCES "devices" ("id") DEFERRABLE INITIALLY DEFERRED,
    "user_id"        char(32)    NOT NULL REFERENCES "users" ("id") DEFERRABLE INITIALLY DEFERRED,
    "started_at"     datetime    NOT NULL,
    "ended_at"       datetime    NOT NULL,
    "remote_ip"      varchar(64) NOT NULL,
    "virtual_ip"     varchar(32) NOT NULL,
    "bytes_received" integer     NOT NULL DEFAULT 0,
    "bytes_sent"     integer     NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS "idx_vpn_sessions_user_id"   ON "vpn_sessions" ("user_id", "ended_at");
CREATE INDEX IF NOT EXISTS "idx_vpn_sessions_device_id" ON "vpn_sessions" ("device_id");
CREATE INDEX IF NOT EXISTS "idx_vpn_sessions_ended_at"  ON "vpn_sessions" ("ended_at");

-- +goose Down

DROP TABLE "vpn_sessions";
