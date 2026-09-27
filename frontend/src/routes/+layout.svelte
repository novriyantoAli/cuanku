<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';

	let { children } = $props();

	// One QueryClient for the whole app, provided once here; domains never create
	// their own (frontend skill §7).
	const queryClient = new QueryClient({
		defaultOptions: {
			queries: {
				staleTime: 30_000,
				refetchOnWindowFocus: false,
				retry: 1
			}
		}
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Cuanku</title>
	<meta name="description" content="Manajemen pengguna WPA2-Enterprise untuk WISP." />
</svelte:head>

<QueryClientProvider client={queryClient}>
	{@render children()}
</QueryClientProvider>
