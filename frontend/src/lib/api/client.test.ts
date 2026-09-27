import axios from 'axios';
import { describe, expect, it } from 'vitest';
import { createApiClient, normalizeApiError } from './client';

describe('createApiClient', () => {
	it('defaults to the same-origin proxy base URL', () => {
		expect(createApiClient().defaults.baseURL).toBe('/api');
	});

	it('accepts an explicit base URL', () => {
		expect(createApiClient('http://127.0.0.1:8080/api/v1').defaults.baseURL).toBe(
			'http://127.0.0.1:8080/api/v1'
		);
	});
});

describe('normalizeApiError', () => {
	it('uses the backend error message when the API sends one', () => {
		const error = new axios.AxiosError('Request failed', 'ERR_BAD_REQUEST', undefined, undefined, {
			status: 409,
			statusText: 'Conflict',
			headers: {},
			config: { headers: {} } as never,
			data: { error: 'email sudah terdaftar' }
		});

		expect(normalizeApiError(error)).toEqual({
			message: 'email sudah terdaftar',
			status: 409,
			code: 'ERR_BAD_REQUEST'
		});
	});

	it('falls back to a human message per status class', () => {
		const error = new axios.AxiosError('Request failed', 'ERR_BAD_RESPONSE', undefined, undefined, {
			status: 503,
			statusText: 'Service Unavailable',
			headers: {},
			config: { headers: {} } as never,
			data: {}
		});

		expect(normalizeApiError(error).message).toBe('Server sedang bermasalah.');
	});

	it('reports a timeout without claiming the server said anything', () => {
		const error = new axios.AxiosError('timeout of 15000ms exceeded', 'ECONNABORTED');

		expect(normalizeApiError(error)).toEqual({
			message: 'Permintaan melebihi batas waktu.',
			code: 'ECONNABORTED'
		});
	});

	it('reports an unreachable server', () => {
		const error = new axios.AxiosError('Network Error', 'ERR_NETWORK');

		expect(normalizeApiError(error)).toEqual({
			message: 'Tidak dapat menghubungi server.',
			code: 'ERR_NETWORK'
		});
	});

	it('passes through a plain Error message', () => {
		expect(normalizeApiError(new Error('boom'))).toEqual({ message: 'boom' });
	});
});
