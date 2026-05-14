# UDP Peer Broadcast Slow Peer Risk

Tarih: 2026-05-11

Bu dokuman `runtime.udp_peer_broadcast=true` kullanildiginda yavas peer etkisini
ve ileride uygulanabilecek async/drop policy tasarimini aciklar.

## Kisa Sonuc

- Mevcut `udpPeerHub.broadcast` senkron calisir.
- Bir client-origin UDP paketi hedef oyuna yazildiktan sonra ayni payload diger
  aktif peer WebSocket session'larina sirayla yazilir.
- Peerlerden birinin WebSocket write islemi yavaslarsa broadcast fonksiyonu o
  sure kadar bekler.
- Bu, client-origin packet read loop'unu da geciktirebilir ve diger peerlerin
  paket teslim zamanini etkileyebilir.

Bu nedenle `udp_peer_broadcast` varsayilan olarak kapali kalmalidir.

## Tekrarlanabilir Test

```powershell
go test ./internal/engine -run TestUDPPeerBroadcastSlowPeerBlocksCurrentBroadcast -count=1 -v
```

Test akisi:

1. Bir sender session olusturulur.
2. Bir slow peer `send` fonksiyonunda bilincli bekletilir.
3. Bir fast peer normal cevap verir.
4. Broadcast cagrisi olculur.
5. Test, broadcast suresinin slow peer delay'inden kisa olmadigini dogrular.

Bu test mevcut senkron davranisi olcer. Ileride async/drop policy uygulanirsa
bu test yeni davranisa gore guncellenmelidir.

## Pratik Etki

Riskli durumlar:

- Bir oyuncunun baglantisi yavas veya buffer'i dolu.
- Peer broadcast acik ve client-origin paket orani yuksek.
- Payload boyutu buyuk veya burst seklinde geliyor.
- Oyun lockstep/input packet zamanlamasina hassas.

Etkiler:

- Diger peerler gereksiz bekleyebilir.
- WebSocket write deadline'a kadar bloklama yasanabilir.
- UDP oyunda jitter/desync gorulebilir.

## Tasarim Karari

Kisa vadede mevcut runtime davranisi degistirilmedi.

Sebep:

- `udp_peer_broadcast` zaten opt-in ve varsayilan kapali.
- GZDoom icin dogru cozum peer broadcast degil, `-netmode 1`.
- Async/drop policy runtime semantigini degistirir ve dikkatli test ister.

Yine de olcum sonucu async/drop policy ihtiyacini dogruluyor.

## Onerilen Async/Drop Policy

Plugin runtime icin ileride su alanlar eklenebilir:

```json
{
  "runtime": {
    "udp_peer_broadcast": true,
    "udp_peer_broadcast_policy": "async_drop_oldest",
    "udp_peer_broadcast_queue": 64,
    "udp_peer_broadcast_write_timeout": "25ms"
  }
}
```

Policy adaylari:

- `sync`: mevcut davranis; en basit ve deterministik ama yavas peer herkesi
  etkileyebilir.
- `async_drop_oldest`: her peer icin kuyruk kullanir; kuyruk dolunca eski paket
  atilir.
- `async_drop_newest`: kuyruk dolunca yeni paket atilir; eski state korunur.
- `async_disconnect_slow`: kuyruk veya write timeout surekli asiliyorsa slow
  peer kapatilir.

Oyunlara gore pratik tercih:

- State snapshot oyunlari icin `async_drop_oldest` daha mantiklidir; en yeni
  state daha degerlidir.
- Input/lockstep oyunlari icin drop tehlikelidir; `sync` veya oyunun kendi
  packet-server modu tercih edilmeli.
- Bilinmeyen oyunlarda peer broadcast kapali kalmalidir.

## Uygulama Taslagi

1. `udpPeerSession` icine bounded channel ekle.
2. Her peer icin tek writer goroutine baslat.
3. Broadcast, peer channel'ina non-blocking enqueue yapsin.
4. Kuyruk dolunca secilen drop policy uygulansin.
5. Per-peer dropped packet sayaci ve queue length metriği eklensin.
6. Slow peer write timeout asarsa policy'ye gore logla veya session kapat.
7. E2E testlerde yavas peer varken fast peer teslim suresi olculsun.

## Kabul Kriteri

- Varsayilan davranis geriye donuk olarak `sync` veya mevcut davranisa denk
  kalmali.
- `udp_peer_broadcast=false` profilleri hic etkilenmemeli.
- Async policy acikken slow peer fast peer teslimini bloklamamali.
- Drop sayilari log veya metrics tarafinda gorunmeli.
- GZDoom `-netmode 1` profili peer broadcast kapali kalmali.
