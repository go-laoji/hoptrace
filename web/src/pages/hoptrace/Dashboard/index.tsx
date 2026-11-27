import {
  ApartmentOutlined,
  ClusterOutlined,
  LaptopOutlined,
  RadarChartOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Alert, Button, Col, Progress, Row, Space, Statistic, Table, Tag } from 'antd';
import React, { useEffect, useState } from 'react';
import { type DashboardData, getDashboard } from '@/services/hoptrace';
import { ENDPOINT_TYPE_META, EVENT_META, fmtUnix, ROLE_META } from '../constants';

const DashboardPage: React.FC = () => {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setData(await getDashboard());
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const devTotal = Number(data?.devicesTotal || 0);
  const epTotal = Number(data?.endpointsTotal || 0);

  return (
    <PageContainer
      title="网络总览"
      extra={[
        <Button key="discover" type="primary" icon={<RadarChartOutlined />} onClick={() => history.push('/discovery')}>
          发起设备发现
        </Button>,
        <Button key="topo" icon={<ApartmentOutlined />} onClick={() => history.push('/topology')}>
          查看拓扑图
        </Button>,
        <Button key="refresh" icon={<ReloadOutlined />} loading={loading} onClick={load}>
          刷新
        </Button>,
      ]}
    >
      {data && devTotal === 0 && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="尚未发现任何网络设备"
          description="请先前往「发现与凭据」创建 SNMP 凭据，并输入种子设备 IP（如模拟器种子 10.255.0.1）启动自动发现。"
          action={
            <Button size="small" type="primary" onClick={() => history.push('/discovery')}>
              立即配置
            </Button>
          }
        />
      )}

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <ProCard hoverable onClick={() => history.push('/devices')}>
            <Statistic
              title="网络设备（在线 / 总计）"
              prefix={<ClusterOutlined />}
              value={Number(data?.devicesOnline || 0)}
              suffix={`/ ${devTotal}`}
            />
            <div style={{ marginTop: 8, fontSize: 12, color: '#8c8c8c' }}>
              离线 {data?.devicesOffline || 0} · 未纳管占位 {data?.devicesUnmanaged || 0}
            </div>
          </ProCard>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <ProCard hoverable onClick={() => history.push('/topology')}>
            <Statistic
              title="拓扑逻辑链路"
              prefix={<ApartmentOutlined />}
              value={Number(data?.linksTotal || 0)}
            />
            <div style={{ marginTop: 8, fontSize: 12, color: '#8c8c8c' }}>
              中断 {data?.linksDown || 0} · 陈旧 {data?.linksStale || 0}
            </div>
          </ProCard>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <ProCard hoverable onClick={() => history.push('/endpoints')}>
            <Statistic
              title="接入终端（在线 / 总计）"
              prefix={<LaptopOutlined />}
              value={Number(data?.endpointsOnline || 0)}
              suffix={`/ ${epTotal}`}
            />
            <div style={{ marginTop: 8, fontSize: 12, color: '#8c8c8c' }}>
              近 24 小时新接入 {data?.endpointsNew24h || 0}
            </div>
          </ProCard>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <ProCard hoverable onClick={() => history.push('/endpoints')}>
            <Statistic
              title="待确认信任终端"
              value={Number(data?.endpointsUntrusted || 0)}
              valueStyle={{ color: Number(data?.endpointsUntrusted || 0) > 0 ? '#fa8c16' : '#52c41a' }}
            />
            <div style={{ marginTop: 8, fontSize: 12, color: '#8c8c8c' }}>
              可在接入终端页标记为已信任或阻断
            </div>
          </ProCard>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={12}>
          <ProCard title="设备角色分布" loading={loading}>
            <Space direction="vertical" style={{ width: '100%' }}>
              {(data?.devicesByRole || []).map((kv) => {
                const meta = ROLE_META[kv.name] || { label: kv.name || '未知', color: 'default' };
                const val = Number(kv.value || 0);
                const pct = devTotal > 0 ? Math.round((val / devTotal) * 100) : 0;
                return (
                  <div key={kv.name} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <Tag color={meta.color} style={{ width: 92, textAlign: 'center' }}>
                      {meta.label}
                    </Tag>
                    <Progress percent={pct} size="small" style={{ flex: 1, marginBottom: 0 }} format={() => `${val} 台`} />
                  </div>
                );
              })}
              {(!data?.devicesByRole || data.devicesByRole.length === 0) && (
                <span style={{ color: '#8c8c8c' }}>暂无数据</span>
              )}
            </Space>
          </ProCard>
        </Col>
        <Col xs={24} lg={12}>
          <ProCard title="接入终端类型分布" loading={loading}>
            <Space direction="vertical" style={{ width: '100%' }}>
              {(data?.endpointsByType || []).map((kv) => {
                const meta = ENDPOINT_TYPE_META[kv.name] || { label: kv.name || '未知', color: 'default' };
                const val = Number(kv.value || 0);
                const pct = epTotal > 0 ? Math.round((val / epTotal) * 100) : 0;
                return (
                  <div key={kv.name} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <Tag color={meta.color} style={{ width: 108, textAlign: 'center' }}>
                      {meta.label}
                    </Tag>
                    <Progress percent={pct} size="small" style={{ flex: 1, marginBottom: 0 }} format={() => `${val} 个`} />
                  </div>
                );
              })}
              {(!data?.endpointsByType || data.endpointsByType.length === 0) && (
                <span style={{ color: '#8c8c8c' }}>暂无数据</span>
              )}
            </Space>
          </ProCard>
        </Col>
      </Row>

      <ProCard title="最近终端动态" style={{ marginTop: 16 }} loading={loading}>
        <Table
          rowKey="id"
          size="small"
          pagination={false}
          dataSource={data?.recentEvents || []}
          columns={[
            {
              title: '事件',
              dataIndex: 'type',
              width: 120,
              render: (v: string) => {
                const m = EVENT_META[v] || { label: v, color: 'default' };
                return <Tag color={m.color}>{m.label}</Tag>;
              },
            },
            { title: 'MAC', dataIndex: 'mac', width: 160 },
            { title: 'IP', dataIndex: 'ip', width: 140, render: (v) => v || '-' },
            { title: '终端名称', dataIndex: 'name', width: 160, render: (v) => v || '-' },
            { title: '详情', dataIndex: 'detail' },
            { title: '时间', dataIndex: 'at', width: 170, render: fmtUnix },
          ]}
        />
      </ProCard>
    </PageContainer>
  );
};

export default DashboardPage;
