import {
  ApartmentOutlined,
  CameraOutlined,
  CompassOutlined,
  ExpandOutlined,
  ReloadOutlined,
  SaveOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import { Graph } from '@antv/g6';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import {
  Alert,
  Badge,
  Button,
  Descriptions,
  Drawer,
  Empty,
  Input,
  message,
  Modal,
  Segmented,
  Select,
  Space,
  Steps,
  Switch,
  Tag,
} from 'antd';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  getTopology,
  rebuildTopology,
  resetTopologyLayout,
  saveTopologyLayout,
  subscribeLiveEvents,
  type TopoLink,
  type TopologyData,
  type TopoNode,
  tracePath,
  type TracePathReply,
} from '@/services/hoptrace';
import { ENDPOINT_TYPE_META, fmtBps, ROLE_META, STATUS_META } from '../constants';

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

// 矩形防遮挡后处理：按卡片真实宽高（含角色徽章与留白）消除任意重叠节点
function resolveNodeOverlaps(
  graph: Graph,
  nodes: TopoNode[],
): { id: string; style: { x: number; y: number } }[] {
  const boxes = nodes
    .map((n) => {
      const datum: any = graph.getNodeData(n.id);
      const x = Number(datum?.style?.x);
      const y = Number(datum?.style?.y);
      if (Number.isNaN(x) || Number.isNaN(y)) return null;
      const isEp = n.kind === 'endpoint';
      return {
        id: n.id,
        x,
        y,
        ox: x,
        oy: y,
        // 设备卡片 152x54 + 右上角徽章 + 留白；终端胶囊 132x40 + 留白
        halfW: isEp ? 78 : 96,
        halfH: isEp ? 30 : 40,
      };
    })
    .filter((b): b is NonNullable<typeof b> => b !== null);

  if (boxes.length < 2) return [];

  let moved = false;
  for (let iter = 0; iter < 80; iter++) {
    let anyOverlap = false;
    for (let i = 0; i < boxes.length; i++) {
      const a = boxes[i];
      for (let j = i + 1; j < boxes.length; j++) {
        const b = boxes[j];
        let dx = b.x - a.x;
        let dy = b.y - a.y;
        const overlapX = a.halfW + b.halfW - Math.abs(dx);
        const overlapY = a.halfH + b.halfH - Math.abs(dy);
        if (overlapX > 0 && overlapY > 0) {
          anyOverlap = true;
          moved = true;
          if (Math.abs(dx) < 0.01 && Math.abs(dy) < 0.01) {
            const angle = ((i * 7 + j * 13) % 360) * (Math.PI / 180);
            dx = Math.cos(angle);
            dy = Math.sin(angle);
          }
          if (overlapX < overlapY) {
            const push = overlapX / 2 + 2;
            const sign = dx >= 0 ? 1 : -1;
            a.x -= sign * push;
            b.x += sign * push;
          } else {
            const push = overlapY / 2 + 2;
            const sign = dy >= 0 ? 1 : -1;
            a.y -= sign * push;
            b.y += sign * push;
          }
        }
      }
    }
    if (!anyOverlap) break;
  }

  if (!moved) return [];
  return boxes
    .filter((b) => Math.abs(b.x - b.ox) > 0.5 || Math.abs(b.y - b.oy) > 0.5)
    .map((b) => ({ id: b.id, style: { x: b.x, y: b.y } }));
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
  const [sseConnected, setSseConnected] = useState(false);
  const [roleFilter, setRoleFilter] = useState<string[]>([]);
  const [traceOpen, setTraceOpen] = useState(false);
  const [traceSrc, setTraceSrc] = useState('');
  const [traceDst, setTraceDst] = useState('');
  const [traceResult, setTraceResult] = useState<TracePathReply | null>(null);
  const [relayoutSeed, setRelayoutSeed] = useState(0);

  const fetchTopology = useCallback(async () => {
    setLoading(true);
    try {
      const res = await getTopology({
        includeAllEndpoints,
        expandDeviceIds: includeAllEndpoints ? [] : expandedDeviceIds,
      });
      // 仅隐藏设备间已失效的 stale 链路；接入终端连线(kind=access)即使是离线(stale)也保留虚线显示，避免终端悬空无连线
      const activeLinks = (res.links || []).filter((l) => l.kind === 'access' || l.status !== 'stale');
      setTopo({ nodes: res.nodes || [], links: activeLinks });
    } finally {
      setLoading(false);
    }
  }, [includeAllEndpoints, expandedDeviceIds]);

  useEffect(() => {
    fetchTopology();
  }, [fetchTopology]);

  // 订阅后端 SSE：收到 Trap 增量采集或轮询更新时自动刷新拓扑
  useEffect(() => {
    return subscribeLiveEvents(
      (ev) => {
        if (ev.source === 'trap' && ev.message) {
          message.info(ev.message);
        }
        fetchTopology();
      },
      setSseConnected,
    );
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

    const visibleNodes =
      roleFilter.length === 0
        ? topo.nodes
        : topo.nodes.filter((n) => n.kind === 'endpoint' || roleFilter.includes(n.role));
    const visibleIds = new Set(visibleNodes.map((n) => n.id));
    const visibleLinks = topo.links.filter((l) => visibleIds.has(l.source) && visibleIds.has(l.target));

    const nodeMap = new Map(visibleNodes.map((n) => [n.id, n]));
    const linkMap = new Map(visibleLinks.map((l) => [l.id, l]));
    const traceNodeSet = new Set(traceResult?.found ? traceResult.nodeIds || [] : []);
    const traceLinkSet = new Set(traceResult?.found ? traceResult.linkIds || [] : []);
    const hasEndpoints = visibleNodes.some((n) => n.kind === 'endpoint');
    const hasSavedLayout =
      !hasEndpoints &&
      visibleNodes.filter((n) => n.kind === 'device').length > 0 &&
      visibleNodes.filter((n) => n.kind === 'device').every((n) => n.hasPos);

    const g6Nodes = visibleNodes.map((n) => {
      const isEp = n.kind === 'endpoint';
      const inTrace = traceNodeSet.has(n.id);
      if (isEp) {
        const online = n.status === 'online';
        return {
          id: n.id,
          type: 'rect',
          data: { ...n },
          style: {
            size: [132, 40],
            radius: 20,
            fill: inTrace ? '#fff7e6' : online ? '#f6ffed' : '#f5f5f5',
            stroke: inTrace ? '#fa8c16' : online ? '#52c41a' : '#d9d9d9',
            lineWidth: inTrace ? 3 : 1.5,
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
          ...(hasSavedLayout && n.hasPos ? { x: n.posX, y: n.posY } : {}),
          size: [152, 54],
          radius: 10,
          fill: inTrace ? '#fff7e6' : n.managed ? '#ffffff' : '#fafafa',
          stroke: inTrace ? '#fa8c16' : offline ? '#ff4d4f' : n.managed ? theme.border : '#bfbfbf',
          lineWidth: inTrace ? 3.5 : n.role === 'core' || n.role === 'firewall' ? 2.5 : 1.8,
          lineDash: n.managed ? undefined : [5, 3],
          shadowColor: 'rgba(15, 23, 42, 0.08)',
          shadowBlur: 10,
          shadowOffsetY: 3,
          labelText: label,
          labelPlacement: 'center' as const,
          labelFontSize: 12,
          labelFontWeight: 500,
          labelLineHeight: 18,
          labelFill: '#1f2937',
          labelMaxWidth: 138,
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

    const g6Edges = visibleLinks.map((l) => {
      const isLag = (l.members || []).length > 1;
      const isAccess = l.kind === 'access';
      const isDown = l.status === 'down';
      const isOfflineAccess = isAccess && l.status === 'stale';
      const inTrace = traceLinkSet.has(l.id);
      const util = l.utilizationPct || 0;
      const stroke = inTrace
        ? '#fa8c16'
        : isDown || util >= 90
          ? '#ff4d4f'
          : util >= 70
            ? '#fa8c16'
            : isOfflineAccess
              ? '#bfbfbf'
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
          lineWidth: inTrace ? 4 : isLag ? 3 : isAccess ? 1.5 : 2,
          lineDash: isAccess ? [4, 4] : undefined,
          labelText: '',
          cursor: 'pointer',
        },
      };
    });

    const layoutCfg = hasSavedLayout
      ? undefined
      : layoutType === 'dagre'
        ? { type: 'dagre', rankdir: 'TB', nodesep: 52, ranksep: 120, nodeSize: [184, 72] }
        : layoutType === 'concentric'
          ? { type: 'concentric', nodeSize: 205, minNodeSpacing: 40, preventOverlap: true, equidistant: true }
          : {
              // d3-force 同步迭代收敛 + 碰撞半径，随后叠加矩形防遮挡后处理彻底消除重叠
              type: 'd3-force',
              animation: false,
              iterations: 350,
              preventOverlap: true,
              link: {
                distance: (e: any) => {
                  const kind = e?._original?.data?.kind || e?.data?.kind;
                  return kind === 'access' ? 190 : 280;
                },
                strength: 0.4,
              },
              collide: {
                radius: (n: any) => {
                  const kind = n?._original?.data?.kind || n?.data?.kind;
                  return kind === 'endpoint' ? 92 : 122;
                },
                strength: 1,
                iterations: 10,
              },
              manyBody: {
                strength: -950,
              },
            };

    const initWidth = el.clientWidth || window.innerWidth - 260;
    const initHeight = el.clientHeight || Math.max(window.innerHeight - 190, 560);

    const graph = new Graph({
      container: el,
      width: initWidth,
      height: initHeight,
      autoFit: 'view',
      padding: [36, 36, 36, 36],
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
                  ${Number(edge.inBps || 0) > 0 || Number(edge.outBps || 0) > 0 ? `<div><strong>实时流量：</strong>↓${escapeHtml(fmtBps(edge.inBps))} / ↑${escapeHtml(fmtBps(edge.outBps))} (${edge.utilizationPct || 0}%)</div>` : ''}
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

    graph.on('node:dblclick', (evt: any) => {
      const id = evt?.target?.id;
      const hit = id ? nodeMap.get(id) : undefined;
      if (hit && hit.kind === 'device' && hit.endpointCount > 0 && !includeAllEndpoints) {
        setSelectedNode(null);
        setSelectedLink(null);
        setExpandedDeviceIds((prev) =>
          prev.includes(hit.refId) ? prev.filter((x) => x !== hit.refId) : [...prev, hit.refId],
        );
        setRelayoutSeed((s) => s + 1);
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

    let cancelled = false;
    graphRef.current = graph;
    graph
      .render()
      .then(async () => {
        if (cancelled || graphRef.current !== graph) return;
        if (!hasSavedLayout) {
          const updates = resolveNodeOverlaps(graph, visibleNodes);
          if (updates.length > 0) {
            graph.updateNodeData(updates);
            await graph.draw();
          }
          if (!cancelled && graphRef.current === graph) {
            await graph.fitView();
          }
        }
      })
      .catch(() => {
        // 忽略组件卸载/切换时的中止错误
      });

    const onResize = () => {
      if (graphRef.current && containerRef.current) {
        const w = containerRef.current.clientWidth;
        const h = containerRef.current.clientHeight;
        if (w > 0 && h > 0) {
          graphRef.current.resize(w, h);
        }
      }
    };
    const ro = new ResizeObserver(onResize);
    ro.observe(el);
    window.addEventListener('resize', onResize);
    return () => {
      cancelled = true;
      ro.disconnect();
      window.removeEventListener('resize', onResize);
      graph.destroy();
      graphRef.current = null;
    };
  }, [topo, layoutType, roleFilter, traceResult, relayoutSeed, includeAllEndpoints]);

  const handleSaveLayout = async () => {
    if (!graphRef.current) return;
    const positions: { deviceId: string; x: number; y: number }[] = [];
    for (const n of topo.nodes) {
      if (n.kind !== 'device') continue;
      const d: any = graphRef.current.getNodeData(n.id);
      const x = Number(d?.style?.x);
      const y = Number(d?.style?.y);
      if (!Number.isNaN(x) && !Number.isNaN(y)) {
        positions.push({ deviceId: n.refId, x, y });
      }
    }
    if (positions.length === 0) return;
    await saveTopologyLayout(positions);
    message.success(`已保存 ${positions.length} 个节点的自定义位置`);
    fetchTopology();
  };

  const handleResetLayout = async () => {
    await resetTopologyLayout();
    message.success('已重置为自动布局');
    fetchTopology();
  };

  const handleExportPng = async () => {
    if (!graphRef.current) return;
    const dataUrl = await graphRef.current.toDataURL({ mode: 'overall', type: 'image/png' });
    const a = document.createElement('a');
    a.href = dataUrl;
    a.download = `hoptrace-topology-${Date.now()}.png`;
    a.click();
  };

  const handleTrace = async () => {
    if (!traceSrc.trim()) {
      message.warning('请输入源 IP / MAC / 设备名');
      return;
    }
    const res = await tracePath(traceSrc.trim(), traceDst.trim());
    setTraceResult(res);
    setTraceOpen(false);
    if (!res.found) {
      message.warning(res.message || '未找到路径');
    } else {
      message.success(`路径追踪成功：${res.message}`);
    }
  };

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
    setSelectedNode(null);
    setSelectedLink(null);
    setRelayoutSeed((s) => s + 1);
  };

  return (
    <PageContainer
      title="网络拓扑"
      extra={[
        <Input.Search
          key="search"
          placeholder="搜索设备 / IP / 终端"
          allowClear
          style={{ width: 190 }}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          onSearch={handleSearch}
        />,
        <Select
          key="role"
          mode="multiple"
          allowClear
          placeholder="按角色过滤"
          style={{ minWidth: 130 }}
          maxTagCount={1}
          value={roleFilter}
          onChange={setRoleFilter}
          options={Object.entries(ROLE_META).map(([k, v]) => ({ label: v.label, value: k }))}
        />,
        <Button key="trace" icon={<CompassOutlined />} onClick={() => setTraceOpen(true)}>
          路径追踪
        </Button>,
        <Segmented
          key="layout"
          value={layoutType}
          onChange={(v) => setLayoutType(v as any)}
          options={[
            { label: '层次', value: 'dagre' },
            { label: '力导向', value: 'force' },
            { label: '同心圆', value: 'concentric' },
          ]}
        />,
        <Space key="ep-toggle" size={4}>
          <span style={{ fontSize: 12 }}>全部终端</span>
          <Switch size="small" checked={includeAllEndpoints} onChange={setIncludeAllEndpoints} />
        </Space>,
        <Button key="save" icon={<SaveOutlined />} onClick={handleSaveLayout}>
          保存布局
        </Button>,
        <Button key="reset" onClick={handleResetLayout}>
          重置
        </Button>,
        <Button key="png" icon={<CameraOutlined />} onClick={handleExportPng}>
          导出 PNG
        </Button>,
        <Button key="redraw" icon={<ApartmentOutlined />} onClick={() => setRelayoutSeed((s) => s + 1)}>
          重绘
        </Button>,
        <Button key="fit" icon={<ExpandOutlined />} onClick={() => graphRef.current?.fitView()} />,
        <Button key="rebuild" icon={<SyncOutlined />} loading={rebuilding} onClick={handleRebuild}>
          重建
        </Button>,
        <Button key="reload" type="primary" icon={<ReloadOutlined />} loading={loading} onClick={fetchTopology} />,
      ]}
    >
      {traceResult?.found && (
        <Alert
          type="info"
          showIcon
          closable
          onClose={() => setTraceResult(null)}
          style={{ marginBottom: 12 }}
          message={`路径追踪结果（${traceResult.message}）`}
          description={
            <Steps
              size="small"
              current={traceResult.hops.length}
              style={{ marginTop: 8 }}
              items={traceResult.hops.map((h) => ({
                title: h.name,
                description: [h.ip, h.inPort && `入:${shortPort(h.inPort)}`, h.outPort && `出:${shortPort(h.outPort)}`]
                  .filter(Boolean)
                  .join(' · '),
              }))}
            />
          }
        />
      )}
      <ProCard
        styles={{
          body: {
            padding: 0,
            position: 'relative',
            height: 'calc(100vh - 185px)',
            minHeight: 560,
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
              悬停连线查看端口 · 双击设备展开终端并重绘
            </span>
          </Space>
        }
        title={
          <Space>
            <ApartmentOutlined />
            <span>
              节点 {topo.nodes.length} · 活跃链路 {topo.links.length}
            </span>
            <Badge
              status={sseConnected ? 'processing' : 'default'}
              text={
                <span style={{ fontSize: 12, color: sseConnected ? '#52c41a' : '#8c8c8c' }}>
                  {sseConnected ? '实时推送已连接' : '实时推送重连中'}
                </span>
              }
            />
          </Space>
        }
      >
        {topo.nodes.length === 0 ? (
          <div style={{ padding: '160px 0' }}>
            <Empty description="暂无拓扑数据，请先在「发现与凭据」中扫描网络" />
          </div>
        ) : (
          <div ref={containerRef} style={{ width: '100%', height: '100%' }} />
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

      <Modal
        title="路径追踪 (Path Trace)"
        open={traceOpen}
        onOk={handleTrace}
        onCancel={() => setTraceOpen(false)}
        okText="开始追踪"
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <div>
            <div style={{ marginBottom: 4 }}>源地址（终端 IP / MAC 或设备名/IP）</div>
            <Input
              placeholder="例如 10.255.10.101 或 f8:bc:12:11:22:01"
              value={traceSrc}
              onChange={(e) => setTraceSrc(e.target.value)}
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>目标地址（留空默认追踪至防火墙/核心交换机）</div>
            <Input
              placeholder="可选：目标 IP / MAC / 设备名"
              value={traceDst}
              onChange={(e) => setTraceDst(e.target.value)}
            />
          </div>
        </Space>
      </Modal>
    </PageContainer>
  );
};

export default TopologyPage;
