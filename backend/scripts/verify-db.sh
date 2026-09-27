#!/usr/bin/env bash
#
# Proves the dev database is reachable and that the whole path works:
# migration -> integration tests -> API health check -> frontend proxy -> database.
#
# Safe to re-run: the migration is idempotent, nothing is rolled back, and the
# servers started here are stopped again on exit (as whole process groups, so a
# `pnpm dev` cannot outlive the script).
#
# Every line of output — including the servers' own logs — is passed through a
# redactor, so a DSN that shows up in a driver error message can still be pasted
# into a public issue safely.
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

TMP="$(mktemp -d)"
API_LOG="$TMP/api.log"
WEB_LOG="$TMP/web.log"

failure=0

# Masks the userinfo part of any URL, so `postgres://user:pw@host` cannot leak.
redact() { sed -E 's#(://)[^:/@[:space:]]+:[^@[:space:]]+@#\1***:***@#g'; }

step() { printf '\n=== %s ===\n' "$1"; }
ok() { printf '[ok] %s\n' "$1"; }
bad() {
	printf '[FAIL] %s\n' "$1"
	failure=1
}

# Shows why a server did not answer, instead of leaving the failure unexplained.
# Prints the WHOLE log (up to a cap) plus its line count: a tail is how a previous
# run of this script hid the two facts that mattered — that the process was still
# alive, and that Fx had logged no constructor runs at all.
show_log() {
	local label="$1" log="$2" lines
	lines="$(wc -l <"$log" 2>/dev/null || echo 0)"
	printf '\n--- %s log (%s lines%s) ---\n' "$label" "$lines" "$([[ "$lines" -gt 100 ]] && echo ', last 100')"
	tail -n 100 "$log" 2>/dev/null | redact
	printf -- '--- end of %s log ---\n' "$label"
}

# Reports whether the process survived and what is listening on its port. "The API
# did not answer" has two very different causes — a dead process, and a live
# process with no listener — and they need different fixes.
show_process() {
	local label="$1" pid="$2" port="$3"

	printf '\n--- %s process state ---\n' "$label"
	if ps -o pid=,stat=,etime=,cmd= -p "$pid" 2>/dev/null; then
		printf 'alive\n'
	else
		printf 'NOT RUNNING (pid %s no longer exists)\n' "$pid"
	fi

	printf -- '--- listeners on :%s ---\n' "$port"
	if command -v ss >/dev/null 2>&1; then
		ss -ltnp 2>/dev/null | grep -E "[:.]${port}[[:space:]]" || printf 'nothing is listening on :%s\n' "$port"
	else
		printf 'ss not available\n'
	fi
}

# Starts a command in its own process group so cleanup can kill the whole tree
# (pnpm -> node -> vite), not just the shell that launched it.
start_server() {
	local log="$1"
	shift
	if command -v setsid >/dev/null 2>&1; then
		setsid "$@" >"$log" 2>&1 &
	else
		"$@" >"$log" 2>&1 &
	fi
	printf '%s' "$!"
}

stop_server() {
	local pid="${1:-}"
	[[ -z "$pid" ]] && return 0
	# Negative pid targets the process group created by setsid.
	kill -- "-$pid" 2>/dev/null || kill "$pid" 2>/dev/null
	wait "$pid" 2>/dev/null
	printf '[cleanup] stopped pid %s\n' "$pid"
}

# Bounded, so a hung request returns quickly instead of stalling the wait loop.
http_code() { curl -s --max-time 3 -o /dev/null -w '%{http_code}' "$1" 2>/dev/null; }

# Waits for a URL to answer with any HTTP status. Echoes the status code, or 000.
wait_for_http() {
	local url="$1" deadline=$((SECONDS + WAIT_SECONDS)) code
	while ((SECONDS < deadline)); do
		code="$(http_code "$url")"
		if [[ "$code" != "000" && -n "$code" ]]; then
			printf '%s' "$code"
			return 0
		fi
		sleep 1
	done
	printf '000'
	return 1
}

