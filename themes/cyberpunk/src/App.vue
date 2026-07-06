<script setup lang="ts">
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent, GraphicComponent } from 'echarts/components'
import { init, use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { EChartsOption } from 'echarts'
import type { EChartsType } from 'echarts/core'
import type { AppSettings, CyberNode, DashboardEvent, DashboardNodeProbeStat, NodeMetric, NodeRecord, ProbeResultOverview, ProbeResultsResponse, ProbeTask, PublicIPs, ServerTime, Summary } from './types'

type NodeFilter = 'all' | 'online' | 'warning' | 'offline'
type TrendModalKind = 'hardware' | 'ping'
type TrendRange = '1h' | '4h' | '12h' | '1d' | '7d' | '1m' | '3m' | '6m' | '1y'

use([LineChart, GridComponent, LegendComponent, TooltipComponent, GraphicComponent, CanvasRenderer])

interface FeedItem {
  time: string
  nodeName?: string
  text: string
}

interface PingNodeStat {
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

const refreshIntervalMS = 15000
const minuteMS = 60 * 1000
const hourMS = 60 * minuteMS
const dayMS = 24 * hourMS
const apiBaseURL = __RIVO_API_BASE_URL__
const filter = ref<NodeFilter>('all')
const loading = ref(false)
const loadError = ref('')
const lastLoadedAt = ref(0)
const now = ref(Date.now())
const expandedNodeIDs = ref<Set<string>>(new Set())
const rawNodes = ref<NodeRecord[]>([])
const summary = ref<Summary | null>(null)
const dashboardEvents = ref<DashboardEvent[]>([])
const settings = ref<Partial<AppSettings>>({})
const serverTime = ref<ServerTime | null>(null)
const serverClockDeltaMS = ref(0)
const trendModalOpen = ref(false)
const trendModalKind = ref<TrendModalKind>('hardware')
const trendModalNode = ref<CyberNode | null>(null)
const trendRange = ref<TrendRange>('1h')
const trendRangeAnchor = ref(Date.now())
const trendLoading = ref(false)
const trendError = ref('')
const trendMetrics = ref<NodeMetric[]>([])
const trendProbe = ref<ProbeResultsResponse | null>(null)
const hardwareChartEl = ref<HTMLDivElement | null>(null)
const pingChartEl = ref<HTMLDivElement | null>(null)

let clockTimer: number | undefined
let refreshTimer: number | undefined
let trendRequestID = 0
let hardwareChart: EChartsType | null = null
let pingChart: EChartsType | null = null

const filterOptions: Array<{ value: NodeFilter; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'online', label: '在线' },
  { value: 'warning', label: '告警' },
  { value: 'offline', label: '离线' }
]

const trendRangeOptions: Array<{ label: string; value: TrendRange; hours: number; summary: string }> = [
  { label: '1H', value: '1h', hours: 1, summary: '最近 1 小时' },
  { label: '4H', value: '4h', hours: 4, summary: '最近 4 小时' },
  { label: '12H', value: '12h', hours: 12, summary: '最近 12 小时' },
  { label: '1D', value: '1d', hours: 24, summary: '最近 1 天' },
  { label: '7D', value: '7d', hours: 24 * 7, summary: '最近 7 天' },
  { label: '1M', value: '1m', hours: 24 * 30, summary: '最近 1 个月' },
  { label: '3M', value: '3m', hours: 24 * 90, summary: '最近 3 个月' },
  { label: '6M', value: '6m', hours: 24 * 180, summary: '最近 6 个月' },
  { label: '1Y', value: '1y', hours: 24 * 365, summary: '最近 1 年' }
]

function apiPath(path: string) {
  if (!apiBaseURL) return path
  return `${apiBaseURL.replace(/\/$/, '')}${path}`
}

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(apiPath(path), {
    headers: { accept: 'application/json' },
    cache: 'no-store'
  })
  if (!response.ok) throw new Error(`${path} ${response.status}`)
  return response.json() as Promise<T>
}

function delay(ms: number) {
  return new Promise(resolve => window.setTimeout(resolve, ms))
}

async function fetchTrendJSON<T>(path: string): Promise<T> {
  try {
    return await fetchJSON<T>(path)
  } catch (error) {
    if (!(error instanceof TypeError) && !(error instanceof Error && error.message === 'Failed to fetch')) {
      throw error
    }
    await delay(250)
    return fetchJSON<T>(path)
  }
}

function asNumber(value: unknown, fallback = 0) {
  const number = Number(value)
  return Number.isFinite(number) ? number : fallback
}

function clamp(value: number, min = 0, max = 100) {
  return Math.max(min, Math.min(max, value))
}

function average(values: number[]) {
  const valid = values.filter(Number.isFinite)
  if (!valid.length) return 0
  return valid.reduce((sum, value) => sum + value, 0) / valid.length
}

function formatNumber(value: number, digits = 1) {
  return asNumber(value).toFixed(digits)
}

function optionalScore(value: unknown) {
  const score = Number(value)
  return Number.isFinite(score) ? score : null
}

function formatPercent(value: number, digits = 1) {
  return `${formatNumber(value, digits)}%`
}

