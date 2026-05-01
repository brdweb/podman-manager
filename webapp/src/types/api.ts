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

export interface Image {
  id: string;
  repository: string;
  tag: string;
  digest?: string;
  created: string;
  created_ago?: string;
  size?: string;
  host?: string;
}

export interface Volume {
  name: string;
  driver: string;
  labels?: Record<string, string>;
  createdAt?: string;
  mountpoint?: string;
}

export interface CreateVolumePayload {
  name: string;
  driver?: string;
  labels?: Record<string, string>;
  options?: Record<string, string>;
}

export interface Network {
  name: string;
  driver: string;
  subnets?: string[];
  gateway?: string;
  internal?: boolean;
  labels?: Record<string, string>;
}

export interface CreateNetworkPayload {
  name: string;
  driver?: string;
  subnets?: string[];
  gateway?: string;
  internal?: boolean;
  labels?: Record<string, string>;
  options?: Record<string, string>;
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

export interface CreateContainerPayload {
  name: string;
  image: string;
  network?: string;
  ports?: Array<{
    hostPort: number;
    containerPort: number;
    protocol: string;
  }>;
  envVars?: Record<string, string>;
  labels?: Record<string, string>;
  volumes?: Array<{
    volumeName: string;
    containerPath: string;
  }>;
  restartPolicy?: string;
  privileged?: boolean;
  capAdd?: string[];
  cmd?: string[];
  entrypoint?: string[];
  workingDir?: string;
  user?: string;
  memoryMB?: number;
  cpuShares?: number;
}

export interface ActionResult {
  success: boolean;
  message?: string;
  error?: string;
}

export interface OverviewResponse {
  hosts: HostStatus[];
}

export type PodmanEventType = 'container' | 'image' | 'volume' | 'network' | string;

export interface PodmanEventPayload {
  Type?: PodmanEventType;
  type?: PodmanEventType;
  Action?: string;
  action?: string;
  Status?: string;
  status?: string;
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  Image?: string;
  image?: string;
  Time?: string | number;
  time?: string | number;
  TimeNano?: number;
  timeNano?: number;
  attributes?: Record<string, string>;
  Attributes?: Record<string, string>;
  Actor?: {
    ID?: string;
    Attributes?: Record<string, string>;
  };
  actor?: {
    id?: string;
    attributes?: Record<string, string>;
  };
  [key: string]: unknown;
}

export interface PodmanEvent {
  host: string;
  event: PodmanEventPayload;
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

export interface UpdateCheckResult {
  container_id: string;
  container_name: string;
  image: string;
  local_digest?: string;
  remote_digest?: string;
  update_available: boolean;
  error?: string;
}

export interface UpdateResult {
  success: boolean;
  message?: string;
  error?: string;
  old_image?: string;
  new_image?: string;
}

export interface SessionInfo {
  enabled: boolean;
  authenticated: boolean;
  username?: string;
  role?: 'admin' | 'operator' | 'viewer';
}

export type SessionState = SessionInfo;

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
