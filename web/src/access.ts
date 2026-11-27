import type { AuthUser } from '@/services/hoptrace';

export default function access(initialState: { currentUser?: AuthUser } | undefined) {
  const role = initialState?.currentUser?.role;
  return {
    isAdmin: role === 'admin',
    canWrite: role === 'admin' || role === 'operator',
  };
}
