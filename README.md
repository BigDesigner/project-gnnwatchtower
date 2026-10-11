# GNNwatchtower

> **Modern, Minimalist & Autonomous Infrastructure Monitoring Appliance for Managed Service Providers (MSPs)**  
> *Single Binary • Pure Go SQLite WAL • Embedded Vue 3 SPA • Zero CGO Dependencies • AGPL-3.0 License*

---

## ⚡ Quick Start (Tek Satır Kurulum)

Ubuntu 22.04 / 24.04 LTS sunucunuzda tek bir komutla kurulumu tamamlayıp servisi anında ayağa kaldırabilirsiniz:

```bash
curl -sSL https://raw.githubusercontent.com/BigDesigner/project-gnnwatchtower/main/install.sh | sudo bash
```

Kurulum tamamlandığında izleme konsoluna tarayıcınızdan erişebilirsiniz:
- **URL**: `http://<SUNUCU_IP>:3800`
- **Varsayılan Kullanıcı**: `admin`
- **Varsayılan Parola**: `admin`

---

## 🏗️ Mimari ve Tasarım Prensipleri

GNNwatchtower, harici servis bağımlılıklarını (PostgreSQL havuzları, harici Docker gereksinimi vb.) ortadan kaldıran **cihaz (appliance)** modelinde tasarlanmıştır:

- **Tek Statik İkili Dosya (Single Binary)**: Go 1.22 `//go:embed` teknolojisi ile derlenmiş Vue 3 SPA frontend'i doğrudan ikili dosya içinde barındırılır.
- **Pure Go SQLite (WAL Modu)**: Modernc (`modernc.org/sqlite`) sürücüsü sayesinde CGO olmadan çalışır; kilitlenme önleyici WAL modu ve meşguliyet zaman aşımı ile yüksek eşzamanlılık sunar.
- **Altın Kural: Sıfır Yer Tutucu (Zero Placeholder Invariant)**:
  - Kod tabanında sahte istatistikler, simüle grafikler veya mock veriler kesinlikle bulunmaz.
  - Ekranda görünen her metrik, ping gecikmesi, donanım sıcaklığı ve olay kaydı doğrudan canlı soket ve SQLite telemetrisinden beslenir.
  - Veri bulunmadığında kullanıcıyı yönlendiren otantik boş durumlar (`—` veya `Cihaz Bulunamadı`) sunulur.
- **Obsidian Telemetry NOC UI**: Cisco Meraki Dark minimal NOC kokpit teması (`#0c0e12`), düşük göz yorgunluğu ve yüksek bilgi yoğunluğu.
- **Olay Korelasyon Motoru**: Ağ geçidi kesintilerini veya toplu cihaz düşüşlerini analiz ederek otomatik elektrik kesintisi (`power_outage`) hipotezi üretir ve çok kanallı bildirim gönderir (Telegram, Discord, Brevo e-posta).
- **Otomatik Gece Yedeklemesi**: Her gece saat 03:00'te sistem konfigürasyonu ve geçmiş verilerini JSON bundle olarak otomatik rotasyonla arşivler.

---

## 🚀 Dağıtım Seçenekleri

### 1. Yerel Repodan Kurulum (Ubuntu)
```bash
git clone https://github.com/BigDesigner/project-gnnwatchtower.git
cd project-gnnwatchtower
sudo bash install.sh
```

### 2. Docker Compose ile Dağıtım
```bash
git clone https://github.com/BigDesigner/project-gnnwatchtower.git
cd project-gnnwatchtower
docker compose up -d --build
```

---

## 🖥️ Uç Cihaz Ajanı (GNNwatchtower Agent)

Uzak sunuculardan (Windows ve Linux) CPU, RAM, Disk, Isı ve pre-shutdown (kapanma öncesi) sinyallerini toplamak için hafif ajan mevcuttur.

Panonuzdaki **Ajan Kurulum** butonundan cihazınıza özel tek seferlik doğrulama belirteci (OTP) ile tek satırda kurulum sağlayabilirsiniz:

**Linux:**
```bash
curl -sSL "http://<SUNUCU_IP>:3800/api/agent/install/linux?token=<OTP_TOKEN>" | sudo bash
```

**Windows (PowerShell):**
```powershell
iwr -useb "http://<SUNUCU_IP>:3800/api/agent/install/windows?token=<OTP_TOKEN>" | iex
```

---

## 📜 Lisans

Bu proje **GNU Affero General Public License v3 (AGPL-3.0)** altında lisanslanmıştır. Detaylar için [LICENSE](LICENSE) dosyasına bakınız.
