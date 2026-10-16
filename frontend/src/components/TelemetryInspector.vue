<template>
  <section v-if="device && device.monitoring_mode === 'server'" class="rounded-lg bg-[#141820] border border-[#232936] p-5 flex flex-col gap-5 shadow-xl text-xs">
    <!-- Header -->
    <div class="flex items-center justify-between pb-3.5 border-b border-[#232936]">
      <div class="flex items-center gap-3 flex-wrap">
        <span class="text-xs text-txt-muted">Seçili Sunucu Telemetrisi:</span>
        <div class="flex items-center gap-2">
          <span 
            class="w-2 h-2 rounded-full"
            :class="telemetryCurrent(device, now) ? 'bg-emerald-400 led-pulse-emerald' : 'bg-slate-500'"
          ></span>
          <span class="text-sm font-semibold text-white">{{ device.name }}</span>
          <span class="font-mono text-txt-muted text-xs">({{ device.ip_address }})</span>
        </div>
        <span class="text-[11px] font-mono px-2 py-0.5 rounded bg-[#181e28] text-txt-secondary border border-[#232936]">
          Ajan v{{ device.agent_version || '0.4.11' }}
        </span>
      </div>

      <button 
        @click="$emit('close')" 
        class="p-1 rounded text-txt-muted hover:text-white hover:bg-[#1d2432] transition-colors"
        title="Paneli Kapat"
        aria-label="Kapat"
      >
        <span class="material-symbols-outlined text-[18px]">close</span>
      </button>
    </div>

    <!-- Telemetry Metric Cards -->
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-3.5">
      <!-- CPU -->
      <div class="p-3.5 rounded-lg bg-[#0c0e12] border border-[#232936] flex flex-col gap-2">
        <div class="flex items-center justify-between text-txt-muted text-[11px]">
          <span>İşlemci (CPU)</span>
          <span class="material-symbols-outlined text-[15px] text-blue-400">memory</span>
        </div>
        <div class="flex items-baseline justify-between">
          <span class="text-2xl font-mono font-semibold" :class="getTextColor(device.latest_cpu)">
            {{ metric(device, 'latest_cpu') }}
          </span>
          <span class="text-[10px] text-txt-muted">Yük</span>
        </div>
        <div class="w-full h-1.5 bg-[#181e28] rounded-full overflow-hidden">
          <div 
            class="h-full rounded-full transition-all duration-500"
            :class="getBarColor(device.latest_cpu)"
            :style="{ width: Math.min(100, Math.max(0, device.latest_cpu || 0)) + '%' }"
          ></div>
        </div>
      </div>

      <!-- RAM -->
      <div class="p-3.5 rounded-lg bg-[#0c0e12] border border-[#232936] flex flex-col gap-2">
        <div class="flex items-center justify-between text-txt-muted text-[11px]">
          <span>Bellek (RAM)</span>
          <span class="material-symbols-outlined text-[15px] text-emerald-400">developer_board</span>
        </div>
        <div class="flex items-baseline justify-between">
          <span class="text-2xl font-mono font-semibold" :class="getTextColor(device.latest_ram)">
            {{ metric(device, 'latest_ram') }}
          </span>
          <span class="text-[10px] text-txt-muted">Kullanım</span>
        </div>
        <div class="w-full h-1.5 bg-[#181e28] rounded-full overflow-hidden">
          <div 
            class="h-full rounded-full transition-all duration-500"
            :class="getBarColor(device.latest_ram)"
            :style="{ width: Math.min(100, Math.max(0, device.latest_ram || 0)) + '%' }"
          ></div>
        </div>
      </div>

      <!-- Disk -->
      <div class="p-3.5 rounded-lg bg-[#0c0e12] border border-[#232936] flex flex-col gap-2">
        <div class="flex items-center justify-between text-txt-muted text-[11px]">
          <span>Disk Doluluğu</span>
          <span class="material-symbols-outlined text-[15px] text-cyan-400">storage</span>
        </div>
        <div class="flex items-baseline justify-between">
          <span class="text-2xl font-mono font-semibold" :class="getTextColor(device.latest_disk)">
            {{ metric(device, 'latest_disk') }}
          </span>
          <span class="text-[10px] text-txt-muted">Kullanılan Alan</span>
        </div>
        <div class="w-full h-1.5 bg-[#181e28] rounded-full overflow-hidden">
          <div 
            class="h-full rounded-full transition-all duration-500"
            :class="getBarColor(device.latest_disk)"
            :style="{ width: Math.min(100, Math.max(0, device.latest_disk || 0)) + '%' }"
          ></div>
        </div>
      </div>
    </div>

    <!-- Data Veracity Note -->
    <div class="text-[11px] text-txt-muted flex items-center gap-1.5 bg-[#0c0e12]/60 px-3 py-2 rounded border border-[#232936]">
      <span class="material-symbols-outlined text-[14px] text-txt-secondary">info</span>
      <span>Kaynak: Cihazın yerel ajanından toplanan gerçek telemetridir. Disk doluluğu disk arızasını veya RAID sağlığını göstermez.</span>
    </div>

    <!-- Monitored Services Section -->
    <div class="flex flex-col gap-3 pt-2 border-t border-[#232936]">
      <div class="flex items-center justify-between">
        <span class="font-medium text-txt-primary">İzlenen Port & Servisler (TCP/HTTP)</span>
        <span class="text-[11px] text-txt-muted">{{ (device.services || []).length }} Servis Tanımlı</span>
      </div>

      <!-- Existing Services -->
      <div v-if="(device.services || []).length" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
        <div 
          v-for="s in device.services" 
          :key="s.id"
          class="p-2.5 rounded bg-[#0c0e12] border border-[#232936] flex items-center justify-between"
        >
          <div class="flex items-center gap-2">
            <span 
              class="w-2 h-2 rounded-full"
              :class="s.status === 'online' ? 'bg-emerald-400' : 'bg-rose-400'"
            ></span>
            <div class="flex flex-col">
              <span class="font-medium text-txt-primary">{{ s.name }}</span>
              <span class="text-[10px] font-mono text-txt-muted">
                {{ s.check_type || 'TCP' }}: {{ s.target_port }}
              </span>
            </div>
          </div>
          <button 
            @click="removeService(s)"
            :disabled="busy"
            class="p-1 rounded text-txt-muted hover:text-rose-400 hover:bg-rose-950/20 transition-colors"
            title="Servis izlemesini kaldır"
          >
            <span class="material-symbols-outlined text-[15px]">delete</span>
          </button>
        </div>
      </div>
      <p v-else class="text-txt-muted text-xs italic">
        Henüz özel bir TCP portu veya servis izlemesi tanımlanmadı.
      </p>

      <!-- Add Service Inline Form -->
      <form @submit.prevent="addService" class="mt-1 flex flex-wrap items-center gap-2 bg-[#0c0e12] p-2.5 rounded border border-[#232936]">
        <input 
          v-model.trim="serviceName" 
          required 
          placeholder="Servis adı (örn: SSH, Nginx)" 
          class="bg-[#141820] border border-[#232936] rounded px-2.5 py-1 text-xs text-txt-primary placeholder-txt-muted focus:outline-none focus:border-blue-500 flex-1 min-w-[140px]"
        />
        <input 
          v-model.number="port" 
          required 
          type="number" 
          min="1" 
          max="65535" 
          placeholder="Port (örn: 22, 80)" 
          class="bg-[#141820] border border-[#232936] rounded px-2.5 py-1 text-xs font-mono text-txt-primary placeholder-txt-muted focus:outline-none focus:border-blue-500 w-28"
        />
        <select 
          v-model="serviceType"
          class="bg-[#141820] border border-[#232936] rounded px-2 py-1 text-xs text-txt-primary focus:outline-none focus:border-blue-500"
        >
          <option value="tcp_port">TCP Port</option>
          <option value="http">HTTP Sağlık</option>
        </select>
        <button 
          type="submit" 
          :disabled="busy"
          class="px-3 py-1 rounded bg-blue-600 hover:bg-blue-500 text-white font-medium transition-colors disabled:opacity-50"
        >
          + Servis Ekle
        </button>
      </form>
      <span v-if="serviceError" class="text-rose-400 text-[11px]">{{ serviceError }}</span>
    </div>

    <!-- Recent Telemetry Samples Table -->
    <div v-if="metrics.length" class="flex flex-col gap-2 pt-2 border-t border-[#232936]">
      <span class="font-medium text-txt-primary">Son Ölçüm Geçmişi (En Son 10 Kayıt)</span>
      <div class="overflow-x-auto rounded border border-[#232936]">
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="bg-[#0c0e12] text-txt-muted border-b border-[#232936] text-[11px]">
              <th class="py-2 px-3">Zaman Damgası</th>
              <th class="py-2 px-3 text-center">CPU</th>
              <th class="py-2 px-3 text-center">Bellek</th>
              <th class="py-2 px-3 text-center">Disk</th>
              <th class="py-2 px-3 text-right">Durum</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[#1e2533]">
            <tr v-for="(m, idx) in metrics.slice(0, 10)" :key="idx" class="hover:bg-[#181e28]">
              <td class="py-2 px-3 font-mono text-txt-secondary">{{ dateTime(m.timestamp || m.created_at) }}</td>
              <td class="py-2 px-3 text-center font-mono" :class="getTextColor(m.cpu_usage)">{{ m.cpu_usage != null ? m.cpu_usage.toFixed(1) + '%' : '—' }}</td>
              <td class="py-2 px-3 text-center font-mono" :class="getTextColor(m.ram_usage)">{{ m.ram_usage != null ? m.ram_usage.toFixed(1) + '%' : '—' }}</td>
              <td class="py-2 px-3 text-center font-mono" :class="getTextColor(m.disk_usage)">{{ m.disk_usage != null ? m.disk_usage.toFixed(1) + '%' : '—' }}</td>
              <td class="py-2 px-3 text-right">
                <span class="text-emerald-400 text-[11px]">Raporlandı</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref } from "vue";
