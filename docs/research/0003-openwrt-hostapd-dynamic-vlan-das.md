# Riset: OpenWrt + hostapd — VLAN dinamis, Session-Timeout, dan Disconnect-Request (RFC 5176) untuk enforce suspend

Riset ini menjawab sepuluh pertanyaan teknis yang menentukan apakah rancangan suspend/reaktivasi `RadiusAccount` lewat `Vlan` dinamis + Disconnect-Request dapat dijalankan pada `AccessPoint` OpenWrt, dan apakah asumsi teknis di `docs/adr/0001-suspend-via-dynamic-vlan.md` (ADR-0001) tetap sahih.

Dokumen ini memengaruhi dan dipengaruhi oleh:

- **ADR-0001** — suspend via `Vlan` dinamis + Disconnect/reauth (asumsi: butuh build `wpad`/hostapd **full** dengan `CONFIG_FULL_DYNAMIC_VLAN`; hostapd hanya mendukung **Disconnect-Request**, bukan CoA untuk mengubah `Vlan` sesi berdiri; `Session-Timeout` harus dikirim di **setiap** Access-Accept).
- **Issue #7** — kebijakan `Vlan` + `Session-Timeout` pada Access-Accept (atribut apa saja yang wajib ada di setiap Access-Accept).
- **Issue #8** — Disconnect-Request sebagai *fast-path* suspend/reactivation.
- **Issue #14** — suspend/aktif via `Vlan` pada `SubscriptionState`.
- **Issue #18** — `CaptivePortal` di `Vlan` suspended (identitas, isolasi, allowlist egress).

Konteks yang sudah dikonfirmasi (tidak diulang di sini): topologi **Shared L2**, sehingga isolasi `Vlan` per `Customer` bernilai; konfigurasi `Vlan` switch/gateway ditetapkan MVP sebagai **langkah manual terdokumentasi**.

**Versi yang dipakai sebagai rujukan** (dan alasannya):

| Komponen | Versi rujukan | Catatan |
|---|---|---|
| OpenWrt stabil | **25.12.5** (rilis 25.12.5 ada di `downloads.openwrt.org/releases/`, dan `packages-25.12`/`faillogs-25.12` sudah ada per 2026-09-27) | Paket berbasis **apk** (`wpad-2025.08.26~ca266cc2-r2.apk`). Juga diperiksa **24.10.8** (cabang `openwrt-24.10`) — paket berbasis opkg/ipk, masih dirawat. |
| hostapd/wpad | **2025.08.26** (versi yang dipaketkan OpenWrt 25.12.5) | Berkas konfigurasi varians OpenWrt 24.10 (`hostapd-full.config`, `hostapd-basic.config`) dipakai sebagai bukti *build-time flags*; kutipan `hostapd.conf` diambil dari cermin pohon hostapd upstream (berisi kode 802.11be/MLD → setara master 2025/2026). |
| FreeRADIUS | Dokumentasi & wiki proyek FreeRADIUS (halaman `radclient` diperbarui 21 Mei 2024 = FR 3.2.x; contoh `originate-coa` dari `master` = FR 4.0) | Perbedaan sintaks FR3 vs FR4 dicatat pada Pertanyaan 7. |
| RFC | RFC 2865, RFC 2868, RFC 3580, RFC 5176 | Semuanya disalin langsung dari `rfc-editor.org`. |

Aturan label bukti yang dipakai: **[TERVERIFIKASI]** = sumber primer (dokumentasi resmi/RFC/kode sumber upstream); **[INDIKATIF]** = hanya forum/posting/blog (bukan bukti); **[TIDAK TERVERIFIKASI]** = tidak ditemukan bukti. Batasan lingkungan: domain `w1.fi` (kanonik hostapd) tidak dapat diakses dari lingkungan riset ini, sehingga kutipan berkas upstream diambil dari cermin publik pohon hostapd dan dari salinan dokumen/berkas yang dipublikasikan proyek lain; setiap kutipan diberi catatan cara memverifikasi ulang secara lokal.

## Ringkasan

**ADR-0001 tetap valid, tetapi perlu satu penyempurnaan wajib dan satu tambahan desain.** Ketiga asumsi teknisnya terkonfirmasi oleh sumber primer: (a) `Vlan` dinamis hanya ada di varian build **full** (`wpad`, `wpad-mbedtls`, `wpad-openssl`, `wpad-wolfssl`, `hostapd`, `hostapd-mbedtls`, …), sedangkan varian **basic** bahkan mengompilasi keluar dukungan RADIUS (`CONFIG_NO_RADIUS=y`) dan `CONFIG_FULL_DYNAMIC_VLAN` tidak diaktifkan; (b) hostapd **tidak dapat** mengubah `Vlan` sesi berdiri lewat CoA-Request — satu-satunya `CoA-Request` yang diproses hostapd adalah *HS 2.0 Terms & Conditions filtering*, dan bahkan di build dengan `CONFIG_HS20=y` atribut `Tunnel-*` ditolak sebagai atribut tak didukung (Error-Cause 401); (c) karena `Session-Timeout` tidak dapat disuntikkan reaktif, ia memang harus menjadi kebijakan berdiri pada setiap Access-Accept.

Penyempurnaannya: **`Session-Timeout` saja tidak cukup untuk menjamin re-evaluasi `SubscriptionState` yang mulus.** hostapd hanya mengubah `Session-Timeout` menjadi **re-authentication 802.1X** (`sm->reAuthPeriod`) bila Access-Accept **juga** membawa `Termination-Action = RADIUS-Request (1)`; tanpa `Termination-Action`, hostapd pada saat timeout **mem-deautentikasi klien** (`"deauthenticated due to session timeout"`, `Acct-Terminate-Cause = Session-Timeout`). Untuk reaktivasi setelah `TopUp` dan untuk suspend tanpa memutus sesi, Access-Accept wajib memuat `Session-Timeout` **dan** `Termination-Action=1`. Tambahan desain: **celah identitas (Pertanyaan 9) punya jalur yang benar-benar dapat diverifikasi** — mode *RADIUS MAC ACL* (`macaddr_acl=2`) membuat hostapd mengirim `User-Name = MAC` + `User-Password = MAC` ke FreeRADIUS, dan hostapd **memakai `User-Name` dari Access-Accept sebagai identitas sesi** (untuk accounting) sekaligus memakai `Tunnel-*` sebagai `Vlan`. Artinya perangkat tanpa kredensial 802.1X tetap dapat ditempatkan ke `Vlan` suspended **dan** diatribusikan ke `RadiusAccount`/`Customer` — dengan trade-off MAC dapat dipalsukan dan kebutuhan SSID non-802.1X terpisah.

## Tabel ringkasan temuan

| # | Topik | Verdict | Basis bukti |
|---|---|---|---|
| 1 | Paket OpenWrt dengan `CONFIG_FULL_DYNAMIC_VLAN` | Varian **full** saja: `wpad`, `wpad-mbedtls`, `wpad-openssl`, `wpad-wolfssl` (dan `hostapd`/`hostapd-mbedtls`/…). Varian **basic** TIDAK mendukung (bahkan tanpa RADIUS). | OpenWrt Makefile + `files/hostapd-full.config` vs `files/hostapd-basic.config` + indeks paket 25.12.5 + wiki OpenWrt. [TERVERIFIKASI] |
| 2 | Direktif hostapd & padanan UCI | `dynamic_vlan` (0/1/2), `vlan_file`, `vlan_tagged_interface`, `vlan_bridge`, `vlan_naming` (0/1); UCI: `option dynamic_vlan`, `option vlan_file`, `option vlan_tagged_interface`, `option vlan_bridge`, `option vlan_naming`. Tidak ada `config vlan` UCI — pemetaan lewat berkas teks `vlan_file`. | `hostapd.conf` upstream + wiki OpenWrt `/etc/config/wireless` + `hostapd.uc` 24.10. [TERVERIFIKASI] |
| 3 | Atribut RADIUS penetapan `Vlan` | `Tunnel-Type=VLAN(13)` + `Tunnel-Medium-Type=802(6)` + `Tunnel-Private-Group-Id` (tipe 81, VLANID sebagai string). **Koreksi:** nilai VLAN(13) **tidak** didefinisikan di RFC 2868 (RFC 2868 hanya 1–12); sumbernya RFC 3580/IANA. Label "IEEE-802" adalah penamaan dictionary FreeRADIUS; teks RFC 3580 menulis `802`. | RFC 3580, RFC 2868 §6.1/§6.2, `src/radius/radius.h` hostapd. [TERVERIFIKASI] |
| 4 | Apakah `Vlan` harus ada lebih dulu | Di `AccessPoint`: dengan `CONFIG_FULL_DYNAMIC_VLAN` hostapd **membuat sendiri** bridge + interface `Vlan`; bridge itu **tidak punya uplink** kecuali `vlan_tagged_interface` diarahkan ke perangkat yang membawa trunk. Di switch/gateway (trunk, gateway portal) tetap **manual** — sesuai MVP. | `hostapd.conf` + komentar `hostapd-full.config` + DSA Mini-Tutorial OpenWrt. [TERVERIFIKASI] + [perlu uji lokal] untuk pengkabelan DSA |
| 5 | Efek `Session-Timeout` | Dua mode: dengan `Termination-Action=1` → 802.1X **re-auth** (tanpa putus, `Vlan` diterapkan ulang); tanpa `Termination-Action` → hostapd **memutus** klien dan mengeset `Acct-Terminate-Cause=Session-Timeout`. | Kode `src/ap/ieee802_1x.c` + `src/ap/sta_info.c`; RFC 2865 §5.27/§5.29; RFC 3580 Ch. 3.17. [TERVERIFIKASI] |
| 6 | Disconnect-Request (DAS, RFC 5176) | Benar: hostapd mendukung **Disconnect-Request**; CoA-Request **tidak** dapat mengubah `Vlan` (hanya HS 2.0 T&C filtering). DAS mati kecuali `radius_das_port`, `radius_das_client`, `radius_das_secret` (UCI: `dae_port`/`dae_client`/`dae_secret`) diisi. | `hostapd.conf`, `src/radius/radius_das.c`, `src/ap/hostapd.c`, RFC 5176. [TERVERIFIKASI] |
| 7 | Mengirim Disconnect dari aplikasi Go | Tiga jalur: `radclient disconnect` (paling cepat), implementasi paket RFC 5176 di Go, atau origination di dalam FreeRADIUS (`update disconnect` FR3 / `subrequest ::Disconnect-Request` + modul `radius.coa` FR4) dan aplikasi memicu via kanal terkendali. | Man page `radclient`, wiki FreeRADIUS, `raddb/sites-available/originate-coa` (master). [TERVERIFIKASI] untuk mekanisme; [INFERENSI PENELITI] untuk rekomendasi jalur |
| 8 | `CaptivePortal` di `Vlan` suspended | Gateway wajib: DHCP+DNS (dnsmasq), interface `Vlan` terpasang di zona firewall (agar menerima input), allowlist egress, intercept HTTP/DNS. Isolasi klien: `option isolate 1` + `option bridge_isolate 1` (OpenWrt), atau `per_sta_vif` hostapd. | Wiki OpenWrt (`/etc/config/wireless`, `/etc/config/dhcp`, DSA Mini-Tutorial), `hostapd.conf`. [TERVERIFIKASI] untuk opsi; detail aturan gateway [perlu uji lokal] |
| 9 | Celah identitas klien tersuspend | Dapat dijawab: (A) klien ber-802.1X → identitas EAP tersedia + accounting membawa `User-Name`/`Calling-Station-Id`; (B) MAC-auth (`macaddr_acl=2`) → hostapd kirim `User-Name=MAC`+`User-Password=MAC`, dan **memakai `User-Name` dari Access-Accept** sebagai identitas sesi + `Tunnel-*` sebagai `Vlan`. Klien tanpa kredensial **dan** MAC tak terdaftar tidak punya jalur terverifikasi → butuh OTP di `CaptivePortal`. | Kode `src/ap/ieee802_11_auth.c` + `src/ap/accounting.c` + `hostapd.conf`. [TERVERIFIKASI] |
| 10 | Bukti deployment nyata | Hanya forum/blog (mis. keluhan `wpad-basic` menolak `vlan_tagged_interface`, isu ath10k + dynamic VLAN, `radius_das` di build komunitas) — dipakai sebagai konteks saja. | Forum OpenWrt, ServerFault, blog. [INDIKATIF] |

