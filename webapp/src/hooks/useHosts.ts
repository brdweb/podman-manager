import { useQuery } from '@tanstack/react-query';
import { getOverview } from '../api/hosts';

export function useOverview() {
  return useQuery({
    queryKey: ['overview'],
    queryFn: getOverview,
    refetchInterval: 10_000,
  });
}
