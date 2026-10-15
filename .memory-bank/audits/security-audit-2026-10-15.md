# GNNwatchtower Güvenlik Denetimi

**Tarih:** 15 Ekim 2026  
**Kapsam:** Kaynak kodu, dağıtım yapılandırmaları ve kurulum/güncelleme yönergelerinin salt okunur statik incelemesi  
**Değişiklikler:** Uygulama kaynak kodunda değişiklik yapılmadı.

## Yönetici özeti

İncelemede dağıtım ve ajan iletişim akışlarında istismar edilmesi halinde sistem/ajan güvenliğini ciddi etkileyebilecek sorunlar bulundu. Öncelik, kimlik doğrulamasız ajan kayıt ve rapor kabulünün düzeltilmesi, ilk kurulum parolasının kaldırılması ve TLS'nin zorunlu kılınması olmalıdır.

| Öncelik | Bulgu | Etki |
|---|---|---|
| Yüksek | Kimlik doğrulamasız ajan kaydı ve heartbeat cihazları ele geçirebiliyor | Envanter/telemetri sahteciliği, ajan anahtarının ele geçirilmesi |
| Yüksek | Varsayılan `admin/admin` hesabı ve zorunlu parola değişiminin olmaması | Yönetim paneline yetkisiz erişim |
| Yüksek | Yönetim ve ajan trafiği varsayılan olarak HTTP | Kimlik bilgisi/anahtar dinleme ve ajan güncelleme trafiğine müdahale |
| Orta | Parolalar hızlı SHA-256 ve sabit salt ile saklanıyor | Veritabanı sızıntısında çevrimdışı parola kırma |
| Orta | Web izleme URL'leri iç ağ hedeflerine yönlendirilebiliyor (SSRF) | Sunucunun erişebildiği iç servislerin sorgulanması |
| Orta | Giriş ve ajan raporu uç noktalarında hız/gövde sınırı görünmüyor | Kaba kuvvet ve kaynak tüketimi saldırıları |

## Bulgular

### 1. Kimlik doğrulamasız ajan kayıt/heartbeat cihaz kimliğini taklit edebiliyor — Yüksek

`POST /api/agent/enroll` herkese açık. İstek, `local_ip`, hostname veya machine UUID ile mevcut cihazı bulabiliyor ve eşleşen cihazın `agent_token` değerini yeni bir rastgele token ile değiştiriyor. Sunucu istemcinin o cihaz olduğunu doğrulamıyor. Ayrıca `POST /api/agent/report`, token geçersiz veya yoksa aynı alanlarla eşleşme arıyor; eşleşen cihazı çevrimiçi işaretleyip telemetri/olay durumunu güncelliyor. Bu nedenle ağdan erişebilen saldırgan kayıtlı cihazın kimliğini taklit edebilir, geçerli ajan anahtarını değiştirebilir ve izleme verisini bozabilir. İki uç nokta da herkese açık yönlendiricide tanımlı.

**Kanıt:** [handlers.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:829), [handlers.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:939), [router.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/router.go:38)

**Öneri:** Kayıt için tek kullanımlık/önceden yetkilendirilmiş enrollment kodu veya yönetici onayı kullanın. Heartbeat işlemlerinde bilinmeyen/boş token için otomatik eşleşme ve kayıt yolunu kaldırın; her ajan için geçerli, döndürülebilir anahtar zorunlu olsun. Token'ı yalnızca cihaz IP/hostname/UUID'siyle yenilemeyin.

### 2. Kurulum bilinen yönetici parolası oluşturuyor ve değişikliği zorunlu tutmuyor — Yüksek

Veritabanında kullanıcı bulunmadığında `admin/admin` hesabı oluşturuluyor. README ve kurulum betiği bu parolayı açıkça yayımlıyor. İlk girişte parola değiştirme zorunluluğu görülmedi; parola değiştirme ayrı, isteğe bağlı bir uç nokta. Uygulama ağa açık kurulduğunda varsayılan parolayı bilen herhangi biri yönetici erişimi kazanabilir.

**Kanıt:** [db.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/database/db.go:207), [README.md](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/README.md:16), [install.sh](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/install.sh:245)

**Öneri:** İlk açılışta benzersiz ve güvenli parola belirletin veya kurulum sırasında parola üretip yalnızca yerel/korumalı kanaldan gösterin. Varsayılan kimlik bilgileriyle ağa açık kullanımı engelleyin ve ilk oturumda parola değişimini zorunlu kılın.

### 3. Sunucu ve ajan API trafiğinde TLS zorunlu değil — Yüksek

