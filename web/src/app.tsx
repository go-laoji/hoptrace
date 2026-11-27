import { LogoutOutlined } from '@ant-design/icons';
import type { Settings as LayoutSettings } from '@ant-design/pro-components';
import type { RequestConfig, RunTimeLayoutConfig } from '@umijs/max';
import { history, Link } from '@umijs/max';
import { Dropdown, message } from 'antd';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import relativeTime from 'dayjs/plugin/relativeTime';
import React from 'react';
import { type AuthUser, clearToken, getCurrentUser } from '@/services/hoptrace';
import defaultSettings from '../config/defaultSettings';

dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

export async function getInitialState(): Promise<{
  settings?: Partial<LayoutSettings>;
  authEnabled?: boolean;
  currentUser?: AuthUser;
}> {
  try {
    const res = await getCurrentUser();
    if (res.authEnabled && !res.user && history.location.pathname !== '/login') {
      history.replace('/login');
    }
    return {
      authEnabled: res.authEnabled,
      currentUser: res.user,
      settings: defaultSettings as Partial<LayoutSettings>,
    };
  } catch {
    return {
      authEnabled: false,
      currentUser: { id: '0', username: 'local-admin', displayName: '网络运维', role: 'admin', createdAt: '0' },
      settings: defaultSettings as Partial<LayoutSettings>,
    };
  }
}

export const layout: RunTimeLayoutConfig = ({ initialState, setInitialState }) => {
  return {
    menuItemRender: (item, dom) => {
      if (item.path) {
        return <Link to={item.path}>{dom}</Link>;
      }
      return dom;
    },
    onPageChange: () => {
      if (initialState?.authEnabled && !initialState?.currentUser && history.location.pathname !== '/login') {
        history.replace('/login');
      }
    },
    avatarProps: {
      title: initialState?.currentUser?.displayName || initialState?.currentUser?.username || '网络运维',
      render: (_, dom) =>
        initialState?.authEnabled ? (
          <Dropdown
            menu={{
              items: [
                {
                  key: 'logout',
                  icon: <LogoutOutlined />,
                  label: '退出登录',
                  onClick: () => {
                    clearToken();
                    setInitialState((s) => ({ ...s, currentUser: undefined }));
                    history.replace('/login');
                  },
                },
              ],
            }}
          >
            {dom}
          </Dropdown>
        ) : (
          dom
        ),
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
      if (error?.response?.status === 401) {
        clearToken();
        if (history.location.pathname !== '/login') {
          history.replace('/login');
        }
        return;
      }
      const msg =
        error?.response?.data?.message ||
        error?.info?.message ||
        error?.message ||
        '请求失败';
      message.error(msg);
    },
  },
};
