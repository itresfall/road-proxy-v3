# ROAD Proxy v3 Yapilacaklar Listesi

Tarih: 2026-05-11

Bu liste mevcut kod, dokumanlar, test durumu ve son GZDoom/Lethal Company deneylerinden sonra cikarildi. Guvenlik basliklari bilerek disarida tutuldu; odak urunlesme, UDP uyumlulugu, gelistirici deneyimi ve operasyonel kalite.

## Mevcut Durum

- `go test ./...` geciyor.
- TCP tarafi pratik kullanimda olgun.
- UDP tarafi GZDoom ve Lethal Company gibi gercek oyun senaryolarinda deger uretmeye basladi.
- Plugin sistemi JSON tabanli ve native kod yuklemiyor.
- Plugin Studio artik temel port yakalama aracindan compatibility profile mantigina dogru evrildi.

## Tamamlanan P0

- Release paketleyici eklendi: `scripts/build-cross.ps1 -Package`.
- Windows/Linux build sonrasi otomatik `.zip` paketi ve `.sha256` checksum uretimi eklendi.
- Pakete `configs`, `plugins`, `scripts`, binaryler, README, CHANGELOG ve `VERSION.txt` giriyor.
- Binary versiyon bilgisi eklendi: `--version`, build tarihi, commit/hash ve manuel `ROAD_VERSION`.
- Versiyonlama stratejisi SemVer olarak belirlendi; gelistirme build varsayilani `0.1.0-dev`.
- `CHANGELOG.md` eklendi ve release notlari buradan takip edilecek.
- `SECURITY.md`, `CONTRIBUTING.md` ve `docs/RELEASE-CHECKLIST.md` eklendi.
- README icine minimum Go surumu yazildi: Go 1.23+.
- Repo kokundeki eski binary artiklari temizlendi; ciktilar `build/` altinda tutuluyor.
- `.gitignore` build, root binaryler, log klasorleri ve gecici raporlari kapsayacak sekilde genisletildi.

## Tamamlanan P1 - CI/CD

- GitHub Actions workflow eklendi: `.github/workflows/ci.yml`.
- Push, pull request ve manuel calistirmada `go test ./...` calisiyor.
- Windows amd64 build smoke testi eklendi.
- Linux amd64 build smoke testi eklendi.
- Linux arm64 cross-build smoke testi eklendi.
- Gercek `configs/*.json` dosyalarini yukleyen repo QA testi eklendi.
- Gercek `plugins/*/plugin.json` profillerini loader uzerinden validate eden repo QA testi eklendi.
- Tag uzerinde `scripts/build-cross.ps1 -Package` ile release zip/checksum artifact uretimi eklendi.
- Build scriptlerinin CI'da interactive olmadan calistigi dogrulandi.

## Tamamlanan P1 - Kod Yapisi

- `cmd/road/main.go` yalnizca giris noktasi olacak sekilde kucultuldu.
- Menu, input ve yes/no helperlari `cmd/road/menu.go` dosyasina tasindi.
- Server flow kodu `cmd/road/flow_server.go` dosyasina tasindi.
- Client flow kodu `cmd/road/flow_client.go` dosyasina tasindi.
- Runtime path/config yazma helperlari `cmd/road/runtime_files.go` dosyasina tasindi.
- URL normalize ve control API fetch helperlari `cmd/road/wsurl.go` dosyasina tasindi.
- Port owner bulma, kill ve parser kodu `cmd/road/portcheck.go` dosyasina toplandi.
- TCP/UDP port kontrol tekrarini tek `ensurePortFree(proto, port)` fonksiyonu karsiliyor.
- Windows `netstat` parseri TCP/UDP icin ortak hale getirildi.
- `cmd/server` ve `cmd/client` default config path kontrolu `internal/app.IsDefaultRelativePath` helperina tasindi.

## Tamamlanan P1 - Public Server Wizard MVP

