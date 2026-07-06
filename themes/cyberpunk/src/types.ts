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
  status: string
  agent_version: string
  last_seen_at: number | null
  traffic_limit_bytes: number
  traffic_used_bytes: number
  traffic_remaining_bytes: number
  traffic_billing_direction: string
  traffic_reset_cycle: string
  remaining_days: number
  latest_metric?: NodeMetric | null
}

export interface DashboardNodeProbeStat {
  samples: number
  success_samples: number
  failed_samples: number
  availability_percent?: number | null
  avg_latency_ms?: number | null
  packet_loss_percent?: number | null
  latency_p50_ms?: number | null
  latency_p90_ms?: number | null
  jitter_ms?: number | null
  latency_baseline_ms?: number | null
  latency_spike_ratio?: number | null
}

export interface DashboardNodeHealthScore {
  score: number
  grade: string
  freshness_score: number
  resource_score: number
  load_score: number
  network_score: number
  stability_score: number
}

export interface Summary {
  nodes_total: number
  nodes_online: number
  nodes_offline: number
  cluster_health_score?: number | null
  avg_latency_ms?: number | null
  availability_percent?: number | null
  probe_samples: number
  current_alerts: number
  node_probe_stats: Record<string, DashboardNodeProbeStat>
  node_health_scores?: Record<string, DashboardNodeHealthScore>
}

export interface DashboardEventMetric {
  cpu_usage: number
  mem_used_percent: number
  disk_used_percent: number
  net_rx_bps: number
  net_tx_bps: number
}

export interface DashboardEvent {
  id: string
  event_type: string
  level: string
  node_id: string
  node_name: string
  message: string
  created_at: number
  metric?: DashboardEventMetric | null
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
}

export interface ProbeResultOverview {
  id: number
  task_id: number
  node_id: string
  task_name: string
  type: string
  ip_version: string
  target: string
  status: string
  latency_ms: number | null
  packet_loss: number | null
  created_at: string
  samples: number
  success_samples: number
  failed_samples: number
  min_latency_ms?: number | null
  max_latency_ms?: number | null
}

export interface ProbeResultsResponse {
  tasks: ProbeTask[]
  results: ProbeResultOverview[]
  generated_at: number
  range_anchor?: number
}

export interface AppSettings {
  site_name: string
  site_description: string
  mask_ip_addresses: boolean
}

export interface ServerTime {
  unix_ms: number
  time: string
  timezone: string
  timezone_abbreviation: string
  utc_offset: string
  offset_seconds: number
}

export interface CyberNode {
  id: string
  name: string
  location: string
  nodeSummary: string
  provider: string
  networkLine: string
  networkLines: string[]
  status: 'online' | 'warning' | 'offline'
  cpu: number
  memory: number
  disk: number
  load: string
  availability: string
  throughput: string
  trafficLimit: string
  trafficUsed: string
  trafficRemaining: string
  trafficUsagePercent: number
  trafficResetCycle: string
  trafficBillingDirection: string
  latency: string
  latencyClass: string
  asset: string
  lastSeenAt: number
  hasMetric: boolean
  uptimeSeconds: number
  rx: number
  tx: number
  memoryTotal: number
  memoryUsed: number
  diskTotal: number
  diskUsed: number
  packetLoss: number
  healthScore: number | null
  healthGrade: string
  freshnessScore: number | null
  resourceScore: number | null
  loadScore: number | null
  networkScore: number | null
  stabilityScore: number | null
  jitter: string
  latencySpike: string
}
