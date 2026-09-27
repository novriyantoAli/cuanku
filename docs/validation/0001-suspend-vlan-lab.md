# Validasi lab: penegakan suspend via VLAN dinamis

Runbook untuk membuktikan enam hal yang **tidak bisa ditutup dokumen**, sebelum jalur agent membangun enam issue di atasnya. Setiap uji menyebut apa yang membatalkan keputusan mana.

Lingkungan: `AccessPoint` OpenWrt 23.05.5, FreeRADIUS di VM (lihat `AGENTS.md`).

## Peta uji → klausul yang dipertaruhkan

| Uji | Membuktikan | Kalau gagal |
|---|---|---|
| A | `NT-Password` cukup, dan Access-Accept membawa `Tunnel-*` + `Session-Timeout` + `Termination-Action` | ADR-0005 dan #6 salah model data |
| B | hostapd benar-benar membuat bridge/VLAN dari `Tunnel-*` | ADR-0001 tidak bisa dijalankan |
| C | Disconnect-Request benar-benar memutus sesi | Jalur cepat suspend/reaktivasi hilang |
| D | `Session-Timeout` + `Termination-Action=1` → re-auth, bukan deauth | Klausul wajib ADR-0001 tidak berlaku; klien terputus tiap 30 menit |
| E | Lebar jendela bocor PMKSA saat suspend | Hardening PMKSA naik dari "ditunda" menjadi wajib |
| F | `Class` dari Access-Accept benar-benar tiba di `radacct.Class` | Penanda diagnostik ADR-0005 tidak berguna (billing tetap aman) |

---

## 0. Prasyarat

### 0.1 FreeRADIUS di VM harus listening dulu

Saat terakhir diperiksa, 1812/1813 tertutup. Di VM:

```bash
systemctl status freeradius --no-pager
sudo journalctl -u freeradius -n 50 --no-pager
sudo ss -lunp | grep -E ':1812|:1813'      # harus UDP, bukan kosong
sudo systemctl enable --now freeradius
```

Kalau tetap tidak listening, periksa bahwa `sites-enabled/default` memuat blok `listen { type = auth }` dan `listen { type = acct }` dengan `ipaddr = *` (bukan `127.0.0.1`), dan pastikan tidak ada firewall di VM yang menutup UDP 1812/1813.

Berkas log terpisah untuk uji ini (jangan pakai `radiusd -X` sambil service jalan — `-X` butuh service dimatikan; pakai salah satu):

```bash
sudo journalctl -u freeradius -f          # mode service
# atau
sudo systemctl stop freeradius && sudo radiusd -X   # mode debug, lebih terlihat
```

### 0.2 Siapkan user uji

Secret client default untuk `127.0.0.1` di Debian adalah `testing123` (`/etc/freeradius/3.0/clients.conf`). Pakai itu untuk uji A.

Hash NT dari password uji (`UjiLapangan123`). Perhatikan dua jebakan: `iconv` ke UTF-16LE, dan OpenSSL 3 butuh legacy provider untuk MD4.

```bash
PW='UjiLapangan123'
printf '%s' "$PW" | iconv -f UTF-8 -t UTF-16LE | openssl dgst -md4 -provider legacy
```

Bila `md4` tidak tersedia sama sekali, jangan mengarang hash — lewati uji A1 ke arah sebaliknya: hitung hash dengan implementasi NTLM apa pun, lalu **bukti kebenarannya adalah uji A1 sendiri** (autentikasi berhasil atau tidak).

```sql
-- di DB FreeRADIUS
INSERT INTO radcheck (username, attribute, op, value)
VALUES ('uji@lab', 'NT-Password', ':=', 'HASIL_HASH_HEX_HURUF_BESAR');

-- Tunnel-* per akun (sesuai ADR-0001 revisi: per-akun hanya VLAN)
INSERT INTO radreply (username, attribute, op, value) VALUES
  ('uji@lab', 'Tunnel-Type',              ':=', 'VLAN'),
  ('uji@lab', 'Tunnel-Medium-Type',       ':=', 'IEEE-802'),
  ('uji@lab', 'Tunnel-Private-Group-Id',  ':=', '123');

-- Class sebagai penanda diagnostik (uji F). Perhatikan operator :=
INSERT INTO radreply (username, attribute, op, value)
VALUES ('uji@lab', 'Class', ':=', 'vlan=123;state=active');
```

> Bila kolom `op` di skema lokalmu bukan `:=` untuk `radreply`, sesuaikan; yang penting nilainya persis seperti di atas.

Kebijakan berdiri (`Session-Timeout` + `Termination-Action`) **tidak** ditulis per akun, melainkan di konfigurasi. Tambahkan pada `post-auth` di `sites-available/default`:

```
post-auth {
    update reply {
        Session-Timeout := 60          # 60 detik untuk lab; produksi 1800
        Termination-Action := 1
    }
    ...
}
```

