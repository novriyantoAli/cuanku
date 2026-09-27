# Dual-SSID: WPA2-Enterprise (BYOD) + PSK per-Customer (non-BYOD)

Setiap `AccessPoint` menyiarkan **dua SSID**:

1. **SSID Enterprise** — WPA2-Enterprise (802.1X, EAP-PEAP/MSCHAPv2) untuk perangkat BYOD yang diautentikasi lewat `RadiusAccount` (username+password): ponsel, tablet, laptop, Apple TV.
2. **SSID PSK per-Customer** — WPA2-PSK dengan **passphrase unik per `Customer`** untuk perangkat non-BYOD yang tidak mendukung 802.1X: Smart TV, streaming stick/box, konsol game, speaker pintar, IoT, printer.

Status: accepted

## Alasan

- Riset (`docs/research/0001-wpa2-enterprise-device-support.md`) mengonfirmasi mayoritas perangkat non-BYOD **tidak mendukung** WPA2-Enterprise/PEAP-MSCHAPv2 — Smart TV (Samsung/LG), streaming stick (Fire TV/Chromecast/Roku), konsol (PS/Xbox/Switch), speaker pintar (Echo/Nest), dan IoT umum hanya menyediakan antarmuka "masukkan password" (PSK).
- PSK adalah satu-satunya mekanisme autentikasi yang universal untuk kelas perangkat tersebut.
- Passphrase **per-Customer** (bukan satu PSK bersama) menjaga isolasi dan memungkinkan binding billing: sesi dari SSID PSK tetap diatribusikan ke `Customer` pemiliknya.
- MAC-auth (MAB) ditolak sebagai jalur utama — UX registrasi MAC menyulitkan konsumen dan MAC dapat di-spoof; dipertahankan hanya sebagai opsi perangkat khusus (post-MVP).

## Konsekuensi & prasyarat

- Kedua SSID memetakan ke **VLAN aktif `Customer` yang sama** (ADR-0001), sehingga lalu lintas BYOD dan non-BYOD mendarat di VLAN per-Customer yang sama dan di-bill ke `Customer` yang sama (billing usage-based per `Customer`).
- Passphrase PSK dibuat **server-side**, unik per `Customer`, disimpan aman (tidak cleartext), dan di-rotasi saat ganti kredensial/churn. `Customer` melihat passphrase di portal self-service.
- hostapd/OpenWrt harus mendukung **multi-SSID** (satu SSID WPA2-Enterprise + satu SSID WPA2-PSK per AP) — standar, tetapi image `AccessPoint` wajib menyertakannya.
- Sesi dari SSID PSK wajib membawa **identitas `Customer`** di RADIUS accounting (User-Name/Calling-Station-Id atau identifier per-Customer) agar `Balance`/`Tariff` berlaku benar — bukan dicatat sebagai sesi anonim.
- PSK dibagikan dalam satu rumah tangga — risiko dibatasi dengan rotasi saat churn/kebocoran; dapat diterima untuk segmen rumahan.
- **Rollout** *(diputuskan)*: ditunda — MVP hanya SSID Enterprise (BYOD); SSID PSK per-Customer disiapkan arsitekturnya (ADR ini) dan menjadi scope post-MVP.
