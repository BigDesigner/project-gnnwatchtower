<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
    <div class="bg-[#141820] border border-[#232936] rounded-lg max-w-xl w-full flex flex-col shadow-2xl overflow-hidden text-xs">
      <!-- Header -->
      <div class="p-4 border-b border-[#232936] flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-emerald-400">terminal</span>
          <h3 class="text-sm font-semibold text-white">Tek Tuşla Mini-Ajan Dağıtımı</h3>
        </div>
        <button @click="$emit('close')" class="text-txt-muted hover:text-white">
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <!-- Body -->
      <div class="p-5 flex flex-col gap-5">
        <p class="text-txt-muted leading-relaxed text-[11px]">
          İzlemek istediğiniz sunucunun terminaline aşağıdaki komutu yapıştırın. Ajan otomatik olarak servis şeklinde kurulacak ve GNNwatchtower'a canlı heartbeat gönderecektir.
        </p>

        <!-- Linux / Ubuntu Tab -->
        <div class="flex flex-col gap-1.5">
          <div class="flex items-center justify-between text-txt-secondary font-medium">
            <span class="flex items-center gap-1.5">
              <span class="material-symbols-outlined text-[15px]">terminal</span>
              <span>Ubuntu / Debian / Linux Sunucu:</span>
            </span>
            <button 
              @click="copyText(linuxCmd, 'linux')"
              class="text-blue-400 hover:text-blue-300 flex items-center gap-1"
            >
              <span class="material-symbols-outlined text-[14px]">content_copy</span>
              <span>{{ copied === 'linux' ? 'Kopyalandı!' : 'Kopyala' }}</span>
            </button>
          </div>
          <div class="p-2.5 rounded bg-[#0c0e12] border border-[#232936] font-mono text-[11px] text-txt-primary select-all break-all">
            {{ linuxCmd }}
          </div>
        </div>

        <!-- Windows Server Tab -->
        <div class="flex flex-col gap-1.5">
          <div class="flex items-center justify-between text-txt-secondary font-medium">
            <span class="flex items-center gap-1.5">
              <span class="material-symbols-outlined text-[15px]">desktop_windows</span>
              <span>Windows Server (PowerShell Yönetici):</span>
            </span>
            <button 
              @click="copyText(winCmd, 'win')"
              class="text-blue-400 hover:text-blue-300 flex items-center gap-1"
            >
              <span class="material-symbols-outlined text-[14px]">content_copy</span>
              <span>{{ copied === 'win' ? 'Kopyalandı!' : 'Kopyala' }}</span>
            </button>
          </div>
          <div class="p-2.5 rounded bg-[#0c0e12] border border-[#232936] font-mono text-[11px] text-txt-primary select-all break-all">
            {{ winCmd }}
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

const hostUrl = computed(() => {
  return window.location.origin
})

const linuxCmd = computed(() => {
  return `curl -sSL ${hostUrl.value}/api/agent/install/linux | sudo bash`
})

const winCmd = computed(() => {
  return `powershell -ExecutionPolicy Bypass -Command "irm ${hostUrl.value}/api/agent/install/windows | iex"`
})

const copyText = (text, type) => {
  navigator.clipboard.writeText(text)
  copied.value = type
  setTimeout(() => {
    copied.value = ''
  }, 2000)
}
</script>
