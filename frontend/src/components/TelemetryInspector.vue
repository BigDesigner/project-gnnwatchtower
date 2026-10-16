<script setup>
import { ref } from "vue";
import api from "../services/api";
import { dateTime, metric, apiError, telemetryCurrent } from "../devicePolicy";
const props = defineProps({ now: Number, device: Object, metrics: { type: Array, default: () => [] }, loading: Boolean, error: String });
const emit = defineEmits(["close", "changed"]);
const serviceName = ref("");
const port = ref(null);
const serviceType = ref("tcp_port");
const servicePath = ref("");
const serviceError = ref("");
const busy = ref(false);
async function addService() {
  if (props.device?.monitoring_mode !== "server") return;
  busy.value = true;
  serviceError.value = "";
  try {
    await api.addService(props.device.id, { name: serviceName.value, target_port: port.value, check_type: serviceType.value, target_path: servicePath.value, expected_status_code: 200 });
    serviceName.value = "";
    port.value = null;
    servicePath.value = "";
    emit("changed");
  } catch (e) {
    serviceError.value = apiError(e);
  } finally {
    busy.value = false;
  }
}
async function removeService(s) {
  if (props.device?.monitoring_mode !== "server" || !confirm(s.name + " servis izlemesi kald\u0131r\u0131ls\u0131n m\u0131?")) return;
  busy.value = true;
  serviceError.value = "";
  try {
    await api.deleteService(props.device.id, s.id);
    emit("changed");
  } catch (e) {
    serviceError.value = apiError(e);
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <section v-if="device && device.monitoring_mode === 'server'" class="wt-panel wt-inspector">
    <header class="wt-panel-heading">
      <h2>{{ device.name }} · Ajan Telemetrisi</h2>
      <button @click="$emit('close')">Kapat</button>
    </header>
    <p class="wt-muted">Kaynak: cihazın mevcut ajan raporları. Disk doluluğu, disk veya RAID sağlığı anlamına gelmez.</p>
    <div class="wt-summary-strip">
      <span>
        CPU
        <strong>{{ metric(device,'latest_cpu') }}</strong>
      </span>
      <span>
        Bellek
        <strong>{{ metric(device,'latest_ram') }}</strong>
      </span>
      <span>
        Disk doluluğu
        <strong>{{ metric(device,'latest_disk') }}</strong>
      </span>
      <span>
        Son örnek
        <strong>{{ dateTime(device.latest_metric_at) }}</strong>
      </span>
    </div>
    <p class="wt-muted" v-if="telemetryCurrent(device,now)">CPU sıcaklığı: {{ device.latest_temp != null ? device.latest_temp.toLocaleString('tr-TR') + ' °C' : 'Veri Yok' }} · Alınan trafik: {{ device.latest_net_rx != null ? device.latest_net_rx.toLocaleString('tr-TR') + ' KB/sn' : 'Veri Yok' }} · Gönderilen trafik: {{ device.latest_net_tx != null ? device.latest_net_tx.toLocaleString('tr-TR') + ' KB/sn' : 'Veri Yok' }}</p>
    <p v-if="!telemetryCurrent(device,now)" class="wt-notice">Güncel ajan verisi bulunmuyor. Önceki örnekler güncel donanım sağlığı olarak gösterilmez.</p>
    <p v-if="loading">Örnekler yükleniyor…</p>
    <p v-else-if="error" class="wt-error" role="alert">{{ error }}</p>
    <div v-else-if="metrics.length" class="wt-table-scroll">
      <table class="wt-table">
        <caption class="wt-muted">Son ajan örnekleri · geçmiş veriler</caption>
        <thead>
          <tr>
            <th>Zaman</th>
            <th>CPU (%)</th>
            <th>Bellek (%)</th>
            <th>Disk doluluğu (%)</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(m,i) in metrics" :key="i">
            <td>{{ dateTime(m.recorded_at) }}</td>
            <td>{{ m.cpu ?? '—' }}</td>
            <td>{{ m.ram ?? '—' }}</td>
            <td>{{ m.disk ?? '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-else class="wt-muted">Kaydedilmiş ajan örneği yok.</p>
    <details class="wt-service-details">
      <summary>Mevcut Servis İzlemeleri ({{ device.services?.length || 0 }})</summary>
      <p class="wt-muted">Yalnızca güvenlik politikası kapsamında zaten kullanılan servisler için mevcut kontrol yoludur. Watchtower hedefte servis açmaz veya kurmaz.</p>
      <div v-if="device.services?.length" class="wt-table-scroll">
        <table class="wt-table">
          <thead>
            <tr>
              <th>Servis</th>
              <th>Port</th>
              <th>Son Gözlem</th>
              <th>İşlem</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in device.services" :key="s.id">
              <td>{{ s.name }}</td>
              <td>{{ s.target_port }}</td>
              <td>
                {{ s.last_check_at ? ({online:'Erişilebilir',offline:'Ulaşılamıyor',degraded:'Beklenen Yanıt Alınamadı'})[s.status] || 'Veri Yok' : 'Kontrol Bekleniyor' }}
                <small>{{ dateTime(s.last_check_at) }}</small>
              </td>
              <td>
                <button :disabled="busy" @click="removeService(s)">Kaldır</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <form class="wt-filters" @submit.prevent="addService">
        <input v-model.trim="serviceName" required maxlength="128" placeholder="Servis adı" aria-label="Servis adı" />
        <input v-model.number="port" type="number" required min="1" max="65535" placeholder="Port" aria-label="Servis portu" />
        <select v-model="serviceType" aria-label="Servis kontrol yöntemi">
          <option value="tcp_port">TCP Portu</option>
          <option value="http">HTTP Yanıtı</option>
        </select>
        <input v-if="serviceType==='http'" v-model="servicePath" placeholder="/sağlık" aria-label="HTTP yolu" />
        <button :disabled="busy">Servis İzlemesi Ekle</button>
      </form>
      <p v-if="serviceError" class="wt-error" role="alert">{{ serviceError }}</p>
    </details>
  </section>
</template>
