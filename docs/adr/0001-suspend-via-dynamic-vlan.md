# Suspend via VLAN dinamis + Disconnect/reauth, bukan reject keras

Saat `Balance` habis, alih-alih menolak autentikasi (hard reject), FreeRADIUS menempatkan `RadiusAccount` ke VLAN suspended (walled-garden) yang menampilkan `CaptivePortal` berisi pemberitahuan masa aktif habis + CTA top-up. `RadiusAccount` aktif mendapat VLAN unik per `Customer` (isolasi per rumah); VLAN suspended adalah satu VLAN bersama.

Status: accepted

## Mekanisme perpindahan Customer yang sedang online

- **Kebijakan berdiri (semua AccessPoint)**: FreeRADIUS menyertakan `Session-Timeout` (ditetapkan 1800 detik) **beserta `Termination-Action = RADIUS-Request (1)`** pada **setiap Access-Accept**. Semua AccessPoint — terjangkau langsung maupun di balik NAT/CGNAT — re-autentikasi berkala; saat re-auth, `SubscriptionState` dievaluasi ulang dan VLAN (aktif/suspended) diterapkan. Ini adalah jaminan dasar (floor) untuk suspend **dan** reaktivasi setelah top-up.
- **Jalur cepat (AccessPoint terjangkau langsung)**: FreeRADIUS mengirim **Disconnect-Request (RFC 5176, dahulu 3576)** untuk memutus sesi segera, memaksa re-authentication lebih cepat dari interval `Session-Timeout`.

Catatan teknis: hostapd (OpenWrt) mendukung RADIUS DAS **Disconnect-Request**, bukan CoA-Request untuk mengubah VLAN sesi yang sudah berdiri. `Session-Timeout` hanya dapat disampaikan lewat Access-Accept (bukan dikirim reaktif ke AccessPoint yang tidak terjangkau inbound), sehingga wajib menjadi kebijakan berdiri pada setiap Access-Accept.

## Alasan

- **Self-service recovery**: hard reject membuat Customer tidak bisa top-up sendiri (tidak ada akses internet ke halaman pembayaran). VLAN suspended + `CaptivePortal` memungkinkan Customer melihat pemberitahuan dan langsung top-up online (via data seluler atau allowlist egress gateway).
- **Isolasi per Customer**: VLAN aktif unik per `Customer` mencegah Customer saling melihat di layer 2 dan memudahkan QoS per Customer.
- **Penegakan Balance**: `Session-Timeout` berdiri + jalur cepat Disconnect memastikan suspend berlaku untuk semua AccessPoint, dan reaktivasi setelah top-up terjadi ≤ 1 interval `Session-Timeout`.

## Konsekuensi & prasyarat

- AccessPoint OpenWrt harus mendukung dynamic VLAN assignment (802.1X + atribut RADIUS `Tunnel-Type`/`Tunnel-Private-Group-Id`) dan RADIUS DAS Disconnect-Request (RFC 5176).
- Image AccessPoint harus memakai varian **full** dari `wpad`/`hostapd` (`wpad`, `wpad-mbedtls`, `wpad-openssl`, `wpad-wolfssl`, `hostapd-mbedtls`, …) dengan `CONFIG_FULL_DYNAMIC_VLAN` dan `dynamic_vlan=1`. Varian **basic** tidak layak: selain tanpa dynamic VLAN, dukungan RADIUS-nya dikompilasi keluar (`CONFIG_NO_RADIUS=y`). Nama paket `wpad-full` sudah tidak ada di OpenWrt terkini.
- DAS tidak menyala secara default: `DaePort`/`DaeClient`/`DaeSecret` (UCI) harus diisi, jika tidak AccessPoint diam-diam mengabaikan setiap Disconnect-Request.
- Disconnect-Request wajib diautentikasi dengan **shared-secret kuat & unik per AccessPoint**; AccessPoint menolak DAS dari sumber tidak tepercaya.
- Jaringan harus VLAN-capable: port AccessPoint = trunk, VLAN aktif per Customer + VLAN suspended harus terdefinisi di switch/gateway.
- `CaptivePortal` berjalan di gateway/jaringan VLAN suspended.
- VLAN suspended adalah zona reduced-trust bersama; wajib client-isolation (private VLAN / AP isolation) antar Customer suspended dan TLS pada `CaptivePortal`.
- Top-up di portal diikat ke `Customer` lewat **sesi terautentikasi OTP**, bukan lewat identitas yang disuplai klien. Mengikat identitas dari alamat MAC tidak dapat diandalkan: perangkat modern memakai alamat Wi-Fi berbeda per jaringan (Apple: "different Wi-Fi address" per network, aktif secara default; Android 10+: randomization aktif default, per profil jaringan), sehingga MAC di SSID Enterprise tidak akan cocok dengan MAC di SSID suspended.

