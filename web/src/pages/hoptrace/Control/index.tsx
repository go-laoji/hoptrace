import {
  CheckOutlined,
  CloseOutlined,
  EyeOutlined,
  PlusOutlined,
  RollbackOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  ProCard,
  type ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Alert, Button, Descriptions, Drawer, Form, message, Popconfirm, Space, Table, Tabs, Tag } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import {
  approveControlAction,
  type ControlAction,
  createControlAction,
  deleteWriteCredential,
  type Device,
  type DeviceInterface,
  executeControlAction,
  listControlActions,
  listDevices,
  listInterfaces,
  listWriteCredentials,
  rejectControlAction,
  rollbackControlAction,
  saveWriteCredential,
  type WriteCredential,
} from '@/services/hoptrace';
import { fmtUnix } from '../constants';

const ACTION_META: Record<string, { label: string; color: string }> = {
  shutdown_port: { label: '关闭端口', color: 'volcano' },
  undo_shutdown_port: { label: '启用端口', color: 'green' },
  set_port_vlan: { label: '修改端口 VLAN', color: 'blue' },
  set_port_desc: { label: '修改端口描述', color: 'cyan' },
  block_endpoint: { label: '封禁终端(黑洞MAC)', color: 'red' },
  unblock_endpoint: { label: '解封终端', color: 'lime' },
};

const CTRL_STATUS_META: Record<string, { label: string; color: string }> = {
  pending_approval: { label: '待审批', color: 'gold' },
  approved: { label: '已审批待执行', color: 'processing' },
  rejected: { label: '已拒绝', color: 'default' },
  executed: { label: '已执行', color: 'success' },
  failed: { label: '执行失败', color: 'error' },
  rolled_back: { label: '已回滚', color: 'purple' },
};

