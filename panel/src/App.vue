<script setup lang="ts">
import { computed, h, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { ECharts, EChartsOption } from 'echarts'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import AdminDashboard from './components/AdminDashboard.vue'
import AppTopbar from './components/AppTopbar.vue'
import HomeDashboard from './components/HomeDashboard.vue'
import flagAE from 'flag-icons/flags/4x3/ae.svg?url'
import flagAU from 'flag-icons/flags/4x3/au.svg?url'
import flagBR from 'flag-icons/flags/4x3/br.svg?url'
import flagCA from 'flag-icons/flags/4x3/ca.svg?url'
import flagCN from 'flag-icons/flags/4x3/cn.svg?url'
import flagDE from 'flag-icons/flags/4x3/de.svg?url'
import flagFR from 'flag-icons/flags/4x3/fr.svg?url'
import flagGB from 'flag-icons/flags/4x3/gb.svg?url'
import flagHK from 'flag-icons/flags/4x3/hk.svg?url'
import flagID from 'flag-icons/flags/4x3/id.svg?url'
import flagIN from 'flag-icons/flags/4x3/in.svg?url'
import flagJP from 'flag-icons/flags/4x3/jp.svg?url'
import flagKR from 'flag-icons/flags/4x3/kr.svg?url'
import flagMY from 'flag-icons/flags/4x3/my.svg?url'
import flagNL from 'flag-icons/flags/4x3/nl.svg?url'
import flagPH from 'flag-icons/flags/4x3/ph.svg?url'
import flagRU from 'flag-icons/flags/4x3/ru.svg?url'
import flagSG from 'flag-icons/flags/4x3/sg.svg?url'
import flagTH from 'flag-icons/flags/4x3/th.svg?url'
import flagTR from 'flag-icons/flags/4x3/tr.svg?url'
import flagTW from 'flag-icons/flags/4x3/tw.svg?url'
import flagUS from 'flag-icons/flags/4x3/us.svg?url'
import flagVN from 'flag-icons/flags/4x3/vn.svg?url'
import {
  NButton,
  NConfigProvider,
  NDatePicker,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NSelect,
  NSpace,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  createDiscreteApi
} from 'naive-ui'

interface NodeMetric {
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

interface AgentIPAddresses {
  ipv4?: string[]
  ipv6?: string[]
}

interface PublicIPObservation {
  ip: string
  source?: string
  first_seen?: number
  last_seen?: number
}

interface PublicIPs {
  ipv4?: PublicIPObservation[]
  ipv6?: PublicIPObservation[]
}

interface NodeRecord {
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

interface Summary {
  nodes_total: number
  nodes_online: number
  nodes_offline: number
  avg_latency_ms?: number | null
  availability_percent?: number | null
  probe_samples: number
  current_alerts: number
  node_sparklines: Record<string, Array<{
    ts: number
    cpu_usage: number
    mem_used_percent: number
    disk_used_percent: number
    net_rx_bps: number
    net_tx_bps: number
  }>>
  node_probe_stats: Record<string, {
    samples: number
    success_samples: number
    failed_samples: number
    availability_percent?: number | null
    avg_latency_ms?: number | null
    packet_loss_percent?: number | null
  }>
}

interface LoginResponse {
  token: string
  user: {
    username: string
    display_name: string
  }
}

interface AdminConfig {
  http: { listen_addr: string; admin_path: string }
  tcp: { listen_addr: string; secret_key_configured: boolean }
  database: { driver: string; dsn: string; auto_migrate: boolean }
  auth: { username: string }
  log: { level: string; file: string; retention_days: number }
}

interface AppSettings {
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

interface ThemeInfo {
  id: string
  name: string
  version: string
  description: string
  built_in: boolean
  active: boolean
}

interface RegionOption {
  label: string
  value: string
}

interface ExchangeRatesResponse {
  base_currency: string
  rates: Record<string, number>
  source: string
  updated_at: number
}

interface SystemLog {
  id: number
  service: string
  node_id: string
  level: string
  event_type: string
  message: string
  meta_json: string
  created_at: string
}

interface ProbeTask {
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

interface ProbeResult {
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
  error_message: string
  created_at: string
  samples?: number
  success_samples?: number
  failed_samples?: number
  min_latency_ms?: number | null
  max_latency_ms?: number | null
  bucket_seconds?: number
  aggregated?: boolean
}

interface ProbeResultsResponse {
  tasks: ProbeTask[]
  results: ProbeResult[]
  generated_at: number
  range_anchor?: number
  aggregated?: boolean
  bucket_seconds?: number
}

interface NodeSnapshot {
  id: number
  node_id: string
  ts: number
  process_count: number
  thread_count: number
  connection_count: number
  listen_count: number
  tcp_state_json: string
  top_process_json: string
  connections_json: string
  created_at: string
}

interface SnapshotProcess {
  pid: number
  name: string
  user: string
  state: string
  cpu_percent: number
  memory_bytes: number
  thread_count: number
  command: string
}

interface SnapshotConnection {
  protocol: string
  local_addr: string
  local_port: number
  remote_addr: string
  remote_port: number
  state: string
  pid?: number
  process_name?: string
}

interface SnapshotResponse {
  snapshot: NodeSnapshot | null
  generated_at: number
}

interface ProbeNodeStat {
  key: string
  name: string
  target: string
  type: string
  ipVersion: string
  inactive: boolean
  latestLatency: number | null
  latestStatus: string
  averageLatency: number | null
  packetLoss: number | null
  jitter: number | null
  samples: number
  successSamples: number
  failedSamples: number
  minLatency: number | null
  maxLatency: number | null
}

interface MetricBucket {
  ts: number
  cpu_usage: number
  mem_total: number
  mem_used: number
  mem_used_percent: number
  disk_total: number
  disk_used: number
  disk_used_percent: number
  net_rx_bps: number
  net_tx_bps: number
  net_rx_bytes_total: number
  net_tx_bytes_total: number
}

type TrafficCounterKey = 'net_rx_bytes_total' | 'net_tx_bytes_total'

type TrafficCounterPoint = {
  net_rx_bytes_total: number
  net_tx_bytes_total: number
}

interface GapSegment {
  start: number
  end: number
  duration: number
}

interface NodeEditor {
  name: string
  region: string
  provider: string
  network_line: string[]
  tag: string
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
  service_range: [number, number] | null
  traffic_limit_value: number
  traffic_calibration_value: number
  traffic_limit_unit: TrafficUnit
  traffic_billing_direction: TrafficBillingDirection
  traffic_reset_cycle: string
  probe_task_ids: number[]
}

type ProbeTaskType = 'tcp_ping' | 'icmp'
type ProbeIPVersion = 'auto' | 'ipv4' | 'ipv6'

interface ProbeTaskEditor {
  id: number | null
  name: string
  type: ProbeTaskType
  ip_version: ProbeIPVersion
  target: string
  interval_seconds: number
  timeout_ms: number
  enabled: boolean
  assign_to_all_agents: boolean
}

type TrendKind = 'cpu' | 'memory' | 'disk' | 'network' | 'traffic'
type TrafficBillingDirection = 'bidirectional' | 'outbound'
type TrafficUnit = 'MB' | 'GB' | 'TB'
type ProbeRange = '1h' | '4h' | '12h' | '1d' | '7d' | '1m' | '3m' | '6m' | '1y'

interface ActiveTrend {
  kind: TrendKind
  node: NodeRecord
  x: number
  y: number
}

const appleFontFamily = '-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "PingFang SC", "Hiragino Sans GB", "Segoe UI", "Microsoft YaHei", sans-serif'

const themeOverrides = {
  common: {
    primaryColor: '#0f172a',
    primaryColorHover: '#0b8090',
    primaryColorPressed: '#0b5968',
    borderRadius: '18px',
    fontFamily: appleFontFamily
  }
}

const { message } = createDiscreteApi(['message'])

const summary = ref<Summary>({
  nodes_total: 0,
  nodes_online: 0,
  nodes_offline: 0,
  avg_latency_ms: null,
  availability_percent: null,
  probe_samples: 0,
  current_alerts: 0,
  node_sparklines: {},
  node_probe_stats: {}
})
const nodes = ref<NodeRecord[]>([])
const metrics = ref<NodeMetric[]>([])
const selectedNodeID = ref('')
const loading = ref(false)
const refreshedAt = ref<number | null>(null)
const currentTime = ref(Date.now())

const token = ref(window.localStorage.getItem('rivo_token') ?? '')
const currentUser = ref(window.localStorage.getItem('rivo_user') ?? '')
const loadedFromAdminPath = /^\/[A-Za-z0-9]{6,}(?:\/|$)/.test(window.location.pathname)
const initialViewMode: 'home' | 'admin' = loadedFromAdminPath ? 'admin' : 'home'
const viewMode = ref<'home' | 'admin'>(initialViewMode)
const loginOpen = ref(false)
const loginLoading = ref(false)
const loginError = ref('')
const loginForm = ref({ username: 'admin', password: '' })
const weComTestLoading = ref(false)
const telegramTestLoading = ref(false)
const emailTestLoading = ref(false)
const defaultSiteAvatarURL = '/rivo-logo.png'

const adminConfig = ref<AdminConfig | null>(null)
const appSettings = ref<AppSettings>(defaultAppSettings())
const settingsEditor = ref<AppSettings>(defaultAppSettings())
const regionOptions = ref<RegionOption[]>(fallbackRegionOptions())
const regionOptionsLoaded = ref(false)
const systemLogs = ref<SystemLog[]>([])
const logLevelFilter = ref<string | null>(null)
const logEventFilter = ref<string | null>(null)
const logNodeFilter = ref<string | null>(null)
const probeTasks = ref<ProbeTask[]>([])
const themes = ref<ThemeInfo[]>([])
const themesLoading = ref(false)
const adminLoading = ref(false)
const adminRefreshing = ref(false)
const adminEditNodeID = ref('')
const nodeEditor = ref<NodeEditor>(emptyNodeEditor())
const nodeEditOpen = ref(false)
const probeTaskEditor = ref<ProbeTaskEditor>(emptyProbeTaskEditor())
const probeTaskModalOpen = ref(false)
const probeSaving = ref(false)
const assetModalOpen = ref(false)
const assetListMode = ref<'all' | 'renewal'>('all')
const assetStatsLoading = ref(false)
const exchangeRatesLoading = ref(false)
const exchangeRatesError = ref('')
const exchangeRates = ref<Record<string, number>>(fallbackExchangeRates('CNY'))
const exchangeRatesSource = ref('fallback')

const trendChartEl = ref<HTMLDivElement | null>(null)
const probeChartEl = ref<HTMLDivElement | null>(null)
const metricsCPUChartEl = ref<HTMLDivElement | null>(null)
const metricsMemoryChartEl = ref<HTMLDivElement | null>(null)
const metricsDiskChartEl = ref<HTMLDivElement | null>(null)
const metricsNetworkChartEl = ref<HTMLDivElement | null>(null)
const metricsTrafficChartEl = ref<HTMLDivElement | null>(null)
const trendMetricsCache = ref<Record<string, NodeMetric[]>>({})
const activeTrend = ref<ActiveTrend | null>(null)
const activeProbeNode = ref<NodeRecord | null>(null)
const activeMetricsNode = ref<NodeRecord | null>(null)
const probePanelTasks = ref<ProbeTask[]>([])
const probeResults = ref<ProbeResult[]>([])
const probePanelOpen = ref(false)
const probeRange = ref<ProbeRange>('1h')
const probeRangeAnchor = ref(Date.now())
const probeRefreshCountdown = ref(5)
const showInactiveProbeHistory = ref(false)
const metricsPanelOpen = ref(false)
const metricsPanelMetrics = ref<NodeMetric[]>([])
const latestSnapshot = ref<NodeSnapshot | null>(null)
const metricsRange = ref<ProbeRange>('1h')
const metricsRangeAnchor = ref(Date.now())
const metricsRefreshCountdown = ref(15)
const trendLoading = ref(false)
const probeLoading = ref(false)
const metricsPanelLoading = ref(false)
let trendChart: ECharts | null = null
let probeChart: ECharts | null = null
let metricsCPUChart: ECharts | null = null
let metricsMemoryChart: ECharts | null = null
let metricsDiskChart: ECharts | null = null
let metricsNetworkChart: ECharts | null = null
let metricsTrafficChart: ECharts | null = null
let clockTimer: number | undefined
let refreshTimer: number | undefined
let probeRefreshCountdownTimer: number | undefined
let metricsRefreshCountdownTimer: number | undefined
let trendHideTimer: number | undefined
let trendRequestID = 0
let probeResultsRequestID = 0
let metricsPanelRequestID = 0
const trendRealtimeCooldownMs = 3000
const trendRealtimePollDelayMs = 550
const trendRealtimePollAttempts = 5
const trendWindowHours = 1
const trendWindowMs = trendWindowHours * 60 * 60 * 1000
const trendRealtimeRequestedAt = new Map<string, number>()
const trendRealtimeRequests = new Map<string, Promise<NodeMetric[]>>()

const isLoggedIn = computed(() => Boolean(token.value && currentUser.value))
const selectedNode = computed(() => nodes.value.find((node) => node.node_id === selectedNodeID.value) ?? null)
const latestMetric = computed(() => metrics.value[metrics.value.length - 1] ?? selectedNode.value?.latest_metric ?? null)
const regionCount = computed(() => new Set(nodes.value.map((node) => node.region).filter(Boolean)).size)
const totalCPUCores = computed(() => nodes.value.reduce((total, node) => total + (node.latest_metric?.cpu_cores ?? 0), 0))
const totalMemoryBytes = computed(() => nodes.value.reduce((total, node) => total + (node.latest_metric?.mem_total ?? 0), 0))
const totalDiskBytes = computed(() => nodes.value.reduce((total, node) => total + (node.latest_metric?.disk_total ?? 0), 0))
const totalRxBps = computed(() => nodes.value.reduce((total, node) => total + (isNodeOnline(node) ? node.latest_metric?.net_rx_bps ?? 0 : 0), 0))
const totalTxBps = computed(() => nodes.value.reduce((total, node) => total + (isNodeOnline(node) ? node.latest_metric?.net_tx_bps ?? 0 : 0), 0))
const totalTrafficLimitBytes = computed(() => nodes.value.reduce((total, node) => total + (node.traffic_limit_bytes ?? 0), 0))
const totalTrafficUsedBytes = computed(() => nodes.value.reduce((total, node) => total + (node.traffic_used_bytes ?? 0), 0))
const totalTrafficRemainingBytes = computed(() => nodes.value.reduce((total, node) => total + (node.traffic_remaining_bytes ?? 0), 0))
const assetBaseCurrency = computed(() => normalizeAssetCurrency(appSettings.value.asset_base_currency))
const assetRows = computed(() => nodes.value.map((node) => buildAssetRow(node)).sort((left, right) => left.remainingDays - right.remainingDays))
const nextMonthAssetRows = computed(() => assetRows.value.filter((item) => item.expiresInNextMonth))
const visibleAssetRows = computed(() => assetListMode.value === 'renewal' ? nextMonthAssetRows.value : assetRows.value)
const assetSummary = computed(() => {
  const rows = assetRows.value
  return {
    count: rows.length,
    annualTotal: rows.reduce((total, item) => total + item.annualCost, 0),
    monthlyTotal: rows.reduce((total, item) => total + item.monthlyCost, 0),
    remainingTotal: rows.reduce((total, item) => total + item.remainingValue, 0),
    nextMonthTotal: rows.reduce((total, item) => total + item.nextMonthCost, 0),
    nextMonthCount: rows.filter((item) => item.expiresInNextMonth).length
  }
})
const activeTrendMetrics = computed(() => {
  const nodeID = activeTrend.value?.node.node_id
  if (!nodeID) return []
  const minTs = Date.now() - trendWindowMs
  return (trendMetricsCache.value[nodeID] ?? []).filter((item) => item.ts >= minTs)
})
const activeTrendLatestMetric = computed(() => {
  if (!activeTrend.value) return null
  return activeTrendMetrics.value[activeTrendMetrics.value.length - 1] ?? activeTrend.value.node.latest_metric ?? null
})
const trendPanelStyle = computed(() => {
  const trend = activeTrend.value
  if (!trend) return { left: '24px', top: '72px' }

  const panelWidth = 430
  const panelHeight = 332
  const left = Math.min(trend.x + 18, Math.max(18, window.innerWidth - panelWidth - 18))
  const top = Math.min(trend.y + 18, Math.max(70, window.innerHeight - panelHeight - 18))
  return {
    left: `${left}px`,
    top: `${top}px`
  }
})
const trendTitle = computed(() => {
  if (!activeTrend.value) return ''
  return `${nodeLabel(activeTrend.value.node)} · ${trendKindLabel(activeTrend.value.kind)}趋势`
})
const trendLoadSummary = computed(() => activeTrend.value?.kind === 'cpu' ? formatLoadTitle(activeTrendLatestMetric.value) : '')
const adminNodes = computed(() => nodes.value)
const editingNode = computed(() => nodes.value.find((node) => node.node_id === adminEditNodeID.value) ?? null)
const filteredSystemLogs = computed(() => systemLogs.value.filter((item) => {
  if (logLevelFilter.value && normalizeSystemLogLevel(item.level) !== logLevelFilter.value) return false
  if (logEventFilter.value && item.event_type !== logEventFilter.value) return false
  if (logNodeFilter.value && item.node_id !== logNodeFilter.value) return false
  return true
}))
const logLevelOptions = computed(() => uniqueLogOptions(systemLogs.value.map((item) => normalizeSystemLogLevel(item.level)).filter(Boolean)))
const logEventOptions = computed(() => uniqueLogOptions(systemLogs.value.map((item) => item.event_type).filter(Boolean)))
const logNodeOptions = computed(() => uniqueLogOptions(systemLogs.value.map((item) => item.node_id).filter(Boolean)))
const enabledProbeTasks = computed(() => probeTasks.value.filter((task) => task.enabled))
const probeModalTitle = computed(() => {
  if (!activeProbeNode.value) return 'Ping 延迟图表'
  return `${nodeLabel(activeProbeNode.value)} · Ping 延迟图表${showInactiveProbeHistory.value ? ' · 停用' : ''}`
})
const probeTaskModalTitle = computed(() => (probeTaskEditor.value.id ? '编辑 Ping 节点' : '添加 Ping 节点'))
const siteName = computed(() => normalizeSiteName(appSettings.value.site_name))
const siteDescription = computed(() => normalizeSiteDescription(appSettings.value.site_description))
const siteAvatar = computed(() => normalizeImageURL(appSettings.value.site_avatar_url) || defaultSiteAvatarURL)
const userAvatar = computed(() => normalizeImageURL(appSettings.value.user_avatar_url))
const homeBackgroundURL = computed(() => normalizeImageURL(appSettings.value.home_background_url))
const homeBackgroundImage = computed(() => homeBackgroundURL.value ? cssImageURL(homeBackgroundURL.value) : '')
const siteInitial = computed(() => siteName.value.trim().slice(0, 1).toUpperCase() || 'R')
const activeThemeID = computed(() => appSettings.value.active_theme || 'default')
const activeProbePanelTaskCount = computed(() => probePanelTasks.value.filter((task) => task.enabled).length)
const probePanelInactive = computed(() => probePanelOpen.value && Boolean(activeProbeNode.value) && activeProbePanelTaskCount.value === 0)
const probeAutoRefreshEnabled = computed(() => probePanelOpen.value && Boolean(activeProbeNode.value) && !showInactiveProbeHistory.value && activeProbePanelTaskCount.value > 0)
const availableProbeRangeOptions = computed(() => probeRangeOptions.filter((item) => rangeRetentionMonths(item.value) <= normalizedMetricsRetentionMonths(appSettings.value.metrics_retention_months)))
const probeRangeLabel = computed(() => probeRangeOptions.find((item) => item.value === probeRange.value)?.summary ?? '最近 1 小时')
const rangeProbeResults = computed(() => filterProbeResultsForRange(probeResults.value, probeRange.value, probeRangeAnchor.value))
const visibleProbeResults = computed(() => {
  if (!showInactiveProbeHistory.value) return rangeProbeResults.value
  const activeTaskIDs = new Set(probePanelTasks.value.filter((task) => task.enabled).map((task) => task.id))
  return rangeProbeResults.value.filter((result) => !activeTaskIDs.has(result.task_id))
})
const visibleProbeTaskIDs = computed(() => new Set(visibleProbeResults.value.map((result) => result.task_id)))
const visibleProbeTasks = computed(() => {
  if (!showInactiveProbeHistory.value) return probePanelTasks.value.filter((task) => task.enabled)
  return probePanelTasks.value.filter((task) => !task.enabled && visibleProbeTaskIDs.value.has(task.id))
})
const probeVisibleSampleCount = computed(() => visibleProbeResults.value.reduce((total, item) => total + probeSampleCount(item), 0))
const probeRangeMeta = computed(() => `${probeRangeLabel.value} · 采样 ${probeVisibleSampleCount.value} 条 · ${formatRangeWindow(probeRange.value, probeRangeAnchor.value)} · 刷新 ${formatRangeWindowTime(probeRangeAnchor.value)}`)
const probeStats = computed(() => buildProbeStats(visibleProbeTasks.value, visibleProbeResults.value))
const metricsModalTitle = computed(() => {
  if (!activeMetricsNode.value) return '系统趋势图表'
  return `${nodeLabel(activeMetricsNode.value)} · 系统趋势图表`
})
const activeMetricsNodeOnline = computed(() => activeMetricsNode.value ? isNodeOnline(activeMetricsNode.value) : false)
const metricsRangeLabel = computed(() => probeRangeOptions.find((item) => item.value === metricsRange.value)?.summary ?? '最近 1 小时')
const visibleMetricsPanelMetrics = computed(() => filterMetricsForRange(metricsPanelMetrics.value, metricsRange.value, metricsRangeAnchor.value))
const metricsBuckets = computed(() => aggregateMetrics(visibleMetricsPanelMetrics.value, metricsRange.value, metricsRangeAnchor.value))
const metricsRangeMeta = computed(() => `${metricsRangeLabel.value} · 采样 ${visibleMetricsPanelMetrics.value.length} 条 · ${formatRangeWindow(metricsRange.value, metricsRangeAnchor.value)} · 刷新 ${formatRangeWindowTime(metricsRangeAnchor.value)}`)
const snapshotProcesses = computed(() => parseSnapshotJSON<SnapshotProcess[]>(latestSnapshot.value?.top_process_json, []))
const snapshotConnections = computed(() => parseSnapshotJSON<SnapshotConnection[]>(latestSnapshot.value?.connections_json, []))
const snapshotTCPStates = computed(() => parseSnapshotJSON<Record<string, number>>(latestSnapshot.value?.tcp_state_json, {}))
const snapshotTCPStateItems = computed(() => Object.entries(snapshotTCPStates.value).sort(([left], [right]) => left.localeCompare(right)))

const dropdownOptions = computed(() => [
  { label: '管理后台', key: 'admin' },
  { label: '首页总览', key: 'home' },
  { label: '退出登录', key: 'logout' }
])
const nodeColumns: DataTableColumns<NodeRecord> = [
  {
    title: '节点名称',
    key: 'name',
    width: 160,
    fixed: 'left',
    ellipsis: { tooltip: true },
    render: (row: NodeRecord) => h('button', {
      class: 'table-cell-link table-ellipsis-cell',
      title: nodeLabel(row),
      type: 'button',
      onClick: (event: MouseEvent) => {
        event.stopPropagation()
        openNodeEditor(row)
      }
    }, nodeLabel(row))
  },
  { title: 'Node ID', key: 'node_id', width: 150, ellipsis: { tooltip: true } },
  { title: 'Tag', key: 'tag', width: 110, render: (row: NodeRecord) => row.tag || '-' },
  {
    title: '状态',
    key: 'status',
    width: 92,
    render: (row: NodeRecord) => h(NTag, {
      type: row.status === 'online' ? 'success' : 'error',
      size: 'small',
      round: true
    }, { default: () => row.status })
  },
  {
    title: '公网 IP',
    key: 'public_ip',
    width: 170,
    ellipsis: { tooltip: true },
    render: (row: NodeRecord) => h('span', { class: 'table-ellipsis-cell', title: nodeIPListText(row) }, nodeIPSummary(row) || '-')
  },
  { title: '地区', key: 'region', width: 110, render: (row: NodeRecord) => displayRegion(row.region) },
  { title: '服务商', key: 'provider', width: 120, render: (row: NodeRecord) => row.provider || '-' },
  { title: '线路', key: 'network_line', width: 140, render: (row: NodeRecord) => displayNetworkLine(row.network_line) },
  { title: '心跳(s)', key: 'heartbeat_interval', width: 88 },
  { title: '指标(s)', key: 'metrics_interval', width: 88 },
  { title: '付费方式', key: 'billing_cycle', width: 96, render: (row: NodeRecord) => cycleLabel(row.billing_cycle) },
  { title: '金额', key: 'price_amount', width: 110, render: (row: NodeRecord) => formatMoney(row.price_amount, row.currency) },
  { title: '开始日期', key: 'service_started_at', width: 120, render: (row: NodeRecord) => formatDate(row.service_started_at) },
  { title: '结束日期', key: 'service_expires_at', width: 120, render: (row: NodeRecord) => formatDate(row.service_expires_at) },
  { title: '剩余时间', key: 'remaining_days', width: 100, render: (row: NodeRecord) => formatRemainingDays(row) },
  { title: '剩余价值', key: 'remaining_value', width: 110, render: (row: NodeRecord) => formatMoney(row.remaining_value, row.currency) },
  { title: '总流量', key: 'traffic_limit_bytes', width: 120, render: (row: NodeRecord) => formatTrafficPlan(row) },
  { title: '计费方向', key: 'traffic_billing_direction', width: 116, render: (row: NodeRecord) => trafficBillingDirectionLabel(row.traffic_billing_direction) },
  { title: '校准流量', key: 'traffic_calibration_bytes', width: 120, render: (row: NodeRecord) => formatBytes(row.traffic_calibration_bytes) },
  { title: '周期已用流量', key: 'traffic_used_bytes', width: 130, render: (row: NodeRecord) => formatBytes(row.traffic_used_bytes) },
  { title: '剩余流量', key: 'traffic_remaining_bytes', width: 120, render: (row: NodeRecord) => formatRemainingTraffic(row) },
  { title: '重置周期', key: 'traffic_reset_cycle', width: 110, render: (row: NodeRecord) => resetCycleLabel(row.traffic_reset_cycle) },
  { title: 'Agent', key: 'agent_version', width: 110, render: (row: NodeRecord) => row.agent_version || '-' },
  { title: '最后在线', key: 'last_seen_at', width: 120, render: (row: NodeRecord) => formatAgo(row.last_seen_at) },
  {
    title: '操作',
    key: 'actions',
    width: 112,
    align: 'center',
    render: (row: NodeRecord) => h(NButton, {
      size: 'small',
      tertiary: true,
      onClick: () => openNodeEditor(row)
    }, { default: () => '编辑' })
  }
]

const logColumns: DataTableColumns<SystemLog> = [
  {
    type: 'expand',
    expandable: (row) => hasSystemLogMeta(row),
    renderExpand: (row) => h('pre', { class: 'system-log-meta' }, formatSystemLogMeta(row))
  },
  { title: '时间', key: 'created_at', width: 190, render: (row) => formatLogTime(row.created_at) },
  { title: '来源', key: 'service', width: 88 },
  {
    title: '级别',
    key: 'level',
    width: 92,
    render: (row) => h(NTag, { size: 'small', type: systemLogLevelType(row.level), bordered: false }, { default: () => normalizeSystemLogLevel(row.level) })
  },
  {
    title: '事件',
    key: 'event_type',
    width: 170,
    ellipsis: { tooltip: true },
    render: (row) => h(NTag, { size: 'small', type: 'info', bordered: false }, { default: () => row.event_type || 'system.event' })
  },
  { title: 'Node', key: 'node_id', width: 150, ellipsis: { tooltip: true }, render: (row) => row.node_id || '-' },
  { title: 'Message', key: 'message', minWidth: 260, ellipsis: { tooltip: true } }
]

function defaultAppSettings(): AppSettings {
  return {
    show_home_summary: true,
    show_billing_details: true,
    show_traffic_plan: true,
    show_node_tags: true,
    mask_ip_addresses: false,
    site_name: 'Rivo Monitor',
    site_description: 'Private infrastructure monitor',
    site_avatar_url: defaultSiteAvatarURL,
    user_avatar_url: '',
    home_background_url: '',
    active_theme: 'default',
    admin_path: '',
    snapshot_enabled: false,
    snapshot_collect_processes: true,
    snapshot_collect_connections: true,
    snapshot_mask_sensitive: true,
    snapshot_interval_seconds: 60,
    snapshot_process_limit: 20,
    snapshot_connection_limit: 200,
    metrics_retention_months: 6,
    asset_base_currency: 'CNY',
    exchange_rate_auto_update: true,
    wecom_webhook_enabled: false,
    wecom_webhook_url: '',
    telegram_enabled: false,
    telegram_bot_token: '',
    telegram_chat_id: '',
    email_enabled: false,
    email_smtp_host: '',
    email_smtp_port: 587,
    email_smtp_security: 'starttls',
    email_smtp_username: '',
    email_smtp_password: '',
    email_from: '',
    email_to: '',
    traffic_alert_enabled: true,
    traffic_alert_percent: 80,
    cpu_alert_enabled: true,
    cpu_alert_percent: 85,
    memory_alert_enabled: true,
    memory_alert_percent: 85,
    disk_load_alert_enabled: true,
    disk_load_alert_percent: 90,
    load_alert_enabled: true,
    load_alert_threshold: 5,
    alert_interval_minutes: 30,
    offline_alert_delay_minutes: 1,
    expiry_alert_enabled: true,
    expiry_alert_days: 7
  }
}

const billingCycleOptions = [
  { label: '天付', value: 'daily' },
  { label: '月付', value: 'monthly' },
  { label: '年付', value: 'yearly' },
  { label: '一次性', value: 'one_time' }
]

const currencyOptions = [
  { label: '人民币 CNY', value: 'CNY' },
  { label: '美元 USD', value: 'USD' },
  { label: '欧元 EUR', value: 'EUR' },
  { label: '英镑 GBP', value: 'GBP' },
  { label: '港币 HKD', value: 'HKD' },
  { label: '日元 JPY', value: 'JPY' }
]

const emailSecurityOptions = [
  { label: 'STARTTLS（587）', value: 'starttls' },
  { label: 'SSL/TLS（465）', value: 'tls' },
  { label: '无加密', value: 'none' }
]

const networkLineOptions: Array<{ label: string; value: string }> = [
  { label: 'BGP', value: 'BGP' },
  { label: '国际 BGP', value: '国际 BGP' },
  { label: '精品 BGP', value: '精品 BGP' },
  { label: '普通国际', value: '普通国际' },
  { label: '163 / ChinaNet', value: '163' },
  { label: 'CN2 GT', value: 'CN2 GT' },
  { label: 'CN2 GIA', value: 'CN2 GIA' },
  { label: 'CN2 GIA-E', value: 'CN2 GIA-E' },
  { label: '4837 / CU169', value: '4837' },
  { label: '9929 / CUII', value: '9929' },
  { label: '10099 / CUG', value: '10099' },
  { label: 'CMI', value: 'CMI' },
  { label: 'CMIN2', value: 'CMIN2' },
  { label: 'AS4134', value: 'AS4134' },
  { label: 'AS4809', value: 'AS4809' },
  { label: 'AS4837', value: 'AS4837' },
  { label: 'AS9929', value: 'AS9929' },
  { label: 'AS10099', value: 'AS10099' },
  { label: 'IPLC', value: 'IPLC' },
  { label: 'IEPL', value: 'IEPL' },
  { label: 'NTT', value: 'NTT' },
  { label: 'IIJ', value: 'IIJ' },
  { label: 'KDDI', value: 'KDDI' },
  { label: 'SoftBank', value: 'SoftBank' },
  { label: 'PCCW', value: 'PCCW' },
  { label: 'HE', value: 'HE' },
  { label: 'Cogent', value: 'Cogent' },
  { label: 'Arelion / Telia', value: 'Arelion' },
  { label: 'GTT', value: 'GTT' },
  { label: 'Lumen', value: 'Lumen' },
  { label: 'Zayo', value: 'Zayo' },
  { label: 'RETN', value: 'RETN' },
  { label: 'Tata', value: 'Tata' },
  { label: 'Singtel', value: 'Singtel' },
  { label: 'Equinix', value: 'Equinix' },
  { label: 'Cloudflare', value: 'Cloudflare' },
  { label: 'AWS', value: 'AWS' },
  { label: 'Google Cloud', value: 'Google Cloud' },
  { label: 'Azure', value: 'Azure' },
  { label: 'Oracle Cloud', value: 'Oracle Cloud' },
  { label: '其他', value: '其他' }
]

const networkLineValueMap = new Map(networkLineOptions.map((item) => [item.value.toLowerCase(), item.value]))
const networkLineMenuProps = { class: 'network-line-select-menu' }
const networkLineTagPopoverProps = {
  showArrow: false,
  contentClass: 'network-line-tag-popover-content',
  themeOverrides: {
    color: 'rgba(15, 23, 42, 0.86)',
    textColor: 'rgba(245, 255, 252, 0.94)',
    boxShadow: '0 18px 44px rgba(2, 8, 23, 0.42), inset 0 1px 0 rgba(255, 255, 255, 0.16)',
    borderRadius: '14px',
    padding: '8px 10px'
  }
}

const assetBaseCurrencyOptions = [
  { label: '人民币 CNY', value: 'CNY' },
  { label: '美元 USD', value: 'USD' }
]

const trafficResetOptions = [
  { label: '每日', value: 'daily' },
  { label: '每月（开通日）', value: 'monthly' },
  { label: '每年（开通日）', value: 'yearly' },
  { label: '不重置', value: 'never' }
]

const trafficBillingDirectionOptions: Array<{ label: string; value: TrafficBillingDirection }> = [
  { label: '双向（上行 + 下行）', value: 'bidirectional' },
  { label: '单向（出站 / 上行）', value: 'outbound' }
]

const trafficUnitOptions: Array<{ label: TrafficUnit; value: TrafficUnit }> = [
  { label: 'MB', value: 'MB' },
  { label: 'GB', value: 'GB' },
  { label: 'TB', value: 'TB' }
]

const regionFlagURLs: Record<string, string> = {
  ae: flagAE,
  au: flagAU,
  br: flagBR,
  ca: flagCA,
  cn: flagCN,
  de: flagDE,
  fr: flagFR,
  gb: flagGB,
  hk: flagHK,
  id: flagID,
  in: flagIN,
  jp: flagJP,
  kr: flagKR,
  my: flagMY,
  nl: flagNL,
  ph: flagPH,
  ru: flagRU,
  sg: flagSG,
  th: flagTH,
  tr: flagTR,
  tw: flagTW,
  us: flagUS,
  vn: flagVN
}

function fallbackRegionOptions(): RegionOption[] {
  return [
    { label: '默认', value: 'default' },
    { label: '香港 HK', value: 'HK' },
    { label: '美国 US', value: 'US' },
    { label: '日本 JP', value: 'JP' },
    { label: '新加坡 SG', value: 'SG' },
    { label: '台湾 TW', value: 'TW' },
    { label: '韩国 KR', value: 'KR' },
    { label: '中国 CN', value: 'CN' },
    { label: '德国 DE', value: 'DE' },
    { label: '法国 FR', value: 'FR' },
    { label: '英国 GB', value: 'GB' },
    { label: '荷兰 NL', value: 'NL' },
    { label: '加拿大 CA', value: 'CA' },
    { label: '澳大利亚 AU', value: 'AU' },
    { label: '阿联酋 AE', value: 'AE' },
    { label: '泰国 TH', value: 'TH' },
    { label: '越南 VN', value: 'VN' },
    { label: '印度 IN', value: 'IN' },
    { label: '印尼 ID', value: 'ID' },
    { label: '马来西亚 MY', value: 'MY' },
    { label: '菲律宾 PH', value: 'PH' },
    { label: '巴西 BR', value: 'BR' },
    { label: '俄罗斯 RU', value: 'RU' },
    { label: '土耳其 TR', value: 'TR' }
  ]
}

const probeTypeOptions = [
  { label: 'TCP Ping', value: 'tcp_ping' },
  { label: 'ICMP', value: 'icmp' }
]
const probeTypeValues = probeTypeOptions.map((option) => option.value)
const probeIPVersionOptions = [
  { label: '自动', value: 'auto' },
  { label: 'IPv4', value: 'ipv4' },
  { label: 'IPv6', value: 'ipv6' }
]
const probeIPVersionValues = probeIPVersionOptions.map((option) => option.value)

const alertIntervalOptions = [
  { label: '5 分钟', value: 5 },
  { label: '10 分钟', value: 10 },
  { label: '30 分钟', value: 30 },
  { label: '1 小时', value: 60 },
  { label: '4 小时', value: 240 },
  { label: '12 小时', value: 720 },
  { label: '24 小时', value: 1440 }
]

const minuteMs = 60 * 1000
const hourMs = 60 * minuteMs
const dayMs = 24 * hourMs

declare const __RIVO_API_BASE_URL__: string

const axisSplitCount = 24
const apiBaseURL = String(__RIVO_API_BASE_URL__ ?? '').trim().replace(/\/+$/, '')

const probeRangeOptions: Array<{ label: string; value: ProbeRange; hours: number; bucketMs: number; summary: string; help: string }> = [
  { label: '1H', value: '1h', hours: 1, bucketMs: 2 * minuteMs, summary: '最近 1 小时 · 2 分钟聚合', help: '1H：最近 1 小时，2 分钟聚合，X 轴以刷新时间为最右刻度向前倒推' },
  { label: '4H', value: '4h', hours: 4, bucketMs: 10 * minuteMs, summary: '最近 4 小时 · 10 分钟聚合', help: '4H：最近 4 小时，10 分钟聚合，X 轴以刷新时间为最右刻度向前倒推' },
  { label: '12H', value: '12h', hours: 12, bucketMs: 30 * minuteMs, summary: '最近 12 小时 · 30 分钟聚合', help: '12H：最近 12 小时，30 分钟聚合，X 轴以刷新时间为最右刻度向前倒推' },
  { label: '1D', value: '1d', hours: 24, bucketMs: hourMs, summary: '最近 1 天 · 1 小时聚合', help: '1D：最近 24 小时，1 小时聚合，X 轴以刷新时间为最右刻度向前倒推' },
  { label: '7D', value: '7d', hours: 24 * 7, bucketMs: 6 * hourMs, summary: '最近 7 天 · 6 小时聚合', help: '7D：最近 7 天，6 小时聚合，X 轴以刷新日期为最右刻度向前倒推' },
  { label: '1M', value: '1m', hours: 24 * 30, bucketMs: 6 * hourMs, summary: '最近 1 个月 · 6 小时聚合', help: '1M：最近 30 天，6 小时聚合，X 轴以刷新日期为最右刻度向前倒推' },
  { label: '3M', value: '3m', hours: 24 * 90, bucketMs: 12 * hourMs, summary: '最近 3 个月 · 12 小时聚合', help: '3M：最近 90 天，12 小时聚合，X 轴以刷新日期为最右刻度向前倒推' },
  { label: '6M', value: '6m', hours: 24 * 180, bucketMs: dayMs, summary: '最近 6 个月 · 1 天聚合', help: '6M：最近 180 天，1 天聚合，X 轴以刷新日期为最右刻度向前倒推' },
  { label: '1Y', value: '1y', hours: 24 * 365, bucketMs: 2 * dayMs, summary: '最近 1 年 · 2 天聚合', help: '1Y：最近 365 天，2 天聚合，X 轴以刷新日期为最右刻度向前倒推' }
]

const metricsRetentionOptions = [
  { label: '保留 1 个月', value: 1 },
  { label: '保留 3 个月', value: 3 },
  { label: '保留 6 个月', value: 6 },
  { label: '保留 12 个月', value: 12 }
]

function resolveApiURL(url: string): string {
  if (!apiBaseURL || /^[a-z][a-z\d+\-.]*:/i.test(url)) return url
  const path = url.startsWith('/') ? url : `/${url}`
  if (apiBaseURL.endsWith('/api') && path === '/api') return apiBaseURL
  if (apiBaseURL.endsWith('/api') && path.startsWith('/api/')) {
    return `${apiBaseURL}${path.slice('/api'.length)}`
  }
  return `${apiBaseURL}${path}`
}

class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function requestHasAuthorization(init?: RequestInit): boolean {
  const headers = init?.headers
  if (!headers) return false
  if (headers instanceof Headers) return headers.has('Authorization')
  if (Array.isArray(headers)) {
    return headers.some(([name]) => String(name).toLowerCase() === 'authorization')
  }
  return Object.keys(headers).some((name) => name.toLowerCase() === 'authorization')
}

function clearExpiredLogin() {
  token.value = ''
  currentUser.value = ''
  latestSnapshot.value = null
  loginError.value = '登录已失效，请重新登录'
  loginOpen.value = true
  viewMode.value = 'admin'
  window.localStorage.removeItem('rivo_token')
  window.localStorage.removeItem('rivo_user')
  window.localStorage.removeItem('rivo_view_mode')
}

async function fetchJson<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(resolveApiURL(url), init)
  if (!response.ok) {
    let description = `${response.status} ${response.statusText}`
    try {
      const data = await response.clone().json() as { error?: string }
      if (data.error) description = data.error
    } catch {
      const text = await response.text()
      if (text) description = text
    }
    if (response.status === 401 && requestHasAuthorization(init)) {
      clearExpiredLogin()
      description = '登录已失效，请重新登录'
    }
    throw new ApiError(description, response.status)
  }
  return response.json() as Promise<T>
}

async function authFetch<T>(url: string): Promise<T> {
  return fetchJson<T>(url, {
    headers: {
      Authorization: `Bearer ${token.value}`
    }
  })
}

async function loadRegionOptions(force = false) {
  if (regionOptionsLoaded.value && !force) return
  try {
    const options = await fetchJson<RegionOption[]>('/api/region-options')
    if (options.length > 0) {
      regionOptions.value = options
      regionOptionsLoaded.value = true
    }
  } catch {
    if (!regionOptionsLoaded.value) {
      regionOptions.value = fallbackRegionOptions()
    }
  }
}

async function openAssetModal() {
  if (!isLoggedIn.value) return
  assetModalOpen.value = true
  assetListMode.value = 'all'
  exchangeRates.value = fallbackExchangeRates(assetBaseCurrency.value)
  await refreshExchangeRates()
}

async function refreshExchangeRates() {
  exchangeRatesLoading.value = true
  exchangeRatesError.value = ''
  try {
    const response = await fetchJson<ExchangeRatesResponse>('/api/exchange-rates')
    const base = normalizeAssetCurrency(response.base_currency || appSettings.value.asset_base_currency)
    appSettings.value = { ...appSettings.value, asset_base_currency: base }
    exchangeRates.value = { ...fallbackExchangeRates(base), ...response.rates }
    exchangeRatesSource.value = response.source || 'fallback'
  } catch (error) {
    exchangeRates.value = fallbackExchangeRates(assetBaseCurrency.value)
    exchangeRatesSource.value = 'fallback'
    exchangeRatesError.value = error instanceof Error ? error.message : '汇率获取失败'
  } finally {
    exchangeRatesLoading.value = false
  }
}

async function refreshAssetStats() {
  if (assetStatsLoading.value || exchangeRatesLoading.value) return
  assetStatsLoading.value = true
  try {
    await refreshAll()
    await refreshExchangeRates()
  } finally {
    assetStatsLoading.value = false
  }
}

async function refreshAll() {
  if (loading.value) return
  loading.value = true
  try {
    const [nextSettings, nextSummary, nextNodes] = await Promise.all([
      fetchJson<AppSettings>('/api/settings'),
      fetchJson<Summary>('/api/dashboard/summary'),
      fetchJson<NodeRecord[]>('/api/nodes')
    ])

    appSettings.value = nextSettings
    summary.value = nextSummary
    nodes.value = nextNodes
    if (activeProbeNode.value) {
      const nextActiveProbeNode = nextNodes.find((node) => node.node_id === activeProbeNode.value?.node_id)
      if (nextActiveProbeNode) {
        activeProbeNode.value = nextActiveProbeNode
      }
    }
    if (activeMetricsNode.value) {
      const nextActiveMetricsNode = nextNodes.find((node) => node.node_id === activeMetricsNode.value?.node_id)
      if (nextActiveMetricsNode) {
        activeMetricsNode.value = nextActiveMetricsNode
        if (metricsPanelOpen.value) {
          if (isNodeOnline(nextActiveMetricsNode)) {
            if (!metricsRefreshCountdownTimer) {
              startMetricsAutoRefresh()
            }
          } else {
            stopMetricsAutoRefresh()
          }
        }
      }
    }

    if (!nextNodes.some((node) => node.node_id === selectedNodeID.value)) {
      selectedNodeID.value = nextNodes[0]?.node_id ?? ''
    }
    if (selectedNodeID.value) {
      await loadMetrics(selectedNodeID.value)
    } else {
      metrics.value = []
    }
    if (!nextNodes.some((node) => node.node_id === adminEditNodeID.value)) {
      adminEditNodeID.value = ''
    }

    refreshedAt.value = Date.now()
  } finally {
    loading.value = false
  }
}

async function loadMetrics(nodeID: string) {
  const nextMetrics = await fetchNodeMetrics(nodeID)
  metrics.value = nextMetrics
  setTrendMetrics(nodeID, nextMetrics)
}

async function selectNode(nodeID: string) {
  selectedNodeID.value = nodeID
  await loadMetrics(nodeID)
}

async function ensureTrendMetrics(node: NodeRecord, realtime = false) {
  const nodeID = node.node_id
  const requestID = ++trendRequestID
  trendLoading.value = true
  try {
    const cached = trendMetricsCache.value[nodeID]
    if (realtime && isNodeOnline(node)) {
      try {
        const previousTs = Math.max(latestMetricTimestamp(cached), node.latest_metric?.ts ?? 0)
        await refreshTrendRealtimeMetrics(node, previousTs)
        return
      } catch (error) {
        console.warn('request realtime trend metrics failed', error)
      }
    }

    if (cached) return

    const nextMetrics = await fetchTrendMetrics(nodeID)
    setTrendMetrics(nodeID, nextMetrics)
  } finally {
    if (requestID === trendRequestID) {
      trendLoading.value = false
    }
  }
}

async function fetchNodeMetrics(nodeID: string) {
  return fetchJson<NodeMetric[]>(`/api/nodes/${encodeURIComponent(nodeID)}/metrics?limit=180`)
}

async function fetchTrendMetrics(nodeID: string) {
  return fetchJson<NodeMetric[]>(`/api/nodes/${encodeURIComponent(nodeID)}/metrics?hours=${trendWindowHours}&limit=720`)
}

function setTrendMetrics(nodeID: string, nextMetrics: NodeMetric[]) {
  trendMetricsCache.value = {
    ...trendMetricsCache.value,
    [nodeID]: nextMetrics
  }
  if (selectedNodeID.value === nodeID) {
    metrics.value = nextMetrics
  }
  const latest = nextMetrics[nextMetrics.length - 1]
  if (latest) {
    updateNodeLatestMetric(nodeID, latest)
  }
}

function updateNodeLatestMetric(nodeID: string, latest: NodeMetric) {
  nodes.value = nodes.value.map((node) => (
    node.node_id === nodeID ? { ...node, latest_metric: latest, last_seen_at: latest.ts } : node
  ))
  if (activeTrend.value?.node.node_id === nodeID) {
    activeTrend.value = {
      ...activeTrend.value,
      node: { ...activeTrend.value.node, latest_metric: latest, last_seen_at: latest.ts }
    }
  }
}

function latestMetricTimestamp(items?: NodeMetric[]) {
  if (!items?.length) return 0
  return items.reduce((latest, item) => Math.max(latest, item.ts || 0), 0)
}

async function refreshTrendRealtimeMetrics(node: NodeRecord, previousTs: number) {
  const nodeID = node.node_id
  const now = Date.now()
  const lastRequestedAt = trendRealtimeRequestedAt.get(nodeID) ?? 0
  const inFlight = trendRealtimeRequests.get(nodeID)
  if (inFlight) return inFlight
  if (now - lastRequestedAt < trendRealtimeCooldownMs && trendMetricsCache.value[nodeID]) {
    return trendMetricsCache.value[nodeID]
  }

  const request = (async () => {
    trendRealtimeRequestedAt.set(nodeID, now)
    await fetchJson<{ requested: boolean }>(`/api/nodes/${encodeURIComponent(nodeID)}/request-metrics`, {
      method: 'POST'
    })

    let nextMetrics = trendMetricsCache.value[nodeID] ?? []
    for (let attempt = 0; attempt < trendRealtimePollAttempts; attempt += 1) {
      await wait(trendRealtimePollDelayMs)
      nextMetrics = await fetchTrendMetrics(nodeID)
      setTrendMetrics(nodeID, nextMetrics)
      if (latestMetricTimestamp(nextMetrics) > previousTs) break
    }
    return nextMetrics
  })()

  trendRealtimeRequests.set(nodeID, request)
  try {
    return await request
  } finally {
    trendRealtimeRequests.delete(nodeID)
  }
}

function wait(ms: number) {
  return new Promise<void>((resolve) => {
    window.setTimeout(resolve, ms)
  })
}

async function submitLogin() {
  loginLoading.value = true
  loginError.value = ''
  try {
    const result = await fetchJson<LoginResponse>('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(loginForm.value)
    })

    token.value = result.token
    currentUser.value = result.user.display_name || result.user.username
    window.localStorage.setItem('rivo_token', result.token)
    window.localStorage.setItem('rivo_user', currentUser.value)
    window.localStorage.setItem('rivo_view_mode', 'admin')
    loginOpen.value = false
    viewMode.value = 'admin'
    await refreshAdminView()
    if (navigateToConfiguredAdmin()) return
  } catch (error) {
    loginError.value = error instanceof Error ? error.message : 'Login failed'
  } finally {
    loginLoading.value = false
  }
}

async function loadAdminData() {
  if (!token.value) return
  adminLoading.value = true
  adminRefreshing.value = true
  try {
    const [config, settings, logs, tasks, themeResponse, regions] = await Promise.all([
      authFetch<AdminConfig>('/api/admin/config'),
      authFetch<AppSettings>('/api/admin/settings'),
      authFetch<{ logs: SystemLog[] }>('/api/admin/system-logs?limit=100'),
      authFetch<ProbeTask[]>('/api/admin/probe-tasks'),
      authFetch<{ themes: ThemeInfo[] }>('/api/admin/themes'),
      fetchJson<RegionOption[]>('/api/region-options')
    ])
    adminConfig.value = config
    appSettings.value = settings
    settingsEditor.value = { ...settings }
    systemLogs.value = logs.logs
    probeTasks.value = tasks
    themes.value = themeResponse.themes
    if (regions.length > 0) {
      regionOptions.value = regions
      regionOptionsLoaded.value = true
    }
  } finally {
    adminLoading.value = false
    adminRefreshing.value = false
  }
}

async function refreshAdminView() {
  await refreshAll()
  await loadAdminData()
}

async function loadThemes() {
  if (!token.value) return
  themesLoading.value = true
  try {
    const response = await authFetch<{ themes: ThemeInfo[] }>('/api/admin/themes')
    themes.value = response.themes
  } finally {
    themesLoading.value = false
  }
}

async function activateTheme(themeID: string) {
  const id = normalizeThemeID(themeID)
  if (!token.value || !id || id === activeThemeID.value) return
  themesLoading.value = true
  try {
    const response = await fetchJson<{ active_theme: string }>('/api/admin/themes/active', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token.value}`
      },
      body: JSON.stringify({ id })
    })
    appSettings.value = { ...appSettings.value, active_theme: response.active_theme }
    settingsEditor.value = { ...settingsEditor.value, active_theme: response.active_theme }
    await loadThemes()
    message.success('主题已切换，刷新前台页面后生效')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`主题切换失败：${description}`)
  } finally {
    themesLoading.value = false
  }
}

async function uploadTheme(file: File) {
  if (!token.value) return
  themesLoading.value = true
  try {
    const form = new FormData()
    form.append('file', file)
    await fetchJson<ThemeInfo>('/api/admin/themes/upload', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token.value}`
      },
      body: form
    })
    await loadThemes()
    message.success('主题上传成功')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`主题上传失败：${description}`)
  } finally {
    themesLoading.value = false
  }
}