## Revisi (2026-09-27) — hasil riset FreeRADIUS/OpenWrt

Keputusan inti tetap berdiri (suspend via `Vlan` dinamis, `Session-Timeout` berdiri, Disconnect-Request sebagai jalur cepat). Yang dikoreksi:

1. **`Termination-Action = RADIUS-Request (1)` wajib menyertai setiap `Session-Timeout`.** hostapd hanya mengubah `Session-Timeout` menjadi re-authentication 802.1X (`sm->reAuthPeriod`) bila atribut itu ada; tanpanya hostapd **memutus** klien pada saat timeout (`Acct-Terminate-Cause = Session-Timeout`). Tanpa klausul ini, janji reaktivasi "mulus" tidak terpenuhi.
2. **Nilai 1800 detik ditegakkan sebagai kebijakan konfigurasi di `post-auth`**, bukan sebagai baris per akun di `radreply`. Alasan: invarian yang harus ada di setiap Access-Accept tidak boleh bergantung pada kebenaran data aplikasi; `Tunnel-*` per akun tetap ditulis aplikasi. `Session-Timeout` juga membatasi masa hidup entri PMKSA, jadi nilainya berpengaruh pada ketatnya suspend.
3. **CoA-Request tidak dapat mengubah `Vlan`.** Satu-satunya CoA yang diproses hostapd adalah HS 2.0 Terms & Conditions filtering; CoA yang membawa `Tunnel-*` dijawab NAK (Error-Cause 401). Disconnect-Request tetap satu-satunya jalur reaktif.
4. **Residual risk PMKSA diterima, bukan diabaikan.** Entri PMKSA menyimpan deskriptor `Vlan`, dan klien yang reconnect memakai PMKSA valid melewati EAP sehingga `Vlan` lama diterapkan kembali — suspend dapat terlewati sampai sisa `Session-Timeout`. Mitigasi terpilih: instrumentasi di sisi aplikasi (mendeteksi `Session` yang masih menerima accounting saat `SubscriptionState` sudah `Suspended`, lalu memberi alert). `Disconnect-Request` membersihkan entri PMKSA, jadi jalur cepat sekaligus menutup jendela ini untuk AccessPoint terjangkau. Menonaktifkan PMKSA caching ditunda sampai ada data lapangan (efeknya di build OpenWrt belum diuji).
5. **Sitasi atribut dikoreksi:** `Tunnel-Type = VLAN (13)` bersumber dari RFC 3580/IANA, bukan RFC 2868 (RFC 2868 hanya mendefinisikan nilai 1–12).
6. **Baseline versi:** fitur yang dibutuhkan sudah terverifikasi ada di cabang OpenWrt **23.05** (fleet saat ini), 24.10, dan 25.12 — sehingga MVP divalidasi pada 23.05.5. Karena cabang 23.05 sudah **EOL** (tanpa patch keamanan), AccessPoint baru diprovisikan dengan **≥ 24.10** dan upgrade fleet lama dilacak sebagai pekerjaan tersendiri.

Detail bukti: `docs/research/0002-freeradius-provisioning-schema.md` dan `docs/research/0003-openwrt-hostapd-dynamic-vlan-das.md`.
