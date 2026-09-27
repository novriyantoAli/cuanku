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
- `CaptivePortal` di VLAN suspended (pemberitahuan masa aktif habis + CTA top-up, TLS, identitas pelanggan lewat sesi terautentikasi OTP).
- Top-up online (gateway diabstraksikan; grace access saat notifikasi sukses, kredit `Balance` saat settlement — ADR-0003).
- Suspend/aktif via VLAN dinamis (ADR-0001).
- Monitoring `AccessPoint` via SNMP v2c @5 menit (adapter — ADR-0002); status online/offline/unreachable. Hanya berlaku untuk `AccessPoint` yang punya jalur IP inbound; kontrak adapter disiapkan agar agent push dapat ditambahkan tanpa mengubah domain. Metrik WiFi dikumpulkan sebagai agregat per interface lewat skrip `extend` di perangkat.
- Accounting minimal (start/stop/interim) sebagai sumber durasi & volume; `Vlan`/`SubscriptionState` per `Session` dimiliki tabel aplikasi (ADR-0005).
- Rekap pendapatan (kas diterima = settled `Payment`), notifikasi email (konfirmasi top-up & alert `Balance`), alert `AccessPoint` offline (kecuali unreachable).

## Batas MVP — ditunda

Self-registration Customer · peran teknisi · export CSV · accounting lengkap/report data usage · OTP WhatsApp/SMS · payment gateway konkret · SSID PSK per-Customer (fallback non-BYOD — ADR-0004).

## Kriteria sukses

- **T1** — Customer mengganti kredensial sendiri ≤ 2 menit; kredensial baru berlaku ≤ 1 interval re-auth (Disconnect untuk AccessPoint terjangkau; ≤ 30 menit untuk AccessPoint di balik NAT). Interval 30 menit itu berasal dari kebijakan `Session-Timeout` + `Termination-Action` di ADR-0001, bukan dari perilaku default AccessPoint.
- **T2** — Top-up notifikasi sukses → grace aktif ≤ 1 interval re-auth; settlement → `Balance` dikredit final; reversal/chargeback → `Balance` didebit & suspend ulang.
- **T3** — Admin melihat status `AccessPoint` (online/offline/unreachable) terbaru ≤ 5 menit untuk AP terjangkau SNMP; metrik inti (uptime, traffic RX/TX, CPU/memori, jumlah client per interface, signal terlemah & rata-rata per interface, noise floor kanal per interface) tercatat konsisten dengan interval polling. Metrik *noise per client* tidak dapat dipenuhi dan tidak dijanjikan.
- **T4** — Rekap pendapatan rekonsiliabel terhadap `Payment` settled (kas diterima); kelola `Customer`/`Package`/`AccessPoint` selesai tanpa SQL/CLI (app-side; konfigurasi VLAN switch/gateway = langkah manual terdokumentasi).
- *(Target kuantitatif — jumlah Customer aktif, SLA — diisi kemudian.)*

## Persyaratan keamanan (dari review ce-doc-review)

- **Webhook payment**: wajib verifikasi signature + idempotensi; webhook settlement/reversal direkonsiliasi (retry bila hilang).
- **Auth/authz**: setiap endpoint menyatakan aktornya; `Customer` hanya dapat membaca/mengubah sumber daya miliknya — `RadiusAccount`, `TopUp`, `Payment`, `Session` (anti-IDOR).
- **Session admin**: cookie HttpOnly+Secure+SameSite, timeout idle/absolut, rotasi saat naik/turun hak, CSRF token pada semua endpoint admin yang mengubah state, lockout login gagal.
- **OTP email**: TTL ≤ 10 menit, sekali pakai, rate-limit + lockout, step-up untuk aksi sensitif (ganti kredensial, ganti email, top-up besar). Ganti email: OTP hanya ke alamat lama (terverifikasi). Recovery via admin memerlukan verifikasi out-of-band + audit log.
- **Kredensial WiFi**: tidak pernah disimpan cleartext; NT-hash (MD4) diakui lemah/tanpa salt — risiko didokumentasikan. Cleartext **tidak diperlukan** untuk PEAP-MSCHAPv2 (kunci MPPE diturunkan dari NT hash), jadi tidak ada alasan menyimpannya.
- **Transport RADIUS**: AP↔FreeRADIUS via tunnel/radsecproxy (hostapd hanya UDP; RadSec hanya server-side); record accounting/session = PII (akses admin-only, ada retensi).
- **SNMP v2c**: community string read-only & unik per-AccessPoint; risiko cleartext diakui; upgrade SNMPv3/tunnel dipertimbangkan.
- **Top-up**: nominal `TopUp` dihitung server-side (dari `Package`/`Tariff`); nominal dari klien diabaikan/divalidasi.
- **Binding CaptivePortal**: sesi portal terautentikasi OTP mengikat top-up ke `Customer`/`RadiusAccount`; identitas yang disuplai klien (alamat MAC, alamat IP, parameter URL) ditolak dan tidak pernah otoritatif. Portal wajib memandu Customer agar bisa menyelesaikan OTP (mis. mematikan WiFi sementara atau memakai perangkat lain), karena perangkat yang tersambung ke VLAN suspended tidak punya akses internet untuk membuka email.
- **Secrets** (RADIUS shared-secret per-AccessPoint, payment key, SMTP, sertifikat/CA tunnel): penyimpanan aman, rotasi, revoke saat bocor.

## Asumsi (wajib divalidasi sebelum build)

