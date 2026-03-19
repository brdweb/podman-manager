export interface HostStatus {
  name: string;
  address: string;
  mode?: string;
  status: 'online' | 'offline' | 'error';
  error?: string;
  latency?: string;
  system?: HostSystemInfo;
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
  manager: 'quadlet' | 'compose' | 'standalone';
  systemd_unit?: string;
  stats?: ContainerStats;
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

export interface HostSystemInfo {
  hostname?: string;
  os?: string;
  kernel?: string;
  uptime_seconds?: number;
  cpu_cores?: number;
  load_1?: number;
  load_5?: number;
  load_15?: number;
  memory_total_bytes?: number;
  memory_used_bytes?: number;
  disk_total_bytes?: number;
  disk_used_bytes?: number;
  disk_free_bytes?: number;
}

export interface ContainerStats {
  cpu_percent?: number;
  memory_usage_bytes?: number;
  memory_limit_bytes?: number;
  memory_percent?: number;
  pids?: number;
  network_input_bytes?: number;
  network_output_bytes?: number;
  block_input_bytes?: number;
  block_output_bytes?: number;
}

export interface SessionState {
  enabled: boolean;
  authenticated: boolean;
  username?: string;
}

export interface ConfigResponse {
  path: string;
  yaml: string;
  auth: {
    enabled: boolean;
    username?: string;
    has_password: boolean;
  };
}

export interface SaveConfigPayload {
  yaml: string;
  auth: {
    enabled: boolean;
    username: string;
    password: string;
  };
}

export interface SaveConfigResponse {
  success: boolean;
  message: string;
}
