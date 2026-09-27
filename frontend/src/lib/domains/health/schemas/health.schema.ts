import { z } from 'zod';

/**
 * What the Go API reports from `GET /healthz`. Field names are snake_case
 * because this mirrors `internal/pkg/health.Status` 1:1 — the backend DTO is
 * the contract, so the two must not be renamed independently.
 */
export const BackendHealthSchema = z.object({
	status: z.enum(['ok', 'degraded']),
	service: z.string(),
	version: z.string(),
	database: z.enum(['up', 'down']),
	checked_at: z.string()
});
export type BackendHealth = z.infer<typeof BackendHealthSchema>;

/**
 * What this app's own `/api/health` proxy reports to the browser.
 *
 * It is a separate schema, not a decoration of the one above, because
 * "the Go API is unreachable" and "the Go API answered, and its database is
 * down" are different facts. Collapsing them into one HTTP error would make
 * the status page unable to tell the operator which of the two happened.
 */
export const HealthReportSchema = z.object({
	backend_reachable: z.boolean(),
	health: BackendHealthSchema.nullable(),
	checked_at: z.string()
});
export type HealthReport = z.infer<typeof HealthReportSchema>;

/** Which of the three states the status view is allowed to show. */
export type HealthState = 'ok' | 'degraded' | 'unreachable';

/** Collapses a report into the one state the UI renders. */
export function healthStateOf(report: HealthReport): HealthState {
	if (!report.backend_reachable || report.health === null) {
		return 'unreachable';
	}
	return report.health.status === 'ok' ? 'ok' : 'degraded';
}
