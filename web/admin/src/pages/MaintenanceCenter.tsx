import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { DatePicker, Spin, Empty, Tag, ConfigProvider, Table } from 'antd';
import {
  AlertOutlined, HistoryOutlined, ToolOutlined, EnvironmentOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { api } from '../api/screenClient';
import { DATA_ORDER } from '../utils/constants';

// ---- Shared dark-theme styles ----
const darkCard: React.CSSProperties = {
  background: '#0d1f3c', borderRadius: 6, padding: 12, border: '1px solid #1a3455',
};

const levelColor: Record<string, string> = { critical: '#ff4d4f', warning: '#fa8c16', info: '#00daf3' };
const levelLabel: Record<string, string> = { critical: '严重', warning: '警告', info: '提示' };

const statusColor: Record<string, string> = { '已完成': '#52c41a', '待处理': '#fa8c16', '处理中': '#1677ff' };

interface MaintenanceCenterProps {
  pid: number;
  bid: number;
  devices: any[];
}

export default function MaintenanceCenter({ pid, bid, devices }: MaintenanceCenterProps) {
  const [subTab, setSubTab] = useState('realtime-fault');

  // Build device_id -> type map
  const deviceTypeMap = useMemo(() => {
    const map: Record<number, string> = {};
    devices.forEach((d: any) => { map[d.id] = d.type; });
    return map;
  }, [devices]);

  return (
    <ConfigProvider theme={{ token: { colorPrimary: '#00daf3' } }}>
    <div style={{ height: 'calc(100vh - 112px)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      {/* Sub-tabs */}
      <div style={{ display: 'flex', background: '#0d1f3c', borderBottom: '1px solid #1a3455', paddingLeft: 16 }}>
        {[
          { key: 'realtime-fault', icon: <AlertOutlined />, label: '实时故障' },
          { key: 'history-fault', icon: <HistoryOutlined />, label: '历史故障' },
          { key: 'maintain-record', icon: <ToolOutlined />, label: '维保记录' },
        ].map(t => (
          <div key={t.key} onClick={() => setSubTab(t.key)} style={{
            padding: '10px 24px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
            background: subTab === t.key ? '#152d50' : 'transparent',
            color: subTab === t.key ? '#00daf3' : '#8ba0c0',
            borderBottom: subTab === t.key ? '2px solid #00daf3' : '2px solid transparent',
            fontWeight: subTab === t.key ? 700 : 400,
          }}>{t.icon} {t.label}</div>
        ))}
      </div>

      {/* Content */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {subTab === 'realtime-fault'
          ? <RealtimeFaultPanel pid={pid} bid={bid} deviceTypeMap={deviceTypeMap} />
          : subTab === 'history-fault'
          ? <HistoryFaultPanel pid={pid} bid={bid} deviceTypeMap={deviceTypeMap} />
          : <MaintainRecordPanel pid={pid} bid={bid} deviceTypeMap={deviceTypeMap} />
        }
      </div>
    </div>
    </ConfigProvider>
  );
}

// ======================== REALTIME FAULT PANEL ========================
function RealtimeFaultPanel({ pid, bid, deviceTypeMap }: { pid: number; bid: number; deviceTypeMap: Record<number, string> }) {
  const [alarms, setAlarms] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeType, setActiveType] = useState('全部');
  const mountedRef = useRef(true);

  const fetchAlarms = useCallback(async () => {
    if (!bid) return;
    setLoading(true);
    try {
      const params: any = { building_id: bid, today: '1' };
      if (pid) params.project_id = pid;
      const r = await api.get('/alarm-logs', { params });
      if (!mountedRef.current) return;
      setAlarms(r.data || []);
    } catch (err: any) {
      if (mountedRef.current && err?.response?.status !== 401) {
        console.error('加载实时故障失败:', err);
        setAlarms([]);
      }
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, [pid, bid]);

  useEffect(() => {
    mountedRef.current = true;
    fetchAlarms();
    const t = setInterval(fetchAlarms, 10000); // refresh every 10s
    return () => { mountedRef.current = false; clearInterval(t); };
  }, [fetchAlarms]);

  // Group alarms by device type
  const typeGroups = useMemo(() => {
    const groups: Record<string, any[]> = { '全部': alarms };
    alarms.forEach((a: any) => {
      const dt = deviceTypeMap[a.device_id] || '其他';
      if (!groups[dt]) groups[dt] = [];
      groups[dt].push(a);
    });
    return groups;
  }, [alarms, deviceTypeMap]);

  // Compute available type tabs
  const typeTabs = useMemo(() => {
    const tabs = ['全部'];
    DATA_ORDER.forEach(t => { if (typeGroups[t]?.length) tabs.push(t); });
    // Add any types not in DATA_ORDER
    Object.keys(typeGroups).forEach(t => { if (t !== '全部' && !tabs.includes(t)) tabs.push(t); });
    return tabs;
  }, [typeGroups]);

  const validType = typeTabs.includes(activeType) ? activeType : '全部';
  const displayAlarms = typeGroups[validType] || [];

  if (!bid) {
    return <div style={{ textAlign: 'center', padding: 60, color: '#5a7a9a', fontSize: 15 }}>请先选择楼宇</div>;
  }

  return (
    <div style={{ padding: '8px 16px' }}>
      {/* Type tabs */}
      <div style={{ display: 'flex', borderBottom: '1px solid #1a3455', marginBottom: 8, flexWrap: 'wrap' }}>
        {typeTabs.map(t => {
          const count = (typeGroups[t] || []).length;
          return (
            <div key={t} onClick={() => setActiveType(t)} style={{
              padding: '8px 18px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
              color: validType === t ? '#fff' : '#8ba0c0',
              fontWeight: validType === t ? 700 : 400, fontSize: 13,
              borderBottom: validType === t ? '2px solid #00daf3' : '2px solid transparent',
              background: validType === t ? 'rgba(0,218,243,0.08)' : 'transparent',
            }}>{t}（{count}）</div>
          );
        })}
      </div>

      {/* Alarm cards */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}><Spin size="large" /></div>
      ) : displayAlarms.length === 0 ? (
        <Empty description={<span style={{ color: '#5a7a9a' }}>暂无故障记录</span>} style={{ marginTop: 40 }} />
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {displayAlarms.map((a: any) => (
            <div key={a.id} style={{ ...darkCard, display: 'flex', alignItems: 'center', gap: 16, borderLeft: `3px solid ${levelColor[a.level] || '#5a7a9a'}` }}>
              <Tag color={a.level === 'critical' ? 'red' : a.level === 'warning' ? 'orange' : 'blue'} style={{ fontSize: 11, margin: 0 }}>
                {levelLabel[a.level] || a.level || '未知'}
              </Tag>
              <div style={{ flex: 1 }}>
                <div style={{ color: '#c0d0e0', fontWeight: 600, fontSize: 13 }}>
                  <EnvironmentOutlined style={{ color: '#8ba0c0', marginRight: 4 }} />
                  {a.device_name || `设备${a.device_id}`}
                  <span style={{ color: '#5a7a9a', fontWeight: 400, marginLeft: 8 }}>{a.alarm_type}</span>
                </div>
                <div style={{ color: '#8ba0c0', fontSize: 12, marginTop: 2 }}>{a.message}</div>
                {a.value && (
                  <div style={{ color: '#5a7a9a', fontSize: 11, marginTop: 2 }}>
                    当前值: {a.value}{a.threshold ? ` / 阈值: ${a.threshold}` : ''}
                  </div>
                )}
              </div>
              <div style={{ textAlign: 'right', fontSize: 11, color: '#5a7a9a', whiteSpace: 'nowrap' }}>
                <div>{a.created_at ? dayjs(a.created_at).format('HH:mm:ss') : '-'}</div>
                {a.ack_at ? (
                  <Tag color="green" style={{ fontSize: 10, marginTop: 2 }}>已确认</Tag>
                ) : (
                  <Tag color="red" style={{ fontSize: 10, marginTop: 2 }}>未确认</Tag>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ======================== HISTORY FAULT PANEL ========================
function HistoryFaultPanel({ pid, bid, deviceTypeMap }: { pid: number; bid: number; deviceTypeMap: Record<number, string> }) {
  const [alarms, setAlarms] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeType, setActiveType] = useState('全部');
  const [dateRange, setDateRange] = useState<[any, any]>([dayjs().startOf('day'), dayjs()]);
  const mountedRef = useRef(true);
  const fetchSeqRef = useRef(0);

  const fetchAlarms = useCallback(async () => {
    if (!bid) return;
    setLoading(true);
    const seq = ++fetchSeqRef.current;
    try {
      const params: any = { building_id: bid };
      if (pid) params.project_id = pid;
      if (dateRange[0]) params.date_from = dateRange[0].format('YYYY-MM-DD');
      if (dateRange[1]) params.date_to = dateRange[1].format('YYYY-MM-DD 23:59:59');
      const r = await api.get('/alarm-logs', { params });
      if (seq !== fetchSeqRef.current || !mountedRef.current) return;
      setAlarms(r.data || []);
    } catch (err: any) {
      if (seq !== fetchSeqRef.current || !mountedRef.current) return;
      if (err?.response?.status !== 401) {
        console.error('加载历史故障失败:', err);
        setAlarms([]);
      }
    } finally {
      if (seq === fetchSeqRef.current && mountedRef.current) setLoading(false);
    }
  }, [pid, bid, dateRange]);

  useEffect(() => {
    mountedRef.current = true;
    fetchAlarms();
    return () => { mountedRef.current = false; };
  }, [fetchAlarms]);

  // Group alarms by device type
  const typeGroups = useMemo(() => {
    const groups: Record<string, any[]> = { '全部': alarms };
    alarms.forEach((a: any) => {
      const dt = deviceTypeMap[a.device_id] || '其他';
      if (!groups[dt]) groups[dt] = [];
      groups[dt].push(a);
    });
    return groups;
  }, [alarms, deviceTypeMap]);

  const typeTabs = useMemo(() => {
    const tabs = ['全部'];
    DATA_ORDER.forEach(t => { if (typeGroups[t]?.length) tabs.push(t); });
    Object.keys(typeGroups).forEach(t => { if (t !== '全部' && !tabs.includes(t)) tabs.push(t); });
    return tabs;
  }, [typeGroups]);

  const validType = typeTabs.includes(activeType) ? activeType : '全部';
  const displayAlarms = typeGroups[validType] || [];

  if (!bid) {
    return <div style={{ textAlign: 'center', padding: 60, color: '#5a7a9a', fontSize: 15 }}>请先选择楼宇</div>;
  }

  return (
    <div style={{ padding: '8px 16px' }}>
      {/* Controls */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 12, alignItems: 'center', flexWrap: 'wrap' }}>
        <div>
          <div style={{ color: '#8ba0c0', fontSize: 11, marginBottom: 2 }}>时间范围</div>
          <DatePicker.RangePicker
            value={dateRange as any}
            onChange={(v) => setDateRange(v as any)}
            format="YYYY-MM-DD"
            style={{ background: '#0d1f3c', border: '1px solid #1a3455' }}
          />
        </div>
        <span style={{ color: '#5a7a9a', fontSize: 12, paddingTop: 18 }}>
          共 {alarms.length} 条记录
        </span>
      </div>

      {/* Type tabs */}
      <div style={{ display: 'flex', borderBottom: '1px solid #1a3455', marginBottom: 8, flexWrap: 'wrap' }}>
        {typeTabs.map(t => {
          const count = (typeGroups[t] || []).length;
          return (
            <div key={t} onClick={() => setActiveType(t)} style={{
              padding: '8px 18px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
              color: validType === t ? '#fff' : '#8ba0c0',
              fontWeight: validType === t ? 700 : 400, fontSize: 13,
              borderBottom: validType === t ? '2px solid #00daf3' : '2px solid transparent',
              background: validType === t ? 'rgba(0,218,243,0.08)' : 'transparent',
            }}>{t}（{count}）</div>
          );
        })}
      </div>

      {/* Alarm cards */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}><Spin size="large" /></div>
      ) : displayAlarms.length === 0 ? (
        <Empty description={<span style={{ color: '#5a7a9a' }}>该时间段暂无故障记录</span>} style={{ marginTop: 40 }} />
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {displayAlarms.map((a: any) => (
            <div key={a.id} style={{ ...darkCard, display: 'flex', alignItems: 'center', gap: 16, borderLeft: `3px solid ${levelColor[a.level] || '#5a7a9a'}` }}>
              <Tag color={a.level === 'critical' ? 'red' : a.level === 'warning' ? 'orange' : 'blue'} style={{ fontSize: 11, margin: 0 }}>
                {levelLabel[a.level] || a.level || '未知'}
              </Tag>
              <div style={{ flex: 1 }}>
                <div style={{ color: '#c0d0e0', fontWeight: 600, fontSize: 13 }}>
                  <EnvironmentOutlined style={{ color: '#8ba0c0', marginRight: 4 }} />
                  {a.device_name || `设备${a.device_id}`}
                  <span style={{ color: '#5a7a9a', fontWeight: 400, marginLeft: 8 }}>{a.alarm_type}</span>
                </div>
                <div style={{ color: '#8ba0c0', fontSize: 12, marginTop: 2 }}>{a.message}</div>
                {a.value && (
                  <div style={{ color: '#5a7a9a', fontSize: 11, marginTop: 2 }}>
                    当前值: {a.value}{a.threshold ? ` / 阈值: ${a.threshold}` : ''}
                  </div>
                )}
              </div>
              <div style={{ textAlign: 'right', fontSize: 11, color: '#5a7a9a', whiteSpace: 'nowrap' }}>
                <div>{a.created_at ? dayjs(a.created_at).format('MM-DD HH:mm') : '-'}</div>
                {a.ack_at ? (
                  <Tag color="green" style={{ fontSize: 10, marginTop: 2 }}>已确认</Tag>
                ) : (
                  <Tag color="red" style={{ fontSize: 10, marginTop: 2 }}>未确认</Tag>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ======================== MAINTAIN RECORD PANEL ========================
function MaintainRecordPanel({ pid, bid, deviceTypeMap }: { pid: number; bid: number; deviceTypeMap: Record<number, string>; }) {
  const [records, setRecords] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeType, setActiveType] = useState('全部设备');
  const mountedRef = useRef(true);

  const fetchRecords = useCallback(async () => {
    if (!bid) return;
    setLoading(true);
    try {
      const params: any = { building_id: bid };
      if (pid) params.project_id = pid;
      const r = await api.get('/maintenance-records', { params });
      if (!mountedRef.current) return;
      setRecords(r.data || []);
    } catch (err: any) {
      if (mountedRef.current && err?.response?.status !== 401) {
        console.error('加载维保记录失败:', err);
        setRecords([]);
      }
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, [pid, bid]);

  useEffect(() => {
    mountedRef.current = true;
    fetchRecords();
    const t = setInterval(fetchRecords, 30000); // refresh every 30s
    return () => { mountedRef.current = false; clearInterval(t); };
  }, [fetchRecords]);

  // Group records by device type
  const typeGroups = useMemo(() => {
    const groups: Record<string, any[]> = { '全部设备': records };
    records.forEach((r: any) => {
      const dt = deviceTypeMap[r.device_id] || '其他';
      if (!groups[dt]) groups[dt] = [];
      groups[dt].push(r);
    });
    return groups;
  }, [records, deviceTypeMap]);

  const typeTabs = useMemo(() => {
    const tabs = ['全部设备'];
    DATA_ORDER.forEach(t => { if (typeGroups[t]?.length) tabs.push(t); });
    Object.keys(typeGroups).forEach(t => { if (t !== '全部设备' && !tabs.includes(t)) tabs.push(t); });
    return tabs;
  }, [typeGroups]);

  const validType = typeTabs.includes(activeType) ? activeType : '全部设备';
  const displayRecords = typeGroups[validType] || [];

  // Table columns
  const columns = useMemo(() => [
    { title: '设备', dataIndex: 'device_name', key: 'device_name', width: 100, render: (v: string) => <span style={{ color: '#c0d0e0' }}>{v || '-'}</span> },
    { title: '维保名称', dataIndex: 'name', key: 'name', width: 100, ellipsis: true, render: (v: string) => <span style={{ color: '#c0d0e0' }}>{v || '-'}</span> },
    { title: '类型', dataIndex: 'record_type', key: 'record_type', width: 70, render: (v: string) => <Tag color="blue" style={{ fontSize: 11 }}>{v || '维修'}</Tag> },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true, render: (v: string) => <span style={{ color: '#8ba0c0' }}>{v || '-'}</span> },
    { title: '维保公司', dataIndex: 'company', key: 'company', width: 100, ellipsis: true, render: (v: string) => <span style={{ color: '#8ba0c0' }}>{v || '-'}</span> },
    { title: '操作人', dataIndex: 'operator', key: 'operator', width: 80, render: (v: string) => <span style={{ color: '#8ba0c0' }}>{v || '-'}</span> },
    { title: '费用', dataIndex: 'cost', key: 'cost', width: 80, render: (v: number) => <span style={{ color: '#fa8c16' }}>{v ? `¥${v.toFixed(0)}` : '-'}</span> },
    { title: '状态', dataIndex: 'status', key: 'status', width: 80, render: (v: string) => <Tag color={statusColor[v] || 'default'} style={{ fontSize: 11 }}>{v || '-'}</Tag> },
    { title: '维修日期', dataIndex: 'record_date', key: 'record_date', width: 100, render: (v: string) => <span style={{ color: '#5a7a9a', fontSize: 12 }}>{v || '-'}</span> },
    { title: '下次维保', dataIndex: 'next_date', key: 'next_date', width: 100, render: (v: string | null) => {
      if (!v) return <span style={{ color: '#5a7a9a' }}>-</span>;
      const isOverdue = dayjs(v).isBefore(dayjs(), 'day');
      return <span style={{ color: isOverdue ? '#ff4d4f' : '#52c41a', fontSize: 12 }}>{v}{isOverdue ? ' ⚠' : ''}</span>;
    }},
    { title: '备注', dataIndex: 'remark', key: 'remark', width: 100, ellipsis: true, render: (v: string | null) => <span style={{ color: '#5a7a9a' }}>{v || '-'}</span> },
  ], []);

  if (!bid) {
    return <div style={{ textAlign: 'center', padding: 60, color: '#5a7a9a', fontSize: 15 }}>请先选择楼宇</div>;
  }

  return (
    <div style={{ padding: '8px 16px' }}>
      {/* Type tabs */}
      <div style={{ display: 'flex', borderBottom: '1px solid #1a3455', marginBottom: 12, flexWrap: 'wrap' }}>
        {typeTabs.map(t => {
          const count = (typeGroups[t] || []).length;
          return (
            <div key={t} onClick={() => setActiveType(t)} style={{
              padding: '8px 18px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
              color: validType === t ? '#fff' : '#8ba0c0',
              fontWeight: validType === t ? 700 : 400, fontSize: 13,
              borderBottom: validType === t ? '2px solid #00daf3' : '2px solid transparent',
              background: validType === t ? 'rgba(0,218,243,0.08)' : 'transparent',
            }}>{t}（{count}）</div>
          );
        })}
      </div>

      {/* Records table */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}><Spin size="large" /></div>
      ) : displayRecords.length === 0 ? (
        <Empty description={<span style={{ color: '#5a7a9a' }}>暂无维保记录</span>} style={{ marginTop: 40 }} />
      ) : (
        <Table
          dataSource={displayRecords}
          columns={columns}
          rowKey="id"
          size="small"
          pagination={{ pageSize: 20, showSizeChanger: false, showTotal: (t) => <span style={{ color: '#5a7a9a' }}>共 {t} 条</span> }}
          scroll={{ x: 1200 }}
          style={{ ...darkCard }}
          styles={{
            header: { background: '#0a1628' },
            body: { background: '#0d1f3c' },
          } as any}
          className="maintain-table"
        />
      )}
    </div>
  );
}
