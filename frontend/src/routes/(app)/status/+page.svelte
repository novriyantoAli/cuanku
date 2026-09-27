<script lang="ts">
	import { HealthCard } from '$lib/domains/health';
</script>

<div class="mx-auto max-w-2xl space-y-4 p-6">
	<header class="space-y-1">
		<h1 class="text-lg font-semibold">Cuanku</h1>
		<p class="text-sm text-muted-foreground">
			Halaman pemeriksaan konektivitas frontend &rarr; backend &rarr; PostgreSQL.
		</p>
	</header>

	<!-- The route is a thin adapter: it renders a domain component and holds no
	     fetching, branching on loading/error state, or HTTP calls of its own. -->
	<HealthCard />
</div>