## Detail per pertanyaan

### 1. Paket OpenWrt mana yang menyediakan hostapd dengan `CONFIG_FULL_DYNAMIC_VLAN`?

**Jawaban: hanya varian *full*.** Pada OpenWrt modem (dan cabang stabil 24.10/25.12) nama paketnya adalah `wpad`, `wpad-mbedtls`, `wpad-openssl`, `wpad-wolfssl` (untuk autentikator+supplicant) atau `hostapd`, `hostapd-mbedtls`, `hostapd-openssl`, `hostapd-wolfssl` (autentikator saja). **Tidak ada** paket bernama `wpad-full` pada OpenWrt terkini; kata "full" hanya muncul di internal Makefile (`VARIANT:=wpad-full-mbedtls`, `TITLE+= (mbedTLS full)`). [TERVERIFIKASI]

Bukti primer:

- `package/network/services/hostapd/Makefile` (cabang `openwrt-25.12` dan `main`):

  ```make
  define Package/wpad
  $(call Package/wpad/Default,$(1))
    TITLE+= (built-in full)
    VARIANT:=wpad-full-internal
  endef

  define Package/wpad-mbedtls
  $(call Package/wpad/Default,$(1))
    TITLE+= (mbedTLS full)
    VARIANT:=wpad-full-mbedtls
    DEPENDS+=+PACKAGE_wpad-mbedtls:libmbedtls
  endef

  define Package/wpad-basic-mbedtls
  $(call Package/wpad/Default,$(1))
    TITLE+= (mbedTLS, 11r, 11w)
    VARIANT:=wpad-basic-mbedtls
    DEPENDS+=+PACKAGE_wpad-basic-mbedtls:libmbedtls
  endef
  ```

  `VARIANT` inilah yang memilih berkas konfigurasi build: Makefile menyalin `./files/hostapd-$(CONFIG_VARIANT).config` menjadi `hostapd/.config` (`CONFIG_VARIANT` = `full`/`basic`/`mini`/`mesh`, dan `mesh` dipetakan ke `full`).

- `files/hostapd-full.config` (cabang `openwrt-24.10`) memuat baris aktif:

  ```
  # Enable support for fully dynamic VLANs. This enables hostapd to
  # automatically create bridge and VLAN interfaces if necessary.
  CONFIG_FULL_DYNAMIC_VLAN=y
  ```

- `files/hostapd-basic.config` (cabang `openwrt-24.10`) **tidak** mengaktifkannya, dan sekaligus mematikan RADIUS:

  ```
  # Remove support for RADIUS accounting
  CONFIG_NO_ACCOUNTING=y
  # Remove support for RADIUS
  CONFIG_NO_RADIUS=y
  # Remove support for VLANs
  #CONFIG_NO_VLAN=y
  # Enable support for fully dynamic VLANs. ...
  #CONFIG_FULL_DYNAMIC_VLAN=y
  ```

  Konsekuensinya lebih keras daripada yang diasumsikan ADR-0001: varian **basic tidak bisa** WPA2-Enterprise sama sekali (RADIUS dikompilasi keluar) dan juga tidak bisa `Vlan` dinamis.

- Wiki resmi OpenWrt (`/etc/config/wireless`) menyatakan eksplisit: *"Note: The vlan-related options above will be ignored or rejected by the default stripped down wpad-basic/hostapd-basic binaries. Like a RADIUS-based setup, these require the fully featured wpad/hostapd."*

- Indeks paket resmi (contoh arsitektur `x86_64`, rilis 25.12.5) — `https://downloads.openwrt.org/releases/25.12.5/packages/x86_64/base/`:

  ```
  wpad-2025.08.26~ca266cc2-r2.apk              817.3 KB
  wpad-basic-2025.08.26~ca266cc2-r2.apk        525.2 KB
  wpad-basic-mbedtls-2025.08.26~ca266cc2-r2.apk 551.4 KB
  wpad-mbedtls-2025.08.26~ca266cc2-r2.apk      938.1 KB
  ```

  Perhatikan ukuran: `wpad-basic-*` ±0,55 MB vs `wpad`/`wpad-mbedtls` ±0,82–0,94 MB — konsisten dengan "varian basic adalah binary yang dipreteli". Paket `hostapd*`/`wpad*` berada di feed **base** per arsitektur target.

**Cara memverifikasi lokal** (di `AccessPoint` nyata): `apk list --installed | grep -i wpad` (rilis 25.12+, paket `.apk`) atau `opkg list-installed | grep -i wpad` (rilis 24.10, paket `.ipk`); lalu pastikan binary yang terpasang adalah varian full **dan** berkas `/var/run/hostapd-*.conf` yang dihasilkan netifd memuat baris `dynamic_vlan=` dan `vlan_file=`. Catatan: pada varian basic, opsi yang tidak dikenal muncul sebagai error/peringatan hostapd di syslog (`hostapd: ... unknown configuration item`) — bukan gagal senyap pada semua opsi.

### 2. Konfigurasi hostapd untuk dynamic VLAN dan padanannya di UCI OpenWrt

**Direktif hostapd (kutipan `hostapd.conf` upstream).** Blok mode dinamis:

```
# Dynamic VLAN mode; allow RADIUS authentication server to decide which VLAN
# is used for the stations. This information is parsed from following RADIUS
# attributes based on RFC 3580 and RFC 2868: Tunnel-Type (value 13 = VLAN),
# Tunnel-Medium-Type (value 6 = IEEE 802), Tunnel-Private-Group-ID (value
# VLANID as a string). Optionally, the local MAC ACL list (accept_mac_file) can
# be used to set static client MAC address to VLAN ID mapping.
# Dynamic VLAN mode is also used with VLAN ID assignment based on WPA/WPA2
# passphrase from wpa_psk_file or vlan_id parameter from sae_password.
# 0 = disabled (default); only VLAN IDs from accept_mac_file will be used
# 1 = optional; use default interface if RADIUS server does not include VLAN ID
# 2 = required; reject authentication if RADIUS server does not include VLAN ID
#dynamic_vlan=0
```

Blok `vlan_file`:

```
# VLAN interface list for dynamic VLAN mode is read from a separate text file.
# This list is used to map VLAN ID from the RADIUS server to a network
# interface. Each station is bound to one interface in the same way as with
# multiple BSSIDs or SSIDs. Each line in this text file is defining a new
# interface and the line must include VLAN ID and interface name separated by
# white space (space or tab).
# If no entries are provided by this file, the station is statically mapped
# to <bss-iface>.<vlan-id> interfaces.
# Each line can optionally also contain the name of a bridge to add the VLAN to
#vlan_file=/etc/hostapd.vlan
```

Blok `vlan_tagged_interface` / `vlan_bridge` / `vlan_naming`:

```
# Interface where 802.1q tagged packets should appear when a RADIUS server is
# used to determine which VLAN a station is on. hostapd creates a bridge for
# each VLAN. Then hostapd adds a VLAN interface (associated with the interface
# indicated by 'vlan_tagged_interface') and the appropriate wireless interface
# to the bridge.
#vlan_tagged_interface=eth0

# Bridge (prefix) to add the wifi and the tagged interface to. This gets the
# VLAN ID appended. It defaults to brvlan%d if no tagged interface is given
# and br%s.%d if a tagged interface is given, provided %s = tagged interface
# and %d = VLAN ID.
#vlan_bridge=brvlan

# When hostapd creates a VLAN interface on vlan_tagged_interfaces, it needs
# to know how to name it.
# 0 = vlan<XXX>, e.g., vlan1
# 1 = <vlan_tagged_interface>.<XXX>, e.g. eth0.1
#vlan_naming=0
```

**Format baris `vlan_file`.** Menurut komentar upstream: tiap baris mendefinisikan satu interface, dengan `VLAN ID` dan `nama interface` dipisahkan whitespace; **kolom ketiga opsional** = nama bridge. Wiki OpenWrt memberi contoh yang lebih lengkap (termasuk entri *wildcard* yang hanya jalan dengan full dynamic VLAN, karena interface dibuat/dihapus saat runtime):

```
# VLAN ID to network interface mapping
1 vlan1
2 vlan2
3 vlan3
100 guest
# Optional wildcard entry matching all VLAN IDs. The first "#" in the interface name will be replaced with the VLAN ID.
# The network interfaces are created and removed dynamically when necessary.
* vlan#
# Optional third parameter to override the bridge name
101 vlan100 bridge_name
```

Konsekuensi praktis untuk Cuanku: dengan entri wildcard `* vlan#`, seluruh `Vlan` (per-`Customer` dan `Vlan` suspended) tidak perlu didaftarkan satu per satu di `AccessPoint` — ID dari FreeRADIUS otomatis dipetakan.

**Padanan UCI di `/etc/config/wireless`.** Wiki OpenWrt mendokumentasikan opsi berikut (nama opsi UCI, default, dan artinya):

| Opsi UCI | Default | Arti |
|---|---|---|
| `dynamic_vlan` | `0` | Dynamic VLAN assignment |
| `vlan_naming` | `1` | 0 = `vlan<id>`; 1 = `<vlan_tagged_interface>.<id>` |
| `vlan_tagged_interface` | _(none)_ | interface tempat paket 802.1q bertanda muncul |
| `vlan_bridge` | _(none)_ | prefiks bridge (`brvlan<id>` bila tanpa tagged interface; `br<tagged_iface>.<id>` bila ada tagged interface) |
| `vlan_file` | _(none)_ | path absolut berkas pemetaan ID `Vlan` → interface (lokasi lazim `/etc/hostapd.vlan`) |
| `dae_client` | _(none)_ | klien Dynamic Authorization Extension (pengirim Disconnect-Request/CoA-Request) |
| `dae_port` | `3799` | port UDP server DAE |
| `dae_secret` | _(none)_ | shared secret DAE |
| `isolate`, `bridge_isolate` | `0` | isolasi klien nirkabel (lihat Pertanyaan 8) |

**Tidak ada** seksi UCI bertipe `config vlan`. Pemetaan `Vlan`→interface dilakukan sepenuhnya oleh berkas teks yang ditunjuk `vlan_file`. Ini penting untuk operasional: berkas itu bagian dari konfigurasi `AccessPoint`, sedangkan **daftar `Vlan` yang sah** (per-`Customer` + suspended) datang dari FreeRADIUS.

