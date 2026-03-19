import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getConfig, saveConfig } from '../api/admin';

export function useConfig() {
  return useQuery({
    queryKey: ['admin', 'config'],
    queryFn: getConfig,
  });
}

export function useSaveConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: saveConfig,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['admin', 'config'] });
      void qc.invalidateQueries({ queryKey: ['session'] });
      void qc.invalidateQueries({ queryKey: ['overview'] });
      void qc.invalidateQueries({ queryKey: ['hosts'] });
      void qc.invalidateQueries({ queryKey: ['containers'] });
    },
  });
}