Untuk uji D2, komentari baris `Termination-Action` saja — jangan hapus `Session-Timeout`.

### 0.3 Kalau tidak ada radio Wi-Fi fisik

Uji B–E butuh hostapd nyata. Tanpa kartu Wi-Fi, jalankan OpenWrt di VM dengan radio virtual:

```bash
opkg update && opkg install kmod-mac80211-hwsim iw
modprobe mac80211_hwsim radios=2
iw dev            # harus muncul dua phy
```

Kalau ini tidak bisa dijalankan, uji A dan F tetap bisa dilakukan sekarang dan sisanya ditunda — catat itu sebagai hasil, bukan sebagai lulus.

---

## 1. Uji A — atribut RADIUS yang benar-benar keluar

### A1 — jalur hash (tanpa EAP, cepat)

```bash
# di VM
apt-get install -y freeradius-utils
radtest -x 'uji@lab' 'UjiLapangan123' 127.0.0.1:1812 0 testing123
```

**Harus terlihat:** `Access-Accept` dengan `Tunnel-Type`, `Tunnel-Medium-Type`, `Tunnel-Private-Group-Id`, `Session-Timeout`, `Termination-Action`, `Class`.

**Gagal bila:** `Access-Reject` (hash salah, atau modul `pap` tidak aktif), atau atribut `Tunnel-*` tidak muncul.

### A2 — jalur PEAP-MSCHAPv2 penuh

`eapol_test` tidak dipaketkan Debian; ia dibangun dari sumber wpa_supplicant:

```bash
apt-get install -y build-essential libssl-dev libnl-3-dev libnl-genl-3-dev pkg-config
# ambil wpa_supplicant versi yang setara hostapd yang dipakai OpenWrt target
cd wpa_supplicant-*/wpa_supplicant && make eapol_test
```

Konfigurasi minimal `peap.conf`: `network={ ssid="lab" key_mgmt=WPA-EAP eap=PEAP phase2="auth=MSCHAPV2" identity="uji@lab" password="UjiLapangan123" ca_cert=... }`

```bash
./eapol_test -c peap.conf -a 172.16.0.72 -s testing123
```

**Bukti kunci** ada di `radiusd -X`, bukan di output klien: harus muncul penambahan kunci MS-CHAPv2 MPPE. Ketiadaan kunci MPPE berarti sesi WPA2 tidak akan pernah hidup meskipun autentikasi "berhasil".

**Checkpoint penting:** bila `Tunnel-*`/`Session-Timeout` hanya didefinisikan di `inner-tunnel`, catat apakah atribut itu benar-benar sampai ke Access-Accept luar. Ini pertanyaan yang belum tuntas dan hanya bisa dijawab uji ini — tulis hasilnya di mana atribut itu akhirnya bekerja, karena itu menentukan letak konfigurasi produksi.

---

## 2. Uji B — hostapd membuat VLAN dinamis

Pastikan varian paket benar sebelum menyalakan apa pun:

```bash
opkg list-installed | grep -E '^(wpad|hostapd)'    # harus varian full
opkg list-installed | grep -E 'wpad-basic' && echo "SALAH: varian basic tidak punya dynamic VLAN"
```

Konfigurasi uji (UCI):

```bash
uci set wireless.@wifi-iface[0].encryption='wpa2'
uci set wireless.@wifi-iface[0].auth_server='172.16.0.72'
uci set wireless.@wifi-iface[0].auth_secret='testing123'
uci set wireless.@wifi-iface[0].acct_server='172.16.0.72'
uci set wireless.@wifi-iface[0].acct_secret='testing123'
uci set wireless.@wifi-iface[0].dynamic_vlan='2'
uci set wireless.@wifi-iface[0].vlan_tagged_interface='br-lan'
uci set wireless.@wifi-iface[0].vlan_bridge='br-vlan'
uci commit wireless && wifi reload
```

Verifikasi **konfigurasi yang benar-benar dihasilkan** (bukan yang kamu niatkan):

```bash
grep -E 'dynamic_vlan|vlan_|macaddr_acl' /var/run/hostapd-phy0.conf
```

Lalu sambungkan klien uji dan periksa:

```bash
ip -d link show | grep -A2 'br-vlan'
bridge link show
logread | grep -iE 'vlan' | tail -20
```

**Harus terlihat:** bridge/interface untuk VLAN 123 terbentuk otomatis, dan klien memperoleh DHCP di VLAN itu.

**Gagal bila:** tidak ada interface baru (cek `vlan_tagged_interface` — nilainya beda per perangkat DSA dan salah nilai membuat klien terisolasi total), atau muncul error VLAN di `logread`.

---

## 3. Uji C — Disconnect-Request (DAS)

DAS mati kecuali diaktifkan. Nama opsi UCI sudah terverifikasi ada di 23.05: `dae_port`, `dae_client`, `dae_secret`.

