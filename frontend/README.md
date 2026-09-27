# Cuanku — frontend

SvelteKit app for Cuanku (WPA2-Enterprise user management for WISPs).

Setup, commands, and the end-to-end health-check walkthrough live in the
[repository README](../README.md). Architecture and layering rules are in
[`AGENTS.md`](../AGENTS.md).

Quick reference:

```bash
cp .env.example .env
pnpm dev      # http://localhost:5173
pnpm check    # svelte-check
pnpm lint     # prettier --check + eslint
pnpm test     # vitest
pnpm build
```

Structure follows the `frontend-ddd-sveltekit` layout: one folder per domain under
`src/lib/domains/<domain>/` with `schemas/`, `api/`, `queries/`, `components/`, and a single
public `index.ts` barrel. Localization of strings is in Indonesian, matching the rest of the
product.
