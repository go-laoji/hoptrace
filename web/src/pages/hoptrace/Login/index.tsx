import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { history, useModel } from '@umijs/max';
import { message } from 'antd';
import React from 'react';
import { login, setToken } from '@/services/hoptrace';

const LoginPage: React.FC = () => {
  const { setInitialState } = useModel('@@initialState');

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(135deg, #f0f5ff 0%, #e6f4ff 100%)',
      }}
    >
      <div style={{ width: 400, background: '#fff', borderRadius: 12, padding: '24px 16px', boxShadow: '0 8px 24px rgba(0,0,0,0.08)' }}>
        <LoginForm
          logo="/logo.svg"
          title="HopTrace"
          subTitle="网络拓扑自动发现与终端管理平台"
          onFinish={async (vals: { username: string; password: string }) => {
            const res = await login(vals);
            setToken(res.token);
            setInitialState((s) => ({ ...s, authEnabled: true, currentUser: res.user }));
            message.success('登录成功');
            history.replace('/dashboard');
            return true;
          }}
        >
          <ProFormText
            name="username"
            fieldProps={{ size: 'large', prefix: <UserOutlined /> }}
            placeholder="用户名（默认 admin）"
            rules={[{ required: true, message: '请输入用户名' }]}
          />
          <ProFormText.Password
            name="password"
            fieldProps={{ size: 'large', prefix: <LockOutlined /> }}
            placeholder="密码"
            rules={[{ required: true, message: '请输入密码' }]}
          />
        </LoginForm>
      </div>
    </div>
  );
};

export default LoginPage;
