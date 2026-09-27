#!/usr/bin/env bash
#
# Proves the dev database is reachable and that the whole path works:
# migration -> integration tests -> API health check -> frontend proxy -> database.
#
# Safe to re-run: the migration is idempotent, nothing is rolled back, and the
# servers started here are stopped again on exit.
#
# Every line of output is passed through a redactor, so a DSN that shows up in a
# driver error message can still be pasted into a public issue safely.
#
# Run it through the Makefile so the DSN is loaded from ./.env:
#     cd backend && make verify-db

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

API_PORT="${PORT:-8080}"
WEB_PORT="${WEB_PORT:-5173}"
WEB_DIR="$ROOT/../frontend"
WAIT_SECONDS="${WAIT_SECONDS:-20}"

failure=0

# Masks the userinfo part of any URL, so `postgres://user:pw@host` cannot leak.
redact() { sed -E 's#(://)[^:/@[:space:]]+:[^@[:space:]]+@#\1***:***@#g'; }

step() { printf '\n=== %s ===\n' "$1"; }
ok() { printf '[ok] %s\n' "$1"; }
bad() {
	printf '[FAIL] %s\n' "$1"
	failure=1
}

cleanup() {
	if [[ -n "${API_PID:-}" ]]; then
		kill "$API_PID" 2>/dev/null && printf '\n[cleanup] stopped api (pid %s)\n' "$API_PID"
	fi
	if [[ -n "${WEB_PID:-}" ]]; then
		kill "$WEB_PID" 2>/dev/null && printf '[cleanup] stopped web (pid %s)\n' "$WEB_PID"
	fi
}
trap cleanup EXIT

# Waits for a URL to answer with any HTTP status, then prints it.
wait_for_http() {
	local url="$1" deadline=$((SECONDS + WAIT_SECONDS)) code
	while ((SECONDS < deadline)); do
		code="$(curl -s -o /dev/null -w '%{http_code}' "$url" 2>/dev/null)"
		if [[ "$code" != "000" && -n "$code" ]]; then
			printf '%s' "$code"
			return 0
		fi
		sleep 1
	done
	printf '000'
	return 1
}

if [[ -z "${DATABASE_URL:-}" ]]; then
	printf 'DATABASE_URL is not set in this shell.\n'
	printf 'Run this through the Makefile instead: cd backend && make verify-db\n'
	exit 1
fi

# 1 ---------------------------------------------------------------------------
step "1. config: the DSN reaches PostgreSQL, and which schema version is live"
if go run ./cmd/migration -direction=version 2>&1 | redact; then
	ok "database reachable"
else
	bad "database unreachable — stop here, the rest of the run is meaningless"
	exit 1
fi

# 2 ---------------------------------------------------------------------------
step "2. migrate-up (first run)"
if go run ./cmd/migration -direction=up 2>&1 | redact; then
	ok "migrations applied"
else
	bad "migrations failed"
fi

# 3 ---------------------------------------------------------------------------
step "3. migrate-up (second run — must report applied: false)"
second="$(go run ./cmd/migration -direction=up 2>&1 | redact)"
printf '%s\n' "$second"
if grep -q 'applied: false' <<<"$second"; then
	ok "re-run is a no-op (idempotent)"
else
	bad "re-run changed the schema, or the run failed"
fi

# 4 ---------------------------------------------------------------------------
step "4. integration tests against the real database"
if go test ./internal/pkg/migrator/... -count=1 -v -run 'TestUp|TestVersion' 2>&1 | redact; then
	ok "integration tests passed"
else
	bad "integration tests failed"
fi

# 5 ---------------------------------------------------------------------------
step "5. build + start the API"
if ! make build >/dev/null 2>&1; then
	bad "backend build failed"
	exit 1
fi
./bin/cuanku-api >/dev/null 2>&1 &
API_PID=$!

code="$(wait_for_http "http://127.0.0.1:${API_PORT}/healthz")"
if [[ "$code" == "000" ]]; then
	bad "API did not answer on :${API_PORT} within ${WAIT_SECONDS}s"
else
	printf 'GET /healthz -> HTTP %s\n' "$code"
	curl -s "http://127.0.0.1:${API_PORT}/healthz" | redact
	printf '\n'
	if [[ "$code" == "200" ]]; then
		ok "backend health check is 200 and reports the database"
	else
		bad "backend health check returned ${code} (503 means the database is not reachable)"
	fi
fi

# 6 ---------------------------------------------------------------------------
step "6. frontend proxy: browser -> SvelteKit /api/health -> Go /healthz -> PostgreSQL"
if [[ ! -d "$WEB_DIR/node_modules" ]]; then
	printf 'skipped: %s/node_modules is missing — run `pnpm install` there first\n' "$WEB_DIR"
else
	(cd "$WEB_DIR" && pnpm dev --port "$WEB_PORT" >/dev/null 2>&1) &
	WEB_PID=$!

	code="$(wait_for_http "http://127.0.0.1:${WEB_PORT}/api/health")"
	if [[ "$code" == "000" ]]; then
		bad "frontend did not answer on :${WEB_PORT} within ${WAIT_SECONDS}s"
	else
		printf 'GET /api/health -> HTTP %s\n' "$code"
		curl -s "http://127.0.0.1:${WEB_PORT}/api/health" | redact
		printf '\n'
		if [[ "$code" == "200" ]]; then
			ok "frontend reached the backend and the backend reached the database"
		else
			bad "frontend proxy returned ${code}"
		fi
	fi
fi

# 7 ---------------------------------------------------------------------------
step "result"
if [[ "$failure" -eq 0 ]]; then
	printf 'ALL STEPS PASSED\n'
else
	printf 'SOME STEPS FAILED (see [FAIL] above)\n'
fi
exit "$failure"
