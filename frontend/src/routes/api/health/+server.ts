import { json } from '@sveltejs/kit';
import { serverEnv } from '$lib/config/server-env';
import {
	BackendHealthSchema,
	HealthReportSchema,
	type HealthReport
} from '$lib/domains/health/schemas/health.schema';
import type { RequestHandler } from './$types';

/**
 * Same-origin proxy to the Go API's `/healthz` (frontend skill §9, Pattern A).
 *
 * Why not call the backend straight from the browser: the Go API's address is
 * server-side configuration, and once authentication lands the session cookie
 * will be httpOnly — only a server route can read it. Doing it here from day one
 * means no token ever has to reach client-side JavaScript.
 *
 * Why it always answers 200 with an envelope instead of forwarding the 503: the
 * status view has to tell "the API is down" apart from "the API is up and its
 * database is down", and a bare HTTP error cannot carry that. Genuine proxy
 * failures still surface as an axios error in the api layer.
 *
 * Note the schema is imported from this domain's `schemas/` rather than the
 * `$lib/domains/health` barrel: a barrel import would pull `HealthCard.svelte`
 * (and therefore TanStack Query) into the server bundle. This is an
 * inside-domain import, not a cross-domain reach into another domain's internals.
 */
export const GET: RequestHandler = async ({ fetch }) => {
	const checkedAt = new Date().toISOString();
	let raw: unknown = null;

	try {
		const response = await fetch(`${serverEnv.backendUrl}/healthz`, {
			headers: { Accept: 'application/json' },
			signal: AbortSignal.timeout(serverEnv.backendTimeoutMs)
		});

		// The Go API answers 503 with a full report when a dependency is down, so a
		// 503 is still a readable answer — only a transport failure means "unreachable".
		if (response.ok || response.status === 503) {
			raw = await response.json();
		}
	} catch {
		raw = null;
	}

	const health = BackendHealthSchema.safeParse(raw);

	const report: HealthReport = HealthReportSchema.parse({
		backend_reachable: health.success,
		health: health.success ? health.data : null,
		checked_at: checkedAt
	});

	return json(report, {
		// A stale status must not be cached: the card's whole job is freshness.
		headers: { 'Cache-Control': 'no-store' }
	});
};
