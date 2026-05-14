# Plugin Studio Capture Replay Draft

Tarih: 2026-05-11

Bu dokuman Plugin Studio capture replay sisteminin ilk taslagidir. Amac, gercek oyunu tekrar acmadan onceki capture raporlarini ayni analiz kodundan gecirebilmektir.

## Hedef

- `studio-report.json` dosyasini tekrar analiz edebilmek.
- Port secimi, topology heuristic ve packet fingerprint sonucunu deterministik test edebilmek.
- Yeni heuristic eklendiginde eski capture raporlari uzerinden regresyon yakalamak.

## Kapsam

Replay sistemi ilk etapta packet payload replay yapmayacak.

Ilk kapsam:

- `endpoint_hits`
- `local_port_hits`
- `remote_port_hits`
- `top_local_ports`
- `top_remote_ports`
- `multi_phase.phases`
- `packet_fingerprint`
- `topology`

## Onerilen Komut

```powershell
plugin-studio replay --report plugins/gzdoom-udp/studio-report.json
```

Opsiyonel ilerleme:

```powershell
plugin-studio replay --report old.json --write-report replayed.json
plugin-studio replay --report old.json --expect-plugin gzdoom
plugin-studio replay --report old.json --expect-topology server_or_host
```

## JSON Replay Akisi

1. `studio-report.json` oku.
2. Eksik eski alanlar icin backward-compatible default uygula.
3. Port secimini yeniden hesapla.
4. Compatibility profile match sonucunu yeniden hesapla.
5. `packet_fingerprint` yoksa endpoint/port hitlerinden sinirli metadata uret.
6. `topology` sonucunu yeniden hesapla.
7. Eski rapor ile yeni sonuc arasindaki farklari yaz.

## Test Kullanimi

Replay sistemi ile acceptance dosyalarina gercek capture raporu eklenebilir.

Ornek:

- GZDoom 3 oyuncu `-netmode 1`
- Lethal Company direct UDP
- Minecraft Java TCP
- Minecraft Bedrock UDP

## Riskler

- Eski raporlar endpoint tick seviyesini tutmuyorsa burst/handshake yeniden birebir hesaplanamaz.
- Payload olmadigi icin replay, packet content buglarini yakalamaz.
- Gercek ag kosullari replay ile simule edilmez.

## Kabul Kriteri

- Replay ayni input icin ayni output uretmeli.
- Eski `studio-report.json` dosyalari bozulmadan okunmali.
- Replay sonucu testlerde kullanilabilir olmali.
- Replay hicbir zaman canli socket taramasi yapmamali.
