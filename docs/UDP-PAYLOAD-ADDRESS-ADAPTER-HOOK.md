# UDP Payload Address Adapter Hook

Tarih: 2026-05-11

Bu dokuman payload icine IP/port gomen oyunlar icin ROAD'da nasil bir adapter
hook tasarlanmasi gerektigini tanimlar.

## Problem

Bazi oyunlar UDP paketinin kaynak/hedef adresine ek olarak payload icine de
adres bilgisi yazar.

Ornekler:

- Peer listesi: `192.168.1.20:5029`
- Binary IPv4: `C0 A8 01 14`
- Binary port: little-endian veya big-endian `13 A5`
- Lobby/handshake icinde local endpoint veya public endpoint
- NAT punch veya peer rendezvous bilgisi

ROAD mevcut haliyle dis UDP/IP header seviyesini proxy eder, payload icindeki
adresleri degistirmez. Bu nedenle oyun payload icindeki eski LAN/local adresi
kullanirsa:

- Yanlis IP/port'a baglanmaya calisabilir.
- Peer'i offline sanabilir.
- Desync veya timeout yasayabilir.
- ROAD uzerinden gelen packet'i protocol seviyesinde reddedebilir.

## Mevcut Pipeline Neden Yetmez

Mevcut plugin pipeline adimlari genel byte transform icindir:

```json
{
  "client_pipeline": [{ "op": "xor", "key": "k" }],
  "server_pipeline": [{ "op": "base64_decode" }]
}
```

Adres rewrite icin bunlar yetmez.

Gereken ek bilgi:

- Paket yonu: client-to-server mi, server-to-client mi?
- Lokal client listen adresi
- Server target adresi
- ROAD public/client-facing adresi
- Gercek peer/player kimligi
- Oyun protokolundeki address field offset/format bilgisi
- Checksum/length/header guncelleme kurali

Bu yuzden adres rewrite, basit pipeline step degil protocol-aware adapter hook
olmalidir.

## Tasarim Karari

Kisa vadede runtime davranisi degistirilmez.

Gelecek tasarim:

- Adapter hook varsayilan kapali olur.
- Sadece bilinen ve test edilmis oyun profili icin acilir.
- Genel "payload icinde IP bul ve degistir" davranisi eklenmez.
- Adapter raw payload okumayi gerektirdigi icin gizlilik ve debug dokumaninda
  acik belirtilir.
- Adapter hata verirse paket default olarak drop edilmeli; bozuk payload
  forward edilmemelidir.

## Onerilen Schema Taslagi

Plugin runtime icinde adapter deklarasyonu:

```json
{
  "runtime": {
    "mode": "passthrough",
    "protocol_adapter": {
      "enabled": true,
      "id": "example-game-address-rewrite",
      "version": "0.1.0",
      "direction": ["client_to_server", "server_to_client"],
      "features": ["address_rewrite", "port_rewrite"],
      "on_error": "drop"
    }
  }
}
```

Adapter config ornegi:

```json
{
  "adapter_config": {
    "address_fields": [
      {
        "name": "peer_ipv4_ascii",
        "format": "ascii_ip_port",
        "scope": "handshake",
        "rewrite_from": "local_endpoint",
        "rewrite_to": "road_endpoint"
      },
      {
        "name": "server_port_le",
        "format": "uint16_le",
        "scope": "server_info",
        "rewrite_from": "target_port",
        "rewrite_to": "client_listen_port"
      }
    ]
  }
}
```

Bu schema bugun runtime tarafinda uygulanmamis bir taslaktir. Amac, ileride
plugin schema v2 veya experimental adapter sistemine temel olusturmaktir.

## Runtime Hook Noktalari

Adapter pipeline'dan sonra, socket write'tan once calismali.

Client tarafinda:

```text
local game -> ROAD client UDP socket
  -> client_pipeline
  -> protocol_adapter client_to_server
  -> WebSocket
```

Server tarafinda:

