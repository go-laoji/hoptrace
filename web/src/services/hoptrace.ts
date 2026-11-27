import { request as umiRequest } from '@umijs/max';

function toSnakeKey(k: string): string {
  return k.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/(\d+)/g, '_$1').replace(/__/g, '_');
}

function toCamelKey(k: string): string {
  return k.replace(/_([a-z0-9])/g, (_, c) => c.toUpperCase());
}

function mapKeys(val: any, fn: (k: string) => string): any {
  if (Array.isArray(val)) return val.map((v) => mapKeys(v, fn));
  if (val !== null && typeof val === 'object') {
    const out: Record<string, any> = {};
    for (const [k, v] of Object.entries(val)) {
      if (v === undefined) continue;
      out[fn(k)] = mapKeys(v, fn);
    }
    return out;
  }
  return val;
}

const TOKEN_KEY = 'hoptrace_token';

export const getToken = () => localStorage.getItem(TOKEN_KEY) || '';
export const setToken = (tok: string) => localStorage.setItem(TOKEN_KEY, tok);
export const clearToken = () => localStorage.removeItem(TOKEN_KEY);

async function request<T = any>(url: string, opts?: Record<string, any>): Promise<T> {
  const next: Record<string, any> = opts ? { ...opts } : {};
  if (next.params) next.params = mapKeys(next.params, toSnakeKey);
  if (next.data) next.data = mapKeys(next.data, toSnakeKey);
  const tok = getToken();
  if (tok) {
    next.headers = { ...(next.headers || {}), Authorization: `Bearer ${tok}` };
  }
  const res = await umiRequest(url, next);
  return mapKeys(res, toCamelKey) as T;
}

export interface KV {
  name: string;
  value: string | number;
}

export interface EndpointEvent {
  id: string;
  endpointId: string;
  mac: string;
  ip: string;
  name: string;
  type: string;
  detail: string;
  at: string;
}

export interface DashboardData {
  devicesTotal: string;
  devicesOnline: string;
  devicesOffline: string;
  devicesUnmanaged: string;
  linksTotal: string;
  linksDown: string;
  linksStale: string;
  endpointsTotal: string;
  endpointsOnline: string;
  endpointsNew24h: string;
  endpointsUntrusted: string;
  devicesByRole: KV[];
  endpointsByType: KV[];
  recentEvents: EndpointEvent[];
}

export interface Device {
  id: string;
  name: string;
  mgmtIp: string;
  snmpPort: number;
  credentialId: string;
  managed: boolean;
  sysName: string;
  sysDescr: string;
  sysOid: string;
  vendor: string;
  family: string;
  model: string;
  osVersion: string;
  serial: string;
  role: string;
  roleManual: boolean;
  status: string;
  site: string;
  location: string;
  ips: string[];
  chassisMac: string;
  cpuUsage: number;
  memUsage: number;
  temperature: number;
  stackMembers: number;
  uptimeSeconds: string;
  lastPollAt: string;
  lastError: string;
  createdAt: string;
  interfaceCount: number;
  endpointCount: number;
}

export interface DeviceInterface {
  id: string;
  deviceId: string;
  ifIndex: number;
  name: string;
  descr: string;
  alias: string;
  speedMbps: string;
  adminStatus: string;
  operStatus: string;
  mac: string;
  kind: string;
  aggIfIndex: number;
  macCount: number;
  neighbor: string;
  inBps?: string;
  outBps?: string;
  utilizationPct?: number;
  inErrors?: string;
  outErrors?: string;
}

export interface InterfaceMetricPoint {
  at: string;
  inBps: string;
  outBps: string;
  utilizationPct: number;
  inErrors: string;
  outErrors: string;
}

export interface Credential {
  id: string;
  name: string;
  version: string;
  username: string;
  authProtocol: string;
  privProtocol: string;
  port: number;
  hasCommunity: boolean;
  hasAuthPassword: boolean;
  hasPrivPassword: boolean;
  createdAt: string;
}

export interface DiscoveryJob {
  id: string;
  status: string;
  seeds: string[];
  cidrs: string[];
  probed: number;
  total: number;
  found: number;
  log: string;
  startedAt: string;
  finishedAt: string;
}

