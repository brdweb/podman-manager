import { get } from './client';
import type { OverviewResponse } from '../types/api';

export function getOverview(): Promise<OverviewResponse> {
  return get<OverviewResponse>('/api/overview');
}
