# GNNwatchtower - Uygulama ve Mimari Planı (IMPLEMENTATION_PLAN.md)

Bu belge, **GNNwatchtower** altyapı, ağ ve sunucu izleme sisteminin tüm teknik mimarisini, veri modelini, kullanıcı arayüzü tasarımını, EDR/XDR uyumlu kurumsal ajan yapısını, otomatik kayıt (auto-enrollment) ve uzaktan ajan güncelleme (remote upgrade) sistemlerini içeren ana planlama dokümanıdır.

---

## 1. Proje Özeti ve Dağıtım Hedefi

* **Hedef Sistem**: 20 GB Disk ve 4 GB RAM içeren bağımsız Ubuntu Sanal Makinesi (VM).
* **Dağıtım Kapsamı**: MSP (Managed Service Provider) ve Enterprise On-Premise modeli. Her müşterinin kendi yerel ağında bağımsız bir sanal cihaz (appliance) olarak çalışacak.
* **Servis Portu**: `3800` (Yerel ağdaki hiçbir standart servis veya web sunucusuyla çakışmayan izole port).
* **Temel Mimari İlkesi**: Tek bir ikili dosya (Single Binary), harici Nginx veya Docker zorunluluğu olmayan yerel Systemd servisi, Pure Go SQLite (WAL modu) ve sıfır harici C bağımlılığı (`modernc.org/sqlite`).
* **Kurumsal Uyumluluk (Enterprise Invariant)**: Ajanlar müşteri ortamlarında aktif çalışan kurumsal EDR/XDR çözümleri (Microsoft Defender for Endpoint, CrowdStrike Falcon, SentinelOne, Cortex XDR) tarafından hacktool/dropper olarak algılanmayacak şekilde yerel OS standartlarına (Windows SCM Servisi, PE Resource Metadata, Event Log) tam uyumlu geliştirilir.

---

## 2. Sistem Mimarisi ve Teknoloji Yığını

```mermaid
graph TD
    subgraph Host["Müşteri Sanal Cihazı (Port 3800 Appliance)"]
        subgraph Binary["GNNwatchtower Tek Binary (/opt/gnnwatchtower/gnnwatchtower)"]
            UI["Gömülü Web UI (Vue 3 + Tailwind CSS)"]
            API["REST API & WebSocket Motoru (Go/Gin)"]
            EnrollmentEngine["Ajan Otomatik Kayıt Motoru (Auto-Enrollment)"]
            UpgradeOrchestrator["Ajan Uzaktan Güncelleme Yöneticisi (Remote Upgrade)"]
            Scanner["Yerel Ağ Keşif Motoru (ARP / Ping / TCP)"]
            Poller["Kademeli İzleme Motoru (Aktif / Uyku / Bakım)"]
            Correlator["Kesinti Korelasyon Motoru (Elektrik vs Bakım)"]
            Notifier["Alarm Dağıtıcı (Brevo API / Discord / Telegram)"]
            BackupEngine["Otomatik 03:00 Yedekleme Motoru"]
        end
        DB[("SQLite WAL Modu Veritabanı (/opt/gnnwatchtower/data/gnnwatchtower.db)")]
        Backups[("Yedekler Dizini (/opt/gnnwatchtower/backups/)")]
        Binaries[("Ajan Dağıtım İkilileri (/opt/gnnwatchtower/bin/)")]
        
        API --> DB
        EnrollmentEngine --> DB
        UpgradeOrchestrator --> DB
        UpgradeOrchestrator --> Binaries
        Poller --> DB
        BackupEngine --> Backups
    end

    subgraph Network["Kurumsal Yerel Ağ (LAN / DMZ)"]
        WinSrv["Windows Sunucular (Yerel Windows SCM Servisi)"]
        UbuSrv["Ubuntu/Linux Sunucular (Systemd Servisi)"]
        NAS["Synology NAS (SNMP / DSM)"]
        ESXi["VMware ESXi (vSphere API / HTTPS)"]
        Switches["Omurga Switch & Router (Ping / SNMP)"]
    end

    Scanner -.-> Network
    Poller -.-> NAS
    Poller -.-> ESXi
    Poller -.-> Switches

    WinSrv -->|1. İlk Başlatma: Auto-Enrollment (Kayıt & Token Talebi)| EnrollmentEngine
    WinSrv -->|2. Periyodik Heartbeat & Pre-Shutdown| API
    API -.->|3. Heartbeat Yanıtı: Güncelleme Talimatı + SHA256| WinSrv
    WinSrv -->|4. Güvenli İkili İndirme & Self-Update| Binaries

    UbuSrv -->|1. İlk Başlatma: Auto-Enrollment| EnrollmentEngine
    UbuSrv -->|2. Periyodik Heartbeat & Pre-Shutdown| API
    API -.->|3. Heartbeat Yanıtı: Güncelleme Talimatı + SHA256| UbuSrv
    UbuSrv -->|4. Güvenli İkili İndirme & Self-Update| Binaries
```