cleanup() {
	[[ -n "${WEB_PID:-}" ]] && stop_server "$WEB_PID"
	[[ -n "${API_PID:-}" ]] && stop_server "$API_PID"
	rm -rf "$TMP"
}
trap cleanup EXIT

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
step "5. build + start the API on :${API_PORT}"

if ! make build >/dev/null 2>&1; then
	bad "backend build failed"
	exit 1
fi

# A port already in use is the single most likely reason this step fails, and
# waiting 20 s to find out tells nobody anything.
if [[ "$(http_code "http://127.0.0.1:${API_PORT}/healthz")" != "000" ]]; then
	bad "something is already serving on :${API_PORT} — stop it, or run with a different port"
	printf '      PORT=18080 make verify-db   (this script also points the frontend at it)\n'
	exit 1
fi

API_PID="$(start_server "$API_LOG" ./bin/cuanku-api)"

code="$(wait_for_http "http://127.0.0.1:${API_PORT}/healthz")"
if [[ "$code" == "000" ]]; then
	bad "API did not answer on :${API_PORT} within ${WAIT_SECONDS}s"
	show_process "api" "$API_PID" "$API_PORT"
	show_log "api" "$API_LOG"
else
	printf 'GET /healthz -> HTTP %s\n' "$code"
	curl -s "http://127.0.0.1:${API_PORT}/healthz" | redact
	printf '\n'
	if [[ "$code" == "200" ]]; then
		ok "backend health check is 200 and reports the database"
	else
		bad "backend health check returned ${code} (503 means the database is not reachable)"
		show_log "api" "$API_LOG"
	fi
fi

# 6 ---------------------------------------------------------------------------
step "6. frontend proxy: browser -> SvelteKit /api/health -> Go /healthz -> PostgreSQL"

# The frontend's own default is :8080; if the API runs elsewhere the proxy has to
# be told, or this step would test a connection to nothing.
if [[ "$API_PORT" != "8080" ]]; then
	export BACKEND_URL="http://127.0.0.1:${API_PORT}"
	printf 'BACKEND_URL=%s (API is not on the default port)\n' "$BACKEND_URL"
fi

if [[ ! -d "$WEB_DIR/node_modules" ]]; then
	printf 'skipped: %s/node_modules is missing — run `pnpm install` there first\n' "$WEB_DIR"
else
	if [[ "$(http_code "http://127.0.0.1:${WEB_PORT}/api/health")" != "000" ]]; then
		bad "something is already serving on :${WEB_PORT} — stop it, or run with WEB_PORT=..."
		exit 1
	fi

	# --strictPort so vite fails instead of silently moving to :5174, which would
	# make the check below poll a server this script does not control.
	WEB_PID="$(start_server "$WEB_LOG" sh -c "cd '$WEB_DIR' && exec pnpm dev --port '$WEB_PORT' --strictPort")"

	code="$(wait_for_http "http://127.0.0.1:${WEB_PORT}/api/health")"
	if [[ "$code" == "000" ]]; then
		bad "frontend did not answer on :${WEB_PORT} within ${WAIT_SECONDS}s"
		show_process "frontend" "$WEB_PID" "$WEB_PORT"
		show_log "frontend" "$WEB_LOG"
	else
		body="$(curl -s "http://127.0.0.1:${WEB_PORT}/api/health" | redact)"
		printf 'GET /api/health -> HTTP %s\n' "$code"
		printf '%s\n' "$body"

		# The proxy answers 200 even when the backend is down — that is the whole
		# point of its envelope — so 200 alone proves nothing. The body does.
		if [[ "$code" != "200" ]]; then
			bad "frontend proxy returned ${code}"
		elif ! grep -q '"backend_reachable":true' <<<"$body"; then
			bad "the frontend reached its own proxy, but the proxy could not reach the backend"
			show_log "frontend" "$WEB_LOG"
		elif ! grep -q '"database":"up"' <<<"$body"; then
			bad "the backend answered but its database is not up"
			show_log "api" "$API_LOG"
		else
			ok "frontend -> backend -> database is connected"
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
