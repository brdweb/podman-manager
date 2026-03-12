import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getContainers,
  getContainer,
  getContainerLogs,
  startContainer,
  stopContainer,
  restartContainer,
} from '../api/containers';

export function useContainers(host: string) {
  return useQuery({
    queryKey: ['containers', host],
    queryFn: () => getContainers(host),
    refetchInterval: 10_000,
  });
}

export function useContainerDetail(host: string, id: string) {
  return useQuery({
    queryKey: ['container', host, id],
    queryFn: () => getContainer(host, id),
    enabled: !!host && !!id,
  });
}

export function useContainerLogs(host: string, id: string, tail = 200) {
  return useQuery({
    queryKey: ['logs', host, id, tail],
    queryFn: () => getContainerLogs(host, id, tail),
    enabled: !!host && !!id,
    refetchInterval: 5_000,
  });
}

export function useContainerAction() {
  const qc = useQueryClient();

  const start = useMutation({
    mutationFn: ({ host, id }: { host: string; id: string }) =>
      startContainer(host, id),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['containers', host] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
    },
  });

  const stop = useMutation({
    mutationFn: ({ host, id }: { host: string; id: string }) =>
      stopContainer(host, id),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['containers', host] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
    },
  });

  const restart = useMutation({
    mutationFn: ({ host, id }: { host: string; id: string }) =>
      restartContainer(host, id),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['containers', host] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
    },
  });

  return { start, stop, restart };
}
