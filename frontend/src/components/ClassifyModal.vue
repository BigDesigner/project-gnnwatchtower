<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
    <div class="bg-[#141820] border border-[#232936] rounded-lg max-w-lg w-full flex flex-col shadow-2xl overflow-hidden text-xs">
      <!-- Header -->
      <div class="p-4 border-b border-[#232936] flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-amber-400">report_problem</span>
          <h3 class="text-sm font-semibold text-white">Kesinti Sınıflandırma (Post-Mortem)</h3>
        </div>
        <button @click="$emit('close')" class="text-txt-muted hover:text-white">
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <!-- Body -->
      <div class="p-5 flex flex-col gap-4">
        <div v-if="incident" class="p-3 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-1 text-[11px]">
          <div class="flex items-center justify-between">
            <span class="text-txt-muted">Cihaz ID: #{{ incident.device_id }}</span>
            <span class="font-mono text-txt-secondary">{{ formatDate(incident.started_at) }}</span>
          </div>
          <span v-if="incident.auto_detection_hypothesis" class="text-amber-400">
            Otomatik Tespit: {{ incident.auto_detection_hypothesis }}
          </span>
        </div>

        <div class="flex flex-col gap-2">
          <label class="text-txt-muted font-medium">Kesinti Sebebi *</label>
          <div class="grid grid-cols-2 gap-2">
            <label 
              v-for="cause in causes" 
              :key="cause.id"
              :class="selectedCause === cause.id ? 'bg-blue-600/20 border-blue-500 text-white' : 'bg-[#0c0e12] border-[#232936] text-txt-secondary'"
              class="border rounded p-2.5 flex items-center gap-2 cursor-pointer hover:border-slate-600 transition-colors"
            >
              <input type="radio" v-model="selectedCause" :value="cause.id" class="hidden">
              <span class="material-symbols-outlined text-[16px]">{{ cause.icon }}</span>
              <span>{{ cause.label }}</span>
            </label>
          </div>
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-txt-muted font-medium">Operatör İnceleme Notu</label>
          <textarea 
            v-model="notes" 
            rows="3" 
            placeholder="Kesinti nedeni, uygulanan müdahale ve müşteri raporuna girecek açıklama..."
            class="bg-[#0c0e12] border border-[#232936] rounded p-2.5 text-txt-primary focus:outline-none focus:border-blue-500"
          ></textarea>
        </div>
      </div>

      <!-- Footer -->
      <div class="p-4 border-t border-[#232936] bg-[#0c0e12]/60 flex items-center justify-end gap-2">
        <button @click="$emit('close')" class="px-3 py-1.5 rounded text-txt-secondary hover:text-white">
          İptal
        </button>
        <button 
          @click="submitClassification" 
          :disabled="isSubmitting"
          class="px-4 py-1.5 rounded font-medium bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white"
        >
          {{ isSubmitting ? 'Kaydediliyor...' : 'Doğrula ve Kaydet' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import api from '../services/api'

const props = defineProps({
  isOpen: Boolean,
  incident: Object
})

const emit = defineEmits(['close', 'classified'])

const selectedCause = ref('power_outage')
const notes = ref('')
const isSubmitting = ref(false)

const causes = [
  { id: 'power_outage', label: 'Şebeke / Elektrik', icon: 'bolt' },
  { id: 'maintenance', label: 'Planlı Bakım', icon: 'build' },
  { id: 'hardware_crash', label: 'Donanım / Çökme', icon: 'memory' },
  { id: 'network_issue', label: 'Ağ / Kablo', icon: 'lan' }
]

watch(() => props.incident, (inc) => {
  if (inc) {
    selectedCause.value = inc.classified_cause === 'unclassified' ? 'power_outage' : inc.classified_cause
    notes.value = inc.operator_notes || ''
  }
}, { immediate: true })

const submitClassification = async () => {
  if (!props.incident) return
  isSubmitting.value = true
  try {
    await api.classifyIncident(props.incident.id, {
      cause: selectedCause.value,
      notes: notes.value
    })
    emit('classified')
    emit('close')
  } catch (err) {
    alert('Sınıflandırma hatası: ' + (err.response?.data?.error || err.message))
  } finally {
    isSubmitting.value = false
  }
}

const formatDate = (dateStr) => {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('tr-TR')
}
</script>
