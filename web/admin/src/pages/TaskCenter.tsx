import { useEffect, useState } from 'react';
import { Table, Select, DatePicker, Button, Tabs, message, Space, Popconfirm, Modal, Form, Input, Switch, TimePicker, Tag, Row, Col } from 'antd';
import { SearchOutlined, PlusOutlined, ClockCircleOutlined, UnorderedListOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import api from '../api/client';
import dayjs from 'dayjs';

export default function TaskCenter() {
  // -------------------- Common filters --------------------
  const [projects, setProjects] = useState<any[]>([]);
  const [allBuildings, setAllBuildings] = useState<any[]>([]);

  // === Execution Records ===
  const [exSelProject, setExSelProject] = useState<number | null>(null);
  const [exSelBuilding, setExSelBuilding] = useState<number | null>(null);
  const [exBuildings, setExBuildings] = useState<any[]>([]);
  const [exDevices, setExDevices] = useState<any[]>([]);
  const [exSelDevice, setExSelDevice] = useState<number | null>(null);
  const [dateRange, setDateRange] = useState<[any, any]>([dayjs().subtract(7, 'day'), dayjs()]);
  const [records, setRecords] = useState<any[]>([]);
  const [recLoading, setRecLoading] = useState(false);
  const [searchParams] = useSearchParams();

  // === Scheduled Tasks ===
  const [taskSelProject, setTaskSelProject] = useState<number | null>(null);
  const [taskSelBuilding, setTaskSelBuilding] = useState<number | null>(null);
  const [taskBuildings, setTaskBuildings] = useState<any[]>([]);
  const [taskDevices, setTaskDevices] = useState<any[]>([]);
  const [tasks, setTasks] = useState<any[]>([]);
  const [taskModalOpen, setTaskModalOpen] = useState(false);
  const [taskEditing, setTaskEditing] = useState<any>(null);
  const [taskForm] = Form.useForm();

  // ---- Init ----
  useEffect(() => {
    api.get('/projects').then(r => setProjects(r.data));
    api.get('/buildings').then(r => {
      setAllBuildings(r.data);
      // Restore from URL params
      const bid = searchParams.get('building_id');
      const pid = searchParams.get('project_id');
      if (pid) setExSelProject(Number(pid));
      if (bid) setExSelBuilding(Number(bid));
    });
  }, []);

  // ---- Execution Records cascade ----
  useEffect(() => {
    if (exSelProject) {
      setExBuildings(allBuildings.filter((b: any) => Number(b.project_id) === Number(exSelProject)));
      setExSelBuilding(null);
      setExSelDevice(null);
    } else {
      setExBuildings([]);
      setExSelBuilding(null);
      setExSelDevice(null);
    }
  }, [exSelProject]);
  useEffect(() => {
    if (exSelBuilding) {
      api.get('/devices?building_id=' + exSelBuilding).then(r => setExDevices(r.data));
      setExSelDevice(null);
    } else {
      setExDevices([]);
      setExSelDevice(null);
    }
  }, [exSelBuilding]);

  // ---- Scheduled Tasks cascade ----
  useEffect(() => {
    if (taskSelProject) {
      setTaskBuildings(allBuildings.filter((b: any) => Number(b.project_id) === Number(taskSelProject)));
      setTaskSelBuilding(null);
    } else {
      setTaskBuildings([]);
      setTaskSelBuilding(null);
    }
  }, [taskSelProject]);
  useEffect(() => {
    if (taskSelBuilding) {
      api.get('/scheduled-tasks?building_id=' + taskSelBuilding).then(r => setTasks(r.data));
      api.get('/devices?building_id=' + taskSelBuilding).then(r => setTaskDevices(r.data));
    } else {
      setTasks([]);
      setTaskDevices([]);
    }
  }, [taskSelBuilding]);

  // ---- Fetch execution records ----
  const fetchRecords = () => {
    if (!exSelBuilding) {
      message.warning('请先选择楼宇');
      return;
    }
    setRecLoading(true);
    let params = '?start=' + (dateRange[0] ? dateRange[0].format('YYYY-MM-DD') : '') +
      '&end=' + (dateRange[1] ? dateRange[1].format('YYYY-MM-DD') : '') +
      '&building_id=' + exSelBuilding;
    if (exSelDevice) params += '&device_id=' + exSelDevice;
    api.get('/logs/controls' + params)
      .then(r => { setRecords(r.data); setRecLoading(false); })
      .catch(() => { setRecLoading(false); message.error('控制记录加载失败'); });
  };

  // ---- Task CRUD ----
  const taskSave = async () => {
    try {
      const v = taskForm.getFieldsValue();
      if (!v.name || !taskSelBuilding || !v.device_id || !v.schedule_time) {
        message.warning('请填写必填项');
        return;
      }
      const timeStr = v.schedule_time.format('HH:mm:ss');
      const payload: any = {
        name: v.name, building_id: taskSelBuilding, device_id: v.device_id,
        action_type: v.action_type || 'startup', target_value: v.target_value || null,
        schedule_type: v.schedule_type || 'once', schedule_time: timeStr,
        days_of_week: v.days_of_week || null, enabled: v.enabled !== false,
      };
      if (taskEditing) {
        await api.put('/scheduled-tasks/' + taskEditing.id, payload);
        message.success('已更新');
      } else {
        await api.post('/scheduled-tasks', payload);
        message.success('已创建');
      }
      setTaskModalOpen(false);
      setTaskEditing(null);
      taskForm.resetFields();
      if (taskSelBuilding) api.get('/scheduled-tasks?building_id=' + taskSelBuilding).then(r => setTasks(r.data));
    } catch { message.error('保存失败'); }
  };
  const taskDel = async (id: number) => {
    try {
      await api.delete('/scheduled-tasks/' + id);
      message.success('已删除');
      if (taskSelBuilding) api.get('/scheduled-tasks?building_id=' + taskSelBuilding).then(r => setTasks(r.data));
    } catch { message.error('删除失败'); }
  };

  const actionLabel = (a: string) => {
    const m: any = { startup: '开机', shutdown: '关机', set_value: '设值', mode_change: '切换模式' };
    return m[a] || a;
  };
  const actionColor = (a: string) => {
    const m: any = { startup: 'green', shutdown: 'red', set_value: 'blue', mode_change: 'orange' };
    return m[a] || 'default';
  };
  const scheduleLabel = (s: string) => {
    const m: any = { once: '单次', daily: '每天', weekly: '每周' };
    return m[s] || s;
  };

  // ---- Execution Record Columns ----
  const recordCols = [
    { title: '时间', dataIndex: 'created_at', width: 170, render: (v: any) => v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-' },
    { title: '设备', dataIndex: 'device_name', width: 130 },
    { title: '属性', dataIndex: 'prop_name', width: 100 },
    { title: '操作值', dataIndex: 'control_value', width: 80 },
    { title: '操作人/来源', dataIndex: 'username', width: 120 },
    { title: '备注', dataIndex: 'user_remark', width: 120 },
  ];

  // ---- Task Columns ----
  const taskCols = [
    { title: 'ID', dataIndex: 'id', width: 40 },
    { title: '名称', dataIndex: 'name', width: 140 },
    { title: '设备', dataIndex: 'device_name', width: 120 },
    {
      title: '动作', dataIndex: 'action_type', width: 90,
      render: (v: string) => <Tag color={actionColor(v)}>{actionLabel(v)}</Tag>,
    },
    { title: '目标值', dataIndex: 'target_value', width: 80, render: (v: any) => v || '-' },
    {
      title: '周期', dataIndex: 'schedule_type', width: 70,
      render: (v: string) => scheduleLabel(v),
    },
    { title: '执行时间', dataIndex: 'schedule_time', width: 90 },
    {
      title: '启用', dataIndex: 'enabled', width: 60,
      render: (v: boolean, r: any) =>
        <Switch size="small" checked={v} onChange={async (c) => {
          try {
            await api.put('/scheduled-tasks/' + r.id, { enabled: c });
            setTasks(tasks.map(t => t.id === r.id ? { ...t, enabled: c } : t));
          } catch { message.error('操作失败'); }
        }} />,
    },
    {
      title: '上次结果', dataIndex: 'last_result', width: 80,
      render: (v: any) =>
        v ? <Tag color={v === 'success' ? 'green' : 'red'}>{v === 'success' ? '成功' : '失败'}</Tag> : '-',
    },
    {
      title: '操作', width: 100,
      render: (_: any, r: any) => (
        <Space size="small">
          <a onClick={() => {
            setTaskEditing(r);
            taskForm.setFieldsValue({ ...r, schedule_time: r.schedule_time ? dayjs(r.schedule_time, 'HH:mm:ss') : null });
            setTaskSelBuilding(r.building_id);
            setTaskModalOpen(true);
          }}>编辑</a>
          <Popconfirm title="确定?" onConfirm={() => taskDel(r.id)}>
            <a style={{ color: 'red' }}>删除</a>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // ---- Tab 1: 执行记录 ----
  const executionTab = (
    <div>
      <div style={{ display: 'flex', gap: 10, marginBottom: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <div>
          <div style={{ marginBottom: 2, color: '#666', fontSize: 11 }}>项目</div>
          <Select style={{ width: 180 }} placeholder="项目" allowClear value={exSelProject}
            onChange={v => setExSelProject(v ? Number(v) : null)}
            options={projects.map(p => ({ value: p.id, label: p.name }))} />
        </div>
        <div>
          <div style={{ marginBottom: 2, color: '#666', fontSize: 11 }}>楼宇</div>
          <Select style={{ width: 180 }} placeholder="楼宇" allowClear value={exSelBuilding}
            disabled={!exSelProject}
            onChange={v => setExSelBuilding(v ? Number(v) : null)}
            options={exBuildings.map(b => ({ value: b.id, label: b.name }))} />
        </div>
        <div>
          <div style={{ marginBottom: 2, color: '#666', fontSize: 11 }}>设备</div>
          <Select style={{ width: 160 }} placeholder="全部设备" allowClear value={exSelDevice}
            disabled={!exSelBuilding}
            onChange={v => setExSelDevice(v ? Number(v) : null)}
            options={exDevices.map(d => ({ value: d.id, label: d.name }))} />
        </div>
        <div>
          <div style={{ marginBottom: 2, color: '#666', fontSize: 11 }}>日期范围</div>
          <DatePicker.RangePicker size="middle" value={dateRange as any}
            onChange={(v: any) => setDateRange(v || [dayjs().subtract(7, 'day'), dayjs()])} />
        </div>
        <Button type="primary" icon={<SearchOutlined />} onClick={fetchRecords}>查询</Button>
        {exSelBuilding && <span style={{ paddingBottom: 2, color: '#006875', fontWeight: 500 }}>共{records.length}条记录</span>}
      </div>
      <Table rowKey={(r: any, i: any) => `${r.created_at ?? i}-${r.device_name ?? ''}-${i}`}
        columns={recordCols} dataSource={records} loading={recLoading}
        scroll={{ x: 800 }} size="small"
        locale={{ emptyText: exSelBuilding ? '暂无控制记录' : '请先选择楼宇，点击查询' }} />
    </div>
  );

  // ---- Tab 2: 定时任务 ----
  const taskTab = (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
        <span></span>
        <Button type="primary" icon={<PlusOutlined />} disabled={!taskSelBuilding}
          onClick={() => { setTaskEditing(null); taskForm.resetFields(); setTaskModalOpen(true); }}>新增</Button>
      </div>
      <div style={{ display: 'flex', gap: 10, marginBottom: 12, alignItems: 'flex-end' }}>
        <div>
          <div style={{ marginBottom: 2, color: '#666', fontSize: 11 }}>项目</div>
          <Select style={{ width: 180 }} placeholder="项目" allowClear value={taskSelProject}
            onChange={v => setTaskSelProject(v ? Number(v) : null)}
            options={projects.map(p => ({ value: p.id, label: p.name }))} />
        </div>
        <div>
          <div style={{ marginBottom: 2, color: '#666', fontSize: 11 }}>楼宇</div>
          <Select style={{ width: 180 }} placeholder="楼宇" allowClear value={taskSelBuilding}
            disabled={!taskSelProject}
            onChange={v => setTaskSelBuilding(v ? Number(v) : null)}
            options={taskBuildings.map(b => ({ value: b.id, label: b.name }))} />
        </div>
        {taskSelBuilding && <span style={{ paddingBottom: 2, color: '#006875', fontWeight: 500 }}>共{tasks.length}个任务</span>}
      </div>
      <Table rowKey="id" columns={taskCols} dataSource={tasks} scroll={{ x: 900 }} size="small"
        locale={{ emptyText: taskSelBuilding ? '暂无任务' : '请先选择项目和楼宇' }} />
      <Modal title={taskEditing ? '编辑任务' : '新增任务'} width={550} open={taskModalOpen}
        onOk={taskForm.submit} onCancel={() => { setTaskModalOpen(false); setTaskEditing(null); }}>
        <Form form={taskForm} layout="vertical" onFinish={taskSave}
          initialValues={{ schedule_type: 'daily', action_type: 'startup', enabled: true }}>
          <Form.Item name="name" label="任务名称" rules={[{ required: true }]}>
            <Input placeholder="例如：早8点开机" />
          </Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="device_id" label="选择设备" rules={[{ required: true }]}>
                <Select placeholder="选择设备"
                  options={taskDevices.map((d: any) => ({ value: d.id, label: d.name }))} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="action_type" label="动作类型" rules={[{ required: true }]}>
                <Select options={[
                  { value: 'startup', label: '开机' }, { value: 'shutdown', label: '关机' },
                  { value: 'set_value', label: '设定数值' }, { value: 'mode_change', label: '切换模式' },
                ]} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="target_value" label="目标值（设定数值/切换模式时填写）">
            <Input placeholder="例如：7°C 或 制冷模式" />
          </Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="schedule_type" label="执行周期">
                <Select options={[
                  { value: 'once', label: '单次' }, { value: 'daily', label: '每天' },
                  { value: 'weekly', label: '每周' },
                ]} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="schedule_time" label="执行时间" rules={[{ required: true }]}>
                <TimePicker format="HH:mm" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="days_of_week" label="每周哪几天（1-7）" extra="逗号分隔，1=周一">
                <Input placeholder="1,2,3,4,5" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="enabled" label="启用" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
    </div>
  );

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>
        <UnorderedListOutlined style={{ marginRight: 8, color: '#006875' }} />任务中心
      </h2>
      <Tabs defaultActiveKey="records" size="large" items={[
        {
          key: 'records',
          label: <span><UnorderedListOutlined /> 执行记录</span>,
          children: executionTab,
        },
        {
          key: 'scheduled',
          label: <span><ClockCircleOutlined /> 定时任务</span>,
          children: taskTab,
        },
      ]} />
    </div>
  );
}
