import { describe, expect, it } from 'vitest';
import {
	BackendHealthSchema,
	HealthReportSchema,
	healthStateOf,
	type BackendHealth,
	type HealthReport
} from './health.schema';

const backendHealth: BackendHealth = {
	status: 'ok',
	service: 'cuanku-api',
	version: '0.1.0',
	database: 'up',
	checked_at: '2026-09-27T10:00:00Z'
};

describe('BackendHealthSchema', () => {
	it('accepts the exact body the Go API returns', () => {
		expect(BackendHealthSchema.parse(backendHealth)).toEqual(backendHealth);
	});

	it('rejects an unknown status', () => {
		expect(BackendHealthSchema.safeParse({ ...backendHealth, status: 'fine' }).success).toBe(false);
	});

	it('rejects a missing database field', () => {
		const { status, service, version, checked_at } = backendHealth;

		expect(BackendHealthSchema.safeParse({ status, service, version, checked_at }).success).toBe(
			false
		);
	});
});

describe('HealthReportSchema', () => {
	it('accepts a reachable report', () => {
		const report = { backend_reachable: true, health: backendHealth, checked_at: 'x' };

		expect(HealthReportSchema.parse(report)).toEqual(report);
	});

	it('accepts an unreachable report with a null health', () => {
		const report = { backend_reachable: false, health: null, checked_at: 'x' };

		expect(HealthReportSchema.parse(report)).toEqual(report);
	});
});

describe('healthStateOf', () => {
	const report = (overrides: Partial<HealthReport>): HealthReport => ({
		backend_reachable: true,
		health: backendHealth,
		checked_at: 'x',
		...overrides
	});

	it('is ok when the backend and its database are up', () => {
		expect(healthStateOf(report({}))).toBe('ok');
	});

	it('is degraded when the backend answers but its database is down', () => {
		const degraded = report({
			health: { ...backendHealth, status: 'degraded', database: 'down' }
		});

		expect(healthStateOf(degraded)).toBe('degraded');
	});

	it('is unreachable when the backend never answered', () => {
		expect(healthStateOf(report({ backend_reachable: false, health: null }))).toBe('unreachable');
	});
});
