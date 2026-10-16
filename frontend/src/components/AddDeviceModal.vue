<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm select-none" @click.self="!busy && $emit('close')">
    <div class="bg-[#141820] border border-[#232936] rounded-xl max-w-lg w-full flex flex-col shadow-2xl overflow-hidden text-xs">
      <!-- Header -->
      <div class="p-4 border-b border-[#232936] flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <div 
            class="w-7 h-7 rounded-lg flex items-center justify-center"
            :class="draft.monitoring_mode === 'website' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : draft.monitoring_mode === 'server' ? 'bg-blue-500/10 text-blue-400 border border-blue-500/20' : 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/20'"
          >
            <span class="material-symbols-outlined text-[17px]">
              {{ draft.monitoring_mode === 'website' ? 'language' : draft.monitoring_mode === 'server' ? 'dns' : 'router' }}
            </span>
          </div>
          <div class="flex flex-col">
            <h3 class="text-sm font-semibold text-white">
              {{ editDevice?.id ? 'Cihazı Düzenle' : draft.monitoring_mode === 'website' ? 'Yeni Web Sitesi Ekle' : draft.monitoring_mode === 'server' ? 'Yeni Sunucu Ekle (Windows / Linux)' : 'Yeni IP Cihazı Ekle (NAS / ESXi)' }}
            </h3>
            <span class="text-[11px] text-txt-muted">
              {{ draft.monitoring_mode === 'website' ? 'HTTP/HTTPS protokol yanıtı denetimi' : draft.monitoring_mode === 'server' ? 'Ajan telemetrisi ve ICMP erişilebilirliği' : 'Yalnızca ICMP ping yoklaması (Sıfır port açılımı)' }}
            </span>
          </div>
        </div>

        <button 
          @click="$emit('close')" 
          :disabled="busy"
          class="p-1 rounded text-txt-muted hover:text-white hover:bg-[#1d2432] transition-colors"
          aria-label="Kapat"
        >
          <span class="material-symbols-outlined text-[18px]">close</span>
        </button>
      </div>

      <!-- Form Body -->
      <form @submit.prevent="save" class="p-5 flex flex-col gap-4">
        <!-- Error Banner -->
        <div v-if="error" class="p-3 rounded-lg bg-rose-500/10 border border-rose-500/25 text-rose-400 text-xs flex items-center gap-2">
          <span class="material-symbols-outlined text-[16px]">error</span>
          <span>{{ error }}</span>
        </div>

        <!-- Cihaz Adı -->
        <div class="flex flex-col gap-1.5">
          <label class="text-txt-secondary font-medium">
            {{ draft.monitoring_mode === 'website' ? 'Web Sitesi Adı / Başlığı *' : 'Cihaz Adı *' }}
          </label>
          <input 
            v-model.trim="draft.name" 
            required 
            maxlength="128" 
            autofocus 
            :placeholder="draft.monitoring_mode === 'website' ? 'Örn: Kurumsal Web Sitesi veya Portal' : 'Örn: Muhasebe-DB-01 veya core-switch'"
            class="bg-[#0c0e12] border border-[#232936] rounded-md px-3 py-2 text-txt-primary focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20 transition-all"
          />
        </div>

        <!-- IP Adresi veya URL -->
        <div class="flex flex-col gap-1.5">
          <label class="text-txt-secondary font-medium">
            {{ draft.monitoring_mode === 'website' ? 'HTTP(S) Hedef URL *' : 'IP Adresi (IPv4 / IPv6) *' }}
          </label>
          <input 
            v-model.trim="draft.ip_address" 
            required 
            :maxlength="draft.monitoring_mode === 'website' ? 2048 : 45" 
            :placeholder="draft.monitoring_mode === 'website' ? 'https://example.com' : '10.38.1.50'"
            class="bg-[#0c0e12] border border-[#232936] rounded-md px-3 py-2 text-txt-primary font-mono focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20 transition-all"
          />
          <small v-if="draft.monitoring_mode !== 'website'" class="text-[11px] text-txt-muted">
            Geçerli bir IP adresi giriniz (örn: 192.168.1.10). Port veya URL eklemeyiniz.
          </small>
        </div>

        <!-- Cihaz / İşletim Sistemi Türü -->
        <div v-if="draft.monitoring_mode !== 'website'" class="flex flex-col gap-1.5">
          <label class="text-txt-secondary font-medium">
            {{ draft.monitoring_mode === 'server' ? 'İşletim Sistemi Türü' : 'Donanım / Envanter Türü' }}
          </label>
          <select 
            v-model="draft.device_type" 
            class="bg-[#0c0e12] border border-[#232936] rounded-md px-3 py-2 text-txt-primary focus:outline-none focus:border-blue-500"
          >
            <option v-if="!types[draft.monitoring_mode]?.includes(draft.device_type)" :value="draft.device_type">
              {{ typeLabels[draft.device_type] || draft.device_type }}
            </option>
            <option v-for="t in types[draft.monitoring_mode]" :value="t" :key="t">
              {{ typeLabels[t] || t }}
            </option>
          </select>
        </div>

        <!-- 2 Grid: İzleme Profili & Yoklama Aralığı -->
        <div class="grid grid-cols-2 gap-3">
          <div class="flex flex-col gap-1.5">
            <label class="text-txt-secondary font-medium">İzleme Profili</label>
            <select 
              v-model="draft.monitoring_profile" 
              class="bg-[#0c0e12] border border-[#232936] rounded-md px-3 py-2 text-txt-primary focus:outline-none focus:border-blue-500"
            >
              <option value="active">Aktif İzleme (Sürekli)</option>
              <option value="dormant_24h">24 Saatlik Uyku (Soğuk Yedek)</option>
              <option value="dormant_30d">30 Günlük Uyku (Arşiv)</option>
              <option value="maintenance">Planlı Bakım (Alarmlar Sessizde)</option>
            </select>
          </div>

          <div class="flex flex-col gap-1.5">
            <label class="text-txt-secondary font-medium">Yoklama Aralığı</label>
            <input 
              v-model.number="draft.check_interval_seconds" 
              type="number" 
              min="15" 
              max="86400" 
              required
              class="bg-[#0c0e12] border border-[#232936] rounded-md px-3 py-2 text-txt-primary font-mono focus:outline-none focus:border-blue-500"
            />
            <small class="text-[10px] text-txt-muted">Minimum 15 saniye</small>
          </div>
        </div>

        <!-- Konum / Notlar -->
        <div class="grid grid-cols-2 gap-3">
          <div class="flex flex-col gap-1.5">
            <label class="text-txt-secondary font-medium">Fiziksel Konum</label>
            <input 
              v-model.trim="draft.location" 
              maxlength="128" 
              placeholder="Örn: Kabinet A, Kat 2"
              class="bg-[#0c0e12] border border-[#232936] rounded-md px-3 py-2 text-txt-primary focus:outline-none focus:border-blue-500"
            />
          </div>

          <div class="flex flex-col gap-1.5">
            <label class="text-txt-secondary font-medium">Operatör Notları</label>
            <input 
              v-model.trim="draft.operator_notes" 
              maxlength="255" 
              placeholder="Örn: Yedek DNS, Kritik VM"
              class="bg-[#0c0e12] border border-[#232936] rounded-md px-3 py-2 text-txt-primary focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        <!-- Actions -->
        <div class="pt-3 border-t border-[#232936] flex items-center justify-end gap-2.5">
          <button 
            type="button" 
            @click="$emit('close')" 
            :disabled="busy"
            class="px-4 py-2 rounded-md bg-[#181e28] hover:bg-[#1d2432] text-txt-secondary hover:text-white border border-[#232936] transition-colors"
          >
            İptal
          </button>
          <button 
            type="submit" 
            :disabled="busy"
            class="px-5 py-2 rounded-md bg-blue-600 hover:bg-blue-500 text-white font-medium shadow-md transition-colors flex items-center gap-1.5 disabled:opacity-50"
          >
            <span v-if="busy" class="material-symbols-outlined text-[16px] animate-spin">sync</span>
            <span>{{ editDevice?.id ? 'Değişiklikleri Kaydet' : 'Cihazı Kaydet' }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, watch } from "vue";
