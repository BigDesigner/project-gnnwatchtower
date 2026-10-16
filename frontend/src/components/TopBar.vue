<script setup>
import { APP_VERSION } from "../version";
defineProps({ lastRefresh: String, refreshing: Boolean });
defineEmits(["refresh", "add-device", "change-password", "logout"]);
</script>
<template>
  <header class="wt-topbar">
    <a href="#" @click.prevent class="wt-brand">
      <span class="material-symbols-outlined">shield</span>
      GNNwatchtower
      <small>v{{ APP_VERSION }}</small>
    </a>
    <div class="wt-toolbar">
      <span class="wt-muted wt-refresh-time">Son veri: {{ lastRefresh || 'Bekleniyor' }}</span>
      <button @click="$emit('refresh')" :disabled="refreshing" aria-label="Verileri yenile">
        <span class="material-symbols-outlined">refresh</span>
      </button>
      <button @click="$emit('add-device')" class="wt-primary">Cihaz Ekle</button>
      <button @click="$emit('change-password')" aria-label="Parolayı değiştir">
        <span class="material-symbols-outlined">key</span>
      </button>
      <button @click="$emit('logout')">Çıkış</button>
    </div>
  </header>
</template>
