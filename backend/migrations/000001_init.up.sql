-- Baseline migration (issue #1).
--
-- There are no domain tables yet: Customer/Package/AccessPoint arrive in
-- #3-#5, the Session/Vlan attribution tables in #12. This migration exists to
-- prove the three things issue #1 asks for — the DSN comes from the
-- environment, the runner applies versioned changes to the dev database, and a
-- re-run is a no-op — and to give every later migration a version to sit after.
--
-- app_schema_baseline is a canary the migration runner reads back after `up`,
-- so a silently skipped migration is reported as a failure instead of a green
-- run against an unchanged schema.

CREATE TABLE IF NOT EXISTS app_schema_baseline (
    id          smallint    PRIMARY KEY,
    app_version text        NOT NULL,
    applied_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT app_schema_baseline_singleton CHECK (id = 1)
);

COMMENT ON TABLE app_schema_baseline IS
    'Single-row marker written by the baseline migration; read back by cmd/migration to confirm the schema actually changed.';

INSERT INTO app_schema_baseline (id, app_version)
VALUES (1, '0.1.0')
ON CONFLICT (id) DO UPDATE SET app_version = EXCLUDED.app_version;
