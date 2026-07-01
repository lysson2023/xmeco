// ===== Modbus / Register constants =====
export const READ_CODES = ['01', '02', '03', '04'] as const;
export const WRITE_CODES = ['05', '06', '10'] as const;
export const DATA_ORDERS = ['高位在前', '低位在前', '低字在前'] as const;
export const REG_COUNTS = [1, 2, 3, 4] as const;
export const DATA_TYPES = [
  '无符号16位整数', '有符号16位整数', '无符号32位整数', '有符号32位整数',
  '无符号16位小数', '有符号16位小数', '无符号32位小数', '有符号32位小数',
  '单精度浮点数',
] as const;
export const MAGNIFICATIONS = [0.0001, 0.001, 0.01, 0.1, 1, 10, 100, 1000] as const;

// ===== Device types =====
export const DEVICE_TYPES = ['主机', '冷冻泵', '冷却泵', '冷却塔', '阀门', '二次泵', '电表', '温湿度传感器', '其它'] as const;

// ===== Operation types =====
export const OP_TYPES = ['只读', '数值', '模式选择', '开关机'] as const;
export const OP_NUMERIC = '数值';
export const OP_SWITCH = '开关机';
export const OP_MODE = '模式选择';

// ===== Mode options (shared between Alarms and Screen) =====
export const MODE_OPTIONS = ['制冷', '制热', '制冷热水', '制热热水', '开机', '关机', '营业模式', '非营业模式'] as const;

// Screen device control modes (subset for quick control)
export const QUICK_MODES = ['制冷', '制热', '除湿', '送风'] as const;

// ===== Topology constants (Screen + DataCenter) =====
export const TOPO_ORDER = ['冷却塔', '冷却泵', '主机', '阀门', '冷冻泵', '二次泵'] as const;

export const TOPO_COLORS: Record<string, string> = {
  '冷却塔': '#0097a7', '冷却泵': '#52c41a', '主机': '#fa8c16',
  '阀门': '#722ed1', '冷冻泵': '#13c2c2', '二次泵': '#eb2f96',
};

export const DATA_ORDER = ['主机', '冷冻泵', '冷却泵', '冷却塔', '二次泵', '阀门', '电表', '温湿度传感器'] as const;

export const DATA_COLORS: Record<string, string> = {
  '主机': '#fa8c16', '冷冻泵': '#13c2c2', '冷却泵': '#52c41a', '冷却塔': '#0097a7',
  '二次泵': '#eb2f96', '阀门': '#722ed1', '电表': '#ffc107', '温湿度传感器': '#00bcd4',
};

export const CHART_COLORS = ['#00daf3', '#52c41a', '#fa8c16', '#ff4d4f', '#722ed1', '#eb2f96', '#13c2c2', '#0097a7'] as const;
