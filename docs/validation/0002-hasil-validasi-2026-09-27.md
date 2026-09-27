# Hasil validasi lab: penegakan suspend via VLAN dinamis — 2026-09-27

Template hasil untuk runbook `docs/validation/0001-suspend-vlan-lab.md`. Prosedur tiap uji **tidak**
diulang di sini; dokumen ini hanya tempat mencatat apa yang terjadi.

Cara mengisi, untuk setiap uji A–F (menurut §7 runbook): **perintah yang dijalankan**, **keluaran
verbatim yang relevan**, **lulus/gagal**, dan **apa yang berubah di dokumen** bila gagal.

**Status: belum ada uji yang dijalankan.** Tidak ada baris di bawah ini yang boleh berubah menjadi
`lulus` tanpa keluaran verbatim yang menyertainya.

## Ringkasan

| Uji | Membuktikan | Status | Klausul yang dipertaruhkan bila gagal |
|---|---|---|---|
| — | Prasyarat §0 (FreeRADIUS listening, user uji, AP) | **belum** | semua uji di bawahnya tidak bisa jalan |
| A | `NT-Password` cukup; Access-Accept membawa `Tunnel-*` + `Session-Timeout` + `Termination-Action` | **belum** | ADR-0005, issue #6 |
| B | hostapd benar-benar membentuk bridge/VLAN dari `Tunnel-*` | **belum** | ADR-0001, issue #14 |
| C | `Disconnect-Request` benar-benar memutus sesi | **belum** | jalur cepat suspend/reaktivasi (issue #8, #14) |
| D | `Session-Timeout` + `Termination-Action=1` → re-auth, bukan deauth | **belum** | **klausul wajib ADR-0001; hentikan #7 bila gagal** |
| E | Lebar jendela bocor PMKSA saat suspend | **belum** | `disable_pmksa_caching` naik dari "ditunda" ke "wajib" |
| F | `Class` dari Access-Accept tiba di `radacct.Class` | **belum** | penanda diagnostik ADR-0005 (tidak memblokir apa pun) |

## Prasyarat §0 — dikerjakan manusia, bukan agent

Diperiksa dari mesin kerja pada 2026-09-27:

- PostgreSQL VM `172.16.0.72:5432` **terjangkau** (TCP connect berhasil).
- `172.16.0.72:1812` dan `:1813` menolak koneksi **TCP**. RADIUS memakai UDP, jadi ini tidak
  membuktikan apa pun tentang status listening sebenarnya — **status UDP belum diverifikasi** dan
  hanya bisa dipastikan dari shell VM.
- `ssh` ke VM **ditolak** (`Permission denied (publickey,password)` untuk `real@` dan `debian@`).
  Kredensial VM tidak tersedia di mesin kerja, sehingga seluruh pekerjaan di dalam VM harus
  dilakukan manual oleh operator.

Daftar periksa sebelum uji A:

- [ ] `sudo systemctl enable --now freeradius` di VM, lalu `sudo ss -lunp | grep -E ':1812|:1813'`
      menunjukkan **UDP** yang listening (bukan kosong)
- [ ] Baris `radcheck` user uji `uji@lab` sudah ada dengan `NT-Password`
- [ ] `Session-Timeout = 60` sementara diset untuk uji D (kembalikan ke 1800 setelahnya)
- [ ] AccessPoint OpenWrt 23.05.5 siap dengan UCI `DaePort` / `DaeClient` / `DaeSecret`
- [ ] Klien uji yang mendukung PEAP-MSCHAPv2 siap

## Uji A — atribut Access-Accept

**Perintah:**

```
(belum dijalankan)
```

**Keluaran verbatim:**

```
(belum ada)
```

**Hasil:** belum · **Perubahan dokumen:** —

## Uji B — VLAN dinamis terbentuk di hostapd

**Perintah:**

```
(belum dijalankan)
```

**Keluaran verbatim:**

```
(belum ada)
```

**Hasil:** belum · **Perubahan dokumen:** —

## Uji C — Disconnect-Request

**Perintah:**

```
(belum dijalankan)
```

**Keluaran verbatim:**

```
(belum ada)
```

**Hasil:** belum · **Perubahan dokumen:** —

## Uji D — `Termination-Action` menentukan re-auth vs deauth

Uji terpenting di seluruh runbook. Ukur **dua kali**: dengan `Termination-Action=1` (D1) dan tanpa
(D2).

**Perintah:**

```
(belum dijalankan)
```

**Keluaran verbatim D1 (dengan `Termination-Action=1`):**

```
(belum ada)
```

**Keluaran verbatim D2 (tanpa `Termination-Action`):**

```
(belum ada)
```

**Hasil:** belum · **Perubahan dokumen:** —

> Bila D1 ternyata juga memutus klien, **ADR-0001 harus direvisi** dan jalur agent **dihentikan di
> issue #7** — jangan membangun `Session-Timeout` yang di lapangan berperilaku deauth.

## Uji E — jendela bocor PMKSA

**Perintah:**

```
(belum dijalankan)
```

**Keluaran verbatim:**

```
(belum ada)
```

**Lama klien bertahan di VLAN aktif setelah state berubah:** _(belum diukur)_

**Hasil:** belum · **Perubahan dokumen:** —

## Uji F — `Class` sampai ke `radacct`

**Perintah:**

```sql
SELECT username, callingstationid, class, acctstarttime, acctupdatetime
FROM radacct ORDER BY acctstarttime DESC LIMIT 5;
```

**Keluaran verbatim:**

```
(belum ada)
```

**Hasil:** belum · **Perubahan dokumen:** —

## Tindak lanjut setelah terisi

1. Perbarui klausul ADR yang terbantah — jangan diamkan.
2. Tutup bagian yang relevan di issue #24.
3. Komentari issue #14 bila hasil uji E mengubah keputusan PMKSA.
4. Bila uji D gagal: hentikan pekerjaan issue #7 dan revisi ADR-0001 lebih dulu.
