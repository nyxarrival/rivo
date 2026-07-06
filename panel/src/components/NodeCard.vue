<script setup lang="ts">
import { computed } from 'vue'
import type { AppSettings, DashboardNodeProbeStat, DashboardSparklinePoint, NodeMetric, NodeRecord, TrendKind } from '../types'

type RegionClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

const props = defineProps<{
  node: NodeRecord
  selected: boolean
  appSettings: AppSettings
  sparkline: DashboardSparklinePoint[]
  probeStat?: DashboardNodeProbeStat
  nodeLabel: (node: NodeRecord) => string
  nodeHoverTags: (node: NodeRecord) => string[]
  nodeIpSummary: (node: NodeRecord) => string
  displayRegion: (region?: string | null) => string
  regionFlagClass: (region?: string | null) => RegionClassValue
  regionFlagStyle: (region?: string | null) => Record<string, string> | undefined
  regionFlagLabel: (region?: string | null) => string
  isNodeOnline: (node: NodeRecord) => boolean
  liveMetric: (node: NodeRecord) => NodeMetric | null
  formatPercent: (value?: number | null) => string
  formatBps: (value?: number | null) => string
  formatMegabytes: (value?: number | null) => string
  formatBytes: (value?: number | null) => string
  formatCpuCores: (metric?: NodeMetric | null) => string
  formatLoad: (metric?: NodeMetric | null) => string
  formatLoadTitle: (metric?: NodeMetric | null) => string
  formatNodeUptime: (node: NodeRecord) => string
  formatOsDisplay: (metric?: NodeMetric | null) => { key: string; icon: string; label: string }
  formatOsName: (metric?: NodeMetric | null) => string
  trafficUsagePercent: (node: NodeRecord) => number
  trafficRemainingLine: (node: NodeRecord) => string
  packageBillingSummary: (node: NodeRecord) => string
  remainingPackageSummary: (node: NodeRecord) => string
  remainingPackageTitle: (node: NodeRecord) => string
}>()

const emit = defineEmits<{
  (event: 'select', nodeID: string): void
  (event: 'trend', node: NodeRecord, kind: TrendKind, mouseEvent: MouseEvent): void
  (event: 'move-trend', mouseEvent: MouseEvent): void
  (event: 'leave-trend'): void
  (event: 'open-probe', node: NodeRecord): void
  (event: 'open-metrics', node: NodeRecord): void
}>()

const metric = computed(() => props.liveMetric(props.node))
const statusTone = computed(() => {
  if (!props.isNodeOnline(props.node)) return 'down'
  if (resourceValue('cpu') >= 75 || resourceValue('memory') >= 80 || resourceValue('disk') >= 85) return 'warn'
  if ((props.probeStat?.availability_percent ?? 100) < 96) return 'warn'
  if (props.node.service_expires_at && props.node.remaining_days >= 0 && props.node.remaining_days <= 7) return 'warn'
  return 'ok'
})
const statusLabel = computed(() => statusTone.value === 'down' ? '离线' : statusTone.value === 'warn' ? '告警' : '在线')
const accentStyle = computed(() => {
  if (statusTone.value === 'down') return { '--accent': 'rgba(255, 107, 134, .42)' }
  if (statusTone.value === 'warn') return { '--accent': 'rgba(255, 211, 107, .42)' }
  return { '--accent': 'rgba(98, 243, 174, .38)' }
})
const pingLabel = computed(() => {
  const latency = props.probeStat?.avg_latency_ms
  return typeof latency === 'number' && Number.isFinite(latency) ? `${Math.round(latency)}ms` : 'N/A'
})
const osDisplay = computed(() => props.formatOsDisplay(props.node.latest_metric))
const regionLabel = computed(() => props.displayRegion(props.node.region))
const cardTags = computed(() => props.nodeHoverTags(props.node))
const compactUptime = computed(() => {
  if (!props.isNodeOnline(props.node)) return 'OFF'
  const seconds = props.node.latest_metric?.uptime_seconds
  if (!seconds || seconds <= 0) return props.formatNodeUptime(props.node)
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  return `${days}D ${hours}H`
})
const nodeMetaTitle = computed(() => {
  const parts = [
    regionLabel.value,
    osDisplay.value.label
  ].filter((part) => part && part !== '未知')
  return parts.length ? parts.join(' · ') : props.formatOsName(props.node.latest_metric)
})
const alertReasons = computed(() => {
  if (statusTone.value === 'down') return ['节点离线']
  if (statusTone.value !== 'warn') return []

  const reasons: string[] = []
  const cpuUsage = resourceValue('cpu')
  const memoryUsage = resourceValue('memory')
  const diskUsage = resourceValue('disk')
  const availability = props.probeStat?.availability_percent

  if (cpuUsage >= 75) reasons.push(`CPU 使用率 ${props.formatPercent(cpuUsage)}`)
  if (memoryUsage >= 80) reasons.push(`内存使用率 ${props.formatPercent(memoryUsage)}`)
  if (diskUsage >= 85) reasons.push(`磁盘使用率 ${props.formatPercent(diskUsage)}`)
  if (typeof availability === 'number' && Number.isFinite(availability) && availability < 96) {
    reasons.push(`Ping 可用率 ${Math.round(availability)}%`)
  }
  if (props.node.service_expires_at && props.node.remaining_days >= 0 && props.node.remaining_days <= 7) {
    reasons.push(props.node.remaining_days <= 0 ? '服务已到期' : `服务剩余 ${props.node.remaining_days} 天`)
  }

  return reasons
})
const statusTitle = computed(() => {
  if (statusTone.value === 'warn') return alertReasons.value.length ? alertReasons.value.join(' / ') : '存在告警'
  if (statusTone.value === 'down') return '节点离线'
  return '节点在线'
})