async function deleteTheme(themeID: string) {
  const id = normalizeThemeID(themeID)
  if (!token.value || !id || id === 'default') return
  themesLoading.value = true
  try {
    await fetchJson<{ deleted: boolean }>(`/api/admin/themes/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      headers: {
        Authorization: `Bearer ${token.value}`
      }
    })
    await loadThemes()
    message.success('主题已删除')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`主题删除失败：${description}`)
  } finally {
    themesLoading.value = false
  }
}

type SettingsSaveScope = 'display' | 'notify'

function numberSetting(value: unknown, fallback: number, min: number, max: number) {
  const next = Number(value)
  if (!Number.isFinite(next)) return fallback
  return Math.min(max, Math.max(min, next))
}

function boolSetting(value: unknown, fallback: boolean) {
  return typeof value === 'boolean' ? value : fallback
}

function intervalSetting(value: unknown, fallback: number) {
  const next = Number(value)
  return alertIntervalOptions.some((item) => item.value === next) ? next : fallback
}

function normalizeEmailSecurity(value: unknown) {
  const next = String(value || '').trim().toLowerCase()
  return emailSecurityOptions.some((item) => item.value === next) ? next : 'starttls'
}

function normalizeSiteName(value: unknown) {
  const next = String(value || '').trim()
  return next || 'Rivo Monitor'
}

function normalizeSiteDescription(value: unknown) {
  return String(value || '').trim()
}

function normalizeImageURL(value: unknown) {
  const next = String(value || '').trim()
  if (!next) return ''
  if (isSameOriginImageURL(next)) return next
  if (!/^https?:\/\//i.test(next)) return ''
  try {
    const parsed = new URL(next)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? next : ''
  } catch {
    return ''
  }
}

function isSameOriginImageURL(value: string) {
  return value.startsWith('/') && !value.startsWith('//') && !/[\u0000-\u001f\u007f]/.test(value)
}

function normalizeThemeID(value: unknown) {
  const next = String(value || '').trim()
  return /^[A-Za-z0-9_-]+$/.test(next) ? next : 'default'
}

function normalizeAdminPath(value: unknown) {
  return String(value || '').trim().replace(/^\/+|\/+$/g, '')
}

function cssImageURL(value: string) {
  const escaped = value
    .replace(/\\/g, '\\\\')
    .replace(/"/g, '\\"')
    .replace(/[\r\n]/g, '')
  return `url("${escaped}")`
}

function buildSettingsPayload(scope: SettingsSaveScope): AppSettings {
  const base = normalizeAppSettingsPayload(appSettings.value)
  const editor = settingsEditor.value
  const next = { ...base }

  if (scope === 'display') {
    next.site_name = normalizeSiteName(editor.site_name)
    next.site_description = normalizeSiteDescription(editor.site_description)
    next.site_avatar_url = normalizeImageURL(editor.site_avatar_url)
    next.user_avatar_url = normalizeImageURL(editor.user_avatar_url)
    next.home_background_url = normalizeImageURL(editor.home_background_url)
    next.admin_path = normalizeAdminPath(editor.admin_path)
    next.show_home_summary = boolSetting(editor.show_home_summary, base.show_home_summary)
    next.show_billing_details = boolSetting(editor.show_billing_details, base.show_billing_details)
    next.show_traffic_plan = boolSetting(editor.show_traffic_plan, base.show_traffic_plan)
    next.show_node_tags = boolSetting(editor.show_node_tags, base.show_node_tags)
    next.mask_ip_addresses = boolSetting(editor.mask_ip_addresses, base.mask_ip_addresses)
    next.snapshot_enabled = boolSetting(editor.snapshot_enabled, base.snapshot_enabled)
    next.snapshot_collect_processes = boolSetting(editor.snapshot_collect_processes, base.snapshot_collect_processes)
    next.snapshot_collect_connections = boolSetting(editor.snapshot_collect_connections, base.snapshot_collect_connections)
    next.snapshot_mask_sensitive = boolSetting(editor.snapshot_mask_sensitive, base.snapshot_mask_sensitive)
    next.snapshot_interval_seconds = numberSetting(editor.snapshot_interval_seconds, base.snapshot_interval_seconds, 15, 3600)
    next.snapshot_process_limit = numberSetting(editor.snapshot_process_limit, base.snapshot_process_limit, 1, 50)
    next.snapshot_connection_limit = numberSetting(editor.snapshot_connection_limit, base.snapshot_connection_limit, 1, 500)
    next.metrics_retention_months = normalizedMetricsRetentionMonths(editor.metrics_retention_months)
    next.asset_base_currency = normalizeAssetCurrency(editor.asset_base_currency || base.asset_base_currency)
    next.exchange_rate_auto_update = boolSetting(editor.exchange_rate_auto_update, base.exchange_rate_auto_update)
    return normalizeAppSettingsPayload(next)
  }

  next.wecom_webhook_enabled = boolSetting(editor.wecom_webhook_enabled, base.wecom_webhook_enabled)
  next.wecom_webhook_url = String(editor.wecom_webhook_url || '').trim()
  next.telegram_enabled = boolSetting(editor.telegram_enabled, base.telegram_enabled)
  next.telegram_bot_token = String(editor.telegram_bot_token || '').trim()
  next.telegram_chat_id = String(editor.telegram_chat_id || '').trim()
  next.email_enabled = boolSetting(editor.email_enabled, base.email_enabled)
  next.email_smtp_host = String(editor.email_smtp_host || '').trim()
  next.email_smtp_port = numberSetting(editor.email_smtp_port, base.email_smtp_port, 1, 65535)
  next.email_smtp_security = normalizeEmailSecurity(editor.email_smtp_security)
  next.email_smtp_username = String(editor.email_smtp_username || '').trim()
  next.email_smtp_password = String(editor.email_smtp_password || '').trim()
  next.email_from = String(editor.email_from || '').trim()
  next.email_to = String(editor.email_to || '').trim()
  next.alert_interval_minutes = intervalSetting(editor.alert_interval_minutes, base.alert_interval_minutes)
  next.offline_alert_delay_minutes = numberSetting(editor.offline_alert_delay_minutes, base.offline_alert_delay_minutes, 0, 1440)
  next.traffic_alert_enabled = boolSetting(editor.traffic_alert_enabled, base.traffic_alert_enabled)
  next.traffic_alert_percent = numberSetting(editor.traffic_alert_percent, base.traffic_alert_percent, 0, 100)
  next.cpu_alert_enabled = boolSetting(editor.cpu_alert_enabled, base.cpu_alert_enabled)
  next.cpu_alert_percent = numberSetting(editor.cpu_alert_percent, base.cpu_alert_percent, 0, 100)
  next.memory_alert_enabled = boolSetting(editor.memory_alert_enabled, base.memory_alert_enabled)
  next.memory_alert_percent = numberSetting(editor.memory_alert_percent, base.memory_alert_percent, 0, 100)
  next.disk_load_alert_enabled = boolSetting(editor.disk_load_alert_enabled, base.disk_load_alert_enabled)
  next.disk_load_alert_percent = numberSetting(editor.disk_load_alert_percent, base.disk_load_alert_percent, 0, 100)
  next.load_alert_enabled = boolSetting(editor.load_alert_enabled, base.load_alert_enabled)
  next.load_alert_threshold = numberSetting(editor.load_alert_threshold, base.load_alert_threshold, 0, 100)
  next.expiry_alert_enabled = boolSetting(editor.expiry_alert_enabled, base.expiry_alert_enabled)
  next.expiry_alert_days = numberSetting(editor.expiry_alert_days, base.expiry_alert_days, 1, 366)
  return normalizeAppSettingsPayload(next)
}

function normalizeAppSettingsPayload(settings: AppSettings): AppSettings {
  const defaults = defaultAppSettings()
  return {
    site_name: normalizeSiteName(settings.site_name || defaults.site_name),
    site_description: normalizeSiteDescription(settings.site_description ?? defaults.site_description),
    site_avatar_url: normalizeImageURL(settings.site_avatar_url) || defaults.site_avatar_url,
    user_avatar_url: normalizeImageURL(settings.user_avatar_url),
    home_background_url: normalizeImageURL(settings.home_background_url),
    active_theme: normalizeThemeID(settings.active_theme || defaults.active_theme),
    admin_path: normalizeAdminPath(settings.admin_path || defaults.admin_path),
    show_home_summary: boolSetting(settings.show_home_summary, defaults.show_home_summary),
    show_billing_details: boolSetting(settings.show_billing_details, defaults.show_billing_details),
    show_traffic_plan: boolSetting(settings.show_traffic_plan, defaults.show_traffic_plan),
    show_node_tags: boolSetting(settings.show_node_tags, defaults.show_node_tags),
    mask_ip_addresses: boolSetting(settings.mask_ip_addresses, defaults.mask_ip_addresses),
    snapshot_enabled: boolSetting(settings.snapshot_enabled, defaults.snapshot_enabled),
    snapshot_collect_processes: boolSetting(settings.snapshot_collect_processes, defaults.snapshot_collect_processes),
    snapshot_collect_connections: boolSetting(settings.snapshot_collect_connections, defaults.snapshot_collect_connections),
    snapshot_mask_sensitive: boolSetting(settings.snapshot_mask_sensitive, defaults.snapshot_mask_sensitive),
    snapshot_interval_seconds: numberSetting(settings.snapshot_interval_seconds, defaults.snapshot_interval_seconds, 15, 3600),
    snapshot_process_limit: numberSetting(settings.snapshot_process_limit, defaults.snapshot_process_limit, 1, 50),
    snapshot_connection_limit: numberSetting(settings.snapshot_connection_limit, defaults.snapshot_connection_limit, 1, 500),
    metrics_retention_months: normalizedMetricsRetentionMonths(settings.metrics_retention_months),
    asset_base_currency: normalizeAssetCurrency(settings.asset_base_currency || defaults.asset_base_currency),
    exchange_rate_auto_update: boolSetting(settings.exchange_rate_auto_update, defaults.exchange_rate_auto_update),
    wecom_webhook_enabled: boolSetting(settings.wecom_webhook_enabled, defaults.wecom_webhook_enabled),
    wecom_webhook_url: String(settings.wecom_webhook_url || '').trim(),
    telegram_enabled: boolSetting(settings.telegram_enabled, defaults.telegram_enabled),
    telegram_bot_token: String(settings.telegram_bot_token || '').trim(),
    telegram_chat_id: String(settings.telegram_chat_id || '').trim(),
    email_enabled: boolSetting(settings.email_enabled, defaults.email_enabled),
    email_smtp_host: String(settings.email_smtp_host || '').trim(),
    email_smtp_port: numberSetting(settings.email_smtp_port, defaults.email_smtp_port, 1, 65535),
    email_smtp_security: normalizeEmailSecurity(settings.email_smtp_security || defaults.email_smtp_security),
    email_smtp_username: String(settings.email_smtp_username || '').trim(),
    email_smtp_password: String(settings.email_smtp_password || '').trim(),
    email_from: String(settings.email_from || '').trim(),
    email_to: String(settings.email_to || '').trim(),
    traffic_alert_enabled: boolSetting(settings.traffic_alert_enabled, defaults.traffic_alert_enabled),
    traffic_alert_percent: numberSetting(settings.traffic_alert_percent, defaults.traffic_alert_percent, 0, 100),
    cpu_alert_enabled: boolSetting(settings.cpu_alert_enabled, defaults.cpu_alert_enabled),
    cpu_alert_percent: numberSetting(settings.cpu_alert_percent, defaults.cpu_alert_percent, 0, 100),
    memory_alert_enabled: boolSetting(settings.memory_alert_enabled, defaults.memory_alert_enabled),
    memory_alert_percent: numberSetting(settings.memory_alert_percent, defaults.memory_alert_percent, 0, 100),
    disk_load_alert_enabled: boolSetting(settings.disk_load_alert_enabled, defaults.disk_load_alert_enabled),
    disk_load_alert_percent: numberSetting(settings.disk_load_alert_percent, defaults.disk_load_alert_percent, 0, 100),
    load_alert_enabled: boolSetting(settings.load_alert_enabled, defaults.load_alert_enabled),
    load_alert_threshold: numberSetting(settings.load_alert_threshold, defaults.load_alert_threshold, 0, 100),
    alert_interval_minutes: intervalSetting(settings.alert_interval_minutes, defaults.alert_interval_minutes),
    offline_alert_delay_minutes: numberSetting(settings.offline_alert_delay_minutes, defaults.offline_alert_delay_minutes, 0, 1440),
    expiry_alert_enabled: boolSetting(settings.expiry_alert_enabled, defaults.expiry_alert_enabled),
    expiry_alert_days: numberSetting(settings.expiry_alert_days, defaults.expiry_alert_days, 1, 366)
  }
}

async function saveGlobalSettings(scope: SettingsSaveScope) {
  const validationError = validateSettingsEditor(scope)
  if (validationError) {
    message.error(validationError)
    return
  }
  adminLoading.value = true
  try {
    const previousAdminPath = configuredAdminPath()
    const payload = buildSettingsPayload(scope)
    const settings = await fetchJson<AppSettings>('/api/admin/settings', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token.value}`
      },
      body: JSON.stringify(payload)
    })
    appSettings.value = settings
    settingsEditor.value = { ...settings }
    if (scope === 'display' && adminConfig.value && settings.admin_path) {
      adminConfig.value = {
        ...adminConfig.value,
        http: {
          ...adminConfig.value.http,
          admin_path: settings.admin_path
        }
      }
      if (previousAdminPath && settings.admin_path !== previousAdminPath && isCurrentAdminPath(previousAdminPath)) {
        window.location.assign(`/${settings.admin_path}`)
        return
      }
    }
    message.success('设置保存成功')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`设置保存失败：${description}`)
  } finally {
    adminLoading.value = false
  }
}