- `road-proxy public-server` komutu eklendi.
- Built-in menuye Public Server Wizard girisi eklendi.
- `cloudflared` binary kontrolu ve kullanici dizinine auto-download MVP'si eklendi.
- TryCloudflare quick tunnel modu eklendi; URL stdout/stderr uzerinden regex ile yakalaniyor.
- Tokenli Cloudflare tunnel modu eklendi.
- `cloudflared tunnel login`, `tunnel create`, `tunnel route dns` ve generated ingress config kullanan domain modu eklendi.
- Domain modu generated ingress config icinde `/ws` data portuna, `/dashboard` ve `/api/*` control portuna yonlenir.
- Public wizard guvenli ROAD server config uretir: local-only bind, token auth, private plugin API, connection/rate limit.
- Public wizard matching client hint config uretir: `configs/.generated/client.public.menu.json`.
- Public wizard ayni anda ikinci kez baslatilmamasi icin `configs/.generated/public-server.lock` kullanir.
- Public wizard local data/control port override destekler; TryCloudflare `--url` ve named tunnel ingress service ayni local origin'den uretilir.
- Ctrl+C ile ROAD engine ve `cloudflared` processleri birlikte kapatiliyor.
- `cloudflared` auto-download icin SHA256 checksum dogrulamasi eklendi.
- TryCloudflare modunda kullanici `~/.cloudflared/config.yml` dosyasindan etkilenmeyen izole config stratejisi eklendi.
- Domain modunda mevcut tunnel veya DNS kaydi varsa yeniden kullanma akisi eklendi.
- Cloudflare API token olmadan nelerin otomatik yapilabilecegi `docs/CLOUDFLARE-WIZARD-LIMITS.md` icinde netlestirildi.
- `cloudflared tunnel login` sonrasi gelen `cert.pem` ile tunnel create ve route dns otomasyonunun yeterli olduguna karar verildi.
- ROAD Public Server Wizard kullanicidan Cloudflare API token istemeyecek.
- Domain saglayici nameserver tasima isi otomasyon kapsami disinda birakildi; bunun yerine rehber yazildi.
- Acik kalan konu: Cloudflare API token ile zone/domain secimi ancak gercek ihtiyac dogarsa tekrar degerlendirilecek.

## Tamamlanan P2 - UTF-8 ve i18n Temeli

- UTF-8 standardi ve ceviri akisi `docs/I18N.md` icinde belirlendi.
- `internal/i18n` paketi eklendi.
- `locales/en.json` ana ceviri kalibi yapildi.
- `locales/tr.json` ilk ceviri dosyasi olarak eklendi.
- Eksik key durumunda Ingilizce fallback eklendi.
- Missing translation raporu icin `MissingKeys()` destegi eklendi.
- Formatli string destegi test edildi.
- `cmd/road` metinleri key-based i18n sistemine tasindi.
- `cmd/road` icindeki mojibake gorunen kullanici metinleri temizlendi.
- `cmd/plugin-studio` ekran metinleri, profil notlari ve launch hintleri key-based i18n sistemine tasindi.
- `cmd/server` ve `cmd/client` komut seviyesi flag/fatal hata metinleri i18n sistemine baglandi.
- Engine/client icindeki teknik runtime loglari bilerek Ingilizce operasyon logu olarak birakildi.
- Windows PowerShell 5.1 ve `cmd.exe` icinde gecici Turkce runtime config ile UTF-8 menu cikti testi gecti.
- PowerShell 7 icinde Turkce menu, Public Server Wizard ve TryCloudflare akisi UTF-8 bozulmasi olmadan calisti.
- Windows Terminal encoding testi ayri kaldi; `WT_SESSION` ile dogrulanacak.
- Locale QA testi Ingilizce kaliptaki her key'in Turkce dosyada da bulunmasini kontrol ediyor.
- Build ve release paketlerine `locales/` kopyalama eklendi.

## P0 - En Once

- Base config + profile override generator eklendi; bundan sonra yeni profil configleri `road-proxy generate-config --profile <ad>` ile uretilmeli.

## P1 - UDP ve Oyun Uyumlulugu

