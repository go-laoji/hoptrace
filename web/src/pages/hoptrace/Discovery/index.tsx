import { PlayCircleOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import {
  ModalForm,
  PageContainer,
  ProCard,
  ProFormDependency,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import { Button, Col, Form, Input, message, Modal, Popconfirm, Progress, Row, Select, Space, Switch, Table, Tag } from 'antd';
import React, { useEffect, useState } from 'react';
import {
  type Credential,
  createCredential,
  deleteCredential,
  type DiscoveryJob,
  listCredentials,
  listDiscoveryJobs,
  listSites,
  type Site,
  startDiscovery,
  updateCredential,
} from '@/services/hoptrace';
import { fmtUnix } from '../constants';

const DiscoveryPage: React.FC = () => {
  const [creds, setCreds] = useState<Credential[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [jobs, setJobs] = useState<DiscoveryJob[]>([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [credModal, setCredModal] = useState(false);
  const [editingCred, setEditingCred] = useState<Credential | null>(null);
  const [logJob, setLogJob] = useState<DiscoveryJob | null>(null);
  const [form] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try {
      const [c, j, s] = await Promise.all([listCredentials(), listDiscoveryJobs(20), listSites()]);
      setCreds(c.items || []);
      setJobs(j.items || []);
      setSites(s.items || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  // 存在运行中的任务时每 2 秒轮询进度
  useEffect(() => {
    if (!jobs.some((j) => j.status === 'running')) return;
    const t = setInterval(() => {
      listDiscoveryJobs(20).then((r) => setJobs(r.items || [])).catch(() => {});
    }, 2000);
    return () => clearInterval(t);
  }, [jobs]);

  const handleStart = async (vals: any) => {
    const seeds = String(vals.seeds || '')
      .split(/[\s,;]+/)
      .map((s) => s.trim())
      .filter(Boolean);
    const cidrs = String(vals.cidrs || '')
      .split(/[\s,;]+/)
      .map((s) => s.trim())
      .filter(Boolean);
    if (seeds.length === 0 && cidrs.length === 0) {
      message.warning('请至少填写一个种子设备 IP 或网段 CIDR');
      return;
    }
    setSubmitting(true);
    try {
      await startDiscovery({
        seeds,
        cidrs,
        credentialIds: vals.credentialIds || [],
        expandNeighbors: vals.expandNeighbors !== false,
        site: vals.site || '',
      });
      message.success('发现任务已启动');
      load();
    } finally {
      setSubmitting(false);
    }
  };

  const fillDemo = async () => {
    let list = creds;
    if (list.length === 0) {
      const c = await createCredential({ name: 'huawei-demo-v2c', version: 'v2c', community: 'public', port: 161 });
      list = [c];
      setCreds(list);
      message.success('已自动创建演示 SNMP v2c 凭据');
    }
    form.setFieldsValue({
      seeds: '10.255.0.1',
      cidrs: '',
      credentialIds: [list[0].id],
      expandNeighbors: true,
      site: '总部园区',
    });
  };

  return (
    <PageContainer
      title="自动发现与 SNMP 凭据"
      extra={[
        <Button key="demo" onClick={fillDemo}>
          一键填入模拟网络种子 (10.255.0.1)
        </Button>,
        <Button key="refresh" icon={<ReloadOutlined />} loading={loading} onClick={load}>
          刷新
        </Button>,
      ]}
    >
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={10}>
          <ProCard title="发起网络自动发现">
            <Form
              form={form}
              layout="vertical"
              initialValues={{ expandNeighbors: true }}
              onFinish={handleStart}
            >
              <Form.Item name="seeds" label="种子设备 IP（空格或逗号分隔）">
                <Input placeholder="例如 10.255.0.1" />
              </Form.Item>
              <Form.Item name="cidrs" label="待扫描网段 CIDR（可选）">
                <Input placeholder="例如 10.255.0.0/24" />
              </Form.Item>
              <Form.Item name="credentialIds" label="尝试凭据（留空使用全部凭据）">
                <Select
                  mode="multiple"
                  allowClear
                  placeholder="全部凭据"
                  options={creds.map((c) => ({ label: `${c.name} (${c.version})`, value: c.id }))}
                />
              </Form.Item>
              <Form.Item name="site" label="归属机房 / 办公区（可选）">
                <Select
                  allowClear
                  showSearch
                  placeholder="选择已配置的机房或办公区"
                  options={sites.map((s) => ({ label: s.name, value: s.name }))}
                  onChange={(val) => {
                    const hit = sites.find((s) => s.name === val);
                    if (hit && (hit.cidrs || []).length > 0 && !form.getFieldValue('cidrs')) {
                      form.setFieldValue('cidrs', hit.cidrs.join(', '));
                    }
                  }}
                />
              </Form.Item>
              <Form.Item name="expandNeighbors" label="通过 LLDP 邻居递归扩散" valuePropName="checked">
                <Switch />
              </Form.Item>
              <Button type="primary" htmlType="submit" icon={<PlayCircleOutlined />} loading={submitting} block>
                启动发现任务
              </Button>
            </Form>
          </ProCard>
        </Col>

        <Col xs={24} lg={14}>
          <ProCard
            title="SNMP 凭据库（AES-GCM 加密存储）"
            extra={
              <Button
                type="primary"
                size="small"
                icon={<PlusOutlined />}
                onClick={() => {
                  setEditingCred(null);
                  setCredModal(true);
                }}
              >
                新建凭据
              </Button>
            }
          >
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={creds}
              columns={[
                { title: '名称', dataIndex: 'name' },
                {
                  title: '版本',
                  dataIndex: 'version',
                  width: 75,
                  render: (v) => <Tag color={v === 'v3' ? 'green' : 'blue'}>{v}</Tag>,
                },
                {
                  title: '认证信息',
                  render: (_, r) =>
                    r.version === 'v3'
                      ? `${r.username} (${r.authProtocol || 'NoAuth'}/${r.privProtocol || 'NoPriv'})`
                      : r.hasCommunity
                        ? '团体字已加密存储'
                        : '-',
                },
                { title: '端口', dataIndex: 'port', width: 70 },
                {
                  title: '操作',
                  width: 110,
                  render: (_, r) => (
                    <Space>
                      <a
                        onClick={() => {
                          setEditingCred(r);
                          setCredModal(true);
                        }}
                      >
                        编辑
                      </a>
                      <Popconfirm
                        title="确认删除该凭据？"
                        onConfirm={async () => {
                          await deleteCredential(r.id);
                          message.success('已删除');
                          load();
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
        </Col>
      </Row>

      <ProCard title="发现任务历史" style={{ marginTop: 16 }}>
        <Table
          rowKey="id"
          size="small"
          pagination={{ pageSize: 10 }}
          dataSource={jobs}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 65 },
            {
              title: '状态',
              dataIndex: 'status',
              width: 95,
              render: (v: string) => (
                <Tag color={v === 'done' ? 'success' : v === 'running' ? 'processing' : 'error'}>
                  {v === 'done' ? '已完成' : v === 'running' ? '运行中' : '失败'}
                </Tag>
              ),
            },
            {
              title: '目标',
              render: (_, r) => [...(r.seeds || []), ...(r.cidrs || [])].join(', '),
            },
            {
              title: '进度',
              width: 180,
              render: (_, r) => (
                <Progress
                  percent={r.total > 0 ? Math.min(100, Math.round((r.probed / r.total) * 100)) : 0}
                  size="small"
                  format={() => `${r.probed}/${r.total}`}
                />
              ),
            },
            { title: '发现设备', dataIndex: 'found', width: 90 },
            { title: '开始时间', dataIndex: 'startedAt', width: 170, render: fmtUnix },
            {
              title: '日志',
              width: 80,
              render: (_, r) => <a onClick={() => setLogJob(r)}>查看</a>,
            },
          ]}
        />
      </ProCard>

      <ModalForm
        title={editingCred ? `编辑凭据: ${editingCred.name}` : '新建 SNMP 凭据'}
        open={credModal}
        onOpenChange={setCredModal}
        modalProps={{ destroyOnHidden: true }}
        initialValues={
          editingCred
            ? {
                name: editingCred.name,
                version: editingCred.version,
                username: editingCred.username,
                authProtocol: editingCred.authProtocol || 'SHA256',
                privProtocol: editingCred.privProtocol || 'AES',
                port: editingCred.port || 161,
              }
            : { version: 'v2c', port: 161, authProtocol: 'SHA256', privProtocol: 'AES' }
        }
        onFinish={async (vals) => {
          if (editingCred) {
            await updateCredential(editingCred.id, vals);
          } else {
            await createCredential(vals);
          }
          message.success('凭据已保存');
          load();
          return true;
        }}
      >
        <ProFormText name="name" label="凭据名称" rules={[{ required: true }]} />
        <ProFormSelect
          name="version"
          label="SNMP 版本"
          rules={[{ required: true }]}
          options={[
            { label: 'SNMP v2c', value: 'v2c' },
            { label: 'SNMP v3（推荐，华为新版默认）', value: 'v3' },
          ]}
        />
        <ProFormDigit name="port" label="端口" fieldProps={{ precision: 0 }} />
        <ProFormDependency name={['version']}>
          {({ version }) =>
            version === 'v3' ? (
              <>
                <ProFormText name="username" label="用户名 (USM User)" rules={[{ required: true }]} />
                <ProFormSelect
                  name="authProtocol"
                  label="认证算法"
                  options={['MD5', 'SHA', 'SHA224', 'SHA256', 'SHA384', 'SHA512']}
                />
                <ProFormText.Password
                  name="authPassword"
                  label={editingCred ? '认证密码（留空不修改）' : '认证密码'}
                />
                <ProFormSelect
                  name="privProtocol"
                  label="加密算法"
                  options={['DES', 'AES', 'AES192', 'AES256']}
                />
                <ProFormText.Password
                  name="privPassword"
                  label={editingCred ? '加密密码（留空不修改）' : '加密密码'}
                />
              </>
            ) : (
              <ProFormText.Password
                name="community"
                label={editingCred ? '只读团体字 Community（留空不修改）' : '只读团体字 Community'}
                rules={editingCred ? [] : [{ required: true }]}
              />
            )
          }
        </ProFormDependency>
      </ModalForm>

      <Modal
        title={logJob ? `发现任务 #${logJob.id} 日志` : '任务日志'}
        open={!!logJob}
        footer={null}
        width={720}
        onCancel={() => setLogJob(null)}
      >
        <pre
          style={{
            background: '#141414',
            color: '#e6f4ff',
            padding: 12,
            borderRadius: 6,
            maxHeight: 420,
            overflow: 'auto',
            fontSize: 12,
          }}
        >
          {logJob?.log || '暂无日志'}
        </pre>
      </Modal>
    </PageContainer>
  );
};

export default DiscoveryPage;
