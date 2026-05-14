# Same-Machine Multi-Client Instances

Tarih: 2026-05-12

Bu dokuman ayni makinede birden fazla ROAD client instance calistirmak icin
config uretme akisini tanimlar.

## Problem

Ayni bilgisayarda birden fazla oyun client'i acilacaksa her oyun client'i ayni
ROAD client listen portunu kullanamaz.

Ornek:

```text
127.0.0.1:5029
```

Bu portu tek process dinleyebilir. Ikinci ROAD client instance icin farkli bir
listen port gerekir.

## Generate Command

Tek client icin eski davranis ayni kalir:

```powershell
road-proxy generate-config --profile gzdoom
```

Coklu client icin:

```powershell
road-proxy generate-config --profile gzdoom --client-instances 2 --client-start-port 5031
```

Uretilen dosyalar:

```text
configs/.generated/server-gzdoom.json
configs/.generated/client-gzdoom-p1.json
configs/.generated/client-gzdoom-p2.json
```

Portlar:

```text
client-gzdoom-p1.json -> 127.0.0.1:5031
client-gzdoom-p2.json -> 127.0.0.1:5032
```

`--client-start-port` verilmezse profile/base client `listen_addr` portu ilk
port olarak kullanilir.

## Windows Calistirma

Ornek:

```powershell
.\build\windows\road-client.exe -config configs\.generated\client-gzdoom-p1.json
.\build\windows\road-client.exe -config configs\.generated\client-gzdoom-p2.json
```

Ayrı pencerelerde baslatmak icin:

```powershell
Start-Process .\build\windows\road-client.exe -ArgumentList "-config configs\.generated\client-gzdoom-p1.json"
Start-Process .\build\windows\road-client.exe -ArgumentList "-config configs\.generated\client-gzdoom-p2.json"
```

## Linux Calistirma

```bash
./build/linux/amd64/road-client -config configs/.generated/client-gzdoom-p1.json
./build/linux/amd64/road-client -config configs/.generated/client-gzdoom-p2.json
```

## Oyun Tarafi

Her oyun client'i kendi ROAD client listen portuna baglanmali.

Ornek:

```text
Player 2 -> 127.0.0.1:5031
Player 3 -> 127.0.0.1:5032
```

Oyun sadece host/IP kabul edip port kabul etmiyorsa ayni makinede coklu client
testi o oyun icin uygun olmayabilir veya oyun-ozel launch argumani gerekir.

## GZDoom Notu

GZDoom icin 3+ oyuncu testlerinde ana kural degismez:

```text
-netmode 1
```

`udp_peer_broadcast` bu senaryo icin cozum degildir. Her local ROAD client
instance farkli listen port kullanmalidir.

## Sinirlar

- Bu komut sadece config uretir; process supervisor degildir.
- Portlarin bos oldugunu runtime baslarken isletim sistemi dogrular.
- Birden fazla oyun instance'i ayni save/config/log dosyalarini kullaniyorsa
  oyun tarafinda ek ayar gerekebilir.
- ROAD ayni makinede coklu client'i kolaylastirir ama oyun motorunun coklu
  instance desteklemesini garanti etmez.
