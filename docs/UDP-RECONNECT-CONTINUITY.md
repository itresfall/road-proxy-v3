# UDP Reconnect Continuity

Tarih: 2026-05-11

Bu dokuman ROAD UDP session yeniden baglandiginda hedef oyun/server tarafinda
gorunen UDP source adresinin degisebilme riskini aciklar.

## Kisa Sonuc

- ROAD client tarafinda her lokal UDP source address icin bir WebSocket session
  acilir.
- ROAD server tarafinda her UDP WebSocket session icin hedef oyuna giden ayri
  bir UDP socket acilir.
- UDP session idle timeout, WebSocket kopmasi veya yazma hatasi sonrasi session
  kapanip yeniden acilirsa hedef oyunun gordugu ROAD source IP:port degisebilir.
- Bazi oyunlar peer kimligini UDP source IP:port ile eslestirirse bunu yeni peer,
  kopma veya desync olarak yorumlayabilir.

Bu davranis su an ROAD icin bilinen bir uyumluluk riskidir.

## Tekrarlanabilir Test

```powershell
go test ./internal/e2e -run TestUDPReconnectMayChangeTargetVisibleSourcePort -count=1 -v
```

Test akisi:

1. UDP hedef, ROAD server ve ROAD UDP client baslatilir.
2. Client `udp_session_idle_timeout` degeri kisa tutulur.
3. Ayni lokal UDP socket ile hedefe paket gonderilir.
4. Hedef, paketin kendisine hangi ROAD UDP source IP:port ile geldigini cevaplar.
5. Session idle ile kapanana kadar beklenir.
6. Ayni lokal UDP socket tekrar paket yollar.
7. Test, hedefin en az iki farkli ROAD source adresi gordugunu dogrular.

## Pratik Etki

Riskli oyun davranislari:

- Peer kimligini source IP:port ile tutmak.
- Handshake sonrasinda source port degisimini yeni client saymak.
- Eski source porttan gelmeyen paketi drop etmek.
- Lockstep/peer listesinde source endpoint degisimini desync saymak.

Daha guvenli oyun davranislari:

- Oyun icinde application-level player ID/session token kullanmak.
- Server/client modunda yeni UDP source portu kabul etmek.
- Kisa sureli reconnect'i yeniden handshake ile toparlamak.

## Onerilen Ayarlar

Gercek oyun profillerinde `udp_session_idle_timeout` cok kisa tutulmamali.

Ornek:

```json
{
  "udp_session_idle_timeout": "10m0s"
}
```

Kisa timeout sadece test veya cok kontrollu senaryolarda kullanilmali.

## Gelecek Cozum Fikirleri

- UDP target socket reuse/caching tasarimi.
- Plugin bazli reconnect continuity policy.
- Hedef oyuna giden UDP source portu mumkun oldugunca sabit tutan server-side
  session mapper.
- Application-level adapter hook ile oyun icindeki peer/session ID bilgisini
  kullanmak.

Bu isler runtime mimarisine dokunur; bu fazda yalnizca risk test edilip
dokumante edildi.