Catatan implementasi OpenWrt (bukti kode): `package/network/config/wifi-scripts/files/lib/netifd/wireless/mac80211.sh` (24.10) membaca opsi `vlan_file` per `wifi-iface` dan, bila kosong, memanggil `hostapd_set_vlan`; sementara `package/network/services/hostapd/files/hostapd.uc` (24.10) memperlakukan `vlan_file` sebagai `file_fields` — artinya path harus ada saat konfigurasi dimuat, dan hash isi berkas dipakai untuk mendeteksi perubahan konfigurasi. [TERVERIFIKASI]

Contoh konkret (dapat dipakai langsung; sesuaikan nama interface/`Vlan`):

```uci
# /etc/config/wireless
config wifi-iface 'cuanku_enterprise'
	option device 'radio0'
	option mode 'ap'
	option ssid 'Cuanku'
	option encryption 'wpa2'          # WPA2 Enterprise (CCMP)
	option auth_server '10.20.0.10'
	option auth_port '1812'
	option auth_secret 'SECRET-AUTH-AP01'
	option acct_server '10.20.0.10'
	option acct_port '1813'
	option acct_secret 'SECRET-ACCT-AP01'
	option nasid 'ap-customer-0001'
	option ownip '10.20.0.21'         # IP NAS = IP AP di jalur RADIUS

	# --- VLAN dinamis ---
	option dynamic_vlan '2'           # 2 = wajib: Access-Accept tanpa Tunnel-* ditolak
	option vlan_file '/etc/hostapd.vlan'
	option vlan_naming '1'            # interface: <vlan_tagged_interface>.<id>
	option vlan_tagged_interface 'br-lan'
	# option vlan_bridge 'brvlan'     # opsional: paksa prefiks bridge

	# --- Disconnect-Request (DAS, RFC 5176) ---
	option dae_client '10.20.0.10'    # WAJIB: hostapd menolak DAS tanpa client addr
	option dae_port '3799'
	option dae_secret 'SECRET-DAS-AP01'

	# --- isolasi klien (lihat Pertanyaan 8) ---
	option isolate '1'
	option bridge_isolate '1'
```

```text
# /etc/hostapd.vlan  (dipakai oleh option vlan_file)
# <VLAN ID> <interface> [bridge]
10 br-lan.10
11 br-lan.11
*  br-lan.#
```

Setara dalam `hostapd.conf` (untuk pengujian manual / `eapol_test` tanpa OpenWrt):

```ini
interface=wlan0
driver=nl80211
ssid=Cuanku
hw_mode=g
channel=6
ieee8021x=1
eap_server=0
auth_server_addr=10.20.0.10
auth_server_port=1812
auth_server_shared_secret=SECRET-AUTH-AP01
acct_server_addr=10.20.0.10
acct_server_port=1813
acct_server_shared_secret=SECRET-ACCT-AP01
own_ip_addr=10.20.0.21
nas_identifier=ap-customer-0001
wpa=2
wpa_key_mgmt=WPA-EAP
rsn_pairwise=CCMP

dynamic_vlan=2
vlan_file=/etc/hostapd.vlan
vlan_tagged_interface=br-lan
vlan_naming=1

radius_das_port=3799
radius_das_client=10.20.0.10 SECRET-DAS-AP01
```

**Peringatan penting untuk OpenWrt:** pada `wifi-iface` yang terhubung ke `option network 'lan'`, hostapd pada dasarnya menempelkan STA ke bridge `br-lan`. Ketika `dynamic_vlan` aktif, hostapd memindahkan STA ke bridge per-`Vlan` (`br-lan.<id>`/`brvlan<id>`), yang **hanya punya uplink bila `vlan_tagged_interface` menunjuk perangkat yang membawa trunk**. Menyetel nilai `vlan_tagged_interface` yang salah membuat klien terisolasi total (tidak sampai gateway/`CaptivePortal`). Kombinasi nilai yang tepat untuk perangkat DSA **harus diuji di lapangan** (lihat Risiko).

### 3. Atribut RADIUS untuk menempatkan klien di `Vlan`

Tiga atribut (trio RFC 3580), dikutip verbatim dari RFC 3580:

```
   For use in VLAN assignment, the following tunnel attributes are used:

      Tunnel-Type=VLAN (13)
      Tunnel-Medium-Type=802
      Tunnel-Private-Group-ID=VLANID

   Note that the VLANID is 12-bits, taking a value between 1 and 4094,
   inclusive.  Since the Tunnel-Private-Group-ID is of type String as
   defined in [RFC2868], for use with IEEE 802.1X, the VLANID integer
   value is encoded as a string.  When Tunnel attributes are sent, it is
   necessary to fill in the Tag field.
```

Penomoran tipe atribut (dari RFC 2868): `Tunnel-Type` = tipe **64**, `Tunnel-Medium-Type` = tipe **65**, `Tunnel-Private-Group-ID` = tipe **81** (`"Type 81 for Tunnel-Private-Group-ID. Length >= 3"`).

**Koreksi yang diminta (ada perbedaan penomoran nilai):** nilai `VLAN = 13` **bukan** berasal dari RFC 2868. RFC 2868 §6.1 menyatakan:

> "Values 1-12 of the Tunnel-Type Attribute are defined in Section 5.1; the remaining values are available for assignment by the IANA with IETF Consensus [16]."

Jadi `Tunnel-Type=13` (VLAN) berasal dari RFC 3580 (yang menetapkannya untuk penggunaan IEEE 802.1X), bukan dari RFC 2868. Sebaliknya nilai medium `6` memang ada di RFC 2868 §5.2 dengan nama **"802"**:

> "6 802 (includes all 802 media plus Ethernet "canonical format")"

Label `IEEE-802` yang biasa dipakai di konfigurasi FreeRADIUS adalah **nama nilai di dictionary FreeRADIUS**, bukan teks RFC. Sisi hostapd mengonfirmasi pemetaan numerik tersebut dari kode sumber `src/radius/radius.h`:

```c
/* Tunnel-Type */
#define RADIUS_TUNNEL_TYPE_PPTP 1
#define RADIUS_TUNNEL_TYPE_L2TP 3
#define RADIUS_TUNNEL_TYPE_IPIP 7
#define RADIUS_TUNNEL_TYPE_GRE 10
#define RADIUS_TUNNEL_TYPE_VLAN 13

/* Tunnel-Medium-Type */
#define RADIUS_TUNNEL_MEDIUM_TYPE_IPV4 1
#define RADIUS_TUNNEL_MEDIUM_TYPE_IPV6 2
#define RADIUS_TUNNEL_MEDIUM_TYPE_802 6
```

Catatan tambahan yang relevan untuk desain:

- hostapd mem-parsing `Tunnel-Type` + `Tunnel-Medium-Type` + `Tunnel-Private-Group-Id` melalui `radius_msg_get_vlanid()` yang mendukung **penandaan (tag) untuk banyak VLAN** dan penyortiran VLAN bertag; jumlah tag maksimum = `RADIUS_TUNNEL_TAGS 32`. Untuk MVP cukup satu `Vlan` tak bertag (aktif per `Customer`, atau suspended bersama).
- hostapd juga mengenali atribut **`Egress-VLANID`** (ada di tabel atribut `src/radius/radius.c`), tetapi jalur penetapan `Vlan` yang terdokumentasi di `hostapd.conf` dan dipakai driver adalah trio Tunnel-*. **Rekomendasi: jangan mengandalkan `Egress-VLANID` tanpa uji lokal.**
- Varian alternatif mekanisme VLAN di hostapd (bukan atribut RADIUS, tetapi relevan untuk fallback PSK per-`Customer` di ADR-0004): penetapan VLAN per entri `wpa_psk_file` dan parameter `vlan_id` pada `sae_password` — keduanya disebut eksplisit di komentar `dynamic_vlan` yang dikutip pada Pertanyaan 2.

Contoh balasan FreeRADIUS (perlu diverifikasi namanya terhadap dictionary lokal — lihat cara verifikasi di bawah):

```text
# raddb/mods-config/sql/main/... atau radreply:
# Di FR3, nilai "VLAN" pada Tunnel-Type berasal dari dictionary.rfc2868
Cuanku-aktif-<CustomerId>  Reply-Message := "Vlan aktif Customer"
                           Tunnel-Type := VLAN
                           Tunnel-Medium-Type := IEEE-802
                           Tunnel-Private-Group-Id := "10"
                           Session-Timeout := 900
                           Termination-Action := RADIUS-Request
                           Acct-Interim-Interval := 300

Cuanku-suspended          Tunnel-Type := VLAN
                           Tunnel-Medium-Type := IEEE-802
                           Tunnel-Private-Group-Id := "900"
                           Session-Timeout := 600
                           Termination-Action := RADIUS-Request
                           Acct-Interim-Interval := 300
```

**Cara verifikasi lokal nama nilai:** `grep -iE 'Tunnel-(Type|Medium-Type)' /usr/share/freeradius/dictionary.rfc2868` (lokasi dapat berbeda per distro). Nilai numeriknya sudah dikonfirmasi ke kode hostapd; yang di sini belum saya verifikasi adalah nama simbolik di dictionary FreeRADIUS (domain kanonik dictionary FreeRADIUS tidak berhasil diambil dari lingkungan riset ini) → **[TIDAK TERVERIFIKASI]** untuk nama simbolik, **[TERVERIFIKASI]** untuk angka 13/6 dan untuk semantik atribut.

### 4. Apakah `Vlan` harus sudah ada di bridge/switch sebelum klien connect?

**Di `AccessPoint`: tidak wajib**, asalkan build full (`CONFIG_FULL_DYNAMIC_VLAN=y`) dan entri `vlan_file` cocok. Komentar berkas konfigurasi build OpenWrt menyatakannya langsung:

> "# Enable support for fully dynamic VLANs. This enables hostapd to automatically create bridge and VLAN interfaces if necessary."

dan komentar `hostapd.conf` menambahkan bahwa hostapd "creates a bridge for each VLAN. Then hostapd adds a VLAN interface (associated with the interface indicated by 'vlan_tagged_interface') and the appropriate wireless interface to the bridge." Bila `vlan_file` tidak memuat entri untuk ID tersebut, hostapd memakai pola statis `<bss-iface>.<vlan-id>` (harus sudah ada).

**Di switch/gateway: ya, wajib manual.** Bridge yang dibuat hostapd tidak otomatis terhubung ke trunk upstream — ia hanya berisi interface `Vlan` bertag yang dibuat di atas `vlan_tagged_interface` plus interface nirkabel. Agar `Vlan` aktif per `Customer` dan `Vlan` suspended dapat mencapai gateway (dan `CaptivePortal`), port uplink `AccessPoint` harus bertag untuk semua ID tersebut. Ini tepat seperti yang sudah ditetapkan MVP: "konfigurasi `Vlan` switch/gateway = langkah manual terdokumentasi".

Sisi gateway/router OpenWrt (DSA), dari wiki resmi OpenWrt (DSA Mini-Tutorial) — contoh VLAN-aware bridge, termasuk port trunk bertag (`:t`) dan port akses untagged dengan PVID (`:u*`):

```uci
# /etc/config/network (contoh DSA; hanya kerangka)
config device 'switch'
	option name 'switch'
	option type 'bridge'
	list ports 'lan1'
	list ports 'lan2'
	list ports 'lan3'
	list ports 'lan4'

config bridge-vlan 'lan_vlan'
	option device 'switch'
	option vlan '10'                  # Vlan aktif Customer
	list ports 'lan1:t'               # uplink trunk (tag)
	list ports 'lan2:t'

config bridge-vlan
	option device 'switch'
	option vlan '900'                 # Vlan suspended bersama
	list ports 'lan1:t'
	list ports 'lan2:t'

config interface 'customer10'
	option proto 'none'               # L2 murni; gateway portal ditangani interface lain
	option device 'switch.10'

config interface 'suspended'
	option proto 'static'
	option device 'switch.900'
	option ipaddr '10.99.0.1'
	option netmask '255.255.255.0'
```

