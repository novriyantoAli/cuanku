# Cuanku — Manajemen Pengguna WPA2-Enterprise

Platform untuk penyedia layanan internet nirkabel (WISP) berbasis OpenWrt + FreeRADIUS: pelanggan mengelola kredensial WiFi mereka sendiri (WPA2-Enterprise), perangkat AccessPoint pelanggan dipantau via SNMP, dan pembayaran dilakukan secara online.

## Language

**Customer**:
Entitas yang membayar layanan internet (subjek billing). Dapat memiliki satu atau lebih `RadiusAccount`.
_Avoid_: User, Pelanggan, Subscriber, Account

**RadiusAccount**:
Kredensial WiFi (username + password) yang diautentikasi oleh FreeRADIUS untuk koneksi WPA2-Enterprise. Dimiliki oleh tepat satu `Customer`.
_Avoid_: User, Account, WiFi account, Credential

**TopUp**:
Transaksi penambahan `Balance` `Customer` melalui pembayaran online.
_Avoid_: Deposit, Isi ulang, Pembayaran, Payment

**Payment**:
Transaksi pembayaran online (via payment gateway) yang mengonfirmasi sebuah `TopUp`. Memiliki status notifikasi (pending/sukses/gagal) dan `settlementStatus` (pending/settled/reversed). Notifikasi sukses memicu `GraceAccess`; settlement mengkredit `Balance`; reversal/chargeback mendebit `Balance` dan men-suspend ulang.
_Avoid_: Transaksi, Pembayaran, Invoice, Checkout

**GraceAccess**:
Keadaan sementara ketika `Payment` sudah melaporkan sukses tetapi settlement belum terjadi: layanan dibuat `Active` tanpa mengkredit `Balance`. Hanya berlaku sampai tenggat yang ditetapkan saat notifikasi sukses diterima; setelah tenggat itu layanan kembali mengikuti `Balance`.
_Avoid_: Grace period, Masa tenggang, Akses gratis

**Balance**:
Sisa saldo `Customer` yang menentukan apakah layanan aktif atau tersuspensi. Berkurang sesuai `Tariff` hanya saat ada `Session` aktif (usage-based).
_Avoid_: Saldo, Credit, Kredit

**Tariff**:
Biaya yang dikurangkan dari `Balance` per jam saat minimal satu `Session` aktif (usage-based, per `Customer` — bukan per `Session`).
_Avoid_: Harga, Rate, Biaya

**Package**:
Paket layanan internet yang dipilih `Customer` (mis. kecepatan bandwidth dan `Tariff`-nya). Satu `Customer` berlangganan satu `Package` aktif pada satu waktu.
_Avoid_: Paket, Plan, Produk, Bundle, Product

**SubscriptionState**:
Status layanan yang dievaluasi per `Customer` lalu diterapkan ke setiap `RadiusAccount` miliknya: `Active` (`Balance` cukup, atau `GraceAccess` sedang berlaku) atau `Suspended` (`Balance` habis tanpa `GraceAccess`). Menentukan `Vlan` yang dikembalikan FreeRADIUS.
_Avoid_: Status, ServiceStatus, Keadaan layanan

**Vlan**:
VLAN dinamis yang dikembalikan FreeRADIUS ke AccessPoint. `RadiusAccount` aktif menempati VLAN unik per `Customer` (dibagi oleh semua `RadiusAccount` milik Customer itu); `RadiusAccount` suspended menempati satu VLAN suspended bersama.
_Avoid_: —

**CaptivePortal**:
Halaman pemberitahuan yang ditampilkan saat `RadiusAccount` berstatus `Suspended`, memberi tahu bahwa masa aktif telah habis dan memungkinkan `TopUp`. Mengautentikasi `Customer` lewat OTP; identitas yang disuplai klien (alamat MAC, alamat IP, parameter URL) tidak dipercaya.
_Avoid_: LoginPage, Portal, Halaman login

**AccessPoint**:
Perangkat OpenWrt di lokasi `Customer` yang menyiarkan jaringan WPA2-Enterprise dan menangani autentikasi via FreeRADIUS. Dipantau status & metriknya melalui monitoring (SNMP/TR-069).
_Avoid_: AP, Router, Device, Perangkat, CPE

**Session**:
Sesi koneksi online sebuah `RadiusAccount`, dicatat dari paket accounting FreeRADIUS (start/stop/interim) — mencatat status online & durasi.
_Avoid_: Koneksi, Login, Sesi online
