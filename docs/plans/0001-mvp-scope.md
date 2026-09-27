# Lingkup MVP — Cuanku (WPA2-Enterprise WISP)

Dokumen tujuan, batas MVP, dan kriteria sukses. Melengkapi `CONTEXT.md` (glossary) dan `docs/adr/` (keputusan).

## Tujuan

- **T1** — Customer dapat mengubah username & password WiFi (`RadiusAccount`) sendiri tanpa bantuan admin.
- **T2** — Customer dapat membayar online (top-up); layanan aktif/suspend otomatis mengikuti `Balance` (dengan *grace access* saat pembayaran diterima).
- **T3** — Admin dapat memantau status & metrik `AccessPoint` Customer.
- **T4** — Admin dapat mengelola `Customer`, `Package`, `AccessPoint`, dan melihat rekap pendapatan.

## Aktor

- **Administrator** — staf internal; autentikasi username+password (session cookie, bcrypt).
- **Customer** — pembayar layanan internet; autentikasi OTP email (sender OTP diabstraksikan).

## Batas MVP — masuk

- CRUD `Customer`, `Package`, `AccessPoint`.
- Provisi `RadiusAccount` (admin membuat `RadiusAccount` awal saat onboarding `Customer`).
- Self-service: ubah username & password `RadiusAccount` (tulis radcheck + trigger re-auth).
- Portal self-service `Customer` (satu permukaan: ganti kredensial + top-up proaktif + lihat `Balance`).
- `CaptivePortal` di VLAN suspended (pemberitahuan masa aktif habis + CTA top-up, TLS, ikat identitas via token server-side).
- Top-up online (gateway diabstraksikan; grace access saat notifikasi sukses, kredit `Balance` saat settlement — ADR-0003).
- Suspend/aktif via VLAN dinamis (ADR-0001).
- Monitoring `AccessPoint` via SNMP v2c @5 menit (adapter — ADR-0002); status online/offline/unreachable.
- Accounting minimal (start/stop/interim) yang mencatat VLAN/`SubscriptionState` per sesi.
- Rekap pendapatan (kas diterima = settled `Payment`), notifikasi email (konfirmasi top-up & alert `Balance`), alert `AccessPoint` offline (kecuali unreachable).

## Batas MVP — ditunda

Self-registration Customer · peran teknisi · export CSV · accounting lengkap/report data usage · OTP WhatsApp/SMS · payment gateway konkret · SSID PSK per-Customer (fallback non-BYOD — ADR-0004).

## Kriteria sukses

- **T1** — Customer mengganti kredensial sendiri ≤ 2 menit; kredensial baru berlaku ≤ 1 interval re-auth (Disconnect untuk AccessPoint terjangkau; ≤ 30 menit untuk AccessPoint di balik NAT).
- **T2** — Top-up notifikasi sukses → grace aktif ≤ 1 interval re-auth; settlement → `Balance` dikredit final; reversal/chargeback → `Balance` didebit & suspend ulang.
- **T3** — Admin melihat status `AccessPoint` (online/offline/unreachable) terbaru ≤ 5 menit untuk AP terjangkau SNMP; metrik inti (uptime, CPU/memori, signal/noise per radio, jumlah client, traffic RX/TX) tercatat konsisten dengan interval polling.
- **T4** — Rekap pendapatan rekonsiliabel terhadap `Payment` settled (kas diterima); kelola `Customer`/`Package`/`AccessPoint` selesai tanpa SQL/CLI (app-side; konfigurasi VLAN switch/gateway = langkah manual terdokumentasi).
- *(Target kuantitatif — jumlah Customer aktif, SLA — diisi kemudian.)*

## Persyaratan keamanan (dari review ce-doc-review)

