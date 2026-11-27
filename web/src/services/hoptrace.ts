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