Dua hal yang ditekankan wiki dan penting untuk `CaptivePortal`:

1. *"That interface must be associated with a firewall zone (or rules) to accept input."* — interface `Vlan` yang tidak dimasukkan ke zona firewall tidak akan menerima trafik masuk (portal tidak dapat diakses).
2. Port yang membawa `Vlan` per-`Customer` tidak perlu `config interface` (boleh `proto 'none'`), tetapi interface untuk `Vlan` suspended **wajib** punya alamat IP + zona firewall.

Untuk `AccessPoint` yang memakai DSA, `vlan_tagged_interface` harus menunjuk perangkat yang benar-benar membawa trunk ke gateway (mis. bridge `br-lan` yang port uplink-nya bertag, atau port fisik uplink). **Nilai yang tepat bergantung topologi per `AccessPoint` → perlu uji lokal** (lihat Risiko & Langkah Berikutnya).

### 5. Apa yang sebenarnya dilakukan `Session-Timeout` pada hostapd?

**Ada dua perilaku berbeda, ditentukan oleh `Termination-Action`.** Kutipan kode `src/ap/ieee802_1x.c` (fungsi `ieee802_1x_receive_auth`), pada cabang `RADIUS_CODE_ACCESS_ACCEPT`:

```c
	session_timeout_set = !radius_msg_get_attr_int32(msg, RADIUS_ATTR_SESSION_TIMEOUT, &session_timeout);
	if (radius_msg_get_attr_int32(msg, RADIUS_ATTR_TERMINATION_ACTION, &termination_action))
		termination_action = RADIUS_TERMINATION_ACTION_DEFAULT;
...
	sta->session_timeout_set = !!session_timeout_set;
	os_get_reltime(&sta->session_timeout);
	sta->session_timeout.sec += session_timeout;
	/* RFC 3580, Ch. 3.17 */
	if (session_timeout_set && termination_action == RADIUS_TERMINATION_ACTION_RADIUS_REQUEST)
		sm->reAuthPeriod = session_timeout;
	else if (session_timeout_set)
		ap_sta_session_timeout(hapd, sta, session_timeout);
	else
		ap_sta_no_session_timeout(hapd, sta);
```

Jadi:

- **Dengan `Termination-Action = RADIUS-Request (1)`**: hostapd menyetel periode re-authentication di state machine EAPOL (`sm->reAuthPeriod`) → **802.1X re-authentication**, bukan pemutusan. Pada re-auth, `Vlan` diterapkan ulang dari Access-Accept baru (lihat di bawah).
- **Tanpa `Termination-Action` (default 0)**: hostapd memanggil `ap_sta_session_timeout()`, yang menjadwalkan `ap_handle_session_timer()` di `src/ap/sta_info.c`:

  ```c
      hostapd_drv_sta_deauth(hapd, sta->addr, WLAN_REASON_PREV_AUTH_NOT_VALID);
      mlme_deauthenticate_indication(hapd, sta, WLAN_REASON_PREV_AUTH_NOT_VALID);
      hostapd_logger(hapd, sta->addr, HOSTAPD_MODULE_IEEE80211, HOSTAPD_LEVEL_INFO,
                     "deauthenticated due to session timeout");
      sta->acct_terminate_cause = RADIUS_ACCT_TERMINATE_CAUSE_SESSION_TIMEOUT;
      ap_free_sta(hapd, sta);
  ```

  Artinya klien **diputus** dan harus mengasosiasi ulang (dan melakukan EAP baru) untuk kembali online. Untuk desain suspend/reaktivasi ini masih memberi re-evaluasi, tetapi melalui pemutusan sesi — bukan transisi `Vlan` yang mulus.

Semantik RFC yang mendasari (dikutip dari RFC 2865):

- §5.27 `Session-Timeout`: *"This Attribute sets the maximum number of seconds of service to be provided to the user before termination of the session or prompt."*
- §5.29 `Termination-Action`: *"This Attribute indicates what action the NAS should take when the specified service is completed. It is only used in Access-Accept packets. … 0 Default / 1 RADIUS-Request. If the Value is set to RADIUS-Request, upon termination of the specified service the NAS MAY send a new Access-Request to the RADIUS server, including the State attribute if any."*

Komentar di kode hostapd (`/* RFC 3580, Ch. 3.17 */`) menunjukkan implementasi ini mengikuti panduan IEEE 802.1X + RADIUS di RFC 3580.

**`Vlan` memang diterapkan ulang saat re-auth.** Sebelum penanganan timeout pada fungsi yang sama, hostapd memperbarui `Vlan` dari Access-Accept yang baru lalu me-rebind interface STA:

```c
	if (hapd->conf->dynamic_vlan != DYNAMIC_VLAN_DISABLED && ieee802_1x_update_vlan(msg, hapd, sta) < 0)
		break;
	if (sta->vlan_id > 0) {
		hostapd_logger(hapd, sta->addr, HOSTAPD_MODULE_RADIUS, HOSTAPD_LEVEL_INFO,
		               "VLAN ID %d", sta->vlan_id);
	}
	if ((sta->flags & WLAN_STA_ASSOC) && ap_sta_bind_vlan(hapd, sta) < 0)
		break;
```

Dengan kata lain, mekanisme yang menjamin `SubscriptionState` dievaluasi ulang secara berkala **ada**, tetapi baru aktif bila Access-Accept memuat `Session-Timeout` **dan** `Termination-Action=1`.

Satu catatan penting (lihat Risiko): hostapd menyimpan entri **PMKSA cache** dengan masa hidup = sisa `Session-Timeout` (`wpa_auth_pmksa_add(..., session_timeout, sta->eapol_sm)`), dan entri PMKSA tersebut menyimpan `struct vlan_description *vlan_desc` (`src/ap/pmksa_cache_auth.h`). Saat klien reconnect dengan PMKSA yang masih valid, hostapd melewati EAP (`"PMK from PMKSA cache - skip IEEE 802.1X/EAP"`) lalu memanggil `pmksa_cache_to_eapol_data()` dan `ap_sta_bind_vlan()` — jadi `Vlan` lama dapat terpakai kembali dalam rentang sisa waktu tersebut. Mitigasi sudah ada di mekanisme DAS: `hostapd_das_disconnect()` memanggil `wpa_auth_pmksa_remove(hapd->wpa_auth, sta->addr)` **sebelum** deautentikasi, sehingga jalur cepat Disconnect membersihkan cache PMKSA dan memaksa EAP penuh pada reconnect berikutnya.

### 6. Disconnect-Request / DAS (RFC 5176) — direktif, parameter, dan batas CoA

**Direktif konfigurasi hostapd (`hostapd.conf` upstream, verbatim):**

```
# Dynamic Authorization Extensions (RFC 5176)
# This mechanism can be used to allow dynamic changes to user session based on
# commands from a RADIUS server (or some other disconnect client that has the
# needed session information). For example, Disconnect message can be used to
# request an associated station to be disconnected.
#
# This is disabled by default. Set radius_das_port to non-zero UDP port
# number to enable.
#radius_das_port=3799
#
# DAS client (the host that can send Disconnect/CoA requests) and shared secret
# Format: <IP address> <shared secret>
# IP address 0.0.0.0 can be used to allow requests from any address.
#radius_das_client=192.168.1.123 shared secret here
#
# DAS Event-Timestamp time window in seconds
#radius_das_time_window=300
#
# DAS require Event-Timestamp
#radius_das_require_event_timestamp=1
#
# DAS require Message-Authenticator
#radius_das_require_message_authenticator=1
```

Nama-nama di atas **adalah** nama yang benar (tidak ada alias lain); di UCI OpenWrt padanannya `dae_port`, `dae_client`, `dae_secret` (dan wiki mengonfirmasi default `dae_port=3799`). Tiga syarat berpasangan: kode `radius_das_init()` langsung menolak inisialisasi bila salah satu kosong —

```c
	if (conf->port == 0 || conf->shared_secret == NULL || conf->client_addr == NULL)
		return NULL;
```

Sehingga **`radius_das_client` (UCI `dae_client`) bersifat wajib**, bukan opsional: tanpa alamat klien DAS, fitur ini tidak aktif sama sekali. Ini juga mengonfirmasi mitigasi keamanan di ADR-0001 (shared secret kuat & unik per `AccessPoint`, DAS hanya dari sumber tepercaya).

**Parameter yang harus dikirim.** Daftar atribut yang diizinkan oleh hostapd untuk Disconnect-Request (`src/radius/radius_das.c`, `radius_das_disconnect()`): `User-Name`, `NAS-IP-Address`, `Calling-Station-Id`, `NAS-Identifier`, `Acct-Session-Id`, `Acct-Multi-Session-Id`, `Event-Timestamp`, `Chargeable-User-Identity`, `Vendor-Specific` (HS 2.0), `NAS-IPv6-Address`. Atribut di luar daftar ini ditolak dengan Error-Cause **401** (`"DAS: Unsupported attribute ... in Disconnect-Request"`). Pencocokan sesi di `hostapd_das_find_sta()` (`src/ap/hostapd.c`):

- `Calling-Station-Id` → dicocokkan dengan MAC STA;
- `Acct-Session-Id` → 16 karakter heksadesimal (dibandingkan dengan `%016llX` dari `sta->acct_session_id`);
- `Acct-Multi-Session-Id`, `Chargeable-User-Identity`;
- `User-Name` → dicocokkan dengan identitas 802.1X (`ieee802_1x_get_identity()`);
- `NAS-Identifier`/`NAS-IP-Address`/`NAS-IPv6-Address` harus cocok dengan konfigurasi BSS (`hostapd_das_nas_mismatch()`), jika tidak → Error-Cause **403**;
- Bila tidak ada atribut identifikasi sesi sama sekali, permintaan **ditolak** (`"RADIUS DAS: No session identification attributes included"`).

RFC 5176 (port & identifikasi, verbatim): *"A Disconnect-Request packet is sent by the Dynamic Authorization Client in order to terminate user session(s) on a NAS and discard all associated session context. The Disconnect-Request packet is sent to UDP port 3799, and identifies the NAS as well as the user session(s) to be terminated by inclusion of the identification attributes described in Section 3."* Daftar atribut identifikasi RFC 5176 mencakup `User-Name` (1), `NAS-IP-Address` (4), `Called-Station-Id` (30), `Calling-Station-Id` (31), `Acct-Session-Id` (44), `Acct-Multi-Session-Id` (50), `NAS-Identifier` (32), `Framed-IP-Address` (8), `Vendor-Specific` (26).

Tentang `Message-Authenticator`, RFC 5176 §3.4 tidak mewajibkannya: *"The Message-Authenticator Attribute MAY be used to authenticate and integrity-protect CoA-Request, CoA-ACK, CoA-NAK, Disconnect-Request, Disconnect-ACK, and Disconnect-NAK packets in order to prevent spoofing. A Dynamic Authorization Server receiving a CoA-Request or Disconnect-Request with a Message-Authenticator Attribute present MUST calculate the correct value of the Message-Authenticator and silently discard [packet dengan nilai salah]."* Artinya: mengirimkannya **disarankan** dan selalu aman (bila dikirim, hostapd memverifikasi); hostapd menyediakan saklar `radius_das_require_message_authenticator`. Rekomendasi implementasi: **selalu sertakan `Message-Authenticator`** yang benar.

