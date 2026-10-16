# GNNwatchtower

[![Sürüm](https://img.shields.io/badge/version-0.4.10-blue)](VERSION)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8)](backend/go.mod)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D)](frontend/package.json)
[![SQLite](https://img.shields.io/badge/SQLite-WAL-003B57)](backend/internal/database/db.go)
[![CGO](https://img.shields.io/badge/CGO-0-555555)](Dockerfile)
[![Lisans](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)

GNNwatchtower; Windows/Linux sunucularını, web sitelerini ve IP cihazlarının erişilebilirliğini tek panelden izler. Arayüz Türkçedir. Sunucu, Vue arayüzünü ve SQLite veritabanı sürücüsünü içeren tek bir Go ikilisi olarak çalışır. Ayrı bir veritabanı servisi gerekmez.

## İzleme kapsamı

| Bölüm | İzlenen kayıtlar | Toplanan bilgi |
|---|---|---|
| Genel Bakış | Tüm kategoriler | Kayıt sayıları, güncel erişilebilirlik, açık olaylar ve ajan verisinin güncelliği |
| Ajan Özeti | Windows ve Linux sunucuları | Erişilebilirlik; ajan varsa CPU, bellek, disk doluluğu ve mevcut diğer ölçümler |
| Web İzleme | HTTP(S) adresleri | HTTP sonucu, yanıt süresi ve erişilebilirlik |
| IP Cihazları | NAS, ESXi, ağ anahtarı, yönlendirici ve diğer IP hedefleri | Yalnızca ICMP ping, ölçülebilen yanıt süresi ve son kontrol zamanı |
| Olay Günlüğü | Kesinti ve geri dönüş kayıtları | Başlangıç, bitiş, süre ve operatör sınıflandırması |
| Yedekleme Merkezi | Yapılandırma ve envanter | JSON dışa aktarma ve geri yükleme |

Cihazlar elle eklenir. Ağ keşfi ve alt ağ taraması kaldırılmıştır. Ajan dağıtımı ve ajan güncelleme işlemleri **Ajan Özeti** bölümündedir.

### Güvenlik sınırı

İzleme için ek saldırı yüzeyi oluşturulmaması projenin temel kuralıdır. NAS ve ESXi cihazlarında SNMP, SSH, vSphere/DSM API, IPMI veya başka bir yönetim servisi açılmaz; bu cihazlara ajan kurulmaz. Mevcut bir servisin açık olması, ona yeni erişim veya kimlik bilgisi tanımlamak için tek başına yeterli değildir.

Ping yanıtı cihazın disk, RAID veya güç sağlığını göstermez. Yanıt alınamaması da cihazın kapalı olduğunu kanıtlamaz; ağ kesintisi veya ICMP filtrelemesi söz konusu olabilir. İzin verilen yöntemle alınamayan veri, arayüzde mevcut değil olarak gösterilir. Ayrıntılar: [güvenlik kararı](.memory-bank/adr/0006-no-additional-attack-surface.md).

## Sunucu kurulumu

### Ubuntu ve systemd

Kurulum betiği Ubuntu 22.04 / 24.04 LTS için hazırlanmıştır. Depoyu indirip betiği inceleyerek çalıştırın:

```bash
git clone https://github.com/BigDesigner/project-gnnwatchtower.git
cd project-gnnwatchtower
sudo bash install.sh
```

Betik sunucuyu `/opt/gnnwatchtower` altına yerleştirir, `gnnwatchtower.service` servisini oluşturur ve başlatır. Yerel kaynak kod ve Go mevcutsa buradan derler; diğer kurulum yolunda deponun `main` dalını indirir. Gerekli derleme paketlerini kurabilir. Derlenmiş sunucu ikilisi kullanılan kurulum yolunda ajan ikililerinin ayrıca hazırlanması gerekebilir.

**Mevcut betik root yetkisi kullanır ve UFW varsa 3800/TCP için izin ekler.** Bu davranış, her müşteri ortamı için uygun bir dağıtım politikası sayılmaz. Kurulum öncesinde servis hesabını ve erişim kapsamını değerlendirin; yönetim panelini internete doğrudan açmayın.

Varsayılan panel adresi `http://SUNUCU_IP:3800` olur. İlk kullanıcı `admin`, varsayılan ilk parola `admin` olarak oluşturulur; ilk girişte değiştirilmesi gerekir. `INITIAL_ADMIN_PASSWORD` ortam değişkeni ilk kurulum parolasını belirleyebilir. Mevcut TLS yapılandırmanız varsa onun adresini kullanın.

### Docker Compose

```bash
git clone https://github.com/BigDesigner/project-gnnwatchtower.git
cd project-gnnwatchtower
docker compose up -d --build
```

Compose dosyası 3800 portunu yayımlar; verileri `gnnwatchtower_data`, yedekleri `gnnwatchtower_backups` birimlerinde tutar. Mevcut tanım `NET_RAW` ve `NET_ADMIN` yeteneklerini ekler; bu yetkilerin gerekliliği ve portun izin verilen kaynakları dağıtım öncesinde ayrıca değerlendirilmelidir.

Docker imajı sunucuyu derler; ajan ikililerini derleyip imaja yerleştirmez. Panelden ajan indirmek için aşağıdaki derleme bölümünde üretilen ikililerin sunucunun `/opt/gnnwatchtower/bin/` dizininde bulunması gerekir. Konteyner kullanıyorsanız dosyaları mevcut dağıtımınıza salt okunur bağlama veya imaj paketleme yoluyla dahil edin; yalnızca indirme için yeni bir servis açmayın.

### Yapılandırma ve dosyalar

| Konum / değişken | Açıklama |
|---|---|
| `/opt/gnnwatchtower/gnnwatchtower` | Sunucu ikilisi |
| `/opt/gnnwatchtower/.env` | systemd kurulumunun ortam dosyası |
| `/opt/gnnwatchtower/data/gnnwatchtower.db` | SQLite veritabanı |
| `/opt/gnnwatchtower/backups/` | Otomatik ve güncelleme öncesi yedekler |
| `/opt/gnnwatchtower/bin/` | İndirilebilir Linux ve Windows ajan ikilileri |
| `SERVER_PORT` | Sunucu portu; varsayılan 3800 |
| `DB_PATH` | Veritabanının dosya yolu |
| `INITIAL_ADMIN_PASSWORD` | İlk kurulum parolası; parola değişimi tamamlanana kadar başlangıç hesabında kullanılır |
| `TLS_ENABLED`, `TLS_CERT_FILE`, `TLS_KEY_FILE` | Sunucunun mevcut TLS desteği |

Sunucu tek başına başlatıldığında `.env` dosyasını kendiliğinden okumaz. systemd birimi bunu `EnvironmentFile` ile yükler; diğer çalıştırma biçimlerinde değişkenleri süreç ortamına aktarın. TLS örneklerinde kullanılan sertifikanın istemciler tarafından doğrulanabilmesi gerekir; sertifika doğrulamasını kapatmayın.

## Sunucu servisi

Bu komutlar Ubuntu/systemd kurulumu içindir:

```bash
sudo systemctl status gnnwatchtower.service --no-pager
sudo systemctl start gnnwatchtower.service
sudo systemctl stop gnnwatchtower.service
sudo systemctl restart gnnwatchtower.service
sudo journalctl -u gnnwatchtower.service -n 50 --no-pager
sudo journalctl -u gnnwatchtower.service -f
```

Docker kurulumunda:

```bash
docker compose ps
docker compose logs --tail=50 -f watchtower
docker compose restart watchtower
```

## Windows ve Linux ajanları

Ajanı yalnızca mevcut ajan politikasının izin verdiği Windows/Linux sunucularında kullanın. Örnekler NAS ve ESXi cihazları için değildir. Ajan merkeze HTTP(S) istekleri gönderir; uç noktada gelen bağlantı kabul eden yeni bir izleme portu açmaz. Bu özellik tek başına kurulumun risksiz olduğu anlamına gelmez.

**Ajan Özeti > Ajan Dağıtımı** üzerinden ilgili ikiliyi indirin ve kayıt anahtarını alın. `/api/agent/install/linux` ve `/api/agent/install/windows` betik uçları yönetici oturumu gerektirir; kimlik doğrulamasız `curl | bash` veya PowerShell çağrılarıyla kullanılamaz.

Aşağıdaki `https://watchtower.example.com:3800` adresini mevcut panelinizin adresiyle, `KAYIT_ANAHTARI` değerini paneldeki anahtarla değiştirin. Örnekler doğrulanabilir HTTPS kullanan bir kurulum içindir. Kayıt anahtarı gizlidir; komut geçmişine, süreç argümanlarına ve dağıtım günlüklerine girebilir. Komutları anahtar içermiyormuş gibi paylaşmayın.

### Linux ajanını kurma

İndirilen Linux ikilisinin bulunduğu dizinde:

```bash
sudo install -d -m 0755 /opt/gnnwatchtower-agent
sudo install -m 0755 ./gnnwatchtower-agent /opt/gnnwatchtower-agent/gnnwatchtower-agent
sudo /opt/gnnwatchtower-agent/gnnwatchtower-agent -service install \
  -server "https://watchtower.example.com:3800" \
  -enroll-key "KAYIT_ANAHTARI"
```

Linux'ta `-service install` systemd birimini oluşturur, etkinleştirir ve servisi başlatır. Yönetim ve günlük komutları:

```bash
sudo systemctl status gnnwatchtower-agent.service --no-pager
sudo systemctl start gnnwatchtower-agent.service
sudo systemctl stop gnnwatchtower-agent.service
sudo systemctl restart gnnwatchtower-agent.service
sudo journalctl -u gnnwatchtower-agent.service -n 50 --no-pager
sudo journalctl -u gnnwatchtower-agent.service -f
```

### Windows ajanını kurma

İndirilen Windows ikilisinin bulunduğu dizinde **yönetici PowerShell** açın:

```powershell
.\gnnwatchtower-agent.exe -service install -server "https://watchtower.example.com:3800" -enroll-key "KAYIT_ANAHTARI"
Start-Service -Name GNNwatchtowerAgent
Get-Service -Name GNNwatchtowerAgent
```

Windows servisinin gerçek adı `GNNwatchtowerAgent`, görünen adı `GNNwatchtower Telemetry Agent` şeklindedir. Dosya `$env:ProgramFiles\GNNwatchtower-Agent` altına kopyalanır. Windows'ta kurulum komutundan sonra servis ayrıca başlatılır.

```powershell
Get-Service -Name GNNwatchtowerAgent
Start-Service -Name GNNwatchtowerAgent
Stop-Service -Name GNNwatchtowerAgent
Restart-Service -Name GNNwatchtowerAgent
Get-Content -Path "$env:ProgramFiles\GNNwatchtower-Agent\agent.log" -Tail 50 -Wait
```

### Ajan komutları ve yerel dosyalar

Her iki platformda ajan ikilisi şu servis işlemlerini destekler: `-service install`, `start`, `stop`, `restart`, `uninstall`, `status` ve `diag`. Her değer `-service` parametresiyle birlikte kullanılır. `-status`, `-diag` ve `-service status` etkileşimli teşhis konsolunu açar; bunlar yalnızca durum yazıp çıkan komutlar değildir. Bağlantı/telemetri kontrolü yapabildikleri için sadece kayıtlı merkez adresiyle çalıştırın.

```bash
sudo /opt/gnnwatchtower-agent/gnnwatchtower-agent -diag
sudo /opt/gnnwatchtower-agent/gnnwatchtower-agent -service uninstall
```

```powershell
& "$env:ProgramFiles\GNNwatchtower-Agent\gnnwatchtower-agent.exe" -diag
& "$env:ProgramFiles\GNNwatchtower-Agent\gnnwatchtower-agent.exe" -service uninstall
```

Kaldırma komutu servis kaydını kaldırır; program dizinini ve yerel anahtarları silmez. Ajan dizinindeki `agent.key` cihaza ait kimliği, `enroll.key` kayıt anahtarını, `server.url` merkez adresini, `signer.pub` ise güncelleme doğrulama açık anahtarını tutar. Bir cihazın kimlik dosyasını başka bir cihaza kopyalamayın. Servis zaten kuruluysa kurulumu tekrar çalıştırmak yerine mevcut servisi ve kayıt durumunu inceleyin.

### Komut satırı seçenekleri

| Parametre | Kullanım |
|---|---|
| `-server` | Merkez HTTP(S) adresi |
| `-enroll-key` | İlk kayıt anahtarı; yerelde `enroll.key` dosyasına kaydedilir |
| `-token` | Mevcut cihaz kimliği; ilk kurulumda normalde otomatik alınır |
| `-signer-key` | Güncelleme imzası için Ed25519 doğrulama açık anahtarı |
| `-interval` | Ön planda çalıştırmada rapor aralığı; varsayılan 15 saniye |
| `-require-https` | Bu çalıştırmada HTTP merkez adresini reddeder |
| `-version` | Sürümü gösterip çıkar |

Servis kurucusu çalışma argümanlarına `-server` ve verilmişse `-token` yazar. Kurulum komutuna `-interval` veya `-require-https` eklemek bu seçenekleri kalıcı servis ayarına dönüştürmez. Servisin kullandığı adresi ve ortamı ayrıca kontrol edin.

### Ajan güncelleme

**Ajan Özeti** içinden tek ajanı veya kayıtlı ajanları topluca güncelleme kuyruğuna alabilirsiniz. Talimat, ajanın sonraki raporuna verilen yanıtta iletilir. Ajan ikiliyi indirir; SHA-256 özeti ve Ed25519 imzasını doğruladıktan sonra uygular ve servisi yeniden başlatır. İşlem sırasında kısa bir raporlama aralığı oluşabilir.

Sunucunun `bin/` dizininde hedef platforma ait ikili bulunmalıdır. Sunucuyu güncellemek, bütün uzak ajanların kendiliğinden güncellendiği anlamına gelmez. `update.sh` ayrıca sunucu üzerinde mevcut `/opt/gnnwatchtower-agent` dizinini bulursa o yerel ajanı da güncelleyip yeniden başlatır.

## Sunucuyu güncelleme ve yedekleme

`update.sh` deponun `main` dalını indirip sunucu ve ajan ikililerini derler. Mevcut `.env` ve veritabanını kullanmaya devam eder:

```bash
sudo systemctl stop gnnwatchtower.service
sudo bash update.sh
sudo systemctl status gnnwatchtower.service --no-pager
```

Komutları incelenmiş deponun kök dizininden çalıştırın. Güncellemeden önce süreç durdurulmuşken veritabanı, varsa WAL/SHM dosyaları, yapılandırma, imza anahtarları ve eski ikilinin tutarlı bir kopyasını saklayın. Betiğin otomatik dosya kopyasını tek başına tam geri dönüş yedeği saymayın.

Panelin JSON yedeği ayarları, cihazları, servis yapılandırmalarını ve son 1000 olayı içerir; ajan metrik geçmişini içermez. Zamanlanmış yedekleme sunucu saatine göre 03:00 civarında çalışır ve 30 günlük saklama uygular. Yedekler gizli anahtarlar içerebilir.

İzleme kategorilerini ekleyen veritabanı geçişinden sonra eski sunucu ikilisini aynı veritabanıyla çalıştırmayın. Eski sürüm kategori sınırlarını uygulamayabilir. Geri dönüşte eşleşen eski ikili ve güncelleme öncesi veri/yapılandırma kopyası birlikte kullanılmalıdır.

## Kaynaktan derleme

Go 1.22 veya üzeri ile Node.js/npm gerekir. Komutlar depo kökünden ve hedef sunucu mimarisine uygun bir derleme ortamında çalıştırılır. Linux/Windows ajan örnekleri amd64 içindir:

```bash
(cd frontend && npm ci && npm run build)
mkdir -p bin
(cd backend && CGO_ENABLED=0 go build -o ../bin/gnnwatchtower ./cmd/server)
(cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../bin/gnnwatchtower-agent ./cmd/agent)
(cd backend && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../bin/gnnwatchtower-agent.exe ./cmd/agent)
```

Arayüz derlemesi `backend/cmd/server/dist/` dizinine yazılır ve sunucu derlenirken ikiliye gömülür. Çalışma anında Node.js gerekmez. ICMP kontrolleri sunucudaki mevcut işletim sistemi `ping` komutunu kullanır; komut veya gerekli izin yoksa ölçüm bilinmiyor olarak kalır. Bunu gidermek için uygulamaya ek yetki ya da hedefte yeni servis vermeyin.

## Sorun giderme

| Belirti | Kontrol |
|---|---|
| Ajan indirme isteği 404 dönüyor | Sunucuda ilgili Linux/Windows ikilisinin `bin/` dizininde bulunup bulunmadığını kontrol edin. |
| Kurulum betiği 401 veya 403 dönüyor | Ajan Dağıtımı ekranındaki yetkili oturumu kullanın; betik uçları oturum ister. IP cihazları ajan işlemlerini kabul etmez. |
| Ajan kayıt isteği 409 dönüyor | Cihaz daha önce kaydedilmiş olabilir. Mevcut ajan kimliğini ve eşleşen envanter kaydını inceleyin; anahtar dosyalarını silerek yeniden kaydı zorlamayın. |
| Windows servisi bulunamıyor | Servis adını `GNNwatchtowerAgent` olarak kullanın ve kurulumu yönetici PowerShell ile yapın. |
| Linux servisi başlayamıyor | `journalctl -u gnnwatchtower-agent.service` çıktısını, kayıtlı merkez adresini ve TLS doğrulamasını inceleyin. |
| IP cihazında metrikler görünmüyor | Bu kategori yalnızca ICMP erişilebilirliğini izler; disk/RAID/CPU verisi toplanmaz. |
| Erişilebilirlik veya ajan verisi güncel değil | Son kontrol/örnek zamanını ve izleme profilini kontrol edin. Eski veri güncel sağlık sonucu olarak kullanılmaz. |

## Lisans

[GNU Affero General Public License v3.0](LICENSE).
