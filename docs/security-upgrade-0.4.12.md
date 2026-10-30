# 0.4.12 güvenlik geçişi

Bu sürümde panel ve ajan trafiği HTTPS kullanır. HTTP kullanan mevcut kurulumlarda önce aşağıdaki geçiş tamamlanmalıdır; `update.sh` hazırlık yoksa çalışan servise dokunmadan durur.

## Mevcut kurulum

1. Mevcut yönetim adresi için istemcilerin **zaten güvendiği** bir sertifika sağlayın. Sertifikanın SAN alanı kullanılan DNS adı/IP adresini kapsamalıdır. Sertifika ve anahtarı `/opt/gnnwatchtower/data/` altında tutun. Anahtarı yalnızca servis hesabı okuyabilmelidir. Sertifika doğrulamasını kapatmayın, `curl -k` kullanmayın.
2. `/opt/gnnwatchtower/.env` içinde `TLS_ENABLED=true`, `TLS_CERT_FILE`, `TLS_KEY_FILE` ve `BIND_ADDRESS` tanımlayın. `BIND_ADDRESS` mevcut yönetim arayüzünün IP adresi olmalıdır; erişim kapsamını genişletmeyin. Yeni firewall kuralı eklemeyin. Mevcut sürüm TLS desteği içerir; geçişi bu sürümle tamamlayın.
3. Sunucuyu kontrollü bir bakım aralığında yeniden başlatın. Paneli aynı adresin `https://` karşılığı üzerinden açın. Sertifika hatası varsa geçiş tamamlanmış değildir.
4. Mevcut ajanların kayıtlı sunucu URL'sini yerel yönetim araçlarıyla aynı adresin `https://` karşılığına geçirin ve servislerini yeniden başlatın. Linux dosyası `/opt/gnnwatchtower-agent/server.url`, Windows dosyası `C:\Program Files\GNNwatchtower-Agent\server.url` konumundadır. Servis komut satırında eski `-server http://...` varsa onu da düzeltin. Mevcut `agent.key` ve `signer.pub` dosyalarını koruyun. Yeni bir CA'ya toplu güven tanımlamayın; sertifika zaten onaylanmış güven zincirini kullanmalıdır.
5. Hazırlık tamamlandığında README'deki normal güncelleme komutunu kullanın. Güncelleme `gnnwatchtower` servis hesabını ve systemd kısıtlarını kurar; veriler bu hesabın sahipliğine geçirilir. `.env` root tarafından okunmaya devam eder. Mevcut firewall kuralları otomatik değiştirilmez.

Güncelleme öncesinde servis durdurularak DB, varsa WAL/SHM yan dosyaları ve `.env` yedeklenir. Özel sertifika/anahtarlar ve `data/agent_signer.key` ayrıca güvenli çevrimdışı yedekte korunmalıdır. Yedekler erişim anahtarları içerebilir; repoya veya herkese açık alana yüklemeyin.

## Yeni kurulum

Panel varsayılan olarak yalnızca `https://localhost:3800` dinler. Otomatik oluşturulan yerel sertifika uzaktaki ajanlar tarafından kendiliğinden güvenilir sayılmaz. Uzaktan ajan kurulumu öncesinde yukarıdaki sertifika ve erişim hazırlığını tamamlayın. Aynı makinede, sertifikanın parmak izini yerel dosyadan doğruladıktan sonra `curl --cacert /opt/gnnwatchtower/data/cert.pem https://localhost:3800/api/health` kullanılabilir.

İlk kullanıcı `admin`; rastgele tek kullanımlık parola:

```bash
sudo cat /opt/gnnwatchtower/data/initial_admin_password.txt
```

Parola değişince dosya silinir. Tamamlanmamış eski `admin/admin` hesapları rastgele parolaya geçirilir; parolasını daha önce değiştirmiş hesaplara dokunulmaz.

## Docker ve ICMP

Compose portu host üzerinde yalnızca loopback'e yayımlar. Konteyner UID/GID `10001` ile çalışır; eski veri hacimleri kontrollü bakım sırasında bu kimliğe geçirilmeli ve mevcut TLS dosyaları bu hesap tarafından okunabilmelidir. `NET_ADMIN` ve `NET_RAW` verilmez. Konteynerin ping grubu yalnızca kendi GID'sine sınırlandırılır. Host servisinde işletim sisteminin mevcut yetkisiz ping desteği yoksa sonuç “bilinmiyor” olur; bunu aşmak için servis root yapılmamalı veya yeni capability verilmemelidir.

Bildirimler en fazla dört işçi ve 128 bekleyen işle sınırlandırılır. Aynı IP/olay türü üç dakika içinde yeniden gönderilmez; olay kayıtları korunur. Kuyruk dolduğunda bildirim atlanır ve günlüğe yazılır. HTTP 429 yanıtlarında en fazla iki tekrar yapılır; uzun `Retry-After` değerleri bekleyen işçileri sınırsız tutmaz.
