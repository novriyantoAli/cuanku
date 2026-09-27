# AGENTS.md

## Development environment

Verified facts — do not rediscover them, and do not assume they are different:

- **Dev PostgreSQL lives in VirtualBox VM `deb12`** (NIC bridged to `enp3s0`): `172.16.0.72:5432`. There is no local PostgreSQL, and no `docker`/`podman` on the work machine.
- **FreeRADIUS is installed in that same VM.** Ports 1812/1813 were not listening as of 2026-09-27 — fix this before working on issues #6/#7.
- **Always read the DSN from the environment** (e.g. `DATABASE_URL`). The VM address is likely DHCP-assigned, so never hardcode the address or credentials in code. The address above is a recorded environment fact, not a constant to depend on.
- **Target `AccessPoint` version: OpenWrt 23.05.5.** Branch 23.05 is EOL; new APs are provisioned on >= 24.10 (see issue #24 and ADR-0001).

## Agent skills

### Issue tracker

Issues live as GitHub issues (via the `gh` CLI); external pull requests are not a triage surface. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles use their default label strings: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