### Temel Bileşenler
1. **Backend & Core (Go 1.22)**:
   - Bellek tüketimi: ~25-35 MB RAM.
   - Go `embed` paketi ile Vue 3 derlenmiş statik dosyaları tek bir ikili dosya içinde sunulur.
   - Harici web sunucusu (Nginx/Apache) veya çalışma ortamı (Node.js/Python) gerekmez.
2. **Veritabanı (Pure Go SQLite - WAL Modu)**:
   - CGO gerektirmeyen saf Go sürücüsü (`modernc.org/sqlite`).
   - Write-Ahead Logging (WAL) modu: Yüksek eşzamanlılık, kilitlenmesiz okuma/yazma.
   - Veritabanı tek bir dosyadır (`gnnwatchtower.db`).
3. **Frontend (Vue 3 + Tailwind CSS + Google Fonts + Material Symbols)**:
   - Minimalist Cisco Meraki / Enterprise konsol stili.
   - Tablo sayılarında ve IP adreslerinde sabit genişlikli (tabular monospace) Google **Inter** fontu.
   - İkonografi: Google **Material Symbols (Outlined)**. Sistem emojileri kesinlikle kullanılmaz.

---

## 3. Cisco Tarzı Minimalist Tasarım Dili (UI/UX)

* **Renk Düzeni**:
  - Arka Plan: Koyu Lacivert ve Arduvaz tonları (`#0b132b`, `#1c2541`, `#0f172a`).
  - Vurgu Rengi: Cisco Mavisi (`#00bceb` / `#0284c7`).
  - Kart ve Tablo Çerçeveleri: İnce yüksek kontrastlı bordürler (`border-slate-800`).
* **Durum LED Göstergeleri**:
  - Çevrimiçi (Online): Zümrüt Yeşili LED (`#10b981`) + hafif parlama efekti.
  - Çevrimdışı (Offline): Gül Kırmızısı LED (`#f43f5e`).
  - Uyarı / Bakım (Warning/Maintenance): Kehribar Sarısı LED (`#f59e0b`).
  - Uyku / Pasif (Dormant): Çelik Grisi (`#64748b`).
* **Ajan ve Sürüm Rozetleri (Enterprise Badges)**:
  - Güncel: Yeşil `v1.1.0` rozeti.
  - Güncelleme Bekliyor: Kehribar Sarısı `v1.0.0 (Güncelleme Var)` rozeti + tek tıkla güncelleme butonu.
  - Güncelleniyor: Mavi nabız atan `Güncelleniyor (%50)...` rozeti.
  - Hata: Kırmızı `Hata (Hash Geçersiz)` rozeti + hata detay modalı.

---

## 4. Temel Fonksiyonel Modüller

### 4.1. Otomatik Ağ Keşfi (Auto-Discovery Engine)
* **Taramalar**: Operatörün belirlediği CIDR bloğunu (örn. `192.168.1.0/24`) 64 eşzamanlı goroutine ile 5 saniyede tarar.
* **SSRF Koruması**: CIDR girişi `net.ParseCIDR` ile doğrulanır; varsayılan olarak yalnızca RFC 1918 özel IP bloklarına (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) izin verilir.
* **Cihaz Tanıma Kuralları**:
  - Port `5000` / `5001` açık -> Synology NAS
  - Port `902` veya `443` (vSphere) açık -> VMware ESXi
  - Port `3389` açık -> Windows Server
  - Port `22` açık -> Ubuntu / Linux Sunucu
  - Port `161` (SNMP) açık -> Switch / Router

