<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/90 backdrop-blur-md">
    <div class="bg-[#141820] border border-[#232936] rounded-xl max-w-sm w-full p-6 flex flex-col gap-5 shadow-2xl text-xs">
      <!-- Brand Logo -->
      <div class="flex flex-col items-center gap-2 text-center">
        <div class="w-10 h-10 rounded-lg bg-[#232936] flex items-center justify-center text-txt-primary">
          <span class="material-symbols-outlined text-[24px] text-blue-400">token</span>
        </div>
        <h2 class="text-base font-semibold text-white">GNNwatchtower</h2>
        <p class="text-txt-muted text-[11px]">Altyapı ve Ağ Yönetim Konsolu Girişi</p>
      </div>

      <!-- Login Form -->
      <form @submit.prevent="handleLogin" class="flex flex-col gap-3.5">
        <div class="flex flex-col gap-1">
          <label class="text-txt-muted font-medium">Kullanıcı Adı</label>
          <input 
            v-model="username" 
            required 
            placeholder="admin"
            class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-2 text-txt-primary focus:outline-none focus:border-blue-500"
          />
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-txt-muted font-medium">Parola</label>
          <input 
            v-model="password" 
            type="password"
            required 
            placeholder="••••••••"
            class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-2 text-txt-primary focus:outline-none focus:border-blue-500"
          />
        </div>

        <div v-if="errorMessage" class="p-2 rounded bg-rose-500/10 border border-rose-500/20 text-rose-400 text-[11px]">
          {{ errorMessage }}
        </div>

        <button 
          type="submit" 
          :disabled="isSubmitting"
          class="w-full mt-2 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-medium rounded transition-colors"
        >
          {{ isSubmitting ? 'Doğrulanıyor...' : 'Giriş Yap' }}
        </button>
      </form>

      <div class="text-center text-[10px] text-txt-muted pt-2 border-t border-[#1e2533]">
        Varsayılan Giriş: <span class="font-mono text-txt-secondary">admin / admin</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import api from '../services/api'

const props = defineProps({
  isOpen: Boolean
})

const emit = defineEmits(['logged-in'])

const username = ref('admin')
const password = ref('')
const isSubmitting = ref(false)
const errorMessage = ref('')

const handleLogin = async () => {
  isSubmitting.value = true
  errorMessage.value = ''
  try {
    await api.login(username.value, password.value)
    emit('logged-in')
  } catch (err) {
    errorMessage.value = err.response?.data?.error || 'Giriş başarısız. Kullanıcı adı veya şifre hatalı.'
  } finally {
    isSubmitting.value = false
  }
}
</script>
