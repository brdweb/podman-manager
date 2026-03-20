import { get, post, postJSON, del } from './client';
import type { ActionResult, Image } from '../types/api';

export function getImages(host: string): Promise<Image[]> {
  return get<Image[]>(`/api/hosts/${encodeURIComponent(host)}/images`);
}

export function pullImage(host: string, imageRef: string): Promise<ActionResult> {
  return postJSON<ActionResult>(`/api/hosts/${encodeURIComponent(host)}/images/pull`, {
    image: imageRef,
  });
}

export function removeImage(host: string, id: string, force = false): Promise<ActionResult> {
  const query = force ? '?force=true' : '';
  return del<ActionResult>(`/api/hosts/${encodeURIComponent(host)}/images/${encodeURIComponent(id)}${query}`);
}

export function pruneImages(host: string): Promise<{ success: boolean; pruned_images: string[] }> {
  return post<{ success: boolean; pruned_images: string[] }>(`/api/hosts/${encodeURIComponent(host)}/images/prune`);
}
