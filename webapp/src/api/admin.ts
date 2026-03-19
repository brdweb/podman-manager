import { get, putJSON } from './client';
import type { ConfigResponse, SaveConfigPayload, SaveConfigResponse } from '../types/api';

export function getConfig(): Promise<ConfigResponse> {
  return get<ConfigResponse>('/api/admin/config');
}

export function saveConfig(payload: SaveConfigPayload): Promise<SaveConfigResponse> {
  return putJSON<SaveConfigResponse>('/api/admin/config', payload);
}
