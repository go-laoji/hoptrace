import { CloudDownloadOutlined, DiffOutlined, EyeOutlined, ReloadOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Button, Drawer, message, Select, Space, Table, Tag } from 'antd';
import React, { useEffect, useState } from 'react';
import {
  type ConfigBackup,
  type Device,
  diffBackups,
  type DiffLine,
  getBackupDetail,
  listBackups,
  listDevices,
  runBackup,
} from '@/services/hoptrace';
import { fmtUnix } from '../constants';

const BackupsPage: React.FC = () => {
  const [devices, setDevices] = useState<Device[]>([]);
  const [deviceId, setDeviceId] = useState<string | undefined>(undefined);
  const [backups, setBackups] = useState<ConfigBackup[]>([]);
  const [loading, setLoading] = useState(false);
  const [running, setRunning] = useState(false);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [viewContent, setViewContent] = useState<{ title: string; content: string } | null>(null);
  const [diffView, setDiffView] = useState<{ title: string; added: number; removed: number; lines: DiffLine[] } | null>(null);

  const load = async (devId = deviceId) => {
    setLoading(true);
    try {
      const [d, b] = await Promise.all([
        listDevices({ page: 1, pageSize: 200 }),
        listBackups({ deviceId: devId, page: 1, pageSize: 50 }),
      ]);
      setDevices((d.items || []).filter((x) => x.managed));
      setBackups(b.items || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const handleRun = async () => {
    if (!deviceId) {
      message.warning('请先在下拉框中选择要备份的设备');
      return;
    }
    setRunning(true);
    try {
      const b = await runBackup(deviceId);
      if (b.status === 'ok') {
        message.success(`备份成功 (${b.sizeBytes} 字节${b.changed ? '，检测到配置变更' : '，与上次一致'})`);
      } else {
        message.error(`备份失败: ${b.error}`);
      }
      load(deviceId);
    } finally {
      setRunning(false);
    }
  };

  const handleDiff = async () => {
    if (selectedIds.length !== 2) {
      message.warning('请勾选恰好 2 条成功的备份记录进行 Diff 对比');
      return;
    }
    const [a, b] = selectedIds.sort((x, y) => Number(x) - Number(y));
    const res = await diffBackups(a, b);
    setDiffView({
      title: `配置变更对比: #${a} → #${b}`,
      added: res.added,
      removed: res.removed,
      lines: res.lines || [],
    });
  };

  return (
    <PageContainer
      title="设备配置备份与变更 Diff"
      extra={[
        <Select
          key="dev"
          allowClear
          placeholder="选择设备筛选/备份"
          style={{ width: 240 }}
          value={deviceId}
          onChange={(v) => {
            setDeviceId(v);
            setSelectedIds([]);
            load(v);
          }}
          options={devices.map((d) => ({ label: `${d.name} (${d.mgmtIp})`, value: d.id }))}
        />,
        <Button key="run" type="primary" icon={<CloudDownloadOutlined />} loading={running} onClick={handleRun}>
          立即备份选中设备
        </Button>,
        <Button key="diff" icon={<DiffOutlined />} disabled={selectedIds.length !== 2} onClick={handleDiff}>
          对比已勾选版本 ({selectedIds.length}/2)
        </Button>,
        <Button key="reload" icon={<ReloadOutlined />} loading={loading} onClick={() => load()}>
          刷新
        </Button>,
      ]}
    >
      <ProCard>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={backups}
          rowSelection={{
            selectedRowKeys: selectedIds,
            onChange: (keys) => setSelectedIds(keys.slice(-2) as string[]),
            getCheckboxProps: (r) => ({ disabled: r.status !== 'ok' }),
          }}
          columns={[
            { title: '版本 ID', dataIndex: 'id', width: 85 },
            { title: '设备', dataIndex: 'deviceName', width: 180 },
            {
              title: '状态',
              dataIndex: 'status',
              width: 90,
              render: (v: string) => <Tag color={v === 'ok' ? 'success' : 'error'}>{v === 'ok' ? '成功' : '失败'}</Tag>,
            },
            {
              title: '变更标记',
              dataIndex: 'changed',
              width: 110,
              render: (v: boolean, r) =>
                r.status !== 'ok' ? '-' : v ? <Tag color="orange">有变更</Tag> : <Tag>无变化</Tag>,
            },
            { title: '触发方式', dataIndex: 'trigger', width: 95, render: (v) => (v === 'scheduled' ? '定时' : '手动') },
            { title: '大小', dataIndex: 'sizeBytes', width: 100, render: (v) => (v ? `${v} B` : '-') },
            { title: 'SHA-256', dataIndex: 'sha256', width: 130, render: (v: string) => (v ? v.slice(0, 12) : '-') },
            { title: '备份时间', dataIndex: 'createdAt', width: 170, render: fmtUnix },
            {
              title: '操作',
              width: 100,
              render: (_, r) =>
                r.status === 'ok' ? (
                  <a
                    onClick={async () => {
                      const d = await getBackupDetail(r.id);
                      setViewContent({ title: `${r.deviceName} · 版本 #${r.id}`, content: d.content });
                    }}
                  >
                    <EyeOutlined /> 查看
                  </a>
                ) : (
                  <span style={{ color: '#ff4d4f', fontSize: 12 }}>{r.error}</span>
                ),
            },
          ]}
        />
      </ProCard>

      <Drawer
        title={viewContent?.title}
        width={780}
        open={!!viewContent}
        onClose={() => setViewContent(null)}
      >
        <pre style={{ background: '#141414', color: '#e6f4ff', padding: 12, borderRadius: 6, fontSize: 12, overflow: 'auto' }}>
          {viewContent?.content}
        </pre>
      </Drawer>

      <Drawer
        title={
          diffView ? (
            <Space>
              <span>{diffView.title}</span>
              <Tag color="success">+{diffView.added}</Tag>
              <Tag color="error">-{diffView.removed}</Tag>
            </Space>
          ) : (
            'Diff'
          )
        }
        width={880}
        open={!!diffView}
        onClose={() => setDiffView(null)}
      >
        <div style={{ fontFamily: 'monospace', fontSize: 12, border: '1px solid #f0f0f0', borderRadius: 6, overflow: 'hidden' }}>
          {(diffView?.lines || []).map((l, idx) => (
            <div
              key={idx}
              style={{
                padding: '2px 10px',
                background: l.kind === 'add' ? '#f6ffed' : l.kind === 'del' ? '#fff2f0' : '#fff',
                color: l.kind === 'add' ? '#237804' : l.kind === 'del' ? '#a8071a' : '#262626',
                whiteSpace: 'pre-wrap',
              }}
            >
              <span style={{ display: 'inline-block', width: 22, opacity: 0.6 }}>
                {l.kind === 'add' ? '+' : l.kind === 'del' ? '-' : ' '}
              </span>
              {l.text}
            </div>
          ))}
        </div>
      </Drawer>
    </PageContainer>
  );
};

export default BackupsPage;