### 4.2. Kademeli İzleme ve Uyku Profilleri (Smart Polling Profiles)
1. **Aktif Profil (Active)**: 15 saniyede bir ping ve servis port kontrolü.
2. **24 Saatlik Uyku (Dormant 24h)**: Günde bir kez kontrol edilir (Soğuk yedek sunucular).
3. **30 Günlük Uyku (Dormant 30d)**: Ayda bir kez kontrol edilir (Arşiv ve dönemsel sunucular).
4. **Bakım Modu (Maintenance)**: Süreli bakım modu; alarm üretilmez ve SLA oranından düşülmez.

### 4.3. Akıllı Kesinti Tespiti (Elektrik Kesintisi vs Bakım)
1. **Pre-Shutdown Sinyali (Temiz Kapanma)**:
   - Ajan işletim sistemi kapanma sinyalini (`SIGTERM`, Windows `SERVICE_CONTROL_SHUTDOWN` / `SERVICE_CONTROL_PRESHUTDOWN`) yakaladığında merkeze anlık sinyal iletir: `"reason": "user_restart" | "os_update"`. Sistem bunu anında **Planlı Bakım** olarak işaretler.
2. **Toplu Kesinti Korelasyonu**:
   - Aynı anda 3 veya daha fazla cihazın bağlantısı kesilirse, sistem bunu **Elektrik Kesintisi veya Ana Switch Arızası** olarak sınıflandırır.
3. **Operatör Doğrulama Formu (Post-Mortem Modal)**:
   - Cihaz açıldığında operatöre tek tıkla doğrulatır (Elektrik, Planlı Bakım, Donanım, Ağ).

### 4.4. Bildirim Motoru (Brevo, Discord, Telegram)
* **Brevo REST API**: HTTPS (Port 443) üzerinden gönderim; Port 25/587 engellerinden etkilenmez.
* **Discord & Telegram**: Anlık kanal bildirimleri.
* **Anti-Flapping & Cooldown**: Kesilip gelen cihazlar için 3 dakikalık alarm sönümleme ve HTTP 429 üstel geri çekilme (exponential backoff).

---

### 4.5. EDR/XDR Uyumlu Kurumsal Ajan Mimarisi (Enterprise-Grade Agent)

Ajanların kurumsal XDR/EDR (Defender for Endpoint, CrowdStrike, SentinelOne) sistemleri tarafından hacktool muamelesi görmemesi için zorunlu kılınan mimari standartlar:

1. **Bellek İçi Yürütme (In-Memory Stager) Yasağı**:
   - `powershell -ExecutionPolicy Bypass -Command "irm ... | iex"` deseni tamamen kaldırılmıştır.
   - Dağıtım dosya tabanlı yürütülür; indirme sonrası ikili doğrudan doğrulanır.
