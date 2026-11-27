import { BellOutlined, CheckOutlined, PlusOutlined, SendOutlined } from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  ProCard,
  type ProColumns,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { Button, message, Popconfirm, Space, Table, Tabs, Tag } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import {
  ackAlert,
  type AlertChannel,
  type AlertItem,
  deleteAlertChannel,
  listAlertChannels,
  listAlerts,
  listSyslogs,
  resolveAlert,
  saveAlertChannel,
  type SyslogEntry,
  testAlertChannel,
} from '@/services/hoptrace';
import { fmtUnix } from '../constants';

const SEV_COLOR: Record<string, string> = {
  critical: 'error',
  warning: 'warning',
  info: 'processing',
};

const KIND_LABEL: Record<string, string> = {
  device_offline: '设备离线',
  link_down: '链路中断',
  new_endpoint: '新终端接入',
  endpoint_moved: '终端漂移',
  ip_conflict: 'IP 冲突',
  suspect_hub: '疑似私接',
  syslog_security: 'Syslog 安全事件',
};

const CHANNEL_LABEL: Record<string, string> = {
  wecom: '企业微信',
  dingtalk: '钉钉',
  feishu: '飞书',
  webhook: '通用 Webhook',
};

const AlertsPage: React.FC = () => {
  const alertRef = useRef<ActionType>(null);
  const [channels, setChannels] = useState<AlertChannel[]>([]);
  const [chOpen, setChOpen] = useState(false);

  const loadChannels = async () => {
    const res = await listAlertChannels();
    setChannels(res.items || []);
  };

  useEffect(() => {
    loadChannels().catch(() => {});
  }, []);

  const alertCols: ProColumns<AlertItem>[] = [
    {
      title: '级别',
      dataIndex: 'severity',
      width: 90,
      valueEnum: { critical: { text: '严重' }, warning: { text: '告警' }, info: { text: '提示' } },
      render: (_, r) => <Tag color={SEV_COLOR[r.severity] || 'default'}>{r.severity.toUpperCase()}</Tag>,
    },
    {
      title: '类型',
      dataIndex: 'kind',
      width: 135,
      valueEnum: Object.fromEntries(Object.entries(KIND_LABEL).map(([k, v]) => [k, { text: v }])),
      render: (_, r) => <Tag>{KIND_LABEL[r.kind] || r.kind}</Tag>,
    },
    { title: '标题', dataIndex: 'title', search: false, width: 240 },
    { title: '详情', dataIndex: 'message', search: false, ellipsis: true },
    {
      title: '状态',
      dataIndex: 'status',
      width: 95,
      valueEnum: { open: { text: '未处理' }, acked: { text: '已确认' }, resolved: { text: '已解决' } },
      render: (_, r) => (
        <Tag color={r.status === 'open' ? 'red' : r.status === 'acked' ? 'orange' : 'green'}>
          {r.status === 'open' ? '未处理' : r.status === 'acked' ? '已确认' : '已解决'}
        </Tag>
      ),
    },
    { title: '时间', dataIndex: 'updatedAt', search: false, width: 165, render: (_, r) => fmtUnix(r.updatedAt) },
    {
      title: '操作',
      valueType: 'option',
      width: 130,
      render: (_, r) => [
        r.status === 'open' && (
          <a
            key="ack"
            onClick={async () => {
              await ackAlert(r.id);
              message.success('已确认告警');
              alertRef.current?.reload();
            }}
          >
            确认
          </a>
        ),
        r.status !== 'resolved' && (
          <a
            key="resolve"
            style={{ color: '#52c41a' }}
            onClick={async () => {
              await resolveAlert(r.id);
              message.success('已标记解决');
              alertRef.current?.reload();
            }}
          >
            <CheckOutlined /> 解决
          </a>
        ),
      ],
    },
  ];

  const syslogCols: ProColumns<SyslogEntry>[] = [
    { title: '关键词', dataIndex: 'keyword', hideInTable: true, fieldProps: { placeholder: '消息 / 标签 / 设备名 / IP' } },
    {
      title: '级别',
      dataIndex: 'severity',
      search: false,
      width: 80,
      render: (v) => <Tag color={Number(v) <= 3 ? 'red' : Number(v) === 4 ? 'orange' : 'blue'}>Sev {String(v)}</Tag>,
    },
    { title: '来源设备', dataIndex: 'deviceName', search: false, width: 160, render: (_, r) => r.deviceName || r.sourceIp },
    { title: '标签', dataIndex: 'tag', search: false, width: 200, ellipsis: true, render: (v) => v || '-' },
    { title: '日志内容', dataIndex: 'message', search: false, ellipsis: true },
    { title: '接收时间', dataIndex: 'receivedAt', search: false, width: 165, render: (_, r) => fmtUnix(r.receivedAt) },
  ];

  return (
    <PageContainer title="告警中心与设备日志">
      <Tabs
        items={[
          {
            key: 'alerts',
            label: (
              <Space>
                <BellOutlined />
                实时告警
              </Space>
            ),
            children: (
              <ProTable<AlertItem>
                actionRef={alertRef}
                rowKey="id"
                columns={alertCols}
                search={{ labelWidth: 'auto' }}
                request={async (params) => {
                  const res = await listAlerts({
                    status: params.status,
                    severity: params.severity,
                    kind: params.kind,
                    page: params.current,
                    pageSize: params.pageSize,
                  });
                  return { data: res.items || [], total: Number(res.total || 0), success: true };
                }}
              />
            ),
          },
          {
            key: 'channels',
            label: '通知渠道（企微 / 钉钉 / 飞书 / Webhook）',
            children: (
              <ProCard
                extra={
                  <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => setChOpen(true)}>
                    新增通知渠道
                  </Button>
                }
              >
                <Table
                  rowKey="id"
                  size="small"
                  pagination={false}
                  dataSource={channels}
                  columns={[
                    { title: '名称', dataIndex: 'name', width: 160 },
                    {
                      title: '平台',
                      dataIndex: 'kind',
                      width: 120,
                      render: (v: string) => <Tag color="blue">{CHANNEL_LABEL[v] || v}</Tag>,
                    },
                    { title: 'Webhook URL', dataIndex: 'webhookUrl', ellipsis: true },
                    {
                      title: '状态',
                      dataIndex: 'enabled',
                      width: 85,
                      render: (v: boolean) => <Tag color={v ? 'success' : 'default'}>{v ? '启用' : '停用'}</Tag>,
                    },
                    {
                      title: '操作',
                      width: 140,
                      render: (_, r) => (
                        <Space>
                          <a
                            onClick={async () => {
                              const res = await testAlertChannel(r.id);
                              if (res.ok) message.success('测试推送成功: ' + res.detail);
                              else message.error('测试推送失败: ' + res.detail);
                            }}
                          >
                            <SendOutlined /> 测试
                          </a>
                          <Popconfirm
                            title="确认删除该通知渠道？"
                            onConfirm={async () => {
                              await deleteAlertChannel(r.id);
                              message.success('已删除');
                              loadChannels();
                            }}
                          >
                            <a style={{ color: '#ff4d4f' }}>删除</a>
                          </Popconfirm>
                        </Space>
                      ),
                    },
                  ]}
                />
              </ProCard>
            ),
          },
          {
            key: 'syslogs',
            label: '设备 Syslog 日志（UDP 1514）',
            children: (
              <ProTable<SyslogEntry>
                rowKey="id"
                columns={syslogCols}
                search={{ labelWidth: 'auto' }}
                request={async (params) => {
                  const res = await listSyslogs({
                    keyword: params.keyword,
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
        title="新增告警通知渠道"
        open={chOpen}
        onOpenChange={setChOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={{ kind: 'wecom', enabled: true }}
        onFinish={async (vals: any) => {
          await saveAlertChannel(vals);
          message.success('通知渠道已保存');
          loadChannels();
          return true;
        }}
      >
        <ProFormText name="name" label="渠道名称" rules={[{ required: true }]} placeholder="例如 运维企业微信群" />
        <ProFormSelect
          name="kind"
          label="平台类型"
          rules={[{ required: true }]}
          options={Object.entries(CHANNEL_LABEL).map(([k, v]) => ({ label: v, value: k }))}
        />
        <ProFormText
          name="webhookUrl"
          label="Webhook 地址"
          rules={[{ required: true }]}
          placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."
        />
        <ProFormSelect
          name="kinds"
          label="订阅告警类型（留空订阅全部）"
          mode="multiple"
          options={Object.entries(KIND_LABEL).map(([k, v]) => ({ label: v, value: k }))}
        />
        <ProFormSwitch name="enabled" label="立即启用" />
      </ModalForm>
    </PageContainer>
  );
};

export default AlertsPage;