async function sendWeComTestMessage() {
  const webhookURL = String(settingsEditor.value.wecom_webhook_url || '').trim()
  if (!webhookURL) {
    message.error('请先填写企业微信 Webhook')
    return
  }
  weComTestLoading.value = true
  try {
    await fetchJson<{ ok: boolean }>('/api/admin/settings/wecom-test', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token.value}`
      },
      body: JSON.stringify({ wecom_webhook_url: webhookURL })
    })
    message.success('企业微信测试消息已发送')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`测试消息发送失败：${description}`)
  } finally {
    weComTestLoading.value = false
  }
}

async function sendTelegramTestMessage() {
  const botToken = String(settingsEditor.value.telegram_bot_token || '').trim()
  const chatID = String(settingsEditor.value.telegram_chat_id || '').trim()
  if (!botToken || !chatID) {
    message.error('请先填写 Telegram Token 和用户 ID')
    return
  }
  telegramTestLoading.value = true
  try {
    await fetchJson<{ ok: boolean }>('/api/admin/settings/telegram-test', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token.value}`
      },
      body: JSON.stringify({
        telegram_bot_token: botToken,
        telegram_chat_id: chatID
      })
    })
    message.success('Telegram 测试消息已发送')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`测试消息发送失败：${description}`)
  } finally {
    telegramTestLoading.value = false
  }
}

async function sendEmailTestMessage() {
  const smtpHost = String(settingsEditor.value.email_smtp_host || '').trim()
  const from = String(settingsEditor.value.email_from || '').trim()
  const to = String(settingsEditor.value.email_to || '').trim()
  if (!smtpHost || !from || !to) {
    message.error('请先填写 SMTP 服务器、发件人和收件人')
    return
  }
  emailTestLoading.value = true
  try {
    await fetchJson<{ ok: boolean }>('/api/admin/settings/email-test', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token.value}`
      },
      body: JSON.stringify({
        email_smtp_host: smtpHost,
        email_smtp_port: numberSetting(settingsEditor.value.email_smtp_port, 587, 1, 65535),
        email_smtp_security: normalizeEmailSecurity(settingsEditor.value.email_smtp_security),
        email_smtp_username: String(settingsEditor.value.email_smtp_username || '').trim(),
        email_smtp_password: String(settingsEditor.value.email_smtp_password || '').trim(),
        email_from: from,
        email_to: to
      })
    })
    message.success('邮件测试消息已发送')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`测试邮件发送失败：${description}`)
  } finally {
    emailTestLoading.value = false
  }
}

async function saveNodeConfig() {
  const nodeID = adminEditNodeID.value
  if (!nodeID) return
  const validationError = validateNodeEditor()
  if (validationError) {
    message.error(validationError)
    return
  }

  adminLoading.value = true
  try {
    await fetchJson<NodeRecord>(`/api/admin/nodes/${encodeURIComponent(nodeID)}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token.value}`
      },
      body: JSON.stringify(nodeUpdatePayload())
    })
    nodeEditOpen.value = false
    await refreshAll()
    await loadAdminData()
    message.success('Agent 配置保存成功')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`Agent 配置保存失败：${description}`)
  } finally {
    adminLoading.value = false
  }
}

async function loadProbeTasks() {
  if (!token.value) return
  probeTasks.value = await authFetch<ProbeTask[]>('/api/admin/probe-tasks')
}

function openCreateProbeTask() {
  probeTaskEditor.value = emptyProbeTaskEditor()
  probeTaskModalOpen.value = true
}

function openEditProbeTask(task: ProbeTask) {
  probeTaskEditor.value = editorFromProbeTask(task)
  probeTaskModalOpen.value = true
}

function handleProbeAssignAllChange(value: boolean) {
  if (value) {
    probeTaskEditor.value.enabled = true
  }
}

async function saveProbeTask() {
  const validationError = validateProbeTaskEditor()
  if (validationError) {
    message.error(validationError)
    return
  }

  probeSaving.value = true
  try {
    const payload = probeTaskPayload()
    const taskID = probeTaskEditor.value.id
    const savedTask = await fetchJson<ProbeTask>(taskID ? `/api/admin/probe-tasks/${taskID}` : '/api/admin/probe-tasks', {
      method: taskID ? 'PUT' : 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token.value}`
      },
      body: JSON.stringify(payload)
    })
    probeTaskEditor.value = emptyProbeTaskEditor()
    probeTaskModalOpen.value = false
    await loadProbeTasks()
    if (!taskID && payload.assign_to_all_agents) {
      addProbeTaskToLocalNodes(savedTask.id)
    }
    message.success(taskID ? 'Ping 节点已更新并下发配置' : (payload.assign_to_all_agents ? 'Ping 节点已添加，并已推送给所有 Agent' : 'Ping 节点已添加'))
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`Ping 节点保存失败：${description}`)
  } finally {
    probeSaving.value = false
  }
}

async function toggleProbeTask(task: ProbeTask, enabled: boolean) {
  try {
    await saveProbeTaskDirect({ ...task, enabled })
    await loadProbeTasks()
    message.success(enabled ? 'Ping 节点已启用' : 'Ping 节点已禁用')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`Ping 节点状态更新失败：${description}`)
  }
}

async function deleteProbeTask(task: ProbeTask) {
  if (!window.confirm(`确认删除 Ping 节点「${task.name || displayTarget(task.target)}」？`)) return
  try {
    await fetchJson<{ deleted: boolean }>(`/api/admin/probe-tasks/${task.id}`, {
      method: 'DELETE',
      headers: {
        Authorization: `Bearer ${token.value}`
      }
    })
    await loadProbeTasks()
    removeProbeTaskFromLocalNodes(task.id)
    message.success('Ping 节点已删除并下发配置')
  } catch (error) {
    const description = error instanceof Error ? error.message : '未知错误'
    message.error(`Ping 节点删除失败：${description}`)
  }
}

async function saveProbeTaskDirect(task: ProbeTask) {
  await fetchJson<ProbeTask>(`/api/admin/probe-tasks/${task.id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token.value}`
    },
    body: JSON.stringify({
      name: task.name,
      type: task.type,
      ip_version: probeIPVersionValues.includes(task.ip_version) ? task.ip_version : 'auto',
      target: task.target,
      interval_seconds: task.interval_seconds,
      timeout_ms: task.timeout_ms,
      enabled: task.enabled
    })
  })
}

async function openProbePanel(node: NodeRecord) {
  activeProbeNode.value = node
  showInactiveProbeHistory.value = false
  probePanelTasks.value = []
  probeResults.value = []
  probeRefreshCountdown.value = 0
  probePanelOpen.value = true
  await loadProbeResults(node.node_id)
}

async function loadProbeResults(nodeID: string, silent = false) {
  const requestID = ++probeResultsRequestID
  if (!silent) {
    probeLoading.value = true
  }
  try {
    const params = new URLSearchParams({ range: rangeQueryValue(probeRange.value) })
    if (showInactiveProbeHistory.value) {
      params.set('include_inactive', '1')
    }
    const response = await fetchJson<ProbeResultsResponse>(`/api/nodes/${encodeURIComponent(nodeID)}/probe-results?${params.toString()}`)
    if (requestID !== probeResultsRequestID) return
    probeRangeAnchor.value = response.range_anchor || Date.now()
    probePanelTasks.value = response.tasks
    probeResults.value = response.results
    syncProbeAutoRefresh()
    renderProbeChart()
  } finally {
    if (!silent && requestID === probeResultsRequestID) {
      probeLoading.value = false
    }
  }
}

async function toggleInactiveProbeHistory() {
  const nextValue = !showInactiveProbeHistory.value
  showInactiveProbeHistory.value = nextValue
  if (!nextValue) {
    probePanelTasks.value = probePanelTasks.value.filter((task) => task.enabled)
    probeResults.value = []
    syncProbeAutoRefresh()
    renderProbeChart()
  }
  if (!activeProbeNode.value) return
  await loadProbeResults(activeProbeNode.value.node_id)
}

async function setProbeRange(value: ProbeRange) {
  probeRange.value = clampRangeToRetention(value)
  if (activeProbeNode.value && probePanelOpen.value) {
    await loadProbeResults(activeProbeNode.value.node_id)
  }
}

function rangeHours(value: ProbeRange) {
  return probeRangeOptions.find((item) => item.value === value)?.hours ?? 24
}

function rangeBucketMs(value: ProbeRange) {
  return probeRangeOptions.find((item) => item.value === value)?.bucketMs ?? hourMs
}

function rangeAxisIntervalMs(value: ProbeRange, anchorTimestamp?: number | null) {
  const labelInterval = rangeAxisLabelIntervalMs(value)
  if (labelInterval) return labelInterval

  const bounds = rangeAxisBounds(value, anchorTimestamp)
  const span = Math.max(bounds.max - bounds.min, rangeBucketMs(value))
  return Math.max(1, Math.round(span / axisSplitCount))
}

function rangeAxisLabelIntervalMs(value: ProbeRange) {
  switch (value) {
    case '1h':
      return rangeBucketMs(value)
    case '4h':
      return rangeBucketMs(value)
    case '12h':
      return rangeBucketMs(value)
    case '1d':
      return rangeBucketMs(value)
    case '7d':
      return dayMs
    case '1m':
      return 2 * dayMs
    case '3m':
      return 7 * dayMs
    case '6m':
      return 15 * dayMs
    case '1y':
      return 30 * dayMs
    default:
      return null
  }
}

function rangeQueryValue(value: ProbeRange) {
  return probeRangeOptions.find((item) => item.value === value)?.value ?? '1h'
}

function rangeRetentionMonths(value: ProbeRange) {
  switch (value) {
    case '3m':
      return 3
    case '6m':
      return 6
    case '1y':
      return 12
    default:
      return 1
  }
}

function normalizedMetricsRetentionMonths(value: unknown) {
  const next = Number(value)
  return [1, 3, 6, 12].includes(next) ? next : 6
}

function clampRangeToRetention(value: ProbeRange): ProbeRange {
  if (rangeRetentionMonths(value) <= normalizedMetricsRetentionMonths(appSettings.value.metrics_retention_months)) {
    return value
  }
  return availableProbeRangeOptions.value[availableProbeRangeOptions.value.length - 1]?.value ?? '1h'
}

function isAggregatedProbeRange(value: ProbeRange) {
  return value === '1m' || value === '3m' || value === '6m' || value === '1y'
}

const chartAxisLineColor = 'rgba(255,255,255,0.14)'
const chartAxisLabelColor = 'rgba(255,255,255,0.52)'
const chartNameColor = 'rgba(255,255,255,0.58)'
const chartLegendTextColor = 'rgba(255,255,255,0.72)'
const chartSplitLineColor = 'rgba(255,255,255,0.12)'
const chartEmptyTextColor = 'rgba(255,255,255,0.5)'
const chartTooltipStyle = {
  backgroundColor: 'rgba(7,17,31,0.9)',
  borderColor: 'rgba(255,255,255,0.18)',
  borderWidth: 1,
  textStyle: { color: 'rgba(255,255,255,0.88)' },
  extraCssText: 'border-radius:14px;box-shadow:0 16px 38px rgba(0,0,0,.26);backdrop-filter:blur(18px) saturate(150%);'
}

function rangeTimeBounds(value: ProbeRange, anchorTimestamp?: number | null) {
  const anchor = Number.isFinite(anchorTimestamp) ? Number(anchorTimestamp) : Date.now()
  const max = anchor
  const min = max - rangeHours(value) * hourMs
  return { min, max }
}

function rangeAxisBounds(value: ProbeRange, anchorTimestamp?: number | null) {
  const bounds = rangeTimeBounds(value, anchorTimestamp)
  const interval = rangeAxisLabelIntervalMs(value) ?? rangeBucketMs(value)
  const intervalCount = Math.max(1, Math.ceil((rangeHours(value) * hourMs) / interval))
  const max = bounds.max
  const min = max - intervalCount * interval
  return {
    min,
    max
  }
}

function chartLabelTargetCount(width?: number | null) {
  const nextWidth = Number(width)
  if (!Number.isFinite(nextWidth) || nextWidth <= 0) return 6
  if (nextWidth < 360) return 4
  if (nextWidth < 560) return 5
  if (nextWidth < 760) return 6
  if (nextWidth < 1024) return 8
  return 10
}

function rangeAxisLabelStride(value: ProbeRange, anchorTimestamp?: number | null, width?: number | null) {
  const bounds = rangeAxisBounds(value, anchorTimestamp)
  const interval = rangeAxisIntervalMs(value, anchorTimestamp)
  const tickCount = Math.max(1, Math.round((bounds.max - bounds.min) / interval))
  return Math.max(1, Math.ceil(tickCount / chartLabelTargetCount(width)))
}

function shouldShowRangeAxisLabel(timestamp: number, range: ProbeRange, anchorTimestamp?: number | null, width?: number | null) {
  if (!Number.isFinite(timestamp)) return false
  const bounds = rangeAxisBounds(range, anchorTimestamp)
  const interval = rangeAxisIntervalMs(range, anchorTimestamp)
  const edgeTolerance = Math.max(1, interval / 2)
  if (Math.abs(timestamp - bounds.min) <= edgeTolerance || Math.abs(timestamp - bounds.max) <= edgeTolerance) return true
  const tickIndex = Math.round((timestamp - bounds.min) / interval)
  return tickIndex >= 0 && tickIndex % rangeAxisLabelStride(range, anchorTimestamp, width) === 0
}

function timeXAxisForRange(value: ProbeRange, anchorTimestamp?: number | null, width?: number | null) {
  const bounds = rangeAxisBounds(value, anchorTimestamp)
  const interval = rangeAxisIntervalMs(value, anchorTimestamp)
  return {
    type: 'value' as const,
    min: bounds.min,
    max: bounds.max,
    interval,
    minInterval: interval,
    maxInterval: interval,
    axisTick: { show: false },
    axisLine: { lineStyle: { color: chartAxisLineColor } },
    splitLine: { show: false },
    axisLabel: {
      color: chartAxisLabelColor,
      hideOverlap: false,
      showMinLabel: true,
      showMaxLabel: true,
      fontSize: 11,
      formatter: (raw: number | string) => formatRangeAxisLabel(Number(raw), value, anchorTimestamp, width)
    }
  }
}

function chartGridForRange(value: ProbeRange) {
  if (value === '6m' || value === '1y') {
    return { top: 44, left: 24, right: 34, bottom: 16, containLabel: true }
  }
  return { top: 44, left: 14, right: 18, bottom: 14, containLabel: true }
}

function formatRangeAxisLabel(timestamp: number, range: ProbeRange, anchorTimestamp?: number | null, width?: number | null) {
  if (!Number.isFinite(timestamp)) return ''
  if (!shouldShowRangeAxisLabel(timestamp, range, anchorTimestamp, width)) return ''

  const date = new Date(timestamp)
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  if (range === '1h' || range === '4h' || range === '12h' || range === '1d') {
    return `${hour}:${minute}`
  }
  if (range === '7d' || range === '1m' || range === '3m' || range === '6m') {
    return `${month}/${day}`
  }
  if (range === '1y') {
    return `${String(date.getFullYear()).slice(2)}/${month}`
  }
  return `${hour}:${minute}`
}

function formatRangeWindow(range: ProbeRange, anchorTimestamp?: number | null) {
  const { min, max } = rangeTimeBounds(range, anchorTimestamp)
  return `${formatRangeWindowTime(min)} - ${formatRangeWindowTime(max)}`
}

