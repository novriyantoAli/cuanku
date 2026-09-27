# Batas integrasi FreeRADIUS: aplikasi pemilik state, DB RADIUS sumber durasi

Aplikasi Go menulis kredensial `RadiusAccount` langsung ke tabel SQL `radcheck`/`radreply`, sementara `Vlan`/`SubscriptionState` per `Session` dimiliki tabel `Session` milik aplikasi. `radacct` dipakai sebagai sumber durasi dan volume, dikorelasikan lewat `AcctUniqueId`. Atribut RADIUS `Class` ditulis hanya sebagai penanda diagnostik dan tidak pernah dibaca logika billing.

Status: accepted

## Keputusan

- **Kredensial**: `RadiusAccount` disimpan sebagai `NT-Password` (MD4 dari password representasi UTF-16LE, 32 digit heksadesimal) — cleartext tidak pernah disimpan, dan **tidak diperlukan** untuk PEAP-MSCHAPv2 karena kunci MPPE diturunkan dari NT hash. `Cleartext-Password` tidak dipakai; kombinasi keduanya dihindari.
- **Jalur tulis**: langsung ke tabel SQL `radcheck`/`radreply` dalam satu transaksi PostgreSQL — bukan `rlm_rest`, bukan berkas `users`.
- **Keunikan `username`** ditegakkan aplikasi. Skema resmi hanya punya indeks **non-unik** `(UserName, Attribute)`, sehingga database tidak akan menolak duplikat.
- **Kepemilikan `Vlan`/`SubscriptionState` per `Session`**: tabel `Session` aplikasi. `radacct` menyediakan durasi/volume (`acctstarttime`, `acctupdatetime`, `acctstoptime`, `acctinputoctets`, `acctoutputoctets`) dan dikorelasikan lewat `AcctUniqueId`.
- **`Class` hanya diagnostik.** Nilainya boleh ditulis (mis. `vlan=123;state=active`) agar `radacct.Class` dapat diperiksa manual saat rollout, tetapi billing tidak pernah membacanya.
- **Stale session memakai `AcctUpdateTime`**, bukan `AcctInterval` — `AcctInterval` berarti selisih sejak update terakhir, sehingga sesi yang interim-nya hilang justru menghasilkan nilai yang membesar.
- **Versi server**: FreeRADIUS **3.2.x**. 3.0.x sudah EOL dan 4.x masih eksperimental (dokumentasi resminya sendiri menganjurkan 3 untuk produksi).

## Alasan

- State yang menentukan uang (billing usage-based, penegakan suspend) tidak boleh bergantung pada data yang diproduksi perangkat pelanggan. `radacct.Class` hilang bila `acct_server` tidak dikonfigurasi di `AccessPoint`, dan bisa ambigu bila `Class` muncul lebih dari satu kali.
- Tulis-sambil-baca pada DB yang sama sudah didukung perilakuFreeRADIUS: hasil authorize dari `radcheck` **tidak di-cache**, jadi kredensial baru berlaku pada Access-Request berikutnya tanpa invalidasi apa pun dari aplikasi.
- Memilih jalur SQL langsung (bukan `rlm_rest`) menghapus satu hop jaringan dari jalur kritis autentikasi.

## Konsekuensi

- Aplikasi bertanggung jawab penuh atas konsistensi: keunikan `username`, transaksi, dan larangan menyisakan baris yatim saat `username` berubah (rename = `UPDATE` dalam satu transaksi; rename tidak memutus sesi berjalan karena sesi berkunci `AcctUniqueId`).
- Job rekonsiliasi berkala membandingkan `radacct` dengan tabel `Session` aplikasi lewat `AcctUniqueId`, dan menjadi satu-satunya jalur penyesuaian billing.
- `Class` yang tidak terbawa (mis. `acct_server` tidak dipasang) hanya mengurangi kemampuan audit, bukan merusak billing.
- **Terbuka**: charset password `RadiusAccount` belum ditetapkan. NT hash dihitung dari representasi UTF-16LE sementara FreeRADIUS memakai UCS-2, sehingga karakter di luar BMP berpotensi menghasilkan hash yang berbeda. Ditetapkan saat implementasi provisi `RadiusAccount`.

Detail bukti: `docs/research/0002-freeradius-provisioning-schema.md`.
