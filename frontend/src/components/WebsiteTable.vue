<template>
  <div class="overflow-x-auto">
    <table class="w-full text-left border-collapse text-xs">
      <thead>
        <tr class="border-b border-[#232936] bg-[#0c0e12]/60 text-txt-muted text-[11px] uppercase tracking-wider font-semibold select-none">
          <th class="py-3 px-3 w-12 font-mono text-center">ID</th>
          <th class="py-3 px-4">Web Sitesi / Başlık</th>
          <th class="py-3 px-4">Hedef URL</th>
          <th class="py-3 px-3 text-center">HTTP Durumu</th>
          <th class="py-3 px-3 text-right">Yanıt Süresi</th>
          <th class="py-3 px-3 text-center">Kontrol Aralığı</th>
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

          <!-- Web Sitesi & Başlık -->
          <td class="py-3 px-4">
            <div class="flex items-center gap-2.5">
              <div class="w-7 h-7 rounded-md bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center shrink-0 text-emerald-400">
                <span class="material-symbols-outlined text-[16px]">language</span>
              </div>
              <div class="flex flex-col">
                <span class="font-medium text-white group-hover:text-emerald-400 transition-colors">
                  {{ d.name }}
                </span>
                <span v-if="d.purpose || d.location" class="text-[11px] text-txt-muted">
                  {{ d.purpose || d.location }}
                </span>
              </div>
            </div>
          </td>

          <!-- Hedef URL -->
          <td class="py-3 px-4 font-mono" @click.stop>
            <a 
              :href="d.ip_address" 
              target="_blank" 
              rel="noopener noreferrer"
              class="text-emerald-400 hover:text-emerald-300 hover:underline inline-flex items-center gap-1.5 truncate max-w-[260px]"
              title="Yeni sekmede aç"
            >
              <span class="material-symbols-outlined text-[14px] text-txt-muted">
                {{ d.ip_address && d.ip_address.startsWith('https') ? 'lock' : 'public' }}
              </span>
              <span class="truncate">{{ d.ip_address }}</span>
              <span class="material-symbols-outlined text-[12px] opacity-70 group-hover:opacity-100">open_in_new</span>
            </a>
          </td>

          <!-- HTTP Durumu -->
          <td class="py-3 px-3 text-center">
            <span 
              v-if="d.operator_notes"
              class="px-2 py-0.5 rounded text-[11px] font-mono border"
              :class="d.status === 'online' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border-rose-500/20'"
            >
              {{ d.operator_notes }}
            </span>
            <span v-else class="text-txt-muted font-mono text-[11px]">
              {{ d.status === 'online' ? 'HTTP 200' : '—' }}
            </span>
          </td>

          <!-- Yanıt Süresi -->
          <td class="py-3 px-3 text-right font-mono">
            <span v-if="typeof d.last_probe_latency_ms === 'number' && d.status === 'online'" class="text-txt-primary">
              {{ d.last_probe_latency_ms.toFixed(1) }} ms
            </span>
            <span v-else class="text-txt-muted">—</span>
          </td>

          <!-- Kontrol Aralığı -->
          <td class="py-3 px-3 text-center font-mono text-txt-muted">
            {{ d.check_interval_seconds || 15 }}s
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
import { dateTime } from "../devicePolicy";

defineProps({ 
  now: Number, 
  devices: { type: Array, default: () => [] } 
});

defineEmits(["edit", "delete", "profile", "incidents"]);
</script>