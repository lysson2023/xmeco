// ===== Core Entities =====

export interface Project {
  id: number;
  name: string;
  agent_id: number | null;
  address: string;
  admin_code: string;
  city_id: number | null;
  city_name: string;
  created_at: string;
}

export interface Building {
  id: number;
  project_id: number;
  name: string;
  outdoor_temp: number | null;
  outdoor_humidity: number | null;
  total_energy: number;
  save_rate: number;
  save_energy: number;
  carbon_rate: number;
  carbon_saving: number;
  created_at: string;
}

export interface Device {
  id: number;
  building_id: number;
  name: string;
  device_type: string;
  gateway_imei: string | null;
  gateway_type: string;
  node_address: number;
  device_no: number;
  ct_ratio: number;
  pt_ratio: number;
  power_sign: number;
  rated_voltage: number | null;
  rated_current: number | null;
  online_status: string;
  device_status: string;
  last_online_at: string | null;
  last_record_at: string | null;
  created_at: string;
}

export interface DeviceProperty {
  id: number;
  device_id: number;
  prop_name: string;
  prop_short: string;
  prop_value: string;
  unit: string;
  operation_type: string;
  is_key: boolean;
  prop_type: string;
  min_value: string;
  max_value: string;
  sort_order: number;
}

export interface Register {
  id: number;
  property_id: number;
  name: string;
  read_addr: number;
  read_code: string;
  write_addr: number | null;
  write_code: string | null;
  command_name: string | null;
  command_code: string;
  status_code: string | null;
  data_type: string;
  data_length: number;
  data_order: string;
  data_mask: string | null;
  magnification: number;
}

// ===== Alarm =====

export interface AlarmRule {
  id: number;
  name: string;
  device_id: number | null;
  property_id: number | null;
  device_type: string | null;
  metric: string | null;
  condition: string | null;
  threshold: number | null;
  level: string | null;
  target_value: string | null;
  min_value: string | null;
  max_value: string | null;
  notify_users: number[];
  enabled: boolean;
}

export interface AlarmLog {
  id: number;
  device_id: number;
  device_name: string;
  alarm_type: string;
  level: string;
  message: string;
  value: string;
  threshold: string;
  created_at: string | null;
  ack_at: string | null;
}

// ===== User & Auth =====

export interface User {
  id: number;
  username: string;
  role_id: number;
  role_code: string;
  role_name: string;
  agent_id: number | null;
  agent_name: string | null;
  default_project_id: number | null;
  is_active: boolean;
  last_login_at: string | null;
  created_at: string;
  remark: string | null;
}

export interface LoginUser {
  id: number;
  username: string;
  role_id: number;
  role_code: string;
  role_level: number;
  agent_id: number | null;
  default_project_id: number | null;
  permissions: string[];
}

export interface LoginResponse {
  token: string;
  user: LoginUser;
}

export interface Agent {
  id: number;
  name: string;
  created_at: string;
}

export interface Role {
  id: number;
  code: string;
  name: string;
  level: number;
  is_system: boolean;
}

export interface Permission {
  id: number;
  code: string;
  name: string;
  perm_group: string;
}

// ===== Startup / Scheduled Tasks =====

export interface StartupPlan {
  id: number;
  name: string;
  building_id: number;
  plan_type: string;
  precheck_online: boolean;
  precheck_alarm: boolean;
  enabled: boolean;
  steps: StartupStep[];
}

export interface StartupStep {
  id: number;
  device_id: number;
  device_name: string;
  sort_order: number;
  wait_seconds: number;
  retry_count: number;
  action: string;
}

export interface ScheduledTask {
  id: number;
  name: string;
  building_id: number;
  device_id: number;
  device_name: string;
  action_type: string;
  target_value: string | null;
  schedule_type: string;
  schedule_time: string;
  days_of_week: string | null;
  enabled: boolean;
  last_run_at: string | null;
  last_result: string | null;
  created_at: string;
}

// ===== Telemetry =====

export interface TelemetryPoint {
  ts: string;
  device_id: number;
  metric: string;
  value: number;
  unit: string;
}

// ===== Dashboard & Screen =====

export interface ScreenDevice {
  id: number;
  name: string;
  type: string;
  status: string;
  device_status: string;
  key_info: string;
}

// ===== City & Weather =====

export interface City {
  id: number;
  name: string;
  province: string;
  admin_code: string;
}

export interface WeatherNow {
  city_name: string;
  temp: string;
  feels_like: string;
  icon: string;
  weather_text: string;
  wind_dir: string;
  wind_scale: string;
  humidity: string;
  precip: string;
  pressure: string;
  fetched_at: string;
}

// ===== Intelligence =====

export interface EfficiencyItem {
  device_id: number;
  device_name: string;
  device_type: string;
  power_kw: number;
  load_pct: number;
  cop: number;
  efficiency: number;
  status: string;
}

export interface MeterInfo {
  id: number;
  name: string;
  building: string;
  project: string;
}
