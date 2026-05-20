import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createLink,
  deleteLink,
  getControlOverview,
  getLinks,
  getStacks,
  importHomepageLinks,
  sendOpenClawMessage,
  updateLink,
} from '../api/control';
import type { LinkPayload } from '../api/control';

export function useControlOverview() {
  return useQuery({
    queryKey: ['control-overview'],
    queryFn: getControlOverview,
    refetchInterval: 15_000,
  });
}

export function useLinks() {
  return useQuery({
    queryKey: ['links'],
    queryFn: getLinks,
    staleTime: 10_000,
  });
}

export function useStacks() {
  return useQuery({
    queryKey: ['stacks'],
    queryFn: getStacks,
    refetchInterval: 15_000,
  });
}

export function useCreateLink() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createLink,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['links'] }),
  });
}

export function useUpdateLink() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: LinkPayload }) =>
      updateLink(id, payload),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['links'] }),
  });
}

export function useDeleteLink() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteLink,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['links'] }),
  });
}

export function useImportHomepageLinks() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: importHomepageLinks,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['links'] }),
  });
}

export function useOpenClawChat() {
  return useMutation({
    mutationFn: sendOpenClawMessage,
  });
}
