<script setup>
import DeviceStatus from "./DeviceStatus.vue";
import DeviceActions from "./DeviceActions.vue";
import { dateTime, latency, metric, typeLabels, observation, telemetryCurrent } from "../devicePolicy";
defineProps({ now: Number, devices: { type: Array, default: () => [] } });
defineEmits(["edit", "delete", "profile", "incidents", "select", "upgrade"]);
</script>
<template>
  <div class="wt-table-scroll">
    <table class="wt-table">
      <thead>
        <tr>
          <th>Cihaz</th>
          <th>Tür</th>
          <th>IP Adresi</th>
          <th>Erişilebilirlik</th>
          <th>Ping Süresi</th>
          <th>Son Kontrol</th>
          <th>İşlemler</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="d in devices" :key="d.id">
          <td>
            <strong>{{ d.name }}</strong>
            <small>{{ d.location }}</small>
          </td>
          <td>{{ typeLabels[d.device_type] || d.device_type }}</td>
          <td class="wt-mono">{{ d.ip_address }}</td>
          <td>
            <DeviceStatus :device="d" :now="now" />
          </td>
          <td class="wt-mono">{{ latency(d) }}</td>
          <td>
            {{ dateTime(d.last_checked_at) }}
            <small v-if="d.probe_error">{{ d.probe_error }}</small>
            <small>Son yanıt: {{ dateTime(d.last_seen_at) }}</small>
          </td>
          <td>
            <DeviceActions :device="d" @edit="$emit('edit',$event)" @delete="$emit('delete',$event)" @profile="$emit('profile',$event)" @incidents="$emit('incidents',$event)" />
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>