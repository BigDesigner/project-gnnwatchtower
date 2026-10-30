# 0.4.13 — tek komutla kurulum ve güncelleme

## İlk kurulum

```bash
curl -sSL https://raw.githubusercontent.com/BigDesigner/project-gnnwatchtower/main/install.sh | sudo bash
```

## Güncelleme

```bash
curl -sSL https://raw.githubusercontent.com/BigDesigner/project-gnnwatchtower/main/update.sh | sudo bash
```

Elle `BIND_ADDRESS`, `TLS_ENABLED` veya sertifika ayarı gerekmez. Mevcut adres ve özel sertifika yolları korunur; eksik adres mevcut dinleyiciden veya yerel IPv4 adresinden belirlenir. Mevcut kurulumda yanlışlıkla `install.sh` çalıştırılırsa güvenli güncelleme akışına geçilir. Betikler firewall kuralı açmaz.

Derleme mevcut servis çalışırken yapılır. Gerekirse geçici derleme dizinine resmi Go indirme metadatasındaki SHA-256 ile doğrulanan derleyici alınır; sistemdeki Go değiştirilmez. Yeni ikililer hazırlandıktan sonra zaten TLS kullanan aktif eski ajanlara mevcut imzalı güncelleme mekanizmasıyla yeni sürüm sunulur ve 45 saniyelik geçiş aralığı bırakılır. Bu aralık bütün ajanların güncellendiğini kanıtlamaz. Ardından servis durdurulur, veri dizini/ortam/önceki ikili yedeklenir ve yeni sunucu başlatılır. HTTPS hazır olmazsa önceki sunucu ve ortam geri yüklenmeye çalışılır; hata günlüğü gösterilir.

## Panel ve ajan kurulumu

Panel çıktıda gösterilen `https://` adresinden açılır. Eski HTTP panel adresi HTTPS'e yönlenir. Özel sertifikada tarayıcı ilk bağlantıda uyarı gösterebilir: sunucu konsolunda gösterilen SHA-256 parmak iziyle karşılaştırın. Kurumunuzun zaten güvendiği sertifika varsa aynı sertifika kullanılmaya devam eder. Tarayıcının ilk sunucu kimliği doğrulamasını güvenli biçimde tamamen ortadan kaldırmak mümkün değildir.

Ajan Özeti > Ajan Dağıtımı ekranındaki Windows veya Linux komutunu kopyalayıp hedefte yönetici olarak çalıştırın. Komut, yetkilendirilmiş HTTPS oturumundan alınan kurulum betiğini içerir; panel oturum anahtarını hedef bilgisayara taşımaz. Betik yalnızca Watchtower'a ait sertifikayı ve imza anahtarını uygulama dizinine kaydeder; işletim sisteminin genel güven deposuna dokunmaz. İndirilen ikilinin SHA-256 özeti doğrulanmadan çalışan ajan durdurulmaz. Tek başına indirilen ikili dosya bu güven yapılandırmasının yerine geçmez.

## Mevcut ajanların geçiş sınırı

Yeni ajan, kayıtlı eski HTTP adresiyle başlatılsa bile kimlik bilgilerini ve telemetriyi yalnızca HTTPS üzerinden gönderir. Mevcut `signer.pub` anahtarıyla doğrulanan, rastgele sorguya bağlı imzalı sertifika bilgisini aynı porttaki mevcut açık anahtar yolundan alabilir. Yanlış imza, eksik pin veya sertifika doğrulama hatası kabul edilmez. Yeni port, genel CA kaydı veya TLS doğrulamasını kapatan seçenek yoktur.

Eski HTTP kurulumlarından gelen, geçiş sırasında çevrimdışı kalan, indirmeyi tamamlayamayan veya önceden güvenilen imza anahtarı bulunmayan eski ajanlarda paneldeki kurulum komutu yeniden çalıştırılmalıdır. Kapalı bir bilgisayara uzaktan güncelleme uygulanmış gibi durum gösterilmez. NAS/ESXi ve ağ cihazlarının ICMP izlemesi bu geçişten bağımsızdır; bu cihazlara ajan veya yönetim servisi eklenmez.

İlk kullanıcı `admin`; rastgele parola `sudo cat /opt/gnnwatchtower/data/initial_admin_password.txt` ile yerel olarak okunur. Önceden değiştirilmiş operatör parolaları korunur.