const ControlPage: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType>(null);
  const [devices, setDevices] = useState<Device[]>([]);
  const [ifaces, setIfaces] = useState<DeviceInterface[]>([]);
  const [writeCreds, setWriteCreds] = useState<WriteCredential[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [credOpen, setCredOpen] = useState(false);
  const [detail, setDetail] = useState<ControlAction | null>(null);
  const [dryPreview, setDryPreview] = useState<ControlAction | null>(null);
  const [form] = Form.useForm();

  const loadAux = async () => {
    const [d, c] = await Promise.all([
      listDevices({ page: 1, pageSize: 200 }),
      listWriteCredentials(),
    ]);
    setDevices((d.items || []).filter((x) => x.managed));
    setWriteCreds(c.items || []);
  };

  useEffect(() => {
    loadAux().catch(() => {});
  }, []);

  const onDeviceChange = async (devId: string) => {
    form.setFieldValue('ifIndex', undefined);
    if (!devId) {
      setIfaces([]);
      return;
    }
    const r = await listInterfaces(devId);
    setIfaces(r.items || []);
  };

  const handleDryRun = async () => {
    const vals = await form.validateFields();
    const res = await createControlAction({ ...vals, dryRunOnly: true });
    setDryPreview(res);
  };

  const columns: ProColumns<ControlAction>[] = [
    { title: '工单号', dataIndex: 'id', search: false, width: 75 },
    {
      title: '管控动作',
      dataIndex: 'action',
      search: false,
      width: 150,
      render: (_, r) => {
        const m = ACTION_META[r.action] || { label: r.action, color: 'default' };
        return <Tag color={m.color}>{m.label}</Tag>;
      },
    },
    {
      title: '目标设备 / 端口 / 终端',
      dataIndex: 'deviceName',
      search: false,
      render: (_, r) =>
        [r.deviceName, r.ifName, r.endpointMac, r.paramValue && `参数:${r.paramValue}`]
          .filter(Boolean)
          .join(' · '),
    },
    {
      title: '通道',
      dataIndex: 'protocol',
      search: false,
      width: 85,
      render: (v) => <Tag>{String(v).toUpperCase()}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 115,
      valueEnum: Object.fromEntries(Object.entries(CTRL_STATUS_META).map(([k, v]) => [k, { text: v.label }])),
      render: (_, r) => {
        const m = CTRL_STATUS_META[r.status] || { label: r.status, color: 'default' };
        return <Tag color={m.color}>{m.label}</Tag>;
      },
    },
    { title: '申请人 / 审批人', dataIndex: 'requestedBy', search: false, width: 150, render: (_, r) => `${r.requestedBy || '-'} / ${r.approvedBy || '-'}` },
    { title: '创建时间', dataIndex: 'createdAt', search: false, width: 165, render: (_, r) => fmtUnix(r.createdAt) },
    {
      title: '操作',
      valueType: 'option',
      width: 220,
      render: (_, r) => [
        <a key="view" onClick={() => setDetail(r)}>
          <EyeOutlined /> 预览/日志
        </a>,
        access.isAdmin && r.status === 'pending_approval' && (
          <Popconfirm
            key="approve"
            title="审批通过并立即下发配置到设备？"
            onConfirm={async () => {
              await approveControlAction(r.id, true);
              message.success('已审批并执行下发');
              actionRef.current?.reload();
            }}
          >
            <a style={{ color: '#52c41a' }}>
              <CheckOutlined /> 审批执行
            </a>
          </Popconfirm>
        ),
        access.isAdmin && r.status === 'pending_approval' && (
          <Popconfirm
            key="reject"
            title="确认拒绝该管控工单？"
            onConfirm={async () => {
              await rejectControlAction(r.id, '管理员拒绝');
              message.info('已拒绝');
              actionRef.current?.reload();
            }}
          >
            <a style={{ color: '#ff4d4f' }}>
              <CloseOutlined /> 拒绝
            </a>
          </Popconfirm>
        ),
        access.isAdmin && r.status === 'approved' && (
          <a
            key="exec"
            onClick={async () => {
              await executeControlAction(r.id);
              message.success('执行完成');
              actionRef.current?.reload();
            }}
          >
            执行
          </a>
        ),
        access.isAdmin && r.status === 'executed' && (
          <Popconfirm
            key="rollback"
            title="确认下发回滚命令恢复原配置？"
            onConfirm={async () => {
              await rollbackControlAction(r.id);
              message.success('已回滚');
              actionRef.current?.reload();
            }}
          >
            <a style={{ color: '#722ed1' }}>
              <RollbackOutlined /> 一键回滚
            </a>
          </Popconfirm>
        ),
      ],
    },
  ];

  return (
    <PageContainer title="端口管控与审批">
      <Alert
        type="info"
        showIcon
        icon={<SafetyCertificateOutlined />}
        style={{ marginBottom: 16 }}
        message="安全护栏已启用：核心交换机、防火墙、上联口及 Eth-Trunk 聚合口受保护名单拦截；所有写操作均需 Dry-Run 预览、管理员审批并自动生成回滚命令。"
      />

      <Tabs
        items={[
          {
            key: 'actions',
            label: '管控工单与审批流',
            children: (
              <ProTable<ControlAction>
                actionRef={actionRef}
                rowKey="id"
                columns={columns}
                search={{ labelWidth: 'auto' }}
                toolBarRender={() =>
                  access.canWrite
                    ? [
                        <Button
                          key="new"
                          type="primary"
                          icon={<PlusOutlined />}
                          onClick={() => {
                            setDryPreview(null);
                            setCreateOpen(true);
                          }}
                        >
                          发起端口管控工单
                        </Button>,
                      ]
                    : []
                }
                request={async (params) => {
                  const res = await listControlActions({
                    status: params.status,
                    page: params.current,
                    pageSize: params.pageSize,
                  });
                  return { data: res.items || [], total: Number(res.total || 0), success: true };
                }}
              />
            ),
          },
          {
            key: 'creds',
            label: 'NETCONF / SSH 写凭据与 NAC 联动',
            children: (
              <ProCard
                extra={
                  access.isAdmin && (
                    <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => setCredOpen(true)}>
                      新增写通道凭据
                    </Button>
                  )
                }
              >
                <Table
                  rowKey="id"
                  size="small"
                  pagination={false}
                  dataSource={writeCreds}
                  columns={[
                    { title: '名称', dataIndex: 'name' },
                    {
                      title: '协议',
                      dataIndex: 'protocol',
                      width: 100,
                      render: (v) => <Tag color={v === 'netconf' ? 'purple' : 'blue'}>{String(v).toUpperCase()}</Tag>,
                    },
                    { title: '用户名', dataIndex: 'username' },
                    { title: '端口', dataIndex: 'port', width: 80 },
                    { title: 'NAC 准入 Webhook (iMaster NCE / RADIUS)', dataIndex: 'nacWebhookUrl', render: (v) => v || '-' },
                    {
                      title: '操作',
                      width: 90,
                      render: (_, r) =>
                        access.isAdmin ? (
                          <Popconfirm
                            title="确认删除该写凭据？"
                            onConfirm={async () => {
                              await deleteWriteCredential(r.id);
                              message.success('已删除');
                              loadAux();
                            }}
                          >
                            <a style={{ color: '#ff4d4f' }}>删除</a>
                          </Popconfirm>
                        ) : (
                          '-'
                        ),
                    },
                  ]}
                />
              </ProCard>
            ),
          },
        ]}
      />

      <ModalForm
        form={form}
        title="发起端口管控工单"
        open={createOpen}
        onOpenChange={setCreateOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={{ action: 'shutdown_port', protocol: 'netconf' }}
        submitter={{
          render: (_, dom) => [
            <Button key="dry" onClick={handleDryRun}>
              Dry-Run 预览命令
            </Button>,
            ...dom,
          ],
        }}
        onFinish={async (vals: any) => {
          await createControlAction(vals);
          message.success('管控工单已提交，等待管理员审批');
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormSelect
          name="action"
          label="管控动作"
          rules={[{ required: true }]}
          options={[
            { label: '关闭端口 (shutdown)', value: 'shutdown_port' },
            { label: '启用端口 (undo shutdown)', value: 'undo_shutdown_port' },
            { label: '修改接入口 VLAN', value: 'set_port_vlan' },
            { label: '回写端口描述', value: 'set_port_desc' },
          ]}
        />
        <ProFormSelect
          name="deviceId"
          label="目标交换机（仅已纳管设备）"
          rules={[{ required: true }]}
          options={devices.map((d) => ({ label: `${d.name} (${d.mgmtIp}) [${d.role}]`, value: d.id }))}
          fieldProps={{ onChange: onDeviceChange }}
        />
        <ProFormSelect
          name="ifIndex"
          label="目标端口（上联/聚合口受保护）"
          rules={[{ required: true }]}
          options={ifaces.map((it) => ({
            label: `${it.name} [${it.kind}] (${it.operStatus})`,
            value: it.ifIndex,
          }))}
        />
        <ProFormSelect
          name="protocol"
          label="下发通道"
          options={[
            { label: 'NETCONF (SSH 830，优先)', value: 'netconf' },
            { label: 'SSH CLI (兜底)', value: 'ssh' },
          ]}
        />
        <ProFormText name="paramValue" label="动作参数（如目标 VLAN ID 或端口描述）" />
        <ProFormTextArea name="reason" label="变更原因 / 工单备注" rules={[{ required: true }]} />

        {dryPreview && (
          <Alert
            type="warning"
            style={{ marginBottom: 12 }}
            message="Dry-Run 配置预览与回滚载荷"
            description={
              <Space direction="vertical" style={{ width: '100%' }}>
                <div>
                  <strong>下发配置：</strong>
                  <pre style={{ margin: '4px 0', background: '#141414', color: '#e6f4ff', padding: 8, borderRadius: 4, fontSize: 12, whiteSpace: 'pre-wrap' }}>
                    {dryPreview.dryRunPreview}
                  </pre>
                </div>
                <div>
                  <strong>对称回滚：</strong>
                  <pre style={{ margin: '4px 0', background: '#1f1f1f', color: '#b7eb8f', padding: 8, borderRadius: 4, fontSize: 12, whiteSpace: 'pre-wrap' }}>
                    {dryPreview.rollbackPreview}
                  </pre>
                </div>
              </Space>
            }
          />
        )}
      </ModalForm>

      <ModalForm
        title="新增 NETCONF / SSH 写通道凭据"
        open={credOpen}
        onOpenChange={setCredOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={{ protocol: 'netconf', port: 830 }}
        onFinish={async (vals: any) => {
          await saveWriteCredential(vals);
          message.success('写凭据已加密保存');
          loadAux();
          return true;
        }}
      >
        <ProFormText name="name" label="凭据名称" rules={[{ required: true }]} />
        <ProFormSelect
          name="protocol"
          label="协议"
          rules={[{ required: true }]}
          options={[
            { label: 'NETCONF (830)', value: 'netconf' },
            { label: 'SSH CLI (22)', value: 'ssh' },
          ]}
        />
        <ProFormDigit name="port" label="端口" fieldProps={{ precision: 0 }} />
        <ProFormText name="username" label="设备管理账号" rules={[{ required: true }]} />
        <ProFormText.Password name="password" label="密码（AES-GCM 加密存储）" rules={[{ required: true }]} />
        <ProFormText
          name="nacWebhookUrl"
          label="准入联动 Webhook（可选，如 iMaster NCE-Campus 回调地址）"
          placeholder="https://nce.example.local/api/v1/nac/hook"
        />
      </ModalForm>

      <Drawer
        title={detail ? `管控工单 #${detail.id}` : '工单详情'}
        width={680}
        open={!!detail}
        onClose={() => setDetail(null)}
      >
        {detail && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="动作">{ACTION_META[detail.action]?.label || detail.action}</Descriptions.Item>
              <Descriptions.Item label="状态">{CTRL_STATUS_META[detail.status]?.label || detail.status}</Descriptions.Item>
              <Descriptions.Item label="设备">{detail.deviceName}</Descriptions.Item>
              <Descriptions.Item label="端口 / 终端">{detail.ifName || detail.endpointMac || '-'}</Descriptions.Item>
              <Descriptions.Item label="申请人">{detail.requestedBy || '-'}</Descriptions.Item>
              <Descriptions.Item label="审批人">{detail.approvedBy || '-'}</Descriptions.Item>
              <Descriptions.Item label="执行时间">{fmtUnix(detail.executedAt)}</Descriptions.Item>
              <Descriptions.Item label="回滚时间">{fmtUnix(detail.rolledBackAt)}</Descriptions.Item>
              <Descriptions.Item label="变更原因" span={2}>
                {detail.reason || '-'}
              </Descriptions.Item>
            </Descriptions>

            <div>
              <h4>下发配置预览</h4>
              <pre style={{ background: '#141414', color: '#e6f4ff', padding: 10, borderRadius: 6, fontSize: 12, whiteSpace: 'pre-wrap' }}>
                {detail.dryRunPreview}
              </pre>
            </div>
            <div>
              <h4>回滚配置预览</h4>
              <pre style={{ background: '#141414', color: '#b7eb8f', padding: 10, borderRadius: 6, fontSize: 12, whiteSpace: 'pre-wrap' }}>
                {detail.rollbackPreview}
              </pre>
            </div>
            {detail.resultLog && (
              <div>
                <h4>执行与回滚审计日志</h4>
                <pre style={{ background: '#f5f5f5', padding: 10, borderRadius: 6, fontSize: 12, whiteSpace: 'pre-wrap' }}>
                  {detail.resultLog}
                </pre>
              </div>
            )}
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
};

export default ControlPage;
