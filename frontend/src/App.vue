<script setup>
import { ref, reactive, computed, watch, onMounted, onUnmounted } from "vue";
import api from "./services/api";
import { sections, stateLabels, typeLabels, categoryView, categoryLabel, observation, dateTime, apiError, telemetryCurrent, nasType } from "./devicePolicy";
import TopBar from "./components/TopBar.vue";
import Sidebar from "./components/Sidebar.vue";
import ServerTable from "./components/ServerTable.vue";
import WebsiteTable from "./components/WebsiteTable.vue";
import IpDeviceTable from "./components/IpDeviceTable.vue";
import TelemetryInspector from "./components/TelemetryInspector.vue";
import AddDeviceModal from "./components/AddDeviceModal.vue";
import AgentModal from "./components/AgentModal.vue";
import ClassifyModal from "./components/ClassifyModal.vue";
import SettingsModal from "./components/SettingsModal.vue";
import LoginModal from "./components/LoginModal.vue";
import ChangePasswordModal from "./components/ChangePasswordModal.vue";
const currentView = ref("overview");
const devices = ref([]);
const incidents = ref([]);
const stats = ref({});
const loading = ref(false);
const loaded = ref(false);
const fetchError = ref("");
const actionError = ref("");
const message = ref("");
const lastRefresh = ref("");
const clock = ref(Date.now());
const loginOpen = ref(true);
const passwordOpen = ref(false);
const passwordForced = ref(false);
const settingsOpen = ref(false);
const agentOpen = ref(false);
const chooserOpen = ref(false);
const addOpen = ref(false);
const editTarget = ref(null);
const initialMode = ref("icmp");
const classifyTarget = ref(null);
const gateway = ref(null);
const gatewayLoading = ref(false);
const gatewayError = ref("");
const selectedID = ref(null);
const selectedMetrics = ref([]);
const metricsLoading = ref(false);
const metricsError = ref("");
const backupBusy = ref(false);
const filters = reactive(Object.fromEntries(sections.map((s) => [s.id, { query: "", status: "", type: "", page: 1 }])));
const incidentFilter = reactive({ device: "", category: "", state: "" });
const categorySections = sections.filter((s) => s.mode);
const viewInfo = computed(() => sections.find((s) => s.id === currentView.value) || sections[0]);
const filter = computed(() => filters[currentView.value]);
const pageSize = 20;
const inCategory = (mode) => devices.value.filter((d) => d.monitoring_mode === mode);
const counts = computed(() => Object.fromEntries(categorySections.map((s) => [s.mode, inCategory(s.mode).length])));
const deviceState = (d) => observation(d, clock.value);
const statusCounts = computed(() => Object.fromEntries(Object.keys(stateLabels).map((s) => [s, devices.value.filter((d) => deviceState(d) === s).length])));
const observedCount = computed(() => statusCounts.value.online + statusCounts.value.offline);
const reachability = computed(() => observedCount.value ? (100 * statusCounts.value.online / observedCount.value).toLocaleString("tr-TR", { maximumFractionDigits: 1 }) + " %" : "\u2014");
const categoryStats = (mode) => {
  const list = inCategory(mode);
  return { total: list.length, online: list.filter((d) => deviceState(d) === "online").length, offline: list.filter((d) => deviceState(d) === "offline").length, other: list.filter((d) => !["online", "offline"].includes(deviceState(d))).length };
};
const installedCount = computed(() => inCategory("server").filter((d) => d.is_agent_installed).length);
const freshServers = computed(() => inCategory("server").filter((d) => telemetryCurrent(d, clock.value)));
const currentDevices = computed(() => inCategory(viewInfo.value.mode));
const inventoryTypes = computed(() => [...new Set(currentDevices.value.map((d) => d.device_type))].sort());
const filteredDevices = computed(() => currentDevices.value.filter((d) => (!filter.value.query || [d.name, d.ip_address, d.location, typeLabels[d.device_type] || d.device_type].some((v) => String(v || "").toLocaleLowerCase("tr-TR").includes(filter.value.query.toLocaleLowerCase("tr-TR")))) && (!filter.value.status || deviceState(d) === filter.value.status) && (!filter.value.type || (filter.value.type === "__nas" ? nasType(d.device_type) : currentView.value === "devices" && filter.value.type === "linux" ? d.device_type !== "windows" : d.device_type === filter.value.type))));
const pages = computed(() => Math.max(1, Math.ceil(filteredDevices.value.length / pageSize)));
const pageDevices = computed(() => filteredDevices.value.slice((filter.value.page - 1) * pageSize, filter.value.page * pageSize));
const selectedDevice = computed(() => currentView.value === "devices" ? devices.value.find((d) => d.id === selectedID.value && d.monitoring_mode === "server") : null);
const deviceFor = (id) => devices.value.find((d) => d.id === id);
const activeIncidents = computed(() => incidents.value.filter((i) => !i.resolved_at));
const filteredIncidents = computed(() => incidents.value.filter((i) => {
  const d = deviceFor(i.device_id);
  const q = filter.value.query.toLocaleLowerCase("tr-TR");
  return (!incidentFilter.device || i.device_id === Number(incidentFilter.device)) && (!incidentFilter.category || d?.monitoring_mode === incidentFilter.category) && (!incidentFilter.state || (incidentFilter.state === "open" ? !i.resolved_at : !!i.resolved_at)) && (!q || [i.device_name, d?.name, d?.ip_address].some((v) => String(v || "").toLocaleLowerCase("tr-TR").includes(q)));
}));
const causeLabel = (cause) => ({ unclassified: "S\u0131n\u0131fland\u0131r\u0131lmad\u0131", power_outage: "Elektrik Kesintisi", maintenance: "Planl\u0131 Bak\u0131m", planned_maintenance: "Planl\u0131 Bak\u0131m", hardware_crash: "Donan\u0131m / \xC7\xF6kme", system_crash: "Sistem \xC7\xF6kmesi", network_issue: "A\u011F Sorunu", network_failure: "A\u011F Sorunu" })[cause] || "Di\u011Fer";
const duration = (i) => {
  const sec = i.duration_seconds ?? Math.max(0, Math.floor((clock.value - Date.parse(i.started_at)) / 1e3));
  return sec >= 3600 ? Math.floor(sec / 3600) + " sa " + Math.floor(sec % 3600 / 60) + " dk" : sec >= 60 ? Math.floor(sec / 60) + " dk " + sec % 60 + " sn" : sec + " sn";
};
let timer, clockTimer, authEpoch = 0, metricsEpoch = 0;
watch(currentView, () => {
  agentOpen.value = false;
  selectedID.value = null;
  selectedMetrics.value = [];
  metricsEpoch++;
  addOpen.value = false;
  editTarget.value = null;
  chooserOpen.value = false;
});
watch(() => [filter.value.query, filter.value.status, filter.value.type], () => {
  filter.value.page = 1;
});
watch(pages, (n) => {
  if (filter.value.page > n) filter.value.page = n;
});
function navigate(view) {
  if (view === "nas") {
    currentView.value = "ip-devices";
    filters["ip-devices"].type = "__nas";
  } else if (sections.some((s) => s.id === view)) currentView.value = view;
}
function openAdd(mode) {
  initialMode.value = mode;
  editTarget.value = null;
  chooserOpen.value = false;
  addOpen.value = true;
}
function editDevice(d) {
  initialMode.value = d.monitoring_mode;
  editTarget.value = d;
  addOpen.value = true;
}
function showIncidents(d) {
  navigate("incidents");
  incidentFilter.device = String(d.id);
  incidentFilter.category = "";
  incidentFilter.state = "";
}
function openIncidentDevice(i) {
  const d = deviceFor(i.device_id);
  if (d) {
    navigate(categoryView(d));
    filters[currentView.value].query = d.ip_address;
    filters[currentView.value].type = "";
    filters[currentView.value].status = "";
  }
}
async function refresh() {
  if (loading.value || loginOpen.value || passwordForced.value) return;
  loading.value = true;
  fetchError.value = "";
  const epoch = authEpoch;
  try {
    const [d, i, s] = await Promise.all([api.getDevices(), api.getIncidents(), api.getDashboardStats()]);
    if (epoch !== authEpoch) return;
    devices.value = d;
    incidents.value = i;
    stats.value = s;
    loaded.value = true;
    lastRefresh.value = (/* @__PURE__ */ new Date()).toLocaleTimeString("tr-TR");
    clock.value = Date.now();
    if (selectedDevice.value) loadMetrics(selectedDevice.value);
  } catch (e) {
    if (epoch === authEpoch) fetchError.value = apiError(e, "Veriler yenilenemedi. \xD6nceki kay\u0131tlar g\xF6steriliyor; son veri zaman\u0131n\u0131 kontrol edin.");
  } finally {
    loading.value = false;
  }
}
async function loadMetrics(d) {
  if (currentView.value !== "devices" || d.monitoring_mode !== "server") return;
  const epoch = ++metricsEpoch;
  selectedID.value = d.id;
  selectedMetrics.value = [];
  metricsError.value = "";
  metricsLoading.value = true;
  try {
    const res = await api.getDeviceTelemetry(d.id);
    if (epoch === metricsEpoch) selectedMetrics.value = res.metrics || [];
  } catch (e) {
    if (epoch === metricsEpoch) metricsError.value = apiError(e);
  } finally {
    if (epoch === metricsEpoch) metricsLoading.value = false;
  }
}
async function runAction(fn, success) {
  actionError.value = "";
  message.value = "";
  try {
    await fn();
    message.value = success;
    await refresh();
  } catch (e) {
    actionError.value = apiError(e);
  }
}
function deleteDevice(d) {
  if (confirm(d.name + " kayd\u0131, telemetrisi ve olay ge\xE7mi\u015Fi silinsin mi?")) runAction(() => api.deleteDevice(d.id), "Cihaz silindi.");
}
function toggleProfile(d) {
  runAction(() => api.updateDevice(d.id, { monitoring_profile: ["paused", "maintenance"].includes(d.monitoring_profile) ? "active" : "maintenance" }), "\u0130zleme profili g\xFCncellendi.");
}
function upgrade(d) {
  if (currentView.value !== "devices" || d.monitoring_mode !== "server") return;
  if (confirm(d.name + " ajan\u0131na g\xFCncelleme talimat\u0131 g\xF6nderilsin mi?")) runAction(() => api.upgradeAgent(d.id), "G\xFCncelleme talimat\u0131 kuyru\u011Fa al\u0131nd\u0131.");
}
function upgradeAll() {
  if (currentView.value === "devices" && confirm("Kay\u0131tl\u0131 Windows/Linux ajanlar\u0131na g\xFCncelleme talimat\u0131 g\xF6nderilsin mi?")) runAction(() => api.upgradeAllAgents(), "Toplu g\xFCncelleme talimat\u0131 kuyru\u011Fa al\u0131nd\u0131.");
}
async function saved(mode) {
  navigate(categorySections.find((s) => s.mode === mode)?.id || "ip-devices");
  await refresh();
}
function authFailed() {
  authEpoch++;
  metricsEpoch++;
  loginOpen.value = true;
  devices.value = [];
  incidents.value = [];
  stats.value = {};
  loaded.value = false;
  lastRefresh.value = "";
  agentOpen.value = false;
  settingsOpen.value = false;
  addOpen.value = false;
  chooserOpen.value = false;
  classifyTarget.value = null;
  selectedID.value = null;
  selectedMetrics.value = [];
  passwordOpen.value = false;
  passwordForced.value = false;
}
function forcePassword() {
  passwordForced.value = true;
  passwordOpen.value = true;
  agentOpen.value = false;
}
function loggedIn(data) {
  loginOpen.value = false;
  if (data?.must_change_password) forcePassword();
  else refresh();
}
async function passwordChanged() {
  passwordForced.value = false;
  passwordOpen.value = false;
  await refresh();
}
async function checkGateway() {
  if (gatewayLoading.value) return;
  gatewayLoading.value = true;
  gatewayError.value = "";
  try {
    gateway.value = await api.getGatewayHealth();
  } catch (e) {
    gateway.value = null;
    gatewayError.value = apiError(e);
  } finally {
    gatewayLoading.value = false;
  }
}
async function downloadBackup() {
  backupBusy.value = true;
  try {
    const bundle = await api.exportBackup();
    const url = URL.createObjectURL(new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "gnnwatchtower-backup-" + (/* @__PURE__ */ new Date()).toISOString().slice(0, 10) + ".json";
    a.click();
    URL.revokeObjectURL(url);
  } catch (e) {
    actionError.value = apiError(e);
  } finally {
    backupBusy.value = false;
  }
}
async function restoreBackup(e) {
  const file = e.target.files?.[0];
  if (!file) return;
  if (file.size > 2 * 1024 * 1024) {
    actionError.value = "Yedek dosyas\u0131 2 MB s\u0131n\u0131r\u0131n\u0131 a\u015F\u0131yor.";
    e.target.value = "";
    return;
  }
  if (!confirm("Yedekteki ayar ve kay\u0131tlar mevcut yap\u0131land\u0131rmaya uygulans\u0131n m\u0131?")) {
    e.target.value = "";
    return;
  }
  backupBusy.value = true;
  try {
    const data = JSON.parse(await file.text());
    await api.importBackup(data);
    message.value = "Yedek geri y\xFCklendi. Yeni hedeflerin ilk kontrol\xFC bekleniyor.";
    await refresh();
  } catch (err) {
    actionError.value = apiError(err, "Yedek okunamad\u0131 veya geri y\xFCklenemedi. JSON bi\xE7imini kontrol edin.");
  } finally {
    backupBusy.value = false;
    e.target.value = "";
  }
}
onMounted(async () => {
  window.addEventListener("gnn-auth-failed", authFailed);
  window.addEventListener("gnn-must-change-password", forcePassword);
  if (localStorage.getItem("gnn_token")) {
    try {
      const me = await api.getMe();
      loggedIn(me);
    } catch (e) {
      if (e.response?.data?.must_change_password) {
        loginOpen.value = false;
        forcePassword();
      } else authFailed();
    }
  }
  timer = setInterval(refresh, 1e4);
  clockTimer = setInterval(() => {
    clock.value = Date.now();
  }, 1e3);
});
onUnmounted(() => {
  clearInterval(timer);
  clearInterval(clockTimer);
  authEpoch++;
  metricsEpoch++;
  window.removeEventListener("gnn-auth-failed", authFailed);
  window.removeEventListener("gnn-must-change-password", forcePassword);
});
</script>

<template>
  <div class="wt-app">
    <TopBar :last-refresh="lastRefresh" :refreshing="loading" @refresh="refresh" @add-device="chooserOpen=true" @change-password="passwordOpen=true" @logout="api.logout()" />
    <Sidebar :current-view="currentView" :counts="counts" @select-view="navigate" @open-settings="settingsOpen=true" />
    <main class="wt-main">
      <header class="wt-page-heading">
        <div>
          <p class="wt-eyebrow">
            {{ stats.client_company_name || 'ALTYAPI İZLEME' }}
            <span v-if="stats.client_location"> · {{ stats.client_location }}</span>
          </p>
          <h1>{{ viewInfo.label }}</h1>
          <p class="wt-muted">{{ ({overview:'Erişilebilirlik ve telemetri kapsamını birlikte takip edin.',devices:'Windows ve Linux envanteri; ajan kurulumu, erişilebilirlik ve güncel örnekler.',websites:'HTTP(S) yanıtları üzerinden web sitesi erişilebilirliği.', 'ip-devices':'NAS, ESXi ve ağ cihazları için yalnızca ICMP erişilebilirliği.',incidents:'Kaydedilmiş erişim kesintileri, geri dönüşler ve operatör değerlendirmeleri.',backups:'Yapılandırma, envanter ve olay kayıtlarını dışa aktarın veya geri yükleyin.'})[currentView] }}</p>
        </div>
        <div class="wt-toolbar">
          <template v-if="currentView==='devices'">
            <button @click="agentOpen=true">Ajan Dağıtımı</button>
            <button v-if="installedCount" @click="upgradeAll">Ajanları Güncelle</button>
          </template>
          <button v-if="viewInfo.mode" class="wt-primary" @click="openAdd(viewInfo.mode)">{{ currentView==='websites' ? 'Web Sitesi Ekle' : currentView==='ip-devices' ? 'IP Cihazı Ekle' : 'Cihaz Ekle' }}</button>
        </div>
      </header>
      <div v-if="fetchError" class="wt-error" role="alert">
        {{ fetchError }}
        <button @click="refresh">Yeniden Dene</button>
      </div>
      <div v-if="actionError" class="wt-error" role="alert">
        {{ actionError }}
        <button @click="actionError=''" aria-label="Uyarıyı kapat">×</button>
      </div>
      <div v-if="message" class="wt-notice" role="status">
        {{ message }}
        <button @click="message=''" aria-label="Bildirimi kapat">×</button>
      </div>
      <p v-if="loading && !loaded" class="wt-empty" role="status">Kayıtlar yükleniyor…</p>
      <template v-if="currentView==='overview' && loaded">
        <div class="wt-kpis">
          <section class="wt-panel">
            <span>Toplam Kayıt</span>
            <strong>{{ devices.length }}</strong>
            <small>Üç ayrı izleme kategorisi</small>
          </section>
          <section class="wt-panel">
            <span>Güncel Erişilebilirlik</span>
            <strong>{{ reachability }}</strong>
            <small>{{ observedCount }} güncel gözlem · geçmiş SLA değildir</small>
          </section>
          <section class="wt-panel">
            <span>Ulaşılamayan</span>
            <strong class="wt-rose">{{ statusCounts.offline }}</strong>
            <small>Güncel başarısız kontroller</small>
          </section>
          <section class="wt-panel">
            <span>Güncel Ajan Verisi</span>
            <strong>
              {{ freshServers.length }}
              <em>/ {{ installedCount }}</em>
            </strong>
            <small>Kayıtlı ajanlardan güncel rapor verenler</small>
          </section>
        </div>
        <div class="wt-summary-strip wt-panel">
          <span v-for="(label,state) in stateLabels" :key="state">
            {{ label }}
            <strong>{{ statusCounts[state] }}</strong>
          </span>
        </div>
        <div class="wt-category-grid">
          <button v-for="s in categorySections" :key="s.id" class="wt-panel wt-category" @click="navigate(s.id)">
            <span class="material-symbols-outlined">{{ s.icon }}</span>
            <h2>{{ s.label }}</h2>
            <strong>{{ categoryStats(s.mode).total }}</strong>
            <p>
              <span class="wt-green">{{ categoryStats(s.mode).online }} erişilebilir</span>
              ·
              <span class="wt-rose">{{ categoryStats(s.mode).offline }} ulaşılamıyor</span>
            </p>
            <small>{{ categoryStats(s.mode).other }} kontrol bekliyor, güncel değil veya askıda</small>
            <small v-if="s.mode==='server'">{{ installedCount }} ajan kayıtlı · {{ counts.server-installedCount }} ajan bekliyor</small>
          </button>
        </div>
        <section class="wt-panel">
          <header class="wt-panel-heading">
            <h2>
              Açık Olaylar
              <small>{{ activeIncidents.length }}</small>
            </h2>
            <button @click="navigate('incidents')">Olay Günlüğü</button>
          </header>
          <p class="wt-muted">Son 200 kayıt içindeki açık olaylar. Bir kesinti kaydı, cihazın güç kaybını veya disk arızasını kanıtlamaz.</p>
          <div v-if="activeIncidents.length" class="wt-table-scroll">
            <table class="wt-table">
              <thead>
                <tr>
                  <th>Cihaz</th>
                  <th>Kategori</th>
                  <th>Hedef</th>
                  <th>Başlangıç</th>
                  <th>Süre</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="i in activeIncidents.slice(0,10)" :key="i.id">
                  <td>
                    <button class="wt-link" @click="openIncidentDevice(i)">{{ i.device_name || deviceFor(i.device_id)?.name }}</button>
                  </td>
                  <td>{{ categoryLabel(deviceFor(i.device_id)) }}</td>
                  <td class="wt-mono">{{ deviceFor(i.device_id)?.ip_address || '—' }}</td>
                  <td>{{ dateTime(i.started_at) }}</td>
                  <td>{{ duration(i) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else class="wt-empty">Görüntülenen kayıtlarda açık olay yok.</p>
        </section>
        <section class="wt-panel">
          <header class="wt-panel-heading">
            <h2>Ağ Geçidi · ICMP</h2>
            <button :disabled="gatewayLoading" @click="checkGateway">{{ gatewayLoading ? 'Kontrol Ediliyor…' : 'Kontrol Et' }}</button>
          </header>
          <p v-if="gateway" class="wt-muted">
            {{ gateway.ip || 'Ağ geçidi yapılandırılmamış' }}
            ·
            {{ gateway.status==='online' ? 'Erişilebilir' : gateway.status==='offline' ? 'Ulaşılamıyor' : 'Kontrol Edilemedi' }}
            <span v-if="gateway.status==='online' && gateway.latency_ms != null"> · {{ gateway.latency_ms }} ms</span>
            <small> · Son manuel kontrol; donanım sağlığı değildir.</small>
          </p>
          <p v-else class="wt-muted">{{ gatewayError || 'Ayarlar bölümünde tanımlı ağ geçidi, düğmeye basıldığında bir kez kontrol edilir.' }}</p>
        </section>
        <section class="wt-panel">
          <h2>Telemetri Kapsamı</h2>
          <p class="wt-muted">Güncel CPU, bellek ve disk doluluğu yalnızca Ajan Özeti içindeki gerçek ajan raporlarından gelir. CPU sıcaklığı ortam sıcaklığı değildir. IP cihazlarında disk, RAID ve güç sağlığı ölçülmez.</p>
          <button @click="navigate('devices')">Ajan Özeti’ni Aç</button>
        </section>
      </template>
      <template v-if="viewInfo.mode && loaded">
        <div v-if="currentView==='ip-devices'" class="wt-notice wt-security">
          <span class="material-symbols-outlined">shield</span>
          <div>
            <strong>Ek yönetim servisi açmadan, yalnızca ICMP ping ile izleme</strong>
            <p>Bu kategoride SNMP, cihaz API’leri, SSH ve ajan kullanılmaz; mevcut servisler etkinleştirilmez. Disk, RAID, CPU ve bellek sağlığı toplanmaz. Ping yanıtı alınamaması ağ kesintisi veya ICMP filtrelemesinden kaynaklanabilir; cihazın kapalı olduğunu tek başına kanıtlamaz.</p>
          </div>
        </div>
        <div v-if="currentView==='websites'" class="wt-notice">Web İzleme HTTP(S) yanıtını kontrol eder. Alan adı tescili, süre sonu ve donanım sağlığı bu kontrolün kapsamı dışındadır.</div>
        <div class="wt-summary-strip wt-panel">
          <span>
            Toplam
            <strong>{{ currentDevices.length }}</strong>
          </span>
          <span>
            Erişilebilir
            <strong>{{ categoryStats(viewInfo.mode).online }}</strong>
          </span>
          <span>
            Ulaşılamıyor
            <strong>{{ categoryStats(viewInfo.mode).offline }}</strong>
          </span>
          <span>
            Bekleyen / Güncel Değil / Askıda
            <strong>{{ categoryStats(viewInfo.mode).other }}</strong>
          </span>
        </div>
        <section class="wt-panel wt-list">
          <div v-if="currentView==='devices'" class="wt-tabs">
            <button v-for="t in [{id:'',label:'Tümü'},{id:'windows',label:'Windows'},{id:'linux',label:'Linux'}]" :key="t.id" :class="{active:filter.type===t.id}" @click="filter.type=t.id">{{ t.label }}</button>
          </div>
          <div class="wt-filters">
            <label class="wt-search">
              <span class="sr-only">Cihazlarda ara</span>
              <input v-model="filter.query" placeholder="Ad, hedef veya konum ara…" />
            </label>
            <label>
              <span class="sr-only">Erişilebilirlik filtresi</span>
              <select v-model="filter.status">
                <option value="">Tüm Durumlar</option>
                <option v-for="(label,state) in stateLabels" :key="state" :value="state">{{ label }}</option>
              </select>
            </label>
            <label v-if="currentView==='ip-devices'">
              <span class="sr-only">Cihaz türü</span>
              <select v-model="filter.type">
                <option value="">Tüm Türler</option>
                <option value="__nas">Tüm NAS Cihazları</option>
                <option v-for="t in inventoryTypes" :key="t" :value="t">{{ typeLabels[t] || t }}</option>
              </select>
            </label>
            <button v-if="filter.query || filter.status || filter.type" @click="filter.query='';filter.status='';filter.type=''">Filtreleri Temizle</button>
          </div>
          <component v-if="pageDevices.length" :is="currentView==='devices' ? ServerTable : currentView==='websites' ? WebsiteTable : IpDeviceTable" :devices="pageDevices" :now="clock" @edit="editDevice" @delete="deleteDevice" @profile="toggleProfile" @incidents="showIncidents" @select="loadMetrics" @upgrade="upgrade" />
          <div v-else-if="loaded" class="wt-empty">
            <h2>{{ currentDevices.length ? 'Filtreye Uyan Kayıt Yok' : 'Henüz Kayıt Eklenmedi' }}</h2>
            <p>{{ currentDevices.length ? 'Arama veya durum filtresini değiştirin.' : 'İzlenecek hedefi bu kategoriye elle ekleyin.' }}</p>
            <button v-if="!currentDevices.length" @click="openAdd(viewInfo.mode)">İlk Kaydı Ekle</button>
          </div>
          <footer v-if="filteredDevices.length" class="wt-pagination">
            <span>{{ filteredDevices.length }} kayıt · {{ filter.page }} / {{ pages }}</span>
            <div>
              <button :disabled="filter.page<=1" @click="filter.page--">Önceki</button>
              <button :disabled="filter.page>=pages" @click="filter.page++">Sonraki</button>
            </div>
          </footer>
        </section>
        <TelemetryInspector v-if="selectedDevice" :device="selectedDevice" :now="clock" :metrics="selectedMetrics" :loading="metricsLoading" :error="metricsError" @close="selectedID=null;metricsEpoch++" @changed="refresh" />
      </template>
      <section v-if="currentView==='incidents'" class="wt-panel wt-list">
        <div class="wt-filters">
          <input v-model="filter.query" placeholder="Cihaz veya hedef ara…" aria-label="Olaylarda ara" />
          <select v-model="incidentFilter.category" aria-label="Olay kategorisi">
            <option value="">Tüm Kategoriler</option>
            <option v-for="s in categorySections" :key="s.mode" :value="s.mode">{{ s.label }}</option>
          </select>
          <select v-model="incidentFilter.state" aria-label="Olay durumu">
            <option value="">Tüm Olaylar</option>
            <option value="open">Açık</option>
            <option value="resolved">Çözüldü</option>
          </select>
          <select v-model="incidentFilter.device" aria-label="Olay cihazı">
            <option value="">Tüm Cihazlar</option>
            <option v-for="d in devices" :key="d.id" :value="String(d.id)">{{ d.name }}</option>
          </select>
        </div>
        <div v-if="filteredIncidents.length" class="wt-table-scroll">
          <table class="wt-table">
            <thead>
              <tr>
                <th>Cihaz / Kategori</th>
                <th>Başlangıç</th>
                <th>Bitiş</th>
                <th>Süre</th>
                <th>Değerlendirme</th>
                <th>İşlem</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="i in filteredIncidents" :key="i.id">
                <td>
                  <button class="wt-link" @click="openIncidentDevice(i)">{{ i.device_name || deviceFor(i.device_id)?.name || 'Kayıtlı Cihaz' }}</button>
                  <small>{{ categoryLabel(deviceFor(i.device_id)) }} · {{ deviceFor(i.device_id)?.ip_address }}</small>
                </td>
                <td>{{ dateTime(i.started_at) }}</td>
                <td>{{ i.resolved_at ? dateTime(i.resolved_at) : 'Açık' }}</td>
                <td>{{ duration(i) }}</td>
                <td>
                  {{ causeLabel(i.classified_cause) }}
                  <small>{{ i.operator_confirmed ? 'Operatör tarafından doğrulandı' : 'Operatör doğrulaması yok' }}</small>
                  <small v-if="i.auto_detection_hypothesis">Hipotez: {{ i.auto_detection_hypothesis }}</small>
                </td>
                <td>
                  <button @click="classifyTarget=i">Sınıflandır</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else-if="loaded" class="wt-empty">Bu filtrelere uyan olay bulunmuyor.</p>
        <p class="wt-muted wt-list-note">En yeni 200 olay gösterilir. Otomatik hipotez, ölçülmüş fiziksel arıza değildir.</p>
      </section>
      <div v-if="currentView==='backups'" class="wt-category-grid">
        <section class="wt-panel">
          <h2>Yedeği İndir</h2>
          <p class="wt-muted">Ayarlar, kayıtlı cihazlar, servis yapılandırmaları ve en yeni 1000 olay dışa aktarılır. Ajan metrik geçmişi yedeğe dahil değildir.</p>
          <p class="wt-notice">Yedek dosyası erişim anahtarları içerebilir. Güvenli bir konumda saklayın.</p>
          <button class="wt-primary" :disabled="backupBusy" @click="downloadBackup">{{ backupBusy ? 'İşleniyor…' : 'JSON Yedeğini İndir' }}</button>
        </section>
        <section class="wt-panel">
          <h2>Yedeği Geri Yükle</h2>
          <p class="wt-muted">Mevcut kayıtlar hedef adresine göre eşleştirilir. İzleme kategorisi değiştirilemez; IP cihazlarının eski servisleri çalıştırılmaz. Eski ağ keşfi ayarları yok sayılır.</p>
          <label class="wt-file">
            JSON Yedek Dosyası
            <input type="file" accept="application/json,.json" :disabled="backupBusy" @change="restoreBackup" />
          </label>
        </section>
      </div>
    </main>
    <div v-if="chooserOpen" class="wt-modal-backdrop" @click.self="chooserOpen=false">
      <section class="wt-modal" role="dialog" aria-modal="true" aria-labelledby="choose-title">
        <header>
          <h2 id="choose-title">Cihaz Ekle · İzleme Kategorisi</h2>
          <button @click="chooserOpen=false" aria-label="Kapat">×</button>
        </header>
        <div class="wt-choice-list">
          <button v-for="s in categorySections" :key="s.mode" @click="openAdd(s.mode)">
            <span class="material-symbols-outlined">{{ s.icon }}</span>
            <div>
              <strong>{{ s.label }}</strong>
              <small>{{ s.mode==='server' ? 'Windows / Linux envanteri' : s.mode==='website' ? 'HTTP(S) web sitesi' : 'NAS / ESXi / ağ cihazı · yalnızca ICMP' }}</small>
            </div>
          </button>
        </div>
      </section>
    </div>
    <AgentModal v-if="currentView==='devices' && agentOpen && !loginOpen && !passwordForced" :is-open="true" @close="agentOpen=false" />
    <AddDeviceModal v-if="addOpen && !loginOpen" :is-open="true" :edit-device="editTarget" :initial-mode="initialMode" @close="addOpen=false;editTarget=null" @saved="saved" />
    <ClassifyModal v-if="classifyTarget" :is-open="true" :incident="classifyTarget" @close="classifyTarget=null" @classified="refresh" />
    <SettingsModal v-if="settingsOpen" :is-open="true" @close="settingsOpen=false" @saved="refresh" />
    <ChangePasswordModal v-if="passwordOpen" :is-open="true" :forced="passwordForced" @close="!passwordForced && (passwordOpen=false)" @password-changed="passwordChanged" />
    <LoginModal v-if="loginOpen" :is-open="true" @logged-in="loggedIn" />
  </div>
</template>