- UDP latency/jitter/loss metrik testleri genisletildi.
- Ag bozulmasi simule eden deterministik UDP testi eklendi: packet loss, reorder, duplicate, burst jitter.
- UDP over WebSocket kaynakli TCP head-of-line blocking riski icin tekrarlanabilir e2e olcum testi ve dokumani eklendi.
- `road-proxy udp-check server/client` eklendi; oyun motorundan bagimsiz coklu oyuncu UDP akisi, sequence, ACK, RTT, jitter, max gap, duplicate, reorder ve missing packet raporu alinabiliyor.
- `udp-check` su an tek dosyada buyudu; ileride `cmd/road/udpcheck/` altina `protocol`, `stats`, `server`, `client`, `report` olarak ayrilmasi planlandi.
- Tum isler toparlandiktan sonra WebSocket transport spike yap: `gorilla/websocket` ile `github.com/coder/websocket` ayni ROAD UDP/TCP benchmark senaryolarinda karsilastirilsin.
- WebSocket transport abstraction tasarla; once mevcut Gorilla adapter korunacak, sonra coder adapter sadece olcum icin eklenecek.
- Benchmark kabul kriteri belirle: paket/saniye, RTT, jitter, CPU, allocation ve 3+ oyuncu UDP stabilitesi.
- QUIC/WebTransport alternatifi icin teknik spike dokumani hazirlandi; ilk deney WebTransport degil, opt-in QUIC DATAGRAM olmali.
- UDP reply policy eklendi: `strict`, `same_ip`, `any`; bilinen UDP profilleri `same_ip`, field eksikse legacy `any`.
- UDP datagram truncation/MTU kontrolu eklendi: `buffer_size` ustu datagram drop edilir, client logu ve control API icinde max payload, >1200, >1400, >1472 sayaclari gorunur.
- UDP reconnect continuity riski test edildi: session yeniden acilinca hedefin gordugu ROAD UDP source IP:port degisebilir; kisa idle timeout risklidir.
- Peer broadcast yavas peer etkisi olculdu: mevcut broadcast senkron blokluyor; async/drop policy tasarimi dokumante edildi.
- LAN discovery/broadcast/multicast analizi yapildi: ROAD genel LAN emulasyonu yapmayacak, direct unicast ana model kalacak; oyun-ozel discovery adapter ileride degerlendirilecek.
- Paket icinde IP/port gomen oyunlar icin adapter hook tasarimi yazildi; runtime genel payload rewrite yapmayacak, protocol-aware adapter opt-in olacak.
- Ayni makinede birden fazla client config uretimi eklendi: `generate-config --client-instances N --client-start-port P`.
- `udp_peer_broadcast` kararini Plugin Studio tarafinda daha akilli hale getir.
- DDNet UDP profili eklendi: `ddnet-udp`, `configs/server-ddnet.json`, `configs/client-ddnet.json`; Lethal yerine temiz UDP oyun baseline'i olarak kullanılacak.
- Her calisan oyun icin acceptance doc eklendi: Minecraft Java, Minecraft Bedrock, GZDoom UDP, Lethal Company direct UDP, DDNet UDP.
- Lethal Company 3 kisi sonucu release engeli degil; profil community-validation olarak kalacak ve kullanici loglariyla olgunlasacak.

## P1 - Plugin Studio

