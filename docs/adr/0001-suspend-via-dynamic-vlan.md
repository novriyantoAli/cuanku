# Suspend via VLAN dinamis + Disconnect/reauth, bukan reject keras

Saat `Balance` habis, alih-alih menolak autentikasi (hard reject), FreeRADIUS menempatkan `RadiusAccount` ke VLAN suspended (walled-garden) yang menampilkan `CaptivePortal` berisi pemberitahuan masa aktif habis + CTA top-up. `RadiusAccount` aktif mendapat VLAN unik per `Customer` (isolasi per rumah); VLAN suspended adalah satu VLAN bersama.

Status: accepted

## Mekanisme perpindahan Customer yang sedang online

- **Kebijakan berdiri (semua AccessPoint)**: FreeRADIUS menyertakan `Session-Timeout` pendek (10–30 menit) pada **setiap Access-Accept**. Semua AccessPoint — terjangkau langsung maupun di balik NAT/CGNAT — re-autentikasi berkala; saat re-auth, `SubscriptionState` dievaluasi ulang dan VLAN (aktif/suspended) diterapkan. Ini adalah jaminan dasar (floor) untuk suspend **dan** reaktivasi setelah top-up.
- **Jalur cepat (AccessPoint terjangkau langsung)**: FreeRADIUS mengirim **Disconnect-Request (RFC 3576)** untuk memutus sesi segera, memaksa re-authentication lebih cepat dari interval `Session-Timeout`.

Catatan teknis: hostapd (OpenWrt) mendukung RADIUS DAS **Disconnect-Request**, bukan CoA-Request untuk mengubah VLAN sesi yang sudah berdiri. `Session-Timeout` hanya dapat disampaikan lewat Access-Accept (bukan dikirim reaktif ke AccessPoint yang tidak terjangkau inbound), sehingga wajib menjadi kebijakan berdiri pada setiap Access-Accept.

## Alasan

- **Self-service recovery**: hard reject membuat Customer tidak bisa top-up sendiri (tidak ada akses internet ke halaman pembayaran). VLAN suspended + `CaptivePortal` memungkinkan Customer melihat pemberitahuan dan langsung top-up online (via data seluler atau allowlist egress gateway).
- **Isolasi per Customer**: VLAN aktif unik per `Customer` mencegah Customer saling melihat di layer 2 dan memudahkan QoS per Customer.
- **Penegakan Balance**: `Session-Timeout` berdiri + jalur cepat Disconnect memastikan suspend berlaku untuk semua AccessPoint, dan reaktivasi setelah top-up terjadi ≤ 1 interval `Session-Timeout`.

## Konsekuensi & prasyarat

- AccessPoint OpenWrt harus mendukung dynamic VLAN assignment (802.1X + atribut RADIUS `Tunnel-Type`/`Tunnel-Private-Group-Id`) dan RADIUS DAS Disconnect-Request (RFC 3576).
- Image AccessPoint harus memakai build **wpad (full) / hostapd** dengan `CONFIG_FULL_DYNAMIC_VLAN` dan `dynamic_vlan=1` — build wpad-basic default OpenWrt TIDAK mendukung dynamic VLAN.
- Disconnect-Request wajib diautentikasi dengan **shared-secret kuat & unik per AccessPoint**; AccessPoint menolak DAS dari sumber tidak tepercaya.
- Jaringan harus VLAN-capable: port AccessPoint = trunk, VLAN aktif per Customer + VLAN suspended harus terdefinisi di switch/gateway.
- `CaptivePortal` berjalan di gateway/jaringan VLAN suspended.
- VLAN suspended adalah zona reduced-trust bersama; wajib client-isolation (private VLAN / AP isolation) antar Customer suspended, TLS pada `CaptivePortal`, dan top-up diikat ke identitas `RadiusAccount` via token server-side yang diturunkan dari sesi RADIUS (User-Name / Calling-Station-Id) — bukan identitas yang disuplai klien.
