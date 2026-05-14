# UDP LAN Discovery, Broadcast, and Multicast

Tarih: 2026-05-11

Bu dokuman ROAD'un LAN discovery, UDP broadcast ve UDP multicast trafikleri
karsisindaki gercek kapsamini tanimlar.

## Kisa Sonuc

ROAD su anda VPN, TAP adapter veya sanal LAN degildir.

Desteklenen model:

- Oyun/mod belirli bir IP:port hedefine unicast TCP/UDP paket yollar.
- ROAD client bu lokal/direct IP:port akisina girer.
- ROAD server bu paketi tek bir configured target IP:port adresine yollar.

Desteklenmeyen model:

- Oyunun LAN server bulmak icin subnet broadcast yapmasi.
- Oyunun multicast grubuna discovery paketi atmasi.
- Oyunun mDNS/SSDP benzeri LAN servis kesfi kullanmasi.
- Oyunun payload icine kendi peer IP/port listesini gomup otomatik LAN mesh
  kurmasi.

Bu nedenle ROAD icin en guvenli kullanim manuel direct connect, localhost target
veya oyunun kendi server/client modudur.

## Trafik Tipleri

### Direct Unicast

Ornek:

```text
127.0.0.1:7777
192.168.1.50:25565
```

Durum: desteklenir.

Bu ROAD'un ana hedefidir. Plugin profili tek bir `target.address` degeri bilir
ve client da lokal bir `listen_addr` acar.

### Limited Broadcast

Ornek:

```text
255.255.255.255:7777
```

Durum: desteklenmez.

Sebep:

- ROAD client normalde oyunun broadcast soketi gibi davranmaz.
- ROAD server tarafinda tek bir `target.address` vardir; broadcast domain yoktur.
- Broadcast paketini WAN/WebSocket uzerinden oldugu gibi tasimak LAN semantigini
  korumaz.

### Directed Subnet Broadcast

Ornek:

```text
192.168.1.255:7777
```

Durum: desteklenmez.

Bu da LAN broadcast domain davranisi ister. ROAD su anda subnet topolojisi veya
virtual interface bilmez.

### Multicast

Ornek:

```text
224.0.0.0/4
239.x.x.x
ff00::/8
```

Durum: desteklenmez.

Sebep:

- ROAD multicast group join yapmaz.
- IGMP/MLD davranisi yoktur.
- Multicast TTL, group membership ve interface secimi tasinmaz.

### Service Discovery

Ornek protokol aileleri:

- mDNS
- SSDP
- oyun-ozel LAN server query paketleri

Durum: genel olarak desteklenmez.

Ancak oyun-ozel adapter ile bazi discovery akislari taklit edilebilir.

## Neden Genel Broadcast Relay Eklemiyoruz

Genel broadcast/multicast relay ilk bakista cazip gorunur ama ROAD'un basit ve
kontrollu port proxy modelini bozar.

Riskler:

- Paket amplification ve gereksiz trafik.
- LAN/WAN tarafinda loop olusturma.
- Bir oyunun discovery paketini baska oyuna veya network'e tasima.
- Firewall ve NAT davranisini belirsiz hale getirme.
- Payload icindeki IP/port bilgisi yine cozulmeden kalir.

Bu nedenle genel "broadcast'i yakala ve her yere bas" davranisi eklenmemeli.

## Kabul Edilebilir Gelecek Cozumler

### 1. Static Direct Connect

Mevcut ve onerilen cozum.

Oyun/mod sunucu adresini elle girebiliyorsa:

```text
127.0.0.1:<road-client-port>
```

veya LAN tarafinda:

```text
<road-client-ip>:<road-client-port>
```

Bu en az riskli yoldur.

### 2. Plugin-Level Discovery Adapter

Oyun-ozel plugin profili su bilgileri tasiyabilir:

```json
{
  "discovery": {
    "mode": "local_response",
    "listen_network": "udp",
    "listen_port": 7777,
    "response_name": "ROAD Server",
    "response_address": "127.0.0.1:7777"
  }
}
```

Mantik:

- ROAD genel broadcast relay yapmaz.
- Sadece bilinen oyunun bilinen discovery sorgusuna sahte/yerel server cevabi
  uretir.
- Kullanici oyunda LAN listesinde "ROAD Server" benzeri bir kayit gorebilir.

Bu bir protocol adapter isidir; genel core davranisi olmamali.

### 3. Targeted Discovery Proxy

Profil kontrollu sekilde belirli discovery query paketini unicast hedefe
cevirebilir.

Ornek:

```json
{
  "discovery": {
    "mode": "broadcast_query_to_unicast",
    "source_port": 19132,
    "target_address": "127.0.0.1:19132"
  }
}
```

Bu model bile payload formatini anlamayi gerektirebilir.

### 4. Multicast Group Adapter

Sadece kanitlanmis oyun icin:

```json
{
  "discovery": {
    "mode": "multicast_group",
    "groups": ["239.255.0.1:12345"]
  }
}
```

Bu yol platform farklari ve firewall davranisi nedeniyle daha risklidir.

### 5. Minimal TUN/TAP Mode

Gercek LAN discovery isteniyorsa en dogru teknik model sanal ag adaptoru veya
VPN benzeri bir katmandir.

Bu ROAD core'unun disinda, uzun vadeli deneysel konu olarak kalmali.

## Plugin Studio Icin Not

Plugin Studio ileride discovery sinyallerini raporlayabilir:

- Process UDP broadcast portu aciyor mu?
- Remote address `255.255.255.255` veya subnet broadcast mi?
- Multicast adres araligina paket var mi?
- Socket snapshot icinde sadece LAN discovery gorulup game traffic gorulmuyor mu?

Bu sinyaller plugin draft'ini otomatik "calisir" yapmamalidir. Sadece raporda
uyari olarak gosterilmelidir.

## ROAD Karari

Kisa vadeli karar:

- ROAD direct/LAN/local unicast IP:port proxy olarak kalir.
- Genel broadcast/multicast relay eklenmez.
- Bilinen oyunlar icin plugin-level discovery adapter tasarimi ileride
  degerlendirilebilir.
- README ve compatibility profilleri "Steam/EOS lobby gibi LAN discovery de
  otomatik tasinmaz" mesajini acik tutmalidir.

Pratik kabul kurali:

- Oyun manuel IP:port kabul ediyorsa ROAD icin iyi adaydir.
- Oyun sadece LAN listesi/broadcast discovery ile oyun buluyorsa ROAD icin
  adapter veya farkli network katmani gerekir.
