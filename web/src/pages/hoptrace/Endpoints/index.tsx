import { DownloadOutlined, UploadOutlined, WifiOutlined } from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Badge, Button, Descriptions, Drawer, Input, message, Modal, Popconfirm, Space, Table, Tabs, Tag, Timeline } from 'antd';
import React, { useRef, useState } from 'react';
import {
  batchImportEndpoints,
  createControlAction,
  deleteEndpoint,
  type Endpoint,
  type EndpointDetail,
  type EndpointEvent,
  getEndpointDetail,
  listEndpoints,
  listEvents,
  updateEndpoint,
} from '@/services/hoptrace';
import { downloadCsv, ENDPOINT_TYPE_META, EVENT_META, fmtUnix, STATUS_META, TRUST_META } from '../constants';

const EndpointsPage: React.FC = () => {
  const actionRef = useRef<ActionType>(null);
  const [detail, setDetail] = useState<EndpointDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [editing, setEditing] = useState<Endpoint | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [importCsv, setImportCsv] = useState('');

  const openDetail = async (id: string) => {
    setDetailLoading(true);
    try {
      setDetail(await getEndpointDetail(id));
    } finally {
      setDetailLoading(false);
    }
  };

  const columns: ProColumns<Endpoint>[] = [
    {
      title: '检索',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: 'MAC / IP / 名称 / 厂商 / 责任人' },
    },
    {
      title: 'MAC 地址',
      dataIndex: 'mac',
      search: false,
      width: 155,
      render: (_, r) => <a onClick={() => openDetail(r.id)}>{r.mac}</a>,
    },
    { title: 'IP 地址', dataIndex: 'ip', search: false, width: 130, render: (v) => v || '-' },
    {
      title: '状态',
      dataIndex: 'status',
      width: 85,
      valueEnum: { online: { text: '在线' }, offline: { text: '离线' } },
      render: (_, r) => {
        const m = STATUS_META[r.status] || { label: r.status, badge: 'default' as const };
        return <Badge status={m.badge} text={m.label} />;
      },
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 115,
      valueEnum: Object.fromEntries(Object.entries(ENDPOINT_TYPE_META).map(([k, v]) => [k, { text: v.label }])),
      render: (_, r) => {
        const m = ENDPOINT_TYPE_META[r.type] || { label: r.type, color: 'default' };
        return <Tag color={m.color}>{m.label}</Tag>;
      },
    },
    {
      title: '信任状态',
      dataIndex: 'trust',
      width: 105,
      valueEnum: Object.fromEntries(Object.entries(TRUST_META).map(([k, v]) => [k, { text: v.label }])),
      render: (_, r) => {
        const m = TRUST_META[r.trust] || { label: r.trust, color: 'default' };
        return <Tag color={m.color}>{m.label}</Tag>;
      },
    },
    {
      title: '接入方式',
      dataIndex: 'accessType',
      width: 95,
      valueEnum: { wired: { text: '有线' }, wireless: { text: '无线 Wi-Fi' } },
      render: (_, r) =>
        r.accessType === 'wireless' ? (
          <Tag color="cyan" icon={<WifiOutlined />}>
            无线
          </Tag>
        ) : (
          <Tag>有线</Tag>
        ),
    },
    {
      title: '接入位置',
      dataIndex: 'deviceName',
      search: false,
      render: (_, r) =>
        r.accessType === 'wireless'
          ? `${r.apName || r.deviceName || '-'} · SSID:${r.ssid || '-'} (${r.rssi || 0} dBm)`
          : `${r.deviceName || '-'} / ${r.ifName || '-'} (VLAN ${r.vlan})`,
    },
    { title: 'OUI 厂商', dataIndex: 'vendor', search: false, ellipsis: true, width: 180, render: (v) => v || '-' },
    { title: '名称 / 责任人', dataIndex: 'name', search: false, width: 140, render: (_, r) => [r.name, r.owner].filter(Boolean).join(' / ') || '-' },
    {
      title: '操作',
      valueType: 'option',
      width: 175,
      render: (_, r) => [
        <a key="detail" onClick={() => openDetail(r.id)}>
          轨迹
        </a>,
        <a key="edit" onClick={() => setEditing(r)}>
          编辑
        </a>,
        <Popconfirm
          key="block"
          title={`为终端 ${r.mac} 发起黑洞 MAC 封禁审批单？`}
          onConfirm={async () => {
            await createControlAction({
              action: 'block_endpoint',
              endpointId: r.id,
              protocol: 'ssh',
              reason: `从终端台账发起封禁 (${r.ip || r.mac})`,
            });
            message.success('已提交封禁审批单，请前往「端口管控与审批」执行');
          }}
        >
          <a style={{ color: '#fa8c16' }}>封禁</a>
        </Popconfirm>,
        <Popconfirm
          key="del"
          title="确认删除该终端记录？"
          onConfirm={async () => {
            await deleteEndpoint(r.id);
            message.success('已删除');
            actionRef.current?.reload();
          }}
        >
          <a style={{ color: '#ff4d4f' }}>删除</a>
        </Popconfirm>,
      ],
    },
  ];

  const eventColumns: ProColumns<EndpointEvent>[] = [
    {
      title: '事件类型',
      dataIndex: 'type',
      width: 120,
      valueEnum: Object.fromEntries(Object.entries(EVENT_META).map(([k, v]) => [k, { text: v.label }])),
      render: (_, r) => {
        const m = EVENT_META[r.type] || { label: r.type, color: 'default' };
        return <Tag color={m.color}>{m.label}</Tag>;
      },
    },
    { title: 'MAC', dataIndex: 'mac', search: false, width: 160 },
    { title: 'IP', dataIndex: 'ip', search: false, width: 140, render: (v) => v || '-' },
    { title: '名称', dataIndex: 'name', search: false, width: 150, render: (v) => v || '-' },
    { title: '详情', dataIndex: 'detail', search: false },
    { title: '时间', dataIndex: 'at', search: false, width: 175, render: (_, r) => fmtUnix(r.at) },
  ];

  return (
    <PageContainer title="接入终端管理">
      <Tabs
        items={[
          {
            key: 'list',
            label: '终端台账与定位',
            children: (
              <ProTable<Endpoint>
                actionRef={actionRef}
                rowKey="id"
                columns={columns}
                search={{ labelWidth: 'auto' }}
                toolBarRender={() => [
                  <Button
                    key="import"
                    icon={<UploadOutlined />}
                    onClick={() => setImportOpen(true)}
                  >
                    批量导入归属 CSV
                  </Button>,
                  <Button
                    key="export"
                    icon={<DownloadOutlined />}
                    onClick={async () => {
                      const res = await listEndpoints({ page: 1, pageSize: 500 });
                      downloadCsv(
                        `hoptrace-endpoints-${Date.now()}.csv`,
                        ['MAC', 'IP', '状态', '接入方式', '类型', '信任状态', '名称', '责任人', '接入设备', '端口/AP', 'VLAN', '厂商'],
                        (res.items || []).map((e) => [
                          e.mac, e.ip, e.status, e.accessType || 'wired', e.type, e.trust,
                          e.name, e.owner, e.deviceName, e.apName || e.ifName, e.vlan, e.vendor,
                        ]),
                      );
                    }}
                  >
                    导出 CSV
                  </Button>,
                ]}
                request={async (params) => {
                  const res = await listEndpoints({
                    keyword: params.keyword,
                    status: params.status,
                    trust: params.trust,
                    type: params.type,
                    accessType: params.accessType,
                    page: params.current,
                    pageSize: params.pageSize,
                  });
                  return { data: res.items || [], total: Number(res.total || 0), success: true };
                }}
              />
            ),
          },
          {
            key: 'events',
            label: '上线 / 漂移事件审计',
            children: (
              <ProTable<EndpointEvent>
                rowKey="id"
                columns={eventColumns}
                search={{ labelWidth: 'auto' }}
                request={async (params) => {
                  const res = await listEvents({
                    type: params.type,
                    page: params.current,
                    pageSize: params.pageSize,
                  });
                  return { data: res.items || [], total: Number(res.total || 0), success: true };
                }}
              />
            ),
          },
        ]}
      />

      <ModalForm
        title={`编辑终端台账: ${editing?.mac || ''}`}
        open={!!editing}
        onOpenChange={(o) => !o && setEditing(null)}
        modalProps={{ destroyOnHidden: true }}
        initialValues={
          editing
            ? {
                name: editing.name,
                owner: editing.owner,
                trust: editing.trust,
                type: editing.typeManual ? editing.type : undefined,
                tags: editing.tags || [],
                remark: editing.remark,
              }
            : undefined
        }
        onFinish={async (vals) => {
          if (!editing) return false;
          await updateEndpoint(editing.id, vals);
          message.success('台账已保存');
          setEditing(null);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="name" label="终端名称 / 主机名" />
        <ProFormText name="owner" label="责任人 / 部门" />
        <ProFormSelect
          name="trust"
          label="信任状态"
          options={Object.entries(TRUST_META).map(([k, v]) => ({ label: v.label, value: k }))}
        />
        <ProFormSelect
          name="type"
          label="终端类型（留空自动识别）"
          allowClear
          options={Object.entries(ENDPOINT_TYPE_META).map(([k, v]) => ({ label: v.label, value: k }))}
        />
        <ProFormSelect name="tags" label="标签" mode="tags" placeholder="输入后回车添加标签" />
        <ProFormTextArea name="remark" label="备注" />
      </ModalForm>

      <Drawer
        title={detail ? `终端详情 · ${detail.endpoint.mac}` : '终端详情'}
        width={760}
        open={!!detail}
        loading={detailLoading}
        onClose={() => setDetail(null)}
      >
        {detail && (
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="MAC">{detail.endpoint.mac}</Descriptions.Item>
              <Descriptions.Item label="IP">{(detail.endpoint.ips || []).join(', ') || detail.endpoint.ip || '-'}</Descriptions.Item>
              <Descriptions.Item label="OUI 厂商" span={2}>
                {detail.endpoint.vendor || '未知'}
              </Descriptions.Item>
              <Descriptions.Item label="当前位置" span={2}>
                {`${detail.endpoint.deviceName} / ${detail.endpoint.ifName} (VLAN ${detail.endpoint.vlan})`}
              </Descriptions.Item>
              <Descriptions.Item label="首次发现">{fmtUnix(detail.endpoint.firstSeen)}</Descriptions.Item>
              <Descriptions.Item label="最近在线">{fmtUnix(detail.endpoint.lastSeen)}</Descriptions.Item>
            </Descriptions>

            <div>
              <h4>位置变更历史</h4>
              <Table
                rowKey="id"
                size="small"
                pagination={false}
                dataSource={detail.locations || []}
                columns={[
                  { title: '接入设备', dataIndex: 'deviceName' },
                  { title: '端口', dataIndex: 'ifName' },
                  { title: 'VLAN', dataIndex: 'vlan', width: 70 },
                  { title: '开始时间', dataIndex: 'since', render: fmtUnix },
                  {
                    title: '结束时间',
                    dataIndex: 'until',
                    render: (v) => (Number(v || 0) ? fmtUnix(v) : <Tag color="green">当前</Tag>),
                  },
                ]}
              />
            </div>

            <div>
              <h4>事件时间线</h4>
              <Timeline
                items={(detail.events || []).map((ev) => {
                  const m = EVENT_META[ev.type] || { label: ev.type, color: 'default' };
                  return {
                    children: (
                      <>
                        <Tag color={m.color}>{m.label}</Tag>
                        <span>{ev.detail}</span>
                        <div style={{ fontSize: 12, color: '#8c8c8c' }}>{fmtUnix(ev.at)}</div>
                      </>
                    ),
                  };
                })}
              />
            </div>
          </Space>
        )}
      </Drawer>

      <Modal
        title="批量导入终端归属信息 (CSV)"
        open={importOpen}
        width={640}
        onCancel={() => setImportOpen(false)}
        onOk={async () => {
          const lines = importCsv
            .split(/\r?\n/)
            .map((l) => l.trim())
            .filter(Boolean);
          const rows = lines
            .filter((l, i) => !(i === 0 && l.toLowerCase().startsWith('mac')))
            .map((l) => {
              const [mac, name, owner, trust, tags, remark] = l.split(',').map((x) => (x || '').trim());
              return {
                mac,
                name,
                owner,
                trust,
                tags: tags ? tags.split(';').filter(Boolean) : [],
                remark,
              };
            });
          if (rows.length === 0) {
            message.warning('请输入至少一行 CSV 数据');
            return;
          }
          const res = await batchImportEndpoints(rows);
          message.success(`导入完成：更新 ${res.updated} 条，跳过 ${res.skipped} 条`);
          setImportOpen(false);
          setImportCsv('');
          actionRef.current?.reload();
        }}
      >
        <div style={{ marginBottom: 8, fontSize: 12, color: '#595959' }}>
          每行格式：<code>mac,名称,责任人,信任状态(trusted/unknown/blocked),标签(分号分隔),备注</code>
        </div>
        <Input.TextArea
          rows={8}
          value={importCsv}
          onChange={(e) => setImportCsv(e.target.value)}
          placeholder={'mac,name,owner,trust,tags,remark\nf8:bc:12:11:22:01,张三笔记本,张三,trusted,研发部;办公,3楼工位'}
        />
      </Modal>
    </PageContainer>
  );
};

export default EndpointsPage;
