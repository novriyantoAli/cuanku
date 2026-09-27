import { apiClient } from '$lib/api/client';
import { HealthReportSchema, type HealthReport } from '../schemas/health.schema';

/**
 * The health read, declared as an interface so the query layer depends on a
 * shape rather than on this object (frontend skill §3) — a test or a story can
 * swap in a fake without touching `queries/`.
 */
export interface HealthApi {
	report(signal?: AbortSignal): Promise<HealthReport>;
}

export const healthApi: HealthApi = {
	async report(signal?: AbortSignal): Promise<HealthReport> {
		const { data } = await apiClient.get('/health', { signal });

		// Parsed, never returned raw: an unexpected body must fail loudly here
		// instead of propagating `any` into the component (Golden Rule 4).
		return HealthReportSchema.parse(data);
	}
};