export interface TopoNode {
  id: string;
  kind: 'device' | 'endpoint';
  refId: string;
  name: string;
  ip: string;
  role: string;
  family: string;
  status: string;
  vendor: string;
  model: string;
  type: string;
  managed: boolean;
  endpointCount: number;
  posX?: number;
  posY?: number;
  hasPos?: boolean;
}

export interface TopoLink {
  id: string;
  source: string;
  target: string;
  sourcePort: string;
  targetPort: string;
  kind: string;
  confidence: number;
  status: string;
  members: string[];
  inBps?: string;
  outBps?: string;
  utilizationPct?: number;
}

export interface TopologyData {
  nodes: TopoNode[];
  links: TopoLink[];
}

export interface Endpoint {
  id: string;
  mac: string;
  ip: string;
  ips: string[];
  vendor: string;
  type: string;
  typeManual: boolean;
  name: string;
  owner: string;
  tags: string[];
  status: string;
  trust: string;
  deviceId: string;
  deviceName: string;
  ifIndex: number;
  ifName: string;
  vlan: number;
  firstSeen: string;
  lastSeen: string;
  remark: string;
  accessType?: 'wired' | 'wireless';
  ssid?: string;
  apName?: string;
  rssi?: number;
}

export interface EndpointLocation {
  id: string;
  deviceId: string;
  deviceName: string;
  ifIndex: number;
  ifName: string;
  vlan: number;
  since: string;
  until: string;
}

export interface EndpointDetail {
  endpoint: Endpoint;
  locations: EndpointLocation[];
  events: EndpointEvent[];
}

// ---------- API 调用 ----------

export const getDashboard = () => request<DashboardData>('/api/v1/dashboard');

export const listDevices = (params: {
  keyword?: string;
  status?: string;
  family?: string;
  role?: string;
  site?: string;
  page?: number;
  pageSize?: number;
}) => request<{ items: Device[]; total: string }>('/api/v1/devices', { params });

export const getDevice = (id: string) => request<Device>(`/api/v1/devices/${id}`);

export const createDevice = (data: {
  name?: string;
  mgmtIp: string;
  snmpPort?: number;
  credentialId: string;
  site?: string;
  location?: string;
}) => request<Device>('/api/v1/devices', { method: 'POST', data });

export const updateDevice = (
  id: string,
  data: { name?: string; credentialId?: string; role?: string; site?: string; location?: string },
) => request<Device>(`/api/v1/devices/${id}`, { method: 'PUT', data });

export const deleteDevice = (id: string) =>
  request(`/api/v1/devices/${id}`, { method: 'DELETE' });

export const pollDevice = (id: string) =>
  request<Device>(`/api/v1/devices/${id}/poll`, { method: 'POST', data: {} });

export const listInterfaces = (deviceId: string) =>
  request<{ items: DeviceInterface[] }>(`/api/v1/devices/${deviceId}/interfaces`);

export const listCredentials = () => request<{ items: Credential[] }>('/api/v1/credentials');

export const createCredential = (data: Record<string, any>) =>
  request<Credential>('/api/v1/credentials', { method: 'POST', data });

export const updateCredential = (id: string, data: Record<string, any>) =>
  request<Credential>(`/api/v1/credentials/${id}`, { method: 'PUT', data });

export const deleteCredential = (id: string) =>
  request(`/api/v1/credentials/${id}`, { method: 'DELETE' });

export const startDiscovery = (data: {
  seeds: string[];
  cidrs: string[];
  credentialIds?: string[];
  expandNeighbors: boolean;
  maxDevices?: number;
  site?: string;
}) => request<DiscoveryJob>('/api/v1/discovery/jobs', { method: 'POST', data });

export const listDiscoveryJobs = (limit = 20) =>
  request<{ items: DiscoveryJob[] }>('/api/v1/discovery/jobs', { params: { limit } });

export const getTopology = (params: {
  expandDeviceIds?: string[];
  includeAllEndpoints?: boolean;
  site?: string;
}) => request<TopologyData>('/api/v1/topology', { params });

export const rebuildTopology = () =>
  request('/api/v1/topology/rebuild', { method: 'POST', data: {} });

