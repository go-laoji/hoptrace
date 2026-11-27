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

const ROLE_THEME: Record<string, { border: string; bg: string; badge: string }> = {
  firewall: { border: '#fa541c', bg: '#fff2e8', badge: '防火墙' },
  core: { border: '#722ed1', bg: '#f9f0ff', badge: '核心' },
  aggregation: { border: '#2f54eb', bg: '#f0f5ff', badge: '汇聚' },
  access: { border: '#1677ff', bg: '#e6f4ff', badge: '接入' },
  router: { border: '#13c2c2', bg: '#e6fffb', badge: '路由' },
  ac: { border: '#eb2f96', bg: '#fff0f6', badge: 'AC' },
  unknown: { border: '#8c8c8c', bg: '#fafafa', badge: '未知' },
};

function shortPort(p?: string): string {
  if (!p) return '-';
  return p
    .replace(/^XGigabitEthernet/i, 'XGE')
    .replace(/^GigabitEthernet/i, 'GE')
    .replace(/^FastEthernet/i, 'FE');
}

function escapeHtml(s?: string): string {
  return String(s || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

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
      // 默认隐藏已失效的 stale 链路，避免与活跃链路重叠造成视觉杂乱
      const activeLinks = (res.links || []).filter((l) => l.status !== 'stale');
      setTopo({ nodes: res.nodes || [], links: activeLinks });
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
      if (isEp) {
        const online = n.status === 'online';
        return {
          id: n.id,
          type: 'rect',
          data: { ...n },
          style: {
            size: [132, 40],
            radius: 20,
            fill: online ? '#f6ffed' : '#f5f5f5',
            stroke: online ? '#52c41a' : '#d9d9d9',
            lineWidth: 1.5,
            shadowColor: 'rgba(0,0,0,0.06)',
            shadowBlur: 6,
            shadowOffsetY: 2,
            labelText: n.ip ? `${n.name}\n${n.ip}` : n.name,
            labelPlacement: 'center' as const,
            labelFontSize: 11,
            labelLineHeight: 15,
            labelFill: '#262626',
            labelMaxWidth: 118,
            labelWordWrap: false,
            cursor: 'pointer',
          },
        };
      }

      const theme = ROLE_THEME[n.role] || ROLE_THEME.unknown;
      const offline = n.status === 'offline';
      const label = n.ip ? `${n.name}\n${n.ip}` : n.name;
      return {
        id: n.id,
        type: 'rect',
        data: { ...n },
        style: {
          size: [152, 54],
          radius: 10,
          fill: n.managed ? '#ffffff' : '#fafafa',
          stroke: offline ? '#ff4d4f' : n.managed ? theme.border : '#bfbfbf',
          lineWidth: n.role === 'core' || n.role === 'firewall' ? 2.5 : 1.8,
          lineDash: n.managed ? undefined : [5, 3],
          shadowColor: 'rgba(15, 23, 42, 0.08)',
          shadowBlur: 10,
          shadowOffsetY: 3,
          // 节点仅显示名称和管理 IP，保持画布整洁
          labelText: label,
          labelPlacement: 'center' as const,
          labelFontSize: 12,
          labelFontWeight: 500,
          labelLineHeight: 18,
          labelFill: '#1f2937',
          labelMaxWidth: 138,
          // 左上角角色小徽章，视觉区分设备角色
          badges: [
            {
              text: theme.badge,
              placement: 'top-right' as const,
              fill: theme.border,
              backgroundFill: theme.bg,
              backgroundStroke: theme.border,
              backgroundLineWidth: 1,
              fontSize: 10,
              padding: [1, 5],
            },
          ],
          cursor: 'pointer',
        },
      };
    });

    const g6Edges = topo.links.map((l) => {
      const isLag = (l.members || []).length > 1;
      const isAccess = l.kind === 'access';
      const isDown = l.status === 'down';
      const stroke = isDown
        ? '#ff4d4f'
        : isAccess
          ? '#95de64'
          : isLag
            ? '#722ed1'
            : '#91caff';
      return {
        id: l.id,
        source: l.source,
        target: l.target,
        type: layoutType === 'dagre' ? 'cubic-vertical' : 'line',
        data: { ...l },
        style: {
          stroke,
          lineWidth: isLag ? 3 : isAccess ? 1.5 : 2,
          lineDash: isAccess ? [4, 4] : undefined,
          // 连线默认不显示文字，悬停时通过 tooltip 展示连接详情
          labelText: '',
          cursor: 'pointer',
        },
      };
    });

    const layoutCfg =
      layoutType === 'dagre'
        ? { type: 'dagre', rankdir: 'TB', nodesep: 48, ranksep: 115, nodeSize: [160, 60] }
        : layoutType === 'concentric'
          ? { type: 'concentric', nodeSize: 170, preventOverlap: true, equidistant: true }
          : { type: 'force', linkDistance: 210, preventOverlap: true, nodeSize: 165 };

    const graph = new Graph({
      container: el,
      width: el.clientWidth || 1000,
      height: 640,
      autoFit: 'view',
      padding: [48, 48, 48, 48],
      data: { nodes: g6Nodes as any, edges: g6Edges as any },
      layout: layoutCfg,
      edge: {
        state: {
          active: {
            stroke: '#1677ff',
            lineWidth: 3.5,
          },
          inactive: {
            strokeOpacity: 0.25,
          },
        },
      },
      node: {
        state: {
          active: {
            shadowColor: 'rgba(22, 119, 255, 0.35)',
            shadowBlur: 14,
          },
          inactive: {
            opacity: 0.35,
          },
        },
      },
      behaviors: [
        'drag-canvas',
        'zoom-canvas',
        'drag-element',
        {
          type: 'hover-activate',
          degree: 1,
          state: 'active',
          inactiveState: 'inactive',
        },
      ],
      plugins: [
        {
          type: 'tooltip',
          trigger: 'hover',
          enable: true,
          enterable: false,
          getContent: (_evt: any, items: any[]) => {
            const item = items?.[0];
            if (!item) return '';
            const edge = linkMap.get(item.id);
            if (edge) {
              const src = nodeMap.get(edge.source);
              const dst = nodeMap.get(edge.target);
              const isLag = (edge.members || []).length > 1;
              const membersHtml = isLag
                ? `<div style="margin-top:6px;padding-top:6px;border-top:1px solid #f0f0f0;font-size:11px;color:#595959;">
                     <div style="margin-bottom:2px;font-weight:500;">聚合链路成员 (${edge.members.length}):</div>
                     ${edge.members.map((m) => `<div>• ${escapeHtml(m)}</div>`).join('')}
                   </div>`
                : '';
              return `
                <div style="padding:4px 6px;min-width:220px;font-size:12px;line-height:1.6;color:#262626;">
                  <div style="font-weight:600;margin-bottom:4px;color:#1677ff;">
                    ${escapeHtml(src?.name || edge.source)} ↔ ${escapeHtml(dst?.name || edge.target)}
                  </div>
                  <div><strong>本端端口：</strong>${escapeHtml(shortPort(edge.sourcePort))}</div>
                  ${edge.targetPort ? `<div><strong>对端端口：</strong>${escapeHtml(shortPort(edge.targetPort))}</div>` : ''}
                  <div><strong>链路类型：</strong>${escapeHtml(edge.kind.toUpperCase())}${isLag ? ' (Eth-Trunk)' : ''} · 置信度 ${Math.round((edge.confidence || 0) * 100)}%</div>
                  ${membersHtml}
                </div>`;
            }
            const node = nodeMap.get(item.id);
            if (node) {
              if (node.kind === 'endpoint') {
                return `
                  <div style="padding:4px 6px;font-size:12px;line-height:1.6;color:#262626;">
                    <div style="font-weight:600;">${escapeHtml(node.name)}</div>
                    <div>IP：${escapeHtml(node.ip || '-')}</div>
                    <div>厂商：${escapeHtml(node.vendor || '-')}</div>
                  </div>`;
              }
              return `
                <div style="padding:4px 6px;font-size:12px;line-height:1.6;color:#262626;">
                  <div style="font-weight:600;">${escapeHtml(node.name)}</div>
                  <div>管理 IP：${escapeHtml(node.ip || '-')}</div>
                  <div>型号：${escapeHtml([node.vendor, node.model].filter(Boolean).join(' ') || '-')}</div>
                  <div>下挂终端：${node.endpointCount || 0} 个（点击展开/查看详情）</div>
                </div>`;
            }
            return '';
          },
        },
      ],
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
        graphRef.current.resize(containerRef.current.clientWidth || 1000, 640);
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
        styles={{
          body: {
            padding: 0,
            position: 'relative',
            minHeight: 640,
            backgroundColor: '#fafbfd',
            backgroundImage: 'radial-gradient(#d9e1ec 1px, transparent 1px)',
            backgroundSize: '20px 20px',
          },
        }}
        extra={
          <Space size={6} wrap>
            <Tag color="volcano">防火墙</Tag>
            <Tag color="purple">核心</Tag>
            <Tag color="geekblue">汇聚</Tag>
            <Tag color="blue">接入</Tag>
            <Tag color="green">在线终端</Tag>
            <span style={{ fontSize: 12, color: '#8c8c8c', marginLeft: 4 }}>
              悬停连线查看端口 · 点击节点展开终端
            </span>
          </Space>
        }
        title={
          <Space>
            <ApartmentOutlined />
            <span>
              节点 {topo.nodes.length} · 活跃链路 {topo.links.length}
            </span>
          </Space>
        }
      >
        {topo.nodes.length === 0 ? (
          <div style={{ padding: '160px 0' }}>
            <Empty description="暂无拓扑数据，请先在「发现与凭据」中扫描网络" />
          </div>
        ) : (
          <div ref={containerRef} style={{ width: '100%', height: 640 }} />
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
