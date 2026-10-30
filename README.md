# GNNwatchtower

[![Sürüm](https://img.shields.io/badge/version-0.4.12-blue)](VERSION)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8)](backend/go.mod)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D)](frontend/package.json)
[![SQLite](https://img.shields.io/badge/SQLite-WAL-003B57)](backend/internal/database/db.go)
[![CGO](https://img.shields.io/badge/CGO-0-555555)](Dockerfile)
[![Lisans](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)

Windows/Linux sunucuları, web siteleri ve IP cihazları için Türkçe izleme paneli. Go sunucusu, gömülü Vue arayüzü ve SQLite ile tek ikili olarak çalışır.

## Kurulum

Ubuntu 22.04 / 24.04 LTS:

```bash
curl -sSL https://raw.githubusercontent.com/BigDesigner/project-gnnwatchtower/main/install.sh | sudo bash
```

Panel varsayılanı: `https://localhost:3800`; uzaktan erişim için mevcut erişim kapsamını koruyarak `BIND_ADDRESS` ayarlayın.

Kullanıcı: `admin`. İlk parola yerel `data/initial_admin_password.txt` dosyasındadır; ilk girişte değiştirilir.

Kurulum betiği yönetici yetkisi ister; servis ayrı `gnnwatchtower` hesabıyla çalışır. Firewall kuralı eklenmez.

## Güncelleme

```bash
curl -sSL https://raw.githubusercontent.com/BigDesigner/project-gnnwatchtower/main/update.sh | sudo bash
```

Betik `main` dalını indirir, sunucuyu ve ajan ikililerini derler, mevcut veritabanı ve ayarları kullanmaya devam eder. Güncellemeden önce tutarlı bir yedek alın; geri dönüş notları [ayrıntılı belgede](docs/operations.md#sunucuyu-güncelleme-ve-yedekleme).

## Sunucu servisi

```bash
sudo systemctl status gnnwatchtower --no-pager  # Durum
sudo systemctl restart gnnwatchtower          # Yeniden başlat
sudo systemctl stop gnnwatchtower             # Durdur
sudo systemctl start gnnwatchtower            # Başlat
sudo journalctl -u gnnwatchtower -n 50 --no-pager
sudo journalctl -u gnnwatchtower -f            # Canlı günlük
```

## Ajan kurulumu ve servisleri

**Ajan Özeti > Ajan Dağıtımı** ekranından Windows/Linux ajanını ve kurulum komutunu alın. Kayıt anahtarı bu ekrandadır. Ajan kurulum betiklerinin API uçları yönetici oturumu gerektirir; kimlik doğrulamasız çağrılmaz.

### Linux

Kurulu ajanı yönetmek için:

```bash
sudo systemctl status gnnwatchtower-agent --no-pager
sudo systemctl restart gnnwatchtower-agent
sudo systemctl stop gnnwatchtower-agent
sudo systemctl start gnnwatchtower-agent
sudo journalctl -u gnnwatchtower-agent -f
```

Teşhis konsolu ve servis kaydını kaldırma:

```bash
sudo /opt/gnnwatchtower-agent/gnnwatchtower-agent -diag
sudo /opt/gnnwatchtower-agent/gnnwatchtower-agent -service uninstall
```

### Windows

Yönetici PowerShell ile; servis adı `GNNwatchtowerAgent`:

```powershell
Get-Service -Name GNNwatchtowerAgent
Restart-Service -Name GNNwatchtowerAgent
Stop-Service -Name GNNwatchtowerAgent
Start-Service -Name GNNwatchtowerAgent
Get-Content "$env:ProgramFiles\GNNwatchtower-Agent\agent.log" -Tail 50 -Wait
```

Teşhis konsolu ve servis kaydını kaldırma:

```powershell
& "$env:ProgramFiles\GNNwatchtower-Agent\gnnwatchtower-agent.exe" -diag
& "$env:ProgramFiles\GNNwatchtower-Agent\gnnwatchtower-agent.exe" -service uninstall
```

`-diag` etkileşimli konsol açar. `-service uninstall` servis kaydını kaldırır; dosyaları ve anahtarları silmez. Uzak ajan güncellemeleri **Ajan Özeti** bölümünden başlatılır.

## Docker Compose

Depo dizininde:

```bash
docker compose up -d --build
docker compose logs --tail=50 -f watchtower
docker compose restart watchtower
```

Ajan ikilileri Docker imajında derlenmez; panelden indirme için ayrıca hazırlanmalıdır. Paketleme ve erişim notları [ayrıntılı belgede](docs/operations.md#docker-compose).

## İzleme kapsamı

- **Ajan Özeti:** Windows/Linux erişilebilirliği ve mevcut ajan ölçümleri.
- **Web İzleme:** HTTP(S) erişilebilirliği ve yanıt süresi.
- **IP Cihazları:** NAS, ESXi ve ağ cihazlarında yalnızca ICMP ping.
- **Olay Günlüğü:** Kesinti, geri dönüş ve operatör sınıflandırması.
- **Yedekleme Merkezi:** Ayar, envanter ve olay kayıtlarının JSON yedeği.

Ağ keşfi kaldırılmıştır. NAS/ESXi izleme için ek yönetim servisi açılmaz veya ajan kurulmaz. Ping, disk/RAID sağlığını ya da cihazın kesin güç durumunu göstermez.

## Belgeler

[Kaynaktan derleme, elle ajan kurulumu, yapılandırma ve sorun giderme](docs/operations.md) · [Güvenlik ilkesi](.memory-bank/adr/0006-no-additional-attack-surface.md) · [AGPL-3.0 lisansı](LICENSE)

### 0.4.12 güvenlik geçişi

HTTP bağlantıları artık kabul edilmez. Güncellemeden önce [HTTPS geçiş adımlarını](docs/security-upgrade-0.4.12.md) tamamlayın. İlk kurulum parolası: `sudo cat /opt/gnnwatchtower/data/initial_admin_password.txt`.
