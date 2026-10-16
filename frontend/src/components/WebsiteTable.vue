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
          <th>Web Sitesi</th>
          <th>URL</th>
          <th>Erişilebilirlik</th>
          <th>HTTP Sonucu</th>
          <th>Yanıt Süresi</th>
          <th>Son Kontrol</th>
          <th>İşlemler</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="d in devices" :key="d.id">
          <td>
            <strong>{{ d.name }}</strong>
          </td>
          <td class="wt-mono wt-target">{{ d.ip_address }}</td>
          <td>
            <DeviceStatus :device="d" :now="now" />
          </td>
          <td>
            {{ d.last_checked_at ? (d.http_result || 'Yanıt bilgisi yok') : 'Kontrol bekleniyor' }}
            <small v-if="observation(d) === 'stale'">Önceki gözlem</small>
          </td>
          <td class="wt-mono">{{ latency(d) }}</td>
          <td>{{ dateTime(d.last_checked_at) }}</td>
          <td>
            <DeviceActions :device="d" @edit="$emit('edit',$event)" @delete="$emit('delete',$event)" @profile="$emit('profile',$event)" @incidents="$emit('incidents',$event)" />
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>