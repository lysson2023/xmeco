import type { CSSProperties } from 'react';

interface DeviceData {
  id: number;
  name: string;
  key_info?: string;
  status?: string;
  device_status?: string;
  type?: string;
}

interface TopoDeviceProps {
  d: DeviceData;
  color: string;
  onOpen: (d: DeviceData) => void;
  size?: 'normal' | 'small' | 'large';
}

// States: 故障(红色闪烁) | 在线+开机(亮色实心) | 在线+关机(暗色空心) | 离线(灰色虚线)
function TopoDevice({ d, color, onOpen, size }: TopoDeviceProps) {
  const isOnline = d.status === '在线';
  const devStatus = d.device_status || '';
  const isFault = devStatus === '故障';
  const isOn = devStatus !== '关机' && devStatus !== '停机' && devStatus !== '';
  const isSmall = size === 'small';
  const isLarge = size === 'large';

  let bg: string;
  let border: string;
  let opacity: number;
  let anim: string;
  let nameColor: string;
  let boxShadow: string;
  let statusDotBg: string;
  let statusDotShadow: string;

  if (isFault) {
    bg = 'rgba(147,0,10,0.4)';
    border = '2px solid #ffb4ab';
    opacity = 1;
    anim = 'faultPulse 1.2s ease-in-out infinite';
    nameColor = '#fff';
    boxShadow = '0 0 14px rgba(255,180,171,0.4)';
    statusDotBg = '#ffb4ab';
    statusDotShadow = '0 0 6px #ffb4ab';
  } else if (isOnline && isOn) {
    bg = color || '#2a6767';
    border = '2px solid rgba(127,255,212,0.5)';
    opacity = 1;
    anim = '';
    nameColor = '#fff';
    boxShadow = '0 0 12px rgba(127,255,212,0.3), 0 4px 16px rgba(0,0,0,0.2)';
    statusDotBg = '#7fffd4';
    statusDotShadow = '0 0 6px rgba(127,255,212,0.6)';
  } else if (isOnline && !isOn) {
    bg = 'rgba(17,33,33,0.6)';
    border = '2px solid rgba(63,72,72,0.5)';
    opacity = 0.6;
    anim = '';
    nameColor = 'rgba(217,229,228,0.4)';
    boxShadow = 'none';
    statusDotBg = 'rgba(255,255,255,0.15)';
    statusDotShadow = 'none';
  } else if (!isOnline && isOn) {
    bg = 'rgba(17,33,33,0.4)';
    border = '2px dashed rgba(63,72,72,0.5)';
    opacity = 0.5;
    anim = '';
    nameColor = 'rgba(217,229,228,0.3)';
    boxShadow = 'none';
    statusDotBg = 'rgba(255,255,255,0.1)';
    statusDotShadow = 'none';
  } else {
    bg = '#222';
    border = '2px dashed #444';
    opacity = 0.4;
    anim = '';
    nameColor = '#444';
    boxShadow = 'none';
    statusDotBg = 'rgba(255,255,255,0.05)';
    statusDotShadow = 'none';
  }

  const boxW = isLarge ? 160 : isSmall ? 44 : 56;
  const boxH = isLarge ? 110 : isSmall ? 44 : 56;
  const fontSize = isLarge ? 14 : isSmall ? 9 : 10;
  const nameMax = isLarge ? 8 : isSmall ? 4 : 6;

  const style: CSSProperties = {
    width: boxW, height: boxH, borderRadius: 8, cursor: 'pointer',
    background: bg, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
    color: '#fff', fontSize, fontWeight: 600, border, opacity, boxShadow,
    animation: anim, transition: 'transform 0.15s',
  };

  return (
    <div
      title={d.key_info ? d.name + ': ' + d.key_info : '点击查看属性'}
      onClick={() => onOpen(d)}
      style={style}
      onMouseEnter={e => (e.currentTarget.style.transform = 'scale(1.05)')}
      onMouseLeave={e => (e.currentTarget.style.transform = 'scale(1)')}
    >
      {!isLarge && (
        <div style={{
          width: 6, height: 6, borderRadius: '50%',
          background: statusDotBg,
          boxShadow: statusDotShadow,
          marginBottom: 2,
        }} />
      )}
      <div style={{
        lineHeight: 1.2, textAlign: 'center', fontWeight: 700,
        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        maxWidth: boxW - 8, color: nameColor,
      }}>
        {d.name.length > nameMax ? d.name.slice(0, nameMax) + '\u2026' : d.name}
      </div>
      {isLarge && d.key_info && (() => {
        const lines = d.key_info.split(/\s*\|\s*/).filter(Boolean);
        return (
          <div style={{ fontSize: 11, color: 'rgba(255,255,255,0.85)', marginTop: 4, lineHeight: 1.5, textAlign: 'center', maxWidth: 150 }}>
            {lines.map((line: string, i: number) => <div key={i}>{line}</div>)}
          </div>
        );
      })()}
    </div>
  );
}

// TopoRow: horizontal row of devices
export function TopoRow({ items, color, onOpen, size, gap: rowGap }: {
  items: DeviceData[];
  color: string;
  onOpen: (d: DeviceData) => void;
  size?: 'normal' | 'small' | 'large';
  gap?: number;
}) {
  if (!items || items.length === 0) return null;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6 }}>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: rowGap ?? 4, justifyContent: 'center' }}>
        {items.map((d: DeviceData) => <TopoDevice key={d.id} d={d} color={color} onOpen={onOpen} size={size} />)}
      </div>
    </div>
  );
}

// TopoCol: vertical column of devices
export function TopoCol({ items, color, onOpen, size, gap: colGap }: {
  items: DeviceData[];
  color: string;
  onOpen: (d: DeviceData) => void;
  size?: 'normal' | 'small' | 'large';
  gap?: number;
}) {
  if (!items || items.length === 0) return null;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: colGap ?? 4, alignItems: 'center' }}>
        {items.map((d: DeviceData) => <TopoDevice key={d.id} d={d} color={color} onOpen={onOpen} size={size} />)}
      </div>
    </div>
  );
}