import api from "../services/api";
import { apiError, typeLabels } from "../devicePolicy";

const props = defineProps({ 
  isOpen: Boolean, 
  editDevice: Object, 
  initialMode: { type: String, default: "icmp" } 
});

const emit = defineEmits(["close", "saved"]);

const draft = reactive({});
const busy = ref(false);
const error = ref("");

const types = { 
  server: ["windows", "linux", "ubuntu", "debian", "centos", "rhel", "rocky", "almalinux"], 
  icmp: ["nas", "synology", "qnap", "esxi", "esx", "switch", "router", "other", "windows", "linux", "ubuntu", "debian"] 
};

watch(() => [props.isOpen, props.editDevice, props.initialMode], () => {
  if (!props.isOpen) return;
  error.value = "";
  const d = props.editDevice;
  Object.keys(draft).forEach((k) => delete draft[k]);
  Object.assign(
    draft, 
    { 
      name: "", 
      ip_address: "", 
      device_type: props.initialMode === "server" ? "windows" : props.initialMode === "website" ? "website" : "nas", 
      monitoring_mode: props.initialMode, 
      monitoring_profile: "active", 
      check_interval_seconds: 15, 
      location: "", 
      operator_notes: "" 
    }, 
    d ? { 
      id: d.id, 
      name: d.name, 
      ip_address: d.ip_address, 
      device_type: d.device_type, 
      monitoring_mode: d.monitoring_mode, 
      monitoring_profile: d.monitoring_profile, 
      check_interval_seconds: d.check_interval_seconds, 
      location: d.location, 
      operator_notes: d.operator_notes, 
      os_version: d.os_version, 
      purpose: d.purpose 
    } : {}
  );
}, { immediate: true });

async function save() {
  if (busy.value) return;
  error.value = "";
  busy.value = true;
  try {
    if (props.editDevice?.id) await api.updateDevice(props.editDevice.id, draft);
    else await api.createDevice(draft);
    emit("saved", draft.monitoring_mode);
    emit("close");
  } catch (e) {
    error.value = apiError(e);
  } finally {
    busy.value = false;
  }
}
</script>
