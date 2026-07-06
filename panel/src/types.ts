import type { DataTableColumns } from 'naive-ui'

export interface NodeMetric {
  id: number
  node_id: string
  ts: number
  cpu_usage: number
  cpu_cores: number
  arch: string
  virtualization: string
  gpu: string
  os_name: string
  load1: number
  load5: number
  load15: number
  mem_total: number
  mem_used: number
  mem_used_percent: number
  swap_total: number
  swap_used: number
  swap_used_percent: number
  disk_total: number
  disk_used: number
  disk_used_percent: number
  net_rx_bps: number
  net_tx_bps: number
  net_rx_bytes_total: number
  net_tx_bytes_total: number
  uptime_seconds?: number
  created_at: string
}

export interface AgentIPAddresses {
  ipv4?: string[]
  ipv6?: string[]
}

export interface PublicIPObservation {
  ip: string
  source?: string
  first_seen?: number
  last_seen?: number
}

export interface PublicIPs {
  ipv4?: PublicIPObservation[]
  ipv6?: PublicIPObservation[]
}

export interface NodeRecord {
  id: number
  node_id: string
  name: string
  region: string
  provider: string
  network_line: string
  tag: string
  public_ip: string
  public_ipv6?: string
  public_ips_json?: string
  public_ips?: PublicIPs
  ip_addresses_json?: string
  ip_addresses?: AgentIPAddresses
  status: string
  agent_version: string
  last_seen_at: number | null
  heartbeat_interval: number
  metrics_interval: number
  snapshot_override: boolean
  snapshot_enabled: boolean
  snapshot_collect_processes: boolean
  snapshot_collect_connections: boolean
  snapshot_mask_sensitive: boolean
  snapshot_interval: number
  snapshot_process_limit: number
  snapshot_connection_limit: number
  billing_cycle: string
  price_amount: number
  currency: string
  service_started_at: number | null
  service_expires_at: number | null
  traffic_limit_bytes: number
  traffic_calibration_bytes: number
  traffic_used_bytes: number
  traffic_remaining_bytes: number
  traffic_billing_direction: TrafficBillingDirection
  traffic_reset_cycle: string
  probe_task_ids: number[]
  remaining_days: number
  remaining_value: number
  created_at: string
  updated_at: string
  latest_metric?: NodeMetric | null
}

export interface DashboardSparklinePoint {
  ts: number
  cpu_usage: number
  mem_used_percent: number
  disk_used_percent: number
  net_rx_bps: number
  net_tx_bps: number
}

export interface DashboardNodeProbeStat {
  samples: number
  success_samples: number
  failed_samples: number
  availability_percent?: number | null
  avg_latency_ms?: number | null
  packet_loss_percent?: number | null
}

export interface Summary {
  nodes_total: number
  nodes_online: number
  nodes_offline: number
  avg_latency_ms?: number | null
  availability_percent?: number | null
  probe_samples: number
  current_alerts: number
  node_sparklines: Record<string, DashboardSparklinePoint[]>
  node_probe_stats: Record<string, DashboardNodeProbeStat>
}

export interface AppSettings {
  show_home_summary: boolean
  show_billing_details: boolean
  show_traffic_plan: boolean
  show_node_tags: boolean
  mask_ip_addresses: boolean
  site_name: string
  site_description: string
  site_avatar_url: string
  user_avatar_url: string
  home_background_url: string
  active_theme: string
  admin_path: string
  snapshot_enabled: boolean
  snapshot_collect_processes: boolean
  snapshot_collect_connections: boolean
  snapshot_mask_sensitive: boolean
  snapshot_interval_seconds: number
  snapshot_process_limit: number
  snapshot_connection_limit: number
  metrics_retention_months: number
  asset_base_currency: string
  exchange_rate_auto_update: boolean
  wecom_webhook_enabled: boolean
  wecom_webhook_url: string
  telegram_enabled: boolean
  telegram_bot_token: string
  telegram_chat_id: string
  email_enabled: boolean
  email_smtp_host: string
  email_smtp_port: number
  email_smtp_security: string
  email_smtp_username: string
  email_smtp_password: string
  email_from: string
  email_to: string
  traffic_alert_enabled: boolean
  traffic_alert_percent: number
  cpu_alert_enabled: boolean
  cpu_alert_percent: number
  memory_alert_enabled: boolean
  memory_alert_percent: number
  disk_load_alert_enabled: boolean
  disk_load_alert_percent: number
  load_alert_enabled: boolean
  load_alert_threshold: number
  alert_interval_minutes: number
  offline_alert_delay_minutes: number
  expiry_alert_enabled: boolean
  expiry_alert_days: number
}

export interface AdminConfig {
  http: { listen_addr: string; admin_path: string }
  tcp: { listen_addr: string; secret_key_configured: boolean }
  database: { driver: string; dsn: string; auto_migrate: boolean }
  auth: { username: string }
  log: { level: string; file: string; retention_days: number }
}

export interface ThemeInfo {
  id: string
  name: string
  version: string
  description: string
  built_in: boolean
  active: boolean
}

export interface SystemLog {
  id: number
  service: string
  node_id: string
  level: string
  event_type: string
  message: string
  meta_json: string
  created_at: string
}

export interface ProbeTask {
  id: number
  name: string
  type: string
  ip_version: string
  target: string
  interval_seconds: number
  timeout_ms: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export type TrendKind = 'cpu' | 'memory' | 'disk' | 'network' | 'traffic'
export type TrafficBillingDirection = 'bidirectional' | 'outbound'

export type NodeColumns = DataTableColumns<NodeRecord>
export type LogColumns = DataTableColumns<SystemLog>