2. **Yerel Windows Servis Mimarisi (`golang.org/x/sys/windows/svc`)**:
   - `Register-ScheduledTask` ve `NT AUTHORITY\SYSTEM` zamanlanmış görev hacksilsilesi kaldırılmıştır.
   - Ajan doğrudan Windows Service Control Manager (SCM) API'sini implemente eder.
   - Komut satırı yönetim yetenekleri:
     - `gnnwatchtower-agent.exe -service install -server <URL>` (SCM'e `GNNwatchtowerAgent` servisi olarak tescil eder)
     - `gnnwatchtower-agent.exe -service uninstall`
     - `gnnwatchtower-agent.exe -service start` / `stop` / `status`
     - `gnnwatchtower-agent.exe -version`
   - Windows Hizmetleri konsolunda (`services.msc`) resmi ad, açıklama ve bağımlılıklarla listelenir.
3. **Windows PE Sürüm Bilgileri ve Manifest (.syso Entegrasyonu)**:
   - Ajan derlenirken PE Header içerisine kurumsal metadata gömülür:
     - `CompanyName`: `GNNwatchtower`
     - `ProductName`: `GNNwatchtower Telemetry Agent`
     - `FileDescription`: `Enterprise Network & Server Telemetry Service`
     - `FileVersion`: `1.0.0.0`
     - `LegalCopyright`: `Copyright (c) 2026 GNNwatchtower`
   - Stripped binary (`-s -w`) yerine kurumsal hata ayıklama ve sürüm sembollerini koruyan derleme profili uygulanır.
4. **Windows Event Log Entegrasyonu (`golang.org/x/sys/windows/svc/eventlog`)**:
   - Servis başlatma, durdurma, güncelleme ve kritik hata olayları Windows `Application` olay günlüğüne tescil edilir (`Source: GNNwatchtowerAgent`). SOC ekipleri için tam denetlenebilirlik sağlanır.

---

### 4.6. Sıfır-Dokunuş Otomatik Kayıt & Token Yönetimi (Zero-Touch Auto-Enrollment Engine)

Ajan kurulum komutuna statik token gömme zorunluluğu kaldırılmıştır.

1. **Kurulum**:
   - Ajan uç noktaya parametresiz veya yalnızca sunucu adresi verilerek kurulur:
     - Windows: `gnnwatchtower-agent.exe -service install -server http://SUNUCU_IP:3800`
     - Linux: `/opt/gnnwatchtower-agent/gnnwatchtower-agent -service install -server http://SUNUCU_IP:3800`
2. **İlk Başlatma ve Kayıt Talebi**:
   - Ajan çalışmaya başladığında yerel `agent.key` dosyasını arar.
   - Dosya yoksa **Auto-Enrollment** sürecini başlatır:
     - Endpoint donanım ve OS parmak izini toplar: `hostname`, `os_name`, `os_version`, `architecture`, `local_ip`, `mac_address`, `machine_uuid`.
     - `POST /api/agent/enroll` adresine kayıt isteği gönderir.
3. **Sunucu Tarafı Token Üretimi**:
   - Sunucu gelen cihazı IP/MAC/UUID üzerinden mevcut cihaz kaydıyla eşleştirir veya yeni cihaz oluşturur.
   - Cihaza özel 64 karakterlik kriptografik `agent_token` üretir.
   - SQLite `devices` tablosuna `agent_token`, `is_agent_installed = 1`, `agent_version` bilgilerini işler.
   - Ajana `{ "status": "enrolled", "agent_id": 42, "token": "agt_...", "check_interval": 15 }` yanıtını döner.
4. **Yerel Anahtar Saklama**:
   - Ajan bu token'ı yerel dizinde yalnızca `SYSTEM` / `root` erişimine açık `agent.key` dosyasına yazar (Windows ACL: Administrators & SYSTEM; Linux: `chmod 600`).
   - Sonraki tüm heartbeat ve telemetri paketleri bu token ile kimliklendirilir (`X-Agent-Token` başlığı).

---

### 4.7. Web Panelinden Uzaktan Ajan Güncelleme (Remote Agent Upgrade Engine)

Uç noktalarda firewall veya ters yönde port açma gerektirmeden, **Heartbeat-Driven Pull Modeli** ile merkezi güncelleme:

```
[Web Panel]                [Merkezi Sunucu (Appliance)]               [Uç Nokta Ajanı]
    │                                   │                                    │
    │ 1. "Ajanı Güncelle" tıklandı     │                                    │
    │ ────────────────────────────────> │                                    │
    │    (upgrade_status = 'pending')   │                                    │
    │                                   │ 2. Rutin Heartbeat (POST /report)  │
    │                                   │ <───────────────────────────────── │
    │                                   │ 3. Yanıt: Action = 'upgrade'       │
    │                                   │    Target Version + SHA256 Hash   │
    │                                   │ ─────────────────────────────────> │
    │                                   │                                    │
    │                                   │ 4. Durum: 'downloading'            │
    │                                   │ <───────────────────────────────── │
    │                                   │    (Ajan yeni ikiliyi indirir)     │
    │                                   │    (SHA256 doğrulaması yapar)      │
    │                                   │                                    │
    │                                   │ 5. Atomic File Swap & Restart      │
    │                                   │    (exe -> exe.old, new -> exe)    │
    │                                   │    (Servis yeniden başlar)         │
    │                                   │                                    │
    │                                   │ 6. Yeni Sürümle İlk Heartbeat      │
    │                                   │    (Version: 1.1.0, Status: 'ok')  │
    │                                   │ <───────────────────────────────── │
    │ 7. Canlı UI Sürüm Güncellendi     │                                    │
    │ <──────────────────────────────── │                                    │
```

1. **Merkezi Sürüm Yönetimi**:
   - Sunucuda `/opt/gnnwatchtower/bin/` altında derlenmiş en son `gnnwatchtower-agent` (Linux) ve `gnnwatchtower-agent.exe` (Windows) ikilileri bulunur.
   - Sunucu bu dosyaların SHA256 özetini ve sürüm bilgisini bellekte/veritabanında tutar.
2. **Kuyruk ve Talimat İletimi**:
   - Operatör panelden tekil veya toplu güncelleme emri verdiğinde veritabanında `agent_upgrade_status = 'pending'` işaretlenir.
   - Ajanın bir sonraki heartbeat'inde sunucu güncelleme direktifini iletir.
3. **Bütünlük Doğrulaması (SHA256 Checksum)**:
   - Ajan yeni ikiliyi `.update` uzantısıyla indirir.
   - İndirilen dosyanın SHA256 hash'ini hesaplar. Hash uyuşmazsa işlemi derhal sonlandırır, dosyayı siler ve sunucuya `upgrade_status = 'failed'` bildirir.
4. **Windows Dosya Kilidi Aşımı (Atomic Swap)**:
   - Çalışan Windows PE dosyaları işletim sistemi tarafından kilitlenir. Ajan şu adımları işletir:
     1. Mevcut `gnnwatchtower-agent.exe` -> `gnnwatchtower-agent.exe.old` olarak yeniden adlandırılır.
     2. `gnnwatchtower-agent.update` -> `gnnwatchtower-agent.exe` olarak taşınır.
     3. Windows SCM üzerinden servis restart tetiklenir.
     4. Yeni servis başarıyla ayağa kalktığında `.old` yedeğini temizler ve sunucuya başarı raporu gönderir.

---

### 4.8. Basit Yedekleme ve Taşıma (Backup & Portability)
1. **Manuel Web Yedekleme**: Yalnızca doğrulanmış admin oturumu ile paneldeki "Yedek Al" butonuna basıldığında ayarları ve cihazları içeren SHA-256 imzalı `gnnwatchtower-backup-TARIH.json` dosyası indirilir.
2. **Manuel Geri Yükleme**: Sıfır kurulan sisteme bu dosya yüklendiğinde şema ve imza kontrolünden geçerek 5 saniyede geri yüklenir.
3. **Otomatik Yerel Yedekleme**: Her gece 03:00'te `/opt/gnnwatchtower/backups/` altına tarihli yedek alınır; 30 günden eski dosyalar otomatik temizlenir.

---

## 5. Kurulum ve Derleme Süreçleri

### Merkezi Appliance Kurulumu (`install.sh`)
```bash
sudo bash install.sh
```
1. Çekirdek ikiliyi derler ve `/opt/gnnwatchtower/gnnwatchtower` dizinine yerleştirir.
2. Windows ve Linux ajan ikililerini derler, Windows PE Version Resource (.syso) bilgilerini ekler ve `/opt/gnnwatchtower/bin/` dizinine yerleştirir.
3. Systemd servisini ve UFW güvenlik duvarını yapılandırır.
4. İlk admin şifresi zorunlu değişim ekranını aktif eder.

### Uç Nokta Ajan Kurulumu (Enterprise Delivery)
* **Linux Sunucu**:
  ```bash
  curl -sSL http://SUNUCU_IP:3800/api/agent/install/linux | sudo bash
  ```
* **Windows Server (PowerShell Yönetici)**:
  ```powershell
  # Betiği geçici dizine indirip güvenli çalıştırır (iex/stager kullanılmaz)
  Invoke-WebRequest -Uri "http://SUNUCU_IP:3800/api/agent/install/windows" -OutFile "$env:TEMP\install-gnnagent.ps1"
  powershell.exe -ExecutionPolicy Bypass -File "$env:TEMP\install-gnnagent.ps1"
  Remove-Item "$env:TEMP\install-gnnagent.ps1"
  ```
* **GPO / SCCM / Intune Dağıtımı (MSI / EXE)**:
  `gnnwatchtower-agent.exe -service install -server http://SUNUCU_IP:3800`

---

## 6. Geliştirme Yol Haritası ve Fazlar

| Aşama | Başlık | Kapsam | Durum |
|---|---|---|---|
| **Faz 1** | Çekirdek Altyapı & Veritabanı | SQLite WAL, Auth, Settings, Modeller | Tamamlandı |
| **Faz 2** | Keşif & Alarm Motoru | CIDR Keşfi, Brevo, Discord, Telegram, Kesinti Korelasyonu | Tamamlandı |
| **Faz 3** | **Enterprise Ajan Mimarisi (SCM & EDR Uyum)** | `golang.org/x/sys/windows/svc` yerel servis, PE Resource (`.syso`), Windows Event Log entegrasyonu | Beklemede (Onay Sonrası) |
| **Faz 4** | **Sıfır-Dokunuş Auto-Enrollment & Token Yönetimi** | `POST /api/agent/enroll` API, güvenli `agent.key` ACL dosya yönetimi, dinamik token üretimi | Beklemede (Onay Sonrası) |
| **Faz 5** | **Merkezi Uzaktan Güncelleme (Remote Upgrade)** | Heartbeat-Driven Pull orkestrasyonu, SHA256 checksum doğrulama, atomic binary swap | Beklemede (Onay Sonrası) |
| **Faz 6** | **Web UI Revizyonu (Ajan Yönetim Konsolu)** | Ajan sürüm rozetleri, tek tıkla güncelleme butonu, temiz dağıtım modalı | Beklemede (Onay Sonrası) |
| **Faz 7** | Doğrulama & E2E Test | Windows Server ve Linux üzerinde servis döngüsü, EDR log analizi ve güncelleme testi | Beklemede (Onay Sonrası) |

---

## 7. Güvenlik, EDR/XDR ve Mimari Denetim Notları

1. **Public Repository Sıfır Sızıntı Sözleşmesi (ADR-0003)**:
   - Proje açık kaynaklı ve herkese açık GitHub deposundadır (`https://github.com/BigDesigner/project-gnnwatchtower.git`).
   - Müşteri IP blokları, cihaz MAC adresleri, keşif veritabanı (`data/gnnwatchtower.db`), gece yedekleri (`backups/`) ve `.env` dosyası Git'e asla eklenemez.
2. **EDR/XDR Hijack & Dropper Koruması**:
   - Bellek içi kod yürütme (`iex`), kontrolsüz zamanlanmış görev (`ScheduledTask`) ve imzasız/metadata'sız Go ikili dosyası dağıtımı kesinlikle yasaktır.
3. **Bütünlük Kontrolü (Tamper Protection)**:
   - Uzaktan güncellenen her ikili dosya için SHA256 zorunludur. Hash doğrulaması başarısız olan hiçbir dosya çalıştırılamaz veya servise aktarılamaz.
4. **Token Güvenliği ve Klon Dayanıklılığı**:
   - Token'lar betiklerin içine gömülemez. Auto-enrollment ile üretilen token'lar yerel dosya sisteminde yalnızca `SYSTEM` / `root` yetkisinde (`600` / Windows ACL) saklanır.
5. **SSRF ve Yerel Ağ Sınır Kontrolü (.specs/boundary-conditions.md)**:
   - CIDR tarayıcısı yalnızca RFC 1918 özel ağlarını (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) tarayacak şekilde sınırlandırılmıştır.
6. **Zero Placeholder Invariant (Altın Kural)**:
   - UI ve backend'de hiçbir mock veri, sahte telemetri veya simülasyon bulunamaz; tüm sürüm numaraları, hash değerleri ve güncelleme ilerlemeleri veritabanı ve gerçek OS süreçlerinden beslenir.
