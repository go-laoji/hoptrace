export default [
  {
    path: '/login',
    layout: false,
    component: './hoptrace/Login',
  },
  {
    path: '/dashboard',
    name: '总览仪表盘',
    icon: 'DashboardOutlined',
    component: './hoptrace/Dashboard',
  },
  {
    path: '/topology',
    name: '网络拓扑',
    icon: 'ApartmentOutlined',
    component: './hoptrace/Topology',
  },
  {
    path: '/sites',
    name: '机房与办公区',
    icon: 'BankOutlined',
    component: './hoptrace/Sites',
  },
  {
    path: '/devices',
    name: '网络设备',
    icon: 'ClusterOutlined',
    component: './hoptrace/Devices',
  },
  {
    path: '/endpoints',
    name: '接入终端',
    icon: 'LaptopOutlined',
    component: './hoptrace/Endpoints',
  },
  {
    path: '/discovery',
    name: '发现与凭据',
    icon: 'RadarChartOutlined',
    component: './hoptrace/Discovery',
  },
  {
    path: '/alerts',
    name: '告警与日志',
    icon: 'BellOutlined',
    component: './hoptrace/Alerts',
  },
  {
    path: '/ipam',
    name: 'IPAM 与异常',
    icon: 'GlobalOutlined',
    component: './hoptrace/Ipam',
  },
  {
    path: '/backups',
    name: '配置备份',
    icon: 'FileTextOutlined',
    component: './hoptrace/Backups',
  },
  {
    path: '/control',
    name: '管控与审批',
    icon: 'SafetyCertificateOutlined',
    component: './hoptrace/Control',
  },
  {
    path: '/users',
    name: '用户与权限',
    icon: 'TeamOutlined',
    access: 'isAdmin',
    component: './hoptrace/Users',
  },
  {
    path: '/',
    redirect: '/dashboard',
  },
  {
    path: '*',
    component: './exception/404',
  },
];
