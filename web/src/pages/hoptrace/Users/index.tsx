import { PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  PageContainer,
  ProCard,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import { Button, message, Popconfirm, Table, Tag } from 'antd';
import React, { useEffect, useState } from 'react';
import { type AuthUser, createUser, deleteUser, listUsers } from '@/services/hoptrace';
import { fmtUnix } from '../constants';

const ROLE_TAG: Record<string, { label: string; color: string }> = {
  admin: { label: '管理员', color: 'red' },
  operator: { label: '运维工程师', color: 'blue' },
  viewer: { label: '只读审计', color: 'default' },
};

const UsersPage: React.FC = () => {
  const [users, setUsers] = useState<AuthUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const res = await listUsers();
      setUsers(res.items || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  return (
    <PageContainer
      title="用户与角色管理"
      extra={[
        <Button key="add" type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          新建用户
        </Button>,
      ]}
    >
      <ProCard>
        <Table
          rowKey="id"
          loading={loading}
          pagination={false}
          dataSource={users}
          columns={[
            { title: '用户名', dataIndex: 'username' },
            { title: '显示名称', dataIndex: 'displayName' },
            {
              title: '角色',
              dataIndex: 'role',
              render: (v: string) => {
                const m = ROLE_TAG[v] || { label: v, color: 'default' };
                return <Tag color={m.color}>{m.label}</Tag>;
              },
            },
            { title: '创建时间', dataIndex: 'createdAt', render: fmtUnix },
            {
              title: '操作',
              width: 100,
              render: (_, r) => (
                <Popconfirm
                  title={`确认删除用户 ${r.username}？`}
                  onConfirm={async () => {
                    await deleteUser(r.id);
                    message.success('已删除');
                    load();
                  }}
                >
                  <a style={{ color: '#ff4d4f' }}>删除</a>
                </Popconfirm>
              ),
            },
          ]}
        />
      </ProCard>

      <ModalForm
        title="新建用户"
        open={open}
        onOpenChange={setOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={{ role: 'operator' }}
        onFinish={async (vals: any) => {
          await createUser(vals);
          message.success('用户已创建');
          load();
          return true;
        }}
      >
        <ProFormText name="username" label="用户名" rules={[{ required: true }]} />
        <ProFormText name="displayName" label="显示名称" />
        <ProFormText.Password
          name="password"
          label="初始密码（至少 6 位）"
          rules={[{ required: true }, { min: 6, message: '密码至少 6 位' }]}
        />
        <ProFormSelect
          name="role"
          label="角色"
          rules={[{ required: true }]}
          options={[
            { label: '管理员 (admin) - 全部权限含用户管理', value: 'admin' },
            { label: '运维工程师 (operator) - 设备/拓扑/终端/发现读写', value: 'operator' },
            { label: '只读审计 (viewer) - 仅查看与检索', value: 'viewer' },
          ]}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default UsersPage;