function resourceValue(kind: 'cpu' | 'memory' | 'disk') {
  if (kind === 'cpu') return metric.value?.cpu_usage ?? 0
  if (kind === 'memory') return metric.value?.mem_used_percent ?? 0
  return metric.value?.disk_used_percent ?? 0
}

function metricUsageDetail(kind: 'cpu' | 'memory' | 'disk') {
  const current = metric.value
  if (!current) return 'N/A'
  if (kind === 'cpu') {
    return `1m ${current.load1.toFixed(2)} / ${props.formatCpuCores(current)}`
  }
  if (kind === 'memory') {
    if (!current.mem_total) return props.formatBytes(current.mem_used)
    return `${props.formatBytes(current.mem_used)} / ${props.formatBytes(current.mem_total)}`
  }
  if (!current.disk_total) return props.formatBytes(current.disk_used)
  return `${props.formatBytes(current.disk_used)} / ${props.formatBytes(current.disk_total)}`
}

function meterColor(value: number) {
  if (value >= 90) return '#ff6b86'
  if (value >= 70) return '#ffd36b'
  return '#62f3ae'
}

function barStyle(value: number) {
  const normalized = Math.max(0, Math.min(100, Math.round(value)))
  return {
    '--value': `${normalized}%`,
    '--meter': meterColor(normalized)
  }
}

function trafficBarStyle() {
  const value = props.trafficUsagePercent(props.node)
  return {
    '--traffic-value': `${Math.max(0, Math.min(100, value))}%`,
    '--traffic-meter': meterColor(value)
  }
}
</script>

