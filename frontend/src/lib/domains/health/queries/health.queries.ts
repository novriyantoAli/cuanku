import { createQuery } from '@tanstack/svelte-query';
import { browser } from '$app/environment';
import { env } from '$lib/config/env';
import { healthApi } from '../api/health.api';
import type { HealthReport } from '../schemas/health.schema';

/** Query key root for this domain; every key starts here so invalidation is predictable. */
export const healthKeys = {
	all: ['health'] as const,
	report: () => [...healthKeys.all, 'report'] as const
};

/**
 * Polls the status endpoint.
 *
 * `enabled: browser` is deliberate: the request goes to this app's own `/api/health`
 * proxy, which only exists once the app is serving requests. Running it during SSR
 * would mean a relative URL with no origin — and would poll the backend from the
 * server on every page render, which is not what a status card is for.
 */
export function createHealthQuery() {
	return createQuery(() => ({
		queryKey: healthKeys.report(),
		queryFn: ({ signal }: { signal: AbortSignal }) => healthApi.report(signal),
		enabled: browser,
		refetchInterval: env.healthPollMs,
		retry: 1,
		staleTime: 0
	}));
}

export type { HealthReport };
