# ROAD UDP Test Report - 2026-05-13

## Sonuc

ROAD UDP tarafi DDNet ve Sven Co-op ile pratik oyun testinden gecti. Iki oyunda da domain uzerinden `wss://road.example.com/ws` akisi kullanildi ve test sirasinda ROAD tarafinda kritik hata gorulmedi.

Lethal Company'deki obje/fizik senkron sorunlari ROAD disinda, saf LAN testinde de tekrarlandi. Bu nedenle Lethal Company LAN modu ROAD icin guvenilir bir dogrulama oyunu sayilmadi.

## Test Kapsami

- Tarih: 2026-05-13
- Windows istemci: `127.0.0.1` oyun hedefi uzerinden ROAD client
- Linux laptop: Fedora, SvenDS/DDNet server ve ROAD server testleri
- Domain: `road.example.com`
- Transport: WebSocket Secure (`wss`)
- Kritik test hedefi: UDP tünelin oyun trafiğinde paket akisini bozup bozmadigini pratik olarak gormek

## Topoloji

### Sven Co-op son dogru topoloji

- Laptop SvenDS: `127.0.0.1:27015`
- Laptop ROAD server: `0.0.0.0:8080` ve API `0.0.0.0:8081`
- Cloudflare tunnel: `road.example.com` -> laptop `127.0.0.1:8080/8081`
- Windows ROAD client: `127.0.0.1:27015` dinledi
- Windows Sven Co-op: `connect 127.0.0.1:27015`
- ROAD plugin: `sven-coop-udp`

Bu topoloji bilerek kuruldu. SvenDS sadece loopback'e bind edildigi icin Windows istemcinin sunucuya LAN IP ile direkt kacmasi engellendi. SvenDS logunda oyuncu adresinin `127.0.0.1:59771` gorunmesi, trafigin ROAD uzerinden geldigini kanitladi.

## DDNet Testi

- Plugin: `ddnet-udp`
- Domain uzerinden ROAD kullanildi.
- DDNet server logunda oyuncu map bitirdi.
- ROAD server tarafinda kritik hata gorulmedi.
- Test sirasinda oynanis genel olarak akiciydi.
- Kisa ekran donmalari yorumlandi; oyun/istemci performansi ile ag donmasi ayirt etmek icin DDNet tek basina nihai kanit sayilmadi, ancak UDP tunel davranisi olumlu goruldu.

Onemli kanit:

```text
player-one finished Tutorial in 5 minute(s) 38.36 second(s)
ROAD errors: 0
```

## Sven Co-op Testi

Sven Co-op testi daha temiz kanit verdi cunku Half-Life/GoldSrc tarzi UDP oyun trafigiyle uzun sureli 3D oyun akisi denendi.

Son API istatistikleri:

```text
plugin: sven-coop-udp
uptime: ~23 dakika
active_connections: 5
total_connections: 11
total_bytes_rx: 7,657,215
total_bytes_tx: 16,750,357
errors: 0
main session age: ~22 dakika 56 saniye
main session rx packets: 226,662
main session tx packets: 32,570
main session max payload rx: 421 bytes
main session max payload tx: 1400 bytes
packets > 1472 bytes: 0
```

SvenDS log kaniti:

```text
"player-one<1><STEAM_ID_LAN><>" connected, address "127.0.0.1:59771"
Started map "hl_c04"
Started map "hl_c05_a1"
```

Yorum:

- Oyun domain uzerinden ROAD'a baglanarak akti.
- Kullanici tarafinda belirgin lag/desync gorulmedi.
- ROAD server `errors: 0` raporladi.
- MTU tarafinda kritik fragmentasyon sinyali yoktu; `>1472=0`.
- Sven Co-op testi, Lethal Company'deki sorunun ROAD'dan ziyade oyunun LAN/fizik senkronundan kaynaklandigi argumanini guclendirdi.

## Lethal Company Degerlendirmesi

Lethal Company LAN testinde ROAD devrede degilken bile ayni tur obje senkron tutarsizliklari goruldu:

- Bir makinede obje masada, digerinde yerde gorunebildi.
- Online/LAN davranisinda obje gorunurlugu ve fizik state farklari olusabildi.
- Bu durum ROAD proxy olmadan da tekrarlandigi icin Lethal Company, ROAD UDP tünelini dogrulamak icin guvenilir referans kabul edilmedi.

Sonuc: Lethal Company ile gorulen obje/fizik desync, ROAD icin dogrudan hata kaniti degil.

## Metrik Notlari

ROAD'un genel UDP metrikleri oyun trafigi icin faydali sinyal verir:

- tx/rx paket ve byte sayilari
- jitter ve max gap
- payload boyutu
- MTU esiklerini asan paket sayilari
- session kapanis bilgileri

Ancak sequence/loss/reorder benzeri metrikler sadece ROAD'un kendi sentetik test protokolunde tam anlamlidir. DDNet, Sven Co-op veya baska oyunlarin ham UDP paketleri farkli protokol kullandigi icin bu paketlerde gorunen loss/reorder degerleri nihai kanit olarak yorumlanmamalidir.

## Operasyonel Reset

Test bitiminde yapilmasi gereken normal durum:

- Cloudflare tunnel tekrar Windows localhost'a donmeli:
  - `/api/*` -> `http://127.0.0.1:8081`
  - diger proxy trafigi -> `http://127.0.0.1:8080`
- Windows Sven ROAD client kapatilmali.
- Laptop SvenDS ve laptop ROAD server test sessionlari kapatilmali.
- Windows ROAD server normal plugin/profil ile calismaya devam edebilir.

## Cikarim

ROAD bu test setinden sonra su seviyede kabul edilebilir:

- TCP tarafi daha once Minecraft gibi testlerle pratik olarak dogrulandi.
- UDP tarafi GZDoom, DDNet ve Sven Co-op ile pratik oyun akisi seviyesinde dogrulandi.
- Lethal Company gibi kendi LAN/fizik senkronu zayif oyunlarda ROAD hatasi ile oyun hatasini ayirmak icin once saf LAN testi sart.

## Sonraki Isler

- `udp-check` sentetik testini daha gorunur CLI komutu haline getirmek.
- Oyun protokolunden bagimsiz packet sequence/loss olcumu icin sadece ROAD test paketlerini ayri isaretlemek.
- API arayuzunde MTU ve max-gap uyarilarini daha okunur gostermek.
- DDNet ve Sven Co-op profillerini README'de test profili olarak belgelemek.
- Uzun sureli 3 kisi testi yapildiginda bu rapora ek not dusmek.
