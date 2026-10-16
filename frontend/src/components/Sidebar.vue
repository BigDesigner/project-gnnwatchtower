<script setup>
import { sections } from "../devicePolicy";
defineProps({ currentView: String, counts: { type: Object, default: () => ({}) } });
defineEmits(["select-view", "open-settings"]);
</script>
<template>
  <aside class="wt-sidebar">
    <div class="wt-nav-caption">İZLEME MERKEZİ</div>
    <nav aria-label="Ana menü">
      <button v-for="section in sections" :key="section.id" :aria-label="section.label" :title="section.label" @click="$emit('select-view', section.id)" :aria-current="currentView === section.id ? 'page' : undefined" :class="{ active: currentView === section.id }">
        <span class="material-symbols-outlined">{{ section.icon }}</span>
        <span>{{ section.label }}</span>
        <small v-if="section.mode">{{ counts[section.mode] || 0 }}</small>
      </button>
    </nav>
    <div class="wt-sidebar-bottom">
      <button @click="$emit('open-settings')">
        <span class="material-symbols-outlined">tune</span>
        Sistem Ayarları
      </button>
      <p>
        Kayıtlı hedefler, sınırlı izleme.
        <br>
        Ek yönetim servisi açılmaz.
      </p>
    </div>
  </aside>
</template>
