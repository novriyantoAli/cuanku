import axios, { AxiosError, type AxiosInstance } from 'axios';
import { env } from '$lib/config/env';

/**
 * The one error shape every layer sees (frontend skill §11): components render
 * `message`, and never a raw AxiosError or a stack trace.
 */
export interface AppError {
	message: string;
	status?: number;
	code?: string;
}

/** Shape the Go API returns on failure: `{ "error": "..." }`. */
interface BackendErrorBody {
	error?: string;
}

/**
 * Normalises anything thrown by axios into an `AppError`. Exported for the
 * tests: this is the only place an error body is interpreted, so a second
 * interpretation cannot drift from it (Golden Rule 4's spirit, applied to errors).
 */
export function normalizeApiError(error: unknown): AppError {
	if (!axios.isAxiosError(error)) {
		return {
			message: error instanceof Error ? error.message : 'Terjadi kesalahan yang tidak dikenal.'
		};
	}

	const axiosError = error as AxiosError<BackendErrorBody>;

	if (axiosError.response) {
		const body = axiosError.response.data;
		return {
			message: body?.error ?? httpMessage(axiosError.response.status),
			status: axiosError.response.status,
			code: axiosError.code
		};
	}

	if (axiosError.code === 'ECONNABORTED') {
		return { message: 'Permintaan melebihi batas waktu.', code: axiosError.code };
	}

	return {
		message: 'Tidak dapat menghubungi server.',
		code: axiosError.code
	};
}

function httpMessage(status: number): string {
	if (status === 404) return 'Data tidak ditemukan.';
	if (status >= 500) return 'Server sedang bermasalah.';
	if (status >= 400) return 'Permintaan ditolak.';
	return 'Terjadi kesalahan yang tidak dikenal.';
}

/**
 * Creates the shared axios instance. Domains never call `axios.create()`
 * themselves (frontend skill §9) — they import `apiClient`.
 *
 * `baseURL` points at SvelteKit's own `/api/**` routes, not at the Go backend:
 * those routes run on the server, where an httpOnly session cookie is readable.
 * That is what makes the token pattern in §9 work once authentication lands.
 */
export function createApiClient(baseURL: string = env.apiBaseUrl): AxiosInstance {
	const client = axios.create({
		baseURL,
		timeout: 15_000,
		headers: { Accept: 'application/json' }
	});

	client.interceptors.response.use(
		(response) => response,
		(error: unknown) => Promise.reject(normalizeApiError(error))
	);

	return client;
}

/** The app-wide axios instance. */
export const apiClient = createApiClient();