- **Webhook payment**: wajib verifikasi signature + idempotensi; webhook settlement/reversal direkonsiliasi (retry bila hilang).
- **Auth/authz**: setiap endpoint menyatakan aktornya; `Customer` hanya dapat membaca/mengubah sumber daya miliknya — `RadiusAccount`, `TopUp`, `Payment`, `Session` (anti-IDOR).
- **Session admin**: cookie HttpOnly+Secure+SameSite, timeout idle/absolut, rotasi saat naik/turun hak, CSRF token pada semua endpoint admin yang mengubah state, lockout login gagal.
- **OTP email**: TTL ≤ 10 menit, sekali pakai, rate-limit + lockout, step-up untuk aksi sensitif (ganti kredensial, ganti email, top-up besar). Ganti email: OTP hanya ke alamat lama (terverifikasi). Recovery via admin memerlukan verifikasi out-of-band + audit log.
- **Kredensial WiFi**: tidak pernah disimpan cleartext; NT-hash (MD4) diakui lemah/tanpa salt — risiko didokumentasikan.
- **Transport RADIUS**: AP↔FreeRADIUS via tunnel/radsecproxy (hostapd hanya UDP; RadSec hanya server-side); record accounting/session = PII (akses admin-only, ada retensi).
- **SNMP v2c**: community string read-only & unik per-AccessPoint; risiko cleartext diakui; upgrade SNMPv3/tunnel dipertimbangkan.
- **Top-up**: nominal `TopUp` dihitung server-side (dari `Package`/`Tariff`); nominal dari klien diabaikan/divalidasi.
- **Binding CaptivePortal**: token server-side (dari sesi RADIUS User-Name/Calling-Station-Id) mengikat top-up ke `RadiusAccount`; identitas dari klien ditolak.
- **Secrets** (RADIUS shared-secret per-AccessPoint, payment key, SMTP, sertifikat/CA tunnel): penyimpanan aman, rotasi, revoke saat bocor.

## Asumsi (wajib divalidasi sebelum build)

- Perangkat klien Customer mendukung WPA2-Enterprise (802.1X/PEAP-MSCHAPv2). Smart TV/IoT mungkin TIDAK mendukung — perlu fallback (PSK per-Customer / MAC-auth) bila populasi perangkat tidak kompatibel.
  - **Validasi (2026-09-27) — TERKONFIRMASI**: BYOD (ponsel/tablet Android & iOS, laptop Windows/macOS, Apple TV) mendukung PEAP-MSCHAPv2 penuh; Smart TV, streaming stick/box, konsol, speaker pintar, dan IoT umum **TIDAK mendukung**. Mix dominan ponsel/laptop → BYOD aman, tapi **fallback tetap wajib** untuk perangkat non-BYOD. Bukti & tabel per-kategori: `docs/research/0001-wpa2-enterprise-device-support.md`.
- Topologi backhaul: isolasi VLAN per-Customer hanya bernilai bila banyak rumah berbagi satu segmen L2 upstream. Bila per-rumah routed/L3-isolated, isolasi VLAN per-Customer dapat didrop (VLAN suspended tetap ada).
  - **Validasi (2026-09-27)**: terkonfirmasi **Shared L2** → isolasi VLAN per-Customer bernilai; **ADR-0001 valid**.

## Keputusan & pertanyaan tersisa

- **Billing** *(diputuskan)*: usage-based, per `Customer` — `Balance` berkurang per jam saat minimal satu `Session` aktif (bukan per `Session`).
- **Stale session** *(default)*: `Session` ditutup otomatis bila tidak ada interim-update selama 2× interval interim; job rekonsiliasi harian; ada jalur penyesuaian billing bila rekonsiliasi menemukan sesi yang salah tagih.
- **Pembulatan** *(default)*: pengurangan pro-rata per detik (minimum 1 menit), interval interim accounting 5 menit.
- **Overdraft** *(diputuskan)*: pengurangan pemakaian diklem di 0 (tidak minus); reversal/chargeback dapat membuat `Balance` negatif (tercatat sebagai utang).
- **Build-vs-buy** *(diputuskan)*: build penuh — kebutuhan self-service + integrasi OpenWrt tidak terpenuhi platform existing.
- **Payment settlement** *(diputuskan)*: **grace access** — aktif sementara saat notifikasi sukses, kredit `Balance` saat settlement, revert saat reversal/chargeback.
- **Rekap pendapatan** *(diputuskan)*: berbasis kas diterima (settled `Payment`).
- **OTP** *(diputuskan)*: email faktor tunggal MVP; recovery via admin; faktor kedua post-MVP.
- **Transport RADIUS** *(diputuskan)*: tunnel/radsecproxy sebagai jalur utama (hostapd UDP-only).
- **Payment gateway** *(ditunda)*: tetap diabstraksikan (mock MVP); pilih Duitku/Tripay saat implementasi adapter.
- **Fallback non-BYOD** *(diputuskan)*: **dual-SSID** — SSID WPA2-Enterprise untuk BYOD + SSID PSK unik per-Customer untuk Smart TV/konsol/IoT (**ADR-0004**). Kedua SSID memetakan ke VLAN aktif `Customer` yang sama; sesi PSK tetap diatribusikan ke `Customer` agar billing usage-based benar. Ditunda (arsitek saja) — MVP hanya SSID Enterprise.
