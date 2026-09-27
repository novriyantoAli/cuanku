import MockAdapter from 'axios-mock-adapter';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { apiClient } from '$lib/api/client';
import { healthApi } from './health.api';

let mock: MockAdapter;

beforeEach(() => {
	mock = new MockAdapter(apiClient);
});

afterEach(() => {
	mock.restore();
});

const report = {
	backend_reachable: true,
	health: {
		status: 'ok',
		service: 'cuanku-api',
		version: '0.1.0',
		database: 'up',
		checked_at: '2026-09-27T10:00:00Z'
	},
	checked_at: '2026-09-27T10:00:01Z'
};

describe('healthApi.report', () => {
	it('calls the same-origin proxy, not the Go backend directly', async () => {
		mock.onGet('/health').reply(200, report);

		await healthApi.report();

		expect(mock.history.get[0].url).toBe('/health');
	});

	it('returns a parsed report', async () => {
		mock.onGet('/health').reply(200, report);

		await expect(healthApi.report()).resolves.toEqual(report);
	});

	it('fails loudly when the proxy answers with an unexpected body', async () => {
		mock.onGet('/health').reply(200, { unexpected: true });

		await expect(healthApi.report()).rejects.toThrow();
	});

	it('surfaces a normalised AppError when the proxy is unreachable', async () => {
		mock.onGet('/health').networkError();

		await expect(healthApi.report()).rejects.toMatchObject({
			message: 'Tidak dapat menghubungi server.'
		});
	});

	it('surfaces a 500 as a normalised AppError', async () => {
		mock.onGet('/health').reply(500, {});

		await expect(healthApi.report()).rejects.toMatchObject({
			status: 500,
			message: 'Server sedang bermasalah.'
		});
	});
});
