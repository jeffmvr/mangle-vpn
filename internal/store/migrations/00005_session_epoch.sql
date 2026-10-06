-- +goose Up

-- A counter that every one of a user's sessions records when it starts.
-- Raising it ends all the sessions that recorded an earlier value: it is how
-- changing a password signs the user out everywhere else, and how an
-- administrator's password or two-factor reset takes effect at once. The
-- default lets the Django release insert users without knowing the column.

ALTER TABLE "users" ADD COLUMN "session_epoch" integer NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE "users" DROP COLUMN "session_epoch";
