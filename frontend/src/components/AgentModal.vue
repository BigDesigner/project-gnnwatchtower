<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
    <div class="bg-[#141820] border border-[#232936] rounded-lg max-w-2xl w-full flex flex-col shadow-2xl overflow-hidden text-xs">
      <!-- Header -->
      <div class="p-4 border-b border-[#232936] flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-emerald-400">verified_user</span>
          <h3 class="text-sm font-semibold text-white">Ajan Dağıtımı (v{{ APP_VERSION }})</h3>
        </div>
        <button @click="$emit('close')" class="text-txt-muted hover:text-white">
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <!-- Body -->
      <div class="p-5 flex flex-col gap-4 max-h-[80vh] overflow-y-auto">
        <div class="p-3 rounded bg-blue-950/30 border border-blue-900/40 text-blue-300 text-[11px] leading-relaxed flex items-start gap-2.5">
          <span class="material-symbols-outlined text-[18px] text-blue-400 shrink-0 mt-0.5">info</span>
          <div>
            <span class="font-semibold text-white">Ajan Kayıt Akışı:</span>
            Yalnızca mevcut güvenlik politikası kapsamında yetkilendirilmiş Windows/Linux uç noktalarında kullanın. Bu ekran komutları kendiliğinden çalıştırmaz. NAS ve ESXi cihazlarına ajan kurulmaz.
          </div>
        </div>

        <div v-if="enrollmentKey" class="flex items-center justify-between p-2.5 rounded bg-[#0c0e12] border border-[#232936] text-[11px]">
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-[16px] text-amber-400">key</span>
            <span class="text-txt-muted">Ajan Kayıt Anahtarı:</span>
            <span class="font-mono text-txt-primary font-semibold select-all">{{ enrollmentKey }}</span>
          </div>
          <button @click="copyText(enrollmentKey, 'key')" class="text-blue-400 hover:text-blue-300 flex items-center gap-1 text-[11px]">
            <span class="material-symbols-outlined text-[14px]">content_copy</span>
            <span>{{ copied === 'key' ? 'Kopyalandı!' : 'Anahtarı Kopyala' }}</span>
          </button>
        </div>

        <!-- Deployment Method Tabs -->
        <div class="flex items-center gap-1 border-b border-[#232936] text-[11px]">
          <button 
            @click="activeMethod = 'powershell'"
            :class="activeMethod === 'powershell' ? 'bg-[#1e2533] text-txt-primary border-b-2 border-brand-500 font-medium' : 'text-txt-secondary hover:text-txt-primary'"
            class="px-3 py-1.5 rounded-t flex items-center gap-1.5 transition-colors"
          >
            <span class="material-symbols-outlined text-[14px]">desktop_windows</span>
            <span>Windows (PowerShell)</span>
          </button>
          <button 
            @click="activeMethod = 'linux'"
            :class="activeMethod === 'linux' ? 'bg-[#1e2533] text-txt-primary border-b-2 border-brand-500 font-medium' : 'text-txt-secondary hover:text-txt-primary'"
            class="px-3 py-1.5 rounded-t flex items-center gap-1.5 transition-colors"
          >
            <span class="material-symbols-outlined text-[14px]">terminal</span>
            <span>Ubuntu / Debian (Linux)</span>
          </button>
          <button 
            @click="activeMethod = 'gpo'"
            :class="activeMethod === 'gpo' ? 'bg-[#1e2533] text-txt-primary border-b-2 border-brand-500 font-medium' : 'text-txt-secondary hover:text-txt-primary'"
            class="px-3 py-1.5 rounded-t flex items-center gap-1.5 transition-colors"
          >
            <span class="material-symbols-outlined text-[14px]">apps</span>
            <span>GPO / SCCM / Manuel Dağıtım</span>
          </button>
        </div>

        <!-- 1. Windows Server PowerShell Tab -->
        <div v-if="activeMethod === 'powershell'" class="flex flex-col gap-2">
          <div class="flex items-center justify-between text-txt-secondary font-medium">
            <span class="text-txt-muted text-[11px]">Yönetici PowerShell penceresinde çalıştırın:</span>
            <button 
              @click="copyText(windowsCmd, 'win')"
              class="text-blue-400 hover:text-blue-300 flex items-center gap-1"
            >
              <span class="material-symbols-outlined text-[14px]">content_copy</span>
              <span>{{ copied === 'win' ? 'Kopyalandı!' : 'Komutu Kopyala' }}</span>
            </button>
          </div>
          <div class="p-2.5 rounded bg-[#0c0e12] border border-[#232936] font-mono text-[11px] text-txt-primary select-all break-all">
            {{ windowsCmd || installError || 'Kurulum hazırlanıyor…' }}
          </div>
          <p class="text-[10px] text-txt-muted">
            * Dosya tabanlı indirme yürütür, hafıza içi (`iex`) çalıştırmaz ve doğrudan yerel Windows Hizmeti (SCM) olarak kaydeder.
          </p>
        </div>

        <!-- 2. Linux Server Tab -->
        <div v-if="activeMethod === 'linux'" class="flex flex-col gap-2">
          <div class="flex items-center justify-between text-txt-secondary font-medium">
            <span class="text-txt-muted text-[11px]">Linux sunucu terminalinde root/sudo yetkisiyle çalıştırın:</span>
            <button 
              @click="copyText(linuxCmd, 'linux')"
              class="text-blue-400 hover:text-blue-300 flex items-center gap-1"
            >
              <span class="material-symbols-outlined text-[14px]">content_copy</span>
              <span>{{ copied === 'linux' ? 'Kopyalandı!' : 'Komutu Kopyala' }}</span>
            </button>
          </div>
          <div class="p-2.5 rounded bg-[#0c0e12] border border-[#232936] font-mono text-[11px] text-txt-primary select-all break-all">
            {{ linuxCmd || installError || 'Kurulum hazırlanıyor…' }}
          </div>
          <p class="text-[10px] text-txt-muted">
            * `/opt/gnnwatchtower-agent` altına yerleştirilir ve `/etc/systemd/system/gnnwatchtower-agent.service` olarak arka planda başlatılır.
          </p>
        </div>

        <!-- 3. GPO / SCCM / Manual Tab -->
        <div v-if="activeMethod === 'gpo'" class="flex flex-col gap-3">
          <div class="flex items-center gap-2">
            <a 
              :href="`${hostUrl}/api/agent/download/windows`"
              class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded bg-blue-600 hover:bg-blue-500 text-white font-medium shadow"
            >
              <span class="material-symbols-outlined text-[16px]">download</span>
              <span>gnnwatchtower-agent.exe İndir</span>
            </a>
            <a 
              :href="`${hostUrl}/api/agent/download/linux`"
              class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded bg-[#1e2533] hover:bg-[#283142] text-txt-primary border border-[#232936] font-medium"
            >
              <span class="material-symbols-outlined text-[16px]">download</span>
              <span>Linux Ajanını İndir</span>
            </a>
          </div>

          <div class="text-txt-muted text-[11px]">GPO / SCCM dağıtımında Windows sekmesindeki sertifika içeren kurulum komutunu kullanın. Tek başına ikili dosya güven yapılandırmasını içermez.</div>
        </div>
      </div>

      <!-- Footer -->
      <div class="p-4 border-t border-[#232936] bg-[#0c0e12]/60 flex items-center justify-end">
        <button @click="$emit('close')" class="px-4 py-1.5 rounded bg-[#1e2533] hover:bg-[#283142] text-white">
          Kapat
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'
import { APP_VERSION } from '../version'
import api from '../services/api'

const props = defineProps({
  isOpen: Boolean
})

defineEmits(['close'])

const copied = ref('')
const activeMethod = ref('powershell')
const enrollmentKey = ref('')
const linuxScript = ref('')
const windowsScript = ref('')
const installError = ref('')

let requestEpoch = 0
watch(() => props.isOpen, async open => {
 const epoch = ++requestEpoch
 enrollmentKey.value = ''; linuxScript.value = ''; windowsScript.value = ''; installError.value = ''; copied.value = ''; activeMethod.value = 'powershell'
 if (!open) return
 try {
  const [settings, linux, windows] = await Promise.all([api.getSettings(), api.getLinuxAgentScript(), api.getWindowsAgentScript()])
  if (epoch !== requestEpoch || !props.isOpen) return
  enrollmentKey.value = settings.agent_enrollment_key || ''; linuxScript.value=linux; windowsScript.value=windows
 } catch(err) { if(epoch===requestEpoch && props.isOpen) installError.value=err.response?.data?.error || 'Kurulum betiği hazırlanamadı.' }

}, { immediate: true })
onUnmounted(() => { requestEpoch++; enrollmentKey.value = ''; linuxScript.value=''; windowsScript.value=''; copied.value = '' })

const safeEnrollKey = computed(() => {
  const k = (enrollmentKey.value || '').trim()
  if (/^[a-zA-Z0-9_\-]{16,64}$/.test(k)) {
    return k
  }
  return ''
})

const safeHostUrl = computed(() => {
  const raw = window.location.origin || 'http://127.0.0.1:3800'
  return raw.replace(/[^a-zA-Z0-9.:\-_/\[\]]/g, '')
})

const hostUrl = safeHostUrl

function encoded(text) { return btoa(Array.from(new TextEncoder().encode(text), byte=>String.fromCharCode(byte)).join('')) }
const linuxCmd = computed(() => linuxScript.value ? "printf '%s' '"+encoded(linuxScript.value)+"' | base64 -d | sudo bash" : '')
const windowsCmd = computed(() => windowsScript.value ? "[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('"+encoded(windowsScript.value)+"')) | Invoke-Expression" : '')

async function copyText(text, type) {
  if (!text) return
  let success = false
  try { await navigator.clipboard.writeText(text); success = true } catch {
    const area = document.createElement('textarea')
    area.value = text; area.style.position = 'fixed'; area.style.opacity = '0'
    document.body.appendChild(area); area.select()
    try { success = document.execCommand('copy') } finally { area.remove() }
  }
  if (success) { copied.value = type; setTimeout(() => { if (copied.value === type) copied.value = '' }, 2000) }
}
</script>
