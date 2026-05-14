# UDP over WebSocket HOL Measurement

Tarih: 2026-05-11

Bu dokuman ROAD icinde UDP paketlerinin WebSocket/TCP uzerinden tasinmasi
nedeniyle olusabilecek head-of-line blocking etkisini nasil olctugumuzu
aciklar.

## Kisa Sonuc

- ROAD UDP tarafinda her yerel UDP kaynak adresi ayri bir WebSocket baglantisi
  acar.
- Bu nedenle farkli UDP oyunculari veya farkli TCP istemcileri normalde ayni
  WebSocket byte stream'ini paylasmaz.
- Ancak tek bir UDP session icinde tum UDP datagramlari WebSocket mesajlari
  olarak ayni TCP akisi uzerinden sira ile tasinir.
- Ayni session icinde buyuk veya burst halinde gelen UDP cevaplari, arkasindan
  gelen kucuk latency-sensitive cevabi geciktirebilir.
- Bu ROAD bug'i degil; UDP'yi TCP tabanli WebSocket uzerinden tasimanin dogal
  sonucudur.

## Tekrarlanabilir Test

```powershell
go test ./internal/e2e -run TestUDPWebSocketHOLMeasurement -count=1 -v
```

Test akisi:

1. UDP hedefi, ROAD server ve ROAD UDP client baslatilir.
2. Ayni yerel UDP socket ile warm-up ping yapilir.
3. Kucuk ping/pong paketleriyle baseline RTT ornekleri alinir.
4. Ayni UDP session icinde hedef once 16 adet 8192 byte bulk UDP cevabi yollar.
5. Bulk cevaplarin hemen arkasindan kucuk `hol-ack` cevabi yollar.
6. Test, `hol-ack` paketinin bulk kuyrugunun arkasinda ne kadar geciktigini
   raporlar.

Bu makinedeki ornek cikti:

```text
udp websocket hol measurement baseline=count=12 min=0s p50=0s p95=0s max=0s hol_burst=count=6 min=16.014ms p50=16.014ms p95=16.015ms max=16.015ms bulk=16packets/8192bytes
```

Not: localhost baseline degeri Windows zamanlayici cozunurlugu nedeniyle `0s`
gorunebilir. Onemli olan testin eksiksiz bulk + ack alabilmesi ve `hol_burst`
degerinin kaydedilmesidir.

## Bu Test Neyi Olcer

- Ayni UDP session icinde WebSocket/TCP siralamasinin kucuk cevabi buyuk/burst
  cevaplarin arkasinda bekletebildigini olcer.
- Localhost uzerinde deterministik backlog etkisini olcer.
- CI ve lokal gelistirme icin hizli, tekrar edilebilir bir smoke/measurement
  testi saglar.

## Bu Test Neyi Olcmez

- Gercek WAN packet loss etkisini olcmez.
- TCP retransmission kaynakli gercek internet HOL etkisini olcmez.
- Router, Wi-Fi, ISP, Cloudflare, VPS veya NAT davranisini modellemez.
- Farkli WebSocket connection'lari arasindaki OS/network congestion etkisini
  ayri ayri izole etmez.

Gercek internet HOL etkisi icin ayrica kontrollu packet loss/jitter ortami
gerekir. Bu is QUIC/WebTransport spike veya harici ag emulasyonu ile ele
alinmalidir.

## Pratik Sonuc

- ROAD ile UDP tasirken kucuk ve sik paket kullanan oyunlar daha iyi adaydir.
- Tek session icinde buyuk datagram burst'u atan oyunlarda latency spike
  gorulebilir.
- `udp_peer_broadcast` acilirsa yavas peer veya buyuk burst etkisi daha fazla
  hissedilebilir; bu yuzden varsayilan kapali kalmalidir.
- QUIC/WebTransport arastirmasi bu riskin uzun vadeli alternatifidir.
