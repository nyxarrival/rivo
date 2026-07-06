<script setup lang="ts">
import { computed } from 'vue'
import type { DashboardSparklinePoint } from '../types'

const props = withDefaults(defineProps<{
  points: DashboardSparklinePoint[]
  metric?: 'cpu' | 'memory' | 'disk' | 'network'
}>(), {
  metric: 'cpu'
})

const values = computed(() => props.points.map((point) => {
  if (props.metric === 'memory') return point.mem_used_percent
  if (props.metric === 'disk') return point.disk_used_percent
  if (props.metric === 'network') return Math.max(point.net_rx_bps, point.net_tx_bps)
  return point.cpu_usage
}).filter((value) => Number.isFinite(value)))

const polyline = computed(() => {
  if (values.value.length === 0) return ''
  if (values.value.length === 1) {
    const y = normalizeY(values.value[0], values.value)
    return `0,${y} 220,${y}`
  }
  const lastIndex = values.value.length - 1
  return values.value.map((value, index) => {
    const x = (index / lastIndex) * 220
    const y = normalizeY(value, values.value)
    return `${x.toFixed(2)},${y.toFixed(2)}`
  }).join(' ')
})

const area = computed(() => {
  if (!polyline.value) return ''
  return `0,36 ${polyline.value} 220,36`
})

function normalizeY(value: number, source: number[]) {
  const max = Math.max(...source)
  const min = Math.min(...source)
  if (max === min) return 18
  return 34 - ((value - min) / (max - min)) * 30
}
</script>

<template>
  <svg class="spark" viewBox="0 0 220 36" preserveAspectRatio="none" aria-hidden="true">
    <polygon v-if="area" :points="area" />
    <polyline v-if="polyline" :points="polyline" />
    <path v-else d="M0 26 C 44 18, 88 30, 132 17 S 192 18, 220 12" />
  </svg>
</template>
