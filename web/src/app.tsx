import type { Settings as LayoutSettings } from '@ant-design/pro-components';
import type { RequestConfig, RunTimeLayoutConfig } from '@umijs/max';
import { Link } from '@umijs/max';
import { message } from 'antd';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import relativeTime from 'dayjs/plugin/relativeTime';
import React from 'react';
import defaultSettings from '../config/defaultSettings';

dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

export async function getInitialState(): Promise<{
  settings?: Partial<LayoutSettings>;
  currentUser?: { name: string };
}> {
  return {
    currentUser: { name: '网络运维' },
    settings: defaultSettings as Partial<LayoutSettings>,
  };
}

export const layout: RunTimeLayoutConfig = ({ initialState }) => {
  return {
    menuItemRender: (item, dom) => {
      if (item.path) {
        return <Link to={item.path}>{dom}</Link>;
      }
      return dom;
    },
    avatarProps: {
      title: initialState?.currentUser?.name || '网络运维',
    },
    footerRender: () => (
      <div style={{ textAlign: 'center', padding: '12px 0', color: '#8c8c8c', fontSize: 12 }}>
        HopTrace · 网络拓扑自动发现与终端管理平台
      </div>
    ),
    ...initialState?.settings,
  };
};

export const request: RequestConfig = {
  baseURL: '',
  timeout: 30000,
  errorConfig: {
    errorHandler: (error: any, opts: any) => {
      if (opts?.skipErrorHandler) throw error;
      const msg =
        error?.response?.data?.message ||
        error?.info?.message ||
        error?.message ||
        '请求失败';
      message.error(msg);
    },
  },
};