- Perangkat klien Customer mendukung WPA2-Enterprise (802.1X/PEAP-MSCHAPv2). Smart TV/IoT mungkin TIDAK mendukung — perlu fallback (PSK per-Customer / MAC-auth) bila populasi perangkat tidak kompatibel.
  - **Validasi (2026-09-27) — TERKONFIRMASI**: BYOD (ponsel/tablet Android & iOS, laptop Windows/macOS, Apple TV) mendukung PEAP-MSCHAPv2 penuh; Smart TV, streaming stick/box, konsol, speaker pintar, dan IoT umum **TIDAK mendukung**. Mix dominan ponsel/laptop → BYOD aman, tapi **fallback tetap wajib** untuk perangkat non-BYOD. Bukti & tabel per-kategori: `docs/research/0001-wpa2-enterprise-device-support.md`.
- Topologi backhaul: isolasi VLAN per-Customer hanya bernilai bila banyak rumah berbagi satu segmen L2 upstream. Bila per-rumah routed/L3-isolated, isolasi VLAN per-Customer dapat didrop (VLAN suspended tetap ada).
  - **Validasi (2026-09-27)**: terkonfirmasi **Shared L2** → isolasi VLAN per-Customer bernilai; **ADR-0001 valid**.
- Reachability `AccessPoint` untuk monitoring: SNMP adalah protokol pull, sehingga hanya AP dengan jalur IP inbound yang dapat dipantau.
  - **Validasi (2026-09-27)**: reverse tunnel **bukan** jalan keluar (SSH mem-forward TCP saja; SNMP di UDP/161), dan TR-069/TR-369 tidak tersedia sebagai paket resmi OpenWrt. T3 tetap berlaku karena sudah dibatasi ke AP terjangkau; AP lain berstatus `unreachable`. Bukti: `docs/research/0004-openwrt-snmp-monitoring.md`.
- Perangkat non-BYOD (Smart TV, konsol, IoT) tidak dapat bergabung ke SSID Enterprise, sehingga juga tidak dapat mencapai `CaptivePortal` di VLAN suspended. Selama ADR-0004 belum dikerjakan, perangkat tersebut tidak punya layanan sama sekali — termasuk tidak punya jalur top-up.

## Keputusan & pertanyaan tersisa

- **Billing** *(diputuskan)*: usage-based, per `Customer` — `Balance` berkurang per jam saat minimal satu `Session` aktif (bukan per `Session`).
- **Stale session** *(default)*: `Session` ditutup otomatis bila tidak ada interim-update selama 2× interval interim; implementasinya memakai `AcctUpdateTime` (bukan `AcctInterval`, yang berarti selisih sejak update terakhir); job rekonsiliasi harian; ada jalur penyesuaian billing bila rekonsiliasi menemukan sesi yang salah tagih.
- **Pembulatan** *(default)*: pengurangan pro-rata per detik (minimum 1 menit), interval interim accounting 5 menit.
- **Overdraft** *(diputuskan)*: pengurangan pemakaian diklem di 0 (tidak minus); reversal/chargeback dapat membuat `Balance` negatif (tercatat sebagai utang).
- **Build-vs-buy** *(diputuskan)*: build penuh — kebutuhan self-service + integrasi OpenWrt tidak terpenuhi platform existing.
- **Payment settlement** *(diputuskan)*: **grace access** — aktif sementara saat notifikasi sukses, kredit `Balance` saat settlement, revert saat reversal/chargeback.
- **Rekap pendapatan** *(diputuskan)*: berbasis kas diterima (settled `Payment`).
- **OTP** *(diputuskan)*: email faktor tunggal MVP; recovery via admin; faktor kedua post-MVP.
- **Transport RADIUS** *(diputuskan)*: tunnel/radsecproxy sebagai jalur utama (hostapd UDP-only).
- **Payment gateway** *(ditunda)*: tetap diabstraksikan (mock MVP); pilih Duitku/Tripay saat implementasi adapter.
- **Fallback non-BYOD** *(diputuskan)*: **dual-SSID** — SSID WPA2-Enterprise untuk BYOD + SSID PSK unik per-Customer untuk Smart TV/konsol/IoT (**ADR-0004**). Kedua SSID memetakan ke VLAN aktif `Customer` yang sama; sesi PSK tetap diatribusikan ke `Customer` agar billing usage-based benar. Ditunda (arsitek saja) — MVP hanya SSID Enterprise.
- **Grace access** *(diputuskan)*: `GraceAccess` bertenggat — notifikasi sukses menyalakan layanan tanpa mengkredit `Balance`, berlaku hanya sampai tenggat yang ditetapkan saat notifikasi diterima (**ADR-0003**).
- **Identitas `CaptivePortal`** *(diputuskan)*: sesi terautentikasi OTP; pengikatan berbasis MAC ditolak karena alamat Wi-Fi perangkat berbeda per jaringan (perilaku default Apple dan Android 10+).
- **Baseline versi AccessPoint** *(diputuskan)*: validasi pada OpenWrt 23.05.5 (fleet saat ini, fitur terverifikasi ada), provisioning AP baru ≥ 24.10. Cabang 23.05 sudah EOL sehingga upgrade fleet dilacak sebagai pekerjaan tersendiri.
- **Charset password `RadiusAccount`** *(terbuka)*: NT hash dihitung dari representasi UTF-16LE sementara FreeRADIUS memakai UCS-2; karakter di luar BMP berpotensi menghasilkan hash berbeda. Ditetapkan saat implementasi provisi `RadiusAccount` (#6).