function formatRangeWindowTime(timestamp: number) {
  const date = new Date(timestamp)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  const second = String(date.getSeconds()).padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:${minute}:${second}`
}

type ChartTooltipParam = {
  axisValue?: string | number
  marker?: string
  seriesName?: string
  value?: unknown
}

function chartTooltipFormatter(unit: string, emptyLabel: string) {
  return (raw: unknown) => {
    const params = (Array.isArray(raw) ? raw : [raw]) as ChartTooltipParam[]
    const timestamp = tooltipTimestamp(params[0])
    const lines = [Number.isFinite(timestamp) ? formatRangeWindowTime(timestamp) : '']
    for (const item of params) {
      const value = tooltipSeriesValue(item)
      const label = value === null || value === undefined ? emptyLabel : `${value} ${unit}`
      lines.push(`${item.marker ?? ''}${item.seriesName ?? ''}<span style="float:right;margin-left:16px;font-weight:700">${label}</span>`)
    }
    return lines.filter(Boolean).join('<br/>')
  }
}

type ProbeChartTooltipSeries = {
  name: string
  color: string
  intervalSeconds: number
  data: Array<[number, number | null]>
}

function probeChartTooltipFormatter(series: ProbeChartTooltipSeries[], range: ProbeRange) {
  return (raw: unknown) => {
    const params = (Array.isArray(raw) ? raw : [raw]) as ChartTooltipParam[]
    const timestamp = tooltipTimestamp(params[0])
    if (!Number.isFinite(timestamp)) return chartTooltipFormatter('ms', '失败')(raw)

    const lines = [formatRangeWindowTime(timestamp)]
    for (const item of series) {
      const nearest = nearestProbeTooltipPoint(item.data, timestamp, probeTooltipToleranceMs(item.intervalSeconds, range))
      let label = '无数据'
      if (nearest) {
        label = nearest.value === null || nearest.value === undefined ? '失败' : `${nearest.value} ms`
      }
      lines.push(`${chartTooltipMarker(item.color)}${escapeHTML(item.name)}<span style="float:right;margin-left:16px;font-weight:700">${label}</span>`)
    }
    return lines.filter(Boolean).join('<br/>')
  }
}

function nearestProbeTooltipPoint(data: Array<[number, number | null]>, timestamp: number, toleranceMs: number) {
  let nearest: { timestamp: number; value: number | null; distance: number } | null = null
  for (const [pointTimestamp, value] of data) {
    if (!Number.isFinite(pointTimestamp)) continue
    const distance = Math.abs(pointTimestamp - timestamp)
    if (distance > toleranceMs) continue
    if (!nearest || distance < nearest.distance) {
      nearest = { timestamp: pointTimestamp, value, distance }
    }
  }
  return nearest
}

function probeTooltipToleranceMs(intervalSeconds: number, range: ProbeRange) {
  const intervalMs = Math.max(1, intervalSeconds || 15) * 1000
  if (isAggregatedProbeRange(range)) {
    return Math.max(rangeBucketMs(range), intervalMs)
  }
  return Math.max(Math.ceil(intervalMs * 0.75), 10 * 1000)
}

function chartTooltipMarker(color: string) {
  return `<span style="display:inline-block;margin-right:4px;border-radius:50%;width:10px;height:10px;background:${color}"></span>`
}

function escapeHTML(value: string) {
  return value.replace(/[&<>"']/g, (char) => {
    switch (char) {
      case '&':
        return '&amp;'
      case '<':
        return '&lt;'
      case '>':
        return '&gt;'
      case '"':
        return '&quot;'
      case '\'':
        return '&#39;'
      default:
        return char
    }
  })
}

function tooltipTimestamp(param?: ChartTooltipParam) {
  if (param?.axisValue !== undefined && param.axisValue !== null) return Number(param.axisValue)
  const value = param?.value
  if (Array.isArray(value)) return Number(value[0])
  return Number(value)
}

function tooltipSeriesValue(param?: ChartTooltipParam) {
  const value = param?.value
  if (Array.isArray(value)) return value[1] as number | null | undefined
  return value as number | null | undefined
}

function isTimestampInRange(timestamp: number, range: ProbeRange, anchorTimestamp?: number | null) {
  if (!Number.isFinite(timestamp)) return false
  const { min, max } = rangeTimeBounds(range, anchorTimestamp)
  return timestamp >= min && timestamp <= max
}

function filterProbeResultsForRange(results: ProbeResult[], range: ProbeRange, anchorTimestamp?: number | null) {
  return results.filter((item) => isTimestampInRange(Date.parse(item.created_at), range, anchorTimestamp))
}

function filterMetricsForRange(items: NodeMetric[], range: ProbeRange, anchorTimestamp?: number | null) {
  return items.filter((item) => isTimestampInRange(item.ts, range, anchorTimestamp))
}

function dataGapDetectionThresholdMs(intervalSeconds?: number, multiplier = 3) {
  const expectedMs = Math.max(1, intervalSeconds || 15) * 1000
  return Math.max(expectedMs * multiplier, 15 * 1000)
}

function bucketPointTimestamp(timestamp: number, range: ProbeRange, anchorTimestamp?: number | null) {
  const bucketMs = rangeBucketMs(range)
  const { min, max } = rangeAxisBounds(range, anchorTimestamp)
  if (timestamp < min || timestamp > max) return null
  const bucketIndex = Math.ceil((timestamp - min) / bucketMs)
  const point = Math.min(max, min + Math.max(0, bucketIndex) * bucketMs)
  return point
}

function seriesAxisValue(timestamp: number, range: ProbeRange, anchorTimestamp?: number | null) {
  if (!isAggregatedProbeRange(range)) {
    if (!isTimestampInRange(timestamp, range, anchorTimestamp)) return null
    return timestamp
  }
  if (!isTimestampInRange(timestamp, range, anchorTimestamp)) return null
  return timestamp
}

function metricBucketTimestamp(timestamp: number, range: ProbeRange, anchorTimestamp?: number | null) {
  return bucketPointTimestamp(timestamp, range, anchorTimestamp)
}

function gapSegmentsFromTimestamps(timestamps: number[], range: ProbeRange, intervalSeconds?: number, anchorTimestamp?: number | null, multiplier = 3) {
  const { min, max } = rangeTimeBounds(range, anchorTimestamp)
  const expectedMs = Math.max(1, intervalSeconds || 15) * 1000
  const thresholdMs = dataGapDetectionThresholdMs(intervalSeconds, multiplier)
  const sorted = Array.from(new Set(timestamps.filter(Number.isFinite))).sort((left, right) => left - right)
  const segments: GapSegment[] = []
  for (let index = 1; index < sorted.length; index += 1) {
    const previous = sorted[index - 1]
    const current = sorted[index]
    const duration = current - previous
    if (duration <= thresholdMs) continue
    const gapStart = previous + Math.floor(expectedMs / 2)
    const gapEnd = current - Math.ceil(expectedMs / 2)
    const start = Math.max(gapStart, min)
    const end = Math.min(gapEnd, max)
    if (end > start) {
      segments.push({ start, end, duration: end - start })
    }
  }
  return segments
}

function applyGapBreaks(data: Array<[number, number | null]>, segments: GapSegment[]): Array<[number, number | null]> {
  if (segments.length === 0) return data.sort(([left], [right]) => left - right)

  let next = data.slice().sort(([left], [right]) => left - right)
  for (const segment of segments) {
    const startValue = interpolateSeriesValue(next, segment.start)
    const endValue = interpolateSeriesValue(next, segment.end)
    next = next.filter(([ts]) => ts <= segment.start || ts >= segment.end)
    if (startValue !== null) next.push([segment.start, startValue])
    if (segment.end - segment.start > 2) {
      next.push([segment.start + 1, null])
      next.push([segment.end - 1, null])
    } else {
      next.push([Math.round((segment.start + segment.end) / 2), null])
    }
    if (endValue !== null) next.push([segment.end, endValue])
  }

  return compactSeriesData(next)
}

function interpolateSeriesValue(data: Array<[number, number | null]>, timestamp: number) {
  const points = data
    .filter(([, value]) => value !== null)
    .sort(([left], [right]) => left - right) as Array<[number, number]>
  if (points.length === 0) return null

  let previous: [number, number] | null = null
  let next: [number, number] | null = null
  for (const point of points) {
    if (point[0] <= timestamp) previous = point
    if (point[0] >= timestamp) {
      next = point
      break
    }
  }

  if (previous && next) {
    if (previous[0] === next[0]) return previous[1]
    const ratio = (timestamp - previous[0]) / (next[0] - previous[0])
    return round(previous[1] + (next[1] - previous[1]) * ratio)
  }
  return previous?.[1] ?? next?.[1] ?? null
}

function compactSeriesData(data: Array<[number, number | null]>) {
  const byTimestamp = new Map<number, number | null>()
  for (const [timestamp, value] of data.sort(([left], [right]) => left - right)) {
    const current = byTimestamp.get(timestamp)
    if (current === undefined || current === null || value === null) {
      byTimestamp.set(timestamp, value)
    }
  }
  return Array.from(byTimestamp.entries()).sort(([left], [right]) => left - right)
}

function syncProbeAutoRefresh() {
  if (!probeAutoRefreshEnabled.value) {
    stopProbeAutoRefresh()
    probeRefreshCountdown.value = 0
    return
  }
  startProbeAutoRefresh()
}

function startProbeAutoRefresh() {
  stopProbeAutoRefresh()
  const seconds = probeRefreshSeconds()
  if (seconds <= 0) {
    probeRefreshCountdown.value = 0
    return
  }
  probeRefreshCountdown.value = seconds
  probeRefreshCountdownTimer = window.setInterval(() => {
    if (!probeAutoRefreshEnabled.value || !activeProbeNode.value) {
      syncProbeAutoRefresh()
      return
    }
    probeRefreshCountdown.value -= 1
    if (probeRefreshCountdown.value > 0) return
    probeRefreshCountdown.value = probeRefreshSeconds()
    void loadProbeResults(activeProbeNode.value.node_id, true)
  }, 1000)
}

function stopProbeAutoRefresh() {
  if (!probeRefreshCountdownTimer) return
  window.clearInterval(probeRefreshCountdownTimer)
  probeRefreshCountdownTimer = undefined
}

function probeRefreshSeconds() {
  const intervals = probePanelTasks.value
    .filter((task) => task.enabled)
    .map((task) => task.interval_seconds)
    .filter((value) => value > 0)
  if (intervals.length === 0) return 0
  return Math.max(3, Math.min(...intervals))
}

async function openMetricsPanel(node: NodeRecord) {
  activeMetricsNode.value = node
  metricsPanelOpen.value = true
  await loadMetricsPanelData(node.node_id)
  if (isNodeOnline(node)) {
    startMetricsAutoRefresh()
  } else {
    stopMetricsAutoRefresh()
  }
}

async function loadMetricsPanelData(nodeID: string, silent = false) {
  const requestID = ++metricsPanelRequestID
  if (!silent) {
    metricsPanelLoading.value = true
  }
  try {
    const [nextMetrics, snapshot] = await Promise.all([
      fetchJson<NodeMetric[]>(`/api/nodes/${encodeURIComponent(nodeID)}/metrics?range=${rangeQueryValue(metricsRange.value)}`),
      fetchLatestSnapshot(nodeID)
    ])
    if (requestID !== metricsPanelRequestID) return
    metricsRangeAnchor.value = Date.now()
    metricsPanelMetrics.value = nextMetrics
    latestSnapshot.value = snapshot
    if (!silent && metricsRefreshCountdownTimer) {
      metricsRefreshCountdown.value = metricsRefreshSeconds()
    }
    renderMetricsPanelCharts()
  } finally {
    if (!silent && requestID === metricsPanelRequestID) {
      metricsPanelLoading.value = false
    }
  }
}

async function refreshMetricsPanelData(nodeID: string) {
  if (activeMetricsNode.value && !isNodeOnline(activeMetricsNode.value)) {
    return
  }
  const requestID = ++metricsPanelRequestID
  metricsPanelLoading.value = true
  try {
    try {
      await fetchJson<{ requested: boolean }>(`/api/nodes/${encodeURIComponent(nodeID)}/request-metrics`, {
        method: 'POST'
      })
      await sleep(700)
    } catch (error) {
      const description = error instanceof Error ? error.message : '未知错误'
      message.warning(`已读取现有数据，主动通知 Agent 失败：${description}`)
    }
    const [nextMetrics, snapshot] = await Promise.all([
      fetchJson<NodeMetric[]>(`/api/nodes/${encodeURIComponent(nodeID)}/metrics?range=${rangeQueryValue(metricsRange.value)}`),
      fetchLatestSnapshot(nodeID)
    ])
    if (requestID !== metricsPanelRequestID) return
    metricsRangeAnchor.value = Date.now()
    metricsPanelMetrics.value = nextMetrics
    latestSnapshot.value = snapshot
    metricsRefreshCountdown.value = metricsRefreshSeconds()
    renderMetricsPanelCharts()
  } finally {
    if (requestID === metricsPanelRequestID) {
      metricsPanelLoading.value = false
    }
  }
}

async function fetchLatestSnapshot(nodeID: string) {
  if (!isLoggedIn.value) {
    latestSnapshot.value = null
    return null
  }
  try {
    const response = await authFetch<SnapshotResponse>(`/api/admin/nodes/${encodeURIComponent(nodeID)}/snapshots/latest`)
    return response.snapshot
  } catch (error) {
    console.warn('load snapshot failed', error)
    return null
  }
}

async function setMetricsRange(value: ProbeRange) {
  metricsRange.value = clampRangeToRetention(value)
  if (activeMetricsNode.value && metricsPanelOpen.value) {
    await loadMetricsPanelData(activeMetricsNode.value.node_id)
    if (isNodeOnline(activeMetricsNode.value)) {
      startMetricsAutoRefresh()
    } else {
      stopMetricsAutoRefresh()
    }
  }
}

function startMetricsAutoRefresh() {
  stopMetricsAutoRefresh()
  if (!metricsPanelOpen.value || !activeMetricsNode.value || !isNodeOnline(activeMetricsNode.value)) {
    return
  }
  metricsRefreshCountdown.value = metricsRefreshSeconds()
  metricsRefreshCountdownTimer = window.setInterval(() => {
    if (!metricsPanelOpen.value || !activeMetricsNode.value) return
    if (!isNodeOnline(activeMetricsNode.value)) {
      stopMetricsAutoRefresh()
      return
    }
    metricsRefreshCountdown.value -= 1
    if (metricsRefreshCountdown.value > 0) return
    metricsRefreshCountdown.value = metricsRefreshSeconds()
    void loadMetricsPanelData(activeMetricsNode.value.node_id, true)
  }, 1000)
}

function stopMetricsAutoRefresh() {
  if (!metricsRefreshCountdownTimer) return
  window.clearInterval(metricsRefreshCountdownTimer)
  metricsRefreshCountdownTimer = undefined
}

function metricsRefreshSeconds() {
  return Math.max(3, activeMetricsNode.value?.metrics_interval || 15)
}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function configuredAdminPath() {
  return normalizeAdminPath(adminConfig.value?.http.admin_path)
}

function isCurrentAdminPath(path = configuredAdminPath()) {
  const normalizedPath = normalizeAdminPath(path)
  if (!normalizedPath) return loadedFromAdminPath
  return window.location.pathname === `/${normalizedPath}` || window.location.pathname.startsWith(`/${normalizedPath}/`)
}

function navigateToConfiguredAdmin() {
  const path = configuredAdminPath()
  if (!path || isCurrentAdminPath(path)) return false
  window.location.assign(`/${path}`)
  return true
}

async function handleUserAction(key: string) {
  if (key === 'logout') {
    token.value = ''
    currentUser.value = ''
    latestSnapshot.value = null
    viewMode.value = 'home'
    window.localStorage.removeItem('rivo_token')
    window.localStorage.removeItem('rivo_user')
    window.localStorage.removeItem('rivo_view_mode')
    if (isCurrentAdminPath()) {
      window.location.assign('/')
    }
    return
  }

  if (key === 'admin') {
    if (!adminConfig.value && token.value) {
      await loadAdminData()
    }
    if (navigateToConfiguredAdmin()) return
    viewMode.value = 'admin'
    window.localStorage.setItem('rivo_view_mode', 'admin')
    await loadAdminData()
    return
  }

  viewMode.value = 'home'
  window.localStorage.setItem('rivo_view_mode', 'home')
  if (window.location.pathname !== '/') {
    window.location.assign('/')
  }
}

function formatPercent(value?: number | null) {
  if (value === undefined || value === null) return 'N/A'
  return `${Math.round(value)}%`
}

function validateNodeEditor() {
  const editor = nodeEditor.value
  const price = Number(editor.price_amount ?? 0)
  const trafficLimit = Number(editor.traffic_limit_value ?? 0)
  const trafficCalibration = Number(editor.traffic_calibration_value ?? 0)
  if ([...(editor.name || '').trim()].length > 64) return '节点名称不能超过 64 个字符'
  if ([...(editor.provider || '').trim()].length > 64) return '服务商不能超过 64 个字符'
  if (networkLineValue(editor.network_line).length > 128) return '线路不能超过 128 个字符'
  if (!billingCycleOptions.some((item) => item.value === editor.billing_cycle)) return '付费方式不正确'
  if (!currencyOptions.some((item) => item.value === editor.currency)) return '货币类型不正确'
  if (!trafficResetOptions.some((item) => item.value === editor.traffic_reset_cycle)) return '流量重置周期不正确'
  if (!trafficBillingDirectionOptions.some((item) => item.value === editor.traffic_billing_direction)) return '流量计费方向不正确'
  if ((editor.heartbeat_interval ?? 0) < 3) return '心跳间隔不能小于 3 秒'
  if ((editor.metrics_interval ?? 0) < 3) return '指标间隔不能小于 3 秒'
  if ((editor.snapshot_interval ?? 0) < 15 || (editor.snapshot_interval ?? 0) > 3600) return '快照采集间隔需要在 15 到 3600 秒之间'
  if ((editor.snapshot_process_limit ?? 0) < 1 || (editor.snapshot_process_limit ?? 0) > 50) return '进程采集数量需要在 1 到 50 之间'
  if ((editor.snapshot_connection_limit ?? 0) < 1 || (editor.snapshot_connection_limit ?? 0) > 500) return '连接采集数量需要在 1 到 500 之间'
  if (price < 0) return '金额不能为负数'
  if (!hasAtMostTwoDecimals(price)) return '金额最多支持两位小数'
  if (trafficLimit < 0) return '总流量不能为负数'
  if (trafficCalibration < 0) return '校准流量不能为负数'
  const tagError = validateTagInput(editor.tag)
  if (tagError) return tagError
  if (!hasAtMostTwoDecimals(trafficLimit)) return '总流量最多支持两位小数'
  if (!hasAtMostTwoDecimals(trafficCalibration)) return '校准流量最多支持两位小数'
  const trafficLimitBytes = bytesFromTraffic(trafficLimit, editor.traffic_limit_unit)
  const trafficCalibrationBytes = bytesFromTraffic(trafficCalibration, editor.traffic_limit_unit)
  if (trafficLimitBytes > 0 && trafficCalibrationBytes > trafficLimitBytes) return '校准流量不能大于总流量'
  const [startedAt, expiresAt] = editor.service_range ?? []
  if (startedAt && expiresAt && startedAt > expiresAt) return '服务开始日期不能晚于结束日期'
  return ''
}

function validateProbeTaskEditor() {
  const editor = probeTaskEditor.value
  if ([...(editor.name || '').trim()].length > 64) return 'Ping 节点名称不能超过 64 个字符'
  if (!editor.target.trim()) return 'Ping 地址不能为空'
  if ([...editor.target.trim()].length > 255) return 'Ping 地址不能超过 255 个字符'
  if (!probeTypeValues.includes(editor.type)) return 'Ping 类型不正确'
  if (!probeIPVersionValues.includes(editor.ip_version)) return 'IP 版本不正确'
  if ((editor.interval_seconds ?? 0) < 3) return 'Ping 间隔不能小于 3 秒'
  if ((editor.timeout_ms ?? 0) < 100 || (editor.timeout_ms ?? 0) > 30000) return '超时时间需要在 100 到 30000 毫秒之间'
  return ''
}

function validateSettingsEditor(scope: SettingsSaveScope) {
  if (scope !== 'display') return ''
  const siteNameValue = String(settingsEditor.value.site_name || '').trim()
  if (!siteNameValue) return '网站名称不能为空'
  if ([...siteNameValue].length > 40) return '网站名称不能超过 40 个字符'
  if (/[\u0000-\u001f\u007f]/.test(siteNameValue)) return '网站名称不能包含控制字符'
  const siteDescriptionValue = String(settingsEditor.value.site_description || '').trim()
  if ([...siteDescriptionValue].length > 80) return '网站说明不能超过 80 个字符'
  if (/[\u0000-\u001f\u007f]/.test(siteDescriptionValue)) return '网站说明不能包含控制字符'
  const siteAvatarError = validateImageURLField('站点头像地址', settingsEditor.value.site_avatar_url)
  if (siteAvatarError) return siteAvatarError
  const userAvatarError = validateImageURLField('右侧头像地址', settingsEditor.value.user_avatar_url)
  if (userAvatarError) return userAvatarError
  const backgroundError = validateImageURLField('首页背景图地址', settingsEditor.value.home_background_url)
  if (backgroundError) return backgroundError
  const adminPathValue = normalizeAdminPath(settingsEditor.value.admin_path)
  if (!adminPathValue) return '后台路径不能为空'
  if (adminPathValue.length <= 5) return '后台路径需要超过 5 个字符'
  if (!/^[A-Za-z0-9]+$/.test(adminPathValue)) return '后台路径只能使用英文字母和数字'
  if (['api', 'healthz', 'themes'].includes(adminPathValue.toLowerCase())) return '后台路径不能使用保留路径'
  return ''
}

function validateImageURLField(label: string, value?: string | null) {
  const raw = String(value || '').trim()
  if (!raw) return ''
  if ([...raw].length > 2048) return `${label}不能超过 2048 个字符`
  if (/[\u0000-\u001f\u007f]/.test(raw)) return `${label}不能包含控制字符`
  if (isSameOriginImageURL(raw)) return ''
  if (!/^https?:\/\//i.test(raw)) return `${label}必须以 http://、https:// 或 / 开头`
  try {
    const parsed = new URL(raw)
    if (parsed.protocol === 'http:' || parsed.protocol === 'https:') return ''
  } catch {
    // handled below
  }
  return `${label}必须是有效的 http(s) 地址或站内绝对路径`
}

function hasAtMostTwoDecimals(value: number) {
  return Math.abs(value * 100 - Math.round(value * 100)) < 0.000001
}

function parseTagTokens(raw?: string | null) {
  return String(raw || '').trim().split(/[\/,，、]+/)
}

function validateTagInput(raw?: string | null) {
  const value = String(raw || '').trim()
  if (!value) return ''
  if (/\s/.test(value)) return 'Tag 不能包含空格或空白字符'
  const tokens = parseTagTokens(value)
  if (tokens.some((item) => item.length === 0)) return 'Tag 分隔符前后不能为空'
  if (tokens.length > 5) return 'Tag 最多 5 个'
  const totalLength = tokens.reduce((total, item) => total + item.length, 0)
  if (totalLength > 25) return 'Tag 总长度不能超过 25 个字符（不含分隔符）'
  return ''
}

function normalizeTagInput(raw?: string | null) {
  const value = String(raw || '').trim()
  if (!value) return ''
  return parseTagTokens(value).join('/')
}

function formatBytes(value?: number | null) {
  if (value === undefined || value === null) return 'N/A'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let current = value
  let unit = 0
  while (current >= 1024 && unit < units.length - 1) {
    current /= 1024
    unit++
  }
  return `${current >= 10 || unit === 0 ? current.toFixed(0) : current.toFixed(1)} ${units[unit]}`
}

function formatMegabytes(value?: number | null) {
  return formatBytes(value)
}

function parseSnapshotJSON<T>(raw: string | undefined | null, fallback: T): T {
  if (!raw) return fallback
  try {
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}

function formatSnapshotTime(snapshot?: NodeSnapshot | null) {
  if (!snapshot?.ts) return '-'
  return formatRangeWindowTime(snapshot.ts)
}

function formatEndpoint(addr: string, port: number) {
  if (!addr && !port) return '-'
  return `${addr || '*'}:${port || '*'}`
}

function formatSnapshotCPU(value?: number | null) {
  if (value === undefined || value === null) return '0.0%'
  return `${value.toFixed(1)}%`
}

function formatBps(value?: number | null) {
  if (value === undefined || value === null) return 'N/A'
  const units = ['bps', 'Kbps', 'Mbps', 'Gbps', 'Tbps']
  let current = value
  let unit = 0
  while (current >= 1000 && unit < units.length - 1) {
    current /= 1000
    unit++
  }
  return `${current >= 10 || unit === 0 ? current.toFixed(0) : current.toFixed(1)} ${units[unit]}`
}

function formatTime(ts?: number | null) {
  if (!ts) return 'Never'
  return new Intl.DateTimeFormat('zh-CN', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  }).format(new Date(ts))
}

function formatLogTime(value?: string | null) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  }).format(date)
}

function systemLogLevelType(level?: string): 'default' | 'primary' | 'info' | 'success' | 'warning' | 'error' {
  const normalized = normalizeSystemLogLevel(level)
  if (normalized === 'error') return 'error'
  if (normalized === 'warning') return 'warning'
  return 'info'
}

function normalizeSystemLogLevel(level?: string | null) {
  const normalized = String(level || '').trim().toLowerCase()
  if (normalized.includes('error')) return 'error'
  if (normalized.includes('warn')) return 'warning'
  return 'info'
}

function systemLogRowKey(row: SystemLog) {
  return row.id || `${row.created_at}-${row.service}-${row.event_type}-${row.message}`
}

function uniqueLogOptions(values: string[]) {
  return Array.from(new Set(values.map((item) => String(item || '').trim()).filter(Boolean)))
    .sort((a, b) => a.localeCompare(b))
    .map((value) => ({ label: value, value }))
}

function hasSystemLogMeta(row: SystemLog) {
  const value = String(row.meta_json || '').trim()
  return value !== '' && value !== '{}' && value !== 'null'
}

function formatSystemLogMeta(row: SystemLog) {
  const value = String(row.meta_json || '').trim()
  if (!value) return '{}'
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

function formatDate(ts?: number | null) {
  if (!ts) return '未设置'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  }).format(new Date(ts))
}

function formatAgo(ts?: number | null) {
  if (!ts) return '从未在线'
  const seconds = Math.max(0, Math.floor((Date.now() - ts) / 1000))
  if (seconds < 60) return '刚刚'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  return `${Math.floor(hours / 24)} 天前`
}

function formatDuration(seconds?: number) {
  if (!seconds) return 'N/A'
  const value = seconds
  const days = Math.floor(value / 86400)
  const hours = Math.floor((value % 86400) / 3600)
  const minutes = Math.floor((value % 3600) / 60)
  if (days > 0) return `${days} 天 ${hours} 小时`
  if (hours > 0) return `${hours} 小时 ${minutes} 分钟`
  return `${minutes} 分钟`
}

function emptyNodeEditor(): NodeEditor {
  return {
    name: '',
    region: '',
    provider: '',
    network_line: [],
    tag: '',
    heartbeat_interval: 15,
    metrics_interval: 15,
    snapshot_override: false,
    snapshot_enabled: false,
    snapshot_collect_processes: true,
    snapshot_collect_connections: true,
    snapshot_mask_sensitive: true,
    snapshot_interval: 60,
    snapshot_process_limit: 20,
    snapshot_connection_limit: 200,
    billing_cycle: 'monthly',
    price_amount: 0,
    currency: 'CNY',
    service_range: null,
    traffic_limit_value: 0,
    traffic_calibration_value: 0,
    traffic_limit_unit: 'GB',
    traffic_billing_direction: 'bidirectional',
    traffic_reset_cycle: 'monthly',
    probe_task_ids: []
  }
}

function emptyProbeTaskEditor(): ProbeTaskEditor {
  return {
    id: null,
    name: '',
    type: 'tcp_ping',
    ip_version: 'auto',
    target: '',
    interval_seconds: 60,
    timeout_ms: 3000,
    enabled: true,
    assign_to_all_agents: false
  }
}

function editorFromProbeTask(task: ProbeTask): ProbeTaskEditor {
  return {
    id: task.id,
    name: task.name,
    type: probeTypeValues.includes(task.type) ? task.type as ProbeTaskType : 'tcp_ping',
    ip_version: probeIPVersionValues.includes(task.ip_version) ? task.ip_version as ProbeIPVersion : 'auto',
    target: task.target,
    interval_seconds: task.interval_seconds || 60,
    timeout_ms: task.timeout_ms || 3000,
    enabled: task.enabled,
    assign_to_all_agents: false
  }
}

