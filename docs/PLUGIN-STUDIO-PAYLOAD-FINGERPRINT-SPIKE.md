# Plugin Studio Payload Fingerprint Spike

Tarih: 2026-05-11

Bu dokuman raw payload veya network library byte fingerprint isinin neden ana Plugin Studio MVP disinda tutuldugunu ve ileride nasil ele alinacagini tanimlar.

## Karar

- Ilk surumde payload okunmayacak.
- Ilk surumde network library byte signature karar mekanizmasina girmeyecek.
- Mevcut `packet_fingerprint` sadece socket snapshot metadata kullanacak.
- Packet size, payload byte pattern, magic byte, protocol header ve library signature icin ayri spike gerekir.

## Neden Simdi Degil

- Windows tarafinda WFP, Npcap veya ETW gerektirebilir.
- Linux tarafinda AF_PACKET, tcpdump/pcap veya eBPF gerektirebilir.
- Admin/root izni ihtiyaci artar.
- Gizlilik ve veri toplama hassasiyeti artar.
- False positive riski yuksektir: ayni engine veya library farkli oyunlarda farkli transport davranisi gosterebilir.
- ROAD icin asil karar genelde IP:port akisi, peer modeli ve direct/LAN uygunlugu uzerinden verilir.

## Spike Hedefleri

1. Packet metadata yakalama icin platform seceneklerini karsilastir.
2. Windows icin WFP, ETW ve Npcap maliyetini yaz.
3. Linux icin AF_PACKET, tcpdump/pcap ve eBPF maliyetini yaz.
4. Sadece metadata okuma ile payload okuma arasindaki guvenlik ve izin farkini yaz.
5. Payload icinden IP/port gomulu mu anlamaya yarayacak minimal patternleri tasarla.
6. Network library signature adaylarini ayri tut: ENet, RakNet/SLikeNet, LiteNetLib, Photon, Steam GNS.
7. Signature eslesmesini hicbir zaman "calisir" garantisi olarak sunma; sadece ek sinyal yap.
8. Address rewrite ihtiyaci cikarsa adapter hook tasarimini `docs/UDP-PAYLOAD-ADDRESS-ADAPTER-HOOK.md` ile iliskilendir.

## Kabul Kriteri

- Spike dokumani hangi platformda hangi yontemin kullanilacagini net soylemeli.
- Admin/root gereksinimi acik yazilmali.
- Kullaniciya payload toplanacaksa bunun acik ve kapali varsayilanli olacagi belirtilmeli.
- Signature sonucu sadece confidence reason olabilmeli, plugin hedefini tek basina degistirmemeli.

## Mevcut Durum

Plugin Studio su anda `packet_fingerprint` icinde sunlari yazar:

- flow direction
- tick frequency
- burst/streak
- TCP state dagilimi
- TCP handshake tick tahmini
- packet size unavailable notu

Bu, payload okumadan alinabilecek guvenli ilk katmandir.