<template>
  <article
    class="vps-card"
    :class="{ active: selected, warn: statusTone === 'warn', down: statusTone === 'down' }"
    :style="accentStyle"
    @click="emit('select', node.node_id)"
  >
    <div class="card-top">
      <div class="server-id">
        <div class="server-icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <rect x="4" y="5" width="16" height="6" rx="2" />
            <rect x="4" y="13" width="16" height="6" rx="2" />
            <path d="M8 8h.01M8 16h.01M12 8h4M12 16h4" />
          </svg>
        </div>
        <div class="server-name">
          <h3 class="node-name-with-id">
            <span class="node-name-trigger" tabindex="0">{{ nodeLabel(node) }}</span>
          </h3>
          <span class="server-meta" :title="nodeMetaTitle">
            <span class="server-region-text">{{ regionLabel }}</span>
            <span
              class="node-flag server-region-flag"
              :class="regionFlagClass(node.region)"
              :style="regionFlagStyle(node.region)"
              role="img"
              :aria-label="regionFlagLabel(node.region)"
            ></span>
            <span class="server-os-text">{{ osDisplay.label }}</span>
          </span>
        </div>
      </div>
      <div class="card-actions">
        <button class="probe-chart-button system-chart-button" type="button" title="系统趋势图表" @click.stop="emit('open-metrics', node)">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M4 18V6m0 12h16M8 15l3-4 3 2 4-6" />
          </svg>
        </button>
        <button v-if="node.probe_task_ids?.length" class="probe-chart-button system-chart-button" type="button" title="Ping 延迟图表" @click.stop="emit('open-probe', node)">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M4 16.5h3.2l2.2-8 3.2 11 2.4-6h5" />
          </svg>
        </button>
        <div class="status" :class="{ warn: statusTone === 'warn', down: statusTone === 'down' }" :title="statusTitle">
          <i></i><span>{{ statusLabel }}</span>
          <small v-if="statusTone === 'warn'" class="status-popover">{{ statusTitle }}</small>
        </div>
      </div>
    </div>

    <div class="main-stat" :class="{ 'has-tags': cardTags.length }">
      <div class="uptime-block">
        <div class="uptime-label">运行时长</div>
        <div class="uptime">{{ compactUptime }}</div>
      </div>
      <div v-if="cardTags.length" class="card-tag-placeholder" aria-hidden="true"></div>
      <div class="ping"><b>{{ pingLabel }}</b><span>Ping</span></div>
    </div>

    <div class="bars">
      <div class="bar-row metric-hotspot" @mouseenter="emit('trend', node, 'cpu', $event)" @mousemove="emit('move-trend', $event)" @mouseleave="emit('leave-trend')">
        <div class="bar-head"><span>CPU</span><b>{{ formatPercent(metric?.cpu_usage) }}</b></div>
        <div class="bar-detail" :title="metricUsageDetail('cpu')">{{ metricUsageDetail('cpu') }}</div>
        <div class="bar-track"><div class="bar-fill" :style="barStyle(resourceValue('cpu'))"></div></div>
      </div>
      <div class="bar-row metric-hotspot" @mouseenter="emit('trend', node, 'memory', $event)" @mousemove="emit('move-trend', $event)" @mouseleave="emit('leave-trend')">
        <div class="bar-head"><span>内存</span><b>{{ formatPercent(metric?.mem_used_percent) }}</b></div>
        <div class="bar-detail" :title="metricUsageDetail('memory')">{{ metricUsageDetail('memory') }}</div>
        <div class="bar-track"><div class="bar-fill" :style="barStyle(resourceValue('memory'))"></div></div>
      </div>
      <div class="bar-row metric-hotspot" @mouseenter="emit('trend', node, 'disk', $event)" @mousemove="emit('move-trend', $event)" @mouseleave="emit('leave-trend')">
        <div class="bar-head"><span>磁盘</span><b>{{ formatPercent(metric?.disk_used_percent) }}</b></div>
        <div class="bar-detail" :title="metricUsageDetail('disk')">{{ metricUsageDetail('disk') }}</div>
        <div class="bar-track"><div class="bar-fill" :style="barStyle(resourceValue('disk'))"></div></div>
      </div>
    </div>

    <div class="net-row">
      <div class="net-box metric-hotspot" @mouseenter="emit('trend', node, 'network', $event)" @mousemove="emit('move-trend', $event)" @mouseleave="emit('leave-trend')">
        <span class="net-label">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M12 19V5m0 0-5 5m5-5 5 5" />
          </svg>
          <em>上行</em>
        </span>
        <b>{{ formatBps(metric?.net_tx_bps) }}</b>
      </div>
      <div class="net-box metric-hotspot" @mouseenter="emit('trend', node, 'network', $event)" @mousemove="emit('move-trend', $event)" @mouseleave="emit('leave-trend')">
        <span class="net-label">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M12 5v14m0 0-5-5m5 5 5-5" />
          </svg>
          <em>下行</em>
        </span>
        <b>{{ formatBps(metric?.net_rx_bps) }}</b>
      </div>
    </div>

    <div v-if="appSettings.show_billing_details" class="plan-panel metric-hotspot" @mouseenter="emit('trend', node, 'traffic', $event)" @mousemove="emit('move-trend', $event)" @mouseleave="emit('leave-trend')">
      <div class="plan-head">
        <span>{{ appSettings.show_traffic_plan ? '套餐流量' : '套餐详情' }}</span>
        <b class="provider-badge">{{ node.provider || '未设置' }}</b>
      </div>
      <template v-if="appSettings.show_traffic_plan">
        <div class="traffic-line">
          <span>{{ node.traffic_limit_bytes ? `${formatBytes(node.traffic_used_bytes)} / ${formatBytes(node.traffic_limit_bytes)}` : `${formatBytes(node.traffic_used_bytes)} / 不限` }}</span>
          <b>{{ trafficUsagePercent(node) }}%</b>
        </div>
        <div class="traffic-track"><div class="traffic-fill" :style="trafficBarStyle()"></div></div>
      </template>
      <div class="plan-stats">
        <div class="plan-stat"><span>费用</span><b>{{ packageBillingSummary(node) }}</b></div>
        <div class="plan-stat package-remaining-card">
          <span>剩余</span>
          <b class="days-value" :class="{ warn: node.service_expires_at && node.remaining_days <= 14 && node.remaining_days > 7, danger: node.service_expires_at && node.remaining_days <= 7 }">{{ remainingPackageSummary(node) }}</b>
          <small class="package-value-popover">{{ remainingPackageTitle(node) }}</small>
        </div>
      </div>
    </div>

    <div class="card-foot">
      <span class="card-foot-tags">
        <template v-if="cardTags.length">
          <span v-for="tag in cardTags" :key="`${node.node_id}-foot-${tag}`">{{ tag }}</span>
        </template>
      </span>
      <span class="tag">{{ nodeIpSummary(node) || formatMegabytes(node.latest_metric?.net_rx_bytes_total) }}</span>
    </div>
  </article>
</template>
