<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
    <div class="bg-[#141820] border border-[#232936] rounded-lg max-w-2xl w-full flex flex-col shadow-2xl overflow-hidden">
      <!-- Header -->
      <div class="p-4 border-b border-[#232936] flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-cyan-400">radar</span>
          <h3 class="text-sm font-semibold text-white">Yerel Ağ Keşfi (CIDR Tarayıcı)</h3>
        </div>
        <button @click="$emit('close')" class="text-txt-muted hover:text-white">
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <!-- Body -->
      <div class="p-5 flex flex-col gap-4 text-xs">
        <div class="flex flex-col sm:flex-row items-center gap-2">
          <div class="relative flex-1 w-full">
            <span class="material-symbols-outlined absolute left-2.5 top-1/2 -translate-y-1/2 text-txt-muted text-[16px]">lan</span>
            <input 
              v-model="cidr" 
              type="text" 
              placeholder="Örn: 192.168.1.0/24"
              class="w-full bg-[#0c0e12] border border-[#232936] rounded-md pl-8 pr-3 py-2 text-txt-primary font-mono focus:outline-none focus:border-cyan-500"
            />
          </div>
          <button 
            @click="startScan" 
            :disabled="isScanning"
            class="w-full sm:w-auto inline-flex items-center justify-center gap-1.5 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-md font-medium transition-colors"
          >
            <span class="material-symbols-outlined text-[16px]" :class="{ 'animate-spin': isScanning }">
              {{ isScanning ? 'sync' : 'search' }}
            </span>
            <span>{{ isScanning ? 'Taranıyor (64 goroutine)...' : 'Ağı Tara' }}</span>
          </button>
        </div>

        <p class="text-[11px] text-txt-muted leading-relaxed">
          64 eşzamanlı iş parçacığıyla yerel alt ağdaki aktif IP'ler, MAC adresleri ve açık yönetim portları (22, 80, 443, 902, 3389, 5000, 5001) otomatik tespit edilir.
        </p>

        <!-- Discovered Table -->
        <div class="border border-[#232936] rounded-md overflow-hidden max-h-60 overflow-y-auto">
          <table class="w-full text-left border-collapse">
            <thead class="bg-[#0c0e12] text-txt-muted text-[10px] uppercase font-mono sticky top-0">
              <tr>
                <th class="p-2"><input type="checkbox" v-model="selectAll" @change="toggleSelectAll" class="rounded border-[#232936]"></th>
                <th class="p-2">IP Adresi</th>
                <th class="p-2">Tahmini Cihaz Tipi</th>
                <th class="p-2">Açık Portlar</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-[#1e2533]">
              <template v-if="discovered.length > 0">
                <tr v-for="h in discovered" :key="h.ip" class="hover:bg-[#181e28]">
                  <td class="p-2"><input type="checkbox" v-model="selectedHosts" :value="h" class="rounded border-[#232936]"></td>
                  <td class="p-2 font-mono text-txt-primary">{{ h.ip }}</td>
                  <td class="p-2 text-txt-secondary">{{ h.suggested_type || 'Bilinmiyor' }}</td>
                  <td class="p-2 font-mono text-txt-muted">{{ h.open_ports ? h.open_ports.join(', ') : 'Port yok' }}</td>
                </tr>
              </template>
              <template v-else-if="!isScanning">
                <tr>
                  <td colspan="4" class="p-6 text-center text-txt-muted">
                    Tarama başlatılmadı veya cihaz bulunamadı.
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Footer -->
      <div class="p-4 border-t border-[#232936] bg-[#0c0e12]/60 flex items-center justify-between">
        <span class="text-xs text-txt-muted">{{ selectedHosts.length }} cihaz seçildi</span>
        <div class="flex items-center gap-2">
          <button @click="$emit('close')" class="px-3 py-1.5 rounded text-xs text-txt-secondary hover:text-white">
            İptal
          </button>
          <button 
            @click="importSelected" 
            :disabled="selectedHosts.length === 0"
            class="px-4 py-1.5 rounded text-xs font-medium bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white"
          >
            Seçilenleri Sisteme Ekle
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import api from '../services/api'

const props = defineProps({
  isOpen: Boolean
})

const emit = defineEmits(['close', 'imported'])

const cidr = ref('192.168.1.0/24')
const isScanning = ref(false)
const discovered = ref([])
const selectedHosts = ref([])
const selectAll = ref(false)

const startScan = async () => {
  if (!cidr.value) return
  isScanning.value = true
  discovered.value = []
  selectedHosts.value = []
  try {
    const res = await api.scanSubnet(cidr.value)
    discovered.value = res.hosts || []
    selectedHosts.value = [...discovered.value]
    selectAll.value = true
  } catch (err) {
    alert('Tarama hatası: ' + (err.response?.data?.error || err.message))
  } finally {
    isScanning.value = false
  }
}

const toggleSelectAll = () => {
  if (selectAll.value) {
    selectedHosts.value = [...discovered.value]
  } else {
    selectedHosts.value = []
  }
}

const importSelected = async () => {
  for (const h of selectedHosts.value) {
    try {
      await api.createDevice({
        name: `${h.suggested_type || 'Cihaz'}-${h.ip.split('.').slice(-2).join('-')}`,
        ip_address: h.ip,
        device_type: h.suggested_type || 'other',
        monitoring_profile: 'active',
        check_interval_seconds: 15
      })
    } catch (e) {
      // Ignore duplicates
    }
  }
  emit('imported')
  emit('close')
}
</script>
