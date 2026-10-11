<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
    <div class="bg-[#141820] border border-[#232936] rounded-lg max-w-sm w-full p-5 flex flex-col gap-4 shadow-2xl text-xs">
      <div class="flex items-center justify-between pb-3 border-b border-[#232936]">
        <div class="flex items-center gap-2">
          <span class="material-symbols-outlined text-blue-400">key</span>
          <h3 class="text-sm font-semibold text-white">Yönetici Parolasını Değiştir</h3>
        </div>
        <button @click="$emit('close')" class="text-txt-muted hover:text-white">
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>

      <form @submit.prevent="handleChangePassword" class="flex flex-col gap-3">
        <div class="flex flex-col gap-1">
          <label class="text-txt-muted font-medium">Mevcut Parola</label>
          <input 
            v-model="currentPassword" 
            type="password"
            required 
            class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-txt-primary focus:outline-none focus:border-blue-500"
          />
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-txt-muted font-medium">Yeni Parola</label>
          <input 
            v-model="newPassword" 
            type="password"
            required 
            class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-txt-primary focus:outline-none focus:border-blue-500"
          />
        </div>

        <div v-if="errorMsg" class="p-2 rounded bg-rose-500/10 border border-rose-500/20 text-rose-400 text-[11px]">
          {{ errorMsg }}
        </div>

        <div class="pt-3 border-t border-[#232936] flex items-center justify-end gap-2 mt-1">
          <button type="button" @click="$emit('close')" class="px-3 py-1.5 rounded text-txt-secondary hover:text-white">
            İptal
          </button>
          <button 
            type="submit" 
            :disabled="isSubmitting"
            class="px-4 py-1.5 rounded font-medium bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white"
          >
            {{ isSubmitting ? 'Kaydediliyor...' : 'Parolayı Güncelle' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import api from '../services/api'

const props = defineProps({
  isOpen: Boolean
})

const emit = defineEmits(['close'])

const currentPassword = ref('')
const newPassword = ref('')
const isSubmitting = ref(false)
const errorMsg = ref('')

const handleChangePassword = async () => {
  isSubmitting.value = true
  errorMsg.value = ''
  try {
    await api.changePassword(currentPassword.value, newPassword.value)
    alert('Parolanız başarıyla güncellendi!')
    emit('close')
  } catch (err) {
    errorMsg.value = err.response?.data?.error || 'Parola güncellenemedi.'
  } finally {
    isSubmitting.value = false
  }
}
</script>
