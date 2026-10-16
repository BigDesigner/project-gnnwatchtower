<script setup>
import { observation, stateLabels } from "../devicePolicy";
defineProps({ now: Number, device: { type: Object, required: true } });
</script>
<template>
  <span class="wt-status" :class="'wt-status-' + observation(device,now)" :title="device.probe_error || stateLabels[observation(device,now)]">
    <i></i>
    {{ stateLabels[observation(device,now)] }}
  </span>
</template>