Sunucu `ListenAndServe` ile tüm arayüzlerde HTTP dinliyor. README kurulum URL'si HTTP veriyor; ajan kurulum betikleri sunucu isteğinin şemasını HTTP olarak başlatıyor ve ajan enrollment/heartbeat/güncelleme indirmelerinde TLS doğrulaması veya başka bir kimlik sabitlemesi görünmüyor. Bu trafik yönetici parolası, oturum bearer token'ı ve ajan token'ı içeriyor. Ağ üzerindeki araya giren biri bu bilgileri okuyabilir veya HTTP yanıtlarını değiştirerek yönetici oturumunu/ajan trafiğini etkileyebilir. Ajan güncelleme dosyasının SHA-256 özeti de aynı HTTP yanıtıyla geldiğinden aktif araya girme saldırısına karşı bağımsız doğrulama sağlamaz.

**Kanıt:** [main.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/server/main.go:71), [README.md](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/README.md:16), [agent main.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:160), [agent main.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:369), [agent main.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:491)

**Öneri:** Üretimde HTTPS zorunlu kılın (uygulamanın içinde veya güvenilir reverse proxy ile), HTTP'yi HTTPS'ye yönlendirin ya da hassas uç noktaları HTTP'de reddedin. Ajan güncelleme imzasını/özeti sunucu API'sinden bağımsız güven köküyle doğrulayın.

### 4. Parola hash'i hızlı SHA-256 ve sabit salt kullanıyor — Orta

Parolalar `SHA-256(password + sabit metin)` biçiminde saklanıyor. Bu, parola saklama için tasarlanmış yavaş ve kullanıcı başına benzersiz salt kullanan bir KDF değildir. Veritabanı veya yedek sızarsa saldırgan parola adaylarını yüksek hızda çevrimdışı deneyebilir; salt kaynak kodda sabittir.

**Kanıt:** [db.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/database/db.go:219)

**Öneri:** Argon2id (tercihen) veya bcrypt/scrypt kullanın; her kullanıcı için rastgele salt saklayın. Mevcut SHA-256 kayıtlarını başarılı girişte yeni biçime yükseltin veya kullanıcı parolalarını sıfırlatın.

### 5. Web izleme hedefleri SSRF'ye açık — Orta

Yönetici tarafından eklenen web cihazının adresi sunucu tarafından düzenli olarak HTTP isteğiyle kontrol ediliyor. İstekler özel/loopback/link-local adreslere kısıtlanmıyor; URL yönlendirmeleri de varsayılan istemci davranışıyla izleniyor. Böylece cihaz ekleme yetkisi olan bir kullanıcı, sunucudan iç ağ servislerine veya bulut metadata adreslerine istek attırabilir. İzleme özelliği gereği dış hedeflere istek beklenir; risk, hedef ve yönlendirme kısıtlarının olmamasından kaynaklanır.

**Kanıt:** [handlers.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:271), [poller.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go:127), [poller.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go:190)

**Öneri:** URL'yi doğrulayın; loopback, link-local ve özel ağ hedeflerini varsayılan olarak engelleyin veya açık operatör izin listesi kullanın. Her yönlendirmede hedefi tekrar doğrulayın ve metadata servislerine erişimi ayrıca engelleyin.

### 6. Giriş ve ajan raporu uç noktalarında hız ve gövde sınırı görünmüyor — Orta

İncelenen router/middleware akışında giriş denemeleri için hız sınırlaması veya artan gecikme/kilitleme görünmüyor. Ajan heartbeat uç noktası herkese açık ve her kabul edilen rapor veritabanına telemetri yazıyor. İstek gövdeleri için görünür bir boyut sınırı da yok. Bu uç noktalar kötüye kullanılarak parola kaba kuvveti veya SQLite/veritabanı/disk kaynaklarının tüketilmesi denenebilir. Bulgular, ağ erişimi ve dağıtım topolojisine bağlıdır.

**Kanıt:** [router.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/router.go:38), [auth.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:43), [handlers.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:931), [main.go](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/server/main.go:71)

**Öneri:** IP/hesap bazlı login hız sınırlaması ve başarısız deneme gecikmesi ekleyin. İstek gövdesi boyutunu sınırlandırın; heartbeat için kota/anti-replay ve yazma frekansı sınırları uygulayın. Proxy arkasında istemci IP'sini güvenilir proxy ayarlarıyla belirleyin.

## İnceleme notları ve sınırlar

- İnceleme statik kod okuması ve yapılandırma taramasıyla sınırlıdır; canlı servise karşı sızma testi, derleme/test çalıştırması veya bağımlılık CVE/advisory eşleştirmesi yapılmadı.
- Bulgular kodda görülen davranışa dayanır. Üretim reverse proxy, firewall ve erişim politikaları bazı risklerin erişilebilirliğini azaltabilir; rapordaki HTTP/TLS bulgusu uygulamanın kendi varsayılan sunucu/ajan iletişimi için geçerlidir.
- Hassas veri sızıntısı, SQL injection veya frontend DOM XSS için bu incelemede doğrulanmış ek bir bulgu tespit edilmedi.
