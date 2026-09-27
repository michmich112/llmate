import { error, isRedirect, redirect } from '@sveltejs/kit';
import { api } from '$lib/api/client';

export async function load({ route }: { route: { id: string | null } }) {
  if (!api.isAuthenticated()) {
    redirect(307, '/login');
  }

  let role = api.getRole();
  if (!role) {
    try {
      const me = await api.getMe();
      if (me.role !== 'admin' && me.role !== 'key') {
        api.clearAccessKey();
        redirect(307, '/login');
      }
      api.rememberRole(me.role);
      role = me.role;
    } catch (e) {
      if (isRedirect(e)) throw e;
      const message = e instanceof Error ? e.message : '';
      if (message === 'Unauthorized') {
        api.clearAccessKey();
        redirect(307, '/login');
      }
      error(503, message || 'Failed to resolve session');
    }
  }

  if (role === 'key' && route.id !== '/(dashboard)') {
    redirect(307, '/');
  }

  return { role };
}