function probeTaskPayload() {
  return {
    name: probeTaskEditor.value.name.trim(),
    type: probeTaskEditor.value.type,
    ip_version: probeTaskEditor.value.ip_version,
    target: probeTaskEditor.value.target.trim(),
    interval_seconds: probeTaskEditor.value.interval_seconds,
    timeout_ms: probeTaskEditor.value.timeout_ms,
    enabled: probeTaskEditor.value.enabled,
    assign_to_all_agents: probeTaskEditor.value.id ? false : probeTaskEditor.value.assign_to_all_agents
  }
}

function normalizedProbeTaskIDs(ids: number[]) {
  return Array.from(new Set(ids.map((id) => Number(id)).filter((id) => Number.isFinite(id) && id > 0)))
}

function addProbeTaskToLocalNodes(taskID: number) {
  if (!Number.isFinite(taskID) || taskID <= 0) return
  nodes.value = nodes.value.map((node) => ({
    ...node,
    probe_task_ids: normalizedProbeTaskIDs([...(node.probe_task_ids ?? []), taskID])
  }))
  if (nodeEditOpen.value && adminEditNodeID.value) {
    nodeEditor.value = {
      ...nodeEditor.value,
      probe_task_ids: normalizedProbeTaskIDs([...nodeEditor.value.probe_task_ids, taskID])
    }
  }
}

function removeProbeTaskFromLocalNodes(taskID: number) {
  if (!Number.isFinite(taskID) || taskID <= 0) return
  nodes.value = nodes.value.map((node) => ({
    ...node,
    probe_task_ids: normalizedProbeTaskIDs(node.probe_task_ids ?? []).filter((id) => id !== taskID)
  }))
  if (nodeEditOpen.value && adminEditNodeID.value) {
    nodeEditor.value = {
      ...nodeEditor.value,
      probe_task_ids: normalizedProbeTaskIDs(nodeEditor.value.probe_task_ids).filter((id) => id !== taskID)
    }
  }
}

function isProbeTaskSelected(taskID: number) {
  return nodeEditor.value.probe_task_ids.includes(taskID)
}

function setProbeTaskSelected(taskID: number, selected: boolean) {
  const current = normalizedProbeTaskIDs(nodeEditor.value.probe_task_ids)
  if (selected) {
    nodeEditor.value.probe_task_ids = normalizedProbeTaskIDs([...current, taskID])
    return
  }
  nodeEditor.value.probe_task_ids = current.filter((id) => id !== taskID)
}

function selectedProbeTaskCount() {
  const enabledIDs = new Set(enabledProbeTasks.value.map((task) => task.id))
  return normalizedProbeTaskIDs(nodeEditor.value.probe_task_ids).filter((id) => enabledIDs.has(id)).length
}

function editorFromNode(node: NodeRecord): NodeEditor {
  const traffic = trafficEditorFromBytes(node.traffic_limit_bytes || 0)
  return {
    name: node.name,
    region: regionEditorValue(node.region),
    provider: node.provider,
    network_line: networkLineSelectionFromValue(node.network_line),
    tag: node.tag || '',
    heartbeat_interval: node.heartbeat_interval || 15,
    metrics_interval: node.metrics_interval || 15,
    snapshot_override: Boolean(node.snapshot_override),
    snapshot_enabled: Boolean(node.snapshot_enabled),
    snapshot_collect_processes: node.snapshot_collect_processes !== false,
    snapshot_collect_connections: node.snapshot_collect_connections !== false,
    snapshot_mask_sensitive: node.snapshot_mask_sensitive !== false,
    snapshot_interval: node.snapshot_interval || 60,
    snapshot_process_limit: node.snapshot_process_limit || 20,
    snapshot_connection_limit: node.snapshot_connection_limit || 200,
    billing_cycle: node.billing_cycle || 'monthly',
    price_amount: node.price_amount || 0,
    currency: node.currency || 'CNY',
    service_range: node.service_started_at && node.service_expires_at
      ? [node.service_started_at, node.service_expires_at]
      : null,
    ...traffic,
    traffic_calibration_value: trafficValueFromBytes(node.traffic_calibration_bytes || 0, traffic.traffic_limit_unit),
    traffic_billing_direction: normalizeTrafficBillingDirection(node.traffic_billing_direction),
    traffic_reset_cycle: node.traffic_reset_cycle || 'monthly',
    probe_task_ids: [...(node.probe_task_ids ?? [])]
  }
}

function nodeUpdatePayload() {
  return {
    name: nodeEditor.value.name,
    region: normalizeRegionCode(nodeEditor.value.region),
    provider: nodeEditor.value.provider,
    network_line: networkLineValue(nodeEditor.value.network_line),
    tag: normalizeTagInput(nodeEditor.value.tag),
    heartbeat_interval: nodeEditor.value.heartbeat_interval,
    metrics_interval: nodeEditor.value.metrics_interval,
    snapshot_override: nodeEditor.value.snapshot_override,
    snapshot_enabled: nodeEditor.value.snapshot_enabled,
    snapshot_collect_processes: nodeEditor.value.snapshot_collect_processes,
    snapshot_collect_connections: nodeEditor.value.snapshot_collect_connections,
    snapshot_mask_sensitive: nodeEditor.value.snapshot_mask_sensitive,
    snapshot_interval: nodeEditor.value.snapshot_interval,
    snapshot_process_limit: nodeEditor.value.snapshot_process_limit,
    snapshot_connection_limit: nodeEditor.value.snapshot_connection_limit,
    billing_cycle: nodeEditor.value.billing_cycle,
    price_amount: nodeEditor.value.price_amount,
    currency: nodeEditor.value.currency,
    service_started_at: normalizeDateValue(nodeEditor.value.service_range?.[0] ?? null),
    service_expires_at: normalizeDateValue(nodeEditor.value.service_range?.[1] ?? null),
    traffic_limit_bytes: bytesFromTraffic(nodeEditor.value.traffic_limit_value, nodeEditor.value.traffic_limit_unit),
    traffic_calibration_bytes: bytesFromTraffic(nodeEditor.value.traffic_calibration_value, nodeEditor.value.traffic_limit_unit),
    traffic_billing_direction: nodeEditor.value.traffic_billing_direction,
    traffic_reset_cycle: nodeEditor.value.traffic_reset_cycle,
    probe_task_ids: normalizedProbeTaskIDs(nodeEditor.value.probe_task_ids)
  }
}

function networkLineValue(value?: string[] | null) {
  return normalizeNetworkLineSelection(value).join('/')
}

function normalizeNetworkLineSelection(value?: string[] | null) {
  const selected: string[] = []
  for (const item of value ?? []) {
    const normalized = networkLineValueMap.get(String(item).trim().toLowerCase())
    if (normalized && !selected.includes(normalized)) {
      selected.push(normalized)
    }
  }
  return selected
}

function networkLineSelectionFromValue(value?: string | null) {
  const selected: string[] = []
  const parts = String(value || '').split(/[\/,，、]/).map((item) => item.trim()).filter(Boolean)
  for (const part of parts) {
    const normalized = networkLineValueMap.get(part.toLowerCase())
    if (normalized && !selected.includes(normalized)) {
      selected.push(normalized)
    }
  }
  if (parts.length > 0 && selected.length === 0) {
    selected.push('其他')
  }
  return selected
}

function networkLineNodeProps(option: unknown) {
  const value = typeof option === 'object' && option !== null && 'value' in option
    ? (option as { value?: unknown }).value
    : ''
  return {
    'data-network-line-value': String(value ?? '')
  }
}

function scrollNetworkLineMenuToFirstSelected() {
  const firstSelected = normalizeNetworkLineSelection(nodeEditor.value.network_line)[0]
  if (!firstSelected || typeof document === 'undefined' || typeof window === 'undefined') return
  window.requestAnimationFrame(() => {
    const menu = document.querySelector('.network-line-select-menu')
    if (!(menu instanceof HTMLElement)) return
    const option = Array.from(menu.querySelectorAll<HTMLElement>('[data-network-line-value]'))
      .find((item) => item.dataset.networkLineValue === firstSelected)
    option?.scrollIntoView({ block: 'center' })
  })
}

function handleNetworkLineSelectShow(show: boolean) {
  if (show) {
    void nextTick(scrollNetworkLineMenuToFirstSelected)
  }
}

function displayNetworkLine(value?: string | null) {
  const lines = networkLineSelectionFromValue(value)
  return lines.length ? lines.join(' / ') : '-'
}

function normalizeDateValue(value: number | null) {
  if (!value) return null
  const date = new Date(value)
  date.setHours(0, 0, 0, 0)
  return date.getTime()
}

function normalizeRegionCode(region: string) {
  const value = region.trim()
  if (!value || value.toLowerCase() === 'auto' || value.toLowerCase() === 'default') return 'default'
  return value.toUpperCase()
}

function displayRegion(region?: string | null) {
  const code = normalizeRegionCode(region ?? '')
  if (code === 'default') return '默认'
  const option = regionOptions.value.find((item) => item.value.toLowerCase() === code.toLowerCase())
  if (!option) return code
  return option.label.replace(new RegExp(`\\s+${option.value}$`, 'i'), '')
}

function regionEditorValue(region: string) {
  const code = normalizeRegionCode(region)
  if (regionOptions.value.some((item) => item.value === code)) return code

  const normalized = region.trim().toLowerCase()
  const aliases: Array<[string, string[]]> = [
    ['HK', ['hong kong', '香港']],
    ['US', ['usa', 'united states', '美国', '美國']],
    ['JP', ['japan', 'tokyo', 'osaka', '日本']],
    ['SG', ['singapore', '新加坡']],
    ['TW', ['taiwan', '台湾', '台灣']],
    ['KR', ['korea', 'seoul', '韩国', '韓國']],
    ['CN', ['china', '中国', '中國']],
    ['DE', ['germany', 'frankfurt', '德国', '德國']],
    ['FR', ['france', 'paris', '法国', '法國']],
    ['GB', ['uk', 'united kingdom', 'london', '英国', '英國']],
    ['NL', ['netherlands', 'amsterdam', '荷兰', '荷蘭']],
    ['CA', ['canada', 'toronto', '加拿大']],
    ['AU', ['australia', 'sydney', '澳大利亚', '澳洲']]
  ]
  return aliases.find(([, values]) => values.some((value) => normalized.includes(value)))?.[0] ?? 'default'
}

function trafficEditorFromBytes(bytes: number): Pick<NodeEditor, 'traffic_limit_value' | 'traffic_limit_unit'> {
  if (!bytes) {
    return { traffic_limit_value: 0, traffic_limit_unit: 'GB' }
  }

  const tb = 1024 ** 4
  const gb = 1024 ** 3
  const mb = 1024 ** 2
  if (bytes >= tb && bytes % tb === 0) {
    return { traffic_limit_value: bytes / tb, traffic_limit_unit: 'TB' }
  }
  if (bytes >= gb) {
    return { traffic_limit_value: Number((bytes / gb).toFixed(2)), traffic_limit_unit: 'GB' }
  }
  return { traffic_limit_value: Number((bytes / mb).toFixed(2)), traffic_limit_unit: 'MB' }
}

function trafficValueFromBytes(bytes: number, unit: TrafficUnit) {
  const multipliers: Record<TrafficUnit, number> = {
    MB: 1024 ** 2,
    GB: 1024 ** 3,
    TB: 1024 ** 4
  }
  if (!bytes) return 0
  return Number((bytes / multipliers[unit]).toFixed(2))
}

function bytesFromTraffic(value: number, unit: TrafficUnit) {
  const multipliers: Record<TrafficUnit, number> = {
    MB: 1024 ** 2,
    GB: 1024 ** 3,
    TB: 1024 ** 4
  }
  return Math.max(0, Math.round((value || 0) * multipliers[unit]))
}

function openNodeEditor(node: NodeRecord) {
  adminEditNodeID.value = node.node_id
  nodeEditor.value = editorFromNode(node)
  nodeEditOpen.value = true
}

function nodeLabel(node: NodeRecord) {
  return node.name || node.node_id
}

function shouldMaskIP() {
  return !isLoggedIn.value || appSettings.value.mask_ip_addresses
}

function displayIP(value?: string | null) {
  if (!value) return ''
  return shouldMaskIP() ? maskIPInText(value) : value
}

function displayIPList(values: string[]) {
  return values.map((value) => displayIP(value)).filter(Boolean).join(', ')
}

function displayTarget(value?: string | null) {
  if (!value) return ''
  return shouldMaskIP() ? maskIPInText(value) : value
}

function maskIPInText(value: string) {
  const maskedIPv4 = value.replace(/\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})(:\d+)?\b/g, (_match, first: string, _second: string, _third: string, _fourth: string, port?: string) => {
    return `${first}.x.x.x${port ?? ''}`
  })
  return maskedIPv4
    .replace(/\[([0-9a-fA-F:]{3,})\]/g, (match, ip: string) => isIPv6Literal(ip) ? `[${maskIPv6(ip)}]` : match)
    .replace(/\b([0-9a-fA-F]{0,4}:[0-9a-fA-F:]{2,})\b/g, (match, ip: string) => isIPv6Literal(ip) ? maskIPv6(ip) : match)
}

function isIPv6Literal(value: string) {
  return value.includes(':') && value.split(':').length >= 3 && /^[0-9a-fA-F:]+$/.test(value)
}

function maskIPv6(value: string) {
  const first = value.split(':').find(Boolean) ?? 'xxxx'
  return `${first}:xxxx:xxxx:xxxx`
}

function nodeIPAddresses(node?: NodeRecord | null): AgentIPAddresses {
  if (!node) return {}
  if (node.ip_addresses) {
    return normalizeAgentIPAddresses(node.ip_addresses)
  }
  if (!node.ip_addresses_json) return {}
  try {
    return normalizeAgentIPAddresses(JSON.parse(node.ip_addresses_json) as AgentIPAddresses)
  } catch {
    return {}
  }
}

function nodePublicIPs(node?: NodeRecord | null): PublicIPs {
  if (!node) return {}
  if (node.public_ips) {
    return normalizePublicIPs(node.public_ips)
  }
  if (!node.public_ips_json) return {}
  try {
    return normalizePublicIPs(JSON.parse(node.public_ips_json) as PublicIPs)
  } catch {
    return {}
  }
}

function normalizeAgentIPAddresses(value: AgentIPAddresses): AgentIPAddresses {
  return {
    ipv4: uniqueStrings(value.ipv4 ?? []),
    ipv6: uniqueStrings(value.ipv6 ?? [])
  }
}

function normalizePublicIPs(value: PublicIPs): PublicIPs {
  return {
    ipv4: uniquePublicIPObservations(value.ipv4 ?? []),
    ipv6: uniquePublicIPObservations(value.ipv6 ?? [])
  }
}

function uniquePublicIPObservations(values: PublicIPObservation[]) {
  const seen = new Set<string>()
  const result: PublicIPObservation[] = []
  for (const item of values) {
    const ip = item.ip?.trim()
    if (!ip || seen.has(ip)) continue
    seen.add(ip)
    result.push({ ...item, ip })
  }
  return result
}

function uniqueStrings(values: string[]) {
  return Array.from(new Set(values.map((value) => value.trim()).filter(Boolean)))
}

function nodeIPv4List(node?: NodeRecord | null) {
  return nodeIPAddresses(node).ipv4 ?? []
}

function nodeIPv6List(node?: NodeRecord | null) {
  return nodeIPAddresses(node).ipv6 ?? []
}

function nodePublicIPv4List(node?: NodeRecord | null) {
  return nodePublicIPs(node).ipv4?.map((item) => item.ip).filter(isPublicIPv4Literal) ?? []
}

function nodePublicIPv6List(node?: NodeRecord | null) {
  return nodePublicIPs(node).ipv6?.map((item) => item.ip).filter(isPublicIPv6Literal) ?? []
}

function nodePrimaryPublicIPv4(node?: NodeRecord | null) {
  if (!node) return ''
  const publicIP = node.public_ip?.trim()
  if (publicIP && isPublicIPv4Literal(publicIP)) return displayIP(publicIP)
  const fallback = nodePublicIPv4List(node)[0] || ''
  return displayIP(fallback)
}

function nodePrimaryPublicIPv6(node?: NodeRecord | null) {
  if (!node) return ''
  const publicIPv6 = node.public_ipv6?.trim()
  if (publicIPv6 && isPublicIPv6Literal(publicIPv6)) return displayIP(publicIPv6)
  const legacyPublicIP = node.public_ip?.trim()
  if (legacyPublicIP && isPublicIPv6Literal(legacyPublicIP)) return displayIP(legacyPublicIP)
  const fallback = nodePublicIPv6List(node)[0] || ''
  return displayIP(fallback)
}

function nodePrimaryIP(node?: NodeRecord | null) {
  if (!node) return ''
  const publicIPv4 = nodePrimaryPublicIPv4(node)
  if (publicIPv4) return publicIPv4
  const publicIPv6 = nodePrimaryPublicIPv6(node)
  if (publicIPv6) return publicIPv6
  return nodePrimaryLocalIP(node)
}

function nodePrimaryLocalIP(node?: NodeRecord | null) {
  if (!node) return ''
  const addresses = nodeIPAddresses(node)
  const fallback = addresses.ipv4?.[0] || addresses.ipv6?.[0] || ''
  return displayIP(fallback)
}

function nodeIPSummary(node: NodeRecord) {
  const publicIPv4 = nodePrimaryPublicIPv4(node)
  const publicIPv6 = nodePrimaryPublicIPv6(node)
  return [publicIPv4, publicIPv6].filter(Boolean).join(' / ')
}

function nodeIPListText(node?: NodeRecord | null) {
  if (!node) return ''
  const lines: string[] = []
  const publicIPv4 = nodePrimaryPublicIPv4(node)
  const publicIPv6 = nodePrimaryPublicIPv6(node)
  const ipv4 = nodeIPv4List(node)
  const ipv6 = nodeIPv6List(node)
  if (publicIPv4) lines.push(`公网 IPv4: ${publicIPv4}`)
  if (publicIPv6) lines.push(`公网 IPv6: ${publicIPv6}`)
  if (ipv4.length) lines.push(`本机 IPv4: ${displayIPList(ipv4)}`)
  if (ipv6.length) lines.push(`本机 IPv6: ${displayIPList(ipv6)}`)
  return lines.join('\n')
}

function isIPv4Literal(value: string) {
  return /^\d{1,3}(?:\.\d{1,3}){3}$/.test(value)
}

