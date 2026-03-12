import { useQuery } from '@tanstack/react-query';
import { getOverview, getHosts } from '../api/hosts';

export function useOverview() {
  return useQuery({
    queryKey: ['overview'],
    queryFn: getOverview,
    refetchInterval: 10_000,
  });
}

export function useHosts() {
  return useQuery({
    queryKey: ['hosts'],
    queryFn: getHosts,
    refetchInterval: 15_000,
  });
}
