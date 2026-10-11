<template>
  <div class="bg-[#141820] rounded-lg border border-[#232936] p-4 flex flex-col gap-4">
    <!-- Header of Telemetry Inspector -->
    <div class="flex items-center justify-between pb-3 border-b border-[#232936]">
      <div class="flex items-center gap-3">
        <span class="text-xs text-txt-muted">Seçili Cihaz Telemetrisi:</span>
        <template v-if="device">
          <div class="flex items-center gap-2">
            <span 
              class="w-2 h-2 rounded-full"
              :class="device.status === 'online' ? 'bg-emerald-500 led-pulse-emerald' : 'bg-rose-500'"
            ></span>
            <span class="text-sm font-semibold text-white">{{ device.name }}</span>
            <span class="text-xs font-mono text-txt-muted">({{ device.ip_address }})</span>
          </div>
        </template>
        <template v-else>
          <span class="text-xs text-txt-muted italic">Herhangi bir cihaz seçilmedi (listeden tıklayınız)</span>
        </template>
      </div>

      <div class="flex items-center gap-3 text-xs text-txt-secondary">
        <span v-if="device && device.is_agent_installed" class="flex items-center gap-1.5 text-emerald-400">
          <span class="material-symbols-outlined text-[15px]">verified_user</span>
          <span>Ajan Aktif</span>
        </span>
        <span v-else-if="device" class="flex items-center gap-1.5 text-txt-muted">
          <span class="material-symbols-outlined text-[15px]">info</span>
          <span>ICMP / Port İzleme (Ajan Yok)</span>
        </span>
      </div>
    </div>

    <!-- 3 Inspector Widgets -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
      <!-- 1. Real CPU & Memory Sparkline -->
      <div class="bg-[#0c0e12]/60 rounded-md border border-[#232936] p-3 flex flex-col justify-between min-h-[160px]">
        <div class="flex items-center justify-between text-xs">
          <span class="text-txt-muted">CPU & Bellek Kullanımı</span>
          <span class="text-[11px] font-mono text-txt-muted">Son Ölçümler</span>
        </div>

        <template v-if="metrics.length > 0">
          <!-- Real SVG Sparkline -->
          <div class="py-2">
            <svg class="w-full h-16 overflow-visible" viewBox="0 0 300 60">
              <!-- CPU Line (Cyan) -->
              <polyline
                fill="none"
                stroke="#38bdf8"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                :points="getCpuPoints(300, 60)"
              />
              <!-- RAM Line (Amber) -->
              <polyline
                fill="none"
                stroke="#f59e0b"
                stroke-width="1.5"
                stroke-dasharray="3,3"
                :points="getRamPoints(300, 60)"
              />
            </svg>
          </div>
          <div class="flex items-center justify-between text-[11px] text-txt-muted pt-1 border-t border-[#1e2533]">
            <span class="flex items-center gap-1 text-[#38bdf8]">
              <span class="w-2 h-0.5 bg-[#38bdf8]"></span> CPU: {{ latestMetric.cpu }}%
            </span>
            <span class="flex items-center gap-1 text-[#f59e0b]">
              <span class="w-2 h-0.5 bg-[#f59e0b]"></span> RAM: {{ latestMetric.ram }}%
            </span>
            <span v-if="latestMetric.temp" class="text-txt-secondary font-mono">
              {{ latestMetric.temp }}°C
            </span>
          </div>
        </template>
        <template v-else>
          <div class="flex flex-col items-center justify-center h-24 text-center text-txt-muted text-xs gap-1">
            <span class="material-symbols-outlined text-[20px]">timeline</span>
            <span>Ajan verisi bekleniyor (Henüz metrik kaydedilmedi)</span>
          </div>
        </template>
      </div>

      <!-- 2. Real Latency / Response Time -->
      <div class="bg-[#0c0e12]/60 rounded-md border border-[#232936] p-3 flex flex-col justify-between min-h-[160px]">
        <div class="flex items-center justify-between text-xs">
          <span class="text-txt-muted">Canlı Ping Gecikmesi</span>
          <span class="font-mono text-[11px] text-txt-secondary">
            {{ device ? device.response_time_ms + ' ms' : '—' }}
          </span>
        </div>

        <div class="flex flex-col items-center justify-center my-auto text-center">
          <span 
            class="text-3xl font-mono font-medium"
            :class="device && device.status === 'online' ? 'text-emerald-400' : 'text-txt-muted'"
          >
            {{ device && device.status === 'online' ? device.response_time_ms + ' ms' : '—' }}
          </span>
          <span class="text-xs text-txt-muted mt-1">
            {{ device ? 'Protokol: ICMP Echo / TCP Port' : 'Cihaz seçiniz' }}
          </span>
        </div>

        <div class="flex items-center justify-between text-[11px] text-txt-muted pt-1 border-t border-[#1e2533]">
          <span>Son Kontrol:</span>
          <span class="font-mono text-txt-secondary">{{ formatDate(device ? device.last_seen_at : null) }}</span>
        </div>
      </div>

      <!-- 3. Recent Real Incidents Timeline -->
      <div class="bg-[#0c0e12]/60 rounded-md border border-[#232936] p-3 flex flex-col justify-between min-h-[160px]">
        <div class="flex items-center justify-between text-xs pb-1.5 border-b border-[#1e2533]">
          <span class="text-txt-muted">Son Kesinti & Olaylar</span>
          <span class="text-[11px] text-txt-muted font-mono">Toplam: {{ incidents.length }}</span>
        </div>

        <div class="overflow-y-auto max-h-24 flex flex-col gap-1.5 py-1 text-xs">
          <template v-if="incidents.length > 0">
            <div 
              v-for="inc in incidents.slice(0, 3)" 
              :key="inc.id"
              class="flex items-center justify-between p-1.5 rounded bg-[#141820] border border-[#232936] text-[11px]"
            >
              <div class="flex items-center gap-1.5 truncate">
                <span 
                  class="w-1.5 h-1.5 rounded-full shrink-0"
                  :class="inc.resolved_at ? 'bg-emerald-400' : 'bg-rose-400'"
                ></span>
                <span class="truncate font-medium text-txt-primary">{{ inc.classified_cause }}</span>
              </div>
              <span class="text-txt-muted font-mono shrink-0 ml-2">
                {{ formatTime(inc.started_at) }}
              </span>
            </div>
          </template>
          <template v-else>
            <div class="flex flex-col items-center justify-center py-4 text-txt-muted text-xs">
              <span class="material-symbols-outlined text-[18px] text-emerald-400">check_circle</span>
              <span class="text-[11px] mt-1">Aktif kesinti kaydı bulunmuyor.</span>
            </div>
          </template>
        </div>

        <div class="text-[11px] text-txt-muted text-right pt-1 border-t border-[#1e2533]">
          <button @click="$emit('open-incidents')" class="text-blue-400 hover:underline">
            Tüm Olayları Görüntüle →
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  device: {
    type: Object,
    default: null
  },
  metrics: {
    type: Array,
    default: () => []
  },
  incidents: {
    type: Array,
    default: () => []
  }
})

defineEmits(['open-incidents'])

const latestMetric = computed(() => {
  if (props.metrics.length === 0) return { cpu: 0, ram: 0, temp: 0 }
  return props.metrics[0]
})

const getCpuPoints = (width, height) => {
  if (props.metrics.length < 2) return `0,${height} ${width},${height}`
  const step = width / (props.metrics.length - 1)
  return props.metrics.map((m, idx) => {
    const x = idx * step
    const y = height - (m.cpu / 100) * height
    return `${x},${y}`
  }).join(' ')
}

const getRamPoints = (width, height) => {
  if (props.metrics.length < 2) return `0,${height} ${width},${height}`
  const step = width / (props.metrics.length - 1)
  return props.metrics.map((m, idx) => {
    const x = idx * step
    const y = height - (m.ram / 100) * height
    return `${x},${y}`
  }).join(' ')
}

const formatDate = (dateStr) => {
  if (!dateStr) return '—'
  return new Date(dateStr).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' })
}

const formatTime = (dateStr) => {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' })
}
</script>
