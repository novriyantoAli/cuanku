// Public surface of the health domain. Nothing outside this folder imports the
// `api/` or `queries/` internals directly (frontend skill Golden Rule 5).
export { default as HealthCard } from './components/HealthCard.svelte';
export { createHealthQuery, healthKeys } from './queries/health.queries';
export {
	BackendHealthSchema,
	HealthReportSchema,
	healthStateOf,
	type BackendHealth,
	type HealthReport,
	type HealthState
} from './schemas/health.schema';
