import { ReloadOutlined, WarningOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Button, Drawer, Progress, Segmented, Space, Table, Tabs, Tag } from 'antd';
import React, { useEffect, useState } from 'react';
import {
  getSubnetDetail,
  type IpLease,
  listAnomalies,
  listSubnets,
  type NetworkAnomaly,
  type SubnetSummary,
} from '@/services/hoptrace';

const ANOMALY_META: Record<string, { label: string; color: string }> = {
  ip_conflict: { label: 'IP 地址冲突', color: 'error' },
  suspect_hub: { label: '疑似私接小交换机', color: 'warning' },
  mac_flap: { label: 'MAC 多口漂移/环路嫌疑', color: 'orange' },
};

const IpamPage: React.FC = () => {
  const [subnets, setSubnets] = useState<SubnetSummary[]>([]);
  const [anomalies, setAnomalies] = useState<NetworkAnomaly[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeCidr, setActiveCidr] = useState<string | null>(null);
  const [leases, setLeases] = useState<IpLease[]>([]);
  const [leaseFilter, setLeaseFilter] = useState<'all' | 'used' | 'free' | 'conflict'>('used');
  const [detailLoading, setDetailLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const [s, a] = await Promise.all([listSubnets(), listAnomalies()]);
      setSubnets(s.items || []);
      setAnomalies(a.items || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const openSubnet = async (cidr: string) => {
    setActiveCidr(cidr);
    setDetailLoading(true);
    try {
      const res = await getSubnetDetail(cidr);
      setLeases(res.leases || []);
    } finally {
      setDetailLoading(false);
    }
  };

  const filteredLeases = leases.filter((l) => {
    if (leaseFilter === 'used') return l.used;
    if (leaseFilter === 'free') return !l.used;
    if (leaseFilter === 'conflict') return l.conflict;
    return true;
  });

  return (
    <PageContainer
      title="IP 地址管理 (IPAM) 与异常检测"
      extra={[
        <Button key="reload" icon={<ReloadOutlined />} loading={loading} onClick={load}>
          刷新分析
        </Button>,
      ]}
    >
      <Tabs
        items={[
          {
            key: 'subnets',
            label: `子网占用率 (${subnets.length})`,
            children: (
              <ProCard>
                <Table
                  rowKey="cidr"
                  loading={loading}
                  pagination={{ pageSize: 20 }}
                  dataSource={subnets}
                  columns={[
                    {
                      title: '网段 CIDR',
                      dataIndex: 'cidr',
                      render: (v: string) => <a onClick={() => openSubnet(v)}>{v}</a>,
                    },
                    { title: '网关/核心设备', dataIndex: 'gatewayDevice', render: (v) => v || '-' },
                    { title: '已用 / 可用', render: (_, r) => `${r.usedHosts} / ${r.freeHosts} (共 ${r.totalHosts})` },
                    {
                      title: '占用率',
                      dataIndex: 'usagePct',
                      width: 220,
                      render: (v: number) => (
                        <Progress
                          percent={v}
                          size="small"
                          status={v > 85 ? 'exception' : 'normal'}
                        />
                      ),
                    },
                    {
                      title: 'IP 冲突',
                      dataIndex: 'conflictCount',
                      width: 100,
                      render: (v: number) => (v > 0 ? <Tag color="error">{v} 处冲突</Tag> : <Tag color="success">正常</Tag>),
                    },
                    {
                      title: '操作',
                      width: 100,
                      render: (_, r) => <a onClick={() => openSubnet(r.cidr)}>查看 IP 表</a>,
                    },
                  ]}
                />
              </ProCard>
            ),
          },
          {
            key: 'anomalies',
            label: (
              <Space>
                <WarningOutlined />
                异常检测 ({anomalies.length})
              </Space>
            ),
            children: (
              <ProCard>
                <Table
                  rowKey={(r, i) => `${r.kind}-${r.target}-${i}`}
                  loading={loading}
                  pagination={{ pageSize: 20 }}
                  dataSource={anomalies}
                  columns={[
                    {
                      title: '异常类型',
                      dataIndex: 'kind',
                      width: 170,
                      render: (v: string) => {
                        const m = ANOMALY_META[v] || { label: v, color: 'default' };
                        return <Tag color={m.color}>{m.label}</Tag>;
                      },
                    },
                    { title: '目标对象', dataIndex: 'target', width: 220 },
                    { title: '详细说明', dataIndex: 'detail' },
                  ]}
                />
              </ProCard>
            ),
          },
        ]}
      />

      <Drawer
        title={`子网 IP 明细 · ${activeCidr || ''}`}
        width={860}
        open={!!activeCidr}
        onClose={() => setActiveCidr(null)}
        extra={
          <Segmented
            value={leaseFilter}
            onChange={(v) => setLeaseFilter(v as any)}
            options={[
              { label: '已用', value: 'used' },
              { label: '空闲', value: 'free' },
              { label: '冲突', value: 'conflict' },
              { label: '全部', value: 'all' },
            ]}
          />
        }
      >
        <Table
          rowKey="ip"
          size="small"
          loading={detailLoading}
          pagination={{ pageSize: 30 }}
          dataSource={filteredLeases}
          columns={[
            { title: 'IP 地址', dataIndex: 'ip', width: 130 },
            {
              title: '状态',
              width: 90,
              render: (_, r) =>
                r.conflict ? (
                  <Tag color="error">冲突</Tag>
                ) : r.used ? (
                  <Tag color="blue">已用</Tag>
                ) : (
                  <Tag>空闲</Tag>
                ),
            },
            {
              title: 'MAC 地址',
              width: 175,
              render: (_, r) => (r.macs && r.macs.length > 1 ? r.macs.join(', ') : r.mac || '-'),
            },
            { title: '厂商', dataIndex: 'vendor', ellipsis: true, render: (v) => v || '-' },
            {
              title: '接入位置',
              render: (_, r) => (r.deviceName ? `${r.deviceName} / ${r.ifName}` : '-'),
            },
          ]}
        />
      </Drawer>
    </PageContainer>
  );
};

export default IpamPage;
