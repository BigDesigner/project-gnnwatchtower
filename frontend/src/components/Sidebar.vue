<template>
  <aside class="fixed left-0 top-14 bottom-0 w-60 bg-[#0c0e12] border-r border-[#232936] z-30 flex flex-col justify-between p-4 select-none">
    <div class="flex flex-col gap-6 overflow-y-auto pr-1">
      <!-- Section: İZLEME MERKEZİ -->
      <div class="flex flex-col gap-1">
        <span class="px-2.5 text-[10px] font-semibold text-txt-muted uppercase tracking-wider">İzleme Merkezi</span>
        <nav class="flex flex-col gap-0.5 mt-1 text-xs">
          <!-- Genel Bakış -->
          <button 
            @click="$emit('select-view', 'overview')"
            :class="currentView === 'overview' ? 'bg-[#141820] text-white font-medium border border-[#232936]' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820] border border-transparent'"
            class="flex items-center gap-2.5 px-2.5 py-2 rounded-md transition-colors text-left group"
          >
            <span class="material-symbols-outlined text-[17px]" :class="currentView === 'overview' ? 'text-blue-400' : 'text-txt-muted group-hover:text-txt-primary'">grid_view</span>
            <span>Genel Bakış</span>
          </button>

          <!-- Ajan Özeti -->
          <button 
            @click="$emit('select-view', 'devices')"
            :class="currentView === 'devices' ? 'bg-[#141820] text-white font-medium border border-[#232936]' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820] border border-transparent'"
            class="flex items-center justify-between px-2.5 py-2 rounded-md transition-colors text-left group"
          >
            <div class="flex items-center gap-2.5">
              <span class="material-symbols-outlined text-[17px]" :class="currentView === 'devices' ? 'text-blue-400' : 'text-txt-muted group-hover:text-txt-primary'">dns</span>
              <span>Ajan Özeti</span>
            </div>
            <span v-if="counts?.server !== undefined" class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[#181e28] text-txt-muted border border-[#232936]">
              {{ counts.server }}
            </span>
          </button>

          <!-- Web İzleme -->
          <button 
            @click="$emit('select-view', 'websites')"
            :class="currentView === 'websites' ? 'bg-[#141820] text-white font-medium border border-[#232936]' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820] border border-transparent'"
            class="flex items-center justify-between px-2.5 py-2 rounded-md transition-colors text-left group"
          >
            <div class="flex items-center gap-2.5">
              <span class="material-symbols-outlined text-[17px]" :class="currentView === 'websites' ? 'text-emerald-400' : 'text-txt-muted group-hover:text-txt-primary'">language</span>
              <span>Web İzleme</span>
            </div>
            <span v-if="counts?.website !== undefined" class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[#181e28] text-emerald-400 border border-emerald-500/20">
              {{ counts.website }}
            </span>
          </button>

          <!-- IP Cihazları -->
          <button 
            @click="$emit('select-view', 'ip-devices')"
            :class="currentView === 'ip-devices' ? 'bg-[#141820] text-white font-medium border border-[#232936]' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820] border border-transparent'"
            class="flex items-center justify-between px-2.5 py-2 rounded-md transition-colors text-left group"
          >
            <div class="flex items-center gap-2.5">
              <span class="material-symbols-outlined text-[17px]" :class="currentView === 'ip-devices' ? 'text-cyan-400' : 'text-txt-muted group-hover:text-txt-primary'">router</span>
              <span>IP Cihazları</span>
            </div>
            <span v-if="counts?.icmp !== undefined" class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[#181e28] text-cyan-400 border border-cyan-500/20">
              {{ counts.icmp }}
            </span>
          </button>

          <!-- Olay Günlüğü -->
          <button 
            @click="$emit('select-view', 'incidents')"
            :class="currentView === 'incidents' ? 'bg-[#141820] text-white font-medium border border-[#232936]' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820] border border-transparent'"
            class="flex items-center justify-between px-2.5 py-2 rounded-md transition-colors text-left group"
          >
            <div class="flex items-center gap-2.5">
              <span class="material-symbols-outlined text-[17px]" :class="currentView === 'incidents' ? 'text-amber-400' : 'text-txt-muted group-hover:text-txt-primary'">history</span>
              <span>Olay Günlüğü</span>
            </div>
            <span v-if="activeIncidentsCount > 0" class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-rose-500/20 text-rose-400 border border-rose-500/30">
              {{ activeIncidentsCount }}
            </span>
          </button>
        </nav>
      </div>

      <!-- Section: YÖNETİM -->
      <div class="flex flex-col gap-1">
        <span class="px-2.5 text-[10px] font-semibold text-txt-muted uppercase tracking-wider">Yönetim</span>
        <nav class="flex flex-col gap-0.5 mt-1 text-xs">
          <!-- Yedekleme Merkezi -->
          <button 
            @click="$emit('select-view', 'backups')"
            :class="currentView === 'backups' ? 'bg-[#141820] text-white font-medium border border-[#232936]' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820] border border-transparent'"
            class="flex items-center gap-2.5 px-2.5 py-2 rounded-md transition-colors text-left group"
          >
            <span class="material-symbols-outlined text-[17px]" :class="currentView === 'backups' ? 'text-blue-400' : 'text-txt-muted group-hover:text-txt-primary'">settings_backup_restore</span>
            <span>Yedekleme Merkezi</span>
          </button>

          <!-- Sistem Ayarları -->
          <button 
            @click="$emit('open-settings')"
            class="flex items-center gap-2.5 px-2.5 py-2 rounded-md text-txt-secondary hover:text-txt-primary hover:bg-[#141820] border border-transparent transition-colors text-left group"
          >
            <span class="material-symbols-outlined text-[17px] text-txt-muted group-hover:text-txt-primary">tune</span>
            <span>Sistem Ayarları</span>
          </button>
        </nav>
      </div>
    </div>

    <!-- Gateway status box at bottom (100% Real Live Telemetry) -->
    <div class="p-3 rounded-lg border border-[#232936] bg-[#141820]/80 text-xs flex flex-col gap-2 shadow-inner">
      <div class="flex items-center justify-between text-txt-muted text-[11px]">
        <span class="font-medium">Ağ Geçidi (ICMP)</span>
        <span 
          class="flex items-center gap-1 font-mono"
          :class="gateway?.status === 'online' ? 'text-emerald-400' : 'text-rose-400'"
        >
          <span 
            class="w-1.5 h-1.5 rounded-full" 
            :class="gateway?.status === 'online' ? 'bg-emerald-400' : 'bg-rose-400'"
          ></span>
          {{ gateway?.status === 'online' ? 'Erişilebilir' : 'Ulaşılamıyor' }}
        </span>
      </div>
      <div class="flex items-center justify-between font-mono text-[11px] text-txt-secondary">
        <span>{{ gateway?.ip || '192.168.1.1' }}</span>
        <span>{{ gateway?.status === 'online' ? (gateway.latency_ms != null ? gateway.latency_ms + ' ms' : '< 1 ms') : '—' }}</span>
      </div>
      <div class="pt-1.5 border-t border-[#232936]/60 text-[10px] text-txt-muted leading-tight">
        Sıfır ek saldırı yüzeyi. Yalnızca ICMP yoklama.
      </div>
    </div>
  </aside>
</template>

<script setup>
defineProps({
  currentView: {
    type: String,
    default: 'overview'
  },
  counts: {
    type: Object,
    default: () => ({})
  },
  activeIncidentsCount: {
    type: Number,
    default: 0
  },
  gateway: {
    type: Object,
    default: () => null
  }
})

defineEmits(['select-view', 'open-settings'])
</script>