**Konfirmasi: CoA-Request untuk mengubah `Vlan` sesi berdiri TIDAK didukung.** Kode `src/ap/hostapd.c` mendaftarkan callback:

```c
	das_conf.disconnect = hostapd_das_disconnect;
	das_conf.coa = hostapd_das_coa;
```

dan `hostapd_das_coa()` hanya ada di dalam `#ifdef CONFIG_HS20`, dengan isi tunggal:

```c
static enum radius_das_res hostapd_das_coa(void *ctx, struct radius_das_attrs *attr)
{
	...
	if (attr->hs20_t_c_filtering) {
		if (attr->hs20_t_c_filtering[0] & BIT(0)) {
			wpa_printf(MSG_DEBUG, "HS 2.0: Unexpected Terms and Conditions filtering required in CoA-Request");
			return RADIUS_DAS_COA_FAILED;
		}
		hs20_t_c_filtering(hapd, sta, 0);
	}
	return RADIUS_DAS_SUCCESS;
}
#else /* CONFIG_HS20 */
#define hostapd_das_coa NULL
#endif /* CONFIG_HS20 */
```

Jadi CoA-Request hanya dapat mematikan/menyalakan *HS 2.0 Terms & Conditions filtering*. Bahkan di build dengan `CONFIG_HS20=y` (OpenWrt `hostapd-full.config` **mengaktifkan** `CONFIG_INTERWORKING=y` dan `CONFIG_HS20=y`), permintaan CoA yang membawa `Tunnel-Type`/`Tunnel-Private-Group-Id` akan ditolak di lapisan parser sebagai atribut tak diizinkan (Error-Cause **401**), dan CoA tanpa atribut perubahan yang didukung ditolak dengan Error-Cause **402** (`"No supported authorization change attribute in CoA-Request"`). Pada build tanpa HS 2.0, `das->coa == NULL` sehingga hostapd menjawab **405** (`"DAS: CoA not supported"`). **Asumsi ADR-0001 pada titik ini TERKONFIRMASI.**

**Apakah build wpad OpenWrt mengaktifkan DAS?** Ya, dengan syarat varian full (RADIUS tidak dikompilasi keluar): kode DAS di `src/ap/hostapd.c` berada di dalam `#ifndef CONFIG_NO_RADIUS`, dan varian basic menetapkan `CONFIG_NO_RADIUS=y`. **[TERVERIFIKASI]** dari berkas konfigurasi build. Verifikasi lokal: setelah mengisi `dae_*`, kirim satu Disconnect-Request dari gateway dan pastikan ada `Disconnect-ACK` + baris log hostapd `"RADIUS DAS: Found a matching session ... - disconnecting"`; periksa juga baris `radius_das_port=`/`radius_das_client=` pada `/var/run/hostapd-*.conf` hasil netifd.

Response yang harus ditangani aplikasi: `Disconnect-ACK` (kode 41) atau `Disconnect-NAK` (kode 42) dengan `Error-Cause` (401 atribut tak didukung, 402/405 CoA, 403 NAS tidak cocok, 503 sesi tidak ditemukan, 508 banyak sesi cocok). Kode 503 adalah kasus normal saat sesi sudah berakhir → idempotensi job harus memperlakukannya sebagai sukses-logis.

### 7. Sisi FreeRADIUS: mengirim Disconnect-Request secara programatik

Tiga jalur yang semuanya didukung dokumentasi proyek FreeRADIUS **[TERVERIFIKASI untuk mekanisme; pemilihan jalur = INFERENSI PENELITI]**:

**(a) `radclient` (paling cepat, FR3 & FR4).** Man page resmi: `radclient [opsi] server {acct|auth|status|coa|disconnect|auto} secret`, dan *"For coa and disconnect packets, port 3799 is used."* Atribut istimewa yang bisa disisipkan di daftar atribut: `Packet-Dst-IP-Address`, `Packet-Dst-Port`, `Packet-Type` (dengan ini `type` bisa diberi nilai `auto`). Opsi berguna: `-r num_retries`, `-t timeout`, `-S shared_secret_file` (menghindari secret terlihat di `ps`/argv), `-x` debug, `-q` quiet, `-s` ringkasan.

Official wiki FreeRADIUS memberi contoh yang dapat dipakai langsung:

```bash
# echo "Acct-Session-Id=D91FE8E51802097" > packet.txt
# echo "User-Name=somebody" >> packet.txt
# echo "NAS-IP-Address=10.0.0.1" >> packet.txt

# cat packet.txt | radclient -x 10.0.0.1:3799 disconnect 'secret'
Sending Disconnect-Request of id 214 to 10.0.0.1 port 3799
      Acct-Session-Id = "D91FE8E51802097"
      User-Name = "somebody"
      NAS-IP-Address = 10.0.0.1
rad_recv: Disconnect-ACK packet from host 10.0.0.1 port 3799, id=214, length=20
```

Untuk kasus Cuanku, paket yang paling andal adalah kombinasi atribut yang dicocokkan hostapd: `User-Name` (username `RadiusAccount`), `Acct-Session-Id` (16 hex), `Calling-Station-Id` (MAC), `NAS-IP-Address` (IP `AccessPoint` di jalur RADIUS) — plus `Message-Authenticator` yang benar.

**(b) Origination di dalam FreeRADIUS (disarankan agar shared secret tidak keluar dari server RADIUS).** Wiki meyebut: *"FreeRADIUS server (radiusd) supports sending Disconnect-Request via the `update coa` and `update disconnect` Unlang statements."* Untuk FR4, contoh resmi `raddb/sites-available/originate-coa` menunjukkan pola `subrequest`:

```text
recv Accounting-Request {
	subrequest ::Disconnect-Request {
		request.User-Name := parent.request.User-Name
		request.Acct-Session-Id := parent.request.Acct-Session-Id
		request.NAS-Identifier := parent.request.NAS-Identifier
		request.NAS-IP-Address := parent.request.NAS-IP-Address
		...
		radius.coa
	}
}
```

dengan catatan penting dari berkas contoh itu sendiri: *"NOTE: This functionality is configured differently from v3."* dan peringatan bahwa atribut yang dibutuhkan bergantung vendor NAS. Aplikasi Go kemudian memicu origination lewat kanal terkendali (mis. perintah `radmin`/control socket, atau modul REST internal), sehingga shared secret `AccessPoint` tetap hanya di server RADIUS.

