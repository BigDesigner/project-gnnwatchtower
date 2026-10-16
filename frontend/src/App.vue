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
const searchQuery = ref("");

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
const observedCount = computed(() => (statusCounts.value.online || 0) + (statusCounts.value.offline || 0));
const reachability = computed(() => observedCount.value ? (100 * (statusCounts.value.online || 0) / observedCount.value).toLocaleString("tr-TR", { maximumFractionDigits: 1 }) + " %" : "—");

const categoryStats = (mode) => {
  const list = inCategory(mode);
  return { 
    total: list.length, 
    online: list.filter((d) => deviceState(d) === "online").length, 
    offline: list.filter((d) => deviceState(d) === "offline").length, 
    other: list.filter((d) => !["online", "offline"].includes(deviceState(d))).length 
  };
};

const installedCount = computed(() => inCategory("server").filter((d) => d.is_agent_installed).length);
const freshServers = computed(() => inCategory("server").filter((d) => telemetryCurrent(d, clock.value)));
const currentDevices = computed(() => inCategory(viewInfo.value.mode));
const inventoryTypes = computed(() => [...new Set(currentDevices.value.map((d) => d.device_type))].sort());

// Search synchronization: sync TopBar search query with active filter query
watch(searchQuery, (newVal) => {
  if (filter.value) {
    filter.value.query = newVal;
  }
});

const filteredDevices = computed(() => currentDevices.value.filter((d) => {
  const q = (filter.value.query || searchQuery.value || "").toLocaleLowerCase("tr-TR");
  const matchesQuery = !q || [d.name, d.ip_address, d.location, typeLabels[d.device_type] || d.device_type].some((v) => String(v || "").toLocaleLowerCase("tr-TR").includes(q));
  const matchesStatus = !filter.value.status || deviceState(d) === filter.value.status;
  const matchesType = !filter.value.type || (
    filter.value.type === "__nas" 
      ? nasType(d.device_type) 
      : currentView.value === "devices" && filter.value.type === "linux" 
        ? d.device_type !== "windows" 
        : d.device_type === filter.value.type
  );
  return matchesQuery && matchesStatus && matchesType;
}));

const pages = computed(() => Math.max(1, Math.ceil(filteredDevices.value.length / pageSize)));
const pageDevices = computed(() => filteredDevices.value.slice((filter.value.page - 1) * pageSize, filter.value.page * pageSize));
const selectedDevice = computed(() => currentView.value === "devices" ? devices.value.find((d) => d.id === selectedID.value && d.monitoring_mode === "server") : null);
const deviceFor = (id) => devices.value.find((d) => d.id === id);
const activeIncidents = computed(() => incidents.value.filter((i) => !i.resolved_at));

const filteredIncidents = computed(() => incidents.value.filter((i) => {
  const d = deviceFor(i.device_id);
  const q = (filter.value.query || searchQuery.value || "").toLocaleLowerCase("tr-TR");
  return (!incidentFilter.device || i.device_id === Number(incidentFilter.device)) && 
         (!incidentFilter.category || d?.monitoring_mode === incidentFilter.category) && 
         (!incidentFilter.state || (incidentFilter.state === "open" ? !i.resolved_at : !!i.resolved_at)) && 
         (!q || [i.device_name, d?.name, d?.ip_address].some((v) => String(v || "").toLocaleLowerCase("tr-TR").includes(q)));
}));

const causeLabel = (cause) => ({ 
  unclassified: "Sınıflandırılmadı", 
  power_outage: "Elektrik Kesintisi", 
  maintenance: "Planlı Bakım", 
  planned_maintenance: "Planlı Bakım", 
  hardware_crash: "Donanım / Çökme", 
  system_crash: "Sistem Çökmesi", 
  network_issue: "Ağ Sorunu", 
  network_failure: "Ağ Sorunu" 
})[cause] || "Diğer";

const duration = (i) => {
  const sec = i.duration_seconds ?? Math.max(0, Math.floor((clock.value - Date.parse(i.started_at)) / 1e3));
  return sec >= 3600 
    ? Math.floor(sec / 3600) + " sa " + Math.floor(sec % 3600 / 60) + " dk" 
    : sec >= 60 
      ? Math.floor(sec / 60) + " dk " + sec % 60 + " sn" 
      : sec + " sn";
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
  } else if (sections.some((s) => s.id === view)) {
    currentView.value = view;
  }
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
    lastRefresh.value = new Date().toLocaleTimeString("tr-TR");
    clock.value = Date.now();
    if (selectedDevice.value) loadMetrics(selectedDevice.value);
  } catch (e) {
    if (epoch === authEpoch) fetchError.value = apiError(e, "Veriler yenilenemedi. Önceki kayıtlar gösteriliyor.");
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
  if (confirm(d.name + " kaydı, telemetrisi ve olay geçmişi silinsin mi?")) {
    runAction(() => api.deleteDevice(d.id), "Cihaz silindi.");
  }
}

function toggleProfile(d) {
  runAction(
    () => api.updateDevice(d.id, { monitoring_profile: ["paused", "maintenance"].includes(d.monitoring_profile) ? "active" : "maintenance" }), 
    "İzleme profili güncellendi."
  );
}

function upgrade(d) {
  if (currentView.value !== "devices" || d.monitoring_mode !== "server") return;
  if (confirm(d.name + " ajanına güncelleme talimatı gönderilsin mi?")) {
    runAction(() => api.upgradeAgent(d.id), "Güncelleme talimatı kuyruğa alındı.");
  }
}

