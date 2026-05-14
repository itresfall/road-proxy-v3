# QUIC / WebTransport Spike

Tarih: 2026-05-11

Bu dokuman ROAD icin WebSocket/TCP yerine QUIC veya WebTransport tabanli
alternatif transport eklemenin teknik degerini ve riskini ozetler.

## Kaynaklar

- QUIC transport standardi: https://www.ietf.org/rfc/rfc9000.html
- QUIC DATAGRAM extension: https://www.ietf.org/rfc/rfc9221.html
- HTTP Datagrams ve Capsule Protocol: https://www.ietf.org/rfc/rfc9297.html
- quic-go datagram dokumani: https://quic-go.net/docs/quic/datagrams/
- quic-go WebTransport dokumani: https://quic-go.net/docs/webtransport/
- Go WebTransport paketi: https://pkg.go.dev/github.com/quic-go/webtransport-go

## Problem

Mevcut ROAD data plane WebSocket uzerinden calisir. WebSocket pratik ve
dagitimi kolaydir, ancak TCP byte stream uzerinde oldugu icin UDP datagramlari
gercekte sirali ve guvenilir bir akistan gecmis olur.

Sonuc:

- UDP paket kaybi dogal sekilde oyuna yansimaz; TCP yeniden iletim bekletir.
- Ayni UDP session icinde buyuk veya burst cevaplar kucuk cevaplari geciktirir.
- Wi-Fi, mobil ag veya uzak VPS gibi kayipli hatlarda jitter artabilir.

## QUIC Ne Saglar

QUIC UDP uzerinde calisan, TLS 1.3 tabanli, stream multiplexing destekleyen bir
transporttur. ROAD icin iki onemli taraf var:

- QUIC streams: TCP benzeri guvenilir akislar icin kullanilabilir.
- QUIC DATAGRAM: UDP benzeri unreliable datagram tasima icin kullanilabilir.

QUIC DATAGRAM, ROAD UDP profilleri icin WebSocket'e gore daha dogru eslestirme
olabilir; kayipli datagram kaybolabilir, sonraki datagramlar TCP retransmission
beklemeden ilerleyebilir.

## WebTransport Ne Saglar

WebTransport, HTTP/3 uzerinden stream ve datagram modelini uygulamaya acar.
Tarayici uyumlulugu gerektiren uygulamalarda anlamlidir.

ROAD icin durum:

- ROAD su an native Go binaryleriyle calisiyor, browser client hedefi yok.
- WebTransport ekstra HTTP/3/WebTransport semantigi getirir.
- quic-go dokumani WebTransport tarafinda draft/final RFC gecislerinde uyumluluk
  kirilmasi riski olabilecegini belirtiyor.

Bu nedenle ROAD icin ilk deney WebTransport degil, dogrudan QUIC DATAGRAM
olmalidir.

## Onerilen Karar

Kisa vadede varsayilan transport degismemeli.

Mevcut `websocket` transport:

- Daha kolay dagitilir.
- HTTP reverse proxy ve Cloudflare benzeri katmanlara daha uyumludur.
- Windows/Linux kullanicisi icin daha az firewall sorunu cikarir.
- TCP ve basit UDP oyunlarda zaten pratik olarak calisiyor.

QUIC sadece deneysel alternatif olarak eklenmeli:

```text
transport: websocket        # default
transport: quic-datagram    # experimental
```

## Teknik Taslak

Yeni soyutlama:

```text
internal/transport/
  websocket/
  quic/
```

Hedef interface:

```text
Dial(ctx, pluginName) Session
Accept(ctx) Session
Session.SendDatagram([]byte)
Session.ReceiveDatagram() []byte
Session.OpenStream()
Session.AcceptStream()
```

UDP profilleri:

- QUIC DATAGRAM ile tasinir.
- Datagram boyutu MTU ve QUIC max datagram parametresine gore sinirlanir.
- Paket buyukse drop/truncate politikasi acik tanimlanir.

TCP profilleri:

- QUIC stream ile tasinir.
- Her lokal TCP connection icin ayri QUIC stream acilir.
- WebSocket davranisina yakin kalir.

## Riskler

- UDP/443 veya custom UDP portu firewall tarafindan engellenebilir.
- HTTP reverse proxy uyumlulugu WebSocket kadar kolay degildir.
- TLS/certificate hikayesi netlesmeden kullanici deneyimi kotu olur.
- Datagram MTU siniri daha gorunur hale gelir; 1200 byte guvenli taban olarak
  ele alinmali, daha buyuk datagramlar PMTU/DPLPMTUD kararina baglanmali.
- QUIC congestion control tum datagramlari etkiler; "UDP gibi tamamen sinirsiz"
  davranmaz.
- Yeni transport, debug ve deployment dokuman maliyetini artirir.

## Kabul Kriteri

QUIC deneyi ancak su kosullarda anlamli sayilmali:

- GZDoom/Lethal/LAN UDP testlerinde WebSocket ile ayni dogruluk.
- Packet loss/jitter testinde UDP p95 latency veya max gap tarafinda olculebilir
  iyilesme.
- TCP profillerinde performans veya stabilite regresyonu yok.
- Windows ve Linux buildleri geciyor.
- Firewall/deployment dokumani yazilmis.
- Default hala WebSocket; QUIC opt-in.

## Uygulama Sirasi

1. `internal/transport` interface taslagini ekle.
2. Mevcut WebSocket kodunu davranis degistirmeden adapter altina al.
3. Sadece UDP icin minimal `quic-datagram` prototype yaz.
4. `TestUDPWebSocketHOLMeasurement` benzeri QUIC karsilastirma testi ekle.
5. Packet loss/jitter testlerini WebSocket ve QUIC icin ayni senaryoda calistir.
6. Sonuclar net iyilesme gostermiyorsa QUIC'i release hedefi yapma.

## Sonuc

QUIC/WebTransport ROAD icin mantikli ama hemen default yapilacak bir sey degil.
Dogru yol:

- WebSocket'i koru.
- UDP reply policy, MTU kontrolu ve mevcut WebSocket gozlemlemesini once bitir.
- Sonra QUIC DATAGRAM'i izole ve opt-in bir deney olarak ekle.
- WebTransport'u ancak browser client veya HTTP/3 ekosistemi gercek ihtiyac
  haline gelirse tekrar degerlendir.
