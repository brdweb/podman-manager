import { get, post, del } from './client';
import type {
  ActionResult,
  Container,
  ContainerDetail,
  CreateContainerPayload,
  CreateNetworkPayload,
  CreateVolumePayload,
  Network,
  UpdateCheckResult,
  UpdateResult,
  Volume,
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

export function createContainer(
  host: string,
  payload: CreateContainerPayload
): Promise<ActionResult> {
  return post<ActionResult>(
    `/api/hosts/${encodeURIComponent(host)}/containers`,
    payload
  );
}

export function listNetworks(host: string): Promise<Network[]> {
  return get<Network[]>(`/api/hosts/${encodeURIComponent(host)}/networks`);
}

export function createNetwork(
  host: string,
  payload: CreateNetworkPayload
): Promise<Network> {
  return post<Network>(
    `/api/hosts/${encodeURIComponent(host)}/networks`,
    payload
  );
}

export function removeNetwork(
  host: string,
  name: string
): Promise<ActionResult> {
  return del<ActionResult>(
    `/api/hosts/${encodeURIComponent(host)}/networks/${encodeURIComponent(name)}`
  );
}

export function listVolumes(host: string): Promise<Volume[]> {
  return get<Volume[]>(`/api/hosts/${encodeURIComponent(host)}/volumes`);
}

export function createVolume(
  host: string,
  payload: CreateVolumePayload
): Promise<Volume> {
  return post<Volume>(
    `/api/hosts/${encodeURIComponent(host)}/volumes`,
    payload
  );
}

export function removeVolume(
  host: string,
  name: string
): Promise<ActionResult> {
  return del<ActionResult>(
    `/api/hosts/${encodeURIComponent(host)}/volumes/${encodeURIComponent(name)}`
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