export const listEndpoints = (params: {
  keyword?: string;
  status?: string;
  trust?: string;
  type?: string;
  deviceId?: string;
  accessType?: string;
  page?: number;
  pageSize?: number;
}) => request<{ items: Endpoint[]; total: string }>('/api/v1/endpoints', { params });

export const getEndpointDetail = (id: string) =>
  request<EndpointDetail>(`/api/v1/endpoints/${id}`);

export const updateEndpoint = (
  id: string,
  data: {
    name?: string;
    owner?: string;
    tags?: string[];
    trust?: string;
    type?: string;
    remark?: string;
  },
) => request<Endpoint>(`/api/v1/endpoints/${id}`, { method: 'PUT', data });

export const deleteEndpoint = (id: string) =>
  request(`/api/v1/endpoints/${id}`, { method: 'DELETE' });

export const listEvents = (params: {
  endpointId?: string;
  type?: string;
  page?: number;
  pageSize?: number;
}) => request<{ items: EndpointEvent[]; total: string }>('/api/v1/events', { params });

// ---------- 鉴权与用户 ----------

export interface AuthUser {
  id: string;
  username: string;
  displayName: string;
  role: 'admin' | 'operator' | 'viewer';
  createdAt: string;
}

export const login = (data: { username: string; password: string }) =>
  request<{ token: string; expiresAt: string; user: AuthUser }>('/api/v1/auth/login', {
    method: 'POST',
    data,
  });

export const getCurrentUser = () =>
  request<{ authEnabled: boolean; user?: AuthUser }>('/api/v1/auth/me', {
    skipErrorHandler: true,
  });

export const changePassword = (data: { oldPassword: string; newPassword: string }) =>
  request('/api/v1/auth/password', { method: 'POST', data });

export const listUsers = () => request<{ items: AuthUser[] }>('/api/v1/users');

export const createUser = (data: {
  username: string;
  password: string;
  displayName?: string;
  role: string;
}) => request<AuthUser>('/api/v1/users', { method: 'POST', data });

export const deleteUser = (id: string) =>
  request(`/api/v1/users/${id}`, { method: 'DELETE' });

// ---------- M2 端口管控、审批与写凭据 ----------

export interface ControlAction {
  id: string;
  action: string;
  deviceId: string;
  deviceName: string;
  ifIndex: number;
  ifName: string;
  endpointId: string;
  endpointMac: string;
  paramValue: string;
  prevValue: string;
  protocol: string;
  dryRunPreview: string;
  rollbackPreview: string;
  status: string;
  reason: string;
  requestedBy: string;
  approvedBy: string;
  resultLog: string;
  createdAt: string;
  executedAt: string;
  rolledBackAt: string;
}

export interface WriteCredential {
  id: string;
  name: string;
  protocol: string;
  username: string;
  port: number;
  hasPassword: boolean;
  nacWebhookUrl: string;
  createdAt: string;
}

export const listControlActions = (params: {
  status?: string;
  deviceId?: string;
  page?: number;
  pageSize?: number;
}) => request<{ items: ControlAction[]; total: string }>('/api/v1/control/actions', { params });

export const createControlAction = (data: {
  action: string;
  deviceId?: string;
  ifIndex?: number;
  endpointId?: string;
  paramValue?: string;
  protocol?: string;
  reason?: string;
  dryRunOnly?: boolean;
}) => request<ControlAction>('/api/v1/control/actions', { method: 'POST', data });

export const approveControlAction = (id: string, autoExecute = true) =>
  request<ControlAction>(`/api/v1/control/actions/${id}/approve`, {
    method: 'POST',
    data: { autoExecute },
  });

export const rejectControlAction = (id: string, reason = '') =>
  request<ControlAction>(`/api/v1/control/actions/${id}/reject`, {
    method: 'POST',
    data: { reason },
  });

export const executeControlAction = (id: string) =>
  request<ControlAction>(`/api/v1/control/actions/${id}/execute`, {
    method: 'POST',
    data: {},
  });

export const rollbackControlAction = (id: string) =>
  request<ControlAction>(`/api/v1/control/actions/${id}/rollback`, {
    method: 'POST',
    data: {},
  });

