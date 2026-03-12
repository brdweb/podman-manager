export interface HostStatus {
  name: string;
  address: string;
  status: 'online' | 'offline' | 'error';
  error?: string;
  latency?: string;
  containers?: Container[];
  container_count: ContainerCount;
}

export interface ContainerCount {
  total: number;
  running: number;
  stopped: number;
}

export interface Container {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  created: string;
  ports: PortMapping[];
  networks: NetworkInfo[];
  mounts: MountInfo[];
  labels?: Record<string, string>;
  host: string;
}

export interface PortMapping {
  host_ip: string;
  host_port: number;
  container_port: number;
  protocol: string;
}

export interface NetworkInfo {
  name: string;
  ip: string;
  gateway: string;
  mac?: string;
}

export interface MountInfo {
  type: string;
  source: string;
  destination: string;
  rw: boolean;
}

export interface ContainerDetail extends Container {
  env?: string[];
  hostname: string;
  restart_policy: string;
  network_mode: string;
  pid: number;
  started_at: string;
  finished_at?: string;
}

export interface ActionResult {
  success: boolean;
  message?: string;
  error?: string;
}

export interface OverviewResponse {
  hosts: HostStatus[];
}

export interface HealthResponse {
  status: string;
  hosts: Record<string, string>;
}

export interface HostInfo {
  name: string;
  address: string;
  mode: string;
  status: string;
  error?: string;
  latency?: string;
}

export interface LogsResponse {
  logs: string;
}
