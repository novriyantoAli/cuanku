import { redirect } from '@sveltejs/kit';

// The scaffold's only page is the status view; `/` sends you there so no page
// has to duplicate the health domain's markup.
export function load() {
	redirect(303, '/status');
}
