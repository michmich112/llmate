import { redirect } from '@sveltejs/kit';
import { api } from '$lib/api/client';

export function load({ route }: { route: { id: string } }) {
  if (!api.isAuthenticated()) {
    redirect(307, '/login');
  }
  const role = api.getRole() ?? 'admin';
  if (role === 'key' && route.id !== '/(dashboard)/usage') {
    redirect(307, '/usage');
  }
  return { role };
}
