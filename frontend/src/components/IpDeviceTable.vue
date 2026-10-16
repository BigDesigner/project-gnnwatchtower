<template>
  <div class="overflow-x-auto">
    <table class="w-full text-left border-collapse text-xs">
      <thead>
        <tr class="border-b border-[#232936] bg-[#0c0e12]/60 text-txt-muted text-[11px] uppercase tracking-wider font-semibold select-none">
          <th class="py-3 px-3 w-12 font-mono text-center">ID</th>
          <th class="py-3 px-4">Cihaz Adı & Tanım</th>
          <th class="py-3 px-4">IP Adresi</th>
          <th class="py-3 px-3 text-center">Donanım Türü</th>
          <th class="py-3 px-3 text-right">ICMP Ping Gecikmesi</th>
          <th class="py-3 px-4 text-center">Durum</th>
          <th class="py-3 px-4 text-right">Son Kontrol</th>
          <th class="py-3 px-4 text-right">İşlemler</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-[#1e2533]">
        <tr 
          v-for="d in devices" 
          :key="d.id"
          class="hover:bg-[#181e28]/70 transition-colors group cursor-pointer"
          @click="$emit('incidents', d)"
        >
          <!-- ID -->
          <td class="py-3 px-3 font-mono text-txt-muted text-[11px] text-center">
            #{{ d.id }}
          </td>

          <!-- Cihaz Adı & Tanım -->
          <td class="py-3 px-4">
            <div class="flex items-center gap-2.5">
              <div class="w-7 h-7 rounded-md bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center shrink-0 text-cyan-400">
                <span class="material-symbols-outlined text-[16px]">
                  {{ getDeviceIcon(d.device_type) }}
                </span>
              </div>
              <div class="flex flex-col">
                <span class="font-medium text-white group-hover:text-cyan-400 transition-colors">
                  {{ d.name }}
                </span>
                <span v-if="d.location || d.purpose" class="text-[11px] text-txt-muted">
                  {{ d.location || d.purpose }}
                </span>
              </div>
            </div>
          </td>

          <!-- IP Adresi -->
          <td class="py-3 px-4 font-mono text-txt-primary">
            {{ d.ip_address }}
          </td>

          <!-- Donanım Türü -->
          <td class="py-3 px-3 text-center">
            <span class="px-2 py-0.5 rounded text-[11px] font-medium bg-[#1a202c] border border-[#283142] text-txt-secondary">
              {{ typeLabels[d.device_type] || d.device_type }}
            </span>
          </td>

          <!-- ICMP Ping Gecikmesi -->
          <td class="py-3 px-3 text-right font-mono">
            <span v-if="d.status === 'online' && typeof d.last_probe_latency_ms === 'number'" class="text-txt-primary font-medium">
              {{ d.last_probe_latency_ms.toFixed(1) }} ms
            </span>
            <span v-else class="text-txt-muted">—</span>
          </td>

          <!-- Durum -->
          <td class="py-3 px-4 text-center">
            <DeviceStatus :device="d" :now="now" />
          </td>

          <!-- Son Kontrol -->
          <td class="py-3 px-4 text-right text-txt-secondary">
            {{ dateTime(d.last_checked_at) }}
          </td>

          <!-- İşlemler -->
          <td class="py-3 px-4 text-right" @click.stop>
            <DeviceActions 
              :device="d" 
              @edit="$emit('edit', $event)" 
              @delete="$emit('delete', $event)" 
              @profile="$emit('profile', $event)" 
              @incidents="$emit('incidents', $event)" 
            />
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import DeviceStatus from "./DeviceStatus.vue";
import DeviceActions from "./DeviceActions.vue";
import { dateTime, typeLabels } from "../devicePolicy";

defineProps({ 
  now: Number, 
  devices: { type: Array, default: () => [] } 
});

defineEmits(["edit", "delete", "profile", "incidents"]);

function getDeviceIcon(type) {
  if (['nas', 'synology', 'qnap', 'storage', 'truenas'].includes(type)) return 'database';
  if (['esxi', 'esx'].includes(type)) return 'lan';
  if (type === 'switch') return 'hub';
  if (type === 'router') return 'router';
  return 'memory';
}
</script>