<script lang="ts">
	import { RefreshCw } from '@lucide/svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createHealthQuery } from '../queries/health.queries';
	import { healthStateOf, type HealthState } from '../schemas/health.schema';

	const query = createHealthQuery();

	const state = $derived(query.data ? healthStateOf(query.data) : null);
	const checkedAt = $derived(
		query.data ? new Date(query.data.checked_at).toLocaleTimeString('id-ID') : null
	);

	// Badge variants come from the generated primitive, so only the mapping from
	// state to presentation lives here.
	const presentation: Record<
		HealthState,
		{ label: string; variant: 'default' | 'destructive' | 'secondary'; detail: string }
	> = {
		ok: {
			label: 'Normal',
			variant: 'default',
			detail: 'API dan basis data merespons.'
		},
		degraded: {
			label: 'Terganggu',
			variant: 'secondary',
			detail: 'API merespons, tetapi basis data tidak terjangkau.'
		},
		unreachable: {
			label: 'Tidak terjangkau',
			variant: 'destructive',
			detail: 'API Cuanku tidak merespons.'
		}
	};
</script>

<Card>
	<CardHeader>
		<CardTitle class="flex flex-wrap items-center gap-2">
			Status layanan
			{#if state}
				<Badge variant={presentation[state].variant}>{presentation[state].label}</Badge>
			{/if}
		</CardTitle>
		<CardDescription>
			Hasil pemeriksaan terakhir dari API Cuanku, diperbarui berkala.
		</CardDescription>
	</CardHeader>

	<CardContent>
		{#if query.isPending}
			<div class="space-y-2" aria-busy="true" aria-label="Memuat status layanan">
				<Skeleton class="h-4 w-56" />
				<Skeleton class="h-4 w-32" />
			</div>
		{:else if query.isError}
			<div class="space-y-3">
				<p class="text-sm text-destructive">{query.error.message}</p>
				<Button variant="outline" size="sm" onclick={() => query.refetch()}>
					<RefreshCw />
					Coba lagi
				</Button>
			</div>
		{:else if query.data && state}
			<dl class="space-y-1 text-sm">
				<div class="flex gap-2">
					<dt class="w-32 text-muted-foreground">Keterangan</dt>
					<dd>{presentation[state].detail}</dd>
				</div>
				<div class="flex gap-2">
					<dt class="w-32 text-muted-foreground">Layanan</dt>
					<dd>{query.data.health?.service ?? '—'}</dd>
				</div>
				<div class="flex gap-2">
					<dt class="w-32 text-muted-foreground">Versi</dt>
					<dd>{query.data.health?.version ?? '—'}</dd>
				</div>
				<div class="flex gap-2">
					<dt class="w-32 text-muted-foreground">Basis data</dt>
					<dd>{query.data.health?.database === 'up' ? 'Terhubung' : 'Tidak terhubung'}</dd>
				</div>
				<div class="flex gap-2">
					<dt class="w-32 text-muted-foreground">Diperiksa</dt>
					<dd>{checkedAt}</dd>
				</div>
			</dl>

			<div class="mt-4">
				<Button variant="outline" size="sm" onclick={() => query.refetch()}>
					<RefreshCw />
					Periksa ulang
				</Button>
			</div>
		{/if}
	</CardContent>
</Card>