function formatBytes(value?: number | null) {
  let current = asNumber(value, 0)
  if (current <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let unit = 0
  while (current >= 1024 && unit < units.length - 1) {
    current /= 1024
    unit++
  }
  return `${current >= 10 || unit === 0 ? current.toFixed(0) : current.toFixed(1)} ${units[unit]}`
}

function trafficLimitLabel(value?: number | null) {
  const limit = asNumber(value, 0)
  return limit > 0 ? formatBytes(limit) : '不限'
}

function trafficRemainingLabel(limit?: number | null, remaining?: number | null) {
  return asNumber(limit, 0) > 0 ? formatBytes(remaining) : '不限'
}

function trafficUsagePercent(used?: number | null, limit?: number | null) {
  const total = asNumber(limit, 0)
  if (total <= 0) return 0
  return Math.round(clamp(asNumber(used, 0) / total * 100))
}

function trafficResetCycleLabel(value?: string | null) {
  switch (String(value || '').toLowerCase()) {
    case 'daily':
      return '每日重置'
    case 'yearly':
      return '每年重置'
    case 'never':
      return '不重置'
    case 'monthly':
    default:
      return '每月重置'
  }
}

function trafficBillingDirectionLabel(value?: string | null) {
  return String(value || '').toLowerCase() === 'outbound' ? '仅上行计费' : '上下行计费'
}

function formatBps(value?: number | null) {
  let current = Math.max(0, asNumber(value, 0))
  const units = ['bps', 'Kbps', 'Mbps', 'Gbps', 'Tbps']
  let unit = 0
  while (current >= 1000 && unit < units.length - 1) {
    current /= 1000
    unit++
  }
  return {
    amount: current >= 10 || unit === 0 ? current.toFixed(0) : current.toFixed(1),
    unit: units[unit],
    label: `${current >= 10 || unit === 0 ? current.toFixed(0) : current.toFixed(1)} ${units[unit]}`
  }
}

function formatUptime(value?: number | null) {
  const seconds = Math.max(0, asNumber(value, 0))
  if (!seconds) return '--'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor(seconds % 86400 / 3600)
  const minutes = Math.floor(seconds % 3600 / 60)
  if (days > 0) return `${days}天 ${hours}小时`
  if (hours > 0) return `${hours}小时 ${minutes}分`
  if (minutes > 0) return `${minutes}分`
  return '<1分'
}

function parseJSON<T>(raw: string | undefined | null, fallback: T): T {
  if (!raw) return fallback
  try {
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}

function publicIPList(node: NodeRecord) {
  const parsed = node.public_ips || parseJSON<PublicIPs>(node.public_ips_json, {})
  const ipv4 = Array.isArray(parsed.ipv4) ? parsed.ipv4.map(item => item.ip).filter(Boolean) : []
  const ipv6 = Array.isArray(parsed.ipv6) ? parsed.ipv6.map(item => item.ip).filter(Boolean) : []
  return [node.public_ip, node.public_ipv6, ...ipv4, ...ipv6].filter(Boolean)
}

function networkLineList(value?: string | null) {
  const lines = String(value || '')
    .split(/[\/,，、]/)
    .map(item => item.trim())
    .filter(Boolean)
  return lines.length ? Array.from(new Set(lines)) : ['未标注线路']
}

function maskIP(ip: string) {
  if (!settings.value.mask_ip_addresses || !ip) return ip
  if (ip.includes(':')) {
    const parts = ip.split(':').filter(Boolean)
    if (parts.length <= 2) return '****'
    return `${parts[0]}:****:${parts[parts.length - 1]}`
  }
  const parts = ip.split('.')
  if (parts.length !== 4) return '****'
  return `${parts[0]}.${parts[1]}.***.***`
}

function normalizeTimestamp(value?: number | null) {
  const stamp = asNumber(value, 0)
  if (!stamp) return 0
  return stamp < 1000000000000 ? stamp * 1000 : stamp
}

function formatTime(value?: number | null) {
  const stamp = normalizeTimestamp(value)
  if (!stamp) return '--'
  return new Date(stamp).toLocaleTimeString('zh-CN', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function formatRangeWindowTime(value: number) {
  const date = new Date(value)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:${minute}`
}

function formatServerClock(value: number) {
  const date = new Date(value)
  const hours = String(date.getUTCHours()).padStart(2, '0')
  const minutes = String(date.getUTCMinutes()).padStart(2, '0')
  const seconds = String(date.getUTCSeconds()).padStart(2, '0')
  return `${hours}:${minutes}:${seconds}`
}

function formatServerDate(value: number) {
  const date = new Date(value)
  const year = date.getUTCFullYear()
  const month = String(date.getUTCMonth() + 1).padStart(2, '0')
  const day = String(date.getUTCDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function meterStyle(value: number) {
  return { '--v': `${clamp(value)}%` }
}

function trafficMeterStyle(value: number) {
  return { '--traffic-v': `${clamp(value)}%` }
}

function probeForNode(nodeID: string): DashboardNodeProbeStat {
  return summary.value?.node_probe_stats?.[nodeID] || {
    samples: 0,
    success_samples: 0,
    failed_samples: 0
  }
}

function mapNode(node: NodeRecord, index: number): CyberNode {
  const metric = node.latest_metric || null
  const probe = probeForNode(node.node_id)
  const health = summary.value?.node_health_scores?.[node.node_id]
  const isOnline = node.status === 'online'
  const hasMetric = Boolean(metric)
  const cpu = isOnline && metric ? asNumber(metric.cpu_usage) : 0
  const memory = isOnline && metric ? asNumber(metric.mem_used_percent) : 0
  const disk = metric ? asNumber(metric.disk_used_percent) : 0
  const rx = isOnline && metric ? asNumber(metric.net_rx_bps) : 0
  const tx = isOnline && metric ? asNumber(metric.net_tx_bps) : 0
  const latency = probe.avg_latency_ms === undefined || probe.avg_latency_ms === null ? null : asNumber(probe.avg_latency_ms)
  const packetLoss = probe.packet_loss_percent === undefined || probe.packet_loss_percent === null ? 0 : asNumber(probe.packet_loss_percent)
  const healthScore = optionalScore(health?.score)
  const freshnessScore = optionalScore(health?.freshness_score)
  const resourceScore = optionalScore(health?.resource_score)
  const loadScore = optionalScore(health?.load_score)
  const networkScore = optionalScore(health?.network_score)
  const stabilityScore = optionalScore(health?.stability_score)
  const warning = isOnline && (
    (healthScore !== null && healthScore < 70) ||
    (healthScore === null && (cpu >= 80 || memory >= 85 || disk >= 90 || packetLoss >= 5 || (latency !== null && latency >= 250)))
  )
  const status: CyberNode['status'] = isOnline ? (warning ? 'warning' : 'online') : 'offline'
  const primaryIP = maskIP(publicIPList(node)[0] || '')
  const region = String(node.region || '未标注地区').trim()
  const provider = String(node.provider || '未知服务商').trim()
  const networkLines = networkLineList(node.network_line)
  const networkLine = networkLines.join(' / ')
  const name = String(node.name || node.node_id || `node-${index + 1}`).trim()
  const nodeSummary = [region, `IP ${primaryIP || '--'}`].filter(Boolean).join(' · ')
  const location = [region, provider, networkLine, primaryIP].filter(Boolean).join(' · ') || '等待 Agent 上报网络信息'
  const availability = probe.availability_percent === undefined || probe.availability_percent === null ? '--' : `${formatNumber(probe.availability_percent, 2)}%`
  const remainingDays = Number.isFinite(Number(node.remaining_days)) ? Number(node.remaining_days) : null
  const uptimeSeconds = metric && Number.isFinite(Number(metric.uptime_seconds)) ? Number(metric.uptime_seconds) : 0
  const trafficPercent = trafficUsagePercent(node.traffic_used_bytes, node.traffic_limit_bytes)

  return {
    id: String(node.node_id || node.id || index),
    name,
    location,
    nodeSummary,
    provider,
    networkLine,
    networkLines,
    status,
    cpu,
    memory,
    disk,
    load: metric ? `${formatNumber(metric.load1, 2)} / ${formatNumber(metric.load5, 2)} / ${formatNumber(metric.load15, 2)}` : '-- / -- / --',
    availability,
    throughput: formatBps(rx + tx).label,
    trafficLimit: trafficLimitLabel(node.traffic_limit_bytes),
    trafficUsed: formatBytes(node.traffic_used_bytes),
    trafficRemaining: trafficRemainingLabel(node.traffic_limit_bytes, node.traffic_remaining_bytes),
    trafficUsagePercent: trafficPercent,
    trafficResetCycle: trafficResetCycleLabel(node.traffic_reset_cycle),
    trafficBillingDirection: trafficBillingDirectionLabel(node.traffic_billing_direction),
    latency: latency === null ? (status === 'offline' ? '超时' : '--') : `${formatNumber(latency, 1)} ms`,
    latencyClass: latency === null ? '' : latency >= 250 ? 'hot' : latency >= 120 ? 'warm' : 'cool',
    asset: remainingDays === null ? '未配置' : remainingDays < 0 ? '已到期' : `${remainingDays} 天`,
    lastSeenAt: normalizeTimestamp(node.last_seen_at || metric?.ts || 0),
    hasMetric,
    uptimeSeconds,
    rx,
    tx,
    memoryTotal: asNumber(metric?.mem_total),
    memoryUsed: asNumber(metric?.mem_used),
    diskTotal: asNumber(metric?.disk_total),
    diskUsed: asNumber(metric?.disk_used),
    packetLoss,
    healthScore,
    healthGrade: health?.grade || '',
    freshnessScore,
    resourceScore,
    loadScore,
    networkScore,
    stabilityScore,
    jitter: probe.jitter_ms === undefined || probe.jitter_ms === null ? '--' : `${formatNumber(probe.jitter_ms, 1)} ms`,
    latencySpike: probe.latency_spike_ratio === undefined || probe.latency_spike_ratio === null ? '--' : `${formatNumber(probe.latency_spike_ratio, 2)}x`
  }
}

const siteTitle = computed(() => {
  const name = String(settings.value.site_name || '').trim()
  return name || 'RIVO MONITOR'
})

const siteDescription = computed(() => {
  return String(settings.value.site_description || '').trim() || 'RIVO · 生产集群 · 实时遥测面板'
})

const cyberNodes = computed(() => rawNodes.value.map(mapNode))

const filteredNodes = computed(() => {
  return cyberNodes.value.filter(node => filter.value === 'all' || node.status === filter.value)
})

const onlineCount = computed(() => cyberNodes.value.filter(node => node.status === 'online').length)
const warningCount = computed(() => cyberNodes.value.filter(node => node.status === 'warning').length)
const offlineCount = computed(() => cyberNodes.value.filter(node => node.status === 'offline').length)
const metricNodes = computed(() => cyberNodes.value.filter(node => node.hasMetric && node.status !== 'offline'))
const avgCPU = computed(() => average(metricNodes.value.map(node => node.cpu)))
const avgMemory = computed(() => average(metricNodes.value.map(node => node.memory)))
const avgDisk = computed(() => average(cyberNodes.value.filter(node => node.hasMetric).map(node => node.disk)))
const totalRx = computed(() => metricNodes.value.reduce((sum, node) => sum + node.rx, 0))
const totalTx = computed(() => metricNodes.value.reduce((sum, node) => sum + node.tx, 0))
const totalNetwork = computed(() => totalRx.value + totalTx.value)
const networkValue = computed(() => formatBps(totalNetwork.value))
const lineOrbColors = ['#2cf6ff', '#2fff96', '#ffe66d', '#8d5cff', '#ff2bd6', '#ff8a3d', '#6ad7ff', '#ffffff']
const lineOrbSlots = [
  { x: 23, y: 34 },
  { x: 77, y: 30 },
  { x: 79, y: 70 },
  { x: 24, y: 72 },
  { x: 50, y: 18 },
  { x: 50, y: 82 },
  { x: 14, y: 52 },
  { x: 86, y: 52 }
]
const lineDistribution = computed(() => {
  const counts = new Map<string, number>()
  for (const node of cyberNodes.value) {
    for (const line of node.networkLines) {
      counts.set(line, (counts.get(line) || 0) + 1)
    }
  }
  const total = cyberNodes.value.length || 1
  return [...counts.entries()]
    .sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
    .map(([name, count], index, source) => {
      const fallbackAngle = -Math.PI * 0.78 + index / Math.max(1, source.length) * Math.PI * 2
      const fallback = {
        x: 50 + Math.cos(fallbackAngle) * 36,
        y: 50 + Math.sin(fallbackAngle) * 31
      }
      const slot = lineOrbSlots[index] || fallback
      const x = Math.max(12, Math.min(88, slot.x))
      const y = Math.max(15, Math.min(85, slot.y))
      return {
        name,
        count,
        percent: Math.round(count / total * 100),
        color: lineOrbColors[index % lineOrbColors.length],
        x,
        y,
        xPos: `${x}%`,
        yPos: `${y}%`
      }
    })
})
const availability = computed(() => {
  if (Number.isFinite(Number(summary.value?.availability_percent))) return Number(summary.value?.availability_percent)
  return cyberNodes.value.length ? onlineCount.value / cyberNodes.value.length * 100 : 0
})
const fallbackHealthScore = computed(() => {
  if (!cyberNodes.value.length) return 0
  return Math.round(clamp(
    availability.value -
    warningCount.value * 5 -
    offlineCount.value * 14 -
    asNumber(summary.value?.current_alerts) * 6 -
    Math.max(0, avgCPU.value - 80) * 0.25 -
    Math.max(0, avgMemory.value - 85) * 0.2
  ))
})
const backendHealthScore = computed<number | null>(() => {
  if (Number.isFinite(Number(summary.value?.cluster_health_score))) return Math.round(Number(summary.value?.cluster_health_score))
  const scores = Object.values(summary.value?.node_health_scores || {})
    .map(item => Number(item.score))
    .filter(score => Number.isFinite(score))
  if (!scores.length) return null
  return Math.round(average(scores))
})
const healthScore = computed(() => {
  if (backendHealthScore.value !== null) return backendHealthScore.value
  return fallbackHealthScore.value
})
const healthScoreSource = computed(() => backendHealthScore.value === null ? '本地估算' : '智能算法')
const healthGradeLabel = computed(() => {
  if (!cyberNodes.value.length) return '--'
  if (healthScore.value >= 95) return '优秀'
  if (healthScore.value >= 85) return '健康'
  if (healthScore.value >= 70) return '关注'
  if (healthScore.value >= 50) return '风险'
  return '严重'
})
const activeAlertLabel = computed(() => `${asNumber(summary.value?.current_alerts)} 条`)
const uptimeRange = computed(() => {
  const values = metricNodes.value
    .map(node => node.uptimeSeconds)
    .filter(value => Number.isFinite(value) && value > 0)
  if (!values.length) return { max: '--', min: '--' }
  return {
    max: formatUptime(Math.max(...values)),
    min: formatUptime(Math.min(...values))
  }
})
const lastUpdatedLabel = computed(() => lastLoadedAt.value ? formatTime(lastLoadedAt.value) : '--')
const serverNow = computed(() => now.value + serverClockDeltaMS.value)
const clockLabel = computed(() => formatServerClock(serverNow.value))
const serverDateLabel = computed(() => formatServerDate(serverNow.value))
const serverClockLabel = computed(() => {
  return `UTC +0.00 ${clockLabel.value}`
})
const noResultText = computed(() => rawNodes.value.length ? '未找到匹配节点' : '暂无节点数据，等待 Agent 接入')

const metricCards = computed(() => [
  {
    label: '平均 CPU',
    amount: formatNumber(avgCPU.value, 1),
    unit: '%',
    meter: avgCPU.value,
    note: metricNodes.value.length ? `${metricNodes.value.length} 个在线节点参与统计` : '等待节点上报',
    icon: 'CPU',
    spark: 'M3 36 C18 20, 30 32, 42 18 S62 8, 76 22 S98 36, 117 10',
    tone: 'cyan'
  },
  {
    label: '内存占用',
    amount: formatNumber(avgMemory.value, 1),
    unit: '%',
    meter: avgMemory.value,
    note: metricNodes.value.length ? `${formatBytes(metricNodes.value.reduce((sum, node) => sum + node.memoryUsed, 0))} / ${formatBytes(metricNodes.value.reduce((sum, node) => sum + node.memoryTotal, 0))}` : '等待节点上报',
    icon: 'RAM',
    spark: 'M3 30 C18 34, 27 13, 42 22 S65 38, 80 18 S98 8, 117 28',
    tone: 'pink'
  },
  {
    label: '磁盘占用',
    amount: formatNumber(avgDisk.value, 1),
    unit: '%',
    meter: avgDisk.value,
    note: cyberNodes.value.some(node => node.hasMetric) ? `${formatBytes(cyberNodes.value.reduce((sum, node) => sum + node.diskUsed, 0))} / ${formatBytes(cyberNodes.value.reduce((sum, node) => sum + node.diskTotal, 0))}` : '等待节点上报',
    icon: 'SSD',
    spark: 'M3 25 C20 14, 25 16, 40 28 S63 36, 72 18 S96 14, 117 24',
    tone: 'cyan'
  },
  {
    label: '网络吞吐',
    amount: networkValue.value.amount,
    unit: networkValue.value.unit,
    meter: totalNetwork.value / 10000000,
    note: metricNodes.value.length ? `入口 ${formatBps(totalRx.value).label} · 出口 ${formatBps(totalTx.value).label}` : '等待节点上报',
    icon: 'NET',
    spark: 'M3 39 C18 9, 30 13, 43 30 S63 36, 75 8 S96 10, 117 31',
    tone: 'pink'
  }
])

function dashboardEventMessage(event: DashboardEvent) {
  if (event.message) return event.message
  if (event.event_type === 'metric.reported' && event.metric) {
    return `上报指标，CPU ${formatNumber(event.metric.cpu_usage, 1)}% · MEM ${formatNumber(event.metric.mem_used_percent, 1)}%。`
  }
  if (event.event_type === 'agent.online') return 'Agent 已上线，心跳链路已建立。'
  if (event.event_type === 'agent.offline') return 'Agent 已离线，心跳链路断开。'
  if (event.event_type === 'alert.triggered') return '告警触发，等待处理。'
  return '系统事件已记录。'
}

const feedItems = computed<FeedItem[]>(() => {
  const events = dashboardEvents.value
    .filter(event => normalizeTimestamp(event.created_at) > 0)
    .map(event => ({
      time: formatTime(event.created_at),
      nodeName: event.node_name || event.node_id,
      text: dashboardEventMessage(event)
    }))

  if (events.length) return events.slice(0, 8)

  const entries: Array<FeedItem & { stamp: number }> = []
  rawNodes.value.forEach(node => {
    const metric = node.latest_metric
    const nodeName = node.name || node.node_id
    if (metric) {
      entries.push({
        stamp: normalizeTimestamp(metric.ts),
        time: formatTime(metric.ts),
        nodeName,
        text: `最近一次指标上报，CPU ${formatNumber(metric.cpu_usage, 1)}% · MEM ${formatNumber(metric.mem_used_percent, 1)}%。`
      })
    }
    if (node.status === 'offline') {
      entries.push({
        stamp: normalizeTimestamp(node.last_seen_at),
        time: formatTime(node.last_seen_at),
        nodeName,
        text: '当前状态离线，等待下一次心跳。'
      })
    }
  })

  if (!entries.length) {
    return [{
      time: formatTime(now.value),
      text: rawNodes.value.length ? '暂无最近事件，等待下一条 Agent 遥测。' : '暂无节点接入，等待第一条 Agent 遥测。'
    }]
  }

  return entries
    .sort((a, b) => b.stamp - a.stamp)
    .slice(0, 8)
    .map(item => ({ time: item.time, nodeName: item.nodeName, text: item.text }))
})

function normalizeDateTime(value?: string | number | null) {
  if (typeof value === 'number') return normalizeTimestamp(value)
  const stamp = Date.parse(String(value || ''))
  return Number.isFinite(stamp) ? stamp : 0
}

function roundTrendValue(value: unknown, digits = 1) {
  const scale = 10 ** digits
  return Math.round(asNumber(value) * scale) / scale
}

function formatLatencyValue(value?: number | null) {
  const number = Number(value)
  return Number.isFinite(number) ? `${formatNumber(number, 1)} ms` : '--'
}

function formatTrendPercent(value?: number | null) {
  const number = Number(value)
  return Number.isFinite(number) ? `${formatNumber(number, 1)}%` : '--'
}

function chartAxisStyle() {
  return {
    axisLine: { lineStyle: { color: 'rgba(44, 246, 255, .24)' } },
    axisTick: { lineStyle: { color: 'rgba(44, 246, 255, .2)' } },
    axisLabel: { color: '#86a9b8', fontFamily: 'inherit', fontSize: 11 },
    splitLine: { lineStyle: { color: 'rgba(44, 246, 255, .1)', type: 'dashed' as const } }
  }
}

function chartTooltipFormatter(params: unknown, unit = '') {
  const items = Array.isArray(params) ? params as Array<{ marker: string; seriesName: string; value: [number, number | null] }> : []
  const valid = items.filter(item => item.value && item.value[1] !== null && Number.isFinite(Number(item.value[1])))
  if (!valid.length) return ''
  const time = formatRangeWindowTime(valid[0].value[0])
  const lines = valid.map(item => `${item.marker}${item.seriesName}: ${formatNumber(Number(item.value[1]), unit === 'x' ? 2 : 1)}${unit}`)
  return `<div class="chart-tooltip"><b>${time}</b>${lines.map(line => `<span>${line}</span>`).join('')}</div>`
}

function rangeHours(value: TrendRange) {
  return trendRangeOptions.find(item => item.value === value)?.hours || 24
}

function rangeAxisLabelIntervalMS(value: TrendRange) {
  switch (value) {
    case '1h':
      return 10 * minuteMS
    case '4h':
      return 30 * minuteMS
    case '12h':
      return hourMS
    case '1d':
      return 4 * hourMS
    case '7d':
      return dayMS
    case '1m':
      return 3 * dayMS
    case '3m':
      return 10 * dayMS
    case '6m':
      return 20 * dayMS
    case '1y':
      return 60 * dayMS
    default:
      return hourMS
  }
}

function rangeAxisBounds(value: TrendRange, anchor = Date.now()) {
  const interval = rangeAxisLabelIntervalMS(value)
  const span = rangeHours(value) * hourMS
  const intervalCount = Math.max(1, Math.ceil(span / interval))
  const max = anchor
  return {
    min: max - intervalCount * interval,
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

function rangeAxisLabelStride(range: TrendRange, anchor: number, width?: number | null) {
  const bounds = rangeAxisBounds(range, anchor)
  const interval = rangeAxisLabelIntervalMS(range)
  const tickCount = Math.max(1, Math.round((bounds.max - bounds.min) / interval))
  return Math.max(1, Math.ceil(tickCount / chartLabelTargetCount(width)))
}

function shouldShowRangeAxisLabel(timestamp: number, range: TrendRange, anchor: number, width?: number | null) {
  if (!Number.isFinite(timestamp)) return false
  const bounds = rangeAxisBounds(range, anchor)
  const interval = rangeAxisLabelIntervalMS(range)
  const edgeTolerance = Math.max(1, interval / 2)
  if (Math.abs(timestamp - bounds.min) <= edgeTolerance || Math.abs(timestamp - bounds.max) <= edgeTolerance) return true
  const tickIndex = Math.round((timestamp - bounds.min) / interval)
  return tickIndex >= 0 && tickIndex % rangeAxisLabelStride(range, anchor, width) === 0
}

function formatRangeAxisLabel(timestamp: number, range: TrendRange, anchor: number, width?: number | null) {
  if (!Number.isFinite(timestamp)) return ''
  if (!shouldShowRangeAxisLabel(timestamp, range, anchor, width)) return ''
  const date = new Date(timestamp)
  const year = String(date.getFullYear()).slice(2)
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  if (range === '1h' || range === '4h' || range === '12h' || range === '1d') return `${hour}:${minute}`
  if (range === '7d' || range === '1m' || range === '3m' || range === '6m') return `${month}/${day}`
  return `${year}/${month}`
}

function rangeWindowLabel(range: TrendRange, anchor = Date.now()) {
  const bounds = rangeAxisBounds(range, anchor)
  return `${formatRangeWindowTime(bounds.min)} - ${formatRangeWindowTime(bounds.max)}`
}

function trendXAxisOption(range: TrendRange, anchor: number, width?: number | null) {
  const bounds = rangeAxisBounds(range, anchor)
  const interval = rangeAxisLabelIntervalMS(range)
  return {
    type: 'value' as const,
    min: bounds.min,
    max: bounds.max,
    interval,
    minInterval: interval,
    maxInterval: interval,
    ...chartAxisStyle(),
    axisLabel: {
      color: '#86a9b8',
      fontFamily: 'inherit',
      fontSize: 11,
      hideOverlap: false,
      showMinLabel: true,
      showMaxLabel: true,
      formatter: (raw: number | string) => formatRangeAxisLabel(Number(raw), range, anchor, width)
    }
  }
}

function emptyChartGraphic(text: string) {
  return {
    type: 'text',
    left: 'center',
    top: 'middle',
    style: {
      text,
      fill: '#86a9b8',
      fontSize: 13,
      fontWeight: 700,
      fontFamily: 'inherit'
    }
  }
}

function trendLineSeries(name: string, color: string, data: Array<[number, number | null]>, yAxisIndex = 0) {
  return {
    name,
    type: 'line',
    smooth: true,
    showSymbol: false,
    connectNulls: false,
    yAxisIndex,
    data,
    lineStyle: {
      width: 2,
      color,
      shadowColor: color,
      shadowBlur: 12
    },
    areaStyle: {
      color: {
        type: 'linear',
        x: 0,
        y: 0,
        x2: 0,
        y2: 1,
        colorStops: [
          { offset: 0, color: `${color}44` },
          { offset: 1, color: `${color}05` }
        ]
      }
    },
    emphasis: {
      focus: 'series'
    }
  }
}

const trendModalTitle = computed(() => {
  const nodeName = trendModalNode.value?.name || '--'
  return trendModalKind.value === 'hardware' ? `${nodeName} · 硬件趋势` : `${nodeName} · Ping 趋势`
})

const trendRangeLabel = computed(() => {
  return trendRangeOptions.find(item => item.value === trendRange.value)?.summary || '最近 1 天'
})

const trendModalSubtitle = computed(() => {
  const windowText = rangeWindowLabel(trendRange.value, trendRangeAnchor.value)
  return trendModalKind.value === 'hardware'
    ? `${trendRangeLabel.value} · ${windowText} · CPU / 内存 / 磁盘 / 负载`
    : `${trendRangeLabel.value} · ${windowText} · Ping 延迟与探测结果`
})

const hardwareTrendStats = computed(() => {
  const latest = trendMetrics.value[trendMetrics.value.length - 1]
  return [
    { label: 'CPU', value: formatTrendPercent(latest?.cpu_usage) },
    { label: '内存', value: formatTrendPercent(latest?.mem_used_percent) },
    { label: '磁盘', value: formatTrendPercent(latest?.disk_used_percent) },
    { label: 'Load1', value: latest ? formatNumber(latest.load1, 2) : '--' }
  ]
})

const pingTrendStats = computed(() => {
  const results = trendProbe.value?.results || []
  const totals = results.reduce((acc, item) => {
    const samples = Math.max(1, asNumber(item.samples, 1))
    const success = asNumber(item.success_samples, item.status === 'success' ? 1 : 0)
    const failed = asNumber(item.failed_samples, item.status === 'success' ? 0 : 1)
    if (item.status === 'success' && Number.isFinite(Number(item.latency_ms))) {
      acc.latencyTotal += Number(item.latency_ms) * Math.max(1, success)
      acc.latencySamples += Math.max(1, success)
      acc.latestLatency = Number(item.latency_ms)
    }
    acc.samples += samples
    acc.success += success
    acc.failed += failed
    return acc
  }, { samples: 0, success: 0, failed: 0, latencyTotal: 0, latencySamples: 0, latestLatency: null as number | null })
  const successRate = totals.samples > 0 ? totals.success / totals.samples * 100 : null
  const lossRate = totals.samples > 0 ? totals.failed / totals.samples * 100 : null
  const averageLatency = totals.latencySamples > 0 ? totals.latencyTotal / totals.latencySamples : null
  return [
    { label: '最新延迟', value: formatLatencyValue(totals.latestLatency) },
    { label: '平均延迟', value: formatLatencyValue(averageLatency) },
    { label: '成功率', value: successRate === null ? '--' : `${formatNumber(successRate, 1)}%` },
    { label: '丢包率', value: lossRate === null ? '--' : `${formatNumber(lossRate, 1)}%` }
  ]
})

function probeTypeLabel(type?: string) {
  if (type === 'icmp') return 'ICMP'
  if (type === 'tcp_ping') return 'TCP Ping'
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

function probeGroupKey(result: ProbeResultOverview) {
  return result.task_id ? String(result.task_id) : `${result.target}|${result.type}|${result.ip_version || 'auto'}`
}

function probeSampleCount(item: ProbeResultOverview) {
  return Math.max(1, asNumber(item.samples, 1))
}

function probeSuccessCount(item: ProbeResultOverview) {
  if (Number.isFinite(Number(item.success_samples))) return Math.max(0, Number(item.success_samples))
  return item.status === 'success' && item.latency_ms !== null ? 1 : 0
}

function probeFailedCount(item: ProbeResultOverview) {
  if (Number.isFinite(Number(item.failed_samples))) return Math.max(0, Number(item.failed_samples))
  return Math.max(0, probeSampleCount(item) - probeSuccessCount(item))
}

function latencyJitter(values: number[]) {
  if (values.length < 2) return null
  let total = 0
  for (let index = 1; index < values.length; index += 1) {
    total += Math.abs(values[index] - values[index - 1])
  }
  return total / (values.length - 1)
}

function statFromProbeResults(base: Pick<PingNodeStat, 'key' | 'name' | 'target' | 'type' | 'ipVersion' | 'inactive'>, source: ProbeResultOverview[]): PingNodeStat {
  const sorted = [...source].sort((left, right) => normalizeDateTime(left.created_at) - normalizeDateTime(right.created_at))
  const latest = sorted[sorted.length - 1]
  let samples = 0
  let successSamples = 0
  let failedSamples = 0
  let latencyTotal = 0
  let minLatency: number | null = null
  let maxLatency: number | null = null
  const latencyValues: number[] = []

  for (const item of sorted) {
    const itemSamples = probeSampleCount(item)
    const itemSuccessSamples = probeSuccessCount(item)
    const itemFailedSamples = probeFailedCount(item)
    samples += itemSamples
    successSamples += itemSuccessSamples
    failedSamples += itemFailedSamples
    if (item.latency_ms !== null && itemSuccessSamples > 0) {
      latencyTotal += item.latency_ms * itemSuccessSamples
      latencyValues.push(item.latency_ms)
      const itemMin = item.min_latency_ms ?? item.latency_ms
      const itemMax = item.max_latency_ms ?? item.latency_ms
      minLatency = minLatency === null ? itemMin : Math.min(minLatency, itemMin)
      maxLatency = maxLatency === null ? itemMax : Math.max(maxLatency, itemMax)
    }
  }

  return {
    ...base,
    latestLatency: latest?.status === 'success' && latest.latency_ms !== null ? latest.latency_ms : null,
    latestStatus: latest?.status || 'unknown',
    averageLatency: successSamples > 0 ? latencyTotal / successSamples : null,
    packetLoss: samples > 0 ? failedSamples / samples * 100 : null,
    jitter: latencyJitter(latencyValues),
    samples,
    successSamples,
    failedSamples,
    minLatency,
    maxLatency
  }
}

function buildPingNodeStats(tasks: ProbeTask[], results: ProbeResultOverview[]) {
  const groups = new Map<string, ProbeResultOverview[]>()
  for (const result of results) {
    const key = probeGroupKey(result)
    const group = groups.get(key) || []
    group.push(result)
    groups.set(key, group)
  }

  const taskStats = tasks.map(task => statFromProbeResults({
    key: String(task.id),
    name: task.name || task.target || `Ping ${task.id}`,
    target: task.target || '--',
    type: task.type,
    ipVersion: task.ip_version || 'auto',
    inactive: !task.enabled
  }, groups.get(String(task.id)) || []))

  const knownKeys = new Set(taskStats.map(item => item.key))
  const orphanStats = [...groups.entries()]
    .filter(([key]) => !knownKeys.has(key))
    .map(([key, group]) => {
      const first = group[0]
      return statFromProbeResults({
        key,
        name: first.task_name || first.target || key,
        target: first.target || '--',
        type: first.type,
        ipVersion: first.ip_version || 'auto',
        inactive: true
      }, group)
    })

  return [...taskStats, ...orphanStats]
}

const pingNodeStats = computed(() => buildPingNodeStats(trendProbe.value?.tasks || [], trendProbe.value?.results || []))

function pingStatusLabel(item: PingNodeStat) {
  if (item.inactive) return '已停用'
  if (item.samples === 0) return '无数据'
  return item.latestStatus === 'success' ? '正常' : '失败'
}

function formatProbeLatency(value?: number | null) {
  return value === undefined || value === null ? 'N/A' : `${formatNumber(value, 1)} ms`
}

function formatProbePacketLoss(value?: number | null) {
  return value === undefined || value === null ? 'N/A' : `${formatNumber(value, 1)}%`
}

const trendHasData = computed(() => {
  return trendModalKind.value === 'hardware'
    ? trendMetrics.value.length > 0
    : (trendProbe.value?.results || []).length > 0
})

function activeTrendChartWidth() {
  return trendModalKind.value === 'hardware'
    ? hardwareChartEl.value?.clientWidth
    : pingChartEl.value?.clientWidth
}

function baseChartOption(unit: string): EChartsOption {
  return {
    backgroundColor: 'transparent',
    color: ['#2cf6ff', '#ff2bd6', '#ffe66d', '#2fff96', '#8d5cff'],
    animationDuration: 420,
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(7, 10, 24, .94)',
      borderColor: 'rgba(44, 246, 255, .34)',
      borderWidth: 1,
      padding: 10,
      textStyle: { color: '#dffaff', fontFamily: 'inherit' },
      extraCssText: 'box-shadow:0 0 28px rgba(44,246,255,.18);backdrop-filter:blur(12px);',
      formatter: (params: unknown) => chartTooltipFormatter(params, unit)
    },
    legend: {
      top: 4,
      right: 4,
      textStyle: { color: '#9ab8c8', fontSize: 11, fontWeight: 700 },
      itemWidth: 16,
      itemHeight: 8
    },
    grid: { left: 46, right: 44, top: 48, bottom: 36 },
    xAxis: trendXAxisOption(trendRange.value, trendRangeAnchor.value, activeTrendChartWidth()),
    yAxis: {
      type: 'value',
      ...chartAxisStyle()
    }
  }
}

function renderHardwareChart() {
  if (!hardwareChartEl.value || trendModalKind.value !== 'hardware') return
  const chart = hardwareChart || init(hardwareChartEl.value)
  hardwareChart = chart
  const source = [...trendMetrics.value].sort((left, right) => normalizeTimestamp(left.ts) - normalizeTimestamp(right.ts))
  const hasData = source.length > 0
  const option: EChartsOption = {
    ...baseChartOption('%'),
    yAxis: [
      { type: 'value', name: '%', min: 0, max: 100, nameTextStyle: { color: '#86a9b8' }, ...chartAxisStyle() },
      { type: 'value', name: 'load', min: 0, nameTextStyle: { color: '#86a9b8' }, ...chartAxisStyle() }
    ],
    series: [
      trendLineSeries('CPU', '#2cf6ff', source.map(item => [normalizeTimestamp(item.ts), roundTrendValue(item.cpu_usage)])),
      trendLineSeries('内存', '#ff2bd6', source.map(item => [normalizeTimestamp(item.ts), roundTrendValue(item.mem_used_percent)])),
      trendLineSeries('磁盘', '#ffe66d', source.map(item => [normalizeTimestamp(item.ts), roundTrendValue(item.disk_used_percent)])),
      trendLineSeries('Load1', '#2fff96', source.map(item => [normalizeTimestamp(item.ts), roundTrendValue(item.load1, 2)]), 1)
    ] as EChartsOption['series'],
    graphic: hasData ? undefined : emptyChartGraphic('暂无硬件趋势数据')
  }
  chart.setOption(option, true)
}

function renderPingChart() {
  if (!pingChartEl.value || trendModalKind.value !== 'ping') return
  const chart = pingChart || init(pingChartEl.value)
  pingChart = chart
  const results = [...(trendProbe.value?.results || [])].sort((left, right) => normalizeDateTime(left.created_at) - normalizeDateTime(right.created_at))
  const taskNames = new Map((trendProbe.value?.tasks || []).map(task => [task.id, task.name || task.target || `Task ${task.id}`]))
  const grouped = new Map<number, typeof results>()
  for (const result of results) {
    if (!grouped.has(result.task_id)) grouped.set(result.task_id, [])
    grouped.get(result.task_id)?.push(result)
  }
  const colors = ['#2cf6ff', '#ff2bd6', '#ffe66d', '#2fff96', '#8d5cff', '#ff8a3d']
  const series = [...grouped.entries()].map(([taskID, rows], index) => trendLineSeries(
    taskNames.get(taskID) || rows[0]?.task_name || `Ping ${index + 1}`,
    colors[index % colors.length],
    rows.map(item => [
      normalizeDateTime(item.created_at),
      item.status === 'success' && Number.isFinite(Number(item.latency_ms)) ? roundTrendValue(item.latency_ms) : null
    ])
  ))
  const hasSuccess = results.some(item => item.status === 'success' && Number.isFinite(Number(item.latency_ms)))
  const option: EChartsOption = {
    ...baseChartOption(' ms'),
    yAxis: { type: 'value', name: 'ms', min: 0, nameTextStyle: { color: '#86a9b8' }, ...chartAxisStyle() },
    series: series as EChartsOption['series'],
    graphic: hasSuccess ? undefined : emptyChartGraphic(results.length ? '暂无成功 Ping 样本' : '暂无 Ping 趋势数据')
  }
  chart.setOption(option, true)
}

function renderActiveTrendChart() {
  if (!trendModalOpen.value) return
  if (trendModalKind.value === 'hardware') {
    renderHardwareChart()
  } else {
    renderPingChart()
  }
}

function disposeTrendCharts() {
  hardwareChart?.dispose()
  pingChart?.dispose()
  hardwareChart = null
  pingChart = null
}

function resizeTrendCharts() {
  hardwareChart?.resize()
  pingChart?.resize()
  renderActiveTrendChart()
}

async function loadTrendData(node: CyberNode, kind: TrendModalKind, clearData = false) {
  const requestID = ++trendRequestID
  trendLoading.value = true
  trendError.value = ''
  if (clearData) {
    trendMetrics.value = []
    trendProbe.value = null
    trendRangeAnchor.value = Date.now()
  }

  const requestedRange = trendRange.value

  try {
    if (kind === 'hardware') {
      const metrics = await fetchTrendJSON<NodeMetric[]>(`/api/nodes/${encodeURIComponent(node.id)}/metrics?range=${requestedRange}`)
      if (requestID !== trendRequestID) return
      trendMetrics.value = Array.isArray(metrics) ? metrics : []
      trendRangeAnchor.value = Date.now()
    } else {
      const probe = await fetchTrendJSON<ProbeResultsResponse>(`/api/nodes/${encodeURIComponent(node.id)}/probe-results?range=${requestedRange}&include_inactive=1`)
      if (requestID !== trendRequestID) return
      trendProbe.value = probe
      trendRangeAnchor.value = normalizeTimestamp(probe.range_anchor || probe.generated_at || Date.now())
    }
  } catch (error) {
    if (requestID !== trendRequestID) return
    trendError.value = error instanceof Error && error.message !== 'Failed to fetch' ? error.message : '趋势数据加载失败，请稍后重试'
  } finally {
    if (requestID === trendRequestID) {
      trendLoading.value = false
      await nextTick()
      renderActiveTrendChart()
    }
  }
}

async function openTrendModal(node: CyberNode, kind: TrendModalKind) {
  trendModalNode.value = node
  trendModalKind.value = kind
  trendRange.value = '1h'
  trendModalOpen.value = true
  trendRangeAnchor.value = Date.now()
  disposeTrendCharts()
  await loadTrendData(node, kind, true)
}

async function setTrendRange(value: TrendRange) {
  if (trendRange.value === value && !trendError.value) return
  trendRange.value = value
  trendRangeAnchor.value = Date.now()
  await nextTick()
  renderActiveTrendChart()
  if (!trendModalNode.value || !trendModalOpen.value) return
  await loadTrendData(trendModalNode.value, trendModalKind.value)
}

function closeTrendModal() {
  trendRequestID++
  trendModalOpen.value = false
  trendLoading.value = false
  trendError.value = ''
  disposeTrendCharts()
  void loadDashboard(true)
}

function isExpanded(nodeID: string) {
  return expandedNodeIDs.value.has(nodeID)
}

function toggleNode(nodeID: string) {
  const next = new Set(expandedNodeIDs.value)
  if (next.has(nodeID)) {
    next.delete(nodeID)
  } else {
    next.add(nodeID)
  }
  expandedNodeIDs.value = next
}

async function loadDashboard(force = false) {
  if (trendModalOpen.value && !force) return
  if (loading.value && !force) return
  loading.value = true
  loadError.value = ''

  try {
    const [settingsResult, timeResult, summaryResult, nodesResult, eventsResult] = await Promise.allSettled([
      fetchJSON<AppSettings>('/api/settings'),
      fetchJSON<ServerTime>('/api/server-time'),
      fetchJSON<Summary>('/api/dashboard/summary'),
      fetchJSON<NodeRecord[]>('/api/nodes'),
      fetchJSON<{ events: DashboardEvent[] }>('/api/dashboard/events?limit=40')
    ])

    if (trendModalOpen.value && !force) return
    if (settingsResult.status === 'fulfilled') {
      settings.value = settingsResult.value
      document.title = `${settingsResult.value.site_name || 'Rivo'} · cyberpunk`
    }
    if (timeResult.status === 'fulfilled') {
      serverTime.value = timeResult.value
      serverClockDeltaMS.value = normalizeTimestamp(timeResult.value.unix_ms) - Date.now()
    }
    summary.value = summaryResult.status === 'fulfilled' ? summaryResult.value : null
    if (nodesResult.status !== 'fulfilled') throw nodesResult.reason
    rawNodes.value = Array.isArray(nodesResult.value) ? nodesResult.value : []
    dashboardEvents.value = eventsResult.status === 'fulfilled' && Array.isArray(eventsResult.value.events) ? eventsResult.value.events : []
    lastLoadedAt.value = Date.now()
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '请稍后重试'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  clockTimer = window.setInterval(() => {
    now.value = Date.now()
  }, 1000)
  refreshTimer = window.setInterval(() => {
    void loadDashboard()
  }, refreshIntervalMS)
  window.addEventListener('resize', resizeTrendCharts)
  void loadDashboard(true)
})

onBeforeUnmount(() => {
  if (clockTimer) window.clearInterval(clockTimer)
  if (refreshTimer) window.clearInterval(refreshTimer)
  window.removeEventListener('resize', resizeTrendCharts)
  disposeTrendCharts()
})

watch(trendModalOpen, async (open) => {
  if (!open) return
  await nextTick()
  renderActiveTrendChart()
})
</script>

<template>
  <div class="noise" />
  <main class="wrap">
    <header class="topbar">
      <section class="brand" aria-label="产品标题">
        <div class="brand-mark has-image" aria-hidden="true">
          <img src="/rivo-logo.png" alt="" draggable="false" />
        </div>
        <div>
          <h1>{{ siteTitle }}</h1>
          <p>{{ siteDescription }}</p>
        </div>
      </section>
      <nav class="top-actions" aria-label="状态摘要">
        <span class="pill ok"><span>在线</span><b>{{ onlineCount }}</b></span>
        <span class="pill warn"><span>告警</span><b>{{ warningCount }}</b></span>
        <span class="pill dead"><span>离线</span><b>{{ offlineCount }}</b></span>
        <span class="pill server-time">
          <span>服务器时间</span>
          <b class="clock-value" :data-date="serverDateLabel" :title="serverDateLabel">{{ serverClockLabel }}</b>
        </span>
      </nav>
    </header>

    <section class="hero" aria-label="总览">
      <article class="hero-panel">
        <div>
          <div class="eyebrow">Cyberpunk Probe Console</div>
          <h2 class="hero-title"><span class="glitch" data-text="VPS MATRIX">VPS MATRIX</span></h2>
        </div>
        <div class="hero-footer">
          <div class="mini-stat"><span>节点总数</span><strong>{{ cyberNodes.length }}</strong><em>已接入</em></div>
          <div class="mini-stat"><span>平均在线率</span><strong>{{ cyberNodes.length ? formatPercent(availability, 2) : '--' }}</strong><em>24 小时</em></div>
          <div class="mini-stat"><span>最大运行时长</span><strong>{{ uptimeRange.max }}</strong><em>最小 {{ uptimeRange.min }}</em></div>
        </div>
        <section class="metrics hero-metrics" aria-label="核心指标">
          <article v-for="card in metricCards" :key="card.label" class="metric">
            <div class="metric-top">
              <div>
                <div class="metric-label">{{ card.label }}</div>
                <div class="metric-value">{{ card.amount }}<small>{{ card.unit }}</small></div>
              </div>
              <div class="metric-icon">{{ card.icon }}</div>
            </div>
            <div class="meter"><span :style="meterStyle(card.meter)" /></div>
            <div class="metric-note">{{ card.note }}</div>
            <svg class="spark" :class="{ pink: card.tone === 'pink' }" viewBox="0 0 120 45" aria-hidden="true">
              <path :d="card.spark" />
            </svg>
          </article>
        </section>
      </article>

      <aside class="health-panel" aria-label="服务器健康度">
        <div class="panel-head">
          <h3 class="panel-title"><b>服务器健康度</b> / 实时评分</h3>
          <span class="live-dot">LIVE</span>
        </div>
        <div class="ring-wrap">
          <div class="ring" :style="{ '--p': healthScore }">
            <div class="ring-core"><strong>{{ healthScore }}</strong><span>HEALTH SCORE</span></div>
          </div>
        </div>
        <div class="health-list">
          <div class="health-row"><span>评分来源</span><b>{{ healthScoreSource }}</b></div>
          <div class="health-row"><span>健康等级</span><b>{{ healthGradeLabel }}</b></div>
          <div class="health-row"><span>活跃告警</span><b>{{ activeAlertLabel }}</b></div>
        </div>
      </aside>
    </section>

    <section class="content">
      <article class="terminal" aria-label="节点列表">
        <div class="terminal-head">
          <div class="terminal-path">
            <div class="window-dots" aria-hidden="true"><i /><i /><i /></div>
            <span class="terminal-title">/var/monitor/nodes.stream</span>
          </div>
          <div class="toolbar">
            <button
              v-for="option in filterOptions"
              :key="option.value"
              class="chip"
              :class="{ active: filter === option.value }"
              type="button"
              @click="filter = option.value"
            >
              {{ option.label }}
            </button>
          </div>
        </div>

        <div class="node-list">
          <article v-for="(node, index) in filteredNodes" :key="node.id" class="node-card" :class="{ open: isExpanded(node.id) }">
            <div class="node-main">
              <div class="node-id">
                <div class="node-glyph" aria-hidden="true">
                  <svg viewBox="0 0 24 24" width="19" height="19" fill="none">
                    <path d="M5 6.5h14v4H5zM5 13.5h14v4H5z" stroke="currentColor" stroke-width="1.8" />
                    <path d="M8 8.5h.01M8 15.5h.01" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
                  </svg>
                </div>
                <div class="node-copy">
                  <div class="node-name-row">
                    <div class="node-name">{{ node.name }}</div>
                    <button class="trend-icon-button" type="button" title="硬件趋势图" :aria-label="`${node.name} 硬件趋势图`" @click.stop="openTrendModal(node, 'hardware')">
                      <svg viewBox="0 0 24 24" aria-hidden="true">
                        <path d="M4 18V6M4 18h16M8 15l3-4 3 2 4-6" />
                        <path d="M8 19v-4M12 19v-8M16 19v-6M20 19V7" />
                      </svg>
                    </button>
                  </div>
                  <div class="node-loc">{{ node.nodeSummary }}</div>
                </div>
              </div>
              <div><span class="cell-label">状态</span><span class="status" :class="node.status">{{ node.status === 'online' ? '在线' : node.status === 'warning' ? '告警' : '离线' }}</span></div>
              <div><span class="cell-label">CPU</span><span class="cell-value">{{ formatPercent(node.cpu, 1) }}</span><div class="meter"><span :style="meterStyle(node.cpu)" /></div></div>
              <div><span class="cell-label">内存</span><span class="cell-value">{{ formatPercent(node.memory, 1) }}</span><div class="meter"><span :style="meterStyle(node.memory)" /></div></div>
              <div class="hide-md"><span class="cell-label">磁盘</span><span class="cell-value">{{ formatPercent(node.disk, 1) }}</span><div class="meter"><span :style="meterStyle(node.disk)" /></div></div>
              <div class="hide-md latency-cell">
                <span class="cell-label">延迟</span>
                <span class="cell-value" :class="node.latencyClass">{{ node.latency }}</span>
                <button class="trend-icon-button latency-trend-button" type="button" title="Ping 趋势图" :aria-label="`${node.name} Ping 趋势图`" @click.stop="openTrendModal(node, 'ping')">
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M3 12h3l2-5 4 10 3-7 2 2h4" />
                    <path d="M4 20h16" />
                  </svg>
                </button>
              </div>
              <button class="expand" type="button" :aria-label="`展开 ${node.name}`" @click="toggleNode(node.id)"><span aria-hidden="true" /></button>
            </div>
            <div class="node-detail">
              <div class="detail-grid">
                <div class="detail-box"><span>运营商 / 线路</span><b>{{ node.provider }} / {{ node.networkLine }}</b></div>
                <div class="detail-box"><span>系统负载</span><b>{{ node.load }}</b></div>
                <div class="detail-box"><span>24 小时可用率</span><b>{{ node.availability }}</b></div>
                <div class="detail-box"><span>实时吞吐</span><b>{{ node.throughput }}</b></div>
                <div class="detail-box traffic-detail">
                  <div class="traffic-detail-head">
                    <span>套餐流量</span>
                    <b>{{ node.trafficUsagePercent }}%</b>
                  </div>
                  <div class="traffic-detail-line">
                    <strong>{{ node.trafficUsed }} / {{ node.trafficLimit }}</strong>
                    <em>剩余 {{ node.trafficRemaining }}</em>
                  </div>
                  <div class="traffic-detail-track"><i :style="trafficMeterStyle(node.trafficUsagePercent)" /></div>
                  <div class="traffic-detail-meta">
                    <span>{{ node.trafficResetCycle }}</span>
                    <span>{{ node.trafficBillingDirection }}</span>
                  </div>
                </div>
                <div class="detail-box"><span>平均延迟</span><b>{{ node.latency }}</b></div>
                <div class="detail-box"><span>健康总分 / 100</span><b>{{ node.healthScore === null ? '--' : formatNumber(node.healthScore, 1) }}</b></div>
                <div class="detail-box"><span>指标新鲜度 / 10</span><b>{{ node.freshnessScore === null ? '--' : formatNumber(node.freshnessScore, 1) }}</b></div>
                <div class="detail-box"><span>硬件资源 / 30</span><b>{{ node.resourceScore === null ? '--' : formatNumber(node.resourceScore, 1) }}</b></div>
                <div class="detail-box"><span>系统负载 / 20</span><b>{{ node.loadScore === null ? '--' : formatNumber(node.loadScore, 1) }}</b></div>
                <div class="detail-box"><span>网络质量 / 35</span><b>{{ node.networkScore === null ? '--' : formatNumber(node.networkScore, 1) }}</b></div>
                <div class="detail-box"><span>稳定性 / 5</span><b>{{ node.stabilityScore === null ? '--' : formatNumber(node.stabilityScore, 1) }}</b></div>
                <div class="detail-box"><span>网络抖动</span><b>{{ node.jitter }}</b></div>
                <div class="detail-box"><span>延迟突变</span><b>{{ node.latencySpike }}</b></div>
              </div>
            </div>
          </article>
        </div>
        <div v-if="loadError" class="no-result visible">数据加载失败：{{ loadError }}</div>
        <div v-else-if="filteredNodes.length === 0" class="no-result visible">{{ noResultText }}</div>
      </article>

      <aside class="side">
        <article class="map-panel">
          <div class="panel-head"><h3 class="panel-title"><b>线路分布</b> / 节点线路</h3><span class="pill">LINE</span></div>
          <div class="line-orbit-map" :class="{ empty: lineDistribution.length === 0 }">
            <svg class="line-links" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
              <line
                v-for="line in lineDistribution"
                :key="`link-${line.name}`"
                :x1="line.x"
                :y1="line.y"
                x2="50"
                y2="50"
                :style="{ '--c': line.color }"
              />
            </svg>
            <span class="line-hub" aria-hidden="true" />
            <div v-if="lineDistribution.length === 0" class="line-empty">暂无线路数据</div>
            <div
              v-for="line in lineDistribution"
              :key="line.name"
              class="line-orb"
              :style="{ '--x': line.xPos, '--y': line.yPos, '--c': line.color }"
            >
              <i aria-hidden="true" />
              <b>{{ line.name }}</b>
              <em>{{ line.count }} 台</em>
            </div>
          </div>
        </article>
        <article class="map-panel feed">
          <div class="panel-head"><h3 class="panel-title"><b>事件流</b> / 最近日志</h3><span class="pill warn">{{ feedItems.length }} 条</span></div>
          <div class="feed-list">
            <div v-for="item in feedItems" :key="`${item.time}-${item.nodeName || item.text}`" class="feed-item">
              <div class="feed-time">{{ item.time }}</div>
              <div class="feed-text"><b v-if="item.nodeName">{{ item.nodeName }}</b>{{ item.nodeName ? ' ' : '' }}{{ item.text }}</div>
            </div>
          </div>
        </article>
      </aside>
    </section>

    <footer class="footer">
      <span>CYBERPUNK THEME v1.0.0 · RIVO LIVE DATA</span>
      <span>自动刷新 15s · 当前视图 <b>{{ filteredNodes.length }}</b> 个节点 · 最近更新 <b>{{ lastUpdatedLabel }}</b></span>
    </footer>

    <div v-if="trendModalOpen" class="trend-modal-backdrop" role="presentation" @click.self="closeTrendModal">
      <section class="trend-modal" role="dialog" aria-modal="true" :aria-label="trendModalTitle">
        <header class="trend-modal-head">
          <div>
            <h3>{{ trendModalTitle }}</h3>
            <span>{{ trendModalSubtitle }}</span>
          </div>
          <div class="trend-modal-actions">
            <button class="modal-icon-button" type="button" :disabled="trendLoading || !trendModalNode" title="刷新趋势数据" @click="trendModalNode && openTrendModal(trendModalNode, trendModalKind)">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M20 12a8 8 0 1 1-2.34-5.66M20 4v5h-5" />
              </svg>
            </button>
            <button class="modal-icon-button" type="button" title="关闭" @click="closeTrendModal">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M6 6l12 12M18 6 6 18" />
              </svg>
            </button>
          </div>
        </header>

        <div class="trend-range-switch" aria-label="趋势时间维度">
          <button
            v-for="item in trendRangeOptions"
            :key="item.value"
            type="button"
            :class="{ active: trendRange === item.value }"
            :disabled="trendLoading"
            @click="setTrendRange(item.value)"
          >
            {{ item.label }}
          </button>
        </div>

        <div class="trend-summary-grid">
          <div v-for="item in trendModalKind === 'hardware' ? hardwareTrendStats : pingTrendStats" :key="item.label">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
          </div>
        </div>

        <div class="trend-chart-shell">
          <div v-show="trendModalKind === 'hardware'" ref="hardwareChartEl" class="trend-chart-canvas" />
          <div v-show="trendModalKind === 'ping'" ref="pingChartEl" class="trend-chart-canvas" />
          <div v-if="trendLoading" class="trend-chart-overlay">
            <span class="loading-orb"></span>
            <strong>Loading...</strong>
          </div>
          <div v-if="trendError && !trendHasData" class="trend-error">{{ trendError }}</div>
        </div>

        <section v-if="trendModalKind === 'ping'" class="ping-node-section" aria-label="Ping 节点信息">
          <div class="ping-node-card" v-for="item in pingNodeStats" :key="item.key">
            <div class="ping-node-title">
              <div>
                <strong>{{ item.name }}</strong>
                <span>{{ item.target }} · {{ probeModeLabel(item.type, item.ipVersion) }}</span>
              </div>
              <b :class="{ failed: item.latestStatus !== 'success' && item.samples > 0 && !item.inactive, inactive: item.inactive }">
                {{ pingStatusLabel(item) }}
              </b>
            </div>
            <div class="ping-node-main">
              <span>最新延迟</span>
              <strong>{{ item.latestStatus === 'success' ? formatProbeLatency(item.latestLatency) : item.samples ? '失败' : 'N/A' }}</strong>
            </div>
            <div class="ping-node-metrics">
              <div><span>均值</span><strong>{{ formatProbeLatency(item.averageLatency) }}</strong></div>
              <div><span>丢包率</span><strong>{{ formatProbePacketLoss(item.packetLoss) }}</strong></div>
              <div><span>抖动</span><strong>{{ formatProbeLatency(item.jitter) }}</strong></div>
              <div><span>成功/总样本</span><strong>{{ item.successSamples }} / {{ item.samples }}</strong></div>
              <div><span>最低</span><strong>{{ formatProbeLatency(item.minLatency) }}</strong></div>
              <div><span>最高</span><strong>{{ formatProbeLatency(item.maxLatency) }}</strong></div>
            </div>
          </div>
          <div v-if="pingNodeStats.length === 0" class="ping-node-empty">暂无 Ping 节点数据</div>
        </section>
      </section>
    </div>
  </main>
</template>
