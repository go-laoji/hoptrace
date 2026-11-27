import { ApartmentOutlined, ClusterOutlined, PlusOutlined } from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
  StatisticCard,
} from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Badge, Button, Card, Drawer, message, Popconfirm, Select, Space, Table, Tag } from 'antd';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  assignSiteDevices,
  createSite,
  deleteSite,
  type Device,
  listDevices,
  listSites,
  type Site,
  updateSite,
} from '@/services/hoptrace';
import { ROLE_META, SITE_TYPE_META, STATUS_META } from '../constants';

function parseCidrs(raw?: string): string[] {
  return String(raw || '')
    .split(/[\s,;，；]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

const SitesPage: React.FC = () => {
  const actionRef = useRef<ActionType>(null);
  const [sites, setSites] = useState<Site[]>([]);
  const [allDevices, setAllDevices] = useState<Device[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Site | null>(null);
  const [managingSite, setManagingSite] = useState<Site | null>(null);
  const [addDeviceIds, setAddDeviceIds] = useState<string[]>([]);
  const [assigning, setAssigning] = useState(false);

  const loadSummary = useCallback(async () => {
    try {
      const [sRes, dRes] = await Promise.all([
        listSites(),
        listDevices({ page: 1, pageSize: 500 }),
      ]);
      setSites(sRes.items || []);
      setAllDevices(dRes.items || []);
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    loadSummary();
  }, [loadSummary]);

  const dcCount = sites.filter((s) => s.type === 'datacenter').length;
  const officeCount = sites.filter((s) => s.type === 'office').length;
  const assignedCount = allDevices.filter((d) => !!d.site).length;
  const unassignedCount = allDevices.length - assignedCount;

  const siteDevices = managingSite
    ? allDevices.filter((d) => d.site === managingSite.name)
    : [];
  const candidateDevices = managingSite
    ? allDevices.filter((d) => d.site !== managingSite.name)
    : [];

  const handleAssignSelected = async () => {
    if (!managingSite || addDeviceIds.length === 0) return;
    setAssigning(true);
    try {
      await assignSiteDevices(managingSite.id, addDeviceIds);
      message.success(`已将 ${addDeviceIds.length} 台设备划入「${managingSite.name}」`);
      setAddDeviceIds([]);
      await loadSummary();
      actionRef.current?.reload();
    } finally {
      setAssigning(false);
    }
  };

  const handleRemoveDevice = async (deviceId: string) => {
    await assignSiteDevices('0', [deviceId]);
    message.success('已移出该机房/办公区');
    await loadSummary();
    actionRef.current?.reload();
  };

  const columns: ProColumns<Site>[] = [
    {
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '名称 / 编码 / 地址 / 负责人' },
    },
    {
      title: '名称',
      dataIndex: 'name',
      search: false,
      render: (_, r) => (
        <Space>
          <a onClick={() => setManagingSite(r)} style={{ fontWeight: 500 }}>
            {r.name}
          </a>
          {r.code && <Tag>{r.code}</Tag>}
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 100,
      valueEnum: Object.fromEntries(
        Object.entries(SITE_TYPE_META).map(([k, v]) => [k, { text: v.label }]),
      ),
      render: (_, r) => {
        const m = SITE_TYPE_META[r.type] || { label: r.type, color: 'default' };
        return <Tag color={m.color}>{m.label}</Tag>;
      },
    },
    { title: '地址 / 楼层', dataIndex: 'address', search: false, render: (v) => v || '-' },
    { title: '负责人', dataIndex: 'contact', search: false, width: 120, render: (v) => v || '-' },
    {
      title: '关联网段',
      dataIndex: 'cidrs',
      search: false,
      render: (_, r) =>
        (r.cidrs || []).length > 0 ? (
          <Space size={4} wrap>
            {r.cidrs.map((c) => (
              <Tag key={c} color="geekblue">
                {c}
              </Tag>
            ))}
          </Space>
        ) : (
          '-'
        ),
    },
    {
      title: '在线 / 设备数',
      dataIndex: 'deviceCount',
      search: false,
      width: 120,
      render: (_, r) => (
        <a onClick={() => setManagingSite(r)}>
          <span style={{ color: '#52c41a', fontWeight: 600 }}>{r.onlineDeviceCount || 0}</span>
          {' / '}
          {r.deviceCount || 0}
        </a>
      ),
    },
    {
      title: '接入终端',
      dataIndex: 'endpointCount',
      search: false,
      width: 90,
      render: (_, r) => r.endpointCount || 0,
    },
    { title: '备注', dataIndex: 'description', search: false, ellipsis: true, render: (v) => v || '-' },
    {
      title: '操作',
      valueType: 'option',
      width: 240,
      render: (_, r) => [
        <a
          key="topo"
          onClick={() => history.push(`/topology?site=${encodeURIComponent(r.name)}`)}
        >
          <ApartmentOutlined /> 拓扑
        </a>,
        <a key="devs" onClick={() => setManagingSite(r)}>
          <ClusterOutlined /> 设备
        </a>,
        <a key="edit" onClick={() => setEditing(r)}>
          编辑
        </a>,
        <Popconfirm
          key="del"
          title="确认删除该机房/办公区？需先移出名下设备。"
          onConfirm={async () => {
            await deleteSite(r.id);
            message.success('已删除');
            await loadSummary();
            actionRef.current?.reload();
          }}
        >
          <a style={{ color: '#ff4d4f' }}>删除</a>
        </Popconfirm>,
      ],
    },
  ];

  const siteTypeOptions = Object.entries(SITE_TYPE_META).map(([k, v]) => ({
    label: v.label,
    value: k,
  }));

  return (
    <PageContainer title="机房与办公区管理">
      <StatisticCard.Group direction="row" style={{ marginBottom: 16 }}>
        <StatisticCard statistic={{ title: '机房总数', value: dcCount, suffix: '个' }} />
        <StatisticCard.Divider />
        <StatisticCard statistic={{ title: '办公区总数', value: officeCount, suffix: '个' }} />
        <StatisticCard.Divider />
        <StatisticCard statistic={{ title: '已归属设备', value: assignedCount, suffix: '台' }} />
        <StatisticCard.Divider />
        <StatisticCard
          statistic={{
            title: '未归属设备',
            value: unassignedCount,
            suffix: '台',
            valueStyle: unassignedCount > 0 ? { color: '#fa8c16' } : undefined,
          }}
        />
      </StatisticCard.Group>

      <ProTable<Site>
        actionRef={actionRef}
        rowKey="id"
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={false}
        toolBarRender={() => [
          <Button
            key="add"
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateOpen(true)}
          >
            新建机房 / 办公区
          </Button>,
        ]}
        request={async (params) => {
          const res = await listSites({
            keyword: params.keyword,
            type: params.type,
          });
          return { data: res.items || [], success: true };
        }}
      />

      <ModalForm
        title="新建机房 / 办公区"
        open={createOpen}
        onOpenChange={setCreateOpen}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (vals: any) => {
          await createSite({
            name: vals.name,
            code: vals.code,
            type: vals.type,
            address: vals.address,
            contact: vals.contact,
            cidrs: parseCidrs(vals.cidrsText),
            description: vals.description,
          });
          message.success('创建成功');
          await loadSummary();
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="name" label="名称" rules={[{ required: true }]} placeholder="例如 总部核心机房 / 北京研发办公区" />
        <ProFormSelect name="type" label="类型" initialValue="datacenter" rules={[{ required: true }]} options={siteTypeOptions} />
        <ProFormText name="code" label="编码（可选）" placeholder="例如 HQ-DC1 / BJ-OFF" />
        <ProFormText name="address" label="物理地址 / 楼层" placeholder="例如 A座3层核心机房" />
        <ProFormText name="contact" label="运维负责人 / 联系方式" placeholder="例如 张三 / 138xxxx" />
        <ProFormText name="cidrsText" label="关联网段 CIDR（空格或逗号分隔）" placeholder="例如 192.168.140.0/24, 10.10.102.0/24" />
        <ProFormTextArea name="description" label="备注说明" />
      </ModalForm>

      <ModalForm
        title={`编辑机房 / 办公区: ${editing?.name || ''}`}
        open={!!editing}
        onOpenChange={(o) => !o && setEditing(null)}
        modalProps={{ destroyOnHidden: true }}
        initialValues={
          editing
            ? {
                name: editing.name,
                code: editing.code,
                type: editing.type,
                address: editing.address,
                contact: editing.contact,
                cidrsText: (editing.cidrs || []).join(', '),
                description: editing.description,
              }
            : undefined
        }
        onFinish={async (vals: any) => {
          if (!editing) return false;
          await updateSite(editing.id, {
            name: vals.name,
            code: vals.code,
            type: vals.type,
            address: vals.address,
            contact: vals.contact,
            cidrs: parseCidrs(vals.cidrsText),
            description: vals.description,
          });
          message.success('已更新（若改名已同步更新名下设备归属）');
          setEditing(null);
          await loadSummary();
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="name" label="名称" rules={[{ required: true }]} />
        <ProFormSelect name="type" label="类型" rules={[{ required: true }]} options={siteTypeOptions} />
        <ProFormText name="code" label="编码" />
        <ProFormText name="address" label="物理地址 / 楼层" />
        <ProFormText name="contact" label="运维负责人 / 联系方式" />
        <ProFormText name="cidrsText" label="关联网段 CIDR（空格或逗号分隔）" />
        <ProFormTextArea name="description" label="备注说明" />
      </ModalForm>

      <Drawer
        title={managingSite ? `设备归属管理 · ${managingSite.name}` : '设备归属管理'}
        width={760}
        open={!!managingSite}
        onClose={() => {
          setManagingSite(null);
          setAddDeviceIds([]);
        }}
        extra={
          managingSite && (
            <Button
              type="primary"
              size="small"
              icon={<ApartmentOutlined />}
              onClick={() => history.push(`/topology?site=${encodeURIComponent(managingSite.name)}`)}
            >
              查看该区域拓扑
            </Button>
          )
        }
      >
        {managingSite && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Card size="small" title="批量划入设备">
              <Space style={{ width: '100%' }} direction="vertical">
                <Select
                  mode="multiple"
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  style={{ width: '100%' }}
                  placeholder="选择要划入当前机房/办公区的设备（支持按名称或 IP 搜索）"
                  value={addDeviceIds}
                  onChange={setAddDeviceIds}
                  options={candidateDevices.map((d) => ({
                    label: `${d.name || d.mgmtIp} (${d.mgmtIp || '未纳管'})${d.site ? ` [当前: ${d.site}]` : ' [未归属]'}`,
                    value: d.id,
                  }))}
                />
                <Button
                  type="primary"
                  loading={assigning}
                  disabled={addDeviceIds.length === 0}
                  onClick={handleAssignSelected}
                >
                  划入已选设备 ({addDeviceIds.length})
                </Button>
              </Space>
            </Card>

            <Card size="small" title={`当前区域设备 (${siteDevices.length} 台)`}>
              <Table
                rowKey="id"
                size="small"
                pagination={{ pageSize: 12 }}
                dataSource={siteDevices}
                columns={[
                  {
                    title: '设备名称',
                    dataIndex: 'name',
                    render: (_, d) => d.name || d.mgmtIp,
                  },
                  { title: '管理 IP', dataIndex: 'mgmtIp', width: 135, render: (v) => v || '-' },
                  {
                    title: '角色',
                    dataIndex: 'role',
                    width: 100,
                    render: (v: string) => {
                      const m = ROLE_META[v] || { label: v, color: 'default' };
                      return <Tag color={m.color}>{m.label}</Tag>;
                    },
                  },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    width: 85,
                    render: (v: string) => {
                      const m = STATUS_META[v] || { label: v, badge: 'default' as const };
                      return <Badge status={m.badge} text={m.label} />;
                    },
                  },
                  { title: '终端数', dataIndex: 'endpointCount', width: 75 },
                  {
                    title: '操作',
                    width: 75,
                    render: (_, d) => (
                      <Popconfirm
                        title="确认将该设备移出当前机房/办公区？"
                        onConfirm={() => handleRemoveDevice(d.id)}
                      >
                        <a style={{ color: '#ff4d4f' }}>移出</a>
                      </Popconfirm>
                    ),
                  },
                ]}
              />
            </Card>
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
};

export default SitesPage;
