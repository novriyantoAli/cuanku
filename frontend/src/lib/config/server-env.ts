import { env as privateEnv } from '$env/dynamic/private';

/**
 * Server-only configuration. Never import this from a component, a `+page.ts`,
 * or `+page.svelte` — SvelteKit will refuse to build the client bundle. It is
 * imported by the `+server.ts` proxy routes only (frontend skill §9, Pattern A).
 */
export const serverEnv = {
	/** Base URL of the Go API. Server-side only; the browser never sees it. */
	backendUrl: privateEnv.BACKEND_URL ?? 'http://127.0.0.1:8080',
	/** Timeout for the server-side hop to the Go API. */
	backendTimeoutMs: Number(privateEnv.BACKEND_TIMEOUT_MS ?? 10_000)
} as const;