```text
WebSocket -> ROAD server
  -> server-side protocol_adapter client_to_server
  -> target UDP socket
```

Ters yon:

```text
target UDP socket -> ROAD server
  -> protocol_adapter server_to_client
  -> server_pipeline
  -> WebSocket
  -> ROAD client
  -> local game
```

Uygulamada adapter context su bilgileri almali:

```text
plugin_name
direction
transport_network
local_client_addr
road_client_listen_addr
server_target_addr
observed_source_addr
reply_policy
payload
```

## Adapter Interface Taslagi

Go seviyesinde taslak:

```go
type PacketDirection string

const (
    ClientToServer PacketDirection = "client_to_server"
    ServerToClient PacketDirection = "server_to_client"
)

type PacketContext struct {
    PluginName           string
    Direction            PacketDirection
    LocalClientAddr      net.Addr
    ClientListenAddr     net.Addr
    TargetAddr           net.Addr
    ObservedSourceAddr   net.Addr
    ReplyPolicy          string
}

type ProtocolAdapter interface {
    ID() string
    RewritePacket(ctx PacketContext, payload []byte) ([]byte, AdapterDecision, error)
}
```

Decision taslagi:

```go
type AdapterDecision string

const (
    AdapterForward AdapterDecision = "forward"
    AdapterDrop    AdapterDecision = "drop"
    AdapterNoop    AdapterDecision = "noop"
)
```

## Guvenlik ve Kararlilik Kurallari

- Adapter sadece explicit profil ile aktif olmali.
- Adapter payload'u mutate etmeden once kopyalamali.
- Paket boyutu degisirse length/checksum alanlari guncellenmeli veya drop
  edilmeli.
- Adapter panic ederse session degil paket drop edilmeli ve hata sayaci artmali.
- Adapter raw payload loglamamali; sadece metadata loglamali.
- Adapter default olarak `on_error=drop` davranmali.
- Generic regex/string replace yasaklanmali; false positive riski yuksek.

## Plugin Studio Ile Iliski

Plugin Studio su an payload okumaz. Bu dogru varsayilan olarak kalmali.

Ileride optional payload capture spike tamamlanirsa Studio su uyari sinyallerini
raporlayabilir:

- Payload icinde local IPv4 string gorundu.
- Payload icinde target port byte pattern'i gorundu.
- Handshake packet boyutu ve tekrar eden offset adres alanina benziyor.
- Oyunun packet library fingerprint'i adres rewrite ihtimali tasiyor.

Bu sinyaller "adapter gerekir" kararini otomatik vermemeli. Sadece report icinde
`address_rewrite_candidate=true` gibi isaretlenmelidir.

## Kabul Kriteri

Bir adapter gercek runtime'a alinmadan once:

- Oyun-ozel test fixture olmali.
- En az client-to-server ve server-to-client golden packet testleri olmali.
- Paket length/checksum degisimi test edilmeli.
- 2+ oyuncu e2e testi olmali.
- Adapter kapaliyken mevcut passthrough davranisi degismemeli.
- README ve compatibility profile acik sekilde "protocol-aware adapter" notu
  tasimali.

## ROAD Karari

Bu fazda adapter hook uygulanmadi; tasarim karari verildi.

Kisa vadeli karar:

- ROAD genel payload rewrite yapmayacak.
- Payload icine IP/port gomen oyunlar "adapter required" olarak isaretlenecek.
- Adapter hook pipeline'dan ayri, protocol-aware ve opt-in olacak.
- Plugin Studio payload fingerprint spike'i tamamlanmadan otomatik adapter
  olusturulmayacak.

Pratik sonuc:

- Oyun manuel direct IP:port kabul ediyor ve payload icinde peer adresi
  tasimiyorsa ROAD icin iyi adaydir.
- Oyun payload icinde peer adresi tasiyorsa adapter hook gerekir.
- Oyun Steam/EOS/GNS gibi harici transport kullaniyorsa ROAD port proxy
  kapsaminda degildir.
