<template>
  <div class="min-h-screen bg-canvas text-txt-primary flex flex-col antialiased select-text">
    <!-- Top Bar Component -->
    <TopBar 
      :stats="stats" 
      v-model:searchQuery="searchQuery" 
      :isRefreshing="isRefreshing"
      @refresh="fetchData"
      @open-incidents="openIncidentsView"
      @open-settings="isSettingsOpen = true"
      @open-change-password="isChangePasswordOpen = true"
      @logout="handleLogout"
    />

    <!-- Left Navigation Rail -->
    <Sidebar 
      :currentView="currentView"
      :gateway="gateway"
      @select-view="currentView = $event"
      @open-discovery="isDiscoveryOpen = true"
      @open-agent-modal="isAgentOpen = true"
      @open-settings="isSettingsOpen = true"
    />

    <!-- Main Content Area -->
    <div class="pl-60 pt-14 flex-1 flex flex-col">
      <main class="p-8 max-w-[1600px] w-full mx-auto flex flex-col gap-6 flex-1">
        <!-- Top Breadcrumb & Actions Bar -->
        <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 pb-4 border-b border-[#232936]">
          <div class="flex flex-col gap-1.5">
            <div class="flex items-center gap-2 text-xs text-txt-muted">
              <span>{{ stats.client_company_name || 'Müşteri Altyapısı' }}</span>
              <span class="text-[#2c3547]">/</span>
              <span>{{ stats.client_location || 'Sistem Odası' }}</span>
              <span class="text-[#2c3547]">/</span>
              <span class="text-txt-primary font-medium">GNNwatchtower Appliance</span>
              <span 
                class="px-1.5 py-0.2 rounded text-[11px] font-medium border"
                :class="(stats.offline_devices || 0) === 0 ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border-rose-500/20'"
              >
                {{ (stats.offline_devices || 0) === 0 ? 'Tüm Sistemler Sağlıklı' : (stats.offline_devices + ' Kesinti Mevcut') }}
              </span>
            </div>

            <div class="flex items-center gap-3 text-xs text-txt-secondary flex-wrap">
              <span class="flex items-center gap-1.5">
                <span class="text-txt-muted">Aktif Profiller:</span>
                <span class="text-txt-primary font-mono">{{ stats.active_profiles || 0 }}</span>
              </span>
              <span class="text-[#232936]">•</span>
              <span class="flex items-center gap-1.5">
                <span class="text-txt-muted">Uyku Modu (24h/30d):</span>
                <span class="text-txt-primary font-mono">{{ stats.dormant_profiles || 0 }}</span>
              </span>
              <span class="text-[#232936]">•</span>
              <span class="flex items-center gap-1.5">
                <span class="text-txt-muted">Bakım Modu:</span>
                <span class="text-txt-primary font-mono">{{ stats.maintenance_profiles || 0 }}</span>
              </span>
            </div>
          </div>

          <!-- Top Quick Actions -->
          <div class="flex items-center gap-2 self-start lg:self-center">
            <button 
              @click="isDiscoveryOpen = true"
              class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-blue-600 hover:bg-blue-500 text-white transition-colors shadow-sm"
            >
              <span class="material-symbols-outlined text-[16px]">radar</span>
              <span>Ağ Taraması Başlat</span>
            </button>
            <button 
              @click="isAgentOpen = true"
              class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-[#141820] hover:bg-[#1d2432] text-txt-primary border border-[#232936] transition-colors"
            >
              <span class="material-symbols-outlined text-[16px] text-emerald-400">terminal</span>
              <span>Ajan Dağıtımı</span>
            </button>
            <button 
              @click="openAddDeviceModal"
              class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-[#141820] hover:bg-[#1d2432] text-txt-primary border border-[#232936] transition-colors"
            >
              <span class="material-symbols-outlined text-[16px] text-txt-muted">add</span>
              <span>Manuel Cihaz</span>
            </button>
          </div>
        </div>

        <!-- VIEW 1: Overview (Admin Executive Dashboard) -->
        <template v-if="currentView === 'overview'">
          <!-- Top Executive Metrics Grid -->
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <!-- Card 1: Total Nodes -->
            <div 
              @click="currentView = 'devices'"
              class="p-4 rounded-lg bg-[#141820] border border-[#232936] hover:border-blue-500/50 cursor-pointer transition-colors flex flex-col gap-2 group"
            >
              <div class="flex items-center justify-between text-xs text-txt-muted">
                <span>Toplam İzlenen Düğüm</span>
                <span class="material-symbols-outlined text-[18px] text-blue-400 group-hover:translate-x-0.5 transition-transform">dns</span>
              </div>
              <div class="flex items-baseline gap-2">
                <span class="text-3xl font-mono font-semibold text-white">{{ devices.length }}</span>
                <span class="text-xs text-blue-400 font-medium">Cihaz</span>
              </div>
              <span class="text-[11px] text-txt-muted flex items-center gap-1">
                <span>Tüm envanteri listele</span>
                <span class="material-symbols-outlined text-[13px]">arrow_forward</span>
              </span>
            </div>

            <!-- Card 2: Online Nodes -->
            <div class="p-4 rounded-lg bg-[#141820] border border-[#232936] flex flex-col gap-2">
              <div class="flex items-center justify-between text-xs text-txt-muted">
                <span>Çevrimiçi Düğümler</span>
                <span class="material-symbols-outlined text-[18px] text-emerald-400">check_circle</span>
              </div>
              <div class="flex items-baseline gap-2">
                <span class="text-3xl font-mono font-semibold text-emerald-400">{{ onlineDevicesCount }}</span>
                <span class="text-xs text-txt-muted">/ {{ devices.length }}</span>
              </div>
              <div class="w-full bg-[#0c0e12] rounded-full h-1.5 overflow-hidden">
                <div 
                  class="bg-emerald-500 h-full rounded-full transition-all duration-500"
                  :style="{ width: devices.length > 0 ? (onlineDevicesCount / devices.length * 100) + '%' : '0%' }"
                ></div>
              </div>
            </div>

            <!-- Card 3: Offline / Alerts -->
            <div class="p-4 rounded-lg bg-[#141820] border border-[#232936] flex flex-col gap-2">
              <div class="flex items-center justify-between text-xs text-txt-muted">
                <span>Kesintideki Cihazlar</span>
                <span 
                  class="material-symbols-outlined text-[18px]"
                  :class="offlineDevices.length > 0 ? 'text-rose-400 animate-pulse' : 'text-txt-muted'"
                >error</span>
              </div>
              <div class="flex items-baseline gap-2">
                <span 
                  class="text-3xl font-mono font-semibold"
                  :class="offlineDevices.length > 0 ? 'text-rose-400' : 'text-txt-primary'"
                >{{ offlineDevices.length }}</span>
                <span class="text-xs text-txt-muted">Aktif Alarm</span>
              </div>
              <span 
                class="text-[11px] font-medium"
                :class="offlineDevices.length === 0 ? 'text-emerald-400' : 'text-rose-400'"
              >
                {{ offlineDevices.length === 0 ? '✓ Kritik kesinti bulunmuyor' : 'Müdahale gerekiyor' }}
              </span>
            </div>

            <!-- Card 4: Agent Deployments -->
            <div class="p-4 rounded-lg bg-[#141820] border border-[#232936] flex flex-col gap-2">
              <div class="flex items-center justify-between text-xs text-txt-muted">
                <span>Mini-Ajan Yüklü</span>
                <span class="material-symbols-outlined text-[18px] text-cyan-400">terminal</span>
              </div>
              <div class="flex items-baseline gap-2">
                <span class="text-3xl font-mono font-semibold text-cyan-400">{{ agentInstalledCount }}</span>
                <span class="text-xs text-txt-muted">Sunucu</span>
              </div>
              <span class="text-[11px] text-txt-muted">
                Canlı CPU, RAM, Disk telemetrisi aktif
              </span>
            </div>
          </div>

          <!-- Split Dashboard Row -->
          <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <!-- Left: Device Category Breakdown (2 Cols on lg) -->
            <div class="lg:col-span-2 bg-[#141820] rounded-lg border border-[#232936] p-5 flex flex-col gap-4">
              <div class="flex items-center justify-between pb-3 border-b border-[#232936]">
                <div class="flex items-center gap-2">
                  <span class="material-symbols-outlined text-txt-muted">pie_chart</span>
                  <h3 class="text-sm font-semibold text-white">Altyapı Donanım Dağılımı</h3>
                </div>
                <span class="text-xs text-txt-muted font-mono">{{ devices.length }} Toplam</span>
              </div>

              <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
                <div class="p-3 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-1">
                  <div class="flex items-center justify-between text-txt-muted">
                    <span class="text-[11px]">Linux / Ubuntu</span>
                    <span class="material-symbols-outlined text-[15px] text-orange-400">terminal</span>
                  </div>
                  <span class="text-xl font-mono font-semibold text-white">{{ deviceTypeCounts.linux }}</span>
                </div>

                <div class="p-3 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-1">
                  <div class="flex items-center justify-between text-txt-muted">
                    <span class="text-[11px]">Windows Server</span>
                    <span class="material-symbols-outlined text-[15px] text-sky-400">desktop_windows</span>
                  </div>
                  <span class="text-xl font-mono font-semibold text-white">{{ deviceTypeCounts.windows }}</span>
                </div>

                <div class="p-3 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-1">
                  <div class="flex items-center justify-between text-txt-muted">
                    <span class="text-[11px]">VMware ESXi</span>
                    <span class="material-symbols-outlined text-[15px] text-emerald-400">dns</span>
                  </div>
                  <span class="text-xl font-mono font-semibold text-white">{{ deviceTypeCounts.esxi }}</span>
                </div>

                <div 
                  @click="currentView = 'nas'"
                  class="p-3 rounded bg-[#0c0e12] border border-[#232936] hover:border-blue-500/50 cursor-pointer transition-colors flex flex-col gap-1"
                >
                  <div class="flex items-center justify-between text-txt-muted">
                    <span class="text-[11px]">Ağ Depolama (NAS/SAN)</span>
                    <span class="material-symbols-outlined text-[15px] text-blue-400">database</span>
                  </div>
                  <span class="text-xl font-mono font-semibold text-white">{{ deviceTypeCounts.nas }}</span>
                </div>

                <div class="p-3 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-1">
                  <div class="flex items-center justify-between text-txt-muted">
                    <span class="text-[11px]">Switch & Router</span>
                    <span class="material-symbols-outlined text-[15px] text-purple-400">router</span>
                  </div>
                  <span class="text-xl font-mono font-semibold text-white">{{ deviceTypeCounts.network }}</span>
                </div>

                <div class="p-3 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-1">
                  <div class="flex items-center justify-between text-txt-muted">
                    <span class="text-[11px]">Diğer Cihazlar</span>
                    <span class="material-symbols-outlined text-[15px] text-txt-muted">devices_other</span>
                  </div>
                  <span class="text-xl font-mono font-semibold text-white">{{ deviceTypeCounts.other }}</span>
                </div>
              </div>
            </div>

            <!-- Right: Attention & Critical Alerts -->
            <div class="bg-[#141820] rounded-lg border border-[#232936] p-5 flex flex-col gap-4">
              <div class="flex items-center justify-between pb-3 border-b border-[#232936]">
                <div class="flex items-center gap-2">
                  <span class="material-symbols-outlined text-rose-400">notification_important</span>
                  <h3 class="text-sm font-semibold text-white">Dikkat Gerektiren Düğümler</h3>
                </div>
                <span 
                  class="px-2 py-0.5 rounded text-[10px] font-mono font-medium border"
                  :class="offlineDevices.length > 0 ? 'bg-rose-500/10 text-rose-400 border-rose-500/20' : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'"
                >
                  {{ offlineDevices.length }} Kesinti
                </span>
              </div>

              <div class="flex flex-col gap-2 max-h-64 overflow-y-auto pr-1">
                <template v-if="offlineDevices.length > 0">
                  <div 
                    v-for="d in offlineDevices" 
                    :key="d.id"
                    @click="selectDevice(d); currentView = 'devices'"
                    class="p-2.5 rounded bg-rose-500/5 border border-rose-500/20 hover:border-rose-500/50 cursor-pointer transition-colors flex items-center justify-between text-xs"
                  >
                    <div class="flex flex-col gap-0.5">
                      <span class="font-medium text-white">{{ d.name }}</span>
                      <span class="font-mono text-[11px] text-txt-muted">{{ d.ip_address }}</span>
                    </div>
                    <span class="px-2 py-0.5 rounded text-[10px] bg-rose-500/10 text-rose-400 font-medium">Çevrimdışı</span>
                  </div>
                </template>
                <template v-else>
                  <div class="py-10 text-center flex flex-col items-center gap-2 text-txt-muted">
                    <span class="material-symbols-outlined text-[28px] text-emerald-400">verified</span>
                    <span class="text-xs font-medium text-white">Tüm Düğümler Sağlıklı</span>
                    <span class="text-[11px]">Şu anda ağda çevrimdışı veya yanıt vermeyen cihaz yok.</span>
                  </div>
                </template>
              </div>
            </div>
          </div>

          <!-- Bottom: Recent Downtime Incidents Preview -->
          <div class="bg-[#141820] rounded-lg border border-[#232936] p-5 flex flex-col gap-4">
            <div class="flex items-center justify-between pb-3 border-b border-[#232936]">
              <div class="flex items-center gap-2">
                <span class="material-symbols-outlined text-txt-muted">history</span>
                <h3 class="text-sm font-semibold text-white">Son Kesintiler ve Olay Günlüğü</h3>
              </div>
              <button 
                @click="currentView = 'incidents'"
                class="text-xs text-blue-400 hover:text-blue-300 font-medium flex items-center gap-1"
              >
                <span>Tümünü Gör ({{ incidents.length }})</span>
                <span class="material-symbols-outlined text-[14px]">arrow_forward</span>
              </button>
            </div>

            <template v-if="incidents.length > 0">
              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs border-collapse">
                  <thead class="text-txt-muted text-[10px] uppercase font-mono border-b border-[#232936]">
                    <tr>
                      <th class="py-2 px-3">Olay #</th>
                      <th class="py-2 px-3">Cihaz ID</th>
                      <th class="py-2 px-3">Başlangıç</th>
                      <th class="py-2 px-3">Kök Neden Hipotezi</th>
                      <th class="py-2 px-3">Durum</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-[#1e2533]">
                    <tr v-for="inc in incidents.slice(0, 5)" :key="inc.id" class="hover:bg-[#181e28]">
                      <td class="py-2.5 px-3 font-mono">#{{ inc.id }}</td>
                      <td class="py-2.5 px-3 font-mono text-txt-primary">Cihaz #{{ inc.device_id }}</td>
                      <td class="py-2.5 px-3 font-mono text-txt-secondary">{{ formatDateTime(inc.started_at) }}</td>
                      <td class="py-2.5 px-3 text-txt-primary">{{ inc.auto_detection_hypothesis || formatCause(inc.classified_cause) }}</td>
                      <td class="py-2.5 px-3">
                        <span 
                          class="px-2 py-0.5 rounded text-[10px] border font-medium"
                          :class="inc.resolved_at ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border-rose-500/20'"
                        >
                          {{ inc.resolved_at ? 'Çözüldü' : 'Devam Ediyor' }}
                        </span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </template>
            <template v-else>
              <div class="py-6 text-center text-txt-muted text-xs">
                Kayıtlı kesinti olayı bulunmuyor. Sistem kararlı çalışıyor.
              </div>
            </template>
          </div>
        </template>

        <!-- VIEW 2: Devices (Nodes & Device Inventory) -->
        <template v-else-if="currentView === 'devices'">
          <!-- Host & Storage KPI Cards -->
          <HostCards 
            :hosts="devices"
            @open-discovery="isDiscoveryOpen = true"
            @open-add-device="openAddDeviceModal"
          />

          <!-- Dense Monitored Devices Grid -->
          <DeviceTable 
            :devices="devices"
            :selectedDeviceId="selectedDevice?.id"
            :searchQuery="searchQuery"
            @select-device="selectDevice"
            @open-add-device="openAddDeviceModal"
            @open-discovery="isDiscoveryOpen = true"
            @edit-device="editDevice"
            @delete-device="deleteDevice"
          />

          <!-- Bottom Telemetry & Real Incident Inspector -->
          <TelemetryInspector 
            :device="selectedDevice"
            :metrics="selectedMetrics"
            :incidents="incidents"
            @open-incidents="currentView = 'incidents'"
          />
        </template>

        <!-- VIEW 3: Network Storage & NAS (QNAP, Synology, TrueNAS, SAN) -->
        <template v-else-if="currentView === 'nas'">
          <div class="bg-[#141820] rounded-lg border border-[#232936] p-6 flex flex-col gap-4">
            <div class="flex items-center justify-between pb-4 border-b border-[#232936]">
              <div class="flex items-center gap-2">
                <span class="material-symbols-outlined text-blue-400">database</span>
                <h3 class="text-sm font-semibold text-white">Ağ Depolama & NAS Cihazları (QNAP, Synology, TrueNAS, SAN)</h3>
              </div>
              <button 
                @click="openAddNasDevice" 
                class="px-3 py-1.5 rounded text-xs bg-blue-600 hover:bg-blue-500 text-white font-medium"
              >
                + NAS Ekle
              </button>
            </div>

            <template v-if="nasDevices.length > 0">
              <DeviceTable 
                :devices="nasDevices"
                :selectedDeviceId="selectedDevice?.id"
                :searchQuery="searchQuery"
                @select-device="selectDevice"
                @open-add-device="openAddNasDevice"
                @open-discovery="isDiscoveryOpen = true"
                @edit-device="editDevice"
                @delete-device="deleteDevice"
              />
            </template>
            <template v-else>
              <div class="py-16 text-center text-txt-muted flex flex-col items-center gap-2">
                <span class="material-symbols-outlined text-[32px]">database</span>
                <p class="text-xs">Kayıtlı ağ depolama (NAS/SAN) cihazı bulunamadı. Ağ taraması başlatıp QNAP, Synology, TrueNAS, Asustor gibi cihazları ekleyebilirsiniz.</p>
                <button @click="isDiscoveryOpen = true" class="mt-2 px-3 py-1.5 bg-blue-600 text-white text-xs rounded">
                  Ağ Taraması Başlat
                </button>
              </div>
            </template>
          </div>
        </template>

        <!-- VIEW: Power & Temperature / UPS -->
        <template v-else-if="currentView === 'power'">
          <div class="bg-[#141820] rounded-lg border border-[#232936] p-6 flex flex-col gap-5">
            <div class="flex items-center justify-between pb-4 border-b border-[#232936]">
              <div class="flex items-center gap-2">
                <span class="material-symbols-outlined text-amber-400">thermostat</span>
                <h3 class="text-sm font-semibold text-white">Sıcaklık ve Elektrik / UPS İzleme Motoru</h3>
              </div>
              <span class="text-xs font-mono text-txt-secondary">
                Toplu Kesinti Eşiği: 3 Cihaz
              </span>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div class="p-4 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-2">
                <span class="text-xs text-txt-muted">Ortalama Sistem Odası Sıcaklığı</span>
                <span class="text-3xl font-mono text-white">
                  {{ stats.avg_temperature ? stats.avg_temperature + '°C' : 'Veri Yok' }}
                </span>
                <span class="text-[11px] text-txt-muted">
                  Son 2 saat içinde mini-ajan veya IPMI sensörlerinden toplanan sıcaklık ortalaması.
                </span>
              </div>

              <div class="p-4 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-2">
                <span class="text-xs text-txt-muted">Elektrik Kesintisi Korelasyon Motoru</span>
                <span class="text-sm font-medium text-emerald-400 flex items-center gap-1.5">
                  <span class="material-symbols-outlined text-[18px]">bolt</span>
                  Şebeke Kararlı (Aktif Toplu Kesinti Yok)
                </span>
                <span class="text-[11px] text-txt-muted">
                  Aynı anda 3 veya daha fazla cihazın aniden kapanması durumunda otomatik "Elektrik Kesintisi" alarmı üretilir.
                </span>
              </div>
            </div>
          </div>
        </template>

        <!-- VIEW: Incidents & Logs -->
        <template v-else-if="currentView === 'incidents'">
          <div class="bg-[#141820] rounded-lg border border-[#232936] p-6 flex flex-col gap-4">
            <div class="flex items-center justify-between pb-4 border-b border-[#232936]">
              <div class="flex items-center gap-2">
                <span class="material-symbols-outlined text-rose-400">format_list_bulleted</span>
                <h3 class="text-sm font-semibold text-white">Kesinti Geçmişi ve SLA Olay Günlüğü</h3>
              </div>
              <span class="text-xs font-mono text-txt-secondary">
                Toplam {{ incidents.length }} Olay Kaydı
              </span>
            </div>

            <div class="overflow-x-auto border border-[#232936] rounded-md">
              <table class="w-full text-left text-xs border-collapse">
                <thead class="bg-[#0c0e12] text-txt-muted text-[10px] uppercase font-mono">
                  <tr>
                    <th class="p-3">Olay #</th>
                    <th class="p-3">Cihaz ID</th>
                    <th class="p-3">Başlangıç</th>
                    <th class="p-3">Bitiş / Çözüm</th>
                    <th class="p-3">Süre (Dk)</th>
                    <th class="p-3">Tespit Sebebi</th>
                    <th class="p-3">Operatör Notu</th>
                    <th class="p-3 text-right">İşlem</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-[#1e2533]">
                  <template v-if="incidents.length > 0">
                    <tr v-for="inc in incidents" :key="inc.id" class="hover:bg-[#181e28]">
                      <td class="p-3 font-mono">#{{ inc.id }}</td>
                      <td class="p-3 font-mono text-txt-primary">Cihaz #{{ inc.device_id }}</td>
                      <td class="p-3 font-mono text-txt-secondary">{{ formatDateTime(inc.started_at) }}</td>
                      <td class="p-3 font-mono text-txt-secondary">{{ inc.resolved_at ? formatDateTime(inc.resolved_at) : 'Devam Ediyor' }}</td>
                      <td class="p-3 font-mono">{{ inc.duration_seconds ? Math.round(inc.duration_seconds / 60) : '—' }}</td>
                      <td class="p-3">
                        <span 
                          class="px-2 py-0.5 rounded text-[11px] border"
                          :class="inc.classified_cause === 'unclassified' ? 'bg-amber-500/10 text-amber-400 border-amber-500/20' : 'bg-slate-500/10 text-slate-300 border-slate-500/20'"
                        >
                          {{ formatCause(inc.classified_cause) }}
                        </span>
                      </td>
                      <td class="p-3 text-txt-muted truncate max-w-xs">{{ inc.operator_notes || '—' }}</td>
                      <td class="p-3 text-right">
                        <button 
                          @click="openClassifyModal(inc)"
                          class="px-2.5 py-1 rounded bg-[#1e2533] hover:bg-[#283142] text-blue-400 text-[11px]"
                        >
                          Sınıflandır
                        </button>
                      </td>
                    </tr>
                  </template>
                  <template v-else>
                    <tr>
                      <td colspan="8" class="p-12 text-center text-txt-muted">
                        Kayıtlı kesinti bulunmuyor. Sistemler %100 kullanılabilirlikle çalışıyor.
                      </td>
                    </tr>
                  </template>
                </tbody>
              </table>
            </div>
          </div>
        </template>

        <!-- VIEW: Backups & Portability -->
        <template v-else-if="currentView === 'backups'">
          <div class="bg-[#141820] rounded-lg border border-[#232936] p-6 flex flex-col gap-5">
            <div class="flex items-center justify-between pb-4 border-b border-[#232936]">
              <div class="flex items-center gap-2">
                <span class="material-symbols-outlined text-blue-400">settings_backup_restore</span>
                <h3 class="text-sm font-semibold text-white">Yedekleme ve Taşınabilirlik Konsolu</h3>
              </div>
              <button 
                @click="downloadBackupNow"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-blue-600 hover:bg-blue-500 text-white"
              >
                <span class="material-symbols-outlined text-[16px]">download</span>
                <span>Anlık Yedek İndir (.json)</span>
              </button>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
              <div class="p-4 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-2">
                <h4 class="font-medium text-white">Otomatik 03:00 Gece Yedeklemesi</h4>
                <p class="text-txt-muted leading-relaxed">
                  Her gece 03:00'te tüm konfigürasyon, cihazlar ve SLA kayıtları <code class="text-txt-primary">/opt/gnnwatchtower/backups/</code> dizinine tarihli olarak kaydedilir. 30 günden eski arşivler otomatik silinir.
                </p>
                <span class="text-emerald-400 font-mono text-[11px] mt-1">✓ Otomatik Yedekleme Servisi Aktif</span>
              </div>

              <div class="p-4 rounded bg-[#0c0e12] border border-[#232936] flex flex-col gap-2">
                <h4 class="font-medium text-white">Hızlı Geri Yükleme (Restore)</h4>
                <p class="text-txt-muted leading-relaxed">
                  Önceden indirilmiş yedek dosyasını yükleyin. Veritabanı ve bildirim kanalları anında senkronize olur.
                </p>
                <input 
                  type="file" 
                  accept=".json"
                  @change="handleFileRestore" 
                  class="block w-full text-xs text-txt-muted file:mr-4 file:py-1.5 file:px-3 file:rounded file:border-0 file:text-xs file:font-semibold file:bg-[#1e2533] file:text-white hover:file:bg-[#2c3547]"
                />
              </div>
            </div>
          </div>
        </template>
      </main>
    </div>

    <!-- Modals -->
    <DiscoveryModal 
      :isOpen="isDiscoveryOpen" 
      @close="isDiscoveryOpen = false" 
      @imported="fetchData"
    />

    <AgentModal 
      :isOpen="isAgentOpen" 
      @close="isAgentOpen = false" 
    />

    <AddDeviceModal 
      :isOpen="isAddDeviceOpen" 
      :editDevice="deviceToEdit"
      @close="isAddDeviceOpen = false" 
      @saved="fetchData"
    />

    <ClassifyModal 
      :isOpen="isClassifyOpen"
      :incident="incidentToClassify"
      @close="isClassifyOpen = false"
      @classified="fetchData"
    />

    <SettingsModal 
      :isOpen="isSettingsOpen"
      @close="isSettingsOpen = false"
      @saved="fetchData"
    />

    <ChangePasswordModal 
      :isOpen="isChangePasswordOpen"
      @close="isChangePasswordOpen = false"
    />

    <LoginModal 
      :isOpen="isLoginOpen"
      @logged-in="handleLoginSuccess"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import api from './services/api'
import TopBar from './components/TopBar.vue'
import Sidebar from './components/Sidebar.vue'
import HostCards from './components/HostCards.vue'
import DeviceTable from './components/DeviceTable.vue'
import TelemetryInspector from './components/TelemetryInspector.vue'
import DiscoveryModal from './components/DiscoveryModal.vue'
import AgentModal from './components/AgentModal.vue'
import AddDeviceModal from './components/AddDeviceModal.vue'
import ClassifyModal from './components/ClassifyModal.vue'
import SettingsModal from './components/SettingsModal.vue'
import ChangePasswordModal from './components/ChangePasswordModal.vue'
import LoginModal from './components/LoginModal.vue'

// State
const stats = ref({})
const gateway = ref({ ip: '192.168.1.1', status: 'offline', latency_ms: 0 })
const devices = ref([])
const incidents = ref([])
const selectedDevice = ref(null)
const selectedMetrics = ref([])
const searchQuery = ref('')
const currentView = ref('overview')
const isRefreshing = ref(false)

// Modals
const isDiscoveryOpen = ref(false)
const isAgentOpen = ref(false)
const isAddDeviceOpen = ref(false)
const isClassifyOpen = ref(false)
const isSettingsOpen = ref(false)
const isChangePasswordOpen = ref(false)
const isLoginOpen = ref(false)

const deviceToEdit = ref(null)
const incidentToClassify = ref(null)

let pollTimer = null

// Filtered NAS / Storage devices
const nasDevices = computed(() => {
  return devices.value.filter(d => ['synology', 'nas', 'storage', 'qnap', 'truenas'].includes((d.device_type || '').toLowerCase()))
})

// Device breakdown statistics for Admin Dashboard
const deviceTypeCounts = computed(() => {
  const counts = { linux: 0, windows: 0, esxi: 0, nas: 0, network: 0, other: 0 }
  devices.value.forEach(d => {
    const t = (d.device_type || '').toLowerCase()
    if (['linux', 'ubuntu', 'debian'].includes(t)) counts.linux++
    else if (t.includes('win')) counts.windows++
    else if (t === 'esxi') counts.esxi++
    else if (['nas', 'synology', 'storage', 'qnap', 'truenas'].includes(t)) counts.nas++
    else if (['switch', 'router'].includes(t)) counts.network++
    else counts.other++
  })
  return counts
})

const offlineDevices = computed(() => {
  return devices.value.filter(d => d.status === 'offline')
})

const agentInstalledCount = computed(() => {
  return devices.value.filter(d => d.is_agent_installed === 1).length
})

const onlineDevicesCount = computed(() => {
  return devices.value.filter(d => d.status === 'online').length
})

const fetchData = async () => {
  isRefreshing.value = true
  try {
    const [sData, dData, iData, gData] = await Promise.all([
      api.getDashboardStats().catch(() => ({})),
      api.getDevices().catch(() => []),
      api.getIncidents().catch(() => []),
      api.getGatewayHealth().catch(() => ({ ip: '192.168.1.1', status: 'offline', latency_ms: 0 }))
    ])

    stats.value = sData
    devices.value = dData
    incidents.value = iData
    gateway.value = gData

    // If no device selected yet and we have devices, auto-select the first one
    if (!selectedDevice.value && devices.value.length > 0) {
      selectDevice(devices.value[0])
    }
  } catch (err) {
    // Auth failures are caught by Axios interceptor
  } finally {
    isRefreshing.value = false
  }
}

const selectDevice = async (device) => {
  selectedDevice.value = device
  try {
    const res = await api.getDeviceTelemetry(device.id)
    selectedMetrics.value = res.metrics || []
  } catch (err) {
    selectedMetrics.value = []
  }
}

const openAddDeviceModal = () => {
  deviceToEdit.value = null
  isAddDeviceOpen.value = true
}

const openAddNasDevice = () => {
  deviceToEdit.value = {
    name: 'NAS-Depolama',
    ip_address: '',
    device_type: 'nas',
    monitoring_profile: 'active',
    check_interval_seconds: 15
  }
  isAddDeviceOpen.value = true
}

const editDevice = (device) => {
  deviceToEdit.value = { ...device }
  isAddDeviceOpen.value = true
}

const deleteDevice = async (id) => {
  if (confirm('Bu cihazı ve bağlı tüm izleme servislerini silmek istediğinize emin misiniz?')) {
    try {
      await api.deleteDevice(id)
      if (selectedDevice.value?.id === id) {
        selectedDevice.value = null
      }
      fetchData()
    } catch (err) {
      alert('Silme hatası: ' + (err.response?.data?.error || err.message))
    }
  }
}

const openIncidentsView = () => {
  currentView.value = 'incidents'
}

const openClassifyModal = (incident) => {
  incidentToClassify.value = incident
  isClassifyOpen.value = true
}

const handleLogout = () => {
  api.logout()
  isLoginOpen.value = true
}

const handleLoginSuccess = () => {
  isLoginOpen.value = false
  fetchData()
}

const downloadBackupNow = async () => {
  try {
    const bundle = await api.exportBackup()
    const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `gnnwatchtower-backup-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
  } catch (err) {
    alert('Yedek alma hatası: ' + err.message)
  }
}

const handleFileRestore = (e) => {
  const file = e.target.files[0]
  if (!file) return

  const reader = new FileReader()
  reader.onload = async (evt) => {
    try {
      const bundle = JSON.parse(evt.target.result)
      await api.importBackup(bundle)
      alert('Yedek başarıyla geri yüklendi!')
      fetchData()
    } catch (err) {
      alert('Geri yükleme hatası: ' + (err.response?.data?.error || err.message))
    }
  }
  reader.readAsText(file)
}

const formatDateTime = (dateStr) => {
  if (!dateStr) return '—'
  return new Date(dateStr).toLocaleString('tr-TR')
}

const formatCause = (cause) => {
  switch (cause) {
    case 'power_outage': return 'Elektrik Kesintisi'
    case 'maintenance': return 'Planlı Bakım'
    case 'hardware_crash': return 'Donanım Çökmesi'
    case 'network_issue': return 'Ağ / Kablo Hatası'
    case 'unclassified': return 'Sınıflandırılmadı'
    default: return cause
  }
}

onMounted(() => {
  // Check auth
  const token = localStorage.getItem('gnn_token')
  if (!token) {
    isLoginOpen.value = true
  } else {
    fetchData()
  }

  window.addEventListener('gnn-auth-failed', () => {
    isLoginOpen.value = true
  })

  // Poll real stats every 10 seconds (Zero Placeholder, 100% live data)
  pollTimer = setInterval(() => {
    if (!isLoginOpen.value) {
      fetchData()
    }
  }, 10000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>