export const listWriteCredentials = () =>
  request<{ items: WriteCredential[] }>('/api/v1/control/credentials');

export const saveWriteCredential = (data: Record<string, any>) =>
  request<WriteCredential>('/api/v1/control/credentials', { method: 'POST', data });

export const deleteWriteCredential = (id: string) =>
  request(`/api/v1/control/credentials/${id}`, { method: 'DELETE' });

// ---------- SSE 实时事件订阅 ----------

export interface LiveEvent {
  type: string;
  source?: string;
  device?: string;
  message?: string;
  at: number;
}

export function subscribeLiveEvents(
  onUpdate: (ev: LiveEvent) => void,
  onStatus?: (connected: boolean) => void,
): () => void {
  const tok = getToken();
  const url = tok
    ? `/api/v1/events/stream?token=${encodeURIComponent(tok)}`
    : '/api/v1/events/stream';
  const es = new EventSource(url);
  es.addEventListener('connected', () => onStatus?.(true));
  es.onopen = () => onStatus?.(true);
  es.onerror = () => onStatus?.(false);
  es.addEventListener('topology_updated', (e: MessageEvent) => {
    try {
      onUpdate(JSON.parse(e.data) as LiveEvent);
    } catch {
      onUpdate({ type: 'topology_updated', at: Math.floor(Date.now() / 1000) });
    }
  });
  return () => es.close();
}

// ---------- M3: 路径追踪 / 布局保存 / 接口速率 / 批量导入 / 告警 / Syslog / IPAM / 配置备份 ----------

export interface PathHop {
  nodeId: string;
  kind: string;
  name: string;
  ip: string;
  role: string;
  inPort: string;
  outPort: string;
  linkId: string;
}

export interface TracePathReply {
  found: boolean;
  message: string;
  hops: PathHop[];
  nodeIds: string[];
  linkIds: string[];
}

export const tracePath = (source: string, target = '') =>
  request<TracePathReply>('/api/v1/topology/trace', { params: { source, target } });

export const saveTopologyLayout = (positions: { deviceId: string; x: number; y: number }[]) =>
  request('/api/v1/topology/layout', { method: 'POST', data: { positions } });

export const resetTopologyLayout = () =>
  request('/api/v1/topology/layout', { method: 'DELETE' });

export const listInterfaceMetrics = (deviceId: string, ifIndex: number, limit = 30) =>
  request<{ items: InterfaceMetricPoint[] }>(`/api/v1/devices/${deviceId}/interfaces/${ifIndex}/metrics`, {
    params: { limit },
  });

export const batchImportEndpoints = (items: {
  mac: string;
  name?: string;
  owner?: string;
  tags?: string[];
  trust?: string;
  type?: string;
  remark?: string;
}[]) =>
  request<{ updated: number; skipped: number; errors: string[] }>('/api/v1/endpoints/import', {
    method: 'POST',
    data: { items },
  });

export interface AlertItem {
  id: string;
  kind: string;
  severity: string;
  title: string;
  message: string;
  targetKey: string;
  deviceId: string;
  endpointId: string;
  status: string;
  ackedBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface AlertChannel {
  id: string;
  name: string;
  kind: string;
  webhookUrl: string;
  enabled: boolean;
  kinds: string[];
  createdAt: string;
}

export interface SyslogEntry {
  id: string;
  sourceIp: string;
  deviceId: string;
  deviceName: string;
  severity: number;
  facility: string;
  tag: string;
  message: string;
  receivedAt: string;
}

export const listAlerts = (params: { status?: string; severity?: string; kind?: string; page?: number; pageSize?: number }) =>
  request<{ items: AlertItem[]; total: string }>('/api/v1/alerts', { params });

export const ackAlert = (id: string) =>
  request<AlertItem>(`/api/v1/alerts/${id}/ack`, { method: 'POST', data: {} });

export const resolveAlert = (id: string) =>
  request<AlertItem>(`/api/v1/alerts/${id}/resolve`, { method: 'POST', data: {} });

export const listAlertChannels = () => request<{ items: AlertChannel[] }>('/api/v1/alerts/channels');

export const saveAlertChannel = (data: Record<string, any>) =>
  request<AlertChannel>('/api/v1/alerts/channels', { method: 'POST', data });

export const deleteAlertChannel = (id: string) =>
  request(`/api/v1/alerts/channels/${id}`, { method: 'DELETE' });

export const testAlertChannel = (id: string) =>
  request<{ ok: boolean; detail: string }>(`/api/v1/alerts/channels/${id}/test`, { method: 'POST', data: {} });

export const listSyslogs = (params: { keyword?: string; sourceIp?: string; maxSeverity?: number; page?: number; pageSize?: number }) =>
  request<{ items: SyslogEntry[]; total: string }>('/api/v1/syslogs', { params });

export interface SubnetSummary {
  cidr: string;
  totalHosts: number;
  usedHosts: number;
  freeHosts: number;
  usagePct: number;
  conflictCount: number;
  gatewayDevice: string;
}

export interface IpLease {
  ip: string;
  used: boolean;
  mac: string;
  macs: string[];
  conflict: boolean;
  vendor: string;
  endpointName: string;
  deviceName: string;
  ifName: string;
}

export interface NetworkAnomaly {
  kind: string;
  severity: string;
  target: string;
  detail: string;
  deviceId: string;
  deviceName: string;
  ifName: string;
}

export const listSubnets = () => request<{ items: SubnetSummary[] }>('/api/v1/ipam/subnets');

export const getSubnetDetail = (cidr: string) =>
  request<{ summary: SubnetSummary; leases: IpLease[] }>('/api/v1/ipam/subnet', { params: { cidr } });

export const listAnomalies = () => request<{ items: NetworkAnomaly[] }>('/api/v1/ipam/anomalies');

export interface ConfigBackup {
  id: string;
  deviceId: string;
  deviceName: string;
  sha256: string;
  sizeBytes: number;
  changed: boolean;
  trigger: string;
  status: string;
  error: string;
  createdAt: string;
}

export interface DiffLine {
  kind: 'context' | 'add' | 'del';
  oldLine: number;
  newLine: number;
  text: string;
}

export const listBackups = (params: { deviceId?: string; page?: number; pageSize?: number }) =>
  request<{ items: ConfigBackup[]; total: string }>('/api/v1/backups', { params });

export const runBackup = (deviceId: string) =>
  request<ConfigBackup>('/api/v1/backups/run', { method: 'POST', data: { deviceId } });

export const getBackupDetail = (id: string) =>
  request<{ meta: ConfigBackup; content: string }>(`/api/v1/backups/${id}`);

export const diffBackups = (fromId: string, toId: string) =>
  request<{ from: ConfigBackup; to: ConfigBackup; added: number; removed: number; lines: DiffLine[] }>(
    '/api/v1/backups/diff',
    { params: { fromId, toId } },
  );

// ---------- 机房 / 办公区管理 ----------

export interface Site {
  id: string;
  name: string;
  code: string;
  type: 'datacenter' | 'office' | 'branch' | 'campus' | string;
  address: string;
  contact: string;
  cidrs: string[];
  description: string;
  deviceCount: number;
  onlineDeviceCount: number;
  endpointCount: number;
  createdAt: string;
}

export const listSites = (params?: { keyword?: string; type?: string }) =>
  request<{ items: Site[] }>('/api/v1/sites', { params });

export const getSite = (id: string) => request<Site>(`/api/v1/sites/${id}`);

export const createSite = (data: {
  name: string;
  code?: string;
  type?: string;
  address?: string;
  contact?: string;
  cidrs?: string[];
  description?: string;
}) => request<Site>('/api/v1/sites', { method: 'POST', data });

export const updateSite = (
  id: string,
  data: {
    name: string;
    code?: string;
    type?: string;
    address?: string;
    contact?: string;
    cidrs?: string[];
    description?: string;
  },
) => request<Site>(`/api/v1/sites/${id}`, { method: 'PUT', data });

export const deleteSite = (id: string) =>
  request(`/api/v1/sites/${id}`, { method: 'DELETE' });

export const assignSiteDevices = (id: string, deviceIds: string[]) =>
  request(`/api/v1/sites/${id}/devices`, { method: 'POST', data: { deviceIds } });