- Plugin Studio artik sadece port bulan arac degil, compatibility database + teshis araci olarak evrilecek.
- Ilk hedef buyuk oyun listesi degil, dogru ve bakimi kolay compatibility database olacak.
- ROAD kapsami her profilde acik yazilacak: direct/LAN/local IP:port akisi; Steam/EOS lobby/relay destek garantisi yok.
- Profile semasina `road_scope`, `steam_lobby_supported`, `requires_game_launch_args` alanlari eklendi.
- Confidence sistemi eklendi: `platinum`, `gold`, `silver`, `bronze`, `unknown`.
- Confidence hesabi su an exe name, port/protocol ve profil eslesmesi ile yapiliyor; packet metadata sinyali sonraki adim.
- Steam AppID ve exe hash sadece ek guven sinyali olacak; tek basina "calisir" karari vermeyecek. Plugin Studio bu sinyalleri rapora ekliyor.
- Match reason listesi report'a yaziliyor: exe name, port, protocol, Steam AppID, exe SHA256, road scope, Steam lobby durumu, launch arg ihtiyaci.
- Studio report artik `port_selection` yazar: secilen network/port, secim nedeni ve reddedilen port adaylari.
- Bilinmeyen oyunlarda `unknown-game-report.json` uretiliyor: process, portlar, protocol, capture tarihi, ROAD version ve port secim raporu.
- Windows-only `v0` durumu kaldirildi; Plugin Studio artik Windows ve Linux runtime taramayi hedefliyor.
- Windows endpoint scanner artik once PowerShell NetTCPIP cmdletlerini kullanir: `Get-NetTCPConnection`, `Get-NetUDPEndpoint`; `netstat` fallback olarak kaldi.
- Windows process metadata PowerShell/CIM uzerinden okunuyor; `tasklist` fallback olarak kaldi.
- Linux process scanner eklendi: once `ss -H -tunap`, sonra `lsof -nP -iTCP -iUDP`, son fallback olarak `/proc/net/*` + `/proc/<pid>/fd` socket inode eslesmesi.
- Linux process metadata `/proc/<pid>/comm`, `/proc/<pid>/exe`, `/proc/<pid>/cmdline` uzerinden okunuyor.
- Compatibility profile listesi Go kodundan `compat-profiles/*.json` dosyalarina tasindi.
- `compat-profiles/_schema.json` olusturuldu.
- Ilk profiller olusturuldu: `gzdoom`, `lethal-company`, `minecraft-java`, `minecraft-bedrock`.
- Ilk DB hedefi mevcut 4 profil ile sinirli tutuldu. Dogrulanmamis 20-30 oyun ayni anda eklenmeyecek.
- Profil notlarindaki tekrarli satirlar temizlendi.
- Bilinen profillerde plugin draft notlari `compat-profiles/<id>.json` icin `notes_source` referansi olarak tutuluyor; asil kaynak profile dosyasi.
- Non-interactive CLI modu eklendi: `--pid` / `--process`, `--seconds`, `--network`, `--target-host`, `--target-port`, `--client-listen-port`, `--plugin-name`, `--udp-peer-broadcast`, `--force`.
- Ornek: `plugin-studio --process lethal --seconds 20 --plugin-name lethal-company-udp --force`
- Cok fazli capture eklendi: `lobby`, `connect`, `ingame`, `disconnect`.
- Non-interactive cok fazli ornek: `plugin-studio --process gzdoom --multi-phase --phase-seconds 8 --plugin-name gzdoom-udp --force`
- Multi-phase raporda port rolleri yaziliyor: `persistent`, `connect_only`, `game_only`, `lobby_only`, `disconnect_only`, `connect_and_game`, `multi_phase`.
- Launch script uretimi ekle: oyun icin dogru argumanlarla `.ps1` ve `.sh` taslagi.
- Studio raporunu daha okunur hale getir: secilen profil, reddedilen port adaylari, neden secildi. `port_selection` ve `multi_phase.port_roles` eklendi.
- Unknown game report JSON uretildi; exe hash ve Steam AppID bulunabiliyorsa rapora ekleniyor.
- Packet fingerprint ilk etapta payload okumadan metadata ile eklendi: socket snapshot kaynagindan flow direction, tick frequency, burst/streak ve TCP handshake tahmini yaziliyor.
- Packet size current scanner ile olculemedigi icin `packet_size_observed=false` ve `packet_size_source=unavailable_without_packet_capture` olarak rapora acik yaziliyor.
- Topology heuristic eklendi: `server_or_host`, `client_to_server`, `peer_to_peer_candidate`, `mixed_or_unclear`, `unknown`.
- Raw payload/network library byte fingerprint opsiyonel spike olarak ayrildi: `docs/PLUGIN-STUDIO-PAYLOAD-FINGERPRINT-SPIKE.md`.
- Capture replay sistemi taslagi yazildi: `docs/PLUGIN-STUDIO-CAPTURE-REPLAY-DRAFT.md`.

## P1 - Kod Yapisi ve Refactor Borcu

- `compatProfiles` listesini Go koduna gomulu veri olmaktan cikar; JSON tabanli compatibility database'e tasi. Tamamlandi.
- Compatibility note'lari plugin JSON, profile JSON ve Plugin Studio icinde iki kere tutulmayacak hale getir. Bilinen profiller icin `notes_source` referansi eklendi.
- `cmd/plugin-studio/main.go` parcalara ayrildi; main giris/flag secici olarak kaldi, interaktif akis, non-interactive akis, capture, fingerprint, topology, port analiz, report, scanner, signals ve input helperlari ayri dosyalara tasindi.
- Stats atomic yapisi, `sync.Once` session kapatma, buffer pool ve engine shutdown patternleri korunacak iyi kararlar olarak not edilmeli; refactor sirasinda bozulmamali.

## P1 - Gozlemleme

- Built-in RTT/ping olcumu eklendi: `/api/ping` ve `road-proxy ping`.
- ROAD client-server arasi gecikme, jitter ve packet loss gorulebilir: `/api/ping` RTT, `/api/stats` UDP jitter/loss.
- Per-plugin stats eklendi: `/api/stats.plugins` altinda session, byte, error ve UDP saglik sayaçlari.
- Control API aktif session listesini detaylandirdi: `/api/sessions`.
- Her session icin plugin, remote address, target address, tx/rx, age, idle suresi ve UDP saglik sayaçlari gorunur.
- Basit local dashboard eklendi: `/dashboard`.
- Dashboard hedefi tamamlandi: bagli clientlar, pluginler, portlar, trafik, hata, ping ve aktif session bilgisi.
- Dashboard Control Deck seviyesine genisletildi: overview, sessions, plugin catalog, UDP diagnostics, security ve API sekmeleri.
- Auth aktifken dashboard HTML kabugu yuklenebilir; API verileri icin tarayici icinde token giris paneli kullanilir.
- JSON log opsiyonu eklendi: server/client config icinde `logging.format: "json"`.
- Diagnostic bundle komutu eklendi: `road-proxy diagnostic-bundle --out diagnostics`, config/plugin/log/version ve netstat/ss snapshot tek zip icinde toplanir.
- UDP packet metadata recorder eklendi: `udp_record.enabled=true` ile client/server JSONL metadata capture alir, payload yazmaz.
- UDP replay/analysis taslagi eklendi: `docs/UDP-REPLAY-ANALYSIS-DRAFT.md`; mevcut recorder metadata-only oldugu icin byte-perfect replay iddiasi yok.
- UDP metrics merkezi toplanabilir hale geldi; client logu disinda control API stats ve recorder metadata birlikte kullanilabilir.

## P2 - Operasyon ve Kullanim

- Linux deploy otomasyonu eklendi: `deploy/linux/deploy-linux.ps1`; `-WhatIfOnly` ile SSH/SCP aksiyonlari dry-run gorulebilir.
- `scp`, `ssh`, `chmod`, config kopyalama ve servis baslatma tek script ile yapilabiliyor.
- `systemd` service template eklendi: `deploy/systemd/road-server.service`.
- Windows service destegi NSSM uzerinden dokumante edildi: `docs/WINDOWS-SERVICE-NSSM.md`.
- Firewall helper eklendi: `deploy/windows/open-firewall.ps1`, `deploy/linux/firewall-ufw.sh`.
- Windows/Linux port acma kontrolu ve izin verme islemi helper dosyalariyla baslatildi.
- Start/stop/status komutlarini tek `roadctl` mantiginda toparlama plani yazildi: `docs/ROADCTL-PLAN.md`.
- Paket icinden calisma yolu netlestirildi: relative `configs`, `plugins`, `locales`, `docs`, `compat-profiles`, `deploy` build ciktilarina kopyalaniyor.
- TLS/WSS karar notu netlestirildi: ROAD local TLS sunmayacak; TLS/WSS Nginx/Cloudflare gibi ters proxy katmanina birakilacak.
- Cloudflare, VPS ve LAN deployment presetleri yazildi: `docs/DEPLOYMENT-PRESETS.md`.
- Public deployment guvenlik dokumani eklendi: `docs/PUBLIC-DEPLOYMENT-SECURITY.md`.
- Cloudflare/local-only server preset eklendi: `configs/server-cloudflare-local.json`.
- WebSocket ve control API icin shared-token auth aktif hale getirildi.
- Public deployment icin `allowed_hosts`, `allowed_origins`, `max_connections`, `max_connections_per_ip`, `rate_limit_per_minute`, `plugin_api_public` alanlari eklendi.
- Docker destegi degerlendirildi: ilk stabil hedef icin default yol olmayacak, kosullar `docs/DOCKER-EVALUATION.md` icinde yazildi.
- VPS/LAN presetleri ile Docker karari ayri, kisa dokumanlarda tutuldu.
- macOS resmi hedef degil: proje Windows + Linux odakli kalacak; macOS destegi ancak test edebilen dis katki gelirse kabul edilecek.

## P2 - Config Teknik Borc

- `docs/CONFIG-PLUGIN-SYSTEM.md` icinde base config + profile override modeli tasarlandi.
- `configs/base/server.json`, `configs/base/client.json` ve `configs/profiles/*.json` uzerinden `road-proxy generate-config --profile <ad>` eklendi.
- `road-proxy validate --server ... --client ...` ve `road-proxy validate --all-configs` eklendi.
- `road-proxy validate-plugin plugins/<name>/plugin.json` eklendi.
- Plugin schema `compatibility.status`, `tested_players`, `known_ports`, `launch_args`, `notes`, `last_verified` alanlariyla genisletildi.
- Client config `server_ws_url` artik load sirasinda `ws`/`wss`, host ve URL parse acisindan validate ediliyor.
- Auth alanlari aktif: server `http.auth_token` / `http.auth_tokens`, client `auth_token`; `env:NAME` degerleri destekleniyor.
- Client config icindeki `read_timeout` ve `write_timeout` davranisi dokumante edildi; su an runtime deadline icin `ws_idle_timeout` ve `ws_ping_interval` esas aliniyor.
- `server_ws_url` elle duzenleme hatalarini azaltmak icin interaktif client menu duzeltme akisi eklendi.
- Server/client configlerinde ortak alanlari tekrar azalt.

## P2 - UTF-8 ve Coklu Dil Altyapisi

Mevcut durum:

- Windows konsol icin UTF-8 codepage ayari baslatiliyor.
- `configs/app.json` icinde `language` alani var.
- `cmd/road` icinde basit `t(tr, en)` fonksiyonu var.
- Ancak metinler kod icine gomulu, ana ceviri kalibi yok ve bazi Turkce metinlerde mojibake riski gorunuyor.

Karar:

- Ana kaynak dil Ingilizce olmali.
- Diger diller Ingilizce kalip dosyasindan cevrilmeli.
- Kullanici kendi dilini eklemek istediginde sadece bir locale dosyasi olusturabilmeli.
- UTF-8 proje standardi olmali; Windows, Linux, PowerShell ve markdown/json dosyalari ayni davranmali.

Yol haritasi:

1. Encoding envanteri cikar.
2. Tum `.go`, `.ps1`, `.sh`, `.json`, `.md` dosyalarinda UTF-8 standardini belirle.
3. `cmd/road/main.go` icindeki mojibake gorunen Turkce stringleri tespit et.
4. Windows konsol UTF-8 davranisini test et: `cmd.exe`, Windows PowerShell, PowerShell 7, Windows Terminal.
5. PowerShell scriptlerine gerekirse `$OutputEncoding` ve `[Console]::OutputEncoding` ayari ekle.
6. Inline `t(tr, en)` modelini gecici kabul et; yeni hedefi key-based i18n olarak belirle.
7. `locales/en.json` dosyasini ana kalip yap.
8. `locales/tr.json` dosyasini ilk ceviri dosyasi yap.
9. Ornek key formati belirle: `menu.choose_mode`, `server.starting`, `client.invalid_ws_url`.
10. Runtime i18n loader ekle: dil dosyasini oku, eksik key varsa Ingilizceye fallback yap.
11. Formatli string destegi ekle: `%s`, `%d` gibi argumanlar guvenli calismali.
12. Missing translation raporu ekle.
13. Ceviri kalibi uretici ekle: `road-proxy i18n export-template`.
14. Yeni dil ekleme dokumani yaz: `docs/I18N.md`.
15. Once `cmd/road` metinlerini i18n sistemine tasi. Tamamlandi.
16. Sonra `cmd/plugin-studio` metinlerini i18n sistemine tasi. Tamamlandi.
17. Sonra `cmd/server` ve `cmd/client` log ve hata metinlerini gozden gecir.
18. PowerShell menu ve helper scriptleri icin ya ayri locale sistemi kur ya da Go menu'yu primary ilan et.
19. Test ekle: dil fallback, eksik key, format argumanlari, UTF-8 cikti.
20. Release paketine `locales/en.json`, `locales/tr.json` ve ceviri dokumani dahil et.

