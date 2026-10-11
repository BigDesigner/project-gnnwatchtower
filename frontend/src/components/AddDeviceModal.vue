<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
    <div class="bg-[#141820] border border-[#232936] rounded-lg max-w-lg w-full flex flex-col shadow-2xl overflow-hidden text-xs">
      <!-- Header -->
      <div class="p-4 border-b border-[#232936] flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span 
            class="material-symbols-outlined" 
            :class="form.device_type === 'website' ? 'text-emerald-400' : 'text-blue-400'"
          >
            {{ form.device_type === 'website' ? 'language' : 'dns' }}
          </span>
          <h3 class="text-sm font-semibold text-white">
            {{ editDevice ? (form.device_type === 'website' ? 'Web Sitesini Düzenle' : 'Cihazı Düzenle') : (form.device_type === 'website' ? '🌐 Yeni Web Sitesi / URL Ekle (HTTP/HTTPS)' : 'Yeni Cihaz Ekle') }}
          </h3>
        </div>
        <button @click="$emit('close')" class="text-txt-muted hover:text-white">
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <!-- Form -->
      <form @submit.prevent="saveDevice" class="p-5 flex flex-col gap-3.5">
        <div class="flex flex-col gap-1">
          <label class="text-txt-muted font-medium">
            {{ form.device_type === 'website' ? 'Web Sitesi Adı / Başlığı *' : 'Cihaz Adı *' }}
          </label>
          <input 
            v-model="form.name" 
            required 
            :placeholder="form.device_type === 'website' ? 'Örn: Kurumsal Web Sitesi veya API Sunucusu' : 'Örn: ESXi-Cluster-01 veya core-switch'"
            class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-txt-primary focus:outline-none focus:border-blue-500"
          />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div class="flex flex-col gap-1">
            <label class="text-txt-muted font-medium">
              {{ form.device_type === 'website' ? 'Web Sitesi URL / Host *' : 'IP Adresi *' }}
            </label>
            <input 
              v-model="form.ip_address" 
              required 
              :placeholder="form.device_type === 'website' ? 'https://example.com' : '192.168.1.50'"
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-txt-primary font-mono focus:outline-none focus:border-blue-500"
            />
          </div>
          <div class="flex flex-col gap-1">
            <label class="text-txt-muted font-medium">Cihaz / Servis Tipi</label>
            <select 
              v-model="form.device_type" 
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-txt-primary focus:outline-none focus:border-blue-500"
            >
              <option value="website">🌐 Web Sitesi / URL (HTTP/HTTPS)</option>
              <option value="linux">Ubuntu / Linux Sunucu</option>
              <option value="windows">Windows Server</option>
              <option value="esxi">VMware ESXi Hipervizör</option>
              <option value="nas">Ağ Depolama (NAS - QNAP / Synology / TrueNAS / Asustor)</option>
              <option value="switch">Omurga Switch / Router</option>
              <option value="other">Diğer Donanım</option>
            </select>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div class="flex flex-col gap-1">
            <label class="text-txt-muted font-medium">İzleme Profili</label>
            <select 
              v-model="form.monitoring_profile" 
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-txt-primary focus:outline-none focus:border-blue-500"
            >
              <option value="active">Aktif İzleme (15s)</option>
              <option value="dormant_24h">24 Saatlik Uyku (Soğuk Yedek)</option>
              <option value="dormant_30d">30 Günlük Uyku (Arşiv)</option>
              <option value="maintenance">Planlı Bakım Modu</option>
            </select>
          </div>
          <div class="flex flex-col gap-1">
            <label class="text-txt-muted font-medium">Yoklama Aralığı (Saniye)</label>
            <input 
              v-model.number="form.check_interval_seconds" 
              type="number"
              min="5"
              max="3600"
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-txt-primary font-mono focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-txt-muted font-medium">Kullanım Amacı / Rol</label>
          <input 
            v-model="form.purpose" 
            placeholder="Örn: Kubernetes Worker Node veya Oracle DB"
            class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-txt-primary focus:outline-none focus:border-blue-500"
          />
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-txt-muted font-medium">Lokasyon / Raf</label>
          <input 
            v-model="form.location" 
            placeholder="Örn: Kabin A - U12"
            class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-txt-primary focus:outline-none focus:border-blue-500"
          />
        </div>

        <!-- Footer -->
        <div class="pt-4 border-t border-[#232936] flex items-center justify-end gap-2 mt-2">
          <button 
            type="button" 
            @click="$emit('close')" 
            class="px-3 py-1.5 rounded text-txt-secondary hover:text-white"
          >
            İptal
          </button>
          <button 
            type="submit" 
            :disabled="isSaving"
            class="px-4 py-1.5 rounded font-medium bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white"
          >
            {{ isSaving ? 'Kaydediliyor...' : 'Kaydet' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import api from '../services/api'

const props = defineProps({
  isOpen: Boolean,
  editDevice: Object,
  initialType: {
    type: String,
    default: 'linux'
  }
})

const emit = defineEmits(['close', 'saved'])

const isSaving = ref(false)

const form = ref({
  name: '',
  ip_address: '',
  device_type: 'linux',
  monitoring_profile: 'active',
  check_interval_seconds: 15,
  purpose: '',
  location: ''
})

watch([() => props.editDevice, () => props.isOpen, () => props.initialType], ([dev, open, initType]) => {
  if (dev) {
    form.value = { ...dev }
  } else {
    form.value = {
      name: '',
      ip_address: initType === 'website' ? 'https://' : '',
      device_type: initType || 'linux',
      monitoring_profile: 'active',
      check_interval_seconds: 15,
      purpose: initType === 'website' ? 'Web Sitesi / URL İzleme' : '',
      location: ''
    }
  }
}, { immediate: true })

watch(() => form.value.device_type, (newType) => {
  if (newType === 'website' && (!form.value.ip_address || form.value.ip_address === '')) {
    form.value.ip_address = 'https://'
  }
})

const saveDevice = async () => {
  isSaving.value = true
  try {
    if (props.editDevice && props.editDevice.id) {
      await api.updateDevice(props.editDevice.id, form.value)
    } else {
      await api.createDevice(form.value)
    }
    emit('saved')
    emit('close')
  } catch (err) {
    alert('Kayıt hatası: ' + (err.response?.data?.error || err.message))
  } finally {
    isSaving.value = false
  }
}
</script>
