<script setup>
import { reactive, ref, watch } from "vue";
import api from "../services/api";
import { apiError, typeLabels } from "../devicePolicy";
const props = defineProps({ isOpen: Boolean, editDevice: Object, initialMode: { type: String, default: "icmp" } });
const emit = defineEmits(["close", "saved"]);
const draft = reactive({});
const busy = ref(false);
const error = ref("");
const types = { server: ["windows", "linux", "ubuntu", "debian", "centos", "rhel", "rocky", "almalinux"], icmp: ["nas", "synology", "qnap", "esxi", "esx", "switch", "router", "other", "windows", "linux", "ubuntu", "debian"] };
watch(() => [props.isOpen, props.editDevice, props.initialMode], () => {
  if (!props.isOpen) return;
  error.value = "";
  const d = props.editDevice;
  Object.keys(draft).forEach((k) => delete draft[k]);
  Object.assign(draft, { name: "", ip_address: "", device_type: props.initialMode === "server" ? "windows" : props.initialMode === "website" ? "website" : "nas", monitoring_mode: props.initialMode, monitoring_profile: "active", check_interval_seconds: 15, location: "", operator_notes: "" }, d ? { id: d.id, name: d.name, ip_address: d.ip_address, device_type: d.device_type, monitoring_mode: d.monitoring_mode, monitoring_profile: d.monitoring_profile, check_interval_seconds: d.check_interval_seconds, location: d.location, operator_notes: d.operator_notes, os_version: d.os_version, purpose: d.purpose } : {});
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
<template>
  <div v-if="isOpen" class="wt-modal-backdrop" @click.self="!busy && $emit('close')">
    <section class="wt-modal" role="dialog" aria-modal="true" aria-labelledby="device-form-title">
      <header>
        <h2 id="device-form-title">{{ editDevice?.id ? 'Cihazı Düzenle' : initialMode === 'website' ? 'Web Sitesi Ekle' : initialMode === 'server' ? 'Cihaz Ekle · Ajan Özeti' : 'IP Cihazı Ekle' }}</h2>
        <button @click="$emit('close')" :disabled="busy" aria-label="Kapat">×</button>
      </header>
      <form @submit.prevent="save">
        <p class="wt-muted">{{ draft.monitoring_mode === 'website' ? 'HTTP(S) yanıtını izler. Alan adı tescili veya geçerlilik süresi denetlenmez.' : draft.monitoring_mode === 'icmp' ? 'Yalnızca ICMP erişilebilirliği izlenir. Ajan veya yönetim servisi kullanılmaz.' : 'Windows/Linux envanteri. Ajan otomatik kurulmaz; dağıtım Ajan Özeti içinden ayrı yürütülür.' }}</p>
        <label>
          Cihaz Adı
          <input v-model.trim="draft.name" required maxlength="128" autofocus />
        </label>
        <label>
          {{ draft.monitoring_mode === 'website' ? 'HTTP(S) URL' : 'IP Adresi' }}
          <input v-model.trim="draft.ip_address" required :maxlength="draft.monitoring_mode === 'website' ? 2048 : 45" :placeholder="draft.monitoring_mode === 'website' ? 'https://example.com' : '192.0.2.10'" />
          <small v-if="draft.monitoring_mode !== 'website'">Tekil IPv4 veya IPv6 adresi; URL, port ve ana makine adı kabul edilmez.</small>
        </label>
        <label v-if="draft.monitoring_mode !== 'website'">
          {{ draft.monitoring_mode === 'server' ? 'İşletim Sistemi' : 'Envanter Türü' }}
          <select v-model="draft.device_type">
            <option v-if="!types[draft.monitoring_mode]?.includes(draft.device_type)" :value="draft.device_type">{{ typeLabels[draft.device_type] || draft.device_type }}</option>
            <option v-for="t in types[draft.monitoring_mode]" :value="t" :key="t">{{ typeLabels[t] || t }}</option>
          </select>
        </label>
        <div class="wt-form-grid">
          <label>
            İzleme Profili
            <select v-model="draft.monitoring_profile">
              <option value="active">Etkin</option>
              <option value="dormant_24h">Günde Bir</option>
              <option value="dormant_30d">30 Günde Bir</option>
              <option value="maintenance">Bakımda</option>
              <option value="paused">Duraklatıldı</option>
            </select>
          </label>
          <label>
            Kontrol Aralığı (saniye)
            <input type="number" v-model.number="draft.check_interval_seconds" required min="15" max="86400" step="1" :disabled="draft.monitoring_profile !== 'active'" />
          </label>
        </div>
        <label>
          Konum
          <input v-model.trim="draft.location" maxlength="256" />
        </label>
        <label>
          Operatör Notu
          <textarea v-model="draft.operator_notes" maxlength="4096" rows="3"></textarea>
        </label>
        <p v-if="error" class="wt-error" role="alert">{{ error }}</p>
        <footer>
          <button type="button" @click="$emit('close')" :disabled="busy">İptal</button>
          <button class="wt-primary" :disabled="busy">{{ busy ? 'Kaydediliyor…' : 'Kaydet' }}</button>
        </footer>
      </form>
    </section>
  </div>
</template>
