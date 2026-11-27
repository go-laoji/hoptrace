import dayjs from 'dayjs';

export const ROLE_META: Record<string, { label: string; color: string }> = {
  firewall: { label: '防火墙', color: 'volcano' },
  core: { label: '核心交换机', color: 'purple' },
  aggregation: { label: '汇聚交换机', color: 'geekblue' },
  access: { label: '接入交换机', color: 'blue' },
  router: { label: '路由器', color: 'cyan' },
  ac: { label: '无线控制器', color: 'magenta' },
  unknown: { label: '未知/占位', color: 'default' },
};

export const STATUS_META: Record<string, { label: string; badge: 'success' | 'error' | 'default' | 'processing' }> = {
  online: { label: '在线', badge: 'success' },
  offline: { label: '离线', badge: 'error' },
  unknown: { label: '未知', badge: 'default' },
};

export const IF_KIND_META: Record<string, { label: string; color: string }> = {
  uplink: { label: '上联/互联', color: 'purple' },
  edge: { label: '边缘接入口', color: 'green' },
  suspect: { label: '疑似私接', color: 'orange' },
  lag: { label: '聚合口', color: 'geekblue' },
  lag_member: { label: '聚合成员', color: 'default' },
};

export const ENDPOINT_TYPE_META: Record<string, { label: string; color: string }> = {
  pc: { label: 'PC / 办公终端', color: 'blue' },
  phone: { label: 'IP 电话', color: 'cyan' },
  ap: { label: '无线 AP', color: 'purple' },
  camera: { label: '安防摄像头', color: 'volcano' },
  printer: { label: '网络打印机', color: 'gold' },
  server: { label: '服务器', color: 'geekblue' },
  mobile: { label: '移动设备', color: 'lime' },
  iot: { label: 'IoT 设备', color: 'magenta' },
  unknown: { label: '未知类型', color: 'default' },
};

export const TRUST_META: Record<string, { label: string; color: string }> = {
  trusted: { label: '已信任', color: 'success' },
  unknown: { label: '未确认', color: 'warning' },
  blocked: { label: '已标记阻断', color: 'error' },
};

export const EVENT_META: Record<string, { label: string; color: string }> = {
  new: { label: '新终端接入', color: 'blue' },
  online: { label: '上线', color: 'green' },
  offline: { label: '下线', color: 'default' },
  moved: { label: '位置漂移', color: 'orange' },
  ip_changed: { label: 'IP 变更', color: 'purple' },
};

export function fmtUnix(ts?: string | number): string {
  const n = Number(ts || 0);
  if (!n) return '-';
  return dayjs.unix(n).format('YYYY-MM-DD HH:mm:ss');
}

export function fmtUptime(secs?: string | number): string {
  const n = Number(secs || 0);
  if (!n) return '-';
  const d = Math.floor(n / 86400);
  const h = Math.floor((n % 86400) / 3600);
  const m = Math.floor((n % 3600) / 60);
  return d > 0 ? `${d}天 ${h}小时` : `${h}小时 ${m}分`;
}
