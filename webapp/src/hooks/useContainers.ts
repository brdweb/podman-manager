import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getAllContainers,
  getContainers,
  getContainer,
  getContainerLogs,
  startContainer,
  stopContainer,
  restartContainer,
  removeContainer,
  checkContainerUpdate,
  updateContainer,
} from '../api/containers';

export function useContainers(host: string) {
  return useQuery({
    queryKey: ['containers', host],
    queryFn: () => getContainers(host),
    enabled: !!host,
    refetchInterval: 10_000,
  });
}

export function useAllContainers() {
  return useQuery({
    queryKey: ['containers', 'all'],
    queryFn: getAllContainers,
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
      void qc.invalidateQueries({ queryKey: ['containers', 'all'] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
    },
  });

  const stop = useMutation({
    mutationFn: ({ host, id }: { host: string; id: string }) =>
      stopContainer(host, id),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['containers', host] });
      void qc.invalidateQueries({ queryKey: ['containers', 'all'] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
    },
  });

  const restart = useMutation({
    mutationFn: ({ host, id }: { host: string; id: string }) =>
      restartContainer(host, id),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['containers', host] });
      void qc.invalidateQueries({ queryKey: ['containers', 'all'] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
    },
  });

  const remove = useMutation({
    mutationFn: ({ host, id, force }: { host: string; id: string; force?: boolean }) =>
      removeContainer(host, id, force),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['containers', host] });
      void qc.invalidateQueries({ queryKey: ['containers', 'all'] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
    },
  });

  return { start, stop, restart, remove };
}

export function useCheckUpdate(host: string, id: string, isRunning: boolean) {
  return useQuery({
    queryKey: ['container-update', host, id],
    queryFn: () => checkContainerUpdate(host, id),
    enabled: !!host && !!id && isRunning,
    refetchInterval: 300_000, // 5 minutes
    staleTime: 60_000, // 1 minute
  });
}

export function useUpdateContainer() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ host, id }: { host: string; id: string }) =>
      updateContainer(host, id),
    onSuccess: (_data, { host, id }) => {
      void qc.invalidateQueries({ queryKey: ['containers', host] });
      void qc.invalidateQueries({ queryKey: ['containers', 'all'] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
      void qc.invalidateQueries({ queryKey: ['container-update', host, id] });
    },
  });
}
