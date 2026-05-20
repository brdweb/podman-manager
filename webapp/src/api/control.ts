import { del, get, postJSON, putJSON } from './client';

export interface LinkCategory {
  id: string;
  name: string;
  slug: string;
  description?: string;
  sort_order: number;
}

export interface ManagedLink {
  id: string;
  category_id: string;
  category_name: string;
  title: string;
  url: string;
  description?: string;
  icon?: string;
  icon_kind: 'favicon' | 'slug' | 'url' | 'upload';
  target: '_blank' | '_self';
  visibility_role: 'viewer' | 'operator' | 'admin';
  health_url?: string;
  sort_order: number;
  status: string;
  source?: string;
}

export interface LinkGroup {
  category: LinkCategory;
  links: ManagedLink[];
}

export interface LinksResponse {
  categories: LinkCategory[];
  links: ManagedLink[];
  groups: LinkGroup[];
}

export interface LinkPayload {
  category_id?: string;
  category_name: string;
  title: string;
  url: string;
  description?: string;
  icon?: string;
  target: '_blank' | '_self';
  visibility_role: 'viewer' | 'operator' | 'admin';
  health_url?: string;
  sort_order: number;
  status?: string;
  source?: string;
}

export interface ImportResult {
  imported: number;
  created: number;
  updated: number;
  skipped: number;
  errors?: string[];
}

export interface ControlOverview {
  product: string;
  version: string;
  links: number;
  capabilities: string[];
  integrations: Record<string, unknown>;
  hosts: {
    hosts?: unknown[];
  };
}

export interface StackListResponse {
  stacks: unknown[];
  message?: string;
}

export function getControlOverview() {
  return get<ControlOverview>('/api/v1/overview');
}

export function getLinks() {
  return get<LinksResponse>('/api/v1/links');
}

export function createLink(payload: LinkPayload) {
  return postJSON<ManagedLink>('/api/v1/links', payload);
}

export function updateLink(id: string, payload: LinkPayload) {
  return putJSON<ManagedLink>(`/api/v1/links/${encodeURIComponent(id)}`, payload);
}

export function deleteLink(id: string) {
  return del<void>(`/api/v1/links/${encodeURIComponent(id)}`);
}

export function importHomepageLinks() {
  return postJSON<ImportResult>('/api/v1/links/import-homepage', {});
}

export function getStacks() {
  return get<StackListResponse>('/api/v1/stacks');
}

export function sendOpenClawMessage(payload: { message: string; context?: unknown }) {
  return postJSON<unknown>('/api/v1/chat/openclaw', payload);
}
