<template>
  <div class="bg-[#141820] rounded-lg border border-[#232936] flex flex-col overflow-hidden">
    <!-- Header with Filter Tabs & Search -->
    <div class="p-4 border-b border-[#232936] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <!-- Filter Tabs -->
      <div class="flex items-center gap-1 overflow-x-auto text-xs pb-1 sm:pb-0">
        <button 
          v-for="tab in filterTabs" 
          :key="tab.id"
          @click="activeTab = tab.id"
          :class="activeTab === tab.id ? 'bg-[#1e2533] text-txt-primary border-b-2 border-brand-500 font-medium' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#1a202c]'"
          class="px-3 py-1.5 rounded-t transition-colors whitespace-nowrap flex items-center gap-1.5"
        >
          <span>{{ tab.label }}</span>
          <span class="text-[10px] font-mono px-1 py-0.2 rounded bg-[#0c0e12] border border-[#232936]">
            {{ tab.count }}
          </span>
        </button>
      </div>

      <!-- Quick Action Buttons -->
      <div class="flex items-center gap-2 self-end sm:self-auto">
        <button 
          @click="$emit('open-add-device')"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-blue-600 hover:bg-blue-500 text-white transition-colors shadow-sm"
        >
          <span class="material-symbols-outlined text-[16px]">add</span>
          <span>Yeni Cihaz</span>
        </button>
        <button 
          @click="$emit('open-discovery')"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-[#181e28] hover:bg-[#1d2432] text-txt-primary border border-[#232936] transition-colors"
        >
          <span class="material-symbols-outlined text-[16px] text-cyan-400">radar</span>
          <span>Ağ Keşfi</span>
        </button>
      </div>
    </div>

    <!-- Table -->
    <div class="overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-[#232936] bg-[#0c0e12]/40 text-txt-muted text-[11px] uppercase tracking-wider font-medium">
            <th class="py-2.5 px-4 w-10">
              <input type="checkbox" class="rounded border-[#232936] bg-[#141820] text-blue-600 focus:ring-0">
            </th>
            <th class="py-2.5 px-4">Cihaz Adı</th>
            <th class="py-2.5 px-4">İşletim Sistemi / Tip</th>
            <th class="py-2.5 px-4">IP & MAC Adresi</th>
            <th class="py-2.5 px-4">Profil</th>
            <th class="py-2.5 px-4">Yanıt (Ping)</th>
            <th class="py-2.5 px-4">Durum</th>
            <th class="py-2.5 px-4 text-right">İşlemler</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-[#1e2533]">
          <template v-if="filteredDevices.length > 0">
            <tr 
              v-for="device in filteredDevices" 
              :key="device.id"
              :class="{ 'bg-blue-950/20': selectedDeviceId === device.id }"
              class="hover:bg-[#181e28]/60 transition-colors group cursor-pointer"
              @click="$emit('select-device', device)"
            >
              <td class="py-3 px-4" @click.stop>
                <input type="checkbox" class="rounded border-[#232936] bg-[#141820] text-blue-600 focus:ring-0">
              </td>

              <!-- Name & Agent Indicator -->
              <td class="py-3 px-4">
                <div class="flex items-center gap-2">
                  <span 
                    class="w-2 h-2 rounded-full shrink-0"
                    :class="getStatusDotClass(device.status)"
                  ></span>
                  <div class="flex flex-col">
                    <span class="font-medium text-txt-primary group-hover:text-blue-400 transition-colors">
                      {{ device.name }}
                    </span>
                    <span v-if="device.purpose" class="text-[11px] text-txt-muted truncate max-w-[200px]">
                      {{ device.purpose }}
                    </span>
                  </div>
                </div>
              </td>

              <!-- OS / Type -->
              <td class="py-3 px-4 text-txt-secondary">
                <div class="flex items-center gap-1.5">
                  <span class="material-symbols-outlined text-[15px] text-txt-muted">
                    {{ getDeviceIcon(device.device_type) }}
                  </span>
                  <span>{{ formatDeviceType(device) }}</span>
                </div>
              </td>

              <!-- IP & MAC -->
              <td class="py-3 px-4 font-mono text-txt-secondary">
                <div class="flex flex-col">
                  <span class="text-txt-primary">{{ device.ip_address }}</span>
                  <span v-if="device.mac_address" class="text-[10px] text-txt-muted">{{ device.mac_address }}</span>
                </div>
              </td>

              <!-- Profile -->
              <td class="py-3 px-4">
                <span class="px-2 py-0.5 rounded text-[11px] bg-[#1c222e] text-txt-secondary border border-[#283142]">
                  {{ formatProfile(device.monitoring_profile) }}
                </span>
              </td>

              <!-- Ping Latency -->
              <td class="py-3 px-4 font-mono">
                <span 
                  :class="device.response_time_ms > 100 ? 'text-amber-400' : 'text-emerald-400'"
                >
                  {{ device.status === 'offline' ? '—' : (device.response_time_ms === 0 ? '< 1 ms' : device.response_time_ms + ' ms') }}
                </span>
              </td>

              <!-- Status Badge -->
              <td class="py-3 px-4">
                <span 
                  class="px-2 py-0.5 rounded text-[11px] font-medium border"
                  :class="getStatusBadgeClass(device.status)"
                >
                  {{ formatStatus(device.status) }}
                </span>
              </td>

              <!-- Actions -->
              <td class="py-3 px-4 text-right" @click.stop>
                <div class="flex items-center justify-end gap-1">
                  <button 
                    @click="$emit('select-device', device)"
                    title="Telemetri İncele"
                    class="p-1 rounded hover:bg-[#232936] text-txt-muted hover:text-blue-400 transition-colors"
                  >
                    <span class="material-symbols-outlined text-[16px]">show_chart</span>
                  </button>
                  <button 
                    @click="$emit('edit-device', device)"
                    title="Düzenle"
                    class="p-1 rounded hover:bg-[#232936] text-txt-muted hover:text-txt-primary transition-colors"
                  >
                    <span class="material-symbols-outlined text-[16px]">edit</span>
                  </button>
                  <button 
                    @click="$emit('delete-device', device.id)"
                    title="Sil"
                    class="p-1 rounded hover:bg-[#232936] text-txt-muted hover:text-rose-400 transition-colors"
                  >
                    <span class="material-symbols-outlined text-[16px]">delete</span>
                  </button>
                </div>
              </td>
            </tr>
          </template>

          <!-- Authentic Empty State Row -->
          <template v-else>
            <tr>
              <td colspan="8" class="py-12 text-center text-txt-muted">
                <div class="flex flex-col items-center justify-center gap-2">
                  <span class="material-symbols-outlined text-[28px] text-txt-muted">search_off</span>
                  <span class="text-xs">Bu filtreye uygun cihaz bulunamadı.</span>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- Table Footer -->
    <div class="p-3 border-t border-[#232936] bg-[#0c0e12]/40 flex items-center justify-between text-xs text-txt-muted">
      <span>Toplam {{ devices.length }} cihazdan {{ filteredDevices.length }} tanesi listeleniyor</span>
      <span class="font-mono text-[11px]">Canlı Yoklama: 15s</span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  devices: {
    type: Array,
    default: () => []
  },
  selectedDeviceId: {
    type: Number,
    default: null
  },
  searchQuery: {
    type: String,
    default: ''
  }
})