function isPublicIPv4Literal(value: string) {
  if (!isIPv4Literal(value)) return false
  const parts = value.split('.').map((part) => Number(part))
  if (parts.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return false
  const [first, second] = parts
  if (first === 10 || first === 127 || first === 0 || first >= 224) return false
  if (first === 172 && second >= 16 && second <= 31) return false
  if (first === 192 && second === 168) return false
  if (first === 169 && second === 254) return false
  if (first === 100 && second >= 64 && second <= 127) return false
  return true
}

function isPublicIPv6Literal(value: string) {
  if (!isIPv6Literal(value)) return false
  const normalized = value.trim().toLowerCase()
  if (normalized === '::' || normalized === '::1') return false
  if (normalized.startsWith('fe80:') || normalized.startsWith('fc') || normalized.startsWith('fd') || normalized.startsWith('ff')) return false
  return true
}

function isNodeOnline(node: NodeRecord) {
  return node.status === 'online'
}

function liveMetric(node: NodeRecord): NodeMetric | null {
  const metric = node.latest_metric
  if (!metric) {
    return isNodeOnline(node) ? null : zeroMetric(node.node_id)
  }
  if (isNodeOnline(node)) return metric
  return {
    ...metric,
    cpu_usage: 0,
    load1: 0,
    load5: 0,
    load15: 0,
    mem_used_percent: 0,
    disk_used_percent: 0,
    net_rx_bps: 0,
    net_tx_bps: 0
  }
}

function zeroMetric(nodeID: string): NodeMetric {
  return {
    id: 0,
    node_id: nodeID,
    ts: Date.now(),
    cpu_usage: 0,
    cpu_cores: 0,
    arch: '',
    virtualization: '',
    gpu: '',
    os_name: '',
    load1: 0,
    load5: 0,
    load15: 0,
    mem_total: 0,
    mem_used: 0,
    mem_used_percent: 0,
    swap_total: 0,
    swap_used: 0,
    swap_used_percent: 0,
    disk_total: 0,
    disk_used: 0,
    disk_used_percent: 0,
    net_rx_bps: 0,
    net_tx_bps: 0,
    net_rx_bytes_total: 0,
    net_tx_bytes_total: 0,
    uptime_seconds: 0,
    created_at: ''
  }
}

function countryFlagCode(region?: string | null) {
  const rawRegion = region?.trim() ?? ''
  if (!rawRegion || rawRegion.toLowerCase() === 'auto' || rawRegion.toLowerCase() === 'default') return 'default'

  const normalized = rawRegion.toLowerCase()
  const option = regionOptions.value.find((item) => item.value.toLowerCase() === normalized)
  const optionCode = flagCodeFromRegionValue(option?.value || '')
  if (optionCode !== 'default') return optionCode

  const directCode = flagCodeFromRegionValue(rawRegion)
  if (directCode !== 'default') return directCode

  const tokens = normalized.split(/[^a-z0-9\u4e00-\u9fa5]+/).filter(Boolean)
  const hasToken = (...values: string[]) => values.some((value) => tokens.includes(value))
  const hasText = (...values: string[]) => values.some((value) => normalized.includes(value))

  if (hasToken('hk') || hasText('hong kong', '香港')) return 'hk'
  if (hasToken('tw') || hasText('taiwan', '台湾', '台灣')) return 'tw'
  if (hasToken('jp') || hasText('japan', 'tokyo', 'osaka', '日本')) return 'jp'
  if (hasToken('sg') || hasText('singapore', '新加坡')) return 'sg'
  if (hasToken('us', 'usa') || hasText('united states', 'los angeles', 'ashburn', 'new york', '美国', '美國')) return 'us'
  if (hasToken('cn') || hasText('china', 'mainland', '中国', '中國')) return 'cn'
  if (hasToken('kr') || hasText('korea', 'seoul', '韩国', '韓國')) return 'kr'
  if (hasToken('de') || hasText('germany', 'frankfurt', '德国', '德國')) return 'de'
  if (hasToken('fr') || hasText('france', 'paris', '法国', '法國')) return 'fr'
  if (hasToken('uk', 'gb') || hasText('united kingdom', 'london', '英国', '英國')) return 'gb'
  if (hasToken('nl') || hasText('netherlands', 'amsterdam', '荷兰', '荷蘭')) return 'nl'
  if (hasToken('ca') || hasText('canada', 'toronto', '加拿大')) return 'ca'
  if (hasToken('au') || hasText('australia', 'sydney', '澳大利亚', '澳洲')) return 'au'
  return 'default'
}

function flagCodeFromRegionValue(value: string) {
  const code = value.trim().toLowerCase()
  if (!code || code === 'auto' || code === 'default') return 'default'
  if (code === 'uk') return 'gb'
  if (/^[a-z]{2}$/.test(code)) return code
  return 'default'
}

function regionFlagClass(region?: string | null) {
  return ['region-flag-icon', { 'region-flag-default': !regionFlagURL(region) }]
}

function regionFlagURL(region?: string | null) {
  return regionFlagURLs[countryFlagCode(region)] ?? ''
}

function regionFlagStyle(region?: string | null) {
  const url = regionFlagURL(region)
  return url ? { backgroundImage: `url(${url})` } : undefined
}

function regionFlagLabel(region?: string | null) {
  if (!regionFlagURL(region)) return '默认地区'
  return `${displayRegion(region)}地区旗帜`
}

function renderRegionOptionLabel(option: SelectOption) {
  const value = String(option.value ?? '')
  const label = String(option.label ?? (value || '默认'))
  return h('span', { class: 'region-select-option' }, [
    h('span', {
      class: ['region-select-flag', ...regionFlagClass(value)],
      style: regionFlagStyle(value),
      role: 'img',
      'aria-label': regionFlagLabel(value)
    }),
    h('span', label)
  ])
}

function nodeTags(node: NodeRecord) {
  return parseTagTokens(node.tag)
    .map((item) => item.trim())
    .filter(Boolean)
}

function nodeHoverMeta(node: NodeRecord) {
  const lines: Array<{ type: string; text: string }> = []
  const publicIPv4 = nodePrimaryPublicIPv4(node)
  const publicIPv6 = nodePrimaryPublicIPv6(node)
  if (publicIPv4) {
    lines.push({ type: 'meta', text: `公网 IPv4: ${publicIPv4}` })
  }
  if (publicIPv6) {
    lines.push({ type: 'meta', text: `公网 IPv6: ${publicIPv6}` })
  }
  if (node.region) {
    lines.push({ type: 'meta', text: `地区: ${displayRegion(node.region)}` })
  }
  return lines
}

function nodeHoverTags(node: NodeRecord) {
  if (!appSettings.value.show_node_tags) return []
  return nodeTags(node).map((tag) => `#${tag}`)
}

function billingSummary(node: NodeRecord) {
  return `${formatMoney(node.price_amount, node.currency)}(${cycleLabel(node.billing_cycle)})`
}

function packageBillingSummary(node: NodeRecord) {
  return `${formatMoney(node.price_amount, node.currency)}/${cycleShortLabel(node.billing_cycle)}`
}

function remainingPackageSummary(node: NodeRecord) {
  return remainingDaysLabel(node)
}

function remainingPackageTitle(node: NodeRecord) {
  return formatMoney(node.remaining_value, node.currency)
}

function remainingDaysLabel(node: NodeRecord) {
  if (!node.service_expires_at) return '未设置'
  return node.remaining_days <= 0 ? '已到期' : `${node.remaining_days}天`
}

function cycleShortLabel(cycle?: string) {
  switch (cycle) {
    case 'daily':
      return '天'
    case 'yearly':
      return '年'
    case 'one_time':
      return '次'
    case 'monthly':
    default:
      return '月'
  }
}

function formatLoad(metric?: NodeMetric | null) {
  if (!metric) return 'N/A'
  const loads = [
    { label: '1m', value: metric.load1 },
    { label: '5m', value: metric.load5 },
    { label: '15m', value: metric.load15 }
  ]
  const current = loads[Math.floor(currentTime.value / 3000) % loads.length]
  return `${current.label} ${current.value.toFixed(2)}`
}

function formatLoadTitle(metric?: NodeMetric | null) {
  if (!metric) return 'N/A'
  return `1m ${metric.load1.toFixed(2)} · 5m ${metric.load5.toFixed(2)} · 15m ${metric.load15.toFixed(2)}`
}

function cycleLabel(cycle?: string) {
  if (cycle === 'daily') return '天付'
  if (cycle === 'yearly') return '年付'
  if (cycle === 'one_time') return '一次性'
  return '月付'
}

function resetCycleLabel(cycle?: string) {
  if (cycle === 'daily') return '每日重置'
  if (cycle === 'yearly') return '每年重置'
  if (cycle === 'never') return '不重置'
  return '每月重置'
}

function trafficResetSummary(node: NodeRecord) {
  const label = resetCycleLabel(node.traffic_reset_cycle)
  const nextReset = nextTrafficResetAt(node)
  if (!nextReset) return label
  return `${label} · 下次 ${formatResetDate(nextReset)}`
}

function trafficRemainingLine(node: NodeRecord) {
  const remaining = `剩余 ${formatRemainingTraffic(node)}`
  return `${remaining} · ${trafficResetSummary(node)}`
}

function nextTrafficResetAt(node: NodeRecord) {
  const cycle = node.traffic_reset_cycle
  if (cycle === 'never') return null
  const now = new Date(currentTime.value)
  const year = now.getFullYear()
  const month = now.getMonth()
  const date = now.getDate()
  if (cycle === 'daily') return new Date(year, month, date + 1, 0, 0, 0, 0)
  if (cycle === 'yearly') {
    if (!node.service_started_at) return new Date(year + 1, 0, 1, 0, 0, 0, 0)
    return nextAnchoredYearlyReset(new Date(node.service_started_at), now)
  }
  if (!node.service_started_at) return new Date(year, month + 1, 1, 0, 0, 0, 0)
  return nextAnchoredMonthlyReset(new Date(node.service_started_at), now)
}

function nextAnchoredMonthlyReset(anchor: Date, now: Date) {
  const day = anchor.getDate()
  const candidate = anchoredResetDate(now.getFullYear(), now.getMonth(), day, anchor)
  if (candidate.getTime() > now.getTime()) return candidate
  return anchoredResetDate(now.getFullYear(), now.getMonth() + 1, day, anchor)
}

function nextAnchoredYearlyReset(anchor: Date, now: Date) {
  const day = anchor.getDate()
  const month = anchor.getMonth()
  const candidate = anchoredResetDate(now.getFullYear(), month, day, anchor)
  if (candidate.getTime() > now.getTime()) return candidate
  return anchoredResetDate(now.getFullYear() + 1, month, day, anchor)
}

function anchoredResetDate(year: number, month: number, day: number, anchor: Date) {
  return new Date(
    year,
    month,
    Math.min(day, daysInDateMonth(year, month)),
    anchor.getHours(),
    anchor.getMinutes(),
    anchor.getSeconds(),
    anchor.getMilliseconds()
  )
}

function daysInDateMonth(year: number, month: number) {
  return new Date(year, month + 1, 0).getDate()
}

function formatResetDate(date: Date) {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${month}/${day}`
}

function probeTypeLabel(type?: string) {
  if (type === 'icmp') return 'ICMP'
  return 'TCP Ping'
}

function probeIPVersionLabel(ipVersion?: string) {
  if (ipVersion === 'ipv4') return 'IPv4'
  if (ipVersion === 'ipv6') return 'IPv6'
  return '自动'
}

function probeModeLabel(type?: string, ipVersion?: string) {
  return `${probeTypeLabel(type)} · ${probeIPVersionLabel(ipVersion)}`
}

function probeDisplayName(input: { task_name?: string; name?: string; target: string; type?: string }) {
  const name = input.task_name || input.name || input.target || probeTypeLabel(input.type)
  return shouldMaskIP() ? maskIPInText(name) : name
}

function buildProbeStats(tasks: ProbeTask[], results: ProbeResult[]): ProbeNodeStat[] {
  const groups = new Map<string, ProbeResult[]>()
  for (const result of results) {
    const key = probeGroupKey(result)
    const group = groups.get(key) ?? []
    group.push(result)
    groups.set(key, group)
  }

  const taskStats = tasks.map((task) => statFromResults({
    key: String(task.id),
    name: probeDisplayName(task),
    target: displayTarget(task.target),
    type: task.type,
    ipVersion: task.ip_version || 'auto',
    inactive: !task.enabled
  }, groups.get(String(task.id)) ?? []))

  const knownKeys = new Set(taskStats.map((item) => item.key))
  const orphanStats = Array.from(groups.entries())
    .filter(([key]) => !knownKeys.has(key))
    .map(([key, group]) => {
      const first = group[0]
      return statFromResults({
        key,
        name: probeDisplayName(first),
        target: displayTarget(first.target),
        type: first.type,
        ipVersion: first.ip_version || 'auto',
        inactive: true
      }, group)
    })

  return [...taskStats, ...orphanStats]
}

function statFromResults(base: Pick<ProbeNodeStat, 'key' | 'name' | 'target' | 'type' | 'ipVersion' | 'inactive'>, source: ProbeResult[]): ProbeNodeStat {
  const sorted = [...source].sort((left, right) => Date.parse(left.created_at) - Date.parse(right.created_at))
  const latest = sorted[sorted.length - 1]
  let samples = 0
  let successSamples = 0
  let failedSamples = 0
  let latencyTotal = 0
  let minLatency: number | null = null
  let maxLatency: number | null = null
  const bucketLatencies: number[] = []
  for (const item of sorted) {
    const itemSamples = probeSampleCount(item)
    const itemSuccessSamples = probeSuccessCount(item)
    const itemFailedSamples = probeFailedCount(item)
    samples += itemSamples
    successSamples += itemSuccessSamples
    failedSamples += itemFailedSamples
    if (item.latency_ms !== null && itemSuccessSamples > 0) {
      latencyTotal += item.latency_ms * itemSuccessSamples
      bucketLatencies.push(item.latency_ms)
      const itemMin = item.min_latency_ms ?? item.latency_ms
      const itemMax = item.max_latency_ms ?? item.latency_ms
      minLatency = minLatency === null ? itemMin : Math.min(minLatency, itemMin)
      maxLatency = maxLatency === null ? itemMax : Math.max(maxLatency, itemMax)
    }
  }
  const averageLatency = successSamples > 0 ? latencyTotal / successSamples : null

  return {
    ...base,
    latestLatency: latest?.status === 'success' && latest.latency_ms !== null ? latest.latency_ms : null,
    latestStatus: latest?.status ?? 'unknown',
    averageLatency,
    packetLoss: samples ? (failedSamples / samples) * 100 : null,
    jitter: jitter(bucketLatencies),
    samples,
    successSamples,
    failedSamples,
    minLatency,
    maxLatency
  }
}

function probeGroupKey(result: ProbeResult) {
  return result.task_id ? String(result.task_id) : `${result.target}|${result.type}|${result.ip_version || 'auto'}`
}

function probeSampleCount(item: ProbeResult) {
  return Math.max(1, item.samples ?? 1)
}

function probeSuccessCount(item: ProbeResult) {
  if (typeof item.success_samples === 'number') {
    return Math.max(0, item.success_samples)
  }
  return item.status === 'success' && item.latency_ms !== null ? 1 : 0
}

function probeFailedCount(item: ProbeResult) {
  if (typeof item.failed_samples === 'number') {
    return Math.max(0, item.failed_samples)
  }
  return Math.max(0, probeSampleCount(item) - probeSuccessCount(item))
}

function average(values: number[]) {
  if (values.length === 0) return null
  return values.reduce((total, value) => total + value, 0) / values.length
}

function bucketStart(timestamp: number, bucketMs: number) {
  return Math.floor(timestamp / bucketMs) * bucketMs
}

function jitter(values: number[]) {
  if (values.length < 2) return null
  let total = 0
  for (let index = 1; index < values.length; index++) {
    total += Math.abs(values[index] - values[index - 1])
  }
  return total / (values.length - 1)
}

function formatLatency(value?: number | null) {
  if (value === undefined || value === null) return 'N/A'
  return `${round(value)} ms`
}

function formatPacketLoss(value?: number | null) {
  if (value === undefined || value === null) return 'N/A'
  return `${round(value)}%`
}

function formatMoney(amount?: number | null, currency = 'CNY') {
  const value = amount ?? 0
  const symbols: Record<string, string> = {
    CNY: '¥',
    USD: '$',
    EUR: '€',
    GBP: '£',
    HKD: 'HK$',
    JPY: '¥'
  }
  return `${symbols[currency] ?? currency + ' '}${value.toFixed(2)}`
}

function formatAssetMoney(amount?: number | null) {
  return formatMoney(amount ?? 0, assetBaseCurrency.value)
}

function normalizeAssetCurrency(value?: string | null) {
  const normalized = String(value || '').trim().toUpperCase()
  return normalized.includes('USD') ? 'USD' : 'CNY'
}

function fallbackExchangeRates(base: string) {
  const cnyRates: Record<string, number> = {
    CNY: 1,
    USD: 7.2,
    EUR: 7.8,
    GBP: 9.1,
    HKD: 0.92,
    JPY: 0.05
  }
  const normalizedBase = normalizeAssetCurrency(base)
  const baseRate = cnyRates[normalizedBase] || 1
  return Object.fromEntries(Object.entries(cnyRates).map(([currency, rate]) => [currency, rate / baseRate]))
}

function convertAssetMoney(amount: number, currency: string) {
  const rate = exchangeRates.value[String(currency || 'CNY').toUpperCase()] ?? fallbackExchangeRates(assetBaseCurrency.value)[String(currency || 'CNY').toUpperCase()] ?? 1
  return amount * rate
}

function buildAssetRow(node: NodeRecord) {
  const monthlyCost = convertAssetMoney(monthlyNodeCost(node), node.currency)
  const annualCost = convertAssetMoney(annualNodeCost(node), node.currency)
  const remainingValue = convertAssetMoney(node.remaining_value || 0, node.currency)
  const renewalCost = convertAssetMoney(node.price_amount || 0, node.currency)
  const expiresInNextMonth = node.service_expires_at ? node.service_expires_at > Date.now() && node.service_expires_at <= Date.now() + 31 * 24 * 60 * 60 * 1000 : false
  return {
    node,
    monthlyCost,
    annualCost,
    remainingValue,
    renewalCost,
    nextMonthCost: expiresInNextMonth ? renewalCost : 0,
    expiresInNextMonth,
    remainingDays: node.service_expires_at ? Math.max(0, node.remaining_days) : Number.POSITIVE_INFINITY
  }
}

function monthlyNodeCost(node: NodeRecord) {
  const amount = node.price_amount || 0
  switch ((node.billing_cycle || 'monthly').toLowerCase()) {
    case 'daily':
      return amount * 30
    case 'yearly':
      return amount / 12
    case 'one_time':
      return oneTimeAnnualCost(node) / 12
    default:
      return amount
  }
}

function annualNodeCost(node: NodeRecord) {
  const amount = node.price_amount || 0
  switch ((node.billing_cycle || 'monthly').toLowerCase()) {
    case 'daily':
      return amount * 365
    case 'yearly':
      return amount
    case 'one_time':
      return oneTimeAnnualCost(node)
    default:
      return amount * 12
  }
}

function oneTimeAnnualCost(node: NodeRecord) {
  const amount = node.price_amount || 0
  if (!node.service_started_at || !node.service_expires_at || node.service_expires_at <= node.service_started_at) {
    return amount
  }
  const days = Math.max(1, (node.service_expires_at - node.service_started_at) / (24 * 60 * 60 * 1000))
  return amount * 365 / days
}

function formatExchangeRateSummary() {
  const base = assetBaseCurrency.value
  const parts = ['USD', 'HKD', 'EUR']
    .filter((currency) => currency !== base)
    .map((currency) => `${currency} ${formatRate(exchangeRates.value[currency] ?? fallbackExchangeRates(base)[currency] ?? 0)}`)
  return parts.join(' · ')
}

function formatRate(value: number) {
  if (!value) return '-'
  return value >= 100 ? value.toFixed(0) : value.toFixed(2)
}

function formatRemainingDays(node: NodeRecord) {
  if (!node.service_expires_at) return '未设置'
  if (node.remaining_days <= 0) return '已到期'
  return `${node.remaining_days} 天`
}

function formatTrafficPlan(node: NodeRecord) {
  const limit = node.traffic_limit_bytes > 0 ? formatBytes(node.traffic_limit_bytes) : '不限'
  return `${limit} · ${resetCycleLabel(node.traffic_reset_cycle)}`
}

function normalizeTrafficBillingDirection(value?: string): TrafficBillingDirection {
  return value === 'outbound' ? 'outbound' : 'bidirectional'
}

function trafficBillingDirectionLabel(value?: string) {
  return normalizeTrafficBillingDirection(value) === 'outbound' ? '单向出站' : '双向'
}

function formatRemainingTraffic(node: NodeRecord) {
  if (!node.traffic_limit_bytes) return '不限'
  return formatBytes(node.traffic_remaining_bytes)
}

function formatTrafficUsageSummary() {
  if (!totalTrafficLimitBytes.value) {
    return `${formatBytes(totalTrafficUsedBytes.value)} / 不限`
  }
  return `${formatBytes(totalTrafficUsedBytes.value)} / ${formatBytes(totalTrafficLimitBytes.value)}`
}

function trafficUsagePercent(node: NodeRecord) {
  if (!node.traffic_limit_bytes) return 0
  return Math.min(100, Math.round((node.traffic_used_bytes / node.traffic_limit_bytes) * 100))
}

function formatCPUCores(metric?: NodeMetric | null) {
  if (!metric?.cpu_cores) return 'N/A'
  return `${metric.cpu_cores} Core`
}

function formatMetricText(value?: string | null) {
  const text = value?.trim()
  if (!text || text === 'unknown') return 'N/A'
  return text
}

function formatSwap(metric?: NodeMetric | null) {
  if (!metric?.swap_total) return '未启用'
  return `${formatBytes(metric.swap_used)} / ${formatBytes(metric.swap_total)} (${formatPercent(metric.swap_used_percent)})`
}

function formatOSName(metric?: NodeMetric | null) {
  return formatOSDisplay(metric).label
}

function osIconSVG(key: string) {
  const icons: Record<string, string> = {
    debian: '<svg viewBox="0 0 32 32" aria-hidden="true"><path fill="#a80030" d="M17.4 2.8c6.2.7 10.7 5 10.1 10.2-.7 6.7-8.8 9.6-14.2 6.6 6.9 1.2 11.4-2.1 12-6.7.5-4-2.7-7.3-7.8-8-5.7-.7-10.6 2.6-11.2 7-.4 3.4 1.8 6 5.2 7.1-4.9-.6-8.1-3.7-7.6-7.8.7-5.5 6.4-9.3 13.5-8.4Z"/><path fill="#d70a53" d="M15.1 8.1c3.4.1 5.9 1.9 5.7 4.2-.3 3-4.3 4.3-7 2.9 3 .2 5.1-.9 5.3-2.8.1-1.4-1.5-2.5-3.8-2.6-2.9-.1-5.2 1.4-5.3 3.3-.1 1 .4 1.8 1.3 2.4-2.2-.5-3.5-1.8-3.4-3.5.2-2.4 3.4-4 7.2-3.9Z"/></svg>',
    ubuntu: '<svg viewBox="0 0 32 32" aria-hidden="true"><circle cx="16" cy="16" r="12" fill="#e95420"/><circle cx="16" cy="16" r="5" fill="#fff"/><circle cx="16" cy="6.4" r="2.5" fill="#fff"/><circle cx="7.7" cy="20.8" r="2.5" fill="#fff"/><circle cx="24.3" cy="20.8" r="2.5" fill="#fff"/><path fill="#e95420" d="M14.4 9.1h3.2v7.6h-3.2zM10.2 19.4l1.6-2.8 6.6 3.8-1.6 2.8zM21.8 19.4l-1.6-2.8-6.6 3.8 1.6 2.8z"/></svg>',
    centos: '<svg viewBox="0 0 32 32" aria-hidden="true"><path fill="#9ccd2a" d="M5 5h9v9H5z"/><path fill="#932279" d="M18 5h9v9h-9z"/><path fill="#efa724" d="M5 18h9v9H5z"/><path fill="#262577" d="M18 18h9v9h-9z"/><path fill="#fff" d="M9 9h14v2H9zm0 12h14v2H9zm0-12h2v14H9zm12 0h2v14h-2z" opacity=".95"/></svg>',
    rocky: '<svg viewBox="0 0 32 32" aria-hidden="true"><circle cx="16" cy="16" r="13" fill="#10b981"/><path fill="#064e3b" d="M5.2 20.8 14 9.5l3.3 4.5 2.1-2.7 7.4 9.5c-2.1 4.7-7.5 7.4-12.8 6.2-3.9-.9-6.9-3.2-8.8-6.2Z"/><path fill="#fff" d="m9.3 20.2 4.7-6.1 3.3 4.5 2-2.6 3.4 4.2z" opacity=".92"/></svg>',
    alma: '<svg viewBox="0 0 32 32" aria-hidden="true"><circle cx="16" cy="16" r="13" fill="#0f172a"/><circle cx="12" cy="9" r="4.2" fill="#22c55e"/><circle cx="22" cy="13" r="4.2" fill="#38bdf8"/><circle cx="17.5" cy="23" r="4.2" fill="#f97316"/><circle cx="9.5" cy="20" r="3.4" fill="#ef4444"/><circle cx="16" cy="16" r="3.2" fill="#fff" opacity=".92"/></svg>',
    fedora: '<svg viewBox="0 0 32 32" aria-hidden="true"><rect width="26" height="26" x="3" y="3" rx="13" fill="#294172"/><path fill="#fff" d="M20.4 9.5c-2.5 0-4 1.4-5.2 3.8l-.7 1.4h-2.1c-2.6 0-4.5 1.9-4.5 4.2 0 2.2 1.7 3.8 3.9 3.8 2.5 0 4-1.4 5.2-3.8l.7-1.4h2.1c2.6 0 4.5-1.9 4.5-4.2 0-2.2-1.7-3.8-3.9-3.8Zm-8.5 11c-1 0-1.7-.7-1.7-1.6 0-.9.8-1.8 2.1-1.8h1.1l-.3.6c-.4.9-.8 1.6-1.2 2.1Zm7.9-5.3h-1.1l.3-.6c.4-.9.8-1.6 1.2-2.1 1 .1 1.7.7 1.7 1.6 0 .9-.8 1.8-2.1 1.8Z"/></svg>',
    arch: '<svg viewBox="0 0 32 32" aria-hidden="true"><path fill="#1793d1" d="M16 3 5 28c4-2.4 7.1-3.4 10.8-3.4 3.6 0 7.1 1 11.2 3.4L16 3Z"/><path fill="#fff" d="M16 8.6 12.5 17c2.2-.8 4.7-.8 7 0L16 8.6Z" opacity=".9"/></svg>',
    alpine: '<svg viewBox="0 0 32 32" aria-hidden="true"><rect width="26" height="26" x="3" y="3" rx="6" fill="#0d597f"/><path fill="#fff" d="m5.8 23.5 6.7-10.2 3.3 4.5 4.7-7.2 5.7 12.9h-4.7l-1.8-4-2.5 4H14l-2.1-3-2 3H5.8Z"/><path fill="#7dd3fc" d="m12.6 13.3 1.8 5.4 1.4-.9-3.2-4.5Z"/></svg>',
    windows: '<svg viewBox="0 0 32 32" aria-hidden="true"><path fill="#00a4ef" d="m4 6 10.5-1.5v10.7H4z"/><path fill="#7fba00" d="M16.1 4.3 28 2.6v12.6H16.1z"/><path fill="#f25022" d="M4 16.8h10.5v10.7L4 26z"/><path fill="#ffb900" d="M16.1 16.8H28v12.6l-11.9-1.7z"/></svg>',
    unknown: '<svg viewBox="0 0 32 32" aria-hidden="true"><rect width="24" height="24" x="4" y="4" rx="8" fill="#334155"/><path fill="#fff" d="M10 11.2c1.2-1.7 3-2.7 5.5-2.7 3.4 0 5.7 1.9 5.7 4.7 0 2-1.1 3.2-3.1 4.2-1.3.7-1.7 1.1-1.7 2.4h-3c0-2 .7-3.1 2.5-4.1 1.5-.8 2-1.4 2-2.5 0-1.2-.9-2-2.5-2-1.5 0-2.6.7-3.4 1.9L10 11.2Zm3.2 11.5h3.5v3.3h-3.5z"/></svg>'
  }
  return icons[key] ?? icons.unknown
}

function osIconURL(key: string) {
  const icons: Record<string, string> = {
    debian: '/os-icons/debian.svg',
    ubuntu: '/os-icons/ubuntu.svg',
    centos: '/os-icons/centos.svg',
    rocky: '/os-icons/rocky.svg',
    alma: '/os-icons/alma.svg',
    fedora: '/os-icons/fedora.svg',
    arch: '/os-icons/arch.svg',
    alpine: '/os-icons/alpine.svg',
    windows: '/os-icons/windows.svg',
    unknown: '/os-icons/unknown.svg'
  }
  return icons[key] ?? icons.unknown
}

function formatOSDisplay(metric?: NodeMetric | null) {
  const raw = metric?.os_name?.trim()
  if (!raw) return { key: 'unknown', icon: osIconURL('unknown'), label: 'N/A' }
  const normalized = raw.replace(/^"|"$/g, '')
  const version = normalized.match(/\b\d+(?:\.\d+)?/)?.[0] ?? ''
  const lower = normalized.toLowerCase()
  if (lower.includes('debian')) return { key: 'debian', icon: osIconURL('debian'), label: `Debian${version ? ` ${version}` : ''}` }
  if (lower.includes('ubuntu')) return { key: 'ubuntu', icon: osIconURL('ubuntu'), label: `Ubuntu${version ? ` ${version}` : ''}` }
  if (lower.includes('centos')) return { key: 'centos', icon: osIconURL('centos'), label: `CentOS${version ? ` ${version}` : ''}` }
  if (lower.includes('rocky')) return { key: 'rocky', icon: osIconURL('rocky'), label: `Rocky${version ? ` ${version}` : ''}` }
  if (lower.includes('alma')) return { key: 'alma', icon: osIconURL('alma'), label: `Alma${version ? ` ${version}` : ''}` }
  if (lower.includes('fedora')) return { key: 'fedora', icon: osIconURL('fedora'), label: `Fedora${version ? ` ${version}` : ''}` }
  if (lower.includes('arch')) return { key: 'arch', icon: osIconURL('arch'), label: `Arch${version ? ` ${version}` : ''}` }
  if (lower.includes('alpine')) return { key: 'alpine', icon: osIconURL('alpine'), label: `Alpine${version ? ` ${version}` : ''}` }
  if (lower.includes('windows')) return { key: 'windows', icon: osIconURL('windows'), label: `Windows${version ? ` ${version}` : ''}` }
  return { key: 'unknown', icon: osIconURL('unknown'), label: normalized.length > 18 ? `${normalized.slice(0, 18)}…` : normalized }
}

function formatNodeUptime(node: NodeRecord) {
  if (!isNodeOnline(node)) return '离线'
  return node.latest_metric?.uptime_seconds ? formatDuration(node.latest_metric.uptime_seconds) : formatAgo(node.last_seen_at)
}

function trendKindLabel(kind: TrendKind) {
  const labels: Record<TrendKind, string> = {
    cpu: 'CPU',
    memory: '内存',
    disk: '硬盘',
    network: '网络',
    traffic: '流量'
  }
  return labels[kind]
}

function showTrend(node: NodeRecord, kind: TrendKind, event: MouseEvent) {
  cancelHideTrend()
  trendLoading.value = isNodeOnline(node) || !trendMetricsCache.value[node.node_id]
  activeTrend.value = {
    kind,
    node,
    x: event.clientX,
    y: event.clientY
  }

  void ensureTrendMetrics(node, true).then(renderTrendChart)
  renderTrendChart()
}

function moveTrend(event: MouseEvent) {
  if (!activeTrend.value) return
  activeTrend.value = {
    ...activeTrend.value,
    x: event.clientX,
    y: event.clientY
  }
}

function scheduleHideTrend() {
  cancelHideTrend()
  trendHideTimer = window.setTimeout(() => {
    hideTrend()
  }, 120)
}

function cancelHideTrend() {
  if (!trendHideTimer) return
  window.clearTimeout(trendHideTimer)
  trendHideTimer = undefined
}

function hideTrend() {
  activeTrend.value = null
  trendChart?.dispose()
  trendChart = null
}

function renderTrendChart() {
  void nextTick(() => {
    if (!trendChartEl.value || !activeTrend.value) return

    trendChart ??= echarts.init(trendChartEl.value)
    trendChart.setOption(trendOption(activeTrend.value.kind, activeTrendMetrics.value), true)
  })
}

function trendOption(kind: TrendKind, source: NodeMetric[]): EChartsOption {
  const labels = source.map((item) => formatTime(item.ts))
  const trafficTxDelta = trafficDeltaMBSeries(source, 'net_tx_bytes_total')
  const trafficRxDelta = trafficDeltaMBSeries(source, 'net_rx_bytes_total')
  const trendMap: Record<TrendKind, { unit: string; series: Array<{ name: string; data: number[]; color: string }> }> = {
    cpu: {
      unit: '%',
      series: [{ name: 'CPU', data: source.map((item) => round(item.cpu_usage)), color: '#0ea5e9' }]
    },
    memory: {
      unit: '%',
      series: [{ name: '内存', data: source.map((item) => round(item.mem_used_percent)), color: '#22c55e' }]
    },
    disk: {
      unit: '%',
      series: [{ name: '硬盘', data: source.map((item) => round(item.disk_used_percent)), color: '#f59e0b' }]
    },
    network: {
      unit: 'Mbps',
      series: [
        { name: '上行', data: source.map((item) => toMbps(item.net_tx_bps)), color: '#0f766e' },
        { name: '下行', data: source.map((item) => toMbps(item.net_rx_bps)), color: '#d97706' }
      ]
    },
    traffic: {
      unit: 'MB',
      series: [
        { name: '上行新增', data: trafficTxDelta, color: '#0f766e' },
        { name: '下行新增', data: trafficRxDelta, color: '#d97706' }
      ]
    }
  }
  const { unit, series } = trendMap[kind]

  return {
    color: series.map((item) => item.color),
    backgroundColor: 'transparent',
    animationDuration: 360,
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value) => `${value} ${unit}`,
      ...chartTooltipStyle
    },
    legend: {
      top: 0,
      right: 0,
      itemWidth: 9,
      itemHeight: 9,
      textStyle: { color: chartLegendTextColor }
    },
    grid: { top: 38, left: 12, right: 16, bottom: 10, containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: labels,
      axisTick: { show: false },
      axisLine: { lineStyle: { color: chartAxisLineColor } },
      axisLabel: { color: chartAxisLabelColor, hideOverlap: true }
    },
    yAxis: {
      type: 'value',
      name: unit,
      nameGap: 10,
      nameTextStyle: {
        color: chartNameColor,
        fontSize: 11,
        fontWeight: 700
      },
      axisLabel: {
        color: chartAxisLabelColor,
        formatter: (value: number | string) => {
          const label = typeof value === 'number' ? String(value) : value
          return unit === '%' ? `${label}%` : `${label} ${unit}`
        }
      },
      splitLine: { lineStyle: { color: chartSplitLineColor, type: 'dashed' } }
    },
    series: series.map((item) => ({
      name: item.name,
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: { width: 2.6 },
      areaStyle: { opacity: 0.12 },
      data: item.data
    }))
  }
}

function renderProbeChart() {
  void nextTick(() => {
    if (!probeChartEl.value || !probePanelOpen.value) return

    probeChart ??= echarts.init(probeChartEl.value)
    probeChart.setOption(probeOption(visibleProbeResults.value), true)
  })
}

function probeOption(source: ProbeResult[]): EChartsOption {
  const taskMeta = new Map(probePanelTasks.value.map((task) => [task.id, {
    intervalSeconds: task.interval_seconds,
    inactive: !task.enabled
  }]))
  const groups = new Map<string, { name: string; data: Array<[number, number | null]>; intervalSeconds: number; inactive: boolean }>()
  const anchorTimestamp = probeRangeAnchor.value
  for (const item of source) {
    const timestamp = Date.parse(item.created_at)
    if (!Number.isFinite(timestamp)) continue
    const key = probeGroupKey(item)
    const meta = taskMeta.get(item.task_id)
    const inactive = meta ? meta.inactive : showInactiveProbeHistory.value
    const name = `${probeDisplayName(item)} · ${probeModeLabel(item.type, item.ip_version)}${inactive ? ' · 已停用' : ''}`
    const intervalSeconds = meta?.intervalSeconds ?? probeRefreshSeconds()
    const group = groups.get(key) ?? { name, data: [], intervalSeconds, inactive }
    group.intervalSeconds = Math.min(group.intervalSeconds, intervalSeconds)
    group.inactive = group.inactive || inactive
    group.data.push([timestamp, item.status === 'success' && item.latency_ms !== null ? item.latency_ms : null])
    groups.set(key, group)
  }

  const colors = ['#0ea5e9', '#22c55e', '#f97316', '#8b5cf6', '#ef4444', '#14b8a6', '#f59e0b']
  const series = Array.from(groups.values()).map((group, index) => {
    const color = group.inactive ? '#94a3b8' : colors[index % colors.length]
    const aggregateRange = isAggregatedProbeRange(probeRange.value)
    const breakSegments = gapSegmentsFromTimestamps(
      group.data.map(([timestamp]) => timestamp),
      probeRange.value,
      aggregateRange ? Math.floor(rangeBucketMs(probeRange.value) / 1000) : group.intervalSeconds,
      anchorTimestamp,
      aggregateRange ? 1.5 : 3
    )
    const tooltipData = probeSeriesData(group.data, probeRange.value, anchorTimestamp)
    const chartData = probeSeriesData(group.data, probeRange.value, anchorTimestamp, breakSegments)
    const pointCount = chartData.filter(([, value]) => value !== null).length

    return {
      name: group.name,
      color,
      intervalSeconds: group.intervalSeconds,
      inactive: group.inactive,
      pointCount,
      tooltipData,
      chartData
    }
  })
  const successPointCount = series.reduce((total, item) => total + item.chartData.filter(([, value]) => value !== null).length, 0)
  const tooltipSeries = series.map((item): ProbeChartTooltipSeries => ({
    name: item.name,
    color: item.color,
    intervalSeconds: item.intervalSeconds,
    data: item.tooltipData
  }))

  return {
    color: colors,
    backgroundColor: 'transparent',
    animationDuration: 360,
    tooltip: {
      trigger: 'axis',
      formatter: probeChartTooltipFormatter(tooltipSeries, probeRange.value),
      valueFormatter: (value) => (value === null || value === undefined ? '失败' : `${value} ms`),
      ...chartTooltipStyle
    },
    legend: {
      top: 0,
      right: 0,
      type: 'scroll',
      itemWidth: 9,
      itemHeight: 9,
      textStyle: { color: chartLegendTextColor }
    },
    grid: chartGridForRange(probeRange.value),
    xAxis: timeXAxisForRange(probeRange.value, anchorTimestamp, probeChartEl.value?.clientWidth),
    yAxis: {
      type: 'value',
      name: 'ms',
      nameGap: 10,
      nameTextStyle: {
        color: chartNameColor,
        fontSize: 11,
        fontWeight: 700
      },
      axisLabel: {
        color: chartAxisLabelColor,
        formatter: '{value} ms'
      },
      splitLine: { lineStyle: { color: chartSplitLineColor, type: 'dashed' } }
    },
    graphic: successPointCount === 0 ? {
      type: 'text',
      left: 'center',
      top: 'middle',
      style: {
        text: source.length === 0 ? (showInactiveProbeHistory.value ? '暂无 Ping 历史数据' : '暂无启用的 Ping 节点') : '暂无成功延迟数据',
        fill: chartEmptyTextColor,
        fontSize: 13,
        fontWeight: 700
      }
    } : undefined,
    series: series.map((item) => ({
      name: item.name,
      type: 'line' as const,
      smooth: true,
      showSymbol: item.pointCount <= 1,
      symbol: item.pointCount <= 1 ? 'circle' : 'none',
      symbolSize: 7,
      connectNulls: false,
      lineStyle: { width: item.inactive ? 1.8 : 2.2, type: item.inactive ? 'dashed' as const : 'solid' as const },
      areaStyle: { opacity: item.inactive ? 0.03 : 0.08 },
      itemStyle: { color: item.color },
      data: item.chartData
    }))
  }
}

function probeSeriesData(
  source: Array<[number, number | null]>,
  range: ProbeRange,
  anchorTimestamp?: number | null,
  breakSegments: GapSegment[] = []
): Array<[number, number | null]> {
  const buckets = new Map<number, number[]>()
  const bucketSeen = new Set<number>()
  for (const [timestamp, latency] of source) {
    const axisValue = seriesAxisValue(timestamp, range, anchorTimestamp)
    if (axisValue === null || typeof axisValue !== 'number') continue
    const values = buckets.get(axisValue) ?? []
    if (latency !== null) {
      values.push(latency)
    }
    bucketSeen.add(axisValue)
    buckets.set(axisValue, values)
  }

  const data = Array.from(bucketSeen)
    .sort((left, right) => left - right)
    .map((ts): [number, number | null] => {
      const values = buckets.get(ts) ?? []
      return [ts, values.length ? round(average(values) ?? 0) : null]
    })

  return applyGapBreaks(data, breakSegments)
}

function aggregateMetrics(source: NodeMetric[], range: ProbeRange, anchorTimestamp?: number | null): MetricBucket[] {
  const buckets = new Map<number, NodeMetric[]>()
  for (const item of source) {
    const ts = metricBucketTimestamp(item.ts, range, anchorTimestamp)
    if (ts === null) continue
    const values = buckets.get(ts) ?? []
    values.push(item)
    buckets.set(ts, values)
  }
  return Array.from(buckets.entries())
    .sort(([left], [right]) => left - right)
    .map(([ts, values]) => {
      const last = values[values.length - 1]
      return {
        ts,
        cpu_usage: average(values.map((item) => item.cpu_usage)) ?? 0,
        mem_total: last.mem_total,
        mem_used: average(values.map((item) => item.mem_used)) ?? 0,
        mem_used_percent: average(values.map((item) => item.mem_used_percent)) ?? 0,
        disk_total: last.disk_total,
        disk_used: average(values.map((item) => item.disk_used)) ?? 0,
        disk_used_percent: average(values.map((item) => item.disk_used_percent)) ?? 0,
        net_rx_bps: average(values.map((item) => item.net_rx_bps)) ?? 0,
        net_tx_bps: average(values.map((item) => item.net_tx_bps)) ?? 0,
        net_rx_bytes_total: last.net_rx_bytes_total,
        net_tx_bytes_total: last.net_tx_bytes_total
      }
    })
}

function renderMetricsPanelCharts() {
  void nextTick(() => {
    if (!metricsPanelOpen.value) return
    const source = metricsBuckets.value
    if (metricsCPUChartEl.value) {
      metricsCPUChart ??= echarts.init(metricsCPUChartEl.value)
      metricsCPUChart.setOption(metricLineOption(source, 'CPU 使用率', '%', '#2563eb', (item) => round(item.cpu_usage)), true)
    }
    if (metricsMemoryChartEl.value) {
      metricsMemoryChart ??= echarts.init(metricsMemoryChartEl.value)
      metricsMemoryChart.setOption(metricLineOption(source, '内存使用率', '%', '#16a34a', (item) => round(item.mem_used_percent), (item) => formatUsageBytes(item.mem_used, item.mem_total)), true)
    }
    if (metricsDiskChartEl.value) {
      metricsDiskChart ??= echarts.init(metricsDiskChartEl.value)
      metricsDiskChart.setOption(metricLineOption(source, '硬盘使用率', '%', '#f97316', (item) => round(item.disk_used_percent), (item) => formatUsageBytes(item.disk_used, item.disk_total)), true)
    }
    if (metricsNetworkChartEl.value) {
      metricsNetworkChart ??= echarts.init(metricsNetworkChartEl.value)
      metricsNetworkChart.setOption(metricMultiLineOption(source, 'Mbps', [
        { name: '上行', color: '#0f766e', valueOf: (item) => toMbps(item.net_tx_bps) },
        { name: '下行', color: '#d97706', valueOf: (item) => toMbps(item.net_rx_bps) }
      ]), true)
    }
    if (metricsTrafficChartEl.value) {
      metricsTrafficChart ??= echarts.init(metricsTrafficChartEl.value)
      const trafficTxDelta = trafficDeltaMBSeries(source, 'net_tx_bytes_total')
      const trafficRxDelta = trafficDeltaMBSeries(source, 'net_rx_bytes_total')
      metricsTrafficChart.setOption(metricMultiLineOption(source, 'MB', [
        { name: '上行新增', color: '#0284c7', valueOf: (_item, index) => trafficTxDelta[index] ?? 0 },
        { name: '下行新增', color: '#dc2626', valueOf: (_item, index) => trafficRxDelta[index] ?? 0 }
      ]), true)
    }
  })
}

function metricLineOption(
  source: MetricBucket[],
  name: string,
  unit: string,
  color: string,
  valueOf: (item: MetricBucket) => number,
  detailOf?: (item: MetricBucket) => string
): EChartsOption {
  return metricMultiLineOption(source, unit, [{ name, color, valueOf, detailOf }])
}

function metricsChartWidth() {
  const widths = [
    metricsCPUChartEl.value?.clientWidth,
    metricsMemoryChartEl.value?.clientWidth,
    metricsDiskChartEl.value?.clientWidth,
    metricsNetworkChartEl.value?.clientWidth,
    metricsTrafficChartEl.value?.clientWidth
  ]
  return widths.find((width) => Number(width) > 0) ?? null
}

type MetricSeriesItem = {
  name: string
  color: string
  valueOf: (item: MetricBucket, index: number) => number
  detailOf?: (item: MetricBucket) => string
}

function metricMultiLineOption(
  source: MetricBucket[],
  unit: string,
  seriesItems: MetricSeriesItem[]
): EChartsOption {
  const anchorTimestamp = metricsRangeAnchor.value
  const gapSegments = gapSegmentsFromTimestamps(
    visibleMetricsPanelMetrics.value.map((item) => item.ts),
    metricsRange.value,
    activeMetricsNode.value?.metrics_interval,
    anchorTimestamp
  )
  return {
    color: seriesItems.map((item) => item.color),
    backgroundColor: 'transparent',
    animationDuration: 360,
    tooltip: {
      trigger: 'axis',
      formatter: metricChartTooltipFormatter(source, unit, '-', seriesItems),
      valueFormatter: (value) => (value === null || value === undefined ? '-' : `${value} ${unit}`),
      ...chartTooltipStyle
    },
    legend: seriesItems.length > 1 ? {
      top: 0,
      right: 0,
      itemWidth: 9,
      itemHeight: 9,
      textStyle: { color: chartLegendTextColor }
    } : undefined,
    grid: chartGridForRange(metricsRange.value),
    xAxis: timeXAxisForRange(metricsRange.value, anchorTimestamp, metricsChartWidth()),
    yAxis: {
      type: 'value',
      name: unit,
      nameTextStyle: { color: chartNameColor, fontSize: 11, fontWeight: 700 },
      axisLabel: {
        color: chartAxisLabelColor,
        formatter: (value: number | string) => {
          const label = typeof value === 'number' ? String(value) : value
          return unit === '%' ? `${label}%` : `${label} ${unit}`
        }
      },
      splitLine: { lineStyle: { color: chartSplitLineColor, type: 'dashed' } }
    },
    graphic: source.length === 0 ? {
      type: 'text',
      left: 'center',
      top: 'middle',
      style: {
        text: '暂无指标数据',
        fill: chartEmptyTextColor,
        fontSize: 13,
        fontWeight: 700
      }
    } : undefined,
    series: seriesItems.map((item) => {
      const data = metricSeriesData(source, metricsRange.value, item.valueOf, anchorTimestamp, gapSegments)
      const pointCount = data.filter(([, value]) => value !== null).length
      return {
        name: item.name,
        type: 'line',
        smooth: true,
        connectNulls: false,
        showSymbol: pointCount <= 1,
        symbol: pointCount <= 1 ? 'circle' : 'none',
        symbolSize: 7,
        lineStyle: { width: 2.4 },
        areaStyle: { opacity: 0.1 },
        data
      }
    })
  }
}

function metricSeriesData(
  source: MetricBucket[],
  range: ProbeRange,
  valueOf: (item: MetricBucket, index: number) => number,
  anchorTimestamp?: number | null,
  breakSegments: GapSegment[] = []
): Array<[number, number | null]> {
  const values = new Map(source.map((bucket, index) => [bucket.ts, valueOf(bucket, index)]))
  const data = source.map((bucket): [number, number | null] => [
    bucket.ts,
    values.get(bucket.ts) ?? null
  ]).sort(([left], [right]) => left - right)

  return applyGapBreaks(data, breakSegments)
}

function metricsLatestValue(kind: 'cpu' | 'memory' | 'disk' | 'network' | 'traffic') {
  const latest = metricsBuckets.value[metricsBuckets.value.length - 1]
  if (!latest) return '当前 -'
  switch (kind) {
    case 'cpu':
      return `当前 ${formatPercent(latest.cpu_usage)}`
    case 'memory':
      return `当前 ${formatPercent(latest.mem_used_percent)} · ${formatUsageBytes(latest.mem_used, latest.mem_total)}`
    case 'disk':
      return `当前 ${formatPercent(latest.disk_used_percent)} · ${formatUsageBytes(latest.disk_used, latest.disk_total)}`
    case 'network':
      return `↑ ${toMbps(latest.net_tx_bps)} Mbps / ↓ ${toMbps(latest.net_rx_bps)} Mbps`
    case 'traffic': {
      const source = metricsBuckets.value
      const trafficTxDelta = trafficDeltaMBSeries(source, 'net_tx_bytes_total')
      const trafficRxDelta = trafficDeltaMBSeries(source, 'net_rx_bytes_total')
      return `区间 ↑ ${trafficTxDelta[trafficTxDelta.length - 1] ?? 0} MB / ↓ ${trafficRxDelta[trafficRxDelta.length - 1] ?? 0} MB`
    }
    default:
      return '当前 -'
  }
}

function metricChartTooltipFormatter(source: MetricBucket[], unit: string, emptyLabel: string, seriesItems: MetricSeriesItem[]) {
  const bucketByTimestamp = new Map(source.map((item) => [item.ts, item]))
  const itemByName = new Map(seriesItems.map((item) => [item.name, item]))
  return (raw: unknown) => {
    const params = (Array.isArray(raw) ? raw : [raw]) as ChartTooltipParam[]
    const timestamp = tooltipTimestamp(params[0])
    const bucket = bucketByTimestamp.get(timestamp)
    const lines = [Number.isFinite(timestamp) ? formatRangeWindowTime(timestamp) : '']
    for (const item of params) {
      const value = tooltipSeriesValue(item)
      const metricItem = item.seriesName ? itemByName.get(item.seriesName) : undefined
      const detail = bucket && metricItem?.detailOf ? ` · ${metricItem.detailOf(bucket)}` : ''
      const label = value === null || value === undefined ? emptyLabel : `${value} ${unit}${detail}`
      lines.push(`${item.marker ?? ''}${item.seriesName ?? ''}<span style="float:right;margin-left:16px;font-weight:700">${label}</span>`)
    }
    return lines.filter(Boolean).join('<br/>')
  }
}

function formatUsageBytes(used?: number | null, total?: number | null) {
  return `${formatBytes(used)} / ${formatBytes(total)}`
}

function round(value: number) {
  return Number(value.toFixed(2))
}

function toMbps(value: number) {
  return Number((value / 1000 / 1000).toFixed(2))
}

function toMB(value: number) {
  return Number((value / 1024 / 1024).toFixed(2))
}

function trafficDeltaBytesSeries<T extends TrafficCounterPoint>(source: T[], key: TrafficCounterKey) {
  if (!source.length) return []

  let baseline = normalizeTrafficCounter(source[0][key])
  let previous = baseline
  let carried = 0

  return source.map((item, index) => {
    const current = normalizeTrafficCounter(item[key])
    if (index === 0) {
      baseline = current
      previous = current
      return 0
    }

    if (current < previous) {
      carried += Math.max(0, previous - baseline)
      baseline = current
    }
    previous = current
    return Math.max(0, carried + current - baseline)
  })
}

function trafficDeltaMBSeries<T extends TrafficCounterPoint>(source: T[], key: TrafficCounterKey) {
  return trafficDeltaBytesSeries(source, key).map(toMB)
}

function normalizeTrafficCounter(value: number) {
  return Number.isFinite(value) && value > 0 ? value : 0
}

function resizeCharts() {
  trendChart?.resize()
  probeChart?.resize()
  resizeMetricsCharts()
  if (probePanelOpen.value) renderProbeChart()
  if (metricsPanelOpen.value) renderMetricsPanelCharts()
}

function resizeMetricsCharts() {
  metricsCPUChart?.resize()
  metricsMemoryChart?.resize()
  metricsDiskChart?.resize()
  metricsNetworkChart?.resize()
  metricsTrafficChart?.resize()
}

function disposeMetricsCharts() {
  metricsCPUChart?.dispose()
  metricsMemoryChart?.dispose()
  metricsDiskChart?.dispose()
  metricsNetworkChart?.dispose()
  metricsTrafficChart?.dispose()
  metricsCPUChart = null
  metricsMemoryChart = null
  metricsDiskChart = null
  metricsNetworkChart = null
  metricsTrafficChart = null
}

watch([activeTrend, activeTrendMetrics], renderTrendChart)
watch([probePanelOpen, visibleProbeResults], renderProbeChart)
watch([metricsPanelOpen, metricsBuckets], renderMetricsPanelCharts)
watch(homeBackgroundImage, (value) => {
  if (value) {
    document.body.style.setProperty('--home-background-image', value)
  } else {
    document.body.style.removeProperty('--home-background-image')
  }
}, { immediate: true })
watch(() => appSettings.value.metrics_retention_months, () => {
  const nextProbeRange = clampRangeToRetention(probeRange.value)
  if (nextProbeRange !== probeRange.value) {
    probeRange.value = nextProbeRange
    if (activeProbeNode.value && probePanelOpen.value) {
      void loadProbeResults(activeProbeNode.value.node_id)
    }
  }

  const nextMetricsRange = clampRangeToRetention(metricsRange.value)
  if (nextMetricsRange !== metricsRange.value) {
    metricsRange.value = nextMetricsRange
    if (activeMetricsNode.value && metricsPanelOpen.value) {
      void loadMetricsPanelData(activeMetricsNode.value.node_id)
    }
  }
})
watch(probePanelOpen, (open) => {
  if (open && activeProbeNode.value) {
    syncProbeAutoRefresh()
    return
  }
  stopProbeAutoRefresh()
  probeChart?.dispose()
  probeChart = null
})
watch(metricsPanelOpen, (open) => {
  if (open) {
    renderMetricsPanelCharts()
    return
  }
  stopMetricsAutoRefresh()
  disposeMetricsCharts()
})

onMounted(() => {
  void loadRegionOptions()
  void refreshAll()
  if (isLoggedIn.value) {
    void loadAdminData()
  } else if (loadedFromAdminPath) {
    loginOpen.value = true
  }
  clockTimer = window.setInterval(() => {
    currentTime.value = Date.now()
  }, 1000)
  refreshTimer = window.setInterval(() => {
    if (viewMode.value === 'home') {
      void refreshAll()
    }
  }, 3000)
  window.addEventListener('resize', resizeCharts)
})

onBeforeUnmount(() => {
  if (clockTimer) {
    window.clearInterval(clockTimer)
  }
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
  cancelHideTrend()
  hideTrend()
  stopProbeAutoRefresh()
  stopMetricsAutoRefresh()
  probeChart?.dispose()
  probeChart = null
  disposeMetricsCharts()
  document.body.style.removeProperty('--home-background-image')
  window.removeEventListener('resize', resizeCharts)
})
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <main class="page-shell">
      <AppTopbar
        :site-name="siteName"
        :site-description="siteDescription"
        :site-avatar="siteAvatar"
        :site-initial="siteInitial"
        :user-avatar="userAvatar"
        :is-logged-in="isLoggedIn"
        :show-account-menu="viewMode === 'admin'"
        :current-user="currentUser"
        :dropdown-options="dropdownOptions"
        @open-assets="openAssetModal"
        @login="loginOpen = true"
        @user-action="handleUserAction"
      />

      <template v-if="viewMode === 'home'">
        <HomeDashboard
          :app-settings="appSettings"
          :summary="summary"
          :nodes="nodes"
          :current-time="currentTime"
          :region-count="regionCount"
          :total-cpu-cores="totalCPUCores"
          :total-memory-bytes="totalMemoryBytes"
          :total-disk-bytes="totalDiskBytes"
          :total-rx-bps="totalRxBps"
          :total-tx-bps="totalTxBps"
          :selected-node-id="selectedNodeID"
          :node-label="nodeLabel"
          :node-hover-tags="nodeHoverTags"
          :node-ip-summary="nodeIPSummary"
          :display-region="displayRegion"
          :region-flag-class="regionFlagClass"
          :region-flag-style="regionFlagStyle"
          :region-flag-label="regionFlagLabel"
          :is-node-online="isNodeOnline"
          :live-metric="liveMetric"
          :format-percent="formatPercent"
          :format-bps="formatBps"
          :format-megabytes="formatMegabytes"
          :format-bytes="formatBytes"
          :format-time="formatTime"
          :format-cpu-cores="formatCPUCores"
          :format-load="formatLoad"
          :format-load-title="formatLoadTitle"
          :format-node-uptime="formatNodeUptime"
          :format-os-display="formatOSDisplay"
          :format-os-name="formatOSName"
          :traffic-usage-percent="trafficUsagePercent"
          :traffic-remaining-line="trafficRemainingLine"
          :package-billing-summary="packageBillingSummary"
          :remaining-package-summary="remainingPackageSummary"
          :remaining-package-title="remainingPackageTitle"
          :format-traffic-usage-summary="formatTrafficUsageSummary"
          @select-node="selectNode"
          @trend="showTrend"
          @move-trend="moveTrend"
          @leave-trend="scheduleHideTrend"
          @open-probe="openProbePanel"
          @open-metrics="openMetricsPanel"
        />

        <aside
          v-if="activeTrend"
          class="trend-float liquid-glass"
          :style="trendPanelStyle"
          @mouseenter="cancelHideTrend"
          @mouseleave="scheduleHideTrend"
        >
          <div class="trend-float-head">
            <div>
              <h3>{{ trendTitle }}</h3>
              <span>{{ activeTrendMetrics.length }} samples · {{ trendKindLabel(activeTrend.kind) }}</span>
              <span v-if="trendLoadSummary" class="trend-load-summary">{{ trendLoadSummary }}</span>
            </div>
            <span class="trend-pill">{{ activeTrend.node.status }}</span>
          </div>
          <div v-if="trendLoading" class="trend-loading">加载趋势数据...</div>
          <div ref="trendChartEl" class="trend-canvas" />
        </aside>
      </template>

      <AdminDashboard
        v-else
        v-model:log-level-filter="logLevelFilter"
        v-model:log-event-filter="logEventFilter"
        v-model:log-node-filter="logNodeFilter"
        :admin-refreshing="adminRefreshing"
        :loading="loading"
        :admin-loading="adminLoading"
        :we-com-test-loading="weComTestLoading"
        :telegram-test-loading="telegramTestLoading"
        :email-test-loading="emailTestLoading"
        :themes-loading="themesLoading"
        :admin-nodes="adminNodes"
        :node-columns="nodeColumns"
        :probe-tasks="probeTasks"
        :themes="themes"
        :filtered-system-logs="filteredSystemLogs"
        :log-columns="logColumns"
        :log-level-options="logLevelOptions"
        :log-event-options="logEventOptions"
        :log-node-options="logNodeOptions"
        :admin-config="adminConfig"
        :settings-editor="settingsEditor"
        :asset-base-currency-options="assetBaseCurrencyOptions"
        :email-security-options="emailSecurityOptions"
        :metrics-retention-options="metricsRetentionOptions"
        :alert-interval-options="alertIntervalOptions"
        :display-target="displayTarget"
        :probe-mode-label="probeModeLabel"
        :system-log-row-key="systemLogRowKey"
        :normalize-image-url="normalizeImageURL"
        :refresh-admin-view="refreshAdminView"
        :open-create-probe-task="openCreateProbeTask"
        :open-edit-probe-task="openEditProbeTask"
        :toggle-probe-task="toggleProbeTask"
        :delete-probe-task="deleteProbeTask"
        :upload-theme="uploadTheme"
        :activate-theme="activateTheme"
        :delete-theme="deleteTheme"
        :save-global-settings="saveGlobalSettings"
        :send-we-com-test-message="sendWeComTestMessage"
        :send-telegram-test-message="sendTelegramTestMessage"
        :send-email-test-message="sendEmailTestMessage"
      />

      <footer class="site-footer">
        © {{ new Date().getFullYear() }} rivo. Built for private infrastructure monitoring.
      </footer>

      <n-modal v-model:show="assetModalOpen" preset="card" class="asset-modal" :closable="false">
        <div class="asset-modal-scroll">
          <div class="asset-modal-head">
            <h3>资产统计</h3>
            <div class="chart-modal-actions">
              <button class="modal-icon-button" type="button" :disabled="assetStatsLoading || exchangeRatesLoading" title="刷新资产统计" @click="refreshAssetStats">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M20 12a8 8 0 1 1-2.34-5.66M20 4v5h-5" />
                </svg>
              </button>
              <button class="modal-icon-button" type="button" title="关闭" @click="assetModalOpen = false">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M6 6l12 12M18 6 6 18" />
                </svg>
              </button>
            </div>
          </div>

          <div class="asset-section-title">总览</div>

          <div class="asset-overview-grid">
            <div>
              <span>服务器数量</span>
              <strong>{{ assetSummary.count }}</strong>
            </div>
            <div>
              <span>年化总支出</span>
              <strong>{{ formatAssetMoney(assetSummary.annualTotal) }}</strong>
            </div>
            <div>
              <span>月均支出</span>
              <strong>{{ formatAssetMoney(assetSummary.monthlyTotal) }}</strong>
            </div>
            <div>
              <span>剩余价值</span>
              <strong class="asset-positive">{{ formatAssetMoney(assetSummary.remainingTotal) }}</strong>
            </div>
            <div>
              <span>下月预估费用</span>
              <strong>{{ formatAssetMoney(assetSummary.nextMonthTotal) }}</strong>
              <small>{{ assetSummary.nextMonthCount }} 个资产将在 31 天内到期</small>
            </div>
            <div>
              <span>统计货币</span>
              <strong>{{ assetBaseCurrency }}</strong>
              <small>{{ appSettings.exchange_rate_auto_update ? '实时汇率优先' : '使用兜底汇率' }}</small>
            </div>
          </div>

          <div class="asset-list-head">
            <strong>明细</strong>
            <div class="asset-list-tabs">
              <button type="button" :class="{ active: assetListMode === 'all' }" @click="assetListMode = 'all'">
                全部资产
              </button>
              <button type="button" :class="{ active: assetListMode === 'renewal' }" @click="assetListMode = 'renewal'">
                下月续费
                <b>{{ assetSummary.nextMonthCount }}</b>
              </button>
            </div>
          </div>

          <div class="asset-table-card" :class="{ scrollable: visibleAssetRows.length > 4 }">
            <div class="asset-list-columns">
              <span>名称</span>
              <span>费用</span>
              <span>付费周期</span>
              <span>服务商</span>
              <span>截止日期</span>
            </div>
            <div v-for="item in visibleAssetRows" :key="item.node.node_id" class="asset-row">
              <div class="asset-row-main">
                <div class="asset-row-name">
                  <span
                    class="node-flag"
                    :class="regionFlagClass(item.node.region)"
                    :style="regionFlagStyle(item.node.region)"
                    role="img"
                    :aria-label="regionFlagLabel(item.node.region)"
                  />
                  <strong class="node-name-with-id asset-node-name">
                    <span class="node-name-trigger">{{ nodeLabel(item.node) }}</span>
                    <small class="node-id-popover">
                      <span
                        v-for="line in nodeHoverMeta(item.node)"
                        :key="`${item.node.node_id}-asset-${line.type}-${line.text}`"
                      >
                        {{ line.text }}
                      </span>
                      <span v-if="nodeHoverTags(item.node).length" class="node-popover-tags">
                        <span
                          v-for="tag in nodeHoverTags(item.node)"
                          :key="`${item.node.node_id}-asset-${tag}`"
                        >
                          {{ tag }}
                        </span>
                      </span>
                    </small>
                  </strong>
                </div>
                <div class="asset-row-price">
                  <span>{{ formatAssetMoney(item.monthlyCost) }}/月</span>
                </div>
                <div class="asset-row-cycle">{{ cycleLabel(item.node.billing_cycle) }}</div>
                <div class="asset-row-provider">{{ item.node.provider || '未设置' }}</div>
                <div class="asset-row-date">{{ formatDate(item.node.service_expires_at) }}</div>
              </div>
            </div>
            <div v-if="visibleAssetRows.length === 0" class="probe-empty">
              {{ assetListMode === 'renewal' ? '下月暂无需要续费的资产' : '暂无资产数据' }}
            </div>
          </div>

          <div class="asset-rate-footer">
            <span>{{ exchangeRatesSource === 'frankfurter' ? '实时汇率' : '兜底汇率' }}</span>
            <div class="asset-rate-content">
              <strong>{{ formatExchangeRateSummary() }}</strong>
              <small v-if="exchangeRatesError">{{ exchangeRatesError }}</small>
            </div>
          </div>
        </div>
        <div v-if="assetModalOpen && (assetStatsLoading || exchangeRatesLoading)" class="content-loading-overlay modal-loading-overlay" role="status" aria-live="polite">
          <span class="loading-orb"></span>
          <strong>Loading...</strong>
        </div>
      </n-modal>

	      <n-modal v-model:show="loginOpen" preset="card" class="login-modal" title="Master 登录">
	        <n-form :model="loginForm" @submit.prevent="submitLogin">
          <n-form-item label="账号">
            <n-input v-model:value="loginForm.username" placeholder="admin" />
          </n-form-item>
          <n-form-item label="密码">
            <n-input v-model:value="loginForm.password" type="password" show-password-on="click" />
          </n-form-item>
          <p v-if="loginError" class="login-error">{{ loginError }}</p>
          <n-button block type="primary" :loading="loginLoading" @click="submitLogin">登录</n-button>
        </n-form>
      </n-modal>

      <n-modal v-model:show="probePanelOpen" preset="card" class="probe-modal" :closable="false">
        <div class="chart-modal-scroll">
          <div class="chart-modal-head">
            <div>
              <h3>{{ probeModalTitle }} <small v-if="probeAutoRefreshEnabled">{{ probeRefreshCountdown }}s 后刷新</small></h3>
            </div>
            <div class="chart-modal-actions">
              <button class="range-help modal-help" type="button" aria-label="查看时间维度说明">
                ?
                <b>
                  <span v-for="item in availableProbeRangeOptions" :key="item.value">{{ item.help }}</span>
                  <span>短周期（1H/4H/12H/1D/7D）使用原始采样数据，按 Ping 间隔判断断档。</span>
                  <span>长周期（1M/3M/6M/1Y）由后端按更小数据桶聚合：6 小时、12 小时、1 天、2 天；可选范围受数据保留期限制。</span>
                  <span>长周期 X 轴标签间距保持不变，只增加折线数据点密度；连续缺失聚合桶时才截断。</span>
                </b>
              </button>
              <button
                class="modal-icon-button probe-history-toggle"
                type="button"
                :class="{ active: showInactiveProbeHistory }"
                :disabled="probeLoading || !activeProbeNode"
                :title="showInactiveProbeHistory ? '隐藏停用历史' : '显示停用历史'"
                :aria-label="showInactiveProbeHistory ? '隐藏停用历史' : '显示停用历史'"
                :aria-pressed="showInactiveProbeHistory"
                @click="toggleInactiveProbeHistory"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M3 12a9 9 0 1 0 3-6.7M3 5v5h5" />
                  <path d="M12 7v5l3 2" />
                </svg>
              </button>
              <button class="modal-icon-button" type="button" :disabled="probeLoading || !activeProbeNode" title="刷新" @click="activeProbeNode && loadProbeResults(activeProbeNode.node_id)">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M20 12a8 8 0 1 1-2.34-5.66M20 4v5h-5" />
                </svg>
              </button>
              <button class="modal-icon-button" type="button" title="关闭" @click="probePanelOpen = false">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M6 6l12 12M18 6 6 18" />
                </svg>
              </button>
            </div>
          </div>
          <div class="probe-panel-head">
            <div>
              <span>时间维度</span>
              <small>{{ probeRangeMeta }}</small>
            </div>
            <div class="probe-panel-actions">
              <div class="probe-range-switch">
                <button
                  v-for="item in availableProbeRangeOptions"
                  :key="item.value"
                  type="button"
                  :class="{ active: probeRange === item.value }"
                  @click="setProbeRange(item.value)"
                >
                  {{ item.label }}
                </button>
              </div>
            </div>
          </div>
          <div ref="probeChartEl" class="probe-chart-canvas" />
          <div class="probe-stat-grid">
            <div v-for="item in probeStats" :key="item.key" class="probe-stat-card">
              <div class="probe-stat-title">
                <div>
                  <strong>{{ item.name }}</strong>
                  <span>{{ item.target }} · {{ probeModeLabel(item.type, item.ipVersion) }}</span>
                </div>
                <b :class="{ failed: item.latestStatus !== 'success' && item.samples > 0 && !item.inactive, inactive: item.inactive }">
                  {{ item.inactive ? '已停用' : item.samples === 0 ? '无数据' : item.latestStatus === 'success' ? '正常' : '失败' }}
                </b>
              </div>
              <div class="probe-stat-main">
                <span>最新延迟</span>
                <strong>{{ item.latestStatus === 'success' ? formatLatency(item.latestLatency) : item.samples ? '失败' : 'N/A' }}</strong>
              </div>
              <div class="probe-stat-metrics">
                <div><span>均值</span><strong>{{ formatLatency(item.averageLatency) }}</strong></div>
                <div><span>丢包率</span><strong>{{ formatPacketLoss(item.packetLoss) }}</strong></div>
                <div><span>抖动</span><strong>{{ formatLatency(item.jitter) }}</strong></div>
                <div><span>成功/总样本</span><strong>{{ item.successSamples }} / {{ item.samples }}</strong></div>
                <div><span>最低</span><strong>{{ formatLatency(item.minLatency) }}</strong></div>
                <div><span>最高</span><strong>{{ formatLatency(item.maxLatency) }}</strong></div>
              </div>
            </div>
            <div v-if="probeStats.length === 0" class="probe-empty">
              {{ showInactiveProbeHistory ? '暂无 Ping 历史数据' : '暂无启用的 Ping 节点' }}
            </div>
          </div>
        </div>
        <div v-if="probeLoading" class="content-loading-overlay modal-loading-overlay" role="status" aria-live="polite">
          <span class="loading-orb"></span>
          <strong>Loading...</strong>
        </div>
      </n-modal>

      <n-modal v-model:show="metricsPanelOpen" preset="card" class="metrics-modal" :closable="false">
        <div class="chart-modal-scroll">
          <div class="chart-modal-head">
            <div>
              <h3>{{ metricsModalTitle }} <small v-if="activeMetricsNodeOnline">{{ metricsRefreshCountdown }}s 后刷新</small></h3>
            </div>
            <div class="chart-modal-actions">
              <button class="range-help modal-help" type="button" aria-label="查看时间维度说明">
                ?
                <b>
                  <span v-for="item in availableProbeRangeOptions" :key="item.value">{{ item.help }}</span>
                </b>
              </button>
              <button class="modal-icon-button" type="button" :disabled="metricsPanelLoading || !activeMetricsNode || !activeMetricsNodeOnline" title="通知 Agent 上报并刷新" @click="activeMetricsNode && refreshMetricsPanelData(activeMetricsNode.node_id)">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M20 12a8 8 0 1 1-2.34-5.66M20 4v5h-5" />
                </svg>
              </button>
              <button class="modal-icon-button" type="button" title="关闭" @click="metricsPanelOpen = false">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M6 6l12 12M18 6 6 18" />
                </svg>
              </button>
            </div>
          </div>
          <div class="probe-panel-head">
            <div>
              <span>统计维度</span>
              <small>{{ metricsRangeMeta }}</small>
            </div>
            <div class="probe-panel-actions">
              <div class="probe-range-switch">
                <button
                  v-for="item in availableProbeRangeOptions"
                  :key="item.value"
                  type="button"
                  :class="{ active: metricsRange === item.value }"
                  @click="setMetricsRange(item.value)"
                >
                  {{ item.label }}
                </button>
              </div>
            </div>
          </div>

          <div class="metrics-summary-grid" v-if="activeMetricsNode">
            <div class="metric-wide"><span>系统详细名称</span><strong :title="activeMetricsNode.latest_metric?.os_name || formatOSName(activeMetricsNode.latest_metric)">{{ activeMetricsNode.latest_metric?.os_name || formatOSName(activeMetricsNode.latest_metric) }}</strong></div>
            <div><span>CPU</span><strong>{{ formatCPUCores(activeMetricsNode.latest_metric) }}</strong></div>
            <div><span>架构</span><strong>{{ formatMetricText(activeMetricsNode.latest_metric?.arch) }}</strong></div>
            <div><span>虚拟化</span><strong>{{ formatMetricText(activeMetricsNode.latest_metric?.virtualization) }}</strong></div>
            <div><span>显卡</span><strong :title="activeMetricsNode.latest_metric?.gpu || 'N/A'">{{ formatMetricText(activeMetricsNode.latest_metric?.gpu) }}</strong></div>
            <div><span>内存</span><strong>{{ formatBytes(activeMetricsNode.latest_metric?.mem_total) }}</strong></div>
            <div><span>Swap</span><strong>{{ formatSwap(activeMetricsNode.latest_metric) }}</strong></div>
            <div><span>硬盘</span><strong>{{ formatBytes(activeMetricsNode.latest_metric?.disk_total) }}</strong></div>
            <div><span>负载</span><strong>{{ formatLoad(activeMetricsNode.latest_metric) }}</strong></div>
            <div><span>在线</span><strong>{{ formatNodeUptime(activeMetricsNode) }}</strong></div>
          </div>

          <div class="metrics-chart-shell">
            <div class="metrics-chart-grid">
              <section>
                <div class="metrics-chart-title">
                  <strong>CPU 使用率</strong>
                  <span>{{ metricsLatestValue('cpu') }}</span>
                </div>
                <div ref="metricsCPUChartEl" class="metrics-chart-canvas" />
              </section>
              <section>
                <div class="metrics-chart-title">
                  <strong>内存使用率</strong>
                  <span>{{ metricsLatestValue('memory') }}</span>
                </div>
                <div ref="metricsMemoryChartEl" class="metrics-chart-canvas" />
              </section>
              <section>
                <div class="metrics-chart-title">
                  <strong>硬盘使用率</strong>
                  <span>{{ metricsLatestValue('disk') }}</span>
                </div>
                <div ref="metricsDiskChartEl" class="metrics-chart-canvas" />
              </section>
              <section>
                <div class="metrics-chart-title">
                  <strong>网络上下行</strong>
                  <span>{{ metricsLatestValue('network') }}</span>
                </div>
                <div ref="metricsNetworkChartEl" class="metrics-chart-canvas" />
              </section>
              <section>
                <div class="metrics-chart-title">
                  <strong>流量上下行</strong>
                  <span>{{ metricsLatestValue('traffic') }}</span>
                </div>
                <div ref="metricsTrafficChartEl" class="metrics-chart-canvas" />
              </section>
            </div>
          </div>

          <section v-if="isLoggedIn && latestSnapshot" class="snapshot-panel">
            <div class="snapshot-panel-head">
              <div>
                <strong>进程与连接快照</strong>
                <span>最新采样 {{ formatSnapshotTime(latestSnapshot) }}</span>
              </div>
              <b>{{ `${latestSnapshot.process_count} 进程 · ${latestSnapshot.connection_count} 连接` }}</b>
            </div>

            <div class="snapshot-summary-grid">
              <div><span>进程</span><strong>{{ latestSnapshot.process_count }}</strong></div>
              <div><span>线程</span><strong>{{ latestSnapshot.thread_count }}</strong></div>
              <div><span>连接</span><strong>{{ latestSnapshot.connection_count }}</strong></div>
              <div><span>监听</span><strong>{{ latestSnapshot.listen_count }}</strong></div>
            </div>

            <div v-if="latestSnapshot && snapshotTCPStateItems.length" class="snapshot-state-list">
              <span v-for="[state, count] in snapshotTCPStateItems" :key="state">{{ state }} {{ count }}</span>
            </div>

            <div class="snapshot-data-grid">
              <div class="snapshot-table-card">
                <div class="snapshot-table-head">
                  <strong>Top 进程</strong>
                  <span>{{ snapshotProcesses.length }} 条</span>
                </div>
                <div class="snapshot-table">
                  <div class="snapshot-row snapshot-row-head">
                    <span>PID</span><span>名称</span><span>CPU</span><span>内存</span><span>线程</span>
                  </div>
                  <div class="snapshot-table-body">
                    <div v-for="item in snapshotProcesses" :key="`${item.pid}-${item.name}`" class="snapshot-row">
                      <span>{{ item.pid }}</span>
                      <span :title="item.command || item.name">{{ item.name || item.command || '-' }}</span>
                      <span>{{ formatSnapshotCPU(item.cpu_percent) }}</span>
                      <span>{{ formatBytes(item.memory_bytes) }}</span>
                      <span>{{ item.thread_count }}</span>
                    </div>
                    <div v-if="snapshotProcesses.length === 0" class="snapshot-empty">暂无进程快照</div>
                  </div>
                </div>
              </div>

              <div class="snapshot-table-card">
                <div class="snapshot-table-head">
                  <strong>连接信息</strong>
                  <span>{{ snapshotConnections.length }} 条</span>
                </div>
                <div class="snapshot-table snapshot-connection-table">
                  <div class="snapshot-row snapshot-row-head">
                    <span>协议</span><span>本地</span><span>远端</span><span>状态</span><span>进程</span>
                  </div>
                  <div class="snapshot-table-body">
                    <div v-for="(item, index) in snapshotConnections" :key="`${item.protocol}-${item.local_addr}-${item.local_port}-${index}`" class="snapshot-row">
                      <span>{{ item.protocol }}</span>
                      <span :title="formatEndpoint(item.local_addr, item.local_port)">{{ formatEndpoint(item.local_addr, item.local_port) }}</span>
                      <span :title="formatEndpoint(item.remote_addr, item.remote_port)">{{ formatEndpoint(item.remote_addr, item.remote_port) }}</span>
                      <span>{{ item.state }}</span>
                      <span>{{ item.process_name || (item.pid ? String(item.pid) : '-') }}</span>
                    </div>
                    <div v-if="snapshotConnections.length === 0" class="snapshot-empty">暂无连接快照</div>
                  </div>
                </div>
              </div>
            </div>
          </section>
        </div>
        <div v-if="metricsPanelLoading" class="content-loading-overlay modal-loading-overlay" role="status" aria-live="polite">
          <span class="loading-orb"></span>
          <strong>Loading...</strong>
        </div>
      </n-modal>

      <n-modal
        v-model:show="nodeEditOpen"
        preset="card"
        class="node-editor-modal"
        content-class="node-editor-modal-content"
        footer-class="node-editor-modal-footer"
        title="编辑 Agent 基本信息"
        style="display: flex; width: min(980px, calc(100vw - 96px)); height: min(760px, calc(100vh - 88px)); max-width: calc(100vw - 96px); max-height: calc(100vh - 88px); overflow: hidden; flex-direction: column;"
      >
        <div class="node-editor-scroll">
          <div class="node-editor-layout">
            <aside class="node-editor-guide">
            <span class="guide-kicker">Agent 配置</span>
            <h3>{{ editingNode ? nodeLabel(editingNode) : adminEditNodeID || '未选择 Agent' }}</h3>
            <p>保存后 Master 会通过当前 TCP 长连接下发心跳和指标间隔；Agent 不在线时会在下次注册后生效。</p>
            <dl>
              <div>
                <dt>Node ID</dt>
                <dd>{{ adminEditNodeID || '-' }}</dd>
              </div>
              <div>
                <dt>公网 IPv4</dt>
                <dd>{{ nodePrimaryPublicIPv4(editingNode) || '-' }}</dd>
              </div>
              <div>
                <dt>公网 IPv6</dt>
                <dd>{{ nodePrimaryPublicIPv6(editingNode) || '-' }}</dd>
              </div>
              <div>
                <dt>本机主 IP</dt>
                <dd>{{ nodePrimaryLocalIP(editingNode) || '-' }}</dd>
              </div>
              <div>
                <dt>本机 IPv4</dt>
                <dd>{{ displayIPList(nodeIPv4List(editingNode)) || '-' }}</dd>
              </div>
              <div>
                <dt>本机 IPv6</dt>
                <dd>{{ displayIPList(nodeIPv6List(editingNode)) || '-' }}</dd>
              </div>
              <div>
                <dt>地区</dt>
                <dd class="node-region-detail">
                  <span
                    class="node-flag"
                    :class="regionFlagClass(editingNode?.region)"
                    :style="regionFlagStyle(editingNode?.region)"
                    role="img"
                    :aria-label="regionFlagLabel(editingNode?.region)"
                  />
                  <span>{{ displayRegion(editingNode?.region) }}</span>
                </dd>
              </div>
              <div>
                <dt>状态</dt>
                <dd>{{ editingNode?.status || '-' }}</dd>
              </div>
            </dl>
            </aside>

            <n-tabs class="node-editor-tabs" type="line" animated>
            <n-tab-pane name="basic" tab="基础信息">
              <n-form class="node-editor-form" :model="nodeEditor" label-placement="left" label-width="84">
                <n-form-item label="节点名称">
                  <n-input v-model:value="nodeEditor.name" placeholder="例如 HK-BGP-01" />
                </n-form-item>
                <n-form-item label="Tag">
                  <n-input v-model:value="nodeEditor.tag" placeholder="最多 5 个，例如 prod/cn2/backup，不允许空格" />
                </n-form-item>
                <n-form-item label="服务商">
                  <n-input v-model:value="nodeEditor.provider" placeholder="例如 Oracle / VMISS" />
                </n-form-item>
                <n-form-item label="线路">
                  <n-select
                    class="network-line-select"
                    v-model:value="nodeEditor.network_line"
                    filterable
                    multiple
                    :virtual-scroll="false"
                    :max-tag-count="2"
                    :menu-props="networkLineMenuProps"
                    :ellipsis-tag-popover-props="networkLineTagPopoverProps"
                    :node-props="networkLineNodeProps"
                    :options="networkLineOptions"
                    placeholder="搜索选择线路，可多选"
                    @update:show="handleNetworkLineSelectShow"
                  />
                </n-form-item>
                <n-form-item label="地区">
                  <n-select
                    v-model:value="nodeEditor.region"
                    filterable
                    :options="regionOptions"
                    :render-label="renderRegionOptionLabel"
                    placeholder="选择国家/地区代码"
                  />
                </n-form-item>
                <n-form-item label="心跳间隔">
                  <n-input-number v-model:value="nodeEditor.heartbeat_interval" :min="3" :step="1">
                    <template #suffix>秒</template>
                  </n-input-number>
                </n-form-item>
                <n-form-item label="指标间隔">
                  <n-input-number v-model:value="nodeEditor.metrics_interval" :min="3" :step="1">
                    <template #suffix>秒</template>
                  </n-input-number>
                </n-form-item>
                <n-form-item label="付费方式">
                  <n-select v-model:value="nodeEditor.billing_cycle" :options="billingCycleOptions" />
                </n-form-item>
                <n-form-item label="金额">
                  <n-input-number v-model:value="nodeEditor.price_amount" :min="0" :precision="2" />
                </n-form-item>
                <n-form-item label="币种">
                  <n-select v-model:value="nodeEditor.currency" :options="currencyOptions" />
                </n-form-item>
                <n-form-item label="服务周期">
                  <n-date-picker v-model:value="nodeEditor.service_range" type="daterange" clearable to="body" placement="bottom-end" />
                </n-form-item>
                <n-form-item label="总流量">
                  <div class="traffic-limit-input">
                    <n-input-number v-model:value="nodeEditor.traffic_limit_value" :min="0" :precision="2" placeholder="0 为不限" />
                    <n-select v-model:value="nodeEditor.traffic_limit_unit" :options="trafficUnitOptions" />
                  </div>
                </n-form-item>
                <n-form-item label="校准流量">
                  <n-input-number v-model:value="nodeEditor.traffic_calibration_value" :min="0" :precision="2" placeholder="当前周期补偿流量">
                    <template #suffix>{{ nodeEditor.traffic_limit_unit }}</template>
                  </n-input-number>
                </n-form-item>
                <n-form-item label="计费方向">
                  <n-select v-model:value="nodeEditor.traffic_billing_direction" :options="trafficBillingDirectionOptions" />
                </n-form-item>
                <n-form-item label="重置周期">
                  <n-select v-model:value="nodeEditor.traffic_reset_cycle" :options="trafficResetOptions" />
                </n-form-item>
              </n-form>
            </n-tab-pane>

            <n-tab-pane name="probe" tab="Ping 节点">
              <div class="probe-editor-panel">
                <div class="probe-editor-note">
                  <strong>选择这个 Agent 使用的 Ping 节点</strong>
                  <span>这里只展示已启用的全局 Ping 节点；卡片开关控制当前 Agent 是否启用该节点，停用的全局节点不会下发。</span>
                </div>

                <div class="probe-assignment-head">
                  <div>
                    <strong>{{ selectedProbeTaskCount() }} / {{ enabledProbeTasks.length }}</strong>
                    <span>已选择</span>
                  </div>
                  <n-button size="small" tertiary @click="openCreateProbeTask">
                    添加全局节点
                  </n-button>
                </div>

                <div class="probe-assignment-grid">
                  <div
                    v-for="task in enabledProbeTasks"
                    :key="task.id"
                    class="probe-assignment-card"
                    :class="{ selected: isProbeTaskSelected(task.id) }"
                  >
                    <div class="probe-assignment-main">
                      <div>
                        <strong>{{ task.name || displayTarget(task.target) }}</strong>
                        <span>{{ displayTarget(task.target) }}</span>
                      </div>
                      <n-switch
                        :value="isProbeTaskSelected(task.id)"
                        size="small"
                        @update:value="(value: boolean) => setProbeTaskSelected(task.id, value)"
                      />
                    </div>
                    <div class="probe-assignment-meta">
                      <span>{{ probeModeLabel(task.type, task.ip_version) }}</span>
                      <span>{{ task.interval_seconds }}s</span>
                      <span>{{ task.timeout_ms }}ms</span>
                      <b :class="{ off: !isProbeTaskSelected(task.id) }">{{ isProbeTaskSelected(task.id) ? '启用' : '禁用' }}</b>
                    </div>
                  </div>
                  <div v-if="enabledProbeTasks.length === 0" class="probe-empty">暂无启用的全局 Ping 节点，请先添加或启用。</div>
                </div>
              </div>
            </n-tab-pane>

            <n-tab-pane name="snapshot" tab="快照采集">
              <div class="snapshot-editor-panel">
                <div class="probe-editor-note">
                  <strong>进程与连接快照</strong>
                  <span>默认继承全局设置。开启覆盖后，Master 会把当前 Agent 的采集策略下发给 Agent，Agent 按独立间隔上报，不混入 metrics。</span>
                </div>

                <n-form class="node-editor-form" :model="nodeEditor" label-placement="left" label-width="104">
                  <n-form-item label="覆盖全局设置">
                    <n-switch v-model:value="nodeEditor.snapshot_override" />
                  </n-form-item>
                  <n-form-item label="启用采集">
                    <n-switch v-model:value="nodeEditor.snapshot_enabled" :disabled="!nodeEditor.snapshot_override" />
                  </n-form-item>
                  <n-form-item label="采集进程">
                    <n-switch v-model:value="nodeEditor.snapshot_collect_processes" :disabled="!nodeEditor.snapshot_override" />
                  </n-form-item>
                  <n-form-item label="采集连接">
                    <n-switch v-model:value="nodeEditor.snapshot_collect_connections" :disabled="!nodeEditor.snapshot_override" />
                  </n-form-item>
                  <n-form-item label="敏感信息脱敏">
                    <n-switch v-model:value="nodeEditor.snapshot_mask_sensitive" :disabled="!nodeEditor.snapshot_override" />
                  </n-form-item>
                  <n-form-item label="采集间隔">
                    <n-input-number v-model:value="nodeEditor.snapshot_interval" :min="15" :max="3600" :step="5" :disabled="!nodeEditor.snapshot_override">
                      <template #suffix>秒</template>
                    </n-input-number>
                  </n-form-item>
                  <n-form-item label="进程数量">
                    <n-input-number v-model:value="nodeEditor.snapshot_process_limit" :min="1" :max="50" :step="1" :disabled="!nodeEditor.snapshot_override">
                      <template #suffix>条</template>
                    </n-input-number>
                  </n-form-item>
                  <n-form-item label="连接数量">
                    <n-input-number v-model:value="nodeEditor.snapshot_connection_limit" :min="1" :max="500" :step="10" :disabled="!nodeEditor.snapshot_override">
                      <template #suffix>条</template>
                    </n-input-number>
                  </n-form-item>
                </n-form>
              </div>
            </n-tab-pane>
            </n-tabs>
          </div>
        </div>

        <template #footer>
          <n-space justify="end">
            <n-button @click="nodeEditOpen = false">取消</n-button>
            <n-button type="primary" :disabled="!adminEditNodeID" :loading="adminLoading" @click="saveNodeConfig">
              保存 Agent 配置
            </n-button>
          </n-space>
        </template>
      </n-modal>

      <n-modal v-model:show="probeTaskModalOpen" preset="card" class="probe-task-modal" :title="probeTaskModalTitle">
        <n-form class="probe-editor-form" :model="probeTaskEditor" label-placement="left" label-width="96">
          <n-form-item label="名称">
            <n-input v-model:value="probeTaskEditor.name" placeholder="例如 Google / HK BGP" />
          </n-form-item>
          <n-form-item label="地址">
            <n-input v-model:value="probeTaskEditor.target" placeholder="TCP: host:port 或裸 IP，ICMP: host / IPv4 / IPv6" />
          </n-form-item>
          <n-form-item label="协议">
            <n-select v-model:value="probeTaskEditor.type" :options="probeTypeOptions" />
          </n-form-item>
          <n-form-item label="IP 版本">
            <n-select v-model:value="probeTaskEditor.ip_version" :options="probeIPVersionOptions" />
          </n-form-item>
          <n-form-item label="间隔">
            <n-input-number v-model:value="probeTaskEditor.interval_seconds" :min="3" :step="1">
              <template #suffix>秒</template>
            </n-input-number>
          </n-form-item>
          <n-form-item label="超时">
            <n-input-number v-model:value="probeTaskEditor.timeout_ms" :min="100" :max="30000" :step="100">
              <template #suffix>ms</template>
            </n-input-number>
          </n-form-item>
          <n-form-item label="启用">
            <n-switch v-model:value="probeTaskEditor.enabled" :disabled="!probeTaskEditor.id && probeTaskEditor.assign_to_all_agents" />
          </n-form-item>
          <n-form-item v-if="!probeTaskEditor.id" label="推送">
            <div class="probe-push-field">
              <n-switch v-model:value="probeTaskEditor.assign_to_all_agents" @update:value="handleProbeAssignAllChange" />
              <span>创建后启用该节点，默认分配给所有 Agent，并立即下发配置</span>
            </div>
          </n-form-item>
        </n-form>

        <template #footer>
          <n-space justify="end">
            <n-button @click="probeTaskModalOpen = false">取消</n-button>
            <n-button type="primary" :loading="probeSaving" @click="saveProbeTask">
              保存
            </n-button>
          </n-space>
        </template>
      </n-modal>
    </main>
  </n-config-provider>
</template>