```bash
uci set wireless.@wifi-iface[0].dae_port='3799'
uci set wireless.@wifi-iface[0].dae_client='172.16.0.72'
uci set wireless.@wifi-iface[0].dae_secret='secret-das-kuat'
uci commit wireless && wifi reload
grep radius_das /var/run/hostapd-phy0.conf   # harus radius_das_port dan radius_das_client
```

Ambil `Acct-Session-Id` sesi aktif dari `radacct`, lalu kirim Disconnect dari VM:

```bash
radclient -h | grep -i disconnect     # konfirmasi dukungan tipe request di versimu
echo "Acct-Session-Id=<nilai-dari-radacct>" | radclient -x <ip-AP>:3799 disconnect 'secret-das-kuat'
```

**Harus terlihat:** klien hilang dari `iw dev <dev> station dump`, dan `logread` mencatat disconnect.

**Gagal/baca kode:** 503 = sesi tidak ditemukan (identifikasi kurang spesifik); 508 = multi-sesi (pakai `Acct-Session-Id`, bukan `User-Name`); tidak ada reaksi sama sekali = DAS tidak terkonfigurasi di AP, atau AP mensyaratkan `Message-Authenticator` yang tidak dikirim `radclient`.

---

## 4. Uji D — `Termination-Action` menentukan re-auth vs deauth

Ini uji terpenting di seluruh runbook. Gunakan `Session-Timeout = 60` dari langkah 0.2 dan ukur dua kali.

- **D1 (dengan `Termination-Action=1`):** sambungkan klien, tunggu lewat 60 detik. Harus muncul Access-Request **baru** di `radiusd -X`, klien **tetap tersambung**, dan VLAN diterapkan ulang sesuai state terbaru.
- **D2 (tanpa `Termination-Action`):** ulangi. Harus terlihat klien **ter-deautentikasi** dengan `Acct-Terminate-Cause = Session-Timeout`.

Catat perbedaannya secara verbatim dari `logread` dan `radiusd -X`. Perbedaan inilah bukti yang menopang klausul wajib ADR-0001 — dan kalau D1 ternyata juga memutus klien, ADR-0001 harus direvisi, bukan sekadar dicatat.

---

## 5. Uji E — lebar jendela bocor PMKSA

Prosedur:

1. Klien online di VLAN 123. Konfirmasi entri PMKSA muncul di `logread` (cari penambahan PMKSA).
2. Ubah state akun menjadi suspended (ubah `Tunnel-Private-Group-Id` ke VLAN suspended) **tanpa** mengirim Disconnect.
3. Di klien, matikan lalu nyalakan WiFi (reconnect) — sebelum `Session-Timeout` habis.
4. Perhatikan: apakah hostapd melewati EAP dengan PMKSA dan mengembalikan klien ke VLAN 123 (= bocor), atau melakukan EAP penuh dan menerapkan VLAN suspended (= tidak bocor)?
5. Ulangi dengan Disconnect-Request dikirim lebih dulu, lalu reconnect. Di sini harus terjadi EAP penuh (Disconnect membersihkan entri PMKSA).

**Ukur dan catat** berapa lama klien tetap di VLAN aktif setelah state berubah. Angka inilah yang dipakai memutuskan apakah `disable_pmksa_caching` perlu dinyalakan — keputusan yang di ADR-0001 sengaja ditunda menunggu data ini.

---

## 6. Uji F — `Class` sampai ke `radacct`

Setelah sesi berjalan (uji B atau D), di VM:

```sql
SELECT username, callingstationid, class, acctstarttime, acctupdatetime
FROM radacct ORDER BY acctstarttime DESC LIMIT 5;
```

**Harus terlihat:** kolom `class` berisi `vlan=123;state=active`.

**Kalau kosong:** cek `acct_server` benar-benar aktif di AP. Hasil ini hanya memengaruhi kemampuan audit — billing tidak bergantung padanya (ADR-0005), jadi kegagalan di sini tidak memblokir apa pun.

---

## 7. Cara melaporkan hasil

Untuk setiap uji, catat empat hal: **perintah yang dijalankan**, **keluaran verbatim yang relevan**, **lulus/gagal**, dan **apa yang berubah di dokumen** bila gagal.

Hasilnya ditulis ke `docs/validation/0002-hasil-validasi-<tanggal>.md`, lalu:

- Perbarui klausul ADR yang terbantah (jangan diamkan).
- Tutup bagian yang relevan di issue #24, dan komentari #14 bila hasil uji E mengubah keputusan PMKSA.
- Bila uji D gagal, **hentikan jalur agent di #7** — membangun di atas `Session-Timeout` yang ternyata memutus klien akan menghasilkan suspend yang terlihat benar di dokumen tapi tidak di lapangan.

Bukti pendukung: `docs/research/0002-freeradius-provisioning-schema.md` (skema, hash, letak konfigurasi) dan `docs/research/0003-openwrt-hostapd-dynamic-vlan-das.md` (direktif, atribut, DAS, PMKSA).
