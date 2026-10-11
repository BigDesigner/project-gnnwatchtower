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

### 1. Sıfır Yapılandırmalı Otomatik Kayıt (Token Gerekmez)
Herhangi bir parametre vermeden doğrudan çalıştırıldığında sistem otomatik bir kayıt anahtarı atar. Ajan ilk canlılık sinyalini (`heartbeat`) gönderdiğinde sunucu panele **otomatik olarak yeni cihaz olarak eklenir**:

* **Linux (Ubuntu / Debian):**
  ```bash
  curl -sSL http://<SUNUCU_IP>:3800/api/agent/install/linux | sudo bash
  ```
* **Windows Server (PowerShell Yönetici):**
  ```powershell
  powershell -ExecutionPolicy Bypass -Command "irm http://<SUNUCU_IP>:3800/api/agent/install/windows | iex"
  ```

### 2. Belirli Bir Cihazla Eşleştirme (Opsiyonel Token)
Eğer cihazı panele önceden manuel eklediyseniz veya mevcut bir envanter kaydıyla eşleştirmek istiyorsanız, komutun sonuna ilgili cihazın ajan anahtarını ekleyebilirsiniz:
* `http://<SUNUCU_IP>:3800/api/agent/install/linux?token=<CIHAZ_AJAN_TOKENI>`
* `http://<SUNUCU_IP>:3800/api/agent/install/windows?token=<CIHAZ_AJAN_TOKENI>`

---

## 📜 Lisans

Bu proje **GNU Affero General Public License v3 (AGPL-3.0)** altında lisanslanmıştır. Detaylar için [LICENSE](LICENSE) dosyasına bakınız.
