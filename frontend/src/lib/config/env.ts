import { env as publicEnv } from '$env/dynamic/public';

/**
 * The single place the browser side reads configuration from (frontend skill
 * §10). Everything here is safe to ship to a browser, because SvelteKit only
 * exposes `PUBLIC_`-prefixed variables, and this file imports the *public* env
 * module only.
 *
 * Server-only configuration (`BACKEND_URL`) lives in `server-env.ts`: importing
 * a private env module from a component would break the client build, which is
 * exactly why the two are separate files rather than one.
 */
export const env = {
	/**
	 * Base URL the browser calls. Defaults to the same-origin SvelteKit
	 * `/api/**` routes (frontend skill §9, Pattern A), so no token ever has to
	 * be readable by client-side JavaScript.
	 */
	apiBaseUrl: publicEnv.PUBLIC_API_BASE_URL ?? '/api',
	/** How often the status view re-reads the backend health check. */
	healthPollMs: Number(publicEnv.PUBLIC_HEALTH_POLL_MS ?? 30_000)
} as const;
