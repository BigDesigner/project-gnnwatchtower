<script setup>
defineProps({ device: Object });
defineEmits(["edit", "delete", "profile", "incidents"]);
</script>

<template>
  <div class="flex items-center gap-1">
    <!-- Edit -->
    <button 
      @click.stop="$emit('edit', device)" 
      class="p-1 rounded text-txt-muted hover:text-txt-primary hover:bg-[#1d2432] transition-colors"
      :title="device.name + ' yapılandırmasını düzenle'"
      aria-label="Düzenle"
    >
      <span class="material-symbols-outlined text-[16px]">edit</span>
    </button>

    <!-- Incidents -->
    <button 
      @click.stop="$emit('incidents', device)" 
      class="p-1 rounded text-txt-muted hover:text-amber-400 hover:bg-[#1d2432] transition-colors"
      title="Olay geçmişini görüntüle"
      aria-label="Olaylar"
    >
      <span class="material-symbols-outlined text-[16px]">history</span>
    </button>

    <!-- Maintenance / Profile Toggle -->
    <button 
      @click.stop="$emit('profile', device)" 
      class="p-1 rounded text-txt-muted hover:text-blue-400 hover:bg-[#1d2432] transition-colors"
      :title="['paused','maintenance'].includes(device.monitoring_profile) ? 'İzlemeyi tekrar başlat' : 'Planlı bakıma al'"
      aria-label="Bakım Durumu"
    >
      <span class="material-symbols-outlined text-[16px]">
        {{ ['paused','maintenance'].includes(device.monitoring_profile) ? 'play_circle' : 'pause_circle' }}
      </span>
    </button>

    <!-- Delete -->
    <button 
      @click.stop="$emit('delete', device)" 
      class="p-1 rounded text-txt-muted hover:text-rose-400 hover:bg-rose-950/30 transition-colors"
      :title="device.name + ' kaydını sil'"
      aria-label="Sil"
    >
      <span class="material-symbols-outlined text-[16px]">delete</span>
    </button>
  </div>
</template>
