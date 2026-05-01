import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createContainer,
  getAllContainers,
  getContainers,
  getContainer,
  listNetworks,
  createNetwork,
  removeNetwork,
  listVolumes,
  createVolume,
  removeVolume,
  startContainer,
  stopContainer,
  restartContainer,
  removeContainer,
  checkContainerUpdate,
  updateContainer,
} from '../api/containers';
import type { CreateContainerPayload, CreateNetworkPayload, CreateVolumePayload } from '../types/api';

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

export function useNetworks(host: string) {
  return useQuery({
    queryKey: ['networks', host],
    queryFn: () => listNetworks(host),
    enabled: !!host,
    refetchInterval: 30_000,
  });
}

export function useVolumes(host: string) {
  return useQuery({
    queryKey: ['volumes', host],
    queryFn: () => listVolumes(host),
    enabled: !!host,
    refetchInterval: 30_000,
  });
}

export function useCreateContainer() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ host, payload }: { host: string; payload: CreateContainerPayload }) =>
      createContainer(host, payload),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['containers', host] });
      void qc.invalidateQueries({ queryKey: ['containers', 'all'] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
    },
  });
}

export function useCreateNetwork() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ host, payload }: { host: string; payload: CreateNetworkPayload }) =>
      createNetwork(host, payload),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['networks', host] });
    },
  });
}

export function useRemoveNetwork() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ host, name }: { host: string; name: string }) =>
      removeNetwork(host, name),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['networks', host] });
    },
  });
}

export function useCreateVolume() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ host, payload }: { host: string; payload: CreateVolumePayload }) =>
      createVolume(host, payload),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['volumes', host] });
    },
  });
}

export function useRemoveVolume() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ host, name }: { host: string; name: string }) =>
      removeVolume(host, name),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['volumes', host] });
    },
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
