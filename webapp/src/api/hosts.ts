import { get } from './client';
import type { HealthResponse, HostInfo, OverviewResponse } from '../types/api';

export function getHealth(): Promise<HealthResponse> {
  return get<HealthResponse>('/api/health');
}

export function getHosts(): Promise<HostInfo[]> {
  return get<HostInfo[]>('/api/hosts');
}

export function getOverview(): Promise<OverviewResponse> {
  return get<OverviewResponse>('/api/overview');
}
