<script setup>
defineProps({ device: Object });
defineEmits(["edit", "delete", "profile", "incidents"]);
</script>
<template>
  <div class="wt-row-actions">
    <button @click="$emit('edit',device)" :aria-label="device.name + ' düzenle'">Düzenle</button>
    <button @click="$emit('incidents',device)">Olaylar</button>
    <button @click="$emit('profile',device)" :title="['paused','maintenance'].includes(device.monitoring_profile) ? 'İzlemeyi sürdür' : 'Bakıma al'">{{ ['paused','maintenance'].includes(device.monitoring_profile) ? 'Sürdür' : 'Bakım' }}</button>
    <button class="wt-danger" @click="$emit('delete',device)" :aria-label="device.name + ' sil'">Sil</button>
  </div>
</template>
