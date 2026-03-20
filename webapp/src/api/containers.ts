import { get, post, del } from './client';
import type {
  ActionResult,
  Container,
  ContainerDetail,
  LogsResponse,
  UpdateCheckResult,
  UpdateResult,
} from '../types/api';

export function getContainers(host: string): Promise<Container[]> {
  return get<Container[]>(`/api/hosts/${encodeURIComponent(host)}/containers`);
}

export function getAllContainers(): Promise<Container[]> {
  return get<Container[]>('/api/containers');
}

export function getContainer(
  host: string,
  id: string
): Promise<ContainerDetail> {
  return get<ContainerDetail>(
    `/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(id)}`
  );
}

export function startContainer(
  host: string,
  id: string
): Promise<ActionResult> {
  return post<ActionResult>(
    `/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(id)}/start`
  );
}

export function stopContainer(
  host: string,
  id: string
): Promise<ActionResult> {
  return post<ActionResult>(
    `/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(id)}/stop`
  );
}

export function restartContainer(
  host: string,
  id: string
): Promise<ActionResult> {
  return post<ActionResult>(
    `/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(id)}/restart`
  );
}

export function getContainerLogs(
  host: string,
  id: string,
  tail = 100
): Promise<LogsResponse> {
  return get<LogsResponse>(
    `/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(id)}/logs?tail=${tail}`
  );
}

export function removeContainer(
  host: string,
  id: string,
  force: boolean = false
): Promise<ActionResult> {
  return del<ActionResult>(
    `/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(id)}?force=${force}`
  );
}

export function checkContainerUpdate(
  host: string,
  id: string
): Promise<UpdateCheckResult> {
  return get<UpdateCheckResult>(
    `/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(id)}/update-check`
  );
}

export function updateContainer(
  host: string,
  id: string
): Promise<UpdateResult> {
  return post<UpdateResult>(
    `/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(id)}/update`
  );
}
