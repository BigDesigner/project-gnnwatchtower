<script setup>
import { computed } from "vue";
import { observation, stateLabels } from "../devicePolicy";

const props = defineProps({ 
  now: Number, 
  device: { type: Object, required: true } 
});

const state = computed(() => observation(props.device, props.now));
const label = computed(() => props.device.monitoring_mode === 'icmp' && state.value === 'offline' ? 'ICMP ile Ulaşılamıyor' : stateLabels[state.value] || 'Bilinmiyor');

const badgeClass = computed(() => {
  switch (state.value) {
    case 'online':
      return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/25';
    case 'offline':
      return 'bg-rose-500/10 text-rose-400 border-rose-500/25';
    case 'maintenance':
    case 'paused':
      return 'bg-amber-500/10 text-amber-400 border-amber-500/25';
    default:
      return 'bg-slate-500/10 text-slate-400 border-slate-500/25';
  }
});

const dotClass = computed(() => {
  switch (state.value) {
    case 'online':
      return 'bg-emerald-400 led-pulse-emerald';
    case 'offline':
      return 'bg-rose-400 led-pulse-rose';
    case 'maintenance':
    case 'paused':
      return 'bg-amber-400';
    default:
      return 'bg-slate-400';
  }
});
</script>

<template>
  <span 
    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] font-medium border whitespace-nowrap shadow-sm"
    :class="badgeClass"
    :title="device.probe_error || label"
  >
    <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="dotClass"></span>
    <span>{{ label }}</span>
  </span>
</template>
