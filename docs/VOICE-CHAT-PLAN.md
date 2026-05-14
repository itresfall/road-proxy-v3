# ROAD Voice Chat Plan

## Karar

Voice chat ozelligi ROAD icinde bir plugin olarak degil, ayri bir program olarak gelistirilecek.

Server tarafi bu repo icinde yeni bir binary olacak:

```text
cmd/voice-server
internal/voice
configs/voice-server.json
```

Android tarafi Gradle/APK dosyalariyla bu repoyu kirletmemek icin repo disinda ayri klasorde tutulacak:

```text
C:\Projects\road-voice-android
```

## Neden Plugin Degil?

ROAD v3 plugin sistemi su an JSON tabanli proxy profili icin tasarlandi.

Plugin'in iyi yaptigi is:

```text
Minecraft gibi bir hedef servisi tanimlamak:
network = tcp/udp
target.address = 192.168.x.x:25565
pipeline = passthrough/xor/base64
```

Voice chat icin gerekenler farkli:

```text
Kullanici listesi
Oda durumu
Mikrofon mute/deafen state
Ses frame relay
Android istemci protokolu
WebSocket session yonetimi
Gecikme ve jitter toleransi
```

Bunlari plugin sistemine sokmak plugin modelini gereksiz karmasik hale getirir. Voice chat aslinda proxy profili degil, ayri bir realtime uygulamadir.

## Neden Ayri Program?

Ayri `voice-server` daha temiz ve guvenli:

```text
Minecraft proxy portlariyla karismaz.
8080/8081/25565/25567 kullanimina dokunmaz.
Kendi config'i olur.
Kendi log'u olur.
Kendi WebSocket endpoint'i olur.
Gerekirse kapatip acmak Minecraft proxy'yi etkilemez.
```

Baslangic portu:

```text
127.0.0.1:8090
```

Bu port sadece lokal dinler. Disaridan erisim Cloudflare Tunnel uzerinden gelir.

## Hedef Mimari

```text
Android APK
  -> wss://voice.example.com/ws
  -> Cloudflare Tunnel
  -> 127.0.0.1:8090
  -> ROAD voice-server
```

Minecraft proxy ayri kalir:

```text
road.example.com
  -> 127.0.0.1:8080
  -> ROAD Minecraft proxy
```

Voice ayri hostname kullanir:

```text
voice.example.com
  -> 127.0.0.1:8090
  -> ROAD voice-server
```

## Cloudflare Tunnel Config

Mevcut `%USERPROFILE%\.cloudflared\config.yml` icine yeni ingress eklenir:

```yaml
  - hostname: voice.example.com
    service: http://127.0.0.1:8090
```

Not: Bu satir `http_status:404` fallback satirindan once olmalidir.

## Voice Server Config

Ornek config:

```json
{
  "listen_addr": "127.0.0.1:8090",
  "ws_endpoint": "/ws",
  "room_name": "default",
  "max_clients": 8,
  "read_timeout": "30s",
  "write_timeout": "10s",
  "ping_interval": "20s",
  "client_idle_timeout": "60s"
}
```

## Android Uygulama Akisi

Ilk acilis:

```text
Adres:
voice.example.com

Isim:
PlayerOne
```

Baglandiktan sonra tek ekran:

```text
Sesli Kanala Gir / Cik
Mikrofon Ac / Kapat
Sagirlastir Ac / Kapat
Hoparlor / Kulaklik Sec
Odada olan kisiler
```

Ekstra ozellik zorunlu degil. Ilk hedef basit ve calisan sistemdir.

## Protokol Taslagi

Kontrol mesajlari JSON olur:

```json
{
  "type": "join",
  "name": "PlayerOne"
}
```

```json
{
  "type": "users",
  "users": [
    { "id": "abc", "name": "PlayerOne", "muted": false, "deafened": false }
  ]
}
```

```json
{
  "type": "state",
  "muted": true,
  "deafened": false
}
```

Ses frame'leri ilk surumde WebSocket binary message olarak gonderilir.

## Ses Codec Karari

MVP icin iki secenek var:

```text
1. PCM ile baslamak
2. Opus ile baslamak
```

Pragmatik karar:

```text
Ilk once calisan PCM relay yap.
Sonra gecikme ve internet kullanimi sorun olursa Opus'a gec.
```

PCM daha kolay debug edilir. Opus daha dogru uzun vadeli cozumdur ama Android tarafinda native/codec entegrasyonu isi buyutur.

## Gelistirme Asamalari

1. `cmd/voice-server` binary'sini ekle.
2. `internal/voice` icinde room, client, websocket relay kodunu yaz.
3. `configs/voice-server.json` ekle.
4. Lokal test icin basit WebSocket test client hazirla.
5. Cloudflare Tunnel config'e `voice.example.com` ekle.
6. Android APK projesini `C:\Projects\road-voice-android` altinda olustur.
7. Android'de adres + isim + baglan ekranini yap.
8. Mikrofon capture ve playback ekle.
9. Mute/deafen/hoparlor secimini ekle.
10. Iki telefonla gercek test yap.

## Net Sonuc

Bu is plugin olarak yapilmayacak.

Dogru yol:

```text
ROAD repo icinde ayri Go binary: voice-server
Repo disinda ayri Android proje: road-voice-android
Cloudflare uzerinde ayri hostname: voice.example.com
Ayri lokal port: 127.0.0.1:8090
```

## MVP Durumu

Ilk server ve Android iskeleti baslatildi.

ROAD repo icinde eklenenler:

```text
cmd/voice-server
internal/voice
configs/voice-server.json
scripts/build-windows.ps1
```

Android projesi:

```text
C:\Projects\road-voice-android
```

Debug APK cikti yolu:

```text
C:\Projects\road-voice-android\app\build\outputs\apk\debug\app-debug.apk
```

PC server baslatma:

```powershell
go run ./cmd/voice-server -config configs/voice-server.json
```

Windows build cikti yolundan baslatma:

```powershell
./build/windows/voice-server.exe -config configs/voice-server.json
```

Android APK build:

```powershell
cd C:\Projects\road-voice-android
./scripts/build-debug.ps1
```
