import { Tag } from 'antd';
import { TopoRow, TopoCol } from '../components/TopoDevice';
import { TOPO_ORDER, TOPO_COLORS } from '../utils/constants';

interface Props {
  data: any;
  groups: Record<string, any[]>;
  openDevice: (d: any) => void;
}

export default function ScreenMonitor({ data, groups, openDevice }: Props) {
  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 106px)' }}>
      {/* LEFT SIDEBAR */}
      <div style={{ width: 250, padding: '12px 10px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 10 }}>
        {/* Weather */}
        <div className="glass-panel" style={{ padding: 14 }}>
          <div className="screen-monitor-title">
            <span className="material-symbols-outlined" style={{ fontSize: 16 }}>cloud</span> 今日天气
          </div>
          {data.weather ? (
            <div style={{ textAlign: 'center', padding: '6px 0' }}>
              <div style={{ fontFamily: 'Inter', fontSize: 11, color: '#bfc8c8', marginBottom: 2 }}>
                <span className="material-symbols-outlined" style={{ fontSize: 12, verticalAlign: 'middle' }}>location_on</span> {data.weather.city}
              </div>
              <div style={{ fontFamily: 'Space Grotesk', fontSize: 36, fontWeight: 700, color: '#01daf3', lineHeight: 1.1 }}>
                {data.weather.temp}°<span style={{ fontSize: 18, color: '#bfc8c8' }}>C</span>
              </div>
              <div style={{ fontFamily: 'Manrope', fontSize: 12, color: '#d9e5e4', opacity: 0.8 }}>
                {data.weather.text} · 湿度 {data.weather.humidity}%
              </div>
              <div style={{ fontFamily: 'Inter', fontSize: 11, color: '#bfc8c8', marginTop: 2 }}>
                {data.weather.wind_dir} {data.weather.wind_scale}级
              </div>
            </div>
          ) : <div style={{ textAlign: 'center', padding: 16, color: '#5a7a9a' }}>暂无天气数据</div>}
        </div>

        {/* Tasks */}
        <div className="glass-panel" style={{ padding: 14, flex: 1 }}>
          <div className="screen-monitor-title">
            <span className="material-symbols-outlined" style={{ fontSize: 16 }}>schedule</span> 定时任务
          </div>
          {(data.tasks || []).length === 0 ? <div style={{ color: '#5a7a9a', textAlign: 'center', padding: 12 }}>暂无任务</div> : (
            (data.tasks || []).slice(0, 6).map((t: any, i: number) => (
              <div key={i} style={{ fontSize: 11, padding: '4px 0', borderBottom: '1px solid rgba(255,255,255,0.05)', display: 'flex', alignItems: 'center', gap: 6 }}>
                <span className="status-dot status-dot--online" />
                <span style={{ color: '#01daf3', fontFamily: 'Space Grotesk', fontSize: 12, fontWeight: 600 }}>{t.time}</span>
                <span style={{ flex: 1, color: '#d9e5e4' }}>{t.device}</span>
                <Tag color={t.enabled ? 'green' : 'default'} style={{ fontSize: 10 }}>{t.enabled ? '启用' : '停用'}</Tag>
              </div>
            ))
          )}
        </div>

        {/* Alarms */}
        <div className="glass-panel" style={{ padding: 14, flex: 1, borderColor: data.alarms?.length > 0 ? 'rgba(255,77,79,0.3)' : undefined }}>
          <div className="screen-monitor-title" style={{ color: '#ffb4ab' }}>
            <span className="material-symbols-outlined" style={{ fontSize: 16 }}>warning</span> 故障报警
          </div>
          {(data.alarms || []).length === 0 ? <div style={{ color: '#5a7a9a', textAlign: 'center', padding: 12 }}>无告警</div> : (
            (data.alarms || []).slice(0, 8).map((a: any, i: number) => (
              <div key={i} style={{ fontSize: 11, padding: '4px 0', borderBottom: '1px solid rgba(255,255,255,0.05)', display: 'flex', alignItems: 'center', gap: 6 }}>
                <span className={`status-dot ${a.level === 'critical' ? 'status-dot--fault' : 'status-dot--warning'}`} />
                <Tag color={a.level === 'critical' ? 'red' : 'orange'} style={{ fontSize: 10 }}>{a.level === 'critical' ? '严重' : '警告'}</Tag>
                <span style={{ flex: 1, fontSize: 11, color: '#d9e5e4' }}>{a.device} {a.msg?.slice(0, 16)}</span>
                <span style={{ color: '#899392', fontSize: 10 }}>{a.time}</span>
              </div>
            ))
          )}
        </div>
      </div>

      {/* CENTER — Topology */}
      <div style={{ flex: 1, padding: '16px 12px', overflowY: 'auto', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', position: 'relative' }}>
        {TOPO_ORDER.some(t => groups[t]?.length) ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 30, maxWidth: 820, width: '100%' }}>
            <div className="stagger-1" style={{ zIndex: 1 }}>
              <TopoRow items={groups['冷却塔']} color={TOPO_COLORS['冷却塔']} onOpen={openDevice} gap={16} />
            </div>
            <div className="stagger-2" style={{ display: 'flex', alignItems: 'flex-start', gap: 48, zIndex: 1 }}>
              <TopoCol items={groups['冷却泵']} color={TOPO_COLORS['冷却泵']} onOpen={openDevice} gap={36} />
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 36 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <div style={{ alignSelf: 'center', marginTop: 33 }}>
                    <TopoCol items={(groups['阀门'] || []).slice(0, 1)} color={TOPO_COLORS['阀门']} onOpen={openDevice} size="small" />
                  </div>
                  <TopoCol items={(groups['主机'] || []).slice(0, 1)} color={TOPO_COLORS['主机']} onOpen={openDevice} size="large" />
                  <div style={{ alignSelf: 'center', marginTop: 33 }}>
                    <TopoCol items={(groups['阀门'] || []).slice(2, 3)} color={TOPO_COLORS['阀门']} onOpen={openDevice} size="small" />
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <div style={{ alignSelf: 'center', marginTop: 33 }}>
                    <TopoCol items={(groups['阀门'] || []).slice(1, 2)} color={TOPO_COLORS['阀门']} onOpen={openDevice} size="small" />
                  </div>
                  <TopoCol items={(groups['主机'] || []).slice(1, 2)} color={TOPO_COLORS['主机']} onOpen={openDevice} size="large" />
                  <div style={{ alignSelf: 'center', marginTop: 33 }}>
                    <TopoCol items={(groups['阀门'] || []).slice(3, 4)} color={TOPO_COLORS['阀门']} onOpen={openDevice} size="small" />
                  </div>
                </div>
              </div>
              <TopoCol items={groups['冷冻泵']} color={TOPO_COLORS['冷冻泵']} onOpen={openDevice} gap={36} />
            </div>
            <div className="stagger-3" style={{ marginTop: 24, zIndex: 1 }}>
              <TopoRow items={groups['二次泵']} color={TOPO_COLORS['二次泵']} onOpen={openDevice} gap={16} />
            </div>
          </div>
        ) : (
          <div style={{ color: '#5a7a9a', textAlign: 'center', padding: 40 }}>暂无设备数据，请选择项目和楼宇</div>
        )}
      </div>

      {/* RIGHT SIDEBAR */}
      <div style={{ width: 250, padding: '12px 10px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 10 }}>
        {/* Energy Overview */}
        <div className="glass-panel" style={{ padding: 14 }}>
          <div className="screen-monitor-title">
            <span className="material-symbols-outlined" style={{ fontSize: 16 }}>bolt</span> 能效概览
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', padding: '5px 0', fontSize: 12, color: '#bfc8c8' }}>
            <span>节能率</span><span style={{ color: '#52c41a', fontWeight: 700, fontFamily: 'Space Grotesk' }}>{((data.saving_rate || 0) * 100).toFixed(1)}%</span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', padding: '5px 0', fontSize: 12, color: '#bfc8c8' }}>
            <span>节电量</span><span style={{ color: '#01daf3', fontWeight: 700, fontFamily: 'Space Grotesk' }}>{(data.power_saved || 0).toFixed(1)} kWh</span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', padding: '5px 0', fontSize: 12, color: '#bfc8c8' }}>
            <span>节碳量</span><span style={{ color: '#13c2c2', fontWeight: 700, fontFamily: 'Space Grotesk' }}>{(data.carbon_saved || 0).toFixed(1)} kg</span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', padding: '5px 0', fontSize: 12, color: '#bfc8c8' }}>
            <span>运行时长</span><span style={{ color: '#fa8c16', fontWeight: 700, fontFamily: 'Space Grotesk' }}>{data.running_days || 0} 天</span>
          </div>
        </div>

        {/* Power Stats */}
        <div className="glass-panel" style={{ padding: 14, flex: 1 }}>
          <div className="screen-monitor-title">
            <span className="material-symbols-outlined" style={{ fontSize: 16 }}>electric_meter</span> 电能统计
          </div>
          {(data.meters || []).length === 0 ? <div style={{ color: '#5a7a9a', textAlign: 'center', padding: 12 }}>暂无电表</div> : (
            (data.meters || []).map((m: any, i: number) => (
              <div key={i} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0', fontSize: 11, color: '#bfc8c8' }}>
                <span>{m.name}</span><span style={{ color: '#01daf3', fontWeight: 600, fontFamily: 'Space Grotesk' }}>{(Number(m.power) || 0).toFixed(1)} kW</span>
              </div>
            ))
          )}
          <div style={{ height: 1, background: 'linear-gradient(90deg, transparent, #01daf333, transparent)', margin: '6px 0' }} />
          <div style={{ display: 'flex', justifyContent: 'space-between', padding: '5px 0', fontSize: 12 }}>
            <span style={{ fontWeight: 600 }}>总电能</span>
            <span style={{ color: '#01daf3', fontWeight: 700, fontFamily: 'Space Grotesk', fontSize: 16 }}>{(data.meter_power || 0).toFixed(1)} kW</span>
          </div>
        </div>
      </div>
    </div>
  );
}
