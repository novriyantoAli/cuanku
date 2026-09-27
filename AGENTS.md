# AGENTS.md

## Development environment

Verified facts — do not rediscover them, and do not assume they are different:

- **Dev PostgreSQL lives in VirtualBox VM `deb12`** (NIC bridged to `enp3s0`): `172.16.0.72:5432`. There is no local PostgreSQL, and no `docker`/`podman` on the work machine.
- **FreeRADIUS is installed in that same VM.** Ports 1812/1813 were not listening as of 2026-09-27 — fix this before working on issues #6/#7.
- **Always read the DSN from the environment** (e.g. `DATABASE_URL`). The VM address is likely DHCP-assigned, so never hardcode the address or credentials in code. The address above is a recorded environment fact, not a constant to depend on.
- **Target `AccessPoint` version: OpenWrt 23.05.5.** Branch 23.05 is EOL; new APs are provisioned on >= 24.10 (see issue #24 and ADR-0001).
- **`ssh` to the VM needs a password** (publickey is rejected as of 2026-09-27), so anything that has to run inside the VM — FreeRADIUS configuration, `radclient`, `logread` — is human work, not agent work. Issue #1's scaffold runs entirely on the work machine.

## Repository layout and toolchain

Two independently-built applications in one repository, plus the docs that govern them:

```
backend/    Go 1.25 · Uber Fx · Gin · GORM · PostgreSQL · zap · viper · golang-migrate
frontend/   SvelteKit (Svelte 5 runes) · Tailwind v4 · shadcn-svelte · axios · TanStack Query · zod
```

Architecture, layering rules, and naming conventions come from the `backend-ddd-golang` and
`frontend-ddd-sveltekit` skills, read at the start of a session. Where the skills and this repo
disagree, this repo wins: the deviations are listed under "Scaffold decisions" below.

## Verification gates

Run these before calling any change done. They are what CI runs, so a green run here means a green
run there.

```bash
cd backend  && make ci          # build + vet + test + golangci-lint
cd backend  && make migrate-up  # needs DATABASE_URL; idempotent, safe to re-run
cd frontend && pnpm check && pnpm lint && pnpm test && pnpm build
```

Integration tests that need a database **skip** when `DATABASE_URL` is unset, so `make test` still
passes on a workstation that cannot reach the VM. A skip is not a pass: run `make migrate-up` and
the integration tests at least once per session that touches persistence.

`golangci-lint` on this work machine is a stub that reports no issues for any input. Do not trust
it for local verification — use `go vet`, `staticcheck`, and `gofmt`/`goimports` locally and let CI
run the real golangci-lint.

## Configuration contract

- The DSN is read from `DATABASE_URL` and nothing else. There is no default for it anywhere: an
  unset DSN fails at startup with a message naming the variable. Do not reintroduce one.
- No secret or DSN may be committed. `.env` files are git-ignored; `.env.example` is not. The
  GitHub repository is **public**, so this matters more than usual.
- Frontend server-only configuration (`BACKEND_URL`) lives in `lib/config/server-env.ts`; anything
  the browser may see lives in `lib/config/env.ts` and carries a `PUBLIC_` prefix. Importing a
  private env module from a component breaks the client build — that is why the two files are
  separate rather than one.

## Registering a new domain

A domain is one vertical slice. Both applications have exactly one place that mounts domains, so a
new domain adds itself there and nowhere else:

- **Backend** — `internal/application/<domain>/module.go` provides the domain's repository, service,
  and handler constructors. Aggregate that module in `internal/server/api/module.go`, and append the
  HTTP handler to the `[]Routed{}` slice in `internal/server/api/server.go` so it mounts under
  `/api/v1`. `/healthz` stays outside `/api/v1` on purpose.
- **Frontend** — `src/lib/domains/<domain>/index.ts` is the only surface other code may import. The
  domain's `api/` and `queries/` internals stay private (the frontend skill's Golden Rule 5).

## Scaffold decisions worth not re-litigating

- **Migrations are versioned SQL files**, not GORM AutoMigrate, because app-owned and
  FreeRADIUS-owned tables share one database and the changes must be reviewable (ADR-0005).
- **`app_schema_baseline`** is a canary: `cmd/migration` reads it back after `up`, so a silently
  skipped migration is reported as a failure instead of a green run against an unchanged schema.
- **Worker and gRPC servers do not exist yet.** They were omitted because Asynq needs Redis and
  nothing in the MVP scope needs RPC. Add them as `internal/server/<name>/` when a domain needs
  them — the directory names are reserved, so nothing has to be renamed to promote one later.
- **The Dockerfile is not built by CI.** There is no container runtime on the work machine (see
  "Development environment"), so a `make docker-build` here could not be verified. It is kept as the
  deploy artifact and is reachable through that target, not wired into CI on faith.
- **Swaggo annotations are documentation-only for now.** Handlers carry `@Summary`/`@Router` because
  the backend skill asks for them, but there is no generator target yet
  (`make swagger-gen` does not exist) and no `swag` dependency, so nothing checks them. When one is
  added it must write to `backend/api/docs`, **not** the repo's `docs/` — that directory belongs to
  `CONTEXT.md`'s ADRs and validation runbooks.
- **`make test` skips the database integration tests when `DATABASE_URL` is unset** rather than
  failing. A skip is not a pass; run `make migrate-up` and the tests at least once per session that
  touches persistence (see "Verification gates").
- **shadcn-svelte primitives are generated code.** `src/lib/components/ui/**` is excluded from
  ESLint and Prettier: re-running `pnpm dlx shadcn-svelte add <name>` is how they change, and
  formatting them here only creates diffs that the next `add` undoes. `components.json` carries the
  `vega` style, matching the sibling project `lite-point-of-sale`.
- **The frontend talks to its own `/api/**` routes, never to the Go API directly** (BFF pattern).
  `/api/health` always answers 200 with `{ backend_reachable, health, checked_at }` so the UI can
  tell "API down" apart from "API up, database down"; a forwarded 503 cannot carry that.

## Agent skills

### Issue tracker

Issues live as GitHub issues (via the `gh` CLI); external pull requests are not a triage surface. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles use their default label strings: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