defineEmits([
  'select-device', 
  'open-add-device', 
  'open-discovery', 
  'edit-device', 
  'delete-device'
])

const activeTab = ref('all')

const filterTabs = computed(() => {
  const allCount = props.devices.length
  const onlineCount = props.devices.filter(d => d.status === 'online').length
  const warningCount = props.devices.filter(d => d.status === 'warning').length
  const offlineCount = props.devices.filter(d => d.status === 'offline').length
  const maintCount = props.devices.filter(d => d.monitoring_profile === 'maintenance').length

  return [
    { id: 'all', label: 'Tümü', count: allCount },
    { id: 'online', label: 'Çalışıyor', count: onlineCount },
    { id: 'warning', label: 'Uyarı', count: warningCount },
    { id: 'offline', label: 'Kesinti', count: offlineCount },
    { id: 'maintenance', label: 'Bakım', count: maintCount },
  ]
})

const filteredDevices = computed(() => {
  let list = props.devices

  // Tab filter
  if (activeTab.value === 'online') {
    list = list.filter(d => d.status === 'online')
  } else if (activeTab.value === 'warning') {
    list = list.filter(d => d.status === 'warning')
  } else if (activeTab.value === 'offline') {
    list = list.filter(d => d.status === 'offline')
  } else if (activeTab.value === 'maintenance') {
    list = list.filter(d => d.monitoring_profile === 'maintenance')
  }

  // Search filter
  if (props.searchQuery.trim()) {
    const q = props.searchQuery.toLowerCase()
    list = list.filter(d => 
      d.name.toLowerCase().includes(q) || 
      d.ip_address.toLowerCase().includes(q) ||
      (d.mac_address && d.mac_address.toLowerCase().includes(q)) ||
      (d.purpose && d.purpose.toLowerCase().includes(q))
    )
  }

  return list
})

const getStatusDotClass = (status) => {
  switch (status) {
    case 'online': return 'bg-emerald-500 led-pulse-emerald'
    case 'warning': return 'bg-amber-500'
    case 'offline': return 'bg-rose-500'
    default: return 'bg-slate-500'
  }
}

const getStatusBadgeClass = (status) => {
  switch (status) {
    case 'online': return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
    case 'warning': return 'bg-amber-500/10 text-amber-400 border-amber-500/20'
    case 'offline': return 'bg-rose-500/10 text-rose-400 border-rose-500/20'
    default: return 'bg-slate-500/10 text-slate-400 border-slate-500/20'
  }
}

const formatStatus = (status) => {
  switch (status) {
    case 'online': return 'Çevrimiçi'
    case 'warning': return 'Uyarı'
    case 'offline': return 'Çevrimdışı'
    default: return 'Bilinmiyor'
  }
}

const formatProfile = (profile) => {
  switch (profile) {
    case 'active': return 'Aktif (15s)'
    case 'dormant_24h': return 'Uyku (24h)'
    case 'dormant_30d': return 'Uyku (30d)'
    case 'maintenance': return 'Bakım Modu'
    default: return profile
  }
}

const formatDeviceType = (device) => {
  if (device.os_version) return device.os_version
  switch (device.device_type) {
    case 'synology': return 'Synology DSM'
    case 'nas': return 'Ağ Depolama (NAS)'
    case 'storage': return 'Depolama Ünitesi'
    case 'esxi': return 'VMware ESXi'
    case 'windows': return 'Windows Server'
    case 'linux':
    case 'ubuntu': return 'Linux / Ubuntu'
    case 'switch': return 'Omurga Switch'
    case 'router': return 'Ağ Yönlendirici'
    default: return device.device_type || 'Diğer'
  }
}

const getDeviceIcon = (type) => {
  switch (type) {
    case 'esxi': return 'dns'
    case 'synology':
    case 'nas':
    case 'storage': return 'database'
    case 'windows': return 'desktop_windows'
    case 'linux':
    case 'ubuntu': return 'terminal'
    case 'switch':
    case 'router': return 'router'
    default: return 'devices'
  }
}
</script>
