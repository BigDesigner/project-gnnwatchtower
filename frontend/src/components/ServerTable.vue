<template>
  <div class="overflow-x-auto">
    <table class="w-full text-left border-collapse text-xs">
      <thead>
        <tr class="border-b border-[#232936] bg-[#0c0e12]/60 text-txt-muted text-[11px] uppercase tracking-wider font-semibold select-none">
          <th class="py-3 px-3 w-12 font-mono text-center">ID</th>
          <th class="py-3 px-4">Sunucu & İşletim Sistemi</th>
          <th class="py-3 px-4">IP Adresi & Durum</th>
          <th class="py-3 px-4">Ajan Durumu</th>
          <th class="py-3 px-3 w-28 text-center">CPU</th>
          <th class="py-3 px-3 w-28 text-center">Bellek</th>
          <th class="py-3 px-3 w-28 text-center">Disk</th>
          <th class="py-3 px-4 text-right">Son Kontrol & Ping</th>
          <th class="py-3 px-4 text-right">İşlemler</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-[#1e2533]">
        <tr 
          v-for="d in devices" 
          :key="d.id"
          @click="$emit('select', d)"
          class="hover:bg-[#181e28]/70 transition-colors group cursor-pointer"
        >
          <!-- ID -->
          <td class="py-3 px-3 font-mono text-txt-muted text-[11px] text-center">
            #{{ d.id }}
          </td>

          <!-- Sunucu & OS -->
          <td class="py-3 px-4">
            <div class="flex items-center gap-2.5">
              <div class="w-7 h-7 rounded-md bg-[#1e2533] border border-[#2a3447] flex items-center justify-center shrink-0">
                <span class="material-symbols-outlined text-[16px] text-blue-400">
                  {{ d.device_type === 'windows' ? 'desktop_windows' : 'terminal' }}
                </span>
              </div>
              <div class="flex flex-col">
                <span class="font-medium text-white group-hover:text-blue-400 transition-colors">
                  {{ d.name }}
                </span>
                <span class="text-[11px] text-txt-muted">
                  {{ typeLabels[d.device_type] || d.device_type }}
                  <span v-if="d.location"> · {{ d.location }}</span>
                </span>
              </div>
            </div>
          </td>

          <!-- IP & Durum -->
          <td class="py-3 px-4">
            <div class="flex flex-col gap-1">
              <span class="font-mono text-txt-primary">{{ d.ip_address }}</span>
              <div>
                <DeviceStatus :device="d" :now="now" />
              </div>
            </div>
          </td>

          <!-- Ajan Durumu & Sürüm -->
          <td class="py-3 px-4">
            <div v-if="d.is_agent_installed" class="flex flex-col gap-0.5">
              <div class="flex items-center gap-1.5">
                <span 
                  class="w-1.5 h-1.5 rounded-full shrink-0"
                  :class="telemetryCurrent(d, now) ? 'bg-emerald-400' : 'bg-slate-500'"
                ></span>
                <span class="font-medium" :class="telemetryCurrent(d, now) ? 'text-emerald-400' : 'text-txt-muted'">
                  {{ telemetryCurrent(d, now) ? 'Güncel Veri' : 'Veri Bekleniyor' }}
                </span>
                <span class="text-[10px] font-mono px-1 py-0.2 rounded bg-[#181e28] text-txt-secondary border border-[#232936]">
                  v{{ d.agent_version || '?' }}
                </span>
              </div>
              <span v-if="d.latest_metric_at" class="text-[10px] text-txt-muted">
                {{ dateTime(d.latest_metric_at) }}
              </span>
            </div>
            <div v-else class="flex items-center gap-1.5 text-txt-muted">
              <span class="w-1.5 h-1.5 rounded-full bg-slate-600"></span>
              <span>Ajan Bekleniyor</span>
            </div>
          </td>

          <!-- CPU Gauge -->
          <td class="py-3 px-3 text-center" @click.stop>
            <div v-if="typeof d.latest_cpu === 'number' && telemetryCurrent(d, now)" class="flex flex-col gap-1 items-center">
              <span class="font-mono font-medium text-[11px]" :class="getTextColor(d.latest_cpu)">
                {{ d.latest_cpu.toFixed(1) }}%
              </span>
              <div class="w-16 h-1 bg-[#1e2533] rounded-full overflow-hidden">
                <div 
                  class="h-full rounded-full transition-all duration-500" 
                  :class="getBarColor(d.latest_cpu)"
                  :style="{ width: Math.min(100, Math.max(0, d.latest_cpu)) + '%' }"
                ></div>
              </div>
            </div>
            <span v-else class="text-txt-muted font-mono">—</span>
          </td>

          <!-- RAM Gauge -->
          <td class="py-3 px-3 text-center" @click.stop>
            <div v-if="typeof d.latest_ram === 'number' && telemetryCurrent(d, now)" class="flex flex-col gap-1 items-center">
              <span class="font-mono font-medium text-[11px]" :class="getTextColor(d.latest_ram)">
                {{ d.latest_ram.toFixed(1) }}%
              </span>
              <div class="w-16 h-1 bg-[#1e2533] rounded-full overflow-hidden">
                <div 
                  class="h-full rounded-full transition-all duration-500" 
                  :class="getBarColor(d.latest_ram)"
                  :style="{ width: Math.min(100, Math.max(0, d.latest_ram)) + '%' }"
                ></div>
              </div>
            </div>
            <span v-else class="text-txt-muted font-mono">—</span>
          </td>

          <!-- Disk Gauge -->
          <td class="py-3 px-3 text-center" @click.stop>
            <div v-if="typeof d.latest_disk === 'number' && telemetryCurrent(d, now)" class="flex flex-col gap-1 items-center">
              <span class="font-mono font-medium text-[11px]" :class="getTextColor(d.latest_disk)">
                {{ d.latest_disk.toFixed(1) }}%
              </span>
              <div class="w-16 h-1 bg-[#1e2533] rounded-full overflow-hidden">
                <div 
                  class="h-full rounded-full transition-all duration-500" 
                  :class="getBarColor(d.latest_disk)"
                  :style="{ width: Math.min(100, Math.max(0, d.latest_disk)) + '%' }"
                ></div>
              </div>
            </div>
            <span v-else class="text-txt-muted font-mono">—</span>
          </td>

          <!-- Son Kontrol & Gecikme -->
          <td class="py-3 px-4 text-right">
            <div class="flex flex-col items-end">
              <span class="text-txt-primary">{{ dateTime(d.last_checked_at) }}</span>
              <span class="text-[11px] font-mono text-txt-muted">
                Ping: <span class="text-txt-secondary">{{ latency(d) }}</span>
              </span>
            </div>
          </td>

          <!-- İşlemler & Ajan Güncelleme -->
          <td class="py-3 px-4 text-right" @click.stop>
            <div class="flex flex-col items-end gap-1.5">
              <DeviceActions 
                :device="d" 
                @edit="$emit('edit', $event)" 
                @delete="$emit('delete', $event)" 
                @profile="$emit('profile', $event)" 
                @incidents="$emit('incidents', $event)" 
              />

              <!-- Ajan Güncelleme Butonu / Durum Rozeti -->
              <div v-if="d.is_agent_installed" class="flex items-center gap-1 text-[11px]">
                <button 
                  v-if="!['pending','downloading','applying'].includes(d.agent_upgrade_status)"
                  @click="$emit('upgrade', d)"
                  class="text-blue-400 hover:text-blue-300 hover:underline flex items-center gap-1 text-[11px]"
                  title="Ajanı uzaktan son sürüme güncelle"
                >
                  <span class="material-symbols-outlined text-[13px]">system_update_alt</span>
                  <span>Ajanı Güncelle</span>
                </button>

                <!-- Canlı Güncelleme Durum Rozeti -->
                <span 
                  v-if="d.agent_upgrade_status && d.agent_upgrade_status !== 'none'"
                  class="px-1.5 py-0.2 rounded font-mono text-[10px] border flex items-center gap-1"
                  :class="getUpgradeBadgeClass(d.agent_upgrade_status)"
                  :title="d.agent_upgrade_error || d.agent_upgrade_status"
                >
                  <span v-if="['pending','downloading','applying'].includes(d.agent_upgrade_status)" class="material-symbols-outlined text-[11px] animate-spin">sync</span>
                  <span>{{ getUpgradeStatusLabel(d.agent_upgrade_status) }}</span>
                </span>
              </div>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import DeviceStatus from "./DeviceStatus.vue";
import DeviceActions from "./DeviceActions.vue";
import { dateTime, latency, typeLabels, telemetryCurrent } from "../devicePolicy";

defineProps({ 
  now: Number, 
  devices: { type: Array, default: () => [] } 
});

defineEmits(["edit", "delete", "profile", "incidents", "select", "upgrade"]);

function getTextColor(val) {
  if (val >= 85) return 'text-rose-400';
  if (val >= 70) return 'text-amber-400';
  return 'text-emerald-400';
}

function getBarColor(val) {
  if (val >= 85) return 'bg-rose-500';
  if (val >= 70) return 'bg-amber-500';
  return 'bg-emerald-500';
}

function getUpgradeStatusLabel(status) {
  const map = {
    pending: 'Kuyrukta',
    downloading: 'İndiriliyor',
    verifying: 'Doğrulanıyor',
    applying: 'Uygulanıyor',
    failed: 'Hata',
    success: 'Güncel'
  };
  return map[status] || status;
}

function getUpgradeBadgeClass(status) {
  switch (status) {
    case 'pending':
    case 'downloading':
    case 'verifying':
    case 'applying':
      return 'bg-blue-500/10 text-blue-400 border-blue-500/20';
    case 'success':
      return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
    case 'failed':
      return 'bg-rose-500/10 text-rose-400 border-rose-500/20';
    default:
      return 'bg-slate-500/10 text-slate-400 border-slate-500/20';
  }
}
</script>