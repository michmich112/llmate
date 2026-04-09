import { redirect } from '@sveltejs/kit';
import { api } from '$lib/api/client';

export function load() {
  if (!api.isAuthenticated()) {
    redirect(307, '/login');
  }
}
