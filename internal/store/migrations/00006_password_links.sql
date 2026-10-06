-- +goose Up

-- Single-use links for choosing a password: sent with an invitation, by an
-- administrator's reset, or asked for from the sign in page. Only a SHA-256
-- hash of each link's code is kept, so a copy of the database gives none of
-- them away. A link lapses at expires_at and works once, which used_at
-- records.
--
-- The Django release neither reads nor writes this table.

CREATE TABLE IF NOT EXISTS "password_links" (
    "id"         char(32)    NOT NULL PRIMARY KEY,
    "created_at" datetime    NOT NULL,
    "user_id"    char(32)    NOT NULL REFERENCES "users" ("id") DEFERRABLE INITIALLY DEFERRED,
    "code_hash"  varchar(64) NOT NULL UNIQUE,
    "purpose"    varchar(16) NOT NULL,
    "expires_at" datetime    NOT NULL,
    "used_at"    datetime    NULL
);

CREATE INDEX IF NOT EXISTS "idx_password_links_user_id" ON "password_links" ("user_id");

-- +goose Down

DROP TABLE "password_links";
