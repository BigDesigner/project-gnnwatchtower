<template>
  <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
    <!-- Active Host Cards Rendered from Real Devices -->
    <template v-if="hosts.length > 0">
      <div 
        v-for="host in hosts.slice(0, 3)" 
        :key="host.id"
        class="bg-[#141820] rounded-lg border border-[#232936] p-4 flex flex-col justify-between"
      >
        <div class="flex flex-col gap-3">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <span 
                class="w-2 h-2 rounded-full"
                :class="host.status === 'online' ? 'bg-emerald-500 led-pulse-emerald' : host.status === 'warning' ? 'bg-amber-500' : 'bg-rose-500'"
              ></span>
              <span class="font-medium text-sm text-txt-primary truncate max-w-[180px]">{{ host.name }}</span>
            </div>
            <span class="text-[11px] font-mono text-txt-muted">{{ host.ip_address }}</span>
          </div>

          <p class="text-xs text-txt-muted leading-relaxed truncate">
            {{ host.os_version || host.device_type.toUpperCase() }} • {{ host.purpose || host.location || 'Üretim Düğümü' }}
          </p>

          <div class="flex flex-col gap-2.5 pt-1">
            <!-- CPU / Ping Metric -->
            <div>
              <div class="flex justify-between text-xs mb-1">
                <span class="text-txt-muted">Yanıt Gecikmesi</span>
                <span class="font-mono text-txt-primary">
                  {{ host.response_time_ms }} ms
                </span>
              </div>
              <div class="h-1.5 w-full bg-[#1c222e] rounded-full overflow-hidden">
                <div 
                  class="h-full bg-blue-500 rounded-full transition-all duration-500" 
                  :style="{ width: Math.min(host.response_time_ms * 2, 100) + '%' }"
                ></div>
              </div>
            </div>

            <!-- Profile & Check Interval -->
            <div class="flex items-center justify-between text-[11px] text-txt-muted pt-1">
              <span>Profil: <strong class="text-txt-primary font-normal">{{ host.monitoring_profile }}</strong></span>
              <span>Aralık: <strong class="text-txt-primary font-normal font-mono">{{ host.check_interval_seconds }}s</strong></span>
            </div>
          </div>
        </div>

        <div class="pt-3 mt-3 border-t border-[#1e2533] flex items-center justify-between text-[11px] text-txt-muted">
          <span>Durum Değişimi:</span>
          <span class="font-mono text-txt-secondary">{{ formatDate(host.last_status_change_at) }}</span>
        </div>
      </div>
    </template>

    <!-- Authentic Empty State when 0 devices configured (GOLDEN RULE) -->
    <template v-else>
      <div class="col-span-3 bg-[#141820]/60 rounded-lg border border-dashed border-[#232936] p-8 flex flex-col items-center justify-center text-center gap-3">
        <div class="w-10 h-10 rounded-full bg-[#1c222e] flex items-center justify-center text-txt-muted">
          <span class="material-symbols-outlined text-[22px]">dns</span>
        </div>
        <div class="flex flex-col gap-1 max-w-md">
          <h4 class="text-sm font-medium text-txt-primary">Henüz İzlenen Cihaz Bulunmuyor</h4>
          <p class="text-xs text-txt-muted">
            Altın Kural gereğince sistemde hiçbir sahte metrik gösterilmez. Ağınızdaki sunucu ve switchleri keşfetmek için tarama başlatın.
          </p>
        </div>
        <div class="flex items-center gap-3 mt-2">
          <button 
            @click="$emit('open-discovery')"
            class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-blue-600 hover:bg-blue-500 text-white transition-colors"
          >
            <span class="material-symbols-outlined text-[16px]">radar</span>
            <span>Ağ Keşfi Başlat (CIDR)</span>
          </button>
          <button 
            @click="$emit('open-add-device')"
            class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-[#141820] hover:bg-[#1d2432] text-txt-primary border border-[#232936] transition-colors"
          >
            <span class="material-symbols-outlined text-[16px]">add</span>
            <span>Manuel Cihaz Ekle</span>
          </button>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
const props = defineProps({
  hosts: {
    type: Array,
    default: () => []
  }
})

defineEmits(['open-discovery', 'open-add-device'])

const formatDate = (dateStr) => {
  if (!dateStr) return '—'
  const d = new Date(dateStr)
  return d.toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' })
}
</script>
