<template>
  <header class="fixed top-0 left-0 right-0 z-40 h-14 bg-[#0c0e12]/95 backdrop-blur border-b border-[#232936] flex items-center justify-between px-4 lg:px-6 select-none">
    <div class="flex items-center gap-4 lg:gap-6">
      <!-- Brand Logo -->
      <a href="#" @click.prevent class="flex items-center gap-2.5 group">
        <div class="w-7 h-7 rounded-lg bg-blue-600/15 border border-blue-500/30 flex items-center justify-center text-blue-400 group-hover:border-blue-400 transition-colors shadow-sm">
          <span class="material-symbols-outlined text-[16px]">shield</span>
        </div>
        <span class="font-semibold text-sm tracking-tight text-white group-hover:text-blue-100 transition-colors">GNNwatchtower</span>
        <span class="text-[10px] font-mono text-emerald-400 font-medium px-1.5 py-0.5 rounded bg-emerald-500/10 border border-emerald-500/25">v{{ APP_VERSION }}</span>
      </a>

      <div class="hidden md:block h-4 w-px bg-[#232936]"></div>

      <!-- Live Cluster Telemetry Strip -->
      <div class="hidden md:flex items-center gap-3.5 text-xs text-txt-secondary">
        <!-- Fleet Status -->
        <span class="flex items-center gap-1.5" :title="'Aktif cihaz oranı: ' + (stats?.online_devices || 0) + ' / ' + (stats?.total_devices || 0)">
          <span 
            class="w-2 h-2 rounded-full" 
            :class="(stats?.online_devices || 0) > 0 ? 'bg-emerald-500 led-pulse-emerald' : 'bg-slate-500'"
          ></span>
          <span v-if="(stats?.total_devices || 0) > 0">
            Filo <strong class="text-txt-primary font-mono">{{ stats.total_devices }}</strong> Düğüm 
            (<span class="text-emerald-400 font-mono">{{ stats.online_devices }}</span> Aktif)
          </span>
          <span v-else class="text-txt-muted">
            0 Düğüm Kayıtlı
          </span>
        </span>

        <span class="text-[#232936]">•</span>

        <!-- Gateway Telemetry -->
        <span class="flex items-center gap-1.5" :title="'Ağ Geçidi IP: ' + (gateway?.ip || '192.168.1.1')">
          <span 
            class="w-2 h-2 rounded-full" 
            :class="gateway?.status === 'online' ? 'bg-emerald-400' : 'bg-rose-400'"
          ></span>
          <span>Ağ Geçidi:</span>
          <strong class="font-mono text-txt-primary font-normal">
            {{ gateway?.status === 'online' ? (gateway.latency_ms != null ? gateway.latency_ms + ' ms' : '< 1 ms') : 'Ulaşılamıyor' }}
          </strong>
        </span>

        <span class="text-[#232936]">•</span>

        <!-- SLA / Reachability -->
        <span class="flex items-center gap-1.5">
          <span class="text-txt-muted">Erişilebilirlik:</span>
          <strong class="font-mono text-emerald-400 font-normal">
            {{ reachability ? reachability : '%100.0' }}
          </strong>
        </span>

        <span class="text-[#232936]">•</span>

        <!-- Last refresh clock -->
        <span class="text-[11px] text-txt-muted flex items-center gap-1">
          <span>Son Veri:</span>
          <span class="font-mono text-txt-secondary">{{ lastRefresh || 'Bekleniyor' }}</span>
        </span>
      </div>
    </div>

    <!-- Right Controls: Search, Add Device, Notifications & Profile -->
    <div class="flex items-center gap-2.5">
      <!-- Search Input -->
      <div class="relative w-44 lg:w-60">
        <span class="material-symbols-outlined absolute left-2.5 top-1/2 -translate-y-1/2 text-txt-muted text-[15px]">search</span>
        <input 
          :value="searchQuery" 
          @input="$emit('update:searchQuery', $event.target.value)"
          ref="searchInput"
          class="w-full bg-[#141820] border border-[#232936] rounded-md pl-8 pr-9 py-1.5 text-xs text-txt-primary placeholder-txt-muted focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20 transition-all" 
          placeholder="Cihaz veya IP ara..." 
          type="text"
        />
        <kbd class="absolute right-2 top-1/2 -translate-y-1/2 text-[9px] text-txt-muted font-mono bg-[#1c222e] px-1 py-0.2 rounded border border-[#283142]">⌘K</kbd>
      </div>

      <!-- Quick Add Device Button -->
      <button 
        @click="$emit('add-device')"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-blue-600 hover:bg-blue-500 text-white transition-colors shadow-sm whitespace-nowrap"
        title="Yeni cihaz veya hedef ekle"
      >
        <span class="material-symbols-outlined text-[16px]">add</span>
        <span class="hidden sm:inline">Cihaz Ekle</span>
      </button>

      <div class="flex items-center gap-1 border-l border-[#232936] pl-2">
        <!-- Refresh Button -->
        <button 
          @click="$emit('refresh')" 
          :disabled="isRefreshing"
          class="p-1.5 rounded hover:bg-[#141820] text-txt-muted hover:text-txt-primary transition-colors disabled:opacity-40" 
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
          title="Kesintiler ve Olay Günlüğü"
        >
          <span class="material-symbols-outlined text-[18px]">notifications</span>
          <span 
            v-if="activeIncidentsCount > 0"
            class="w-2 h-2 rounded-full bg-rose-500 absolute top-1 right-1 led-pulse-rose"
          ></span>
        </button>

        <!-- Operator User Menu -->
        <div class="relative">
          <button 
            @click="isUserMenuOpen = !isUserMenuOpen"
            class="flex items-center gap-2 ml-1 pl-2 border-l border-[#232936] hover:opacity-85 transition-opacity"
            title="Kullanıcı Menüsü"
          >
            <div class="w-6 h-6 rounded-full bg-[#1e2533] border border-[#2a3447] flex items-center justify-center text-[10px] font-semibold text-txt-secondary">
              AD
            </div>
            <span class="text-xs text-txt-secondary hidden xl:inline">Yönetici</span>
            <span class="material-symbols-outlined text-[14px] text-txt-muted">expand_more</span>
          </button>

          <!-- Dropdown Menu -->
          <div 
            v-if="isUserMenuOpen" 
            class="absolute right-0 mt-2 w-48 bg-[#141820] border border-[#232936] rounded-md shadow-2xl py-1 z-50 text-xs"
          >
            <div class="px-3 py-1.5 border-b border-[#232936] text-[11px] text-txt-muted">
              GNNwatchtower Appliance
            </div>
            <button 
              @click="$emit('open-change-password'); isUserMenuOpen = false" 
              class="w-full text-left px-3 py-2 text-txt-secondary hover:text-txt-primary hover:bg-[#1d2432] flex items-center gap-2 transition-colors"
            >
              <span class="material-symbols-outlined text-[16px]">key</span>
              <span>Şifre Değiştir</span>
            </button>
            <button 
              @click="$emit('open-settings'); isUserMenuOpen = false" 
              class="w-full text-left px-3 py-2 text-txt-secondary hover:text-txt-primary hover:bg-[#1d2432] flex items-center gap-2 transition-colors"
            >
              <span class="material-symbols-outlined text-[16px]">tune</span>
              <span>Sistem Ayarları</span>
            </button>
            <div class="border-t border-[#232936] my-1"></div>
            <button 
              @click="$emit('logout'); isUserMenuOpen = false" 
              class="w-full text-left px-3 py-2 text-rose-400 hover:bg-[#1d2432] flex items-center gap-2 transition-colors"
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
import { APP_VERSION } from '../version'

defineProps({
  stats: {
    type: Object,
    default: () => ({})
  },
  gateway: {
    type: Object,
    default: () => null
  },
  reachability: {
    type: String,
    default: ''
  },
  searchQuery: {
    type: String,
    default: ''
  },
  isRefreshing: {
    type: Boolean,
    default: false
  },
  lastRefresh: {
    type: String,
    default: ''
  },
  activeIncidentsCount: {
    type: Number,
    default: 0
  }
})

defineEmits([
  'update:searchQuery', 
  'refresh', 
  'open-incidents', 
  'add-device',
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
