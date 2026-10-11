<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
    <div class="bg-[#141820] border border-[#232936] rounded-lg max-w-2xl w-full flex flex-col shadow-2xl overflow-hidden text-xs">
      <!-- Header -->
      <div class="p-4 border-b border-[#232936] flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-txt-secondary">tune</span>
          <h3 class="text-sm font-semibold text-white">Sistem Yapılandırması ve Yedekleme</h3>
        </div>
        <button @click="$emit('close')" class="text-txt-muted hover:text-white">
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <!-- Navigation Tabs -->
      <div class="flex items-center gap-1 px-4 pt-3 border-b border-[#232936] bg-[#0c0e12]/40">
        <button 
          @click="activeTab = 'general'"
          :class="activeTab === 'general' ? 'border-b-2 border-brand-500 text-white font-medium' : 'text-txt-secondary hover:text-white'"
          class="px-3 py-1.5 transition-colors"
        >
          Genel & Lokasyon
        </button>
        <button 
          @click="activeTab = 'alerts'"
          :class="activeTab === 'alerts' ? 'border-b-2 border-brand-500 text-white font-medium' : 'text-txt-secondary hover:text-white'"
          class="px-3 py-1.5 transition-colors"
        >
          Bildirim Kanalları
        </button>
        <button 
          @click="activeTab = 'backup'"
          :class="activeTab === 'backup' ? 'border-b-2 border-brand-500 text-white font-medium' : 'text-txt-secondary hover:text-white'"
          class="px-3 py-1.5 transition-colors"
        >
          Yedekleme & Geri Yükleme
        </button>
      </div>

      <!-- Content -->
      <div class="p-5 max-h-[60vh] overflow-y-auto">
        <!-- Tab 1: General -->
        <div v-if="activeTab === 'general'" class="flex flex-col gap-4">
          <div class="flex flex-col gap-1">
            <label class="text-txt-muted font-medium">Müşteri Şirket Adı</label>
            <input 
              v-model="settings.client_company_name" 
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-2 text-txt-primary focus:outline-none focus:border-blue-500"
            />
          </div>
          <div class="flex flex-col gap-1">
            <label class="text-txt-muted font-medium">Lokasyon / Sistem Odası</label>
            <input 
              v-model="settings.client_location" 
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-2 text-txt-primary focus:outline-none focus:border-blue-500"
            />
          </div>
          <div class="flex flex-col gap-1">
            <label class="text-txt-muted font-medium">Varsayılan Ağ Geçidi (Gateway IP)</label>
            <input 
              v-model="settings.default_gateway" 
              placeholder="192.168.1.1"
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-2 text-txt-primary font-mono focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        <!-- Tab 2: Alerts -->
        <div v-if="activeTab === 'alerts'" class="flex flex-col gap-5">
          <!-- Telegram -->
          <div class="p-3 rounded border border-[#232936] bg-[#0c0e12]/60 flex flex-col gap-2.5">
            <div class="flex items-center justify-between">
              <span class="font-medium text-white flex items-center gap-1.5">
                <span class="material-symbols-outlined text-[16px] text-sky-400">send</span>
                Telegram Bot
              </span>
              <label class="flex items-center gap-1 cursor-pointer">
                <input type="checkbox" v-model="settings.telegram_enabled" true-value="true" false-value="false" class="rounded">
                <span class="text-txt-secondary text-[11px]">Aktif</span>
              </label>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <input 
                v-model="settings.telegram_bot_token" 
                placeholder="Bot Token" 
                class="bg-[#141820] border border-[#232936] rounded px-2.5 py-1.5 text-txt-primary font-mono"
              />
              <input 
                v-model="settings.telegram_chat_id" 
                placeholder="Chat ID" 
                class="bg-[#141820] border border-[#232936] rounded px-2.5 py-1.5 text-txt-primary font-mono"
              />
            </div>
          </div>

          <!-- Discord -->
          <div class="p-3 rounded border border-[#232936] bg-[#0c0e12]/60 flex flex-col gap-2.5">
            <div class="flex items-center justify-between">
              <span class="font-medium text-white flex items-center gap-1.5">
                <span class="material-symbols-outlined text-[16px] text-indigo-400">chat</span>
                Discord Webhook
              </span>
              <label class="flex items-center gap-1 cursor-pointer">
                <input type="checkbox" v-model="settings.discord_enabled" true-value="true" false-value="false" class="rounded">
                <span class="text-txt-secondary text-[11px]">Aktif</span>
              </label>
            </div>
            <input 
              v-model="settings.discord_webhook_url" 
              placeholder="https://discord.com/api/webhooks/..." 
              class="bg-[#141820] border border-[#232936] rounded px-2.5 py-1.5 text-txt-primary font-mono"
            />
          </div>

          <!-- Brevo -->
          <div class="p-3 rounded border border-[#232936] bg-[#0c0e12]/60 flex flex-col gap-2.5">
            <div class="flex items-center justify-between">
              <span class="font-medium text-white flex items-center gap-1.5">
                <span class="material-symbols-outlined text-[16px] text-emerald-400">mail</span>
                Brevo (Sendinblue) REST API (HTTPS Port 443)
              </span>
              <label class="flex items-center gap-1 cursor-pointer">
                <input type="checkbox" v-model="settings.brevo_enabled" true-value="true" false-value="false" class="rounded">
                <span class="text-txt-secondary text-[11px]">Aktif</span>
              </label>
            </div>
            <input 
              v-model="settings.brevo_api_key" 
              type="password"
              placeholder="Brevo API Anahtarı (xkeysib-...)" 
              class="bg-[#141820] border border-[#232936] rounded px-2.5 py-1.5 text-txt-primary font-mono"
            />
            <div class="grid grid-cols-2 gap-2">
              <input 
                v-model="settings.brevo_sender_email" 
                placeholder="Gönderici E-posta" 
                class="bg-[#141820] border border-[#232936] rounded px-2.5 py-1.5 text-txt-primary"
              />
              <input 
                v-model="settings.brevo_recipient_email" 
                placeholder="Bildirim Alıcı E-posta" 
                class="bg-[#141820] border border-[#232936] rounded px-2.5 py-1.5 text-txt-primary"
              />
            </div>
          </div>
        </div>

        <!-- Tab 3: Backup & Portability -->
        <div v-if="activeTab === 'backup'" class="flex flex-col gap-4">
          <div class="p-4 rounded border border-[#232936] bg-[#0c0e12]/60 flex flex-col gap-3">
            <h4 class="font-medium text-white">Tek Tıkla Web Yedekleme</h4>
            <p class="text-txt-muted leading-relaxed">
              Tüm cihazları, port izleme ayarlarını, bildirim kanallarını ve kesinti kayıtlarını içeren tek bir JSON dosyası indirin.
            </p>
            <div>
              <button 
                @click="downloadBackup" 
                class="inline-flex items-center gap-1.5 px-4 py-2 rounded bg-blue-600 hover:bg-blue-500 text-white font-medium"
              >
                <span class="material-symbols-outlined text-[16px]">download</span>
                <span>Yedek Dosyası İndir (.json)</span>
              </button>
            </div>
          </div>

          <div class="p-4 rounded border border-[#232936] bg-[#0c0e12]/60 flex flex-col gap-3">
            <h4 class="font-medium text-white">Yedekten Geri Yükleme</h4>
            <p class="text-txt-muted leading-relaxed">
              Önceden indirilmiş bir GNNwatchtower yedek JSON dosyasını sisteme yükleyerek 5 saniyede geri yükleyin.
            </p>
            <input 
              type="file" 
              accept=".json"
              @change="handleFileRestore" 
              class="block w-full text-xs text-txt-muted file:mr-4 file:py-2 file:px-4 file:rounded file:border-0 file:text-xs file:font-semibold file:bg-[#1e2533] file:text-white hover:file:bg-[#2c3547]"
            />
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="p-4 border-t border-[#232936] bg-[#0c0e12]/60 flex items-center justify-end gap-2">
        <button @click="$emit('close')" class="px-3 py-1.5 rounded text-txt-secondary hover:text-white">
          İptal
        </button>
        <button 
          v-if="activeTab !== 'backup'"
          @click="saveSettings" 
          :disabled="isSaving"
          class="px-4 py-1.5 rounded font-medium bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white"
        >
          {{ isSaving ? 'Kaydediliyor...' : 'Ayarları Kaydet' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../services/api'

const props = defineProps({
  isOpen: Boolean
})

const emit = defineEmits(['close', 'saved'])

const activeTab = ref('general')
const isSaving = ref(false)

const settings = ref({
  client_company_name: '',
  client_location: '',
  default_gateway: '',
  telegram_bot_token: '',
  telegram_chat_id: '',
  telegram_enabled: 'false',
  discord_webhook_url: '',
  discord_enabled: 'false',
  brevo_api_key: '',
  brevo_sender_email: '',
  brevo_recipient_email: '',
  brevo_enabled: 'false'
})

const loadSettings = async () => {
  try {
    const data = await api.getSettings()
    if (data && typeof data === 'object') {
      if (Array.isArray(data)) {
        data.forEach(item => {
          if (item && item.key) {
            settings.value[item.key] = String(item.value ?? '')
          }
        })
      } else {
        Object.keys(data).forEach(k => {
          if (!k.match(/^\d+$/) && data[k] !== undefined && data[k] !== null) {
            settings.value[k] = String(data[k])
          }
        })
      }
    }
  } catch (err) {
    console.error('Ayarlar yuklenemedi:', err)
  }
}

onMounted(() => {
  loadSettings()
})

const saveSettings = async () => {
  isSaving.value = true
  try {
    const payload = {}
    for (const [k, v] of Object.entries(settings.value)) {
      if (!k.match(/^\d+$/) && v !== undefined && v !== null) {
        payload[k] = String(v)
      }
    }
    await api.updateSettings(payload)
    emit('saved')
    emit('close')
  } catch (err) {
    alert('Ayar kaydetme hatası: ' + (err.response?.data?.error || err.message))
  } finally {
    isSaving.value = false
  }
}

const downloadBackup = async () => {
  try {
    const bundle = await api.exportBackup()
    const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `gnnwatchtower-backup-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
  } catch (err) {
    alert('Yedek alma hatası: ' + err.message)
  }
}

const handleFileRestore = (e) => {
  const file = e.target.files[0]
  if (!file) return

  const reader = new FileReader()
  reader.onload = async (evt) => {
    try {
      const bundle = JSON.parse(evt.target.result)
      await api.importBackup(bundle)
      alert('Yedek başarıyla geri yüklendi!')
      emit('saved')
      emit('close')
    } catch (err) {
      alert('Geri yükleme hatası: ' + (err.response?.data?.error || err.message))
    }
  }
  reader.readAsText(file)
}
</script>
