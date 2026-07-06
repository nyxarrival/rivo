<script setup lang="ts">
import { computed, ref } from 'vue'
import { NCard } from 'naive-ui'
import NodeCard from './NodeCard.vue'
import type { AppSettings, DashboardSparklinePoint, NodeMetric, NodeRecord, Summary, TrendKind } from '../types'

type RegionClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

const props = defineProps<{
  appSettings: AppSettings
  summary: Summary
  nodes: NodeRecord[]
  currentTime: number
  regionCount: number
  totalCpuCores: number
  totalMemoryBytes: number
  totalDiskBytes: number
  totalRxBps: number
  totalTxBps: number
  selectedNodeId: string
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
  formatTime: (value?: number | null) => string
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
  formatTrafficUsageSummary: () => string
}>()

const emit = defineEmits<{
  (event: 'select-node', nodeID: string): void
  (event: 'trend', node: NodeRecord, kind: TrendKind, mouseEvent: MouseEvent): void
  (event: 'move-trend', mouseEvent: MouseEvent): void
  (event: 'leave-trend'): void
  (event: 'open-probe', node: NodeRecord): void
  (event: 'open-metrics', node: NodeRecord): void
}>()

const filter = ref<'all' | 'online' | 'warn' | 'offline'>('all')

const filteredNodes = computed(() => props.nodes.filter((node) => {
  if (filter.value === 'all') return true
  if (filter.value === 'offline') return !props.isNodeOnline(node)
  if (filter.value === 'online') return props.isNodeOnline(node)
  return nodeTone(node) === 'warn'
}))

const totalTrafficUsedBytes = computed(() => props.nodes.reduce((total, node) => total + (node.traffic_used_bytes ?? 0), 0))
const totalTrafficLimitBytes = computed(() => props.nodes.reduce((total, node) => total + (node.traffic_limit_bytes ?? 0), 0))
const uptimeSecondsList = computed(() => props.nodes
  .filter((node) => props.isNodeOnline(node))
  .map((node) => node.latest_metric?.uptime_seconds ?? 0)
  .filter((value) => value > 0))
const maxUptime = computed(() => uptimeSecondsList.value.length ? Math.max(...uptimeSecondsList.value) : null)
const minUptime = computed(() => uptimeSecondsList.value.length ? Math.min(...uptimeSecondsList.value) : null)
const overviewStats = computed(() => [
  {
    label: 'Agent 数量',
    value: String(props.summary.nodes_total),
    note: `${props.summary.nodes_online} 在线 · ${props.summary.nodes_offline} 离线`
  },
  {
    label: '硬件总量',
    value: `${props.totalCpuCores || 'N/A'} C / ${props.formatBytes(props.totalMemoryBytes)} / ${props.formatBytes(props.totalDiskBytes)}`,
    note: 'CPU / 内存 / 磁盘'
  },
  {
    label: '已用总流量',
    value: props.formatBytes(totalTrafficUsedBytes.value),
    note: totalTrafficLimitBytes.value ? `套餐 ${props.formatBytes(totalTrafficLimitBytes.value)}` : '周期累计'
  },
  {
    label: '运行时长',
    value: maxUptime.value === null ? 'N/A' : `Max ${formatCompactUptime(maxUptime.value)}`,
    note: minUptime.value === null ? '暂无在线样本' : `Min ${formatCompactUptime(minUptime.value)}`
  },
  {
    label: '实时网络',
    value: `${props.formatBps(props.totalRxBps)} / ${props.formatBps(props.totalTxBps)}`,
    note: '入站 / 出站'
  }
])

function formatCompactUptime(seconds: number) {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  return `${days}D ${hours}H`
}

function nodeTone(node: NodeRecord) {
  if (!props.isNodeOnline(node)) return 'down'
  const metric = props.liveMetric(node)
  if ((metric?.cpu_usage ?? 0) >= 75 || (metric?.mem_used_percent ?? 0) >= 80 || (metric?.disk_used_percent ?? 0) >= 85) return 'warn'
  if ((props.summary.node_probe_stats[node.node_id]?.availability_percent ?? 100) < 96) return 'warn'
  if (node.service_expires_at && node.remaining_days >= 0 && node.remaining_days <= 7) return 'warn'
  return 'ok'
}

function sparklineFor(node: NodeRecord): DashboardSparklinePoint[] {
  return props.summary.node_sparklines?.[node.node_id] ?? []
}
</script>

<template>
  <section class="home-view">
    <section v-if="appSettings.show_home_summary" class="hero summary-overview-wrap">
      <aside class="summary-card summary-overview-card">
        <div class="summary-overview-head">
          <div class="summary-copy">
            <h2>资源总览</h2>
            <p>Agent、硬件容量、流量与网络状态汇总。</p>
          </div>
        </div>
        <div class="overview-stat-grid">
          <div v-for="item in overviewStats" :key="item.label" class="overview-stat-card">
            <span>{{ item.label }}</span>
            <b>{{ item.value }}</b>
            <small>{{ item.note }}</small>
          </div>
        </div>
      </aside>
    </section>

    <div class="section-head section-head-filters-only">
      <div class="filters" aria-label="筛选">
        <button type="button" class="filter" :class="{ active: filter === 'all' }" @click="filter = 'all'">全部</button>
        <button type="button" class="filter" :class="{ active: filter === 'online' }" @click="filter = 'online'">在线</button>
        <button type="button" class="filter" :class="{ active: filter === 'warn' }" @click="filter = 'warn'">告警</button>
        <button type="button" class="filter" :class="{ active: filter === 'offline' }" @click="filter = 'offline'">离线</button>
      </div>
    </div>

    <section class="grid" aria-label="VPS 卡片列表">
      <NodeCard
        v-for="node in filteredNodes"
        :key="node.node_id"
        :node="node"
        :selected="node.node_id === selectedNodeId"
        :app-settings="appSettings"
        :sparkline="sparklineFor(node)"
        :probe-stat="summary.node_probe_stats?.[node.node_id]"
        :node-label="nodeLabel"
        :node-hover-tags="nodeHoverTags"
        :node-ip-summary="nodeIpSummary"
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
        :format-cpu-cores="formatCpuCores"
        :format-load="formatLoad"
        :format-load-title="formatLoadTitle"
        :format-node-uptime="formatNodeUptime"
        :format-os-display="formatOsDisplay"
        :format-os-name="formatOsName"
        :traffic-usage-percent="trafficUsagePercent"
        :traffic-remaining-line="trafficRemainingLine"
        :package-billing-summary="packageBillingSummary"
        :remaining-package-summary="remainingPackageSummary"
        :remaining-package-title="remainingPackageTitle"
        @select="emit('select-node', $event)"
        @trend="(node, kind, event) => emit('trend', node, kind, event)"
        @move-trend="emit('move-trend', $event)"
        @leave-trend="emit('leave-trend')"
        @open-probe="emit('open-probe', $event)"
        @open-metrics="emit('open-metrics', $event)"
      />

      <n-card v-if="nodes.length === 0" class="empty-card" :bordered="false">
        <h3>暂无 Agent</h3>
      </n-card>
    </section>
  </section>
</template>
