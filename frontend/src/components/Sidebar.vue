<template>
  <aside class="fixed left-0 top-14 bottom-0 w-60 bg-[#0c0e12] border-r border-[#232936] z-30 flex flex-col justify-between p-4 select-none">
    <div class="flex flex-col gap-6 overflow-y-auto pr-1">
      <!-- Section: ALTYAPI -->
      <div class="flex flex-col gap-1">
        <span class="px-2.5 text-[11px] font-medium text-txt-muted uppercase tracking-wider">Altyapı</span>
        <nav class="flex flex-col gap-0.5 mt-1.5 text-xs">
          <button 
            @click="$emit('select-view', 'overview')"
            :class="currentView === 'overview' ? 'bg-[#141820] text-white font-medium border border-[#232936]/80' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820]'"
            class="flex items-center gap-2.5 px-2.5 py-1.5 rounded transition-colors text-left"
          >
            <span class="material-symbols-outlined text-[17px]" :class="{ 'text-blue-400': currentView === 'overview' }">grid_view</span>
            <span>Genel Bakış</span>
          </button>

          <button 
            @click="$emit('select-view', 'devices')"
            :class="currentView === 'devices' ? 'bg-[#141820] text-white font-medium border border-[#232936]/80' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820]'"
            class="flex items-center gap-2.5 px-2.5 py-1.5 rounded transition-colors text-left"
          >
            <span class="material-symbols-outlined text-[17px]" :class="{ 'text-blue-400': currentView === 'devices' }">dns</span>
            <span>Düğümler & Cihazlar</span>
          </button>

          <button 
            @click="$emit('select-view', 'nas')"
            :class="currentView === 'nas' ? 'bg-[#141820] text-white font-medium border border-[#232936]/80' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820]'"
            class="flex items-center gap-2.5 px-2.5 py-1.5 rounded transition-colors text-left"
          >
            <span class="material-symbols-outlined text-[17px]" :class="{ 'text-blue-400': currentView === 'nas' }">database</span>
            <span>Ağ Depolama & NAS</span>
          </button>

          <button 
            @click="$emit('select-view', 'power')"
            :class="currentView === 'power' ? 'bg-[#141820] text-white font-medium border border-[#232936]/80' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820]'"
            class="flex items-center gap-2.5 px-2.5 py-1.5 rounded transition-colors text-left"
          >
            <span class="material-symbols-outlined text-[17px]" :class="{ 'text-blue-400': currentView === 'power' }">thermostat</span>
            <span>Sıcaklık & Güç</span>
          </button>

          <button 
            @click="$emit('select-view', 'incidents')"
            :class="currentView === 'incidents' ? 'bg-[#141820] text-white font-medium border border-[#232936]/80' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820]'"
            class="flex items-center gap-2.5 px-2.5 py-1.5 rounded transition-colors text-left"
          >
            <span class="material-symbols-outlined text-[17px]" :class="{ 'text-blue-400': currentView === 'incidents' }">format_list_bulleted</span>
            <span>Olaylar & Günlükler</span>
          </button>
        </nav>
      </div>

      <!-- Section: SİSTEM -->
      <div class="flex flex-col gap-1">
        <span class="px-2.5 text-[11px] font-medium text-txt-muted uppercase tracking-wider">Sistem</span>
        <nav class="flex flex-col gap-0.5 mt-1.5 text-xs">
          <button 
            @click="$emit('open-discovery')"
            class="flex items-center gap-2.5 px-2.5 py-1.5 rounded text-txt-secondary hover:text-txt-primary hover:bg-[#141820] transition-colors text-left"
          >
            <span class="material-symbols-outlined text-[17px] text-cyan-400">radar</span>
            <span>Ağ Keşfi (CIDR)</span>
          </button>

          <button 
            @click="$emit('open-agent-modal')"
            class="flex items-center gap-2.5 px-2.5 py-1.5 rounded text-txt-secondary hover:text-txt-primary hover:bg-[#141820] transition-colors text-left"
          >
            <span class="material-symbols-outlined text-[17px] text-emerald-400">terminal</span>
            <span>Ajan Dağıtımı</span>
          </button>

          <button 
            @click="$emit('select-view', 'backups')"
            :class="currentView === 'backups' ? 'bg-[#141820] text-white font-medium border border-[#232936]/80' : 'text-txt-secondary hover:text-txt-primary hover:bg-[#141820]'"
            class="flex items-center gap-2.5 px-2.5 py-1.5 rounded transition-colors text-left"
          >
            <span class="material-symbols-outlined text-[17px]" :class="{ 'text-blue-400': currentView === 'backups' }">settings_backup_restore</span>
            <span>Yedekleme & Taşıma</span>
          </button>

          <button 
            @click="$emit('open-settings')"
            class="flex items-center gap-2.5 px-2.5 py-1.5 rounded text-txt-secondary hover:text-txt-primary hover:bg-[#141820] transition-colors text-left"
          >
            <span class="material-symbols-outlined text-[17px]">tune</span>
            <span>Yapılandırma & Alarmlar</span>
          </button>
        </nav>
      </div>
    </div>

    <!-- Gateway status box at bottom (100% Real Live Telemetry) -->
    <div class="p-3 rounded border border-[#232936] bg-[#141820]/60 text-xs flex flex-col gap-1.5">
      <div class="flex items-center justify-between text-txt-muted">
        <span>Ağ Geçidi</span>
        <span 
          class="flex items-center gap-1 font-mono text-[11px]"
          :class="gateway.status === 'online' ? 'text-emerald-400' : 'text-rose-400'"
        >
          <span 
            class="w-1.5 h-1.5 rounded-full" 
            :class="gateway.status === 'online' ? 'bg-emerald-400' : 'bg-rose-400'"
          ></span>
          {{ gateway.status === 'online' ? 'Çevrimiçi' : 'Ulaşılamıyor' }}
        </span>
      </div>
      <div class="flex items-center justify-between font-mono text-[11px] text-txt-secondary">
        <span>{{ gateway.ip || '192.168.1.1' }}</span>
        <span>{{ gateway.status === 'online' ? gateway.latency_ms + ' ms' : '—' }}</span>
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
  gateway: {
    type: Object,
    default: () => ({ ip: '192.168.1.1', status: 'offline', latency_ms: 0 })
  }
})

defineEmits([
  'select-view',
  'open-discovery',
  'open-agent-modal',
  'open-settings'
])
</script>