**(c) Implementasi paket RFC 5176 langsung di Go.** Cocok untuk job reaktif (issue #8) yang butuh retry/timeout/idempotensi sendiri: bangun paket RADIUS kode 40 ke UDP 3799 dengan atribut identifikasi + `Message-Authenticator` (HMAC-MD5 per RFC 3579/2869, dihitung pada paket dengan field `Message-Authenticator` dinolkan) dan `Request Authenticator` sesuai aturan RFC 5176, lalu tangani ACK/NAK dan `Error-Cause`. Library komunitas (mis. `layeh.com/radius`) dapat mempercepat, tetapi dukungan kode paket 40/41/42 dan perhitungan `Message-Authenticator` **harus diverifikasi sendiri** terhadap RFC 5176 — ini **[INDIKATIF]**, bukan bukti primer.

Trade-off ringkas: (a) paling murah untuk MVP tetapi menaruh secret DAS di host job dan mudah salah parsing keluaran; (b) paling aman untuk pengelolaan secret, tetapi menambah ketergantungan pada konfigurasi policy FreeRADIUS (FR3 vs FR4 berbeda); (c) kontrol penuh atas timeout/idempotensi dan mudah diuji dengan `radclient`, tetapi menambah kode kripto sendiri.

Untuk ketiga jalur, perlu diingat: **Disconnect hanya berfungsi bila `AccessPoint` dapat dijangkau masuk (inbound) di UDP 3799 dari pengirim.** Untuk `AccessPoint` di balik NAT, jaminan suspend tetap bertumpu pada `Session-Timeout` + `Termination-Action=1` (lantai dasar), persis seperti yang dirumuskan ADR-0001.

### 8. `CaptivePortal` di `Vlan` suspended

Yang harus disediakan gateway (semuanya dapat dinyatakan sebagai konfigurasi OpenWrt; yang bersifat spesifik-lingkungan ditandai perlu uji lokal):

1. **Interface `Vlan` suspended dengan alamat IP** + **zona firewall**. Wiki DSA OpenWrt menegaskan: *"That interface must be associated with a firewall zone (or rules) to accept input."* Tanpa ini, portal tidak menerima trafik apa pun.
2. **DHCP + DNS (dnsmasq)**. Wiki `/etc/config/dhcp` mendokumentasikan `dhcp_option` sebagai daftar string dengan contoh nyata *"3,192.168.1.1 6,192.168.1.1"* untuk gateway dan DNS server — pola yang tepat untuk memaksa klien suspended memakai DNS gateway (syarat agar DNS dapat di-intercept/allowlist):

   ```uci
   config dhcp 'suspended'
       option interface 'suspended'
       option start '100'
       option limit '150'
       option leasetime '30m'
       option dhcp_option '3,10.99.0.1 6,10.99.0.1'
   ```
   Secara umum `dhcp_option` juga dapat membawa DHCP option 114 (URL captive portal) gaya `114,"http://portal..."`; **sintaks pastinya perlu diverifikasi lokal** karena dokumentasi yang saya baca tidak memuat contoh 114 (`dnsmasq --help dhcp` mencetak daftar nama opsi yang dikenali, seperti disarankan wiki).
3. **Allowlist egress.** Zona firewall `Vlan` suspended sebaiknya default-drop, dengan allowlist minimal: DNS ke resolver gateway, portal `CaptivePortal` (TLS), dan endpoint `Payment`/`TopUp` yang diperlukan. Contoh aturan spesifik (dest IP/port) **perlu uji lokal** — dokumentasi yang saya baca hanya memberi kerangka zona/forwarding, bukan contoh allowlist egress per-destinasi.
4. **Intercept HTTP/DNS.** Klasik: arahkan DNS (UDP/TCP 53) dan HTTP (TCP 80) ke portal; HTTPS (TCP 443) tidak dapat dialihkan tanpa sertifikat yang cocok, sehingga strategi yang lazim adalah: (i) blokir 443 dan biarkan klien mendeteksi captive portal via mekanisme deteksi OS, atau (ii) tampilkan portal di domainnya sendiri dan biarkan deteksi captive portal OS menampilkan notifikasi "login ke jaringan". Detail implementasi (`dnsmasq` hijack vs firewall DNAT, sertifikat TLS, URL deteksi OS) **belum saya verifikasi dari sumber primer** → **[TIDAK TERVERIFIKASI]** dan perlu uji lokal.
5. **Isolasi klien.** `Vlan` suspended adalah zona bersama, jadi isolasi klien wajib. Yang terdokumentasi OpenWrt (`/etc/config/wireless`):
   - `option isolate 1` — *"Isolates wireless clients from each other, only applicable in ap mode."*
   - `option bridge_isolate 1` — *"Isolates wireless clients from each other on the AP's bridge. (i.e. 2.4ghz and 5ghz radios on the same AP.)"*
   - Alternatif/penambah di sisi hostapd: `per_sta_vif=1` — *"If enabled, each station is assigned its own AP_VLAN interface. This implies per-station group keying and ebtables filtering of inter-STA traffic (when passed through the AP)."*
   - Karena semua klien suspended berada di `Vlan` L2 yang sama, **pemisahan antar-klien di gateway** (mis. bridge port isolation / aturan firewall antar-host, atau memecah `Vlan` suspended per-`AccessPoint`) juga perlu dipertimbangkan — **[INFERENSI PENELITI]**, perlu uji lokal.

Perlu dicatat: `isolate`/`bridge_isolate` hanya memisahkan klien **nirkabel pada satu `AccessPoint`**. Karena topologi Shared L2, dua rumah di `AccessPoint` berbeda tetap berada di segmen L2 yang sama pada `Vlan` suspended → isolasi antar-`Customer` di `Vlan` suspended bergantung pada konfigurasi switch/gateway (langkah manual MVP).

### 9. Celah identitas: memetakan klien tersuspend ke `RadiusAccount`/`Customer`

Ini pertanyaan paling penting, dan jawabannya bercabang tiga. Sumber primer untuk semua cabang adalah kode hostapd.

**(A) Klien yang masih mengautentikasi 802.1X (kasus utama ADR-0001).** Menurut ADR-0001, `RadiusAccount` yang suspended **tidak** ditolak keras: ia tetap lolos 802.1X dengan kredensialnya sendiri dan hanya dipindahkan ke `Vlan` suspended. Karena itu sesi tersebut **punya identitas EAP**; `hostapd` mencatatnya, dan tiap paket accounting membawa identitas tersebut. Bukti kode `src/ap/accounting.c`, fungsi `accounting_msg()`:

```c
		/* Use 802.1X identity if available */
		val = ieee802_1x_get_identity(sta->eapol_sm, &len);

		/* Use RADIUS ACL identity if 802.1X provides no identity */
		if (!val && sta->identity) {
			val = (u8 *) sta->identity;
			len = os_strlen(sta->identity);
		}

		/* Use STA MAC if neither 802.1X nor RADIUS ACL provided
		 * identity */
		if (!val) {
			os_snprintf(buf, sizeof(buf), RADIUS_ADDR_FORMAT, MAC2STR(sta->addr));
			val = (u8 *) buf;
			len = os_strlen(buf);
		}

		if (!radius_msg_add_attr(msg, RADIUS_ATTR_USER_NAME, val, len)) { ... }
```

Jadi `User-Name` (**selalu** ada di Accounting-Request), `Calling-Station-Id`, `Acct-Session-Id`, `NAS-IP-Address`/`NAS-Identifier` — cukup untuk memetakan `Session` → `RadiusAccount` → `Customer`, dan untuk menurunkan token server-side bagi `CaptivePortal` (sesuai persyaratan MVP: *"identitas dari klien ditolak"*). **[TERVERIFIKASI]**

**(B) Klien tanpa kredensial 802.1X: MAC authentication (MAB) lewat RADIUS MAC ACL.** hostapd mendukung mode `macaddr_acl=2` (RADIUS): pada frame Authentication/(Re)Association, hostapd mengirim **Access-Request non-EAP** dengan `User-Name = MAC` **dan** `User-Password = MAC`, terenkripsi dengan shared secret NAS — bukti `src/ap/ieee802_11_auth.c`, fungsi `hostapd_radius_acl_query()`:

```c
	os_snprintf(buf, sizeof(buf), RADIUS_ADDR_FORMAT, MAC2STR(addr));
	if (!radius_msg_add_attr(msg, RADIUS_ATTR_USER_NAME, (u8 *) buf, os_strlen(buf))) { ... }
	if (!radius_msg_add_attr_user_password(msg, (u8 *) buf, os_strlen(buf),
	                                       hapd->conf->radius->auth_server->shared_secret,
	                                       hapd->conf->radius->auth_server->shared_secret_len)) { ... }
```

Dokumentasi `hostapd.conf`:

```
# 0 = accept unless in deny list
# 1 = deny unless in accept list
# 2 = use external RADIUS server (accept/deny lists are searched first)
macaddr_acl=0
```

Yang membuat jalur ini menyelesaikan celah identitas: **hostapd membaca balasan Access-Accept dan mengambil `Vlan` serta identitas dari sana** (`hostapd_acl_recv_radius()`):

```c
	if (hapd->conf->ssid.dynamic_vlan != DYNAMIC_VLAN_DISABLED)
		info->vlan_id.notempty = !!radius_msg_get_vlanid(msg, &info->vlan_id.untagged,
		                                                 MAX_NUM_TAGGED_VLAN, info->vlan_id.tagged);
	...
	if (radius_msg_get_attr_ptr(msg, RADIUS_ATTR_USER_NAME, &buf, &len, NULL) == 0) {
		info->identity = os_zalloc(len + 1);
		if (info->identity) os_memcpy(info->identity, buf, len);
	}
	if (radius_msg_get_attr_ptr(msg, RADIUS_ATTR_CHARGEABLE_USER_IDENTITY, &buf, &len, NULL) == 0) { ... }
```

Artinya: FreeRADIUS dapat membalas Access-Request `User-Name=<MAC>` dengan `User-Name=<username RadiusAccount sesungguhnya>`, `Tunnel-*` (aktif atau suspended), `Session-Timeout`, dan `Acct-Interim-Interval`. hostapd memakai `User-Name` itu sebagai identitas STA (`sta->identity`) → muncul di accounting sebagai `User-Name` (prioritas kedua setelah identitas 802.1X). Dengan begitu klien yang **tidak pernah** melakukan EAP pun dapat diatribusikan ke `RadiusAccount`/`Customer` dan ditempatkan di `Vlan` suspended. **[TERVERIFIKASI]**

Batasan dan trade-off jalur (B) — ini harus ditulis eksplisit di ADR:

1. **Butuh BSS non-802.1X.** `macaddr_acl` dievaluasi pada saat asosiasi; pada BSS RSN/802.1X klien tetap harus menyelesaikan EAP. Jadi MAB memerlukan SSID terpisah (terbuka + portal, atau WPA2-PSK per-`Customer`), yang menyambung ke arah ADR-0004 (dual-SSID) — tetapi dengan tambahan: **SSID PSK per-`Customer` juga dapat memakai `macaddr_acl=2`** sehingga MAC harus terdaftar di RADIUS **dan** PSK harus cocok, dan `Vlan` dapat diambil dari RADIUS alih-alih dari berkas PSK.
2. **Identitas berbasis MAC dapat dipalsukan.** `User-Name`/`Calling-Station-Id` yang dipakai `CaptivePortal` untuk mengikat `TopUp` menjadi dapat dikendalikan penyerang pada klien non-802.1X. Karena itu binding `TopUp` pada jalur (B) **tidak boleh** hanya bersandar pada MAC/sesi RADIUS; harus ada langkah tambahan milik pengguna (mis. OTP email yang sudah ada di MVP) sebelum aksi bernilai uang.
3. **Hardening kata sandi MAB.** hostapd kini mewajibkan `Message-Authenticator` pada balasan MAC ACL secara default, dengan saklar kompatibilitas. Kutipan `hostapd.conf`: *"hostapd requires Message-Authenticator attribute to be included in all cases where RADIUS is used for EAP authentication. This is also required for cases where RADIUS is used for MAC ACL (macaddr_acl=2) by default, but that case can be configured to not require this for compatibility with RADIUS servers that do not include the attribute."* → FreeRADIUS wajib mengirim `Message-Authenticator` pada Access-Accept untuk kasus MAC ACL.
4. **Belum terverifikasi:** apakah `Session-Timeout` yang dikembalikan pada jalur ACL benar-benar menghasilkan re-evaluasi berkala (analog `reAuthPeriod` 802.1X) atau hanya disimpan di cache ACL (`RADIUS_ACL_TIMEOUT 30`) dan di masa hidup PMK. Ini **perlu uji lokal** dan berdampak pada jaminan suspend untuk perangkat non-802.1X.

**(C) PSK per-`Customer` dengan penetapan `Vlan` lokal.** Komentar `dynamic_vlan` di `hostapd.conf` menyebut secara eksplisit: *"Dynamic VLAN mode is also used with VLAN ID assignment based on WPA/WPA2 passphrase from `wpa_psk_file` or `vlan_id` parameter from `sae_password`."* OpenWrt menyediakan `option wpa_psk_file` pada `wifi-iface`, dan `hostapd.uc` (24.10) memperlakukan `wpa_psk_file` sebagai `file_fields` (path harus ada; hash isi berkas dipakai untuk deteksi perubahan) serta mendukung perintah kontrol `RELOAD_WPA_PSK` — jadi memindahkan `Customer` dari `Vlan` aktif ke `Vlan` suspended dapat dilakukan dengan menulis ulang berkas PSK lalu `RELOAD_WPA_PSK`. Konsekuensinya: penetapan `Vlan` untuk perangkat non-BYOD menjadi **lokal di `AccessPoint`** (tidak lewat RADIUS), sedangkan atribusi billing-nya melalui `User-Name` = MAC di accounting (prioritas ketiga pada `accounting_msg()` di atas) → perlu registry MAC→`Customer`. Trade-off: implementasi paling sederhana, tetapi identitas paling lemah dan `Vlan` harus dikelola per-`AccessPoint`. Sintaks persis entri VLAN pada `wpa_psk_file` **belum saya verifikasi** dari berkas `hostapd.conf` versi rujukan → perlu dicek lokal (`grep -n -A15 'wpa_psk_file' /path/hostapd.conf`).

**(D) Klien yang tidak punya kredensial dan MAC-nya tidak terdaftar.** Untuk kasus ini **tidak ada jalur terverifikasi** yang dapat memetakan klien → `RadiusAccount`/`Customer` secara otomatis. Pengakuan jujur + opsi: `CaptivePortal` meminta pengguna memasukkan username `RadiusAccount` atau email, lalu mengirim **OTP** (mekanisme OTP email sudah ada di MVP) dan mengikat token server-side setelah OTP valid. Ini menutup celah identitas tanpa bergantung pada MAC, dengan biaya UX satu langkah tambahan. **[INFERENSI PENELITI — rekomendasi desain, bukan kutipan sumber]**

Ringkasan jalur & trade-off:

| Jalur | Klien | Sumber identitas | Kekuatan | Kelemahan |
|---|---|---|---|---|
| A | 802.1X (PEAP-MSCHAPv2) | identitas EAP (`User-Name`) | kuat (kredensial) | perangkat harus mendukung 802.1X; kredensial harus valid |
| B | MAC-auth di BSS non-802.1X | `User-Name` dari Access-Accept (FreeRADIUS memetakan MAC→`RadiusAccount`) | berfungsi untuk perangkat tanpa UI kredensial | MAC dapat dipalsukan; butuh BSS terpisah + registry MAC; butuh `Message-Authenticator` |
| C | PSK per-`Customer` | MAC di accounting (fallback hostapd) | tidak butuh RADIUS untuk `Vlan` | identitas berbasis MAC; `Vlan` dikelola lokal per AP; PSK bocor = akses penuh |
| D | apa pun tanpa kredensial & MAC tak terdaftar | OTP (username/email) | tidak bergantung MAC | satu langkah UX tambahan; butuh rate-limit/lockout OTP |

### 10. Bukti pendukung sekunder (hanya konteks) — [INDIKATIF]

- Forum OpenWrt, "Individual per-passphrase/per-MAC Wifi VLANs using `wpa_psk_file`": melaporkan bahwa `wpad-basic` default "fails silently for most of the relevant options, with the exception of `vlan_tagged_interface` which it rejects violently", dan menyarankan hostapd/wpad full. Konsisten dengan temuan primer, tetapi bukan bukti.
- Isu OpenWrt `#7459` (FS#488) "dynamic VLAN doesn't work on ath10k" — peringatan bahwa dukungan driver (`AP_VLAN`/`4addr`) berbeda antar chipset; relevan sebagai risiko uji lapangan.
- ServerFault "OpenWRT Dynamic VLAN" — diskusi bahwa skrip OpenWrt meneruskan nilai `dynamic_vlan` apa adanya ke konfigurasi hostapd.
- Blog "Wi-Fi: Hostapd VLAN for guest network" — mengutip blok `hostapd.conf` dynamic VLAN (kutipan yang sama dengan versi primer di dokumen ini).
- Komunitas Ubiquiti/pfSense/UBNT — contoh `radius_das_port`/`radius_das_client` di build hostapd pihak ketiga (bukan bukti untuk OpenWrt).
- Tiket OpenWrt lama (#15259, #22841) dan `dev.archive.openwrt.org` changeset 41872 — konteks historis penamaan `wpad-full` era pra-19.07; **nama paket tersebut tidak berlaku di rilis kini**.
- StackOverflow "How does Termination-Action and Session-Timeout work?" — deskripsi perilaku pada NAS lain (MikroTik), berguna sebagai pembanding, bukan bukti untuk hostapd.

## Risiko & ketidakpastian

1. **PMKSA caching dapat menunda suspend (residual risk).** `Session-Timeout` juga dipakai sebagai masa hidup entri PMKSA (`wpa_auth_pmksa_add(..., session_timeout, ...)`), dan entri PMKSA menyimpan `struct vlan_description *vlan_desc` (`src/ap/pmksa_cache_auth.h`). Klien yang reconnect memakai PMKSA valid melewati EAP (`"PMK from PMKSA cache - skip IEEE 802.1X/EAP"`) dan `Vlan` lama diterapkan kembali. Batas waktunya = sisa `Session-Timeout` (≤ 10–30 menit sesuai kebijakan), dan jalur cepat Disconnect membersihkan PMKSA (`wpa_auth_pmksa_remove()` di `hostapd_das_disconnect`). **Mitigasi yang perlu diuji:** durasi `Session-Timeout` yang pendek, `disable_pmksa_caching` (opsi ada di `hostapd.conf`; efeknya di OpenWrt belum diuji), dan menjaga masa hidup PMKSA ≤ interval `Session-Timeout`. **[Interpretasi peneliti dari kode; perlu uji lokal]**
2. **`Session-Timeout` pada sesi MAC-auth (MAB) belum terverifikasi.** Saya belum menemukan bukti setara `sm->reAuthPeriod` untuk jalur ACL; apakah timeout benar-benar memicu re-evaluasi RADIUS untuk klien non-802.1X **harus diuji**, dan bila tidak, jalur MAB tidak memenuhi jaminan suspend/reaktivasi yang sama dengan 802.1X.
3. **`vlan_tagged_interface` yang salah = klien terisolasi total.** Bridge per-`Vlan` yang dibuat hostapd tidak punya uplink kecuali `vlan_tagged_interface` menunjuk perangkat yang membawa trunk. Pada perangkat DSA, nilai yang benar (bridge `br-lan` dengan port uplink bertag, atau port fisik uplink) bergantung topologi → **uji lapangan per model `AccessPoint`**.
4. **Dukungan driver untuk `AP_VLAN`/VLAN per-STA berbeda antar chipset** (bukti sekunder ath10k [INDIKATIF]). Perlu uji untuk setiap model `AccessPoint` yang dipakai.
5. **Kutipan berkas upstream diambil dari cermin**, karena `w1.fi` tidak dapat diakses dari lingkungan riset ini. Nomor/isi kunci (13/6, penanganan `Termination-Action`, penolakan CoA, rangkaian fallback `User-Name` di accounting) konsisten di beberapa salinan independen, tetapi **kutipan harus diverifikasi ulang** terhadap versi hostapd yang benar-benar dipaketkan rilis OpenWrt target (25.12.5 memaketkan hostapd 2025.08.26).
6. **Nama simbolik nilai dictionary FreeRADIUS** (`VLAN`, `IEEE-802`, `RADIUS-Request`) belum diverifikasi dari berkas dictionary [TIDAK TERVERIFIKASI]; hanya angka dan semantiknya yang terverifikasi.
7. **Detail `CaptivePortal` di gateway** (DHCP option 114, mekanisme intercept HTTP/DNS, aturan allowlist egress, sertifikat TLS untuk portal) belum terverifikasi dari sumber primer → **perlu uji lokal**; yang terverifikasi hanyalah kerangka (interface + zona firewall + `dhcp_option` + opsi isolasi klien).
8. **Default `radius_das_require_message_authenticator` / `radius_das_time_window` pada build OpenWrt.** Wiki OpenWrt hanya mendokumentasikan `dae_client`/`dae_port`/`dae_secret`, sehingga saklar tambahan mungkin tidak dapat disetel dari UCI. Periksa berkas `hostapd.conf` hasil generate; bila tidak tersedia, keputusan hardening (mengharuskan `Message-Authenticator`, `Event-Timestamp`) harus dikelola di sisi pengirim (selalu kirim atribut tersebut) — bukan mengandalkan konfigurasi AP.
9. **Idempotensi & keandalan Disconnect.** `Disconnect-NAK` dengan Error-Cause 503 (sesi tidak ditemukan) normal bila sesi sudah berakhir; 508 (multi-sesi) dapat muncul bila identifikasi tidak spesifik (mis. hanya `User-Name` untuk akun yang dipakai dua perangkat). Job reaktif harus mengirim identifikasi sesi yang cukup (`Acct-Session-Id` 16 hex dari accounting) dan menangani 508 dengan memilih sesi yang benar.

## Sumber

Semua URL diakses pada sesi riset ini (2026-09-27); "primer" = dokumentasi resmi proyek/RFC/kode sumber upstream.

**Primer — RFC/IETF**

1. RFC 2865, *Remote Authentication Dial In User Service (RADIUS)*, §5.27 `Session-Timeout`, §5.29 `Termination-Action` — https://www.rfc-editor.org/rfc/rfc2865.txt — definisi normatif kedua atribut yang menentukan perilaku suspend.
2. RFC 2868, *RADIUS Attributes for Tunnel Protocol Support* — https://www.rfc-editor.org/rfc/rfc2868.txt — tipe atribut 64/65/81, nilai Tunnel-Medium-Type `6 = 802`, dan §6.1 yang membuktikan nilai `VLAN=13` bukan dari RFC ini.
3. RFC 3580, *IEEE 802.1X RADIUS Usage Guidelines* — https://www.rfc-editor.org/rfc/rfc3580.txt — kutipan resmi trio atribut penetapan `Vlan` (Tunnel-Type=VLAN (13), Tunnel-Medium-Type=802, Tunnel-Private-Group-ID=VLANID) dan rujukan Ch. 3.17 untuk interaksi `Session-Timeout`/`Termination-Action`.
4. RFC 5176, *Dynamic Authorization Extensions to RADIUS* — https://www.rfc-editor.org/rfc/rfc5176.txt — port 3799, daftar atribut identifikasi NAS/sesi, §3.4 `Message-Authenticator`, makna kode Error-Cause.
5. RFC 3579/2869 (rujukan tidak langsung untuk `Message-Authenticator`) — disebut oleh RFC 5176 §3.4; tidak dikutip langsung di dokumen ini.

**Primer — hostapd/wpa_supplicant (kode & berkas contoh upstream)**

6. `hostapd.conf` (berkas contoh upstream; salinan cermin) — blok `dynamic_vlan`, `vlan_file`, `vlan_tagged_interface`, `vlan_bridge`, `vlan_naming`, `per_sta_vif`, `macaddr_acl`, `radius_das_*`, `wpa_psk_radius`, catatan `Message-Authenticator` untuk MAC ACL — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/hostapd/hostapd.conf (kanonik: `https://w1.fi/cgit/hostap/plain/hostapd/hostapd.conf`, tidak dapat diakses dari lingkungan ini).
7. `src/ap/ieee802_1x.c` — penanganan `Session-Timeout`/`Termination-Action` (`sm->reAuthPeriod`), pembaruan `Vlan` pada Access-Accept (`ieee802_1x_update_vlan`, `ap_sta_bind_vlan`) — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/src/ap/ieee802_1x.c
8. `src/ap/sta_info.c` — `ap_handle_session_timer()`: deautentikasi + `Acct-Terminate-Cause=Session-Timeout` — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/src/ap/sta_info.c
9. `src/radius/radius_das.c` — daftar atribut yang diizinkan, cabang Disconnect vs CoA, Error-Cause 401/402/405/503/508, syarat inisialisasi DAS — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/src/radius/radius_das.c
10. `src/ap/hostapd.c` — `hostapd_das_disconnect()` (pencocokan sesi, `wpa_auth_pmksa_remove()`, deauth) dan `hostapd_das_coa()` (hanya HS 2.0 T&C filtering; `NULL` tanpa `CONFIG_HS20`) — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/src/ap/hostapd.c
11. `src/ap/ieee802_11_auth.c` — `macaddr_acl=2` → Access-Request dengan `User-Name=MAC` + `User-Password=MAC`; pembacaan `Vlan`/`User-Name`/CUI dari balasan Access-Accept — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/src/ap/ieee802_11_auth.c
12. `src/ap/accounting.c` — rangkaian fallback `User-Name` (identitas 802.1X → identitas RADIUS ACL → MAC STA) pada Accounting-Request — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/src/ap/accounting.c
13. `src/radius/radius.h` — `RADIUS_TUNNEL_TYPE_VLAN 13`, `RADIUS_TUNNEL_MEDIUM_TYPE_802 6`, `RADIUS_TUNNEL_TAGS 32` — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/src/radius/radius.h
14. `src/radius/radius.c` — tabel atribut (termasuk `RADIUS_ATTR_EGRESS_VLANID`) dan `radius_msg_get_vlanid()` (VLAN bertag) — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/src/radius/radius.c
15. `src/ap/pmksa_cache_auth.h` — entri PMKSA menyimpan `struct vlan_description *vlan_desc` — https://raw.githubusercontent.com/nikescar/hostapd-mirror/master/src/ap/pmksa_cache_auth.h

**Primer — OpenWrt**

16. `package/network/services/hostapd/Makefile` (cabang `openwrt-25.12`) — definisi paket & `VARIANT` (`wpad-full-internal`, `wpad-full-mbedtls`, `wpad-basic-mbedtls`, `full`/`basic`/`mini`/`mesh`) — https://raw.githubusercontent.com/openwrt/openwrt/openwrt-25.12/package/network/services/hostapd/Makefile
17. `package/network/services/hostapd/files/hostapd-full.config` (basis `openwrt-24.10` dan `openwrt-25.12`) — `CONFIG_FULL_DYNAMIC_VLAN=y`, `CONFIG_INTERWORKING=y`, `CONFIG_HS20=y` — https://raw.githubusercontent.com/openwrt/openwrt/openwrt-24.10/package/network/services/hostapd/files/hostapd-full.config
18. `package/network/services/hostapd/files/hostapd-basic.config` (basis `openwrt-24.10`) — `CONFIG_NO_RADIUS=y`, `CONFIG_NO_ACCOUNTING=y`, `#CONFIG_FULL_DYNAMIC_VLAN=y` — https://raw.githubusercontent.com/openwrt/openwrt/openwrt-24.10/package/network/services/hostapd/files/hostapd-basic.config
19. Indeks paket resmi OpenWrt 25.12.5 (arsitektur `x86_64`, feed `base`) — nama paket, versi `2025.08.26~ca266cc2-r2`, ukuran `wpad`/`wpad-mbedtls`/`wpad-basic-mbedtls` — https://downloads.openwrt.org/releases/25.12.5/packages/x86_64/base/
20. Indeks rilis resmi OpenWrt — daftar rilis 24.10.x/25.12.x (penetapan "versi terkini") — https://downloads.openwrt.org/releases/
21. Wiki OpenWrt, *Wi-Fi /etc/config/wireless* — opsi `dynamic_vlan`, `vlan_naming`, `vlan_tagged_interface`, `vlan_bridge`, `vlan_file`, `dae_client`, `dae_port`, `dae_secret`, `isolate`, `bridge_isolate`, contoh format `vlan_file` (termasuk wildcard), pernyataan "ignored or rejected by the default stripped down wpad-basic/hostapd-basic binaries", tabel nilai `encryption` (`wpa2` = WPA2 Enterprise) — https://openwrt.org/docs/guide-user/network/wifi/basic
22. Wiki OpenWrt, *DSA Mini-Tutorial* — `config device`/`config bridge-vlan`, penandaan port trunk (`:t`) dan untagged/PVID (`:u*`), dan pernyataan bahwa interface harus berada di zona firewall agar menerima input — https://openwrt.org/docs/guide-user/network/dsa/dsa-mini-tutorial
23. Wiki OpenWrt, *DHCP and DNS configuration /etc/config/dhcp* — `dhcp_option` (contoh `'3,192.168.1.1 6,192.168.1.1'`), tagging/classifier, `dhcp_option_force` — https://openwrt.org/docs/guide-user/base-system/dhcp
24. `package/network/config/wifi-scripts/files/lib/netifd/wireless/mac80211.sh` (cabang `openwrt-24.10`) — pembacaan opsi `vlan_file` per-iface dan pemanggilan `hostapd_set_vlan` — https://raw.githubusercontent.com/openwrt/openwrt/openwrt-24.10/package/network/config/wifi-scripts/files/lib/netifd/wireless/mac80211.sh
25. `package/network/services/hostapd/files/hostapd.uc` (cabang `openwrt-24.10`) — `vlan_file`/`wpa_psk_file` sebagai `file_fields`, perintah `RELOAD_WPA_PSK` — https://raw.githubusercontent.com/openwrt/openwrt/openwrt-24.10/package/network/services/hostapd/files/hostapd.uc

**Primer — FreeRADIUS**

26. Man page resmi `radclient` (FreeRADIUS; diperbarui 21 Mei 2024) — jenis paket `coa`/`disconnect`, port default 3799 untuk coa/disconnect, `Packet-Dst-IP-Address`/`Packet-Dst-Port`/`Packet-Type`, `-S`, `-r`, `-t` — https://freeradius.org/radiusd/man/radclient.html
27. Wiki FreeRADIUS, *Disconnect Messages* — kode 40/41/42, keharusan atribut identifikasi, pernyataan bahwa FreeRADIUS dapat mengirim Disconnect-Request via `update coa`/`update disconnect`, dan contoh perintah `radclient ... disconnect` — https://wiki.freeradius.org/protocol/Disconnect-Messages
28. `raddb/sites-available/originate-coa` (FreeRADIUS `master`) — pola `subrequest ::Disconnect-Request` + modul `radius.coa`, catatan "This functionality is configured differently from v3", dan peringatan bahwa atribut yang dibutuhkan tergantung vendor NAS — https://raw.githubusercontent.com/FreeRADIUS/freeradius-server/master/raddb/sites-available/originate-coa
29. Wiki FreeRADIUS, *Mac Auth* — pola plain MAC-auth, MAC+802.1X, normalisasi `Calling-Station-ID` (`rewrite_calling_station_id`), contoh modul `files` dengan `key = "%{Calling-Station-ID}"` — https://wiki.freeradius.org/guide/Mac-Auth

**Sekunder (dipakai hanya sebagai konteks, TIDAK dipakai sebagai bukti klaim)**

30. Forum OpenWrt, "Individual per-passphrase/per-MAC Wifi VLANs using `wpa_psk_file` (no RADIUS required)" — laporan bahwa `wpad-basic` gagal senyap pada opsi dynamic VLAN dan menolak `vlan_tagged_interface` — https://forum.openwrt.org/t/individual-per-passphrase-per-mac-wifi-vlans-using-wpa-psk-file-no-radius-required/161696
31. GitHub OpenWrt issue #7459 (FS#488), "dynamic VLAN doesn't work on ath10k" — variasi dukungan driver — https://github.com/openwrt/openwrt/issues/7459
32. ServerFault, "OpenWRT Dynamic VLAN" — diskusi penerusan opsi `dynamic_vlan` ke konfigurasi hostapd — https://serverfault.com/questions/765063/openwrt-dynamic-vlan
33. Blog "Wi-Fi: Hostapd VLAN for guest network" — mengutip blok `hostapd.conf` dynamic VLAN (kutipan yang sama dengan sumber primer #6) — https://mhtechz.wordpress.com/2016/04/11/wi-fi-hostapd-vlan-for-guest-network/
34. Dev archive OpenWrt (changeset 41872, tiket #15259, #22841) — konteks historis penamaan `wpad-full` era lama — https://dev.archive.openwrt.org/changeset/41872.html
35. StackOverflow, "How does Termination-Action and Session-Timeout work?" — pembanding perilaku pada NAS non-hostapd — https://stackoverflow.com/questions/53179710/how-does-termination-action-and-session-timeout-work
36. Komunitas Ubiquiti, "Request: Enable Dynamic Authorization Extensions in hostapd for RADIUS" — mengutip blok DAS `hostapd.conf` pada build pihak ketiga — https://community.ui.com/questions/Request-Enable-Dynamic-Authorization-Extensions-in-hostapd-for-RADIUS/52ac1019-1ada-4e13-9f6c-881d46ff330d

**Ditolak/dideprioritaskan:** `w1.fi` (kanonik hostapd) tidak dapat diakses dari lingkungan riset ini (digantikan cermin #6–#15); `dev.archive.openwrt.org` dan tiket lama era BB/CC (istilah `wpad-full`) tidak relevan untuk rilis kini; situs agregat/SEO tentang konfigurasi hostapd tidak dipakai; artikel vendor NAS lain (Cisco/MikroTik) tidak dipakai sebagai bukti perilaku hostapd.

## Langkah berikutnya

1. **Uji laboratorium jalur utama 802.1X (menutup Pertanyaan 5 & 6 secara empiris).** Dua `AccessPoint` OpenWrt (satu dengan `wpad-basic-mbedtls` sebagai kontrol, satu dengan `wpad-mbedtls`) + FreeRADIUS: autentikasi klien PEAP-MSCHAPv2, Access-Accept membawa `Tunnel-*` + `Session-Timeout=600` + `Termination-Action=1`; verifikasi (a) klien pindah ke `Vlan` yang benar (`ip -d link`, `bridge vlan show`, `hostapd -dd` log `VLAN ID`), (b) re-auth terjadi tanpa putus pada menit ke-10 dan `Vlan` berubah saat `SubscriptionState` diubah, (c) setelah itu kirim `radclient` Disconnect-Request → `Disconnect-ACK`, klien reconnect dan mendapat `Vlan` baru, (d) pada varian basic: konfirmasi kegagalan (pesan hostapd/ketiadaan `dynamic_vlan` di konfigurasi hasil generate).
2. **Uji jalur MAB (menutup Pertanyaan 9 dan risiko nomor 2).** BSS terpisah (terbuka/PSK) dengan `macaddr_acl=2`; catat Access-Request di FreeRADIUS (`User-Name`=MAC, `User-Password`=MAC) dan balas dengan `User-Name` asli + `Tunnel-*` + `Session-Timeout` + `Message-Authenticator`; verifikasi (a) klien masuk `Vlan` yang diminta, (b) `User-Name` asli muncul di Accounting-Request, (c) apakah berakhirnya `Session-Timeout` benar-benar memicu evaluasi RADIUS ulang atau tidak.
3. **Uji PMKSA caching (risiko nomor 1).** Suspend akun saat klien online, lalu paksa disconnect/reconnect dalam rentang sisa `Session-Timeout` dan amati apakah klien kembali ke `Vlan` lama tanpa Access-Request baru; bandingkan dengan kondisi `Disconnect-Request` (yang membersihkan PMKSA) dan/atau `disable_pmksa_caching=1`; tetapkan angka `Session-Timeout` dan kebijakan PMKSA final untuk ADR.
4. **Pin rilis & paket + validasi konfigurasi gateway.** Tetapkan versi OpenWrt target (mis. 25.12.5) dan daftar paket wajib (`wpad-mbedtls`/`wpad`), dokumentasikan pengecekan pasca-provisi (`apk list --installed`, `grep -E 'dynamic_vlan|vlan_file|radius_das_' /var/run/hostapd-*.conf`), dan verifikasi blok `CaptivePortal` di gateway (interface `Vlan` suspended + zona firewall + `dhcp_option` + allowlist egress + opsi isolasi klien `isolate`/`bridge_isolate`).

## Rekomendasi terhadap ADR-0001 (tanpa mengubah berkas ADR)

- **Status ADR-0001: tetap valid**, dengan revisi kata-kata berikut:
  1. Ubah "hostapd hanya mendukung Disconnect-Request, BUKAN CoA untuk mengubah VLAN sesi berdiri" → tetap benar, tetapi tambahkan alasan spesifik: satu-satunya `CoA-Request` yang diproses hostapd adalah *HS 2.0 Terms & Conditions filtering*; CoA yang membawa `Tunnel-*` dijawab NAK (Error-Cause 401/402/405).
  2. **Wajibkan `Termination-Action = RADIUS-Request (1)`** pada setiap Access-Accept bersama `Session-Timeout`, karena tanpa itu hostapd mem-deautentikasi klien alih-alih melakukan re-authentication (basis: Pertanyaan 5). Tanpa klausul ini, ADR-0001 secara implisit menjanjikan re-evaluasi "mulus" yang tidak dijamin.
  3. Catat jalur **MAC-auth (`macaddr_acl=2`)** sebagai mekanisme terverifikasi untuk pemetaan klien non-802.1X → `RadiusAccount`/`Customer`, dengan syarat: BSS terpisah non-802.1X, registry MAC, `Message-Authenticator` pada balasan FreeRADIUS, dan **identitas tidak boleh dipercaya untuk aksi bernilai uang tanpa OTP**.
  4. Catat **residual risk PMKSA caching** dan bahwa Disconnect-Request membersihkan entri PMKSA (jadi fast-path juga berfungsi sebagai pembersih cache, bukan hanya pemutus sesi).
  5. Pertegas nama paket pada prasyarat build: varian **full** (`wpad`, `wpad-mbedtls`, `wpad-openssl`, `wpad-wolfssl` atau `hostapd`/`hostapd-mbedtls`/…) — bukan nama `wpad-full` yang sudah tidak ada; dan varian **basic** tidak layak karena RADIUS dikompilasi keluar.