import api from "../services/api";
import { dateTime, metric, apiError, telemetryCurrent } from "../devicePolicy";

const props = defineProps({ 
  now: Number, 
  device: Object, 
  metrics: { type: Array, default: () => [] }, 
  loading: Boolean, 
  error: String 
});

const emit = defineEmits(["close", "changed"]);

const serviceName = ref("");
const port = ref(null);
const serviceType = ref("tcp_port");
const servicePath = ref("");
const serviceError = ref("");
const busy = ref(false);

function getTextColor(val) {
  if (val >= 85) return 'text-rose-400';
  if (val >= 70) return 'text-amber-400';
  return 'text-emerald-400';
}

function getBarColor(val) {
  if (val >= 85) return 'bg-rose-500';
  if (val >= 70) return 'bg-amber-500';
  return 'bg-emerald-500';
}

async function addService() {
  if (props.device?.monitoring_mode !== "server") return;
  busy.value = true;
  serviceError.value = "";
  try {
    await api.addService(props.device.id, { 
      name: serviceName.value, 
      target_port: port.value, 
      check_type: serviceType.value, 
      target_path: servicePath.value, 
      expected_status_code: 200 
    });
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
  if (props.device?.monitoring_mode !== "server" || !confirm(s.name + " servis izlemesi kaldırılsın mı?")) return;
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
