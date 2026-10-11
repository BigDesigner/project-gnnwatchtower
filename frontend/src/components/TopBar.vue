<template>
  <header class="fixed top-0 left-0 right-0 z-40 h-14 bg-[#0c0e12]/95 backdrop-blur border-b border-[#232936] flex items-center justify-between px-6">
    <div class="flex items-center gap-6">
      <!-- Brand Logo -->
      <div class="flex items-center gap-2.5">
        <div class="w-6 h-6 rounded bg-[#232936] flex items-center justify-center text-txt-primary">
          <span class="material-symbols-outlined text-[15px]">token</span>
        </div>
        <span class="font-semibold text-sm tracking-tight text-white">GNNwatchtower</span>
        <span class="text-[11px] font-mono text-emerald-400 font-medium px-2 py-0.5 rounded bg-[#141820] border border-emerald-500/30">v0.2.0</span>
      </div>

      <div class="h-4 w-px bg-[#232936]"></div>

      <!-- Live Cluster Telemetry -->
      <div class="flex items-center gap-4 text-xs text-txt-secondary">
        <span class="flex items-center gap-1.5">
          <span 
            class="w-1.5 h-1.5 rounded-full" 
            :class="stats.online_devices > 0 ? 'bg-emerald-500 led-pulse-emerald' : 'bg-slate-500'"
          ></span>
          <span v-if="stats.total_devices > 0">
            Filo {{ stats.total_devices }} Düğüm ({{ stats.online_devices }} Aktif)
          </span>
          <span v-else class="text-txt-muted">
            0 Düğüm Kayıtlı
          </span>
        </span>

        <span class="text-[#232936]">•</span>

        <span>
          Ort. Sıcaklık: 
          <strong class="font-mono text-txt-primary font-normal">
            {{ stats.avg_temperature !== null && stats.avg_temperature !== undefined ? stats.avg_temperature + '°C' : '—' }}
          </strong>
        </span>

        <span class="text-[#232936]">•</span>

        <span>
          SLA Uptime: 
          <strong class="font-mono text-txt-primary font-normal">
            %{{ stats.uptime_percentage || '100.0' }}
          </strong>
        </span>
      </div>
    </div>

    <!-- Search, Refresh & Operator Section -->
    <div class="flex items-center gap-3">
      <!-- Search Input -->
      <div class="relative w-64">
        <span class="material-symbols-outlined absolute left-2.5 top-1/2 -translate-y-1/2 text-txt-muted text-[16px]">search</span>
        <input 
          :value="searchQuery" 
          @input="$emit('update:searchQuery', $event.target.value)"
          ref="searchInput"
          class="w-full bg-[#141820] border border-[#232936] rounded-md pl-8 pr-10 py-1.5 text-xs text-txt-primary placeholder-txt-muted focus:outline-none focus:border-brand-500 transition-colors" 
          placeholder="Cihaz, IP veya etiket ara..." 
          type="text"
        />
        <kbd class="absolute right-2.5 top-1/2 -translate-y-1/2 text-[10px] text-txt-muted font-mono bg-[#1c222e] px-1 rounded border border-[#283142]">⌘K</kbd>
      </div>

      <div class="flex items-center gap-1 border-l border-[#232936] pl-3">
        <!-- Refresh Button -->
        <button 
          @click="$emit('refresh')" 
          :disabled="isRefreshing"
          class="p-1.5 rounded hover:bg-[#141820] text-txt-muted hover:text-txt-primary transition-colors disabled:opacity-50" 
          title="Verileri anlık yenile"
        >
          <span 
            class="material-symbols-outlined text-[18px]" 
            :class="{ 'animate-spin': isRefreshing }"
          >sync</span>
        </button>

        <!-- Incidents Bell -->
        <button 
          @click="$emit('open-incidents')"
          class="p-1.5 rounded hover:bg-[#141820] text-txt-muted hover:text-txt-primary transition-colors relative" 
          title="Kesintiler ve Uyarılar"
        >
          <span class="material-symbols-outlined text-[18px]">notifications</span>
          <span 
            v-if="stats.unclassified_incidents > 0"
            class="w-2 h-2 rounded-full bg-rose-500 absolute top-1 right-1"
          ></span>
        </button>

        <!-- Operator User Menu -->
        <div class="relative">
          <button 
            @click="isUserMenuOpen = !isUserMenuOpen"
            class="flex items-center gap-2 ml-1 pl-2 border-l border-[#232936] hover:opacity-80 transition-opacity"
          >
            <div class="w-6 h-6 rounded-full bg-[#1e2533] border border-[#2a3447] flex items-center justify-center text-xs font-medium text-txt-secondary">
              AD
            </div>
            <span class="text-xs text-txt-secondary hidden sm:inline">Admin</span>
            <span class="material-symbols-outlined text-[14px] text-txt-muted">expand_more</span>
          </button>

          <!-- Dropdown Menu -->
          <div 
            v-if="isUserMenuOpen" 
            class="absolute right-0 mt-2 w-48 bg-[#141820] border border-[#232936] rounded-md shadow-xl py-1 z-50 text-xs"
          >
            <button 
              @click="$emit('open-change-password'); isUserMenuOpen = false" 
              class="w-full text-left px-3 py-2 text-txt-secondary hover:text-txt-primary hover:bg-[#1d2432] flex items-center gap-2"
            >
              <span class="material-symbols-outlined text-[16px]">key</span>
              <span>Şifre Değiştir</span>
            </button>
            <button 
              @click="$emit('open-settings'); isUserMenuOpen = false" 
              class="w-full text-left px-3 py-2 text-txt-secondary hover:text-txt-primary hover:bg-[#1d2432] flex items-center gap-2"
            >
              <span class="material-symbols-outlined text-[16px]">settings</span>
              <span>Sistem Ayarları</span>
            </button>
            <div class="border-t border-[#232936] my-1"></div>
            <button 
              @click="$emit('logout'); isUserMenuOpen = false" 
              class="w-full text-left px-3 py-2 text-rose-400 hover:bg-[#1d2432] flex items-center gap-2"
            >
              <span class="material-symbols-outlined text-[16px]">logout</span>
              <span>Çıkış Yap</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  stats: {
    type: Object,
    default: () => ({})
  },
  searchQuery: {
    type: String,
    default: ''
  },
  isRefreshing: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits([
  'update:searchQuery', 
  'refresh', 
  'open-incidents', 
  'open-settings', 
  'open-change-password', 
  'logout'
])

const isUserMenuOpen = ref(false)
const searchInput = ref(null)

const handleKeydown = (e) => {
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    e.preventDefault()
    searchInput.value?.focus()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>