function upgradeAll() {
  if (currentView.value === "devices" && confirm("Kayıtlı tüm Windows/Linux ajanlarına güncelleme talimatı gönderilsin mi?")) {
    runAction(() => api.upgradeAllAgents(), "Toplu güncelleme talimatı kuyruğa alındı.");
  }
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
  else {
    refresh();
    checkGateway();
  }
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
    a.download = "gnnwatchtower-backup-" + new Date().toISOString().slice(0, 10) + ".json";
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
    actionError.value = "Yedek dosyası 2 MB sınırını aşıyor.";
    e.target.value = "";
    return;
  }
  if (!confirm("Yedekteki ayar ve kayıtlar mevcut yapılandırmaya uygulansın mı?")) {
    e.target.value = "";
    return;
  }
  backupBusy.value = true;
  try {
    const data = JSON.parse(await file.text());
    await api.importBackup(data);
    message.value = "Yedek geri yüklendi. Hedeflerin ilk kontrolü bekleniyor.";
    await refresh();
  } catch (err) {
    actionError.value = apiError(err, "Yedek okunamadı veya geri yüklenemedi. JSON biçimini kontrol edin.");
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
      } else {
        authFailed();
      }
    }
  }
  timer = setInterval(refresh, 10000);
  clockTimer = setInterval(() => {
    clock.value = Date.now();
  }, 1000);
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
  <div class="min-h-screen bg-canvas text-txt-primary flex flex-col antialiased select-text">
    <!-- Top Bar Component -->
    <TopBar 
      :stats="stats" 
      :gateway="gateway"
      :reachability="reachability"
      v-model:searchQuery="searchQuery" 
      :isRefreshing="loading"
      :lastRefresh="lastRefresh"
      :activeIncidentsCount="activeIncidents.length"
      @refresh="refresh"
      @open-incidents="navigate('incidents')"
      @add-device="chooserOpen = true"
      @open-settings="settingsOpen = true"
      @open-change-password="passwordOpen = true"
      @logout="api.logout()"
    />

    <!-- Left Navigation Rail -->
    <Sidebar 
      :currentView="currentView"
      :counts="counts"
      :activeIncidentsCount="activeIncidents.length"
      :gateway="gateway"
      @select-view="navigate"
      @open-settings="settingsOpen = true"
    />

    <!-- Main Content Area -->
    <div class="pl-60 pt-14 flex-1 flex flex-col">
      <main class="p-6 lg:p-8 max-w-[1600px] w-full mx-auto flex flex-col gap-6 flex-1">
        <!-- Top Breadcrumb & Actions Bar -->
        <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 pb-4 border-b border-[#232936]">
          <div class="flex flex-col gap-1.5">
            <div class="flex items-center gap-2 text-xs text-txt-muted flex-wrap">
              <span class="font-medium text-txt-secondary">{{ stats.client_company_name || 'ALTYAPI İZLEME' }}</span>
              <span class="text-[#2c3547]">/</span>
              <span>{{ stats.client_location || 'Sistem Odası' }}</span>
              <span class="text-[#2c3547]">/</span>
              <span class="text-txt-primary font-medium">{{ viewInfo.label }}</span>
              <span 
                class="px-2 py-0.5 rounded text-[11px] font-medium border"
                :class="(statusCounts.offline || 0) === 0 ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border-rose-500/20'"
              >
                {{ (statusCounts.offline || 0) === 0 ? 'Tüm Sistemler Sağlıklı' : (statusCounts.offline + ' Kesinti Mevcut') }}
              </span>
            </div>

            <p class="text-xs text-txt-secondary">
              {{ 
                currentView === 'overview' 
                  ? 'Erişilebilirlik ve gerçek telemetri kapsamını NOC panelinden anlık izleyin.' 
                  : currentView === 'devices' 
                    ? 'Windows ve Linux sunucu envanteri; ajan kurulumu, CPU/RAM/Disk kullanımı ve durum takibi.' 
                    : currentView === 'websites' 
                      ? 'HTTP(S) yanıtları üzerinden web siteleri ve API servislerinin erişilebilirliği.' 
                      : currentView === 'ip-devices' 
                        ? 'NAS, ESXi ve ağ donanımları için sıfır açık port ile yalnızca ICMP erişilebilirlik izleme.' 
                        : currentView === 'incidents' 
                          ? 'Sistem kesintileri, otomatik tespit hipotezleri ve operatör değerlendirmeleri.' 
                          : 'Yapılandırma, envanter ve olay kayıtlarını dışa aktarın veya güvenle geri yükleyin.' 
              }}
            </p>
          </div>

          <!-- Top Quick Actions -->
          <div class="flex items-center gap-2.5 self-start lg:self-center">
            <!-- Contextual Actions for Ajan Özeti -->
            <template v-if="currentView === 'devices'">
              <button 
                @click="agentOpen = true"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-[#181e28] hover:bg-[#1f2736] text-txt-primary border border-[#232936] transition-colors shadow-sm"
              >
                <span class="material-symbols-outlined text-[16px] text-emerald-400">terminal</span>
                <span>Ajan Dağıtımı</span>
              </button>
              <button 
                v-if="installedCount > 0"
                @click="upgradeAll"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-[#181e28] hover:bg-[#1f2736] text-txt-primary border border-[#232936] transition-colors shadow-sm"
                title="Tüm Windows/Linux ajanlarını uzaktan son sürüme güncelle"
              >
                <span class="material-symbols-outlined text-[16px] text-blue-400">system_update_alt</span>
                <span>Ajanları Güncelle</span>
              </button>
            </template>

            <!-- Category Specific Add Device Button -->
            <button 
              v-if="viewInfo.mode"
              @click="openAdd(viewInfo.mode)"
              class="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-medium rounded-md bg-blue-600 hover:bg-blue-500 text-white transition-colors shadow-sm"
            >
              <span class="material-symbols-outlined text-[16px]">add</span>
              <span>{{ currentView === 'websites' ? '+ Web Sitesi Ekle' : currentView === 'ip-devices' ? '+ IP Cihazı Ekle' : '+ Sunucu Ekle' }}</span>
            </button>
          </div>
        </div>

        <!-- Global Alert Toasts -->
        <div v-if="fetchError" class="p-3.5 rounded-lg bg-rose-500/10 border border-rose-500/25 text-rose-400 text-xs flex items-center justify-between shadow-sm">
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-[18px]">error</span>
            <span>{{ fetchError }}</span>
          </div>
          <button @click="refresh" class="px-2.5 py-1 rounded bg-rose-500/20 hover:bg-rose-500/30 font-medium transition-colors">
            Yeniden Dene
          </button>
        </div>

        <div v-if="actionError" class="p-3.5 rounded-lg bg-rose-500/10 border border-rose-500/25 text-rose-400 text-xs flex items-center justify-between shadow-sm">
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-[18px]">warning</span>
            <span>{{ actionError }}</span>
          </div>
          <button @click="actionError = ''" class="text-rose-400 hover:text-white">
            <span class="material-symbols-outlined text-[18px]">close</span>
          </button>
        </div>

        <div v-if="message" class="p-3.5 rounded-lg bg-emerald-500/10 border border-emerald-500/25 text-emerald-400 text-xs flex items-center justify-between shadow-sm">
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-[18px]">check_circle</span>
            <span>{{ message }}</span>
          </div>
          <button @click="message = ''" class="text-emerald-400 hover:text-white">
            <span class="material-symbols-outlined text-[18px]">close</span>
          </button>
        </div>

        <!-- Initial Loading State -->
        <div v-if="loading && !loaded" class="p-12 text-center text-txt-muted text-xs flex flex-col items-center justify-center gap-3">
          <span class="material-symbols-outlined text-[32px] text-blue-400 animate-spin">sync</span>
          <span>Sistem verileri ve telemetri toplanıyor...</span>
        </div>

        <!-- VIEW 1: Overview (Genel Bakış) -->
        <template v-if="currentView === 'overview' && loaded">
          <!-- 4 Executive KPI Cards -->
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <!-- Card 1: Toplam Düğüm -->
            <div 
              @click="navigate('devices')"
              class="p-4 rounded-lg bg-[#141820] border border-[#232936] hover:border-blue-500/50 cursor-pointer transition-colors flex flex-col gap-2 group shadow-sm"
            >
              <div class="flex items-center justify-between text-xs text-txt-muted">
                <span class="font-medium">Toplam Kayıtlı Hedef</span>
                <span class="material-symbols-outlined text-[18px] text-blue-400 group-hover:translate-x-0.5 transition-transform">dns</span>
              </div>
              <div class="flex items-baseline gap-2">
                <span class="text-3xl font-mono font-semibold text-white">{{ devices.length }}</span>
                <span class="text-xs text-blue-400 font-medium">Hedef</span>
              </div>
              <span class="text-[11px] text-txt-muted flex items-center gap-1">
                <span>3 ayrı izleme kategorisi</span>
                <span class="material-symbols-outlined text-[13px]">arrow_forward</span>
              </span>
            </div>

            <!-- Card 2: Güncel Erişilebilirlik -->
            <div class="p-4 rounded-lg bg-[#141820] border border-[#232936] flex flex-col gap-2 shadow-sm">
              <div class="flex items-center justify-between text-xs text-txt-muted">
                <span class="font-medium">Güncel Erişilebilirlik</span>
                <span class="material-symbols-outlined text-[18px] text-emerald-400">network_check</span>
              </div>
              <div class="flex items-baseline gap-2">
                <span class="text-3xl font-mono font-semibold text-emerald-400">{{ reachability }}</span>
              </div>
              <span class="text-[11px] text-txt-muted">
                {{ observedCount }} güncel gözlem · Geçmiş SLA değildir
              </span>
            </div>

            <!-- Card 3: Ulaşılamayan / Kesintiler -->
            <div 
              @click="navigate('incidents')"
              class="p-4 rounded-lg bg-[#141820] border border-[#232936] hover:border-rose-500/50 cursor-pointer transition-colors flex flex-col gap-2 group shadow-sm"
            >
              <div class="flex items-center justify-between text-xs text-txt-muted">
                <span class="font-medium">Ulaşılamayan Düğüm</span>
                <span class="material-symbols-outlined text-[18px] text-rose-400 group-hover:translate-x-0.5 transition-transform">report_problem</span>
              </div>
              <div class="flex items-baseline gap-2">
                <span 
                  class="text-3xl font-mono font-semibold"
                  :class="(statusCounts.offline || 0) > 0 ? 'text-rose-400' : 'text-white'"
                >
                  {{ statusCounts.offline || 0 }}
                </span>
                <span class="text-xs text-txt-muted">Düğüm</span>
              </div>
              <span class="text-[11px] text-txt-muted flex items-center gap-1">
                <span :class="(statusCounts.offline || 0) > 0 ? 'text-rose-400 font-medium' : ''">
                  {{ (statusCounts.offline || 0) > 0 ? 'Aktif kesintileri incele' : 'Tüm hedefler erişilebilir' }}
                </span>
                <span class="material-symbols-outlined text-[13px]">arrow_forward</span>
              </span>
            </div>

            <!-- Card 4: Güncel Ajan Telemetrisi -->
            <div 
              @click="navigate('devices')"
              class="p-4 rounded-lg bg-[#141820] border border-[#232936] hover:border-blue-500/50 cursor-pointer transition-colors flex flex-col gap-2 group shadow-sm"
            >
              <div class="flex items-center justify-between text-xs text-txt-muted">
                <span class="font-medium">Ajan Telemetri Kapsamı</span>
                <span class="material-symbols-outlined text-[18px] text-cyan-400 group-hover:translate-x-0.5 transition-transform">verified_user</span>
              </div>
              <div class="flex items-baseline gap-2">
                <span class="text-3xl font-mono font-semibold text-white">{{ freshServers.length }}</span>
                <span class="text-xs text-txt-muted">/ {{ installedCount }} Ajan Aktif</span>
              </div>
              <span class="text-[11px] text-txt-muted flex items-center gap-1">
                <span>Canlı CPU/RAM/Disk raporlayan</span>
                <span class="material-symbols-outlined text-[13px]">arrow_forward</span>
              </span>
            </div>
          </div>

          <!-- Quick Status Strip -->
          <div class="p-3.5 rounded-lg bg-[#141820] border border-[#232936] flex items-center justify-between flex-wrap gap-4 text-xs shadow-sm">
            <span class="text-txt-muted text-[11px] uppercase tracking-wider font-semibold">Hedef Durum Özeti:</span>
            <div class="flex items-center gap-4 flex-wrap">
              <span v-for="(label, state) in stateLabels" :key="state" class="flex items-center gap-1.5">
                <span class="text-txt-muted">{{ label }}:</span>
                <strong class="font-mono text-txt-primary">{{ statusCounts[state] || 0 }}</strong>
              </span>
            </div>
          </div>

          <!-- 3 Interactive Category Deep-Dive Cards -->
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <!-- Category 1: Ajan Özeti -->
            <div 
              @click="navigate('devices')"
              class="p-5 rounded-lg bg-[#141820] border border-[#232936] hover:border-blue-500/50 cursor-pointer transition-all flex flex-col gap-3 group shadow-sm"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2.5">
                  <div class="w-8 h-8 rounded-lg bg-blue-600/10 border border-blue-500/20 flex items-center justify-center text-blue-400">
                    <span class="material-symbols-outlined text-[18px]">dns</span>
                  </div>
                  <div>
                    <h3 class="text-sm font-semibold text-white group-hover:text-blue-400 transition-colors">Ajan Özeti</h3>
                    <p class="text-[11px] text-txt-muted">Windows & Linux Sunucular</p>
                  </div>
                </div>
                <span class="text-2xl font-mono font-semibold text-white">{{ categoryStats('server').total }}</span>
              </div>

              <div class="flex items-center gap-3 text-xs pt-2 border-t border-[#232936]">
                <span class="flex items-center gap-1 text-emerald-400">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                  {{ categoryStats('server').online }} Erişilebilir
                </span>
                <span class="flex items-center gap-1 text-rose-400">
                  <span class="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
                  {{ categoryStats('server').offline }} Ulaşılamıyor
                </span>
              </div>
              <div class="text-[11px] text-txt-muted">
                {{ installedCount }} ajan kurulu · {{ (counts.server || 0) - installedCount }} ajan bekliyor
              </div>
            </div>

            <!-- Category 2: Web İzleme -->
            <div 
              @click="navigate('websites')"
              class="p-5 rounded-lg bg-[#141820] border border-[#232936] hover:border-emerald-500/50 cursor-pointer transition-all flex flex-col gap-3 group shadow-sm"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2.5">
                  <div class="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
                    <span class="material-symbols-outlined text-[18px]">language</span>
                  </div>
                  <div>
                    <h3 class="text-sm font-semibold text-white group-hover:text-emerald-400 transition-colors">Web İzleme</h3>
                    <p class="text-[11px] text-txt-muted">HTTP / HTTPS URL Kontrolleri</p>
                  </div>
                </div>
                <span class="text-2xl font-mono font-semibold text-white">{{ categoryStats('website').total }}</span>
              </div>

              <div class="flex items-center gap-3 text-xs pt-2 border-t border-[#232936]">
                <span class="flex items-center gap-1 text-emerald-400">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                  {{ categoryStats('website').online }} Erişilebilir
                </span>
                <span class="flex items-center gap-1 text-rose-400">
                  <span class="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
                  {{ categoryStats('website').offline }} Ulaşılamıyor
                </span>
              </div>
              <div class="text-[11px] text-txt-muted">
                HTTP yanıt kodu ve milisaniye bazlı gecikme denetimi
              </div>
            </div>

            <!-- Category 3: IP Cihazları -->
            <div 
              @click="navigate('ip-devices')"
              class="p-5 rounded-lg bg-[#141820] border border-[#232936] hover:border-cyan-500/50 cursor-pointer transition-all flex flex-col gap-3 group shadow-sm"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2.5">
                  <div class="w-8 h-8 rounded-lg bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400">
                    <span class="material-symbols-outlined text-[18px]">router</span>
                  </div>
                  <div>
                    <h3 class="text-sm font-semibold text-white group-hover:text-cyan-400 transition-colors">IP Cihazları</h3>
                    <p class="text-[11px] text-txt-muted">NAS, ESXi & Ağ Donanımları</p>
                  </div>
                </div>
                <span class="text-2xl font-mono font-semibold text-white">{{ categoryStats('icmp').total }}</span>
              </div>

              <div class="flex items-center gap-3 text-xs pt-2 border-t border-[#232936]">
                <span class="flex items-center gap-1 text-emerald-400">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                  {{ categoryStats('icmp').online }} Erişilebilir
                </span>
                <span class="flex items-center gap-1 text-rose-400">
                  <span class="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
                  {{ categoryStats('icmp').offline }} Ulaşılamıyor
                </span>
              </div>
              <div class="text-[11px] text-txt-muted">
                Yalnızca ICMP ping yoklaması · Sıfır ek yönetim portu
              </div>
            </div>
          </div>

          <!-- Active Incidents Panel -->
          <section class="rounded-lg bg-[#141820] border border-[#232936] p-5 flex flex-col gap-4 shadow-sm">
            <div class="flex items-center justify-between pb-3 border-b border-[#232936]">
              <div class="flex items-center gap-2.5">
                <span class="material-symbols-outlined text-[18px] text-amber-400">history</span>
                <h2 class="text-sm font-semibold text-white">
                  Açık Olaylar & Kesintiler
                  <span class="ml-1.5 px-2 py-0.5 rounded-full text-[11px] font-mono bg-amber-500/10 text-amber-400 border border-amber-500/20">
                    {{ activeIncidents.length }}
                  </span>
                </h2>
              </div>
              <button 
                @click="navigate('incidents')"
                class="text-xs text-blue-400 hover:text-blue-300 hover:underline flex items-center gap-1"
              >
                <span>Tüm Olay Günlüğü</span>
                <span class="material-symbols-outlined text-[14px]">arrow_forward</span>
              </button>
            </div>

            <div v-if="activeIncidents.length" class="overflow-x-auto">
              <table class="w-full text-left text-xs border-collapse">
                <thead>
                  <tr class="bg-[#0c0e12]/60 text-txt-muted border-b border-[#232936] text-[11px] uppercase tracking-wider font-semibold">
                    <th class="py-2.5 px-3">Cihaz / Hedef</th>
                    <th class="py-2.5 px-3">Kategori</th>
                    <th class="py-2.5 px-3">IP / URL</th>
                    <th class="py-2.5 px-3">Başlangıç Zamanı</th>
                    <th class="py-2.5 px-3">Süre</th>
                    <th class="py-2.5 px-3 text-right">İşlem</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-[#1e2533]">
                  <tr v-for="i in activeIncidents.slice(0, 8)" :key="i.id" class="hover:bg-[#181e28]/70">
                    <td class="py-2.5 px-3">
                      <button 
                        @click="openIncidentDevice(i)"
                        class="text-white hover:text-blue-400 font-medium transition-colors text-left"
                      >
                        {{ i.device_name || deviceFor(i.device_id)?.name }}
                      </button>
                    </td>
                    <td class="py-2.5 px-3">
                      <span class="px-2 py-0.5 rounded text-[10px] bg-[#1a202c] border border-[#283142] text-txt-secondary">
                        {{ categoryLabel(deviceFor(i.device_id)) }}
                      </span>
                    </td>
                    <td class="py-2.5 px-3 font-mono text-txt-secondary">
                      {{ deviceFor(i.device_id)?.ip_address || '—' }}
                    </td>
                    <td class="py-2.5 px-3 text-txt-secondary">
                      {{ dateTime(i.started_at) }}
                    </td>
                    <td class="py-2.5 px-3 font-mono text-rose-400 font-medium">
                      {{ duration(i) }}
                    </td>
                    <td class="py-2.5 px-3 text-right">
                      <button 
                        @click="classifyTarget = i"
                        class="px-2 py-1 rounded bg-[#181e28] hover:bg-[#1f2736] text-txt-primary border border-[#232936] text-[11px] transition-colors"
                      >
                        Sınıflandır
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="p-6 text-center text-txt-muted text-xs">
              Görüntülenen kayıtlarda aktif kesinti bulunmamaktadır. Tüm sistemler erişilebilir durumda.
            </div>
          </section>

          <!-- Gateway ICMP & Security Scope Info -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <!-- Gateway ICMP Card -->
            <div class="p-5 rounded-lg bg-[#141820] border border-[#232936] flex flex-col justify-between gap-3 shadow-sm text-xs">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span class="material-symbols-outlined text-blue-400 text-[18px]">router</span>
                  <h3 class="font-semibold text-white">Ağ Geçidi ICMP Kontrolü</h3>
                </div>
                <button 
                  :disabled="gatewayLoading" 
                  @click="checkGateway"
                  class="px-2.5 py-1 rounded bg-[#181e28] hover:bg-[#1f2736] text-txt-primary border border-[#232936] flex items-center gap-1.5 transition-colors disabled:opacity-50"
                >
                  <span class="material-symbols-outlined text-[14px]" :class="{ 'animate-spin': gatewayLoading }">sync</span>
                  <span>{{ gatewayLoading ? 'Kontrol Ediliyor...' : 'Ping Testi Yap' }}</span>
                </button>
              </div>

              <div v-if="gateway" class="flex items-center justify-between p-3 rounded bg-[#0c0e12] border border-[#232936] font-mono">
                <div class="flex items-center gap-2">
                  <span class="w-2 h-2 rounded-full" :class="gateway.status === 'online' ? 'bg-emerald-400' : 'bg-rose-400'"></span>
                  <span class="text-white">{{ gateway.ip || '192.168.1.1' }}</span>
                </div>
                <span :class="gateway.status === 'online' ? 'text-emerald-400 font-semibold' : 'text-rose-400'">
                  {{ gateway.status === 'online' ? (gateway.latency_ms != null ? gateway.latency_ms + ' ms' : '< 1 ms') : 'Ulaşılamıyor' }}
                </span>
              </div>
              <p v-else class="text-txt-muted text-[11px]">
                {{ gatewayError || 'Ayarlar bölümünde tanımlı ağ geçidi IP adresine anlık işletim sistemi ICMP ping sorgusu gönderilir.' }}
              </p>
            </div>

            <!-- Telemetry Scope Card -->
            <div class="p-5 rounded-lg bg-[#141820] border border-[#232936] flex flex-col justify-between gap-3 shadow-sm text-xs">
              <div class="flex items-center gap-2">
                <span class="material-symbols-outlined text-cyan-400 text-[18px]">verified</span>
                <h3 class="font-semibold text-white">Telemetri & Güvenlik Sınırları</h3>
              </div>
              <p class="text-txt-secondary leading-relaxed text-[11px]">
                GNNwatchtower veri doğruluğu ilkesine bağlıdır: Gerçek CPU, bellek ve disk doluluk metrikleri yalnızca <strong>Ajan Özeti</strong> içindeki yetkilendirilmiş Windows/Linux ajanlarından toplanır. IP Cihazları (NAS, ESXi) kategorisinde izleme yalnızca ICMP echo sorgusuyla gerçekleştirilir; ek yönetim protokolü (SNMP, API, SSH) açılmaz.
              </p>
              <div class="flex items-center justify-end">
                <button 
                  @click="navigate('devices')"
                  class="text-blue-400 hover:text-blue-300 hover:underline flex items-center gap-1 text-xs"
                >
                  <span>Ajan Özeti'ne Git</span>
                  <span class="material-symbols-outlined text-[14px]">arrow_forward</span>
                </button>
              </div>
            </div>
          </div>
        </template>

        <!-- VIEW 2, 3, 4: Category Tables (devices, websites, ip-devices) -->
        <template v-if="viewInfo.mode && loaded">
          <!-- Security Notice for IP Devices -->
          <div v-if="currentView === 'ip-devices'" class="p-4 rounded-lg bg-cyan-950/20 border border-cyan-800/30 text-cyan-200 text-xs flex items-start gap-3 shadow-sm">
            <span class="material-symbols-outlined text-[20px] text-cyan-400 shrink-0 mt-0.5">shield</span>
            <div class="flex flex-col gap-1 leading-relaxed">
              <span class="font-semibold text-white">Sıfır Ek Saldırı Yüzeyi · Yalnızca ICMP Ping İle İzleme</span>
              <p class="text-txt-secondary text-[11px]">
                Bu kategorideki NAS, ESXi ve ağ anahtarlarında SNMP, cihaz API’leri, SSH veya ajan kullanılmaz; müşteri ortamında yeni dinleyici veya port açılmaz. Disk, RAID ve donanım sağlığı toplanmaz. En sık 60 saniyede bir tek ping gönderilir; yanıt için 3 saniye beklenir. Üç ardışık yanıtsız kontrolde erişim alarmı açılır; başarılı yanıtta kapanır. Firewall, EDR veya ağ kuralları ping yanıtını engelleyebilir: “ICMP ile ulaşılamıyor” cihazın kapalı olduğunu kanıtlamaz. Port ve ARP taraması yapılmaz.
              </p>
            </div>
          </div>

          <!-- Notice for Website Monitoring -->
          <div v-if="currentView === 'websites'" class="p-3.5 rounded-lg bg-emerald-950/20 border border-emerald-800/30 text-emerald-300 text-xs flex items-center gap-2.5 shadow-sm">
            <span class="material-symbols-outlined text-[18px] text-emerald-400 shrink-0">language</span>
            <span class="text-txt-secondary text-[11px]">
              Web İzleme HTTP(S) protokol yanıt kodlarını ve yanıt sürelerini denetler. Alan adı tescil süresi veya donanım sağlığı kapsam dışındadır.
            </span>
          </div>

          <!-- Category KPI Strip -->
          <div class="p-3.5 rounded-lg bg-[#141820] border border-[#232936] flex items-center justify-between flex-wrap gap-4 text-xs shadow-sm">
            <div class="flex items-center gap-5 flex-wrap">
              <span class="flex items-center gap-1.5">
                <span class="text-txt-muted">Toplam Hedef:</span>
                <strong class="font-mono text-white text-sm">{{ currentDevices.length }}</strong>
              </span>
              <span class="text-[#232936]">•</span>
              <span class="flex items-center gap-1.5 text-emerald-400">
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                <span>Erişilebilir:</span>
                <strong class="font-mono">{{ categoryStats(viewInfo.mode).online }}</strong>
              </span>
              <span class="text-[#232936]">•</span>
              <span class="flex items-center gap-1.5 text-rose-400">
                <span class="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
                <span>Ulaşılamıyor:</span>
                <strong class="font-mono">{{ categoryStats(viewInfo.mode).offline }}</strong>
              </span>
              <span class="text-[#232936]">•</span>
              <span class="flex items-center gap-1.5 text-txt-muted">
                <span>Diğer / Bakımda:</span>
                <strong class="font-mono text-txt-secondary">{{ categoryStats(viewInfo.mode).other }}</strong>
              </span>
            </div>

            <!-- In Ajan Özeti: Enrolled vs Awaiting -->
            <div v-if="currentView === 'devices'" class="flex items-center gap-3 text-[11px]">
              <span class="px-2 py-0.5 rounded bg-[#181e28] text-emerald-400 border border-emerald-500/20 font-mono">
                {{ installedCount }} Ajan Kurulu
              </span>
              <span class="px-2 py-0.5 rounded bg-[#181e28] text-txt-muted border border-[#232936] font-mono">
                {{ (counts.server || 0) - installedCount }} Ajan Bekleniyor
              </span>
            </div>
          </div>

          <!-- Table Container with Filter Toolbar -->
          <div class="rounded-lg bg-[#141820] border border-[#232936] flex flex-col overflow-hidden shadow-sm">
            <!-- Filter Toolbar -->
            <div class="p-3.5 border-b border-[#232936] flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
              <!-- Left: Sub-tabs (in Ajan Özeti) or Filters -->
              <div class="flex items-center gap-2 flex-wrap">
                <!-- OS Sub-tabs for Ajan Özeti -->
                <div v-if="currentView === 'devices'" class="flex items-center gap-1 bg-[#0c0e12] p-1 rounded-md border border-[#232936]">
                  <button 
                    v-for="t in [{ id: '', label: 'Tümü' }, { id: 'windows', label: 'Windows' }, { id: 'linux', label: 'Linux' }]" 
                    :key="t.id"
                    @click="filter.type = t.id"
                    :class="filter.type === t.id ? 'bg-[#1e2533] text-white font-medium shadow-sm' : 'text-txt-muted hover:text-txt-primary'"
                    class="px-2.5 py-1 rounded text-xs transition-colors"
                  >
                    {{ t.label }}
                  </button>
                </div>

                <!-- Status Filter Dropdown -->
                <select 
                  v-model="filter.status"
                  class="bg-[#0c0e12] border border-[#232936] rounded px-2.5 py-1 text-xs text-txt-primary focus:outline-none focus:border-blue-500"
                >
                  <option value="">Tüm Durumlar</option>
                  <option v-for="(label, state) in stateLabels" :key="state" :value="state">{{ label }}</option>
                </select>

                <!-- Hardware Type Filter Dropdown for IP Devices -->
                <select 
                  v-if="currentView === 'ip-devices'"
                  v-model="filter.type"
                  class="bg-[#0c0e12] border border-[#232936] rounded px-2.5 py-1 text-xs text-txt-primary focus:outline-none focus:border-blue-500"
                >
                  <option value="">Tüm Donanım Türleri</option>
                  <option value="__nas">Tüm NAS Cihazları (Synology / QNAP)</option>
                  <option v-for="t in inventoryTypes" :key="t" :value="t">{{ typeLabels[t] || t }}</option>
                </select>

                <!-- Reset Filters Button -->
                <button 
                  v-if="filter.query || filter.status || filter.type"
                  @click="filter.query = ''; filter.status = ''; filter.type = ''; searchQuery = ''"
                  class="px-2 py-1 rounded bg-[#181e28] hover:bg-[#1f2736] text-txt-muted hover:text-white border border-[#232936] transition-colors"
                  title="Filtreleri Temizle"
                >
                  Temizle
                </button>
              </div>

              <!-- Right: Record Count -->
              <span class="text-[11px] text-txt-muted">
                {{ filteredDevices.length }} kayıt listeleniyor
              </span>
            </div>

            <!-- Table Component -->
            <component 
              v-if="pageDevices.length" 
              :is="currentView === 'devices' ? ServerTable : currentView === 'websites' ? WebsiteTable : IpDeviceTable" 
              :devices="pageDevices" 
              :now="clock" 
              @edit="editDevice" 
              @delete="deleteDevice" 
              @profile="toggleProfile" 
              @incidents="showIncidents" 
              @select="loadMetrics" 
              @upgrade="upgrade" 
            />

            <!-- Empty State -->
            <div v-else-if="loaded" class="p-12 text-center text-xs flex flex-col items-center justify-center gap-3">
              <span class="material-symbols-outlined text-[36px] text-txt-muted">filter_list_off</span>
              <h3 class="font-semibold text-white">
                {{ currentDevices.length ? 'Filtreye Uyan Kayıt Bulunamadı' : 'Henüz Kayıt Eklenmedi' }}
              </h3>
              <p class="text-txt-muted max-w-sm">
                {{ currentDevices.length ? 'Arama terimini veya filtre parametrelerini temizleyip tekrar deneyin.' : 'Bu izleme kategorisine ilk hedefi elle ekleyerek başlayabilirsiniz.' }}
              </p>
              <button 
                v-if="!currentDevices.length"
                @click="openAdd(viewInfo.mode)"
                class="px-4 py-2 rounded-md bg-blue-600 hover:bg-blue-500 text-white font-medium transition-colors shadow-sm mt-1"
              >
                + İlk Kaydı Ekle
              </button>
            </div>

            <!-- Pagination Bar -->
            <div v-if="filteredDevices.length > pageSize" class="p-3 border-t border-[#232936] bg-[#0c0e12]/40 flex items-center justify-between text-xs">
              <span class="text-txt-muted">
                Toplam {{ filteredDevices.length }} kayıt · Sayfa {{ filter.page }} / {{ pages }}
              </span>
              <div class="flex items-center gap-1.5">
                <button 
                  :disabled="filter.page <= 1" 
                  @click="filter.page--"
                  class="px-2.5 py-1 rounded bg-[#181e28] hover:bg-[#1f2736] text-txt-primary border border-[#232936] disabled:opacity-40 transition-colors"
                >
                  Önceki
                </button>
                <button 
                  :disabled="filter.page >= pages" 
                  @click="filter.page++"
                  class="px-2.5 py-1 rounded bg-[#181e28] hover:bg-[#1f2736] text-txt-primary border border-[#232936] disabled:opacity-40 transition-colors"
                >
                  Sonraki
                </button>
              </div>
            </div>
          </div>

          <!-- Telemetry Inspector Drawer -->
          <TelemetryInspector 
            v-if="selectedDevice" 
            :device="selectedDevice" 
            :now="clock" 
            :metrics="selectedMetrics" 
            :loading="metricsLoading" 
            :error="metricsError" 
            @close="selectedID = null; metricsEpoch++" 
            @changed="refresh" 
          />
        </template>

        <!-- VIEW 5: Incidents (Olay Günlüğü) -->
        <section v-if="currentView === 'incidents'" class="rounded-lg bg-[#141820] border border-[#232936] flex flex-col overflow-hidden shadow-sm text-xs">
          <!-- Filters Toolbar -->
          <div class="p-4 border-b border-[#232936] flex items-center gap-3 flex-wrap">
            <input 
              v-model="filter.query" 
              placeholder="Cihaz veya hedef IP ara..." 
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-xs text-txt-primary placeholder-txt-muted focus:outline-none focus:border-blue-500 w-56"
            />
            <select 
              v-model="incidentFilter.category" 
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-xs text-txt-primary focus:outline-none focus:border-blue-500"
            >
              <option value="">Tüm Kategoriler</option>
              <option v-for="s in categorySections" :key="s.mode" :value="s.mode">{{ s.label }}</option>
            </select>
            <select 
              v-model="incidentFilter.state" 
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-xs text-txt-primary focus:outline-none focus:border-blue-500"
            >
              <option value="">Tüm Olaylar</option>
              <option value="open">Yalnızca Açık Olaylar</option>
              <option value="resolved">Yalnızca Çözülenler</option>
            </select>
            <select 
              v-model="incidentFilter.device" 
              class="bg-[#0c0e12] border border-[#232936] rounded px-3 py-1.5 text-xs text-txt-primary focus:outline-none focus:border-blue-500"
            >
              <option value="">Tüm Cihazlar</option>
              <option v-for="d in devices" :key="d.id" :value="String(d.id)">{{ d.name }}</option>
            </select>
          </div>

          <!-- Incidents Table -->
          <div v-if="filteredIncidents.length" class="overflow-x-auto">
            <table class="w-full text-left border-collapse text-xs">
              <thead>
                <tr class="bg-[#0c0e12]/60 text-txt-muted border-b border-[#232936] text-[11px] uppercase tracking-wider font-semibold select-none">
                  <th class="py-3 px-4">Cihaz & Kategori</th>
                  <th class="py-3 px-4">Başlangıç</th>
                  <th class="py-3 px-4">Bitiş</th>
                  <th class="py-3 px-4">Süre</th>
                  <th class="py-3 px-4">Değerlendirme Sebebi</th>
                  <th class="py-3 px-4 text-right">İşlem</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-[#1e2533]">
                <tr v-for="i in filteredIncidents" :key="i.id" class="hover:bg-[#181e28]/70">
                  <td class="py-3 px-4">
                    <div class="flex flex-col">
                      <button 
                        class="text-left font-medium text-white hover:text-blue-400 transition-colors"
                        @click="openIncidentDevice(i)"
                      >
                        {{ i.device_name || deviceFor(i.device_id)?.name || 'Kayıtlı Cihaz' }}
                      </button>
                      <span class="text-[11px] text-txt-muted">
                        {{ categoryLabel(deviceFor(i.device_id)) }} · {{ deviceFor(i.device_id)?.ip_address }}
                      </span>
                    </div>
                  </td>
                  <td class="py-3 px-4 text-txt-secondary font-mono">{{ dateTime(i.started_at) }}</td>
                  <td class="py-3 px-4 text-txt-secondary font-mono">
                    <span :class="i.resolved_at ? '' : 'text-rose-400 font-semibold'">
                      {{ i.resolved_at ? dateTime(i.resolved_at) : 'Açık Kesinti' }}
                    </span>
                  </td>
                  <td class="py-3 px-4 font-mono font-medium text-txt-primary">{{ duration(i) }}</td>
                  <td class="py-3 px-4">
                    <div class="flex flex-col gap-0.5">
                      <span class="font-medium text-txt-primary">{{ causeLabel(i.classified_cause) }}</span>
                      <span class="text-[10px] text-txt-muted">
                        {{ i.operator_confirmed ? 'Operatörce Doğrulandı' : 'Otomatik Tahmin' }}
                      </span>
                      <span v-if="i.auto_detection_hypothesis" class="text-[10px] text-amber-400 font-mono">
                        Hipotez: {{ i.auto_detection_hypothesis }}
                      </span>
                    </div>
                  </td>
                  <td class="py-3 px-4 text-right">
                    <button 
                      @click="classifyTarget = i"
                      class="px-2.5 py-1 rounded bg-[#181e28] hover:bg-[#1f2736] text-txt-primary border border-[#232936] text-xs transition-colors"
                    >
                      Sınıflandır
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-else-if="loaded" class="p-12 text-center text-txt-muted text-xs">
            Filtreleme kriterlerine uyan olay kaydı bulunamadı.
          </div>
          <div class="p-3 border-t border-[#232936] bg-[#0c0e12]/40 text-[11px] text-txt-muted">
            En yeni 200 olay kaydı gösterilmektedir. Otomatik tespit hipotezi kesin donanım arızası kanıtı değildir.
          </div>
        </section>

        <!-- VIEW 6: Backups (Yedekleme Merkezi) -->
        <div v-if="currentView === 'backups'" class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <!-- Export Card -->
          <section class="rounded-lg bg-[#141820] border border-[#232936] p-6 flex flex-col justify-between gap-5 shadow-sm text-xs">
            <div class="flex flex-col gap-2">
              <div class="flex items-center gap-2.5 text-blue-400">
                <span class="material-symbols-outlined text-[22px]">download</span>
                <h2 class="text-sm font-semibold text-white">Yedek Dosyasını İndir</h2>
              </div>
              <p class="text-txt-secondary leading-relaxed text-[11px]">
                Sistem ayarları, bildirim kanalları, kayıtlı cihazlar, servis tanımları ve son 1000 olay günlüğü JSON formatında şifreli olarak dışa aktarılır. Ajan geçmişi disk tasarrufu gereği yedeğe dahil edilmez.
              </p>
              <div class="p-3 rounded bg-amber-500/10 border border-amber-500/20 text-amber-300 text-[11px] flex items-center gap-2 mt-1">
                <span class="material-symbols-outlined text-[16px]">security</span>
                <span>Yedek dosyası hassas erişim ayarları içerebilir; güvenli bir depolama konumunda saklayınız.</span>
              </div>
            </div>

            <button 
              :disabled="backupBusy" 
              @click="downloadBackup"
              class="px-5 py-2.5 rounded-md bg-blue-600 hover:bg-blue-500 text-white font-medium transition-colors shadow-sm flex items-center justify-center gap-2 disabled:opacity-50"
            >
              <span class="material-symbols-outlined text-[16px]" :class="{ 'animate-spin': backupBusy }">download</span>
              <span>{{ backupBusy ? 'İşleniyor...' : 'JSON Yedeğini İndir' }}</span>
            </button>
          </section>

          <!-- Import Card -->
          <section class="rounded-lg bg-[#141820] border border-[#232936] p-6 flex flex-col justify-between gap-5 shadow-sm text-xs">
            <div class="flex flex-col gap-2">
              <div class="flex items-center gap-2.5 text-emerald-400">
                <span class="material-symbols-outlined text-[22px]">upload</span>
                <h2 class="text-sm font-semibold text-white">Yedeği Geri Yükle</h2>
              </div>
              <p class="text-txt-secondary leading-relaxed text-[11px]">
                Daha önce alınmış bir JSON yedek dosyasını yükleyerek cihaz envanterini ve bildirim kanallarını geri yükleyin. Mevcut kayıtlar hedef adresine göre eşleştirilir.
              </p>
            </div>

            <div class="flex flex-col gap-2">
              <label class="flex flex-col items-center justify-center border-2 border-dashed border-[#232936] hover:border-blue-500/50 rounded-lg p-6 bg-[#0c0e12] cursor-pointer transition-colors group">
                <span class="material-symbols-outlined text-[32px] text-txt-muted group-hover:text-blue-400 transition-colors">upload_file</span>
                <span class="mt-2 text-txt-primary font-medium text-xs">JSON Yedek Dosyasını Seçin</span>
                <span class="text-[10px] text-txt-muted mt-1">Maksimum 2 MB</span>
                <input 
                  type="file" 
                  accept="application/json,.json" 
                  :disabled="backupBusy" 
                  @change="restoreBackup" 
                  class="hidden"
                />
              </label>
            </div>
          </section>
        </div>
      </main>
    </div>

    <!-- Category Chooser Modal for + Cihaz Ekle -->
    <div v-if="chooserOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm select-none" @click.self="chooserOpen = false">
      <div class="bg-[#141820] border border-[#232936] rounded-xl max-w-lg w-full p-5 flex flex-col gap-4 shadow-2xl text-xs">
        <div class="flex items-center justify-between pb-3 border-b border-[#232936]">
          <h2 class="text-sm font-semibold text-white">Yeni İzleme Hedefi Ekle</h2>
          <button @click="chooserOpen = false" class="p-1 rounded text-txt-muted hover:text-white hover:bg-[#1d2432]">
            <span class="material-symbols-outlined text-[18px]">close</span>
          </button>
        </div>

        <p class="text-txt-secondary text-xs">
          İzlemek istediğiniz cihaz veya servis türünü seçiniz:
        </p>

        <div class="flex flex-col gap-2.5">
          <!-- Choice 1: Server -->
          <button 
            @click="openAdd('server')"
            class="p-3.5 rounded-lg bg-[#0c0e12] hover:bg-[#181e28] border border-[#232936] hover:border-blue-500/50 flex items-center gap-3.5 transition-all text-left group"
          >
            <div class="w-9 h-9 rounded-lg bg-blue-600/10 border border-blue-500/20 flex items-center justify-center text-blue-400 shrink-0">
              <span class="material-symbols-outlined text-[20px]">dns</span>
            </div>
            <div class="flex flex-col">
              <span class="font-semibold text-white group-hover:text-blue-400 transition-colors">Sunucu (Windows / Linux)</span>
              <span class="text-[11px] text-txt-muted">Ajan kurulumu ile CPU/RAM/Disk kullanımı ve port izleme</span>
            </div>
          </button>

          <!-- Choice 2: Website -->
          <button 
            @click="openAdd('website')"
            class="p-3.5 rounded-lg bg-[#0c0e12] hover:bg-[#181e28] border border-[#232936] hover:border-emerald-500/50 flex items-center gap-3.5 transition-all text-left group"
          >
            <div class="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 shrink-0">
              <span class="material-symbols-outlined text-[20px]">language</span>
            </div>
            <div class="flex flex-col">
              <span class="font-semibold text-white group-hover:text-emerald-400 transition-colors">Web Sitesi / URL</span>
              <span class="text-[11px] text-txt-muted">HTTP / HTTPS yanıt kodu ve yanıt süresi kontrolleri</span>
            </div>
          </button>

          <!-- Choice 3: IP Device -->
          <button 
            @click="openAdd('icmp')"
            class="p-3.5 rounded-lg bg-[#0c0e12] hover:bg-[#181e28] border border-[#232936] hover:border-cyan-500/50 flex items-center gap-3.5 transition-all text-left group"
          >
            <div class="w-9 h-9 rounded-lg bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400 shrink-0">
              <span class="material-symbols-outlined text-[20px]">router</span>
            </div>
            <div class="flex flex-col">
              <span class="font-semibold text-white group-hover:text-cyan-400 transition-colors">IP Cihazı (NAS / ESXi / Ağ Cihazı)</span>
              <span class="text-[11px] text-txt-muted">Sıfır port açılımı ile yalnızca ICMP ping yoklaması</span>
            </div>
          </button>
        </div>
      </div>
    </div>

    <!-- Modals -->
    <AgentModal 
      v-if="currentView === 'devices' && agentOpen && !loginOpen && !passwordForced" 
      :is-open="true" 
      @close="agentOpen = false" 
    />
    <AddDeviceModal 
      v-if="addOpen && !loginOpen" 
      :is-open="true" 
      :edit-device="editTarget" 
      :initial-mode="initialMode" 
      @close="addOpen = false; editTarget = null" 
      @saved="saved" 
    />
    <ClassifyModal 
      v-if="classifyTarget" 
      :is-open="true" 
      :incident="classifyTarget" 
      @close="classifyTarget = null" 
      @classified="refresh" 
    />
    <SettingsModal 
      v-if="settingsOpen" 
      :is-open="true" 
      @close="settingsOpen = false" 
      @saved="refresh" 
    />
    <ChangePasswordModal 
      v-if="passwordOpen" 
      :is-open="true" 
      :forced="passwordForced" 
      @close="!passwordForced && (passwordOpen = false)" 
      @password-changed="passwordChanged" 
    />
    <LoginModal 
      v-if="loginOpen" 
      :is-open="true" 
      @logged-in="loggedIn" 
    />
  </div>
</template>
