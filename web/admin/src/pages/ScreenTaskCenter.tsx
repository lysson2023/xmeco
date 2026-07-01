import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { Spin, Tag, Table, ConfigProvider, DatePicker, Button } from 'antd';
import {
  ScheduleOutlined, CheckCircleOutlined, CloseCircleOutlined,
  ClockCircleOutlined, UnorderedListOutlined, SearchOutlined,
  
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { api } from '../api/screenClient';

// ---- target device types for execution records ----
const TARGET_TYPES = ['主机', '冷冻泵', '冷却泵', '冷却塔', '阀门', '二次泵'];

// ---- dark theme ----
const darkCard: React.CSSProperties = {
  background: '#0d2525', borderRadius: 6, padding: 16, border: '1px solid #1a3535',
};

const darkTableTheme = {
  token: { colorBgContainer: '#0d2525', colorText: '#d9e5e4', colorBorderSecondary: '#1a3535', colorFillAlter: '#0a1515' },
  components: { Table: { headerBg: '#0a1515', headerColor: 'rgba(255,255,255,0.4)', rowHoverBg: '#152d2d', borderColor: '#1a3535' } },
};

const actionLabel: Record<string, string> = {
  startup: '开机', shutdown: '关机', set_value: '设值', mode_change: '切换模式', start: '开机', stop: '关机',
};
const actionColor: Record<string, string> = {
  startup: '#52c41a', shutdown: '#ff4d4f', set_value: '#1677ff', mode_change: '#fa8c16', start: '#52c41a', stop: '#ff4d4f',
};
const scheduleLabel: Record<string, string> = {
  once: '单次', daily: '每天', weekly: '每周',
};

// ---- deduplicate & humanize operation display ----
function opLabel(propName: string, ctrlVal: string, remark: string): { type: string; color: string } {
  // scheduled-task records use prop_name directly
  if (propName === '开机') return { type: '开机', color: '#52c41a' };
  if (propName === '关机') return { type: '关机', color: '#ff4d4f' };
  // device-control & orchestrator records — control_value has Chinese or raw action
  if (ctrlVal === '开机' || ctrlVal === '关机' || ctrlVal === '重启' || ctrlVal === '暂停' || ctrlVal === '恢复')
    return { type: ctrlVal, color: ctrlVal === '开机' || ctrlVal === '重启' || ctrlVal === '恢复' ? '#52c41a' : '#ff4d4f' };
  // raw action codes in control_value
  const a = actionLabel[ctrlVal];
  if (a) return { type: a, color: actionColor[ctrlVal] || 'rgba(255,255,255,0.4)' };
  // fallback: use remark or prop_name
  const raw = remark || propName;
  const al = actionLabel[raw];
  return al ? { type: al, color: actionColor[raw] || 'rgba(255,255,255,0.4)' } : { type: raw || '控制', color: 'rgba(255,255,255,0.4)' };
}

function sourceLabel(username: string): string {
  if (username === 'orchestrator') return '启停编排';
  if (username === '定时任务') return '定时任务';
  if (username === 'unknown') return '手动操作';
  return username;
}

interface ScreenTaskCenterProps {
  bid: number;
  devices: any[];
}

const SUB_TABS = [
  { key: 'records', icon: <UnorderedListOutlined />, label: '执行记录' },
  { key: 'scheduled', icon: <ClockCircleOutlined />, label: '定时任务' },
];

export default function ScreenTaskCenter({ bid, devices }: ScreenTaskCenterProps) {
  const [subTab, setSubTab] = useState<'records' | 'scheduled'>('records');

  // ---- device id → type map ----
  const deviceTypeMap = useMemo(() => {
    const m: Record<number, string> = {};
    devices.forEach((d: any) => { m[d.id] = d.type || ''; });
    return m;
  }, [devices]);

  // ---- Execution Records ----
  const [records, setRecords] = useState<any[]>([]);
  const [recLoading, setRecLoading] = useState(false);
  const [dateRange, setDateRange] = useState<[any, any]>([dayjs().subtract(7, 'day'), dayjs()]);
  const [typeFilter, setTypeFilter] = useState<string>(''); // '' = all

  const fetchRecords = useCallback(async () => {
    if (!bid) { setRecords([]); return; }
    setRecLoading(true);
    try {
      const r = await api.get('/logs/controls', {
        params: {
          building_id: bid,
          start: dateRange[0]?.format('YYYY-MM-DD') || '',
          end: dateRange[1]?.format('YYYY-MM-DD') || '',
        },
      });
      setRecords(r.data || []);
    } catch {
      setRecords([]);
    } finally {
      setRecLoading(false);
    }
  }, [bid, dateRange]);

  useEffect(() => {
    if (subTab === 'records' && bid) fetchRecords();
  }, [subTab, bid, fetchRecords]);

  // filtered records by device type
  const filteredRecords = useMemo(() => {
    if (!typeFilter) return records;
    return records.filter((r: any) => deviceTypeMap[r.device_id] === typeFilter);
  }, [records, typeFilter, deviceTypeMap]);

  // available device types in current records
  const availableTypes = useMemo(() => {
    const types = new Set<string>();
    records.forEach((r: any) => {
      const t = deviceTypeMap[r.device_id];
      if (t && TARGET_TYPES.includes(t)) types.add(t);
    });
    return Array.from(types);
  }, [records, deviceTypeMap]);

  // ---- Scheduled Tasks ----
  const [tasks, setTasks] = useState<any[]>([]);
  const [taskLoading, setTaskLoading] = useState(false);
  const taskMountedRef = useRef(true);

  const fetchTasks = useCallback(async () => {
    if (!bid) { setTasks([]); return; }
    setTaskLoading(true);
    try {
      const r = await api.get('/scheduled-tasks', { params: { building_id: bid } });
      if (taskMountedRef.current) setTasks(r.data || []);
    } catch {
      if (taskMountedRef.current) setTasks([]);
    } finally {
      if (taskMountedRef.current) setTaskLoading(false);
    }
  }, [bid]);

  useEffect(() => {
    taskMountedRef.current = true;
    if (subTab === 'scheduled') fetchTasks();
    return () => { taskMountedRef.current = false; };
  }, [subTab, fetchTasks]);

  // Auto-refresh tasks every 10s regardless of active sub-tab
  useEffect(() => {
    if (!bid) return;
    const t = setInterval(fetchTasks, 10000);
    return () => clearInterval(t);
  }, [fetchTasks, bid]);

  // ---- record columns ----
  const recordCols = [
    {
      title: '时间', dataIndex: 'created_at', width: 160,
      render: (v: string) => <span style={{ color: 'rgba(255,255,255,0.4)' }}>{v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'}</span>,
    },
    {
      title: '设备', dataIndex: 'device_name', width: 120,
      render: (v: string) => <span style={{ color: '#d9e5e4' }}>{v}</span>,
    },
    {
      title: '类型', dataIndex: 'device_id', width: 70,
      render: (id: number) => {
        const t = deviceTypeMap[id];
        return t ? <Tag style={{ fontSize: 11 }}>{t}</Tag> : <span style={{ color: '#5a7575' }}>-</span>;
      },
    },
    {
      title: '操作', width: 80,
      render: (_: any, r: any) => {
        const op = opLabel(r.prop_name, r.control_value, r.user_remark);
        return <Tag color={op.color}>{op.type}</Tag>;
      },
    },
    {
      title: '操作值', dataIndex: 'control_value', width: 80,
      render: (v: string) => <span style={{ color: 'rgba(255,255,255,0.4)' }}>{v || '-'}</span>,
    },
    {
      title: '来源', dataIndex: 'username', width: 90,
      render: (v: string) => <span style={{ color: 'rgba(255,255,255,0.4)' }}>{sourceLabel(v)}</span>,
    },
  ];

  // ---- task columns ----
  const taskCols = [
    { title: '名称', dataIndex: 'name', width: 160, render: (v: string) => <span style={{ color: '#d9e5e4' }}>{v}</span> },
    { title: '设备', dataIndex: 'device_name', width: 120, render: (v: string) => <span style={{ color: 'rgba(255,255,255,0.4)' }}>{v}</span> },
    {
      title: '动作', dataIndex: 'action_type', width: 80,
      render: (v: string) => <Tag color={actionColor[v] || 'default'}>{actionLabel[v] || v}</Tag>,
    },
    { title: '目标值', dataIndex: 'target_value', width: 80, render: (v: any) => <span style={{ color: 'rgba(255,255,255,0.4)' }}>{v || '-'}</span> },
    {
      title: '周期', dataIndex: 'schedule_type', width: 60,
      render: (v: string) => <span style={{ color: 'rgba(255,255,255,0.4)' }}>{scheduleLabel[v] || v}</span>,
    },
    { title: '执行时间', dataIndex: 'schedule_time', width: 85, render: (v: string) => <span style={{ color: '#d9e5e4' }}>{v}</span> },
    {
      title: '状态', dataIndex: 'enabled', width: 55,
      render: (v: boolean) => v
        ? <Tag color="green" style={{ fontSize: 11 }}>启用</Tag>
        : <Tag color="default" style={{ fontSize: 11 }}>停用</Tag>,
    },
    {
      title: '结果', dataIndex: 'last_result', width: 75,
      render: (v: any) => v
        ? (v === 'success'
          ? <span style={{ color: '#52c41a' }}><CheckCircleOutlined /> 成功</span>
          : <span style={{ color: '#ff4d4f' }}><CloseCircleOutlined /> 失败</span>)
        : <span style={{ color: '#5a7575' }}>-</span>,
    },
  ];

  // ==================== Empty state (no building) ====================
  if (!bid) {
    return (
      <div style={{ padding: 16, height: 'calc(100vh - 112px)', overflowY: 'auto' }}>
        <div style={{ textAlign: 'center', padding: 80, color: '#5a7575' }}>
          <ClockCircleOutlined style={{ fontSize: 48, marginBottom: 12, display: 'block' }} />
          请先选择项目与楼宇
        </div>
      </div>
    );
  }

  return (
    <div style={{ padding: 16, height: 'calc(100vh - 112px)', overflowY: 'auto' }}>
      {/* Sub-tabs */}
      <div style={{ display: 'flex', background: '#0d2525', borderRadius: 6, marginBottom: 16, border: '1px solid #1a3535', overflow: 'hidden' }}>
        {SUB_TABS.map(t => (
          <div key={t.key} onClick={() => setSubTab(t.key as any)} style={{
            padding: '8px 20px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
            background: subTab === t.key ? '#152d2d' : 'transparent',
            color: subTab === t.key ? '#00daf3' : 'rgba(255,255,255,0.4)',
            borderBottom: subTab === t.key ? '2px solid #00daf3' : '2px solid transparent',
            fontWeight: subTab === t.key ? 700 : 400, fontSize: 13,
          }}>{t.icon} {t.label}</div>
        ))}
      </div>

      {/* ==================== EXECUTION RECORDS ==================== */}
      {subTab === 'records' && (
        <>
          <div style={{ display: 'flex', gap: 10, marginBottom: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
            <DatePicker.RangePicker
              size="small"
              value={dateRange as any}
              onChange={(v: any) => v && setDateRange(v)}
              style={{ background: '#0d2525', border: '1px solid #1a3535' }}
              format="YYYY-MM-DD"
            />
            {availableTypes.length > 0 && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <span style={{ color: 'rgba(255,255,255,0.4)', fontSize: 12 }}>设备类型:</span>
                {([''] as string[]).concat(availableTypes).map(t => (
                  <Tag key={t || 'all'} color={typeFilter === t ? '#00daf3' : undefined}
                    style={{ cursor: 'pointer', fontSize: 11 }}
                    onClick={() => setTypeFilter(t)}>
                    {t || '全部'}
                  </Tag>
                ))}
              </div>
            )}
            <Button size="small" icon={<SearchOutlined />} onClick={fetchRecords}
              style={{ background: '#152d2d', border: '1px solid #1a3535', color: '#00daf3' }}>
              查询
            </Button>
            <span style={{ color: '#5a7575', fontSize: 12 }}>共{filteredRecords.length}条记录</span>
          </div>
          <div style={darkCard}>
            {recLoading ? (
              <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
            ) : filteredRecords.length === 0 ? (
              <div style={{ textAlign: 'center', padding: 40, color: '#5a7575' }}>
                暂无控制记录，请选择日期范围后点击查询
              </div>
            ) : (
              <ConfigProvider theme={darkTableTheme}>
                <Table
                  rowKey={(r: any, i: any) => `${r.created_at ?? i}-${r.device_name ?? ''}-${i}`}
                  columns={recordCols}
                  dataSource={filteredRecords}
                  size="small"
                  pagination={{ pageSize: 15, size: 'small' }}
                  scroll={{ x: 650 }}
                  locale={{ emptyText: '' }}
                />
              </ConfigProvider>
            )}
          </div>
        </>
      )}

      {/* ==================== SCHEDULED TASKS ==================== */}
      {subTab === 'scheduled' && (
        <>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
            <span style={{ color: '#5a7575', fontSize: 12 }}>共{tasks.length}个任务</span>
          </div>
          <div style={darkCard}>
            {taskLoading && tasks.length === 0 ? (
              <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
            ) : tasks.length === 0 ? (
              <div style={{ textAlign: 'center', padding: 40, color: '#5a7575' }}>
                <ScheduleOutlined style={{ fontSize: 32, marginBottom: 8, display: 'block' }} />
                暂无定时任务
              </div>
            ) : (
              <ConfigProvider theme={darkTableTheme}>
                <Table
                  rowKey="id"
                  columns={taskCols}
                  dataSource={tasks}
                  size="small"
                  pagination={{ pageSize: 15, size: 'small' }}
                  scroll={{ x: 750 }}
                  locale={{ emptyText: '' }}
                />
              </ConfigProvider>
            )}
          </div>
        </>
      )}
    </div>
  );
}
