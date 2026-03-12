import { get, post } from './client';
import type {
  ActionResult,
  Container,
  ContainerDetail,
  LogsResponse,
} from '../types/api';

export function getContainers(host: string): Promise<Container[]> {
  return get<Container[]>(`/api/hosts/${encodeURIComponent(host)}/containers`);
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
