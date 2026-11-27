import { DownloadOutlined, PlusOutlined, SyncOutlined } from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { Badge, Button, Descriptions, Drawer, message, Popconfirm, Space, Table, Tag } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import {
  type Credential,
  createDevice,
  deleteDevice,
  type Device,
  type DeviceInterface,
  listCredentials,
  listDevices,
  listInterfaces,
  listSites,
  pollDevice,
  type Site,
  updateDevice,
} from '@/services/hoptrace';
import { downloadCsv, fmtBps, fmtUnix, fmtUptime, IF_KIND_META, ROLE_META, STATUS_META } from '../constants';

const DevicesPage: React.FC = () => {
  const actionRef = useRef<ActionType>(null);
  const [creds, setCreds] = useState<Credential[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Device | null>(null);
  const [detail, setDetail] = useState<Device | null>(null);
  const [ifaces, setIfaces] = useState<DeviceInterface[]>([]);
  const [ifLoading, setIfLoading] = useState(false);
  const [pollingId, setPollingId] = useState<string | null>(null);

  useEffect(() => {
    listCredentials().then((r) => setCreds(r.items || [])).catch(() => {});
    listSites().then((r) => setSites(r.items || [])).catch(() => {});
  }, []);

  const openDetail = async (d: Device) => {
    setDetail(d);
    setIfLoading(true);
    try {
      const r = await listInterfaces(d.id);
      setIfaces(r.items || []);
    } finally {
      setIfLoading(false);
    }
  };

  const handlePoll = async (d: Device) => {
    setPollingId(d.id);
    try {
      await pollDevice(d.id);
      message.success(`已重新采集 ${d.name || d.mgmtIp}`);
      actionRef.current?.reload();
    } finally {
      setPollingId(null);
    }
  };

  const columns: ProColumns<Device>[] = [
    {
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '名称 / IP / 型号 / 站点' },
    },
    {
      title: '设备名称',
      dataIndex: 'name',
      search: false,
      render: (_, r) => (
        <Space>
          <a onClick={() => openDetail(r)}>{r.name || r.mgmtIp}</a>
          {!r.managed && <Tag>未纳管占位</Tag>}
          {r.stackMembers > 1 && <Tag color="cyan">堆叠×{r.stackMembers}</Tag>}
        </Space>
      ),
    },
    { title: '管理 IP', dataIndex: 'mgmtIp', search: false, width: 135, render: (v) => v || '-' },
    {
      title: '机房 / 办公区',
      dataIndex: 'site',
      width: 130,
      valueType: 'select',
      fieldProps: {
        allowClear: true,
        placeholder: '全部机房/办公区',
        options: [
          ...sites.map((s) => ({ label: s.name, value: s.name })),
          { label: '未归属', value: '__unassigned__' },
        ],
      },
      render: (_, r) => (r.site ? <Tag color="geekblue">{r.site}</Tag> : <span style={{ color: '#bfbfbf' }}>未归属</span>),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      valueEnum: {
        online: { text: '在线' },
        offline: { text: '离线' },
        unknown: { text: '未知' },
      },
      render: (_, r) => {
        const m = STATUS_META[r.status] || { label: r.status, badge: 'default' as const };
        return <Badge status={m.badge} text={m.label} />;
      },
    },
    {
      title: '角色',
      dataIndex: 'role',
      width: 110,
      valueEnum: Object.fromEntries(Object.entries(ROLE_META).map(([k, v]) => [k, { text: v.label }])),
      render: (_, r) => {
        const m = ROLE_META[r.role] || { label: r.role, color: 'default' };
        return <Tag color={m.color}>{m.label}</Tag>;
      },
    },
    {
      title: '厂商 / 型号',
      dataIndex: 'model',
      search: false,
      render: (_, r) => `${r.vendor || '-'} ${r.model || ''}`.trim(),
    },
    { title: '版本', dataIndex: 'osVersion', search: false, width: 160, render: (v) => v || '-' },
    {
      title: 'CPU / 内存',
      dataIndex: 'cpuUsage',
      search: false,
      width: 110,
      render: (_, r) => (r.managed ? `${r.cpuUsage}% / ${r.memUsage}%` : '-'),
    },
    { title: '接口', dataIndex: 'interfaceCount', search: false, width: 70 },
    { title: '接入终端', dataIndex: 'endpointCount', search: false, width: 85 },
    {
      title: '操作',
      valueType: 'option',
      width: 170,
      render: (_, r) => [
        <a key="detail" onClick={() => openDetail(r)}>
          接口
        </a>,
        r.managed && (
          <a key="poll" onClick={() => handlePoll(r)}>
            {pollingId === r.id ? <SyncOutlined spin /> : '轮询'}
          </a>
        ),
        <a key="edit" onClick={() => setEditing(r)}>
          编辑
        </a>,
        <Popconfirm
          key="del"
          title="确认删除该设备及其关联数据？"
          onConfirm={async () => {
            await deleteDevice(r.id);
            message.success('已删除');
            actionRef.current?.reload();
          }}
        >
          <a style={{ color: '#ff4d4f' }}>删除</a>
        </Popconfirm>,
      ],
    },
  ];

  const credOptions = creds.map((c) => ({ label: `${c.name} (${c.version})`, value: c.id }));
  const siteOptions = sites.map((s) => ({ label: s.name, value: s.name }));

  return (
    <PageContainer title="网络设备">
      <ProTable<Device>
        actionRef={actionRef}
        rowKey="id"
        columns={columns}
        search={{ labelWidth: 'auto' }}
        toolBarRender={() => [
          <Button
            key="export"
            icon={<DownloadOutlined />}
            onClick={async () => {
              const res = await listDevices({ page: 1, pageSize: 500 });
              downloadCsv(
                `hoptrace-devices-${Date.now()}.csv`,
                ['名称', '管理IP', '状态', '角色', '厂商', '型号', '版本', '序列号', 'CPU%', '内存%', '温度', '接入终端数'],
                (res.items || []).map((d) => [
                  d.name, d.mgmtIp, d.status, d.role, d.vendor, d.model, d.osVersion, d.serial,
                  d.cpuUsage, d.memUsage, d.temperature, d.endpointCount,
                ]),
              );
            }}
          >
            导出 CSV
          </Button>,
          <Button key="add" type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            手工添加设备
          </Button>,
        ]}
        request={async (params) => {
          const res = await listDevices({
            keyword: params.keyword,
            status: params.status,
            role: params.role,
            site: params.site,
            page: params.current,
            pageSize: params.pageSize,
          });
          return { data: res.items || [], total: Number(res.total || 0), success: true };
        }}
      />

      <ModalForm
        title="手工添加设备"
        open={createOpen}
        onOpenChange={setCreateOpen}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (vals) => {
          await createDevice(vals as any);
          message.success('设备已添加并触发首次采集');
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="mgmtIp" label="管理 IP" rules={[{ required: true }]} placeholder="例如 10.255.0.1" />
        <ProFormSelect
          name="credentialId"
          label="SNMP 凭据"
          rules={[{ required: true }]}
          options={credOptions}
          placeholder="请选择凭据"
        />
        <ProFormText name="name" label="设备名称（留空自动读取 sysName）" />
        <ProFormDigit name="snmpPort" label="SNMP 端口" initialValue={161} fieldProps={{ precision: 0 }} />
        <ProFormSelect
          name="site"
          label="所属机房 / 办公区"
          allowClear
          showSearch
          options={siteOptions}
          placeholder="请选择机房或办公区（可先在「机房与办公区」中新建）"
        />
        <ProFormText name="location" label="物理位置（机柜/楼层）" />
      </ModalForm>

      <ModalForm
        title={`编辑设备: ${editing?.name || editing?.mgmtIp || ''}`}
        open={!!editing}
        onOpenChange={(o) => !o && setEditing(null)}
        modalProps={{ destroyOnHidden: true }}
        initialValues={editing ? {
          name: editing.name,
          credentialId: editing.credentialId !== '0' ? editing.credentialId : undefined,
          role: editing.roleManual ? editing.role : undefined,
          site: editing.site,
          location: editing.location,
        } : undefined}
        onFinish={async (vals) => {
          if (!editing) return false;
          await updateDevice(editing.id, vals);
          message.success('已更新');
          setEditing(null);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="name" label="设备名称" />
        <ProFormSelect name="credentialId" label="SNMP 凭据" options={credOptions} />
        <ProFormSelect
          name="role"
          label="手工指定角色（留空恢复自动判定）"
          allowClear
          options={Object.entries(ROLE_META).map(([k, v]) => ({ label: v.label, value: k }))}
        />
        <ProFormSelect
          name="site"
          label="所属机房 / 办公区"
          allowClear
          showSearch
          options={siteOptions}
          placeholder="请选择机房或办公区"
        />
        <ProFormText name="location" label="物理位置（机柜/楼层）" />
      </ModalForm>

      <Drawer
        title={detail ? `${detail.name || detail.mgmtIp} (${detail.model || detail.vendor})` : ''}
        width={900}
        open={!!detail}
        onClose={() => setDetail(null)}
      >
        {detail && (
          <>
            <Descriptions bordered size="small" column={2} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="管理 IP">{detail.mgmtIp || '-'}</Descriptions.Item>
              <Descriptions.Item label="底盘 MAC">{detail.chassisMac || '-'}</Descriptions.Item>
              <Descriptions.Item label="厂商 / 型号">{`${detail.vendor} ${detail.model}`}</Descriptions.Item>
              <Descriptions.Item label="系统版本">{detail.osVersion || '-'}</Descriptions.Item>
              <Descriptions.Item label="序列号">{detail.serial || '-'}</Descriptions.Item>
              <Descriptions.Item label="运行时长">{fmtUptime(detail.uptimeSeconds)}</Descriptions.Item>
              <Descriptions.Item label="硬件状态">
                CPU {detail.cpuUsage}% / 内存 {detail.memUsage}% / 温度 {detail.temperature}°C
              </Descriptions.Item>
              <Descriptions.Item label="最近采集">{fmtUnix(detail.lastPollAt)}</Descriptions.Item>
              <Descriptions.Item label="sysDescr" span={2}>
                {detail.sysDescr || '-'}
              </Descriptions.Item>
              {detail.lastError && (
                <Descriptions.Item label="最近错误" span={2}>
                  <span style={{ color: '#ff4d4f' }}>{detail.lastError}</span>
                </Descriptions.Item>
              )}
            </Descriptions>

            <Table
              rowKey="id"
              size="small"
              loading={ifLoading}
              pagination={{ pageSize: 15 }}
              dataSource={ifaces}
              columns={[
                { title: '接口名', dataIndex: 'name', width: 190 },
                {
                  title: '状态',
                  dataIndex: 'operStatus',
                  width: 80,
                  render: (v: string) => <Badge status={v === 'up' ? 'success' : 'error'} text={v} />,
                },
                {
                  title: '类别',
                  dataIndex: 'kind',
                  width: 110,
                  render: (v: string) => {
                    const m = IF_KIND_META[v] || { label: v, color: 'default' };
                    return <Tag color={m.color}>{m.label}</Tag>;
                  },
                },
                { title: '速率(Mbps)', dataIndex: 'speedMbps', width: 95 },
                {
                  title: '实时流量 / 利用率',
                  width: 175,
                  render: (_, it) =>
                    Number(it.inBps || 0) > 0 || Number(it.outBps || 0) > 0
                      ? `↓${fmtBps(it.inBps)} / ↑${fmtBps(it.outBps)} (${it.utilizationPct || 0}%)`
                      : '-',
                },
                {
                  title: '错包(入/出)',
                  width: 95,
                  render: (_, it) =>
                    Number(it.inErrors || 0) > 0 || Number(it.outErrors || 0) > 0
                      ? <Tag color="error">{`${it.inErrors || 0}/${it.outErrors || 0}`}</Tag>
                      : '0/0',
                },
                { title: '学习MAC', dataIndex: 'macCount', width: 80 },
                { title: 'LLDP 邻居', dataIndex: 'neighbor', render: (v) => v || '-' },
              ]}
            />
          </>
        )}
      </Drawer>
    </PageContainer>
  );
};

export default DevicesPage;
