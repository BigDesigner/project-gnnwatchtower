<script setup>
import DeviceStatus from "./DeviceStatus.vue";
import DeviceActions from "./DeviceActions.vue";
import { dateTime, latency, metric, typeLabels, observation, telemetryCurrent } from "../devicePolicy";
defineProps({ now: Number, devices: { type: Array, default: () => [] } });
defineEmits(["edit", "delete", "profile", "incidents", "select", "upgrade"]);
</script>
<template>
  <div class="wt-table-scroll">
    <table class="wt-table wt-server-table">
      <thead>
        <tr>
          <th>Cihaz / İşletim Sistemi</th>
          <th>IP / Erişilebilirlik</th>
          <th>Ajan Telemetrisi</th>
          <th>CPU</th>
          <th>Bellek</th>
          <th>Disk</th>
          <th>Son Kontrol</th>
          <th>İşlemler</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="d in devices" :key="d.id">
          <td>
            <button class="wt-link" @click="$emit('select',d)">{{ d.name }}</button>
            <small>{{ typeLabels[d.device_type] || d.device_type }}</small>
          </td>
          <td>
            <span class="wt-mono">{{ d.ip_address }}</span>
            <small>
              <DeviceStatus :device="d" :now="now" />
            </small>
          </td>
          <td>
            {{ !d.is_agent_installed ? 'Ajan Bekleniyor' : telemetryCurrent(d,now) ? 'Güncel Veri' : 'Güncel Veri Yok' }}
            <small v-if="d.is_agent_installed">v{{ d.agent_version }} · {{ dateTime(d.latest_metric_at) }}</small>
          </td>
          <td>{{ metric(d,'latest_cpu') }}</td>
          <td>{{ metric(d,'latest_ram') }}</td>
          <td>{{ metric(d,'latest_disk') }}</td>
          <td>
            {{ dateTime(d.last_checked_at) }}
            <small>Ping: {{ latency(d) }}</small>
          </td>
          <td>
            <DeviceActions :device="d" @edit="$emit('edit',$event)" @delete="$emit('delete',$event)" @profile="$emit('profile',$event)" @incidents="$emit('incidents',$event)" />
            <button v-if="d.is_agent_installed" class="wt-link" @click="$emit('upgrade',d)">Ajanı Güncelle</button>
            <small v-if="d.agent_upgrade_status && d.agent_upgrade_status !== 'none'">{{ ({pending:'Güncelleme bekliyor',downloading:'İndiriliyor',verifying:'Doğrulanıyor',applying:'Uygulanıyor',failed:'Güncelleme başarısız',success:'Güncellendi'})[d.agent_upgrade_status] || 'Güncelleme sürüyor' }}</small>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>