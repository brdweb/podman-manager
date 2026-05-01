import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getSession, login, logout } from '../api/auth';
import type { SessionInfo } from '../types/api';

export function useSession() {
  return useQuery({
    queryKey: ['session'],
    queryFn: getSession,
    retry: false,
    staleTime: 30_000,
  });
}

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ username, password }: { username: string; password: string }) =>
      login(username, password),
    onSuccess: (data) => {
      qc.setQueryData(['session'], data);
    },
  });
}

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: logout,
    onSuccess: () => {
      qc.setQueryData(['session'], {
        enabled: true,
        authenticated: false,
      });
    },
  });
}

export function hasRole(session: SessionInfo | undefined, role: NonNullable<SessionInfo['role']>) {
  return session?.role === role;
}

export function isAdmin(session: SessionInfo | undefined) {
  return hasRole(session, 'admin');
}
