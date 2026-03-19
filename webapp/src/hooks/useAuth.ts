import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getSession, login, logout } from '../api/auth';

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
