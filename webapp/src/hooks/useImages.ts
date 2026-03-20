import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getImages, pullImage, removeImage, pruneImages } from '../api/images';
import { useOverview } from './useHosts';

export function useImages(host: string) {
  return useQuery({
    queryKey: ['images', host],
    queryFn: () => getImages(host),
    enabled: !!host,
    refetchInterval: 30_000,
  });
}

export function useAllImages() {
  const { data: overview } = useOverview();
  const hosts = overview?.hosts.map((h) => h.name) || [];

  return useQuery({
    queryKey: ['images', 'all'],
    queryFn: async () => {
      const promises = hosts.map(async (host) => {
        try {
          const images = await getImages(host);
          return images.map((img) => ({ ...img, host }));
        } catch (e) {
          console.error(`Failed to fetch images for host ${host}`, e);
          return [];
        }
      });
      const results = await Promise.all(promises);
      return results.flat();
    },
    enabled: hosts.length > 0,
    refetchInterval: 30_000,
  });
}

export function useImageActions() {
  const qc = useQueryClient();

  const pull = useMutation({
    mutationFn: ({ host, imageRef }: { host: string; imageRef: string }) =>
      pullImage(host, imageRef),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['images', host] });
      void qc.invalidateQueries({ queryKey: ['images', 'all'] });
    },
  });

  const remove = useMutation({
    mutationFn: ({ host, id, force }: { host: string; id: string; force?: boolean }) =>
      removeImage(host, id, force),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['images', host] });
      void qc.invalidateQueries({ queryKey: ['images', 'all'] });
    },
  });

  const prune = useMutation({
    mutationFn: ({ host }: { host: string }) => pruneImages(host),
    onSuccess: (_data, { host }) => {
      void qc.invalidateQueries({ queryKey: ['images', host] });
      void qc.invalidateQueries({ queryKey: ['images', 'all'] });
    },
  });

  return { pull, remove, prune };
}
