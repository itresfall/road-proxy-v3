# ROAD Proxy v3 Master Roadmap

Tarih: 2026-05-11

Bu dosya `PROJECT-TODO.md` icindeki daginik maddeleri is sirasina sokan ana yol haritasidir. Format bilincli olarak `{ } y isi yap` seklindedir.

## Platform Karari

{x} Windows resmi hedef platformdur.
{x} Linux resmi hedef platformdur.
{x} macOS resmi hedef platform degildir.
{x} macOS destegi sadece test edebilen bir dis katki sahibi build + runtime test kaniti ile PR gonderirse degerlendirilecek.
{x} macOS icin ekip icinde zaman ayrilmayacak, release engeli sayilmayacak.

## Faz 0 - Proje Temizligi ve Taban Kararlari

{x} Repo kokunde sadece kaynak ve temel dokumanlar kalacak sekilde cop dosyalari temizle.
{x} `scripts/` klasorunu sadece build scriptleriyle sinirli tut.
{x} `configs/` klasorunu sadece aktif varsayilanlar, test edilmis oyun configleri, schema ile sinirli tut.
{x} `.gitignore` dosyasini build, log, runtime state ve gecici dosyalari kapsayacak sekilde guncelle.
{x} Windows ve Linux build ciktilarinin `configs/` ve `plugins/` ile senkron kalmasini sagla.
{x} README icinde sade calistirma modelini koru: build scriptleri + dogrudan binary/go run.

## Faz 1 - Release Hazirligi

{x} Minimum Go surumunu belirle ve README'ye yaz.
{x} Versiyonlama stratejisini sec: SemVer, tarih bazli veya Git tag tabanli.
{x} `--version` destegini tum ana binarylere ekle.
{x} Build tarihi, surum ve commit/hash bilgisini binarylere gom.
{x} `CHANGELOG.md` ekle.
{x} Release notlarini `CHANGELOG.md` uzerinden yazma disiplinini belirle.
{x} `SECURITY.md`, `CONTRIBUTING.md` ve release checklist dosyalarini ekle.
{x} Windows release paketi uret: binaryler, `configs/`, `plugins/`, README, checksum.
{x} Linux amd64 release paketi uret: binaryler, `configs/`, `plugins/`, README, checksum.
{x} Linux arm64 paketini opsiyonel hedef olarak belirle.
{x} Paket isimlendirme standardini belirle: `road-proxy-v3_<version>_<os>_<arch>.zip` gibi.

## Faz 2 - CI/CD ve Otomatik Guvence

{x} GitHub Actions veya esdeger CI pipeline ekle.
{x} CI icinde `go test ./...` calistir.
{x} CI icinde Windows amd64 build smoke testi calistir.
{x} CI icinde Linux amd64 build smoke testi calistir.
{x} CI icinde opsiyonel Linux arm64 build smoke testi calistir.
{x} CI icinde JSON config parse testlerini calistir.
{x} CI icinde plugin schema validation testlerini calistir.
{x} Tag atilinca release paketi ureten pipeline tasarla.
{x} CI artifact veya release asset uretimini dokumante et.

## Faz 3 - Kod Yapisi Refactor

{x} `cmd/road/main.go` dosyasini parcalara ayir.
{x} `cmd/road/menu.go` dosyasina menu, input ve yes/no helperlarini tasi.
{x} `cmd/road/flow_server.go` dosyasina server flow kodunu tasi.
{x} `cmd/road/flow_client.go` dosyasina client flow kodunu tasi.
{x} `cmd/road/portcheck.go` dosyasina port owner bulma ve process kill kodunu tasi.
{x} `cmd/road/wsurl.go` dosyasina URL normalize ve control API fetch kodunu tasi.
{x} `ensureTCPPortFree` ve `ensureUDPPortFree` tekrarini tek `ensurePortFree(proto, port)` fonksiyonuna indir.
{x} `parsePortFromAddr` fonksiyonunu `splitListenAddr` uzerinden calistir.
{x} Windows `netstat` owner parse kodunu tek ortak parse fonksiyonuna indir.
{x} `cmd/server` ve `cmd/client` icindeki default config path kontrolunu `internal/app` helperina tasi.
{x} Refactor sonrasi `go test ./...` calistir.
{x} Refactor sonrasi Windows ve Linux buildleri calistir.

## Faz 4 - UTF-8 ve Coklu Dil Altyapisi

{x} Kod, JSON, Markdown ve script dosyalari icin UTF-8 standardini belirle.
{x} `cmd/road/main.go` icindeki mojibake gorunen stringleri temizle.
{x} Windows PowerShell 5.1 ve cmd.exe icin konsol encoding davranisini test et.
{x} PowerShell 7 icin konsol encoding davranisini test et.
{ } Windows Terminal icin konsol encoding davranisini test et.
{x} `internal/i18n` paketi tasarla.
{x} `locales/en.json` dosyasini ana ceviri kalibi yap.
{x} `locales/tr.json` dosyasini ilk ceviri dosyasi yap.
{x} Key naming standardini belirle: `menu.choose_mode`, `server.starting`, `client.invalid_ws_url` gibi.
{x} Eksik key durumunda Ingilizce fallback uygula.
{x} Formatli string destegini test et.
{x} Missing translation raporu ekle.
{x} `cmd/road` metinlerini key-based i18n sistemine tasi.
{x} `cmd/plugin-studio` metinlerini i18n sistemine tasi.
{x} `cmd/server` ve `cmd/client` log/hata metinlerini gozden gecir.
{x} `docs/I18N.md` dosyasini yaz.
{x} Release paketine locale dosyalarini dahil et.

## Faz 5 - Config ve Plugin Sistemi

{x} Config validate komutu tasarla: `road-proxy validate --server ... --client ...`.
{x} Plugin validate komutu tasarla: `road-proxy validate-plugin plugins/<name>/plugin.json`.
{x} Tum `configs/*.json` dosyalarini yukleyen test ekle.
{x} Tum `plugins/*/plugin.json` dosyalarini validate eden test ekle.
{x} Base config + profile override modelini tasarla.
{x} Server/client config tekrarlarini azalt.
{x} Plugin schema alanlarini genislet: compatibility status, tested players, known ports, launch args, notes, last verified.
{x} Auth legacy alanlarini ya tamamen kaldir ya da dokumanda legacy olarak isaretle.
{x} Client `read_timeout` ve `write_timeout` alanlarinin runtime davranisini netlestir.
{x} `server_ws_url` icin validator ekle.
{x} `server_ws_url` icin wizard/duzeltme akisini ekle.

## Faz 6 - Plugin Studio ve Compatibility Database

{x} ROAD kapsamini profile semasina acik yaz: direct/LAN/local IP:port akisi desteklenir, Steam/EOS lobby/relay garanti edilmez.
{x} Compatibility profile confidence seviyelerini tasarla: `platinum`, `gold`, `silver`, `bronze`, `unknown`.
{x} Match reason modelini tasarla: exe name, exe hash, Steam AppID, port, protocol, packet metadata, profil override.
{x} `compatProfiles` listesini Go kodundan JSON dosyalarina tasi.
{x} `compat-profiles/_schema.json` olustur.
{x} `compat-profiles/gzdoom.json` olustur.
{x} `compat-profiles/lethal-company.json` olustur.
{x} Minecraft Java ve Bedrock icin compatibility profile dosyalari olustur.
{x} Ilk DB hedefini 4 mevcut profil + az sayida guvenli direct/LAN aday ile sinirla; dogrulanmamis 20-30 oyun ekleme.
{x} Plugin Studio'ya JSON profile loader ekle.
{x} Plugin Studio match sonucuna confidence ve match reason alanlarini ekle.
{x} Profile notlarindaki tekrar eden satirlari temizle.
{x} Plugin JSON, compat profile ve Studio report icinde ayni notun uc yerde bakim gerektirmesini engelle.
{x} Profile semasina `road_scope`, `steam_lobby_supported`, `requires_game_launch_args` alanlarini ekle.
{x} Steam AppID tespitini sadece eslesme sinyali olarak tasarla; direct/LAN destek garantisi gibi sunma.
{x} Exe hash tespitini opsiyonel guven sinyali olarak ekle.
{x} Unknown game report JSON formatini tasarla ve Plugin Studio bilinmeyen oyunlarda rapor yazsin.
{x} Cok fazli capture akisini ekle: lobby, connect, ingame, disconnect.
{x} Multi-phase raporda connect-only, game-only ve persistent port ayrimini goster.
{x} Linux process scanner ekle: `ss`, `/proc`, `lsof` kaynaklari.
{x} Windows scanner icin `netstat` parsing yerine daha saglam CIM/native yontem arastir.
{x} Non-interactive CLI modu ekle.
{x} Studio report icinde secim gerekcesini yaz: secilen port, reddedilen portlar, profil eslesme nedeni.
{x} Packet fingerprint cikarma ekle: boyut, frekans, yon, burst pattern, handshake sureleri.
{x} Raw payload/network library byte fingerprint isini opsiyonel ve sonraki spike olarak ayir; ilk surumde metadata fingerprint yeterli.
{x} Server/client mi peer-to-peer mi tahmini icin heuristics ekle.
{x} Capture replay sistemi icin taslak hazirla.

## Faz 7 - UDP Saglamlik ve Oyun Uyumlulugu

{x} UDP latency/jitter/loss testlerini genislet.
{x} Packet loss simulasyonu ekle.
{x} Packet reorder simulasyonu ekle.
{x} Duplicate packet simulasyonu ekle.
{x} Burst jitter simulasyonu ekle.
{x} UDP over WebSocket kaynakli TCP head-of-line blocking etkisini olc.
{x} ROAD icine sentetik `udp-check server/client` komutu ekle; oyun motorundan bagimsiz loss, duplicate, reorder, RTT, jitter ve max gap olcsun.
{x} QUIC/WebTransport icin teknik spike dokumani hazirla.
{x} UDP reply policy ekle: `strict`, `same_ip`, `any`.
{x} UDP datagram truncation/MTU kontrolu ekle; client logu ve control API icinde max payload, >1200, >1400, >1472 sayaclarini goster.
{x} UDP reconnect continuity riskini test et.
{x} Peer broadcast yavas peer etkisini olc ve gerekirse async/drop policy tasarla.
{x} LAN discovery/broadcast/multicast ihtiyaclarini analiz et.
{x} Payload icine IP/port gomen oyunlar icin adapter hook tasarla.
{x} Ayni makinede birden fazla client instance uretimini otomatiklestir.
{x} Lethal Company 3 kisi sonucunu release engeli olmaktan cikar; community-validation profili olarak dokumante et.
{x} DDNet UDP profilini ekle ve Lethal yerine temiz UDP oyun baseline'i olarak dokumante et.
{x} Her calisan oyun icin acceptance dokumani ekle.

## Faz 8 - Gozlemleme ve Debug

{x} ROAD client-server arasi RTT/ping olcumu ekle.
{x} Jitter ve packet loss metriklerini control API'da goster.
{x} Per-plugin stats ekle.
{x} Per-session stats ekle.
{x} Aktif session listesini control API'ya ekle.
{x} Session icin plugin, remote address, tx/rx, age ve idle suresini goster.
{x} Basit local dashboard tasarla.
{x} Dashboard icinde clientlar, pluginler, portlar, trafik, hata ve ping goster.
{x} Dashboard'u sekmeli Control Deck haline getir: overview, sessions, plugins, UDP diagnostics, security ve API.
{x} Auth aktifken dashboard shell yuklensin, API verileri icin tarayici icinde token giris paneli kullanilsin.
{x} JSON log opsiyonu ekle.
{x} Diagnostic bundle komutu ekle: config, plugin, logs, version, netstat/ss snapshot.
{x} UDP recorder ekle: kisa sureli packet metadata kaydi.
{x} UDP replay/analysis araci icin taslak hazirla.

## Faz 9 - Operasyon ve Deployment

{x} Linux deploy scripti tasarla: ssh, scp, chmod, config kopyalama, servis baslatma.
{x} `road-server.service` systemd template ekle.
{x} Windows service/NSSM yardimci dokumani yaz.
{x} Firewall helper tasarla.
{x} Start/stop/status akisini `roadctl` komutu altinda toplama planini yaz.
{x} TLS/WSS karari yaz: ROAD local TLS sunmayacaksa bunu net belirt, TLS Nginx/Cloudflare katmanina birakilsin.
{x} Cloudflare, VPS ve LAN deployment preset dokumanlarini sade tut.
{x} Dockerfile eklemeyi degerlendir.
{x} docker-compose ornegi eklemeyi degerlendir.

## Faz 10 - Public Release Scope Freeze

{x} ROAD public release kapsamindan yardimci ses prototiplerini cikar.
{x} Ana paket hedefini oyun/local servis proxy runtime'i, plugin studio ve dashboard ile sinirli tut.

## Faz 11 - Uzun Vadeli Deneysel Konular

{ } Minimal TUN/TAP modunu ana core disinda ayri deneysel spike olarak tut.
{ } QUIC transport spike sonucuna gore devam/devam etmeme karari ver.
{ } Direct peer/rendezvous modunu arastir.
{ } Protocol adapter SDK taslagini hazirla.
{x} GUI launcher yerine once dependency-free embedded web dashboard kararini uygula.
{ } Native GUI launcher fikrini opsiyonel uzun vadeli kabuk olarak tut.
{ } Plugin marketplace/local registry fikrini arastir.
{ } Otomatik update mekanizmasini degerlendir.

## Faz 12 - Guvenlik ve Public Deployment Sertlestirme

{x} ROAD server icin Cloudflare/local-only preset ekle: data ve control portlari `127.0.0.1` uzerinden dinlesin.
{x} WebSocket data-plane icin shared token auth destegini geri getir.
{x} ROAD client tarafinda `auth_token` ve `auth_header` ile WebSocket header gonderimini uygula.
{x} Auth aktifken control API endpointlerini ayni token ile koru; dashboard shell sadece token giris kabugu olarak yuklenebilir kalsin.
{x} Cloudflare Access, WAF rule ve IP allowlist seceneklerini ayri guvenlik dokumaninda netlestir.
{x} Public deployment icin connection/rate limit tasarla ve config alanlarini ekle.
{x} WebSocket Origin/Host allowlist politikasini config alanlariyla ekle.
{x} `udp_peer_broadcast=true` profilleri icin ekstra runtime uyarisi ekle.
{x} Plugin download/info endpointleri icin public/private mod ayrimini ekle.
{x} Guvenlik kabul testi ekle: auth yokken local dev calisir, auth varken yanlis/eksik token reddedilir.
{ } `udp-check` komutunu alt paketlere ayir: protocol, stats, server, client, report.

## Faz 13 - Public Server Wizard ve Cloudflare Otomasyonu

{x} `road-proxy public-server` komutunu ekle.
{x} Ana menuye Public Server Wizard girisini ekle.
{x} `cloudflared` binary kontrolu ekle.
{x} Windows/Linux icin kullanici dizinine `cloudflared` auto-download MVP'si ekle.
{x} Auto-download icin SHA256/checksum dogrulamasi ekle.
{x} TryCloudflare quick tunnel modunu ekle.
{x} TryCloudflare URL yakalamada stdout ve stderr'i birlikte oku.
{x} TryCloudflare URL parse icin regex tabanli yakalama ekle.
{x} Public wizard icin guvenli ROAD config uret: local bind, token auth, rate/connection limit, private plugin API.
{x} Public endpoint, auth token, server config, client config ve local dashboard adresini ekranda goster.
{x} Public wizard calisirken client config hint dosyasi uret: `configs/.generated/client.public.menu.json`.
{x} Public wizard icin ayni anda ikinci instance'i engelleyen lock dosyasi ekle.
{x} Public wizard local data/control port override ekle ve cloudflared origin ayarini ayni kaynaktan uret.
{x} Ctrl+C ile ROAD engine ve `cloudflared` processlerini birlikte kapat.
{x} Mevcut Cloudflare tunnel token ile calistirma modunu ekle.
{x} `cloudflared tunnel login` tabanli domain modunu ekle.
{x} Domain modunda `cloudflared tunnel create` ve `cloudflared tunnel route dns` komutlarini wizard'a bagla.
{x} Domain modu icin ROAD'a ozel `cloudflared-road.yml` ingress config dosyasi uret.
{x} Domain modu ingress config icinde `/ws` data origin'e, `/dashboard` ve `/api/*` control origin'e route edilsin.
{x} TryCloudflare icin kullanicinin mevcut `~/.cloudflared/config.yml` dosyasindan etkilenmeyen izole cloudflared home/config stratejisi ekle.
{x} Domain modunda mevcut tunnel/DNS kaydi varsa daha iyi hata mesaji ve tekrar kullanma akisi ekle.
{x} Cloudflare dashboard/API token kullanmadan yapilabilen sinirin dokumanini netlestir.
{x} `cloudflared tunnel login` sonrasi gelen `cert.pem` ile tunnel create ve route dns otomasyonunun yeterli olduguna karar ver.
{x} Cloudflare API token istememe kararini dokumante et.
{x} Domain saglayici nameserver tasima isinin ROAD kapsami disinda kalacagina karar ver.
{x} Domaini Cloudflare nameserverlarina baglama rehberini yaz.
{ } Cloudflare API ile tam otomatik zone/domain secimini sadece gercek ihtiyac dogarsa tekrar degerlendir.

## Kabul Kurali

{x} Her faz sonunda `go test ./...` gecmeli.
{x} Her faz sonunda Windows build gecmeli.
{x} Linux ile ilgili fazlarda Linux amd64 build gecmeli.
{x} Dokuman referanslari silinen dosyalara isaret etmemeli.
{x} Windows ve Linux disinda platform destek iddiasi yapilmamali.


