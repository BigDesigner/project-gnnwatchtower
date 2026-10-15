<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
    <div class="bg-[#141820] border border-[#232936] rounded-lg max-w-2xl w-full flex flex-col shadow-2xl overflow-hidden text-xs">
      <!-- Header -->
      <div class="p-4 border-b border-[#232936] flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-emerald-400">verified_user</span>
          <h3 class="text-sm font-semibold text-white">Kurumsal Telemetri Ajanı Dağıtımı (v0.4.0)</h3>
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
            <span class="font-semibold text-white">Wazuh Tarzı Otomatik Kayıt (Zero-Touch Auto-Enrollment):</span>
            Ajan uç noktaya kurulduktan sonra donanım parmak izi ile merkeze otomatik tescil edilir ve şifreli yerel anahtar üretilir. Komut satırında veya betiklerde gizli token bulunmaz; EDR/XDR sistemleri tarafından güvenle karşılanır.
          </div>
        </div>

        <!-- Deployment Method Tabs -->
        <div class="flex items-center gap-1 border-b border-[#232936] text-[11px]">
          <button 
            @click="activeMethod = 'powershell'"
            :class="activeMethod === 'powershell' ? 'bg-[#1e2533] text-txt-primary border-b-2 border-brand-500 font-medium' : 'text-txt-secondary hover:text-txt-primary'"
            class="px-3 py-1.5 rounded-t flex items-center gap-1.5 transition-colors"
          >
            <span class="material-symbols-outlined text-[14px]">desktop_windows</span>
            <span>Windows Server (PowerShell)</span>
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
            <span class="text-txt-muted text-[11px]">Yönetici (Run as Administrator) PowerShell penceresinde çalıştırın:</span>
            <button 
              @click="copyText(winCmd, 'win')"
              class="text-blue-400 hover:text-blue-300 flex items-center gap-1"
            >
              <span class="material-symbols-outlined text-[14px]">content_copy</span>
              <span>{{ copied === 'win' ? 'Kopyalandı!' : 'Komutu Kopyala' }}</span>
            </button>
          </div>
          <div class="p-2.5 rounded bg-[#0c0e12] border border-[#232936] font-mono text-[11px] text-txt-primary select-all break-all">
            {{ winCmd }}
          </div>
          <p class="text-[10px] text-txt-muted">
            * Dosya tabanlı indirme yürütür, hafıza içi (in-memory `iex`) çalıştırmaz ve doğrudan yerel Windows Hizmeti (SCM) olarak kaydeder.
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
            {{ linuxCmd }}
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
              <span>Linux Binary İndir</span>
            </a>
          </div>

          <div class="flex flex-col gap-1.5">
            <span class="text-txt-muted text-[11px] font-medium">Hizmet Kurulum Komutu (GPO / Komut Satırı):</span>
            <div class="p-2.5 rounded bg-[#0c0e12] border border-[#232936] font-mono text-[11px] text-txt-primary select-all">
              gnnwatchtower-agent.exe -service install -server {{ hostUrl }}
            </div>
            <div class="p-2.5 rounded bg-[#0c0e12] border border-[#232936] font-mono text-[11px] text-txt-primary select-all">
              gnnwatchtower-agent.exe -service start
            </div>
          </div>
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
import { ref, computed } from 'vue'

const props = defineProps({
  isOpen: Boolean
})

defineEmits(['close'])

const copied = ref('')
const activeMethod = ref('powershell')

const hostUrl = computed(() => {
  return window.location.origin
})

const linuxCmd = computed(() => {
  return `curl -sSL ${hostUrl.value}/api/agent/install/linux | sudo bash`
})

const winCmd = computed(() => {
  return `New-Item -ItemType Directory -Force -Path "C:\\Program Files\\GNNwatchtower-Agent"; Invoke-WebRequest -Uri "${hostUrl.value}/api/agent/download/windows" -OutFile "C:\\Program Files\\GNNwatchtower-Agent\\gnnwatchtower-agent.exe"; & "C:\\Program Files\\GNNwatchtower-Agent\\gnnwatchtower-agent.exe" -service install -server "${hostUrl.value}"; & "C:\\Program Files\\GNNwatchtower-Agent\\gnnwatchtower-agent.exe" -service start`
})

const copyText = async (text, type) => {
  let success = false

  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      success = true
    } catch (e) {
      success = false
    }
  }

  if (!success) {
    try {
      const textArea = document.createElement('textarea')
      textArea.value = text
      textArea.setAttribute('readonly', '')
      textArea.style.position = 'fixed'
      textArea.style.top = '0'
      textArea.style.left = '0'
      textArea.style.opacity = '0'
      textArea.style.pointerEvents = 'none'
      document.body.appendChild(textArea)
      textArea.focus()
      textArea.select()
      success = document.execCommand('copy')
      document.body.removeChild(textArea)
    } catch (err) {
      console.error('Kopyalama basarisiz:', err)
    }
  }

  if (success) {
    copied.value = type
    setTimeout(() => {
      copied.value = ''
    }, 2000)
  } else {
    window.prompt('Komutu panoya kopyalamak için Ctrl+C tuşlarına basınız:', text)
  }
}
</script>