Son hedef:

- Proje tamamlandiginda `locales/en.json` ana kalip olacak.
- Ceviri yapmak isteyen kisi `locales/en.json` dosyasini kopyalayip `locales/<dil>.json` olarak cevirecek.
- Kod icinde kullaniciya gorunen yeni metin eklenirse i18n export/test bunu yakalayacak.

## P2 - Test Eksikleri

- Paket build testleri ekle.
- Windows/Linux build scriptleri CI veya lokal smoke test ile dogrulansin.
- Tum `configs/*.json` dosyalarini yukleyen repo QA testi var; paket build testleri icinde de korunmali.
- Tum `plugins/*/plugin.json` dosyalarini schema seviyesinde test eden repo QA testi var; schema genisletildikce korunmali.
- Start script smoke testleri sistematik hale getir.
- GZDoom acceptance checklist'i netlestir.
- Lethal Company acceptance dokumani community-validation moduna cekildi; 3 kisi loglari geldikce guncellenecek.
- UDP peer broadcast icin pozitif ve negatif test senaryolari ayrilsin.

## P3 - Uzun Vadeli`r`n`r`n- Yardimci prototipler public release kapsami disinda tutulacak; ana ROAD paketi game/service proxy core, Plugin Studio ve dashboard ile sinirli kalacak.

- Mini compatibility database kur.
- Her oyun icin oyun adi, process adi, port, network, test durumu, oyuncu sayisi, launch argumani ve notlar tutulsun.
- Steam AppID entegrasyonu ek guven sinyali olarak degerlendirilsin, Steam lobby/relay tasima iddiasi olarak kullanilmasin.
- Network library fingerprint spike'i yap: ENet, RakNet/SLikeNet, Photon, LiteNetLib, Steam GNS imzalari; ancak varsayilan karar mekanizmasina kanit olmadan alinmasin.
- Plugin marketplace veya local registry mantigi ekle.
- Native GUI launcher opsiyonel uzun vadeli is; ilk resmi GUI yolu dependency-free embedded web dashboard.
- Kullanici oyun secer, ROAD config ve launch script otomatik hazirlanir.
- Cloudflare, VPS ve LAN deployment presetleri eklenir.
- Docker deployment presetleri eklenir.
- macOS/Darwin build resmi roadmap'e alinmayacak; dis katki gelirse ayri PR/test kaniti ile degerlendirilecek.
- Coder WebSocket gecisi ancak olcumde net kazanc gosterirse yapilacak; sirf daha yeni diye default dependency degistirilmeyecek.
- Otomatik update mekanizmasi degerlendirilir.

## Onerilen Uygulama Sirasi

1. Release paketleme, `.gitignore` ve root temizlik.
2. CI/CD pipeline, `CHANGELOG.md`, surumlama karari.
3. UTF-8 temizlik ve i18n kalip sistemi.
4. `cmd/road/main.go` refactor ve tekrar eden port/config helperlarini toparlama.
5. RTT/ping olcumu ve dashboard.
6. Plugin Studio profile JSON sistemi.
7. Linux deploy ve systemd scriptleri.
8. UDP loss/jitter/reorder testleri.
9. Lethal Company community loglari geldikce acceptance dokumanini guncellemek.

## Notlar

- ROAD su anda VPN degil; bu bilincli bir karar. Bu basitlik projenin pratik gucunu artiriyor.
- UDP uyumlulugunda ana risk, oyunun payload icine IP/port gommesi veya Steam/EOS gibi harici transport kullanmasi.
- Direct/LAN/local port kullanan oyun ve modlar proje icin en uygun hedef kitle.
