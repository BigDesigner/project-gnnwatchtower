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

## 🔄 Güncelleme (Tek Satır Update)

Mevcut veritabanınızı (`data/gnnwatchtower.db`) ve ortam ayarlarınızı (`.env`) koruyarak, otomatik yedekleme eşliğinde sistemi en son sürüme güncellemek için:

```bash
curl -sSL https://raw.githubusercontent.com/BigDesigner/project-gnnwatchtower/main/update.sh | sudo bash
```

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

## 🛠️ Servis Yönetimi (Systemd Komutları)

GNNwatchtower arka plan servisini (`gnnwatchtower.service`) yönetmek için aşağıdaki komutları kullanabilirsiniz:

```bash
# Servis durumunu kontrol et
sudo systemctl status gnnwatchtower

# Servisi başlat
sudo systemctl start gnnwatchtower

# Servisi durdur
sudo systemctl stop gnnwatchtower

# Servisi yeniden başlat
sudo systemctl restart gnnwatchtower

# Canlı günlük kayıtlarını (logları) takip et
sudo journalctl -u gnnwatchtower -f

# Son 50 log kaydını görüntüle
sudo journalctl -u gnnwatchtower -n 50 --no-pager
```

---

## 🖥️ Uç Cihaz Ajanı (GNNwatchtower Agent)

Uzak sunuculardan (Windows ve Linux) CPU, RAM, Disk, Isı ve pre-shutdown (kapanma öncesi) sinyallerini toplamak için hafif ve sıfır bağımlılıklı ajan mevcuttur.

### 1. Wazuh Tarzı Sıfır-Dokunuş Otomatik Kayıt (Zero-Touch Auto-Enrollment)
Ajan, uç noktalara kurulduğunda donanım ve işletim sistemi parmak izi ile merkeze otomatik tescil edilir (`POST /api/agent/enroll`). Komut satırında veya betiklerde gizli token bulunmaz; EDR/XDR sistemleriyle tam uyumlu yerel işletim sistemi servisi (Windows SCM / Linux Systemd) olarak çalışır:

* **Linux (Ubuntu / Debian):**
  ```bash
  curl -sSL http://<SUNUCU_IP>:3800/api/agent/install/linux | sudo bash
  ```
* **Windows Server (PowerShell Yönetici):**
  ```powershell
  Invoke-WebRequest -Uri "http://<SUNUCU_IP>:3800/api/agent/install/windows" -OutFile "$env:TEMP\install-gnnagent.ps1"; powershell.exe -ExecutionPolicy Bypass -File "$env:TEMP\install-gnnagent.ps1"; Remove-Item "$env:TEMP\install-gnnagent.ps1"
  ```
* **GPO / SCCM / Intune / Komut Satırı Dağıtımı:**
  ```cmd
  gnnwatchtower-agent.exe -service install -server http://<SUNUCU_IP>:3800
  gnnwatchtower-agent.exe -service start
  ```

### 2. Merkezi Web Panelinden Uzaktan Ajan Güncelleme (Remote Agent Upgrade)
Ajanlar, firewall arkasındaki uç noktalara doğrudan ters port açmadan **Heartbeat-Driven Pull** modeli ile çalışır. Web panelinden tek tıkla güncelleme emri verildiğinde, ajan yeni ikiliyi indirir, SHA-256 bütünlüğünü doğrular ve kesintisiz *Atomic Swap* ile kendini günceller.

---

## 📜 Lisans

Bu proje **GNU Affero General Public License v3 (AGPL-3.0)** altında lisanslanmıştır. Detaylar için [LICENSE](LICENSE) dosyasına bakınız.
