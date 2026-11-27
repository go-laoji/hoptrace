import {
  ApartmentOutlined,
  ExpandOutlined,
  ReloadOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import { Graph } from '@antv/g6';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import {
  Badge,
  Button,
  Descriptions,
  Drawer,
  Empty,
  Input,
  message,
  Segmented,
  Space,
  Switch,
  Tag,
} from 'antd';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  getTopology,
  rebuildTopology,
  type TopoLink,
  type TopologyData,
  type TopoNode,
} from '@/services/hoptrace';
import { ENDPOINT_TYPE_META, ROLE_META, STATUS_META } from '../constants';

const ROLE_COLOR: Record<string, string> = {
  firewall: '#fa541c',
  core: '#722ed1',
  aggregation: '#2f54eb',
  access: '#1677ff',
  router: '#13c2c2',
  ac: '#eb2f96',
  unknown: '#8c8c8c',
};

const TopologyPage: React.FC = () => {
  const containerRef = useRef<HTMLDivElement>(null);
  const graphRef = useRef<Graph | null>(null);

  const [topo, setTopo] = useState<TopologyData>({ nodes: [], links: [] });
  const [loading, setLoading] = useState(false);
  const [rebuilding, setRebuilding] = useState(false);
  const [layoutType, setLayoutType] = useState<'dagre' | 'force' | 'concentric'>('dagre');
  const [includeAllEndpoints, setIncludeAllEndpoints] = useState(false);
  const [expandedDeviceIds, setExpandedDeviceIds] = useState<string[]>([]);
  const [searchText, setSearchText] = useState('');
  const [selectedNode, setSelectedNode] = useState<TopoNode | null>(null);
  const [selectedLink, setSelectedLink] = useState<TopoLink | null>(null);

  const fetchTopology = useCallback(async () => {
    setLoading(true);
    try {
      const res = await getTopology({
        includeAllEndpoints,
        expandDeviceIds: includeAllEndpoints ? [] : expandedDeviceIds,
      });
      setTopo({ nodes: res.nodes || [], links: res.links || [] });
    } finally {
      setLoading(false);
    }
  }, [includeAllEndpoints, expandedDeviceIds]);

  useEffect(() => {
    fetchTopology();
  }, [fetchTopology]);

  // 渲染 AntV G6 v5 拓扑图
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    if (graphRef.current) {
      graphRef.current.destroy();
      graphRef.current = null;
    }
    if (!topo.nodes.length) return;

    const nodeMap = new Map(topo.nodes.map((n) => [n.id, n]));
    const linkMap = new Map(topo.links.map((l) => [l.id, l]));

    const g6Nodes = topo.nodes.map((n) => {
      const isEp = n.kind === 'endpoint';
      const fill = isEp
        ? n.status === 'online'
          ? '#52c41a'
          : '#bfbfbf'
        : n.managed
          ? ROLE_COLOR[n.role] || '#1677ff'
          : '#bfbfbf';
      const subtitle = isEp
        ? n.ip || ''
        : n.endpointCount > 0
          ? `${n.ip || ''} · ${n.endpointCount}终端`
          : n.ip || '';
      return {
        id: n.id,
        data: { ...n },
        style: {
          size: isEp ? 28 : 44,
          fill,
          stroke: n.status === 'offline' ? '#ff4d4f' : '#ffffff',
          lineWidth: n.status === 'offline' ? 3 : 2,
          lineDash: !isEp && !n.managed ? [4, 3] : undefined,
          labelText: subtitle ? `${n.name}\n${subtitle}` : n.name,
          labelPlacement: 'bottom' as const,
          labelFontSize: isEp ? 11 : 12,
          labelFill: '#262626',
          labelBackground: true,
          labelBackgroundFill: 'rgba(255,255,255,0.85)',
          labelBackgroundRadius: 3,
        },
      };
    });

    const g6Edges = topo.links.map((l) => {
      const isLag = (l.members || []).length > 1;
      const isAccess = l.kind === 'access';
      const stroke =
        l.status === 'down'
          ? '#ff4d4f'
          : l.status === 'stale'
            ? '#bfbfbf'
            : isAccess
              ? '#95de64'
              : isLag
                ? '#722ed1'
                : '#69b1ff';
      const label = isAccess
        ? l.sourcePort
        : isLag
          ? `${l.sourcePort} (×${l.members.length})`
          : `${l.sourcePort} ↔ ${l.targetPort}`;
      return {
        id: l.id,
        source: l.source,
        target: l.target,
        data: { ...l },
        style: {
          stroke,
          lineWidth: isLag ? 3.5 : isAccess ? 1.5 : 2,
          lineDash: l.status === 'stale' || isAccess ? [5, 4] : undefined,
          labelText: label,
          labelFontSize: 10,
          labelFill: '#595959',
          labelBackground: true,
          labelBackgroundFill: 'rgba(255,255,255,0.85)',
          labelAutoRotate: true,
        },
      };
    });

    const layoutCfg =
      layoutType === 'dagre'
        ? { type: 'dagre', rankdir: 'TB', nodesep: 65, ranksep: 90 }
        : layoutType === 'concentric'
          ? { type: 'concentric', nodeSize: 70, preventOverlap: true }
          : { type: 'force', linkDistance: 150, preventOverlap: true };

    const graph = new Graph({
      container: el,
      width: el.clientWidth || 960,
      height: 620,
      autoFit: 'view',
      padding: 40,
      data: { nodes: g6Nodes, edges: g6Edges },
      layout: layoutCfg,
      behaviors: ['drag-canvas', 'zoom-canvas', 'drag-element'],
    });

    graph.on('node:click', (evt: any) => {
      const id = evt?.target?.id;
      const hit = id ? nodeMap.get(id) : undefined;
      if (hit) {
        setSelectedLink(null);
        setSelectedNode(hit);
      }
    });

    graph.on('edge:click', (evt: any) => {
      const id = evt?.target?.id;
      const hit = id ? linkMap.get(id) : undefined;
      if (hit) {
        setSelectedNode(null);
        setSelectedLink(hit);
      }
    });

    graph.render();
    graphRef.current = graph;

    const onResize = () => {
      if (graphRef.current && containerRef.current) {
        graphRef.current.resize(containerRef.current.clientWidth || 960, 620);
      }
    };
    window.addEventListener('resize', onResize);
    return () => {
      window.removeEventListener('resize', onResize);
      graph.destroy();
      graphRef.current = null;
    };
  }, [topo, layoutType]);

  const handleSearch = (kw: string) => {
    setSearchText(kw);
    const q = kw.trim().toLowerCase();
    if (!q || !graphRef.current) return;
    const hit = topo.nodes.find(
      (n) =>
        n.name.toLowerCase().includes(q) ||
        (n.ip || '').toLowerCase().includes(q) ||
        (n.model || '').toLowerCase().includes(q),
    );
    if (hit) {
      graphRef.current.focusElement(hit.id, true);
      setSelectedNode(hit);
      setSelectedLink(null);
    } else {
      message.info('当前视图未匹配到节点');
    }
  };

  const handleRebuild = async () => {
    setRebuilding(true);
    try {
      await rebuildTopology();
      message.success('已完成拓扑融合与终端位置同步');
      await fetchTopology();
    } finally {
      setRebuilding(false);
    }
  };

  const toggleExpandDevice = (refId: string) => {
    setExpandedDeviceIds((prev) =>
      prev.includes(refId) ? prev.filter((id) => id !== refId) : [...prev, refId],
    );
  };

  return (
    <PageContainer
      title="网络拓扑"
      extra={[
        <Input.Search
          key="search"
          placeholder="搜索设备 / IP / 终端"
          allowClear
          style={{ width: 220 }}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          onSearch={handleSearch}
        />,
        <Segmented
          key="layout"
          value={layoutType}
          onChange={(v) => setLayoutType(v as any)}
          options={[
            { label: '层次布局', value: 'dagre' },
            { label: '力导向', value: 'force' },
            { label: '同心圆', value: 'concentric' },
          ]}
        />,
        <Space key="ep-toggle" size={6}>
          <span style={{ fontSize: 13 }}>展开全部终端</span>
          <Switch checked={includeAllEndpoints} onChange={setIncludeAllEndpoints} />
        </Space>,
        <Button key="fit" icon={<ExpandOutlined />} onClick={() => graphRef.current?.fitView()}>
          自适应
        </Button>,
        <Button key="rebuild" icon={<SyncOutlined />} loading={rebuilding} onClick={handleRebuild}>
          重建拓扑
        </Button>,
        <Button key="reload" type="primary" icon={<ReloadOutlined />} loading={loading} onClick={fetchTopology}>
          刷新
        </Button>,
      ]}
    >
      <ProCard
        styles={{ body: { padding: 0, position: 'relative', minHeight: 620 } }}
        extra={
          <Space size={8} wrap>
            <Tag color="volcano">防火墙</Tag>
            <Tag color="purple">核心</Tag>
            <Tag color="geekblue">汇聚</Tag>
            <Tag color="blue">接入</Tag>
            <Tag color="green">在线终端</Tag>
            <Tag>虚线:未纳管/接入</Tag>
          </Space>
        }
        title={
          <Space>
            <ApartmentOutlined />
            <span>
              节点 {topo.nodes.length} · 链路 {topo.links.length}
            </span>
          </Space>
        }
      >
        {topo.nodes.length === 0 ? (
          <div style={{ padding: '140px 0' }}>
            <Empty description="暂无拓扑数据，请先在「发现与凭据」中扫描网络" />
          </div>
        ) : (
          <div ref={containerRef} style={{ width: '100%', height: 620 }} />
        )}
      </ProCard>

      <Drawer
        title={selectedNode ? `${selectedNode.kind === 'device' ? '设备' : '终端'} · ${selectedNode.name}` : '链路详情'}
        width={420}
        open={!!selectedNode || !!selectedLink}
        onClose={() => {
          setSelectedNode(null);
          setSelectedLink(null);
        }}
      >
        {selectedNode && selectedNode.kind === 'device' && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Descriptions bordered size="small" column={1}>
              <Descriptions.Item label="名称">{selectedNode.name}</Descriptions.Item>
              <Descriptions.Item label="管理 IP">{selectedNode.ip || '-'}</Descriptions.Item>
              <Descriptions.Item label="角色">
                <Tag color={ROLE_META[selectedNode.role]?.color}>
                  {ROLE_META[selectedNode.role]?.label || selectedNode.role}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Badge
                  status={STATUS_META[selectedNode.status]?.badge || 'default'}
                  text={STATUS_META[selectedNode.status]?.label || selectedNode.status}
                />
              </Descriptions.Item>
              <Descriptions.Item label="厂商 / 型号">
                {`${selectedNode.vendor || '-'} ${selectedNode.model || ''}`.trim()}
              </Descriptions.Item>
              <Descriptions.Item label="纳管状态">
                {selectedNode.managed ? <Tag color="green">已纳管</Tag> : <Tag>未纳管占位</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="下挂终端数">{selectedNode.endpointCount}</Descriptions.Item>
            </Descriptions>
            {selectedNode.endpointCount > 0 && !includeAllEndpoints && (
              <Button
                type="primary"
                block
                onClick={() => toggleExpandDevice(selectedNode.refId)}
              >
                {expandedDeviceIds.includes(selectedNode.refId)
                  ? '折叠该设备的接入终端'
                  : `在拓扑中展开 ${selectedNode.endpointCount} 个接入终端`}
              </Button>
            )}
          </Space>
        )}

        {selectedNode && selectedNode.kind === 'endpoint' && (
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="名称">{selectedNode.name}</Descriptions.Item>
            <Descriptions.Item label="IP">{selectedNode.ip || '-'}</Descriptions.Item>
            <Descriptions.Item label="类型">
              <Tag color={ENDPOINT_TYPE_META[selectedNode.type]?.color}>
                {ENDPOINT_TYPE_META[selectedNode.type]?.label || selectedNode.type}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="状态">
              <Badge
                status={STATUS_META[selectedNode.status]?.badge || 'default'}
                text={STATUS_META[selectedNode.status]?.label || selectedNode.status}
              />
            </Descriptions.Item>
            <Descriptions.Item label="OUI 厂商">{selectedNode.vendor || '-'}</Descriptions.Item>
          </Descriptions>
        )}

        {selectedLink && (
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="类型">{selectedLink.kind}</Descriptions.Item>
            <Descriptions.Item label="本端接口">{selectedLink.sourcePort || '-'}</Descriptions.Item>
            <Descriptions.Item label="对端接口">{selectedLink.targetPort || '-'}</Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={selectedLink.status === 'up' ? 'success' : selectedLink.status === 'down' ? 'error' : 'default'}>
                {selectedLink.status}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="置信度">
              {Math.round((selectedLink.confidence || 0) * 100)}%
            </Descriptions.Item>
            {(selectedLink.members || []).length > 0 && (
              <Descriptions.Item label="成员物理链路">
                <Space direction="vertical" size={4}>
                  {selectedLink.members.map((m) => (
                    <Tag key={m}>{m}</Tag>
                  ))}
                </Space>
              </Descriptions.Item>
            )}
          </Descriptions>
        )}
      </Drawer>
    </PageContainer>
  );
};

export default TopologyPage;
