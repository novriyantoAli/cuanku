# Riset: Monitoring AccessPoint — SNMP di OpenWrt, metrik WiFi, dan keterjangkauan di balik NAT

Dokumen ini menguji janji **ADR-0002** (`docs/adr/0002-monitoring-adapter-abstraction.md`): adapter monitoring netral protokol, implementasi MVP = polling SNMP v2c tiap 5 menit, dengan cakupan "lengkap" (online/offline, uptime, CPU/memori, signal/noise per radio, jumlah client, traffic RX/TX), plus dua pengakuan ADR itu sendiri — (a) metrik WiFi tidak tersedia di net-snmp standar OpenWrt sehingga butuh `extend`/`pass`, dan (b) `AccessPoint` sering di balik NAT sehingga SNMP (pull) bisa gagal.

Pertanyaan yang dijawab: (1) paket SNMP & UCI resmi di OpenWrt; (2) MIB/modul yang benar-benar terkompilasi; (3) CPU/memori — HOST-RESOURCES-MIB vs UCD-SNMP-MIB; (4) metrik WiFi per radio dan jumlah client (`iwinfo`/`iw`/`IEEE802dot11-MIB`); (5) contoh konkret `extend`/`pass_persist`; (6) opsi keterjangkauan di balik NAT/CGNAT; (7) beban polling; (8) semantik offline vs unreachable; (9) trade-off arsitektur push vs pull; (10) daftar OID siap pakai.

Isu GitHub yang terpengaruh langsung: **#19** ("Monitoring adapter SNMP v2c + poller @5 menit" — poller, status online/offline/unreachable, community read-only unik per `AccessPoint`), **#20** ("Metrik AccessPoint (uptime, CPU/mem, signal/noise, client, traffic)" — menyebut eksplisit "CPU/memori (HOST-RESOURCES-MIB)" dan "signal/noise per radio & jumlah client (skrip extend baca ubus/hostapd)"), **#23** ("Alert AccessPoint offline (kecuali unreachable)"). Verdict di bawah juga menyentuh kriteria sukses **T3** di `docs/plans/0001-mvp-scope.md`.

**Versi rujukan yang dipakai** (semua diverifikasi pada 2026-09-27):

| Komponen | Versi/commit rujukan | Sumber |
|---|---|---|
| OpenWrt | rilis stabil terkini **25.12.5** (2026-06-30); seri 24.10 terakhir **24.10.8** (2026-07-25) | [S19] |
| Feed `packages` | `master` @ `4a69a0b76759dcd6e2164c81d7260b5280d833c1` (commit 2026-09-25); branch `openwrt-24.10`, `openwrt-25.12` | [S1][S2][S3] |
| net-snmp | **5.9.4-6** (branch `openwrt-24.10` dan `openwrt-25.12`), **5.9.5.2-2** (`master`) | [S1][S2][S3] |
| iwinfo | `66bdd1a071895d91babc9b9228bb84626bbce226`, `PKG_SOURCE_DATE 2026-05-26` | [S7] |
| iw | 6.17 (varian `iw` = `tiny` default, `iw-full`) | [S9] |
| Kernel OpenWrt | `target/linux/generic/config-6.12` | [S8] |

---

## Ringkasan

**Cakupan metrik ADR-0002 TIDAK realistis apa adanya dan perlu diturunkan.** Tiga klaim teknisnya terbantah atau harus diubah: (i) frasa "signal/noise per radio" tidak punya padanan di nl80211 maupun `iwinfo` — yang benar-benar ada adalah **signal (dan signal_avg) per stasiun/client** dan **noise floor kanal per interface** (satu nilai yang dipakai ulang untuk semua client), sedangkan **noise per client tidak ada sama sekali** di nl80211; (ii) angka-angka itu juga **tidak berada di MIB mana pun** yang aktif, sehingga jalur `extend`/`pass_persist` yang disebut ADR-0002 memang satu-satunya jalan — tetapi `pass_persist` tidak bisa mengembalikan tipe `counter64`, jadi metrik akumulatif harus dipotong atau di-encode sebagai string; (iii) `IEEE802dot11-MIB` memang **dikompilasi** (OpenWrt menambal net-snmp dengan implementasi penuh, lihat `net/net-snmp/patches/750-ieee802dot11.patch`), **tetapi** implementasi itu bergantung pada Linux **Wireless Extensions** sementara kernel OpenWrt mematikan WEXT (`# CONFIG_WIRELESS_EXT is not set`, `# CONFIG_WEXT_CORE is not set`, `# CONFIG_WEXT_PROC is not set`) — sehingga MIB itu praktis mati pada AccessPoint mac80211 modern. Sebaliknya, kabar baiknya: paket `snmpd` ada di feed resmi dengan **konfigurasi UCI resmi** (`/etc/config/snmpd` → `/var/run/snmpd.conf`), HOST-RESOURCES-MIB **benar-benar masuk daftar modul yang dikompilasi**, IF-MIB + ifXTable menyediakan traffic RX/TX 64-bit, dan UCD-SNMP-MIB (`ssCpuRaw*`, `mem*`) memberi alternatif CPU/memori yang lebih andal.

**Keterjangkauan: SNMP pull tidak bisa diselamatkan dengan "reverse SSH tunnel" biasa, dan tidak ada paket TR-069/TR-369 resmi di OpenWrt — jadi rekomendasi MVP berubah.** SSH port forwarding hanya mendefinisikan channel TCP (`forwarded-tcpip`/`direct-tcpip`, RFC 4254 §7.2); tidak ada channel UDP, sedangkan SNMP berjalan di UDP/161. Jadi `ssh -R` tidak cukup untuk mem-poll SNMP dari balik NAT — perlu relay UDP↔TCP (mis. `socat`) di kedua sisi, yang menambah komponen dan titik gagal. Untuk TR-069/CWMP dan TR-369/USP: setelah menelusuri seluruh pohon feed `packages` resmi (`master` @ 2026-09-25) **tidak ada** direktori `net/icwmp`, `net/usp`, maupun jejak `cwmp`/`obuspa`/`uspd` — implementasi yang ada (icwmp, EasyCwmp, FreeCWMP, OB-USPA, feed_usp SoftAtHome) semuanya **feed pihak ketiga**, dengan EasyCwmp berhenti di OpenWrt 19.07. Yang tersedia resmi dan matang untuk kebutuhan "AP harus menghubungi server" hanyalah paket SSH **`autossh`** dan **`sshtunnel`** (keduanya terverifikasi ada di feed resmi), dan keduanya tetap perlu pelengkap relay UDP untuk SNMP. **Rekomendasi tegas: pertahankan SNMP pull sebagai adapter pertama hanya untuk `AccessPoint` yang benar-benar punya jalur IP inbound (sesuai T3 yang memang sudah membatasi "untuk AP terjangkau SNMP"), turunkan cakupan metrik WiFi seperti tabel di bawah, dan jadikan agent push (HTTPS keluar dari AP) sebagai adapter kedua yang dinaikkan ke MVP bila pilot membuktikan mayoritas AP di balik NAT** — tanpa mengubah ADR-0002 sendiri (ADR itu sudah netral protokol, jadi tidak ada yang perlu dibatalkan; yang perlu diperbaiki adalah *istilah* metrik di ADR-0002 dan asumsi "tunnel = solusi" di bagian Alasan ADR itu).

---

## Tabel ringkasan temuan

| # | Topik | Verdict | Basis bukti |
|---|---|---|---|
| 1 | Paket SNMP di OpenWrt | **Ada & resmi**: paket sumber `net-snmp` (feed `packages`, `net/`), menghasilkan binary `snmpd`/`snmpd-nossl`/`snmpd-ssl`, `snmp-utils`, `snmp-mibs`, `libnetsnmp*`; tersedia di 24.10 **dan** 25.12; ada UCI resmi `/etc/config/snmpd` | [S1][S2][S3][S4][S5] |
| 2 | MIB terkompilasi | **SNMPv2-MIB, IF-MIB (+ifXTable), HOST-RESOURCES-MIB (hr_proc/hr_storage/hr_system/dst), UCD-SNMP-MIB (memory/loadave/vmstat/pass/pass_persist), NET-SNMP-EXTEND-MIB, IEEE802dot11-MIB** ikut dikompilasi (daftar `SNMP_MIB_MODULES_INCLUDED`), sementara grup `host`, `if-mib`, `mibII`, `ucd_snmp`, `hardware`, `notification` **dikecualikan** sebagai grup | [S3][S4] |
| 3 | CPU/memori | **HOST-RESOURCES-MIB ada tapi rapuh** (`hrProcessorLoad` dihitung dari history CPU internal dan bisa mengembalikan `noSuchInstance`); **UCD-SNMP-MIB lebih andal** untuk CPU (`ssCpuRawUser/Nice/System/Idle`) dan memori (`memTotalReal/memAvailReal/memTotalFree/memBuffer/memCached`) | [S3][S11][S12][S13] |
| 4 | Traffic & uptime | **Tersedia standar**: `sysUpTime` (SNMPv2-MIB), `ifHCInOctets`/`ifHCOutOctets` + `ifOperStatus` + `ifName` (IF-MIB/ifXTable). Interface `wlan*`/`phyX-apY` adalah netdev biasa sehingga ikut ter-enumerasi | [S6][S16] |
| 5 | Interface wlan di IF-MIB | **Termasuk** (traffic per-interface), tetapi **IF-MIB tidak memuat signal/noise/jumlah client** — tidak ada kolom seperti itu di RFC 2863 | [S16] |
| 6 | Signal/noise per radio | **Tidak ada sebagai "per radio"**. Yang ada: `signal` + `signal_avg` **per stasiun** (nl80211 `NL80211_STA_INFO_SIGNAL`/`_SIGNAL_AVG`) dan `noise` = **noise floor kanal** dari survey (nl80211 `NL80211_SURVEY_INFO_NOISE`) | [S7][S10][S10b] |
| 7 | Noise per stasiun | **TIDAK ADA**. Tidak ada `NL80211_STA_INFO_NOISE` di `enum nl80211_sta_info`; iwinfo mengisi field `noise` tiap baris assoclist dengan nilai noise kanal yang sama (`e->noise = 0; /* filled in by caller */` lalu diisi sekali dari survey) | [S7][S10] |
| 8 | Jumlah client | **Tidak ada di MIB aktif**; dihitung dari `ubus call iwinfo assoclist` (jumlah elemen `results`) atau `iw dev <dev> station dump`. Catatan risiko: buffer `IWINFO_BUFSIZE 24 * 1024` dan isu upstream rpcd crash pada ±150+ client | [S7][S10][S12b] |
| 9 | IEEE802dot11-MIB | **Dikompilasi tapi efektif mati**: implementasi OpenWrt memakai ioctl WEXT (`SIOCGIWNAME`, `SIOCGIWRANGE`) + `/proc/net/wireless`, sementara kernel OpenWrt mematikan WEXT | [S3][S8][S14] |
| 10 | `extend` vs `pass_persist` | Bisa dipakai keduanya dan sudah ada dukungan UCI (`config extend`, `config pass`); `pass_persist` tidak mendukung `counter64` dan `noSuchObject`, `extend` mengembalikan `DisplayString` + punya `cacheTime` | [S4][S5][S12][S15] |
| 11 | Keterjangkauan AP di balik NAT | **SNMP pull tidak bisa**, dan reverse SSH **tidak bisa langsung** (forwarding SSH hanya TCP). Jalur realistis: relay UDP↔TCP di atas tunnel SSH (kompleks), atau push agent keluar dari AP | [S12][S17] |
| 12 | TR-069/CWMP resmi OpenWrt | **Tidak ada di feed resmi.** icwmp hanya via feed Iopsys; EasyCwmp terakhir mendukung 19.07 (berhenti Des 2019, dicek Jun 2024); FreeCWMP legacy | [S1][S18] |
| 13 | TR-369/USP resmi OpenWrt | **Tidak ada di feed resmi** (tidak ada `net/usp`, tidak ada `obuspa`/`uspd`). Hanya feed pihak ketiga (SoftAtHome `feed_usp`, prpl) atau bangun OB-USPA sendiri | [S1][S18][S21] |
| 14 | Push sederhana rakitan sendiri | **Layak** sebagai adapter kedua: AP sudah terbiasa menginisiasi koneksi keluar (tunnel RADIUS di `0001-mvp-scope.md`, dan ADR-0001 sudah membedakan AP terjangkau vs di balik NAT) | [S20] (interpretasi peneliti) |
| 15 | UCI: community read-only + batas IP | **Didukung**: `com2sec` menerima `default`, host/IP tunggal, atau subnet `IP/MASK`/`IP/BITS`; harus mengganti default pabrik (`public`/`private`, `source default`, `write all`) | [S4][S5][S15] |
| 16 | Beban polling 5 menit | **Tidak ada sumber primer** yang menetapkan ambang jumlah OID aman untuk router kelas bawah → wajib diukur sendiri; perkiraan desain peneliti: puluhan varbind / 1–3 request per AP per siklus [INDIKATIF] | — (lihat §7) |
| 17 | Semantik offline vs unreachable | **Tidak bisa dipisahkan hanya dari lapisan SNMP**: klien hanya membedakan `STAT_TIMEOUT` dan `STAT_ERROR` (kode `apps/snmpstatus.c`), dan community salah/pembatasan sumber menghasilkan *timeout* yang identik dengan host mati. Pembedaan harus dari pengamatan berlapis (ICMP/TCP, riwayat, metadata "AP di balik NAT") | [S12b][S17] |
| 18 | Kepatuhan terhadap T3 MVP | **Sebagian**: T3 sudah membatasi diri pada "AP terjangkau SNMP", jadi SNMP pull tidak melanggar T3. Yang melanggar janji adalah daftar metrik T3 ("signal/noise per radio, jumlah client") bila diartikan apa adanya | [S20] |

---

## Detail per pertanyaan

Legenda keyakinan: **[TERVERIFIKASI]** = dikutip dari sumber primer (kode/config upstream, dokumen IETF, man page upstream). **[INDIKATIF]** = hanya forum/blog/posting — bukan bukti. **[INFERENSI]** = kesimpulan peneliti, bukan teks sumber. **[PERLU UJI LOKAL]** = butuh perangkat/lingkungan yang tidak tersedia di sesi riset ini.

### 1. Paket SNMP di OpenWrt dan UCI resminya

**[TERVERIFIKASI]** Paket sumbernya bernama **`net-snmp`** dan berada di feed resmi **`packages`**, subdirektori **`net/net-snmp`** (bukan di repo inti `openwrt/openwrt`). Daftar paket yang dihasilkan `net/net-snmp/Makefile` [S3][S4]:

| Binary package | Isi | Catatan |
|---|---|---|
| `snmpd-nossl` / `snmpd-ssl` | daemon `snmpd` | keduanya `PROVIDES:=snmpd`; varian `nossl` adalah default (`libnetsnmp-nossl` `DEFAULT_VARIANT:=1`). UCI `/etc/config/snmpd` adalah `conffiles` paket |
| `snmp-utils-nossl` / `snmp-utils-ssl` | `snmpget`, `snmpset`, `snmpstatus`, `snmptest`, `snmptrap`, `snmpwalk` | berguna untuk verifikasi lokal |
| `snmp-mibs` | berkas MIB di `/usr/share/snmp/mibs` | tidak perlu untuk poller numerik |
| `libnetsnmp-nossl` / `-ssl` | pustaka bersama | dependensi: `+libnl-tiny +libpci +libpcre2` |
| `snmptrapd-*` | daemon penerima notifikasi | opsional |

Versi: **5.9.4-6** pada branch `openwrt-24.10` dan `openwrt-25.12`; **5.9.5.2-2** pada `master` [S1][S2][S3]. Tersedia di rilis terkini (25.12.x) — branch paket `openwrt-25.12` sudah ada di feed [S1], dan halaman indeks paket OpenWrt untuk `snmpd` ada [S5] (catatan: halaman indeks itu **stale** — menampilkan `5.9.1-7`, versi era 22.03; jangan pakai sebagai rujukan versi).

**UCI resmi: ada.** `/etc/config/snmpd` (dipasang dari `net/net-snmp/files/snmpd.conf`) diubah menjadi `/var/run/snmpd.conf` oleh `/etc/init.d/snmpd` setiap start/reload, lalu dijalankan sebagai:

```
procd_set_param command $PROG -f -r -p "$pid_file"
procd_append_param command -C -c "$CONFIGFILE"
```

Itu sebabnya menyunting `/etc/snmp/snmpd.conf` langsung **tidak berpengaruh** — file itu hanya symlink/generated. Section UCI yang dikenali init script [S4]: `agent`, `agentx`, `system`, `com2sec`, `com2sec6`, `group`, `view`, `access`, `trap_HostName`, `trap_HostIP`, `access_default`, `access_HostName`, `access_HostIP`, `pass`, `exec`, `extend`, `disk`, `engineid`, `trapcommunity`, `trapsink`, `trap2sink`, `informsink`, `authtrapenable`, `v1trapaddress`, `trapsess`, `v3`, `logging`, dan `snmpd general`.

Struktur minimal yang memenuhi persyaratan keamanan `0001-mvp-scope.md` ("community string read-only & unik per-`AccessPoint`"; upgrade SNMPv3 dipertimbangkan) — pakai **VACM lengkap** (`com2sec`+`group`+`view`+`access`), karena init script **tidak** mengekspos shorthand `rocommunity`:

```
# /etc/config/snmpd — contoh rancangan; verifikasi lokal (uci show snmpd; snmpwalk dari poller)
config agent
	option agentaddress 'UDP:161'

config system
	option sysName 'ap-cust-001'
	option sysLocation 'site-001'

# community read-only unik per AccessPoint, dibatasi ke IP poller saja
config com2sec cuanku_ro
	option secname 'ro_cuanku'
	option source '10.20.0.7'          # atau subnet: '10.20.0.0/24'
	option community '<community-unik-per-AP>'

config group cuanku_ro_v2c
	option group 'cuanku'
	option version 'v2c'
	option secname 'ro_cuanku'

config view cuanku_view
	option viewname 'cuanku'
	option type 'included'
	option oid '.1'                     # persempit bila perlu (mis. .1.3.6.1 + .1.3.6.1.4.1.8072.1.3)

config access cuanku_access
	option group 'cuanku'
	option context 'none'
	option version 'v2c'
	option level 'noauth'
	option prefix 'exact'
	option read 'cuanku'
	option write 'none'
	option notify 'none'

config snmpd general
	option enabled '1'
	# 'list network' HANYA untuk interface tempat poller berada (membuka UDP/161 di zona itu)
	# list network 'lan'
```

Fakta-fakta verbatim yang mendasari contoh di atas:
- Sintaks direktif: `com2sec [-Cn CONTEXT] SECNAME SOURCE COMMUNITY` — *"A restricted source can either be a specific hostname (or address), or a subnet - represented as IP/MASK (e.g. 10.10.10.0/255.255.255.0), or IP/BITS (e.g. 10.10.10.0/24), or the IPv6 equivalents, or a netgroup"* [S15].
- Init script menulis baris `com2sec $secname $source $community`, `group $group $version $secname`, `view $viewname $type $oid $mask`, `access $group $context $version $level $prefix $read $write $notify` [S4].
- `option context 'none'` diterjemahkan menjadi `""` oleh init script [S4].
- Konfigurasi pabrik yang dikirim paket **berbahaya dan harus diganti**: ada `config com2sec public` dengan `option source default` + `option community public`, dan `config com2sec private` dengan `community private` yang dipetakan ke grup dengan `option write all` [S4]. Mengabaikan ini = AP menyiarkan SNMP read-write lintas jaringan dengan community default.

### 2. MIB/modul yang benar-benar terkompilasi

**[TERVERIFIKASI]** Daftar resmi dari `net/net-snmp/Makefile` (branch `master`; identik di 24.10/25.12) [S3][S4]:

```
SNMP_MIB_MODULES_INCLUDED = \
	agent/extend \  agentx \
	host/hr_device  host/hr_disk  host/hr_filesys  host/hr_network \
	host/hr_partition  host/hr_proc  host/hr_storage  host/hr_system \
	ieee802dot11 \
	if-mib/ifXTable \
	ip-mib/ipAddressTable  ip-mib/inetNetToMediaTable \
	ip-forward-mib/inetCidrRouteTable  ip-forward-mib/ipCidrRouteTable \
	mibII/at  mibII/icmp  mibII/ifTable  mibII/ip  mibII/snmp_mib \
	mibII/sysORTable  mibII/system_mib  mibII/tcp  mibII/udp \
	mibII/vacm_context  mibII/vacm_vars \
	snmpv3/snmpEngine  snmpv3/snmpMPDStats  snmpv3/usmConf  snmpv3/usmStats  snmpv3/usmUser \
	tunnel \
	ucd-snmp/disk_hw  ucd-snmp/dlmod  ucd-snmp/extensible  ucd-snmp/loadave \
	ucd-snmp/memory  ucd-snmp/pass  ucd-snmp/pass_persist  ucd-snmp/proc  ucd-snmp/vmstat \
	util_funcs  utilities/execute

SNMP_MIB_MODULES_EXCLUDED = \
	agent_mibs  disman/event  disman/schedule  hardware  host  if-mib  ip-mib \
	mibII  notification  notification-log-mib  snmpv3mibs  target  tcp-mib \
	ucd_snmp  udp-mib  utilities
```

Arti praktisnya:

| MIB | Objek yang ditanyakan ADR-0002 | Status kompilasi |
|---|---|---|
| SNMPv2-MIB | `sysUpTime`, `sysDescr`, `sysName`, `sysObjectID` | **Kompilasi** (`mibII/system_mib`) [S3] |
| IF-MIB | `ifInOctets`/`ifOutOctets`, `ifOperStatus`, `ifName`/`ifDescr` | **Kompilasi** (`mibII/ifTable` + `if-mib/ifXTable` untuk versi 64-bit) [S3] |
| HOST-RESOURCES-MIB | `hrProcessorLoad`, `hrStorage`, `hrSystemUptime` | **Kompilasi** (`host/hr_proc`, `host/hr_storage`, `host/hr_system`) [S3] |
| UCD-SNMP-MIB | `ssCpuRaw*`, `mem*` | **Kompilasi** (`ucd-snmp/vmstat`, `ucd-snmp/memory`, `ucd-snmp/loadave`) [S3] |
| NET-SNMP-EXTEND-MIB | `nsExtend*` untuk skrip WiFi | **Kompilasi** (`agent/extend`) [S3] |
| UCD extensibility | `exec` lama | **Kompilasi** (`ucd-snmp/extensible`) [S3] |
| IEEE802dot11-MIB | `dot11*` | **Kompilasi** (`ieee802dot11`, plus patch 219 KB) — **tapi efektif tidak berfungsi, lihat §4** [S3][S14] |
| NET-SNMP-AGENT-MIB | `nsNotifyStart` dll | `notification` **dikecualikan**, tetapi `agentx`/`util_funcs` disertakan — notifikasi dasar tidak jadi andalan [S3] |

Catatan penting yang **membatalkan asumsi umum**:
- Grup `host`, `if-mib`, `mibII`, `ucd_snmp`, `hardware` memang ada di daftar *excluded*, **tetapi** masing-masing modul anaknya (`host/hr_proc`, `if-mib/ifXTable`, `mibII/ifTable`, `ucd-snmp/vmstat`, …) dicantumkan eksplisit di daftar *included*. Jangan menyimpulkan "HOST-RESOURCES-MIB tidak ada" hanya dari kata `host` di daftar excluded.
- Modul `hardware` (termasuk `hardware/cpu`, yang dipakai `hr_proc.c` lewat `netsnmp_cpu_get_byIdx`) **tidak** dicantumkan eksplisit, padahal `hr_proc.c` menyertakan `<net-snmp/agent/hardware/cpu.h>` dan memanggil API-nya [S11]. Paket ini ter-build di feed resmi, jadi dependensi itu pasti terpuaskan oleh mekanisme dependensi internal net-snmp **[INFERENSI]**, tetapi konsekuensi runtime-nya nyata: lihat §3.
- Opsi build relevan: `--disable-debugging`, `--disable-manuals`, `--disable-scripts`, `--with-pcre2-8`, `--with-nl`, `--without-openssl` (varian nossl), transport hanya `Callback UDP Unix` (+`UDPIPv6` bila IPv6 aktif); transport **TCP dimatikan** (`SNMP_TRANSPORTS_EXCLUDED = TCP TCPIPv6`) [S3].
- Cara **memverifikasi daftar modul di perangkat**: dokumentasi net-snmp menyarankan `snmpd -Dmib_init -H`, *"assumes you have debugging support compiled in"* — **tidak berlaku** di OpenWrt karena `--disable-debugging`. Verifikasi praktis yang bisa dipakai: telusuri subtree dengan `snmpwalk` dari `snmp-utils`:
  ```
  snmpwalk -v2c -c <community> <ip-ap> 1.3.6.1.2.1.1
  snmpwalk -v2c -c <community> <ip-ap> 1.3.6.1.2.1.2.2          # ifTable
  snmpwalk -v2c -c <community> <ip-ap> 1.3.6.1.2.1.31.1.1         # ifXTable
  snmpwalk -v2c -c <community> <ip-ap> 1.3.6.1.2.1.25            # host resources
  snmpwalk -v2c -c <community> <ip-ap> 1.3.6.1.4.1.2021          # UCD
  snmpwalk -v2c -c <community> <ip-ap> 1.3.6.1.2.1.34            # IEEE802dot11
  ```

### 3. CPU/memori: HOST-RESOURCES-MIB vs UCD-SNMP-MIB

**[TERVERIFIKASI dari kode]** `hrProcessorLoad` (`.1.3.6.1.2.1.25.3.3.1.2`) diimplementasikan di `agent/mibgroup/host/hr_proc.c` dan **bukan** pembacaan langsung `/proc/stat`; ia menghitung selisih tick idle/total terhadap `cpu->history[0]`:

```c
cpu = netsnmp_cpu_get_byIdx( proc_idx & HRDEV_TYPE_MASK, 0 );
if ( !cpu || !cpu->history || !cpu->history[0].total_hist ||
   ( cpu->history[0].total_hist == cpu->total_ticks ))
    return NULL;                       /* -> noSuchInstance di sisi klien */
value = (cpu->idle_ticks  - cpu->history[0].idle_hist)*100;
value /= (cpu->total_ticks - cpu->history[0].total_hist);
long_return = 100 - value;             /* hrProcessorLoad = 100 - idle% */
```
[S11]

Konsekuensi yang harus dipahami poller: kalau history belum terisi atau tick tidak berubah antar dua sampel, agent mengembalikan **NULL** → klien menerima `noSuchInstance`, bukan angka. Selain itu kolom `hrProcessorFrwID` (`.25.3.3.1.1`) selalu mengembalikan `nullOid` [S11]. Definisi normatifnya pun menyatakan nilai itu *"The average, over the last minute, of the percentage of time that this processor was not idle"* (RFC 2790) [S13] — resolusi 1 menit, tidak cocok untuk polling 5 menit. Jadwal polling #19 (5 menit) memang lebih panjang dari jendela 1 menit, jadi nilainya hanya berguna sebagai *spot sample*.

**Rekomendasi untuk #20: pakai UCD-SNMP-MIB untuk CPU.** `ssCpuRawUser(50)`, `ssCpuRawNice(51)`, `ssCpuRawSystem(52)`, `ssCpuRawIdle(53)` adalah Counter32 kumulatif sehingga poller bisa menghitung persentase **rata-rata sepanjang interval 5 menit** (selisih dua sampel), yang justru lebih benar daripada satu snapshot. MIB-nya sendiri menulis: *"The number of 'ticks' (typically 1/100s) spent idle. On a multi-processor system, the 'ssCpuRaw*' counters are cumulative over all CPUs…"* [S12]. Objek `ssCpuUser/System/Idle` yang lama sudah `deprecated` [S12].

Perhatikan catatan MIB yang penting sebelum mengandalkan UCD: *"Not supported on all systems! See agent/mibgroup/ucd_snmp.h to see if its loaded for your architecture"* [S12]. Untuk OpenWrt, jawabannya ada di daftar `SNMP_MIB_MODULES_INCLUDED` (§2) — modul `ucd-snmp/memory`, `ucd-snmp/vmstat`, `ucd-snmp/loadave` **disertakan**, jadi seharusnya ada.

Memori: `memTotalReal` (…2021.4.5), `memAvailReal` (.6), `memTotalFree` (.11), `memBuffer` (.14), `memCached` (.15) — semua `Integer32 UNITS "kB"`; ada juga varian 64-bit `memTotalRealX/memAvailRealX/memTotalFreeX` (…4.20/21/22) [S12]. **Catatan penting**: nilainya `Integer32` bertanda, jadi nilai > 2 GiB (seperti `memTotalFree`) bisa tampak negatif/overflow — verifikasi di perangkat dengan RAM besar atau pakai varian `X` bila tersedia [INFERENSI dari tipe SMI di MIB, [S12]].

HOST-RESOURCES-MIB tetap berguna untuk: `hrSystemUptime` (`.25.1.1.0`, TimeTicks, definisi *"amount of time since this host was last initialized. Note that this is different from sysUpTime"* [S13]) dan `hrMemorySize` (`.25.2.2.0`, KBytes, *"physical read-write main memory, typically RAM"* [S13]). `hrStorageTable` (`.25.2.3`) memberi `hrStorageAllocationUnits`(kolom 4)/`hrStorageSize`(5)/`hrStorageUsed`(6) per area penyimpanan [S13] — berguna untuk memantau flash/overlay, **tetapi apakah tabel ini terisi di OpenWrt [PERLU UJI LOKAL]** (di Linux net-snmp menurunkannya dari daftar mount; jangan diasumsikan terisi).

Rangkuman keputusan untuk #20: `sysUpTime` + `hrSystemUptime` = uptime; `ssCpuRawIdle`+`ssCpuRawUser`+`ssCpuRawSystem` = CPU%; `memTotalReal`/`memAvailReal`(+`memTotalFree`) = memori; `hrProcessorLoad`/`hrStorage*` = cadangan yang **wajib diuji ada/tidaknya** sebelum dipakai.

### 4. Metrik WiFi: signal/noise, jumlah client, dan jalur eksposnya

**(a) Apakah IF-MIB mencakup interface `wlan`?** **[TERVERIFIKASI secara struktural]** IF-MIB tidak punya kolom khusus wireless: `ifTable`/`ifXTable` mendefinisikan `ifIndex`, `ifDescr`, `ifType`, `ifSpeed`, `ifPhysAddress`, `ifOperStatus`, `ifInOctets`, `ifOutOctets`, `ifName`, `ifHCInOctets`, `ifHCOutOctets`, `ifHighSpeed`, `ifAlias` — tidak ada signal/noise/jumlah stasiun [S16]. Interface wireless OpenWrt adalah netdev biasa sehingga **muncul** di ifTable/ifXTable dan traffic RX/TX-nya terhitung; **tetapi tidak ada metrik radio di sana**. Verifikasi lokal: `snmpwalk -v2c -c <community> <ip> 1.3.6.1.2.1.31.1.1.1.1` (daftar `ifName`) lalu bandingkan dengan `iw dev`.

**(b) IEEE802dot11-MIB: jangan diandalkan.** OpenWrt membawa patch khusus `net/net-snmp/patches/750-ieee802dot11.patch` (219.802 byte) yang menambahkan `agent/mibgroup/ieee802dot11.c` (+4917 baris), `ieee802dot11.h` (+732 baris), dan `iwlib.h` (+509 baris, dari Wireless Tools) serta menyuntikkannya ke `agent/mib_modules.c` [S14]. Dua kutipan dari patch itu menentukan segalanya:

```c
#include "iwlib.h"
#define DISPLAYWIEXT // display wireless ext info
#define PROC_NET_WIRELESS "/proc/net/wireless"
/* loadWiExt() - load wireless extensions structures; use ioctl calls and read /proc/net/wireless */
if ( ioctl ( skfd, SIOCGIWNAME, &wrq ) >= 0 ) { ... }
if ( ioctl ( skfd, SIOCGIWRANGE, &wrq ) >= 0 ) { ... }
```
[S14]

Artinya modul ini berbasis **Linux Wireless Extensions (WEXT)**. Kernel OpenWrt mematikan WEXT di konfigurasi generiknya [S8]:

```
# CONFIG_WEXT_CORE is not set
# CONFIG_WEXT_PROC is not set
# CONFIG_WEXT_PRIV is not set
# CONFIG_WEXT_SPY is not set
# CONFIG_WIRELESS_EXT is not set
```

→ **[INFERENSI, keyakinan tinggi]** pada AccessPoint OpenWrt 24.10/25.12 dengan driver mac80211 modern (ath9k/ath10k/ath11k/mt76), subtree `1.3.6.1.2.1.34` akan kosong walau modulnya ter-kompilasi, karena ioctl WEXT tidak akan berhasil dan `/proc/net/wireless` kosong/tidak ada. Konfigurasi spesifik target secara teori bisa menimpanya, jadi wajib dikonfirmasi di perangkat [PERLU UJI LOKAL]:

```
cat /proc/net/wireless                      # kosong / tidak ada => WEXT mati
snmpwalk -v2c -c <community> <ip> 1.3.6.1.2.1.34   # kosong => MIB tidak berguna
```

**Kesimpulan Q4a/Q4b: jalur `IEEE802dot11-MIB` ditolak; satu-satunya jalur yang layak adalah skrip yang membaca `iwinfo`/`iw`.**

**(c) Field yang benar-benar ada di `iwinfo` (dari kode rpcd + pustaka iwinfo).** Sumber primer: `openwrt/rpcd` → `iwinfo.c` (plugin `rpcd-mod-iwinfo`) dan `openwrt/iwinfo` → `include/iwinfo.h` + `iwinfo_nl80211.c` [S7][S10].

`ubus call iwinfo assoclist '{"device":"phy0-ap0"}'` mengembalikan `results[]` dengan field berikut (verbatim dari `rpc_iwinfo_assoclist`) [S7]:
`mac`, `signal`, `signal_avg`, `noise`, `inactive`, `connected_time`, `thr`, `authorized`, `authenticated`, `preamble`, `wme`, `mfp`, `tdls`, `mesh llid`, `mesh plid`, `mesh plink`, `mesh local PS`, `mesh peer PS`, `mesh non-peer PS`, lalu sub-tabel `rx` (`drop_misc`, `packets`, `bytes`, + rateinfo) dan `tx` (`failed`, `retries`, `packets`, `bytes`, + rateinfo), dengan rateinfo berisi `ht`/`vht`/`he`/`eht`, `mhz`, `rate`, `mcs`, `nss`, `40mhz`, `short_gi`, `he_gi`, `he_dcm`, `eht_gi`.

`ubus call iwinfo info '{"device":"phy0-ap0"}'` mengembalikan antara lain `quality`, `quality_max`, `signal`, `noise`, `bitrate`, `encryption`, `htmodes`, `hardware` (id/nama), plus mode/ssid/bssid/channel/center_chan1/… [S7].

`ubus call iwinfo survey '{"device":"phy0-ap0"}'` mengembalikan `results[]`: `mhz`, `noise`, `active_time`, `busy_time`, `busy_time_ext`, `rx_time`, `tx_time` [S7].

**Temuan kritis — "noise per stasiun" tidak ada.** Di `iwinfo_nl80211.c`:

```c
if (sinfo[NL80211_STA_INFO_SIGNAL])     e->signal     = nla_get_u8(...);
if (sinfo[NL80211_STA_INFO_SIGNAL_AVG]) e->signal_avg = nla_get_u8(...);
...
e->noise = 0; /* filled in by caller */
```
lalu di `nl80211_get_assoclist()`:
```c
if (!nl80211_get_noise(ifname, &noise))
    for (i = 0, e = arr.buf; i < arr.count; i++, e++) e->noise = noise;
```
[S10]

Jadi field `noise` di baris assoclist **sama untuk semua stasiun** dan berasal dari noise floor **kanal** (`NL80211_CMD_GET_SURVEY` → `NL80211_SURVEY_INFO_NOISE`, *"noise level of channel (u8, dBm)"*). Konfirmasi silang dari header uapi nl80211: di `enum nl80211_sta_info` **tidak ada** `NL80211_STA_INFO_NOISE` (yang ada: SIGNAL, SIGNAL_AVG, CHAIN_SIGNAL, CHAIN_SIGNAL_AVG, ACK_SIGNAL, ACK_SIGNAL_AVG, …), sedangkan `NOISE` hanya ada di `enum nl80211_survey_info` [S10b].

**(d) Field `iw dev <dev> station dump`.** Dari sumber `iw` (station.c) [S11b], baris yang dicetak antara lain: `inactive time`, `rx bytes`, `rx packets`, `tx bytes`, `tx packets`, `tx retries`, `tx failed`, `rx drop misc`, `signal` (dengan rantai per-antena `[a, b]`), `signal avg`, `beacon signal avg`, `Toffset`, `tx bitrate` (MCS/VHT/HE/EHT, lebar kanal, short GI), `rx bitrate`, `expected throughput`, `connected time`, `associated at`, plus flag `authorized`/`authenticated`/`associated`/`preamble`/`WMM/WME`/`MFP`/`TDLS`. **Tidak ada `noise`** — konsisten dengan tidak adanya `NL80211_STA_INFO_NOISE` [S10b][S11b]. Nilai noise level kanal hanya keluar dari **survey**, dan nama field-nya `noise` (format persis keluaran CLI `iw ... survey dump` belum diverifikasi di sesi ini; sumber primer yang tersedia adalah definisi atributnya di nl80211 [S10b]).

**(e) Jumlah client.** Tidak ada di MIB aktif. Cara: hitung elemen `results` dari `ubus call iwinfo assoclist`, atau `iw dev <dev> station dump | grep -c '^Station'`, atau `hostapd_cli all_sta` (keberadaan `hostapd_cli` dan subperintah `all_sta` di image OpenWrt target **perlu diverifikasi lokal** — tidak diuji di sesi ini). **Risiko yang harus dicatat untuk #20**: `iwinfo` memakai buffer tetap `#define IWINFO_BUFSIZE 24 * 1024` di `include/iwinfo.h` [S10] dan ada isu upstream yang melaporkan segfault/crash rpcd pada ±150 client terhubung [S12b]. Ini **bukan** masalah hipotetis untuk AP dengan banyak client; mitigasi: pakai `iw ... station dump` (yang tidak memakai buffer itu) untuk menghitung client, atau baca `hostapd_cli`. **[INDIKATIF untuk besaran angkanya — isu repo upstream tahun 2019, ukuran struct sejak itu bertambah; ukuran buffer 24 KB adalah [TERVERIFIKASI] dari header]**.

**(f) Kesimpulan ekspos via net-snmp.** Tiga kelompok metrik WiFi dengan sifat berbeda:

| Metrik | Sumber nyata | Tipe | Catatan |
|---|---|---|---|
| Jumlah client per interface | `iwinfo assoclist` (jumlah baris) / `iw station dump` | integer | per-interface, akurat |
| Signal per client (terlemah, rata-rata, terbaik) | `assoclist[].signal` / `station dump signal` | integer (dBm, negatif) | per-client; agregasi ke "per radio" = keputusan poller, bukan sifat perangkat |
| Signal avg per client | `assoclist[].signal_avg` / `signal avg` | integer | lebih stabil untuk tren |
| Noise (floor kanal) per interface | `iwinfo info.noise` / `iwinfo survey[].noise` / `iw survey` | integer (dBm, negatif) | **per kanal/interface, bukan per client** |
| Noise per client | — | — | **TIDAK ADA** di nl80211/iw/iwinfo |
| Bitrate per client | `assoclist[].rx.rate`/`tx.rate` / `station dump tx bitrate` | integer (100 kbit/s di nl80211, lihat rateinfo `mhz`+`rate`) | opsional |

### 5. Contoh konkret `extend` / `pass_persist` di OpenWrt

**[TERVERIFIKASI sintaks]** `snmpd.conf(5)` mendefinisikan dua direktif yang dipakai di sini [S15]:

```
extend [-cacheTime TIME] [-execType TYPE] [MIBOID] NAME PROG ARGS
pass [-p priority] MIBOID PROG
pass_persist [-p priority] MIBOID PROG
```

Kutipan penting untuk pemilihan mekanisme:
- `extend`: *"works in a similar manner to the exec directive, but with a number of improvements. The MIB tables (nsExtendConfigTable etc) are indexed by the NAME token, so are unaffected by the order in which entries are read from the configuration files. There are two result tables…"*, dan output di-cache selama `nsExtendCacheTime` — di MIB: *"The length of time for which the output of this command will be cached. During this time, retrieving the output-related values will not reinvoke the command."* [S15][S15b]
- `pass`: *"The PROG command should return the response varbind as three separate lines printed to stdout - the first line should be the OID of the returned value, the second should be its TYPE (one of the text strings integer, gauge, counter, timeticks, ipaddress, objectid, octet, or string), and the third should be the value itself."* **Batasan kritis**: *"The SMIv2 type counter64 and SNMPv2 noSuchObject exception are not supported."* [S15]
- `pass_persist`: *"will also pass control of the subtree rooted at MIBOID to the specified PROG command. However this command will continue to run after the initial request has been answered, so subsequent requests can be processed without the startup overheads. Upon initialization, PROG will be passed the string "PING\n" on stdin, and should respond by printing "PONG\n" to stdout. For GET and GETNEXT requests, PROG will be passed two lines on stdin, the command (get or getnext) and the requested OID."* Dan bila tidak bisa menjawab: *"it should print "NONE\n" to stdout (but continue running)."* [S15]

UCI-nya (dari `snmpd.init`, verbatim) [S4]:

```sh
snmpd_pass_add() {
	config_get miboid "$cfg" miboid; config_get prog "$cfg" prog
	config_get_bool persist "$cfg" persist 0
	[ $persist -ne 0 ] && pass='pass_persist'
	config_get priority "$cfg" priority
	priority=${priority:+-p $priority}
	echo "$pass $priority $miboid $prog" >> $CONFIGFILE
}
snmpd_extend_add() {
	config_get name "$cfg" name; config_get prog "$cfg" prog
	config_get args "$cfg" args; config_get miboid "$cfg" miboid
	echo "extend $miboid $name $prog $args" >> $CONFIGFILE
}
```

**Rekomendasi [INFERENSI peneliti, belum diuji]:** pakai **`pass_persist`** untuk metrik numerik WiFi (menghindari spawn proses tiap query, dan memungkinkan tipe `integer`/`gauge` sehingga poller menerima angka, bukan string), dan pakai **`extend`** hanya untuk data tekstual/adhoc. Karena UCI tidak mengekspos `-cacheTime`, trik yang mungkin — **perlu diverifikasi lokal** — adalah mengisi `option miboid` dengan opsi net-snmp, sebab init script menyisipkan nilai itu tepat di posisi `[-cacheTime TIME] [-execType TYPE] [MIBOID]`:

```
config extend
	option name 'wifi'
	option miboid '-cacheTime 300'
	option prog '/usr/libexec/cuanku-apmetrics.sh'
# menghasilkan baris: extend -cacheTime 300 wifi /usr/libexec/cuanku-apmetrics.sh
```

**Contoh skrip `pass_persist` [INDIKATIF — contoh rancangan mengikuti protokol terdokumentasi; wajib diuji di perangkat]:**

```sh
#!/bin/sh
# /usr/libexec/cuanku-wifi-pass.sh
# Didaftarkan lewat:
#   config pass
#       option miboid  '.1.3.6.1.4.1.<PEN-Cuanku>.1'
#       option persist '1'
#       option prog    '/usr/libexec/cuanku-wifi-pass.sh'
# Protokol (snmpd.conf(5)): "PING" -> "PONG"; "get|getnext" + OID -> OID/TYPE/VALUE.
# Tipe yang diizinkan: integer, gauge, counter, timeticks, ipaddress, objectid, octet, string.
# counter64 TIDAK didukung.

OIDS_BASE='.1.3.6.1.4.1.<PEN-Cuanku>.1'

read -r line
[ "$line" = "PING" ] && printf 'PONG\n'

while read -r cmd; do
	read -r oid
	[ -z "$oid" ] && continue
	case "$oid" in
	  "$OIDS_BASE".1.0)                                    # jumlah client phy0-ap0
		json=$(ubus call iwinfo assoclist '{"device":"phy0-ap0"}' 2>/dev/null)
		n=$(printf '%s' "$json" | jsonfilter -e '@.results[*].mac' 2>/dev/null | grep -c .)
		printf '%s\ninteger\n%s\n' "$OIDS_BASE.1.0" "${n:-0}" ;;
	  "$OIDS_BASE".2.0)                                    # noise kanal phy0-ap0
		json=$(ubus call iwinfo info '{"device":"phy0-ap0"}' 2>/dev/null)
		noise=$(printf '%s' "$json" | jsonfilter -e '@.noise' 2>/dev/null)
		printf '%s\ninteger\n%s\n' "$OIDS_BASE.2.0" "$(( ${noise:-0} + 200 ))" ;;   # geser agar non-negatif
	  *) printf 'NONE\n' ;;
	esac
done
```

Catatan implementasi yang penting (dan mudah salah):
- `jsonfilter` adalah bagian dari OpenWrt dasar; `ubus` selalu ada; `rpcd-mod-iwinfo` **harus terpasang** agar objek `iwinfo` muncul di ubus — verifikasi dengan `ubus list iwinfo` [S7]. Kalau tidak ada, CLI `iwinfo <dev> info`/`assoclist` (paket `iwinfo`) adalah alternatif, tetapi **format teks CLI-nya belum diverifikasi di riset ini** [PERLU UJI LOKAL].
- Nilai `signal`/`noise` yang keluar dari ubus bisa datang sebagai bilangan **tak bertanda** (kode rpcd memakai `blobmsg_add_u32(&buf, "signal", a->signal)` atas `int8_t`) [S7], sementara contoh keluaran pengguna di forum OpenWrt menunjukkan nilai negatif [S12b]. **Selisih ini harus diuji di perangkat** dan dinormalkan di poller (`if v >= 2147483648 then v -= 4294967296`). Lihat daftar kontradiksi di bawah.
- Untuk `extend`, output menjadi: baris 1 → `nsExtendOutput1Line`, seluruh output → `nsExtendOutputFull`, dan tiap baris → `nsExtendOutLine` — jalur parsing per-baris inilah yang paling nyaman untuk poller [S15b].
- Batas `pass`/`pass_persist` tidak mendukung `counter64` [S15] ⇒ **jangan** kirim byte counter kumulatif lewat jalur ini; ambil traffic dari IF-MIB (`ifHCInOctets`/`ifHCOutOctets` yang asli 64-bit, [S16]).

**Biaya/beban pada router kelas bawah [PERLU UJI LOKAL — tidak ada sumber primer].** Yang bisa dikatakan dari sumber primer:
- Paket `snmpd` menarik `libnetsnmp` yang bergantung pada **`libnl-tiny`, `libpci`, `libpcre2`** [S3] — `libpci` dan `libpcre2` adalah tambahan nyata pada flash router kecil (hitung dengan `opkg info snmpd libnetsnmp libpci libpcre2` dan `df -h /`). Paket `snmp-mibs` (berkas MIB teks) **tidak perlu** untuk poller numerik.
- Daemon dijalankan procd dengan `respawn` dan menulis ulang konfigurasi ke tmpfs (`/var/run/snmpd.conf`) setiap start/reload [S4].
- `extend` **mem-spawn** program tiap query (kecuali masih dalam `cacheTime`), sementara `pass_persist` mempertahankan satu proses [S15].
- `iwinfo assoclist` memakai buffer 24 KB dan dilaporkan bisa membuat rpcd crash pada ±150+ client [S10][S12b] — sumber beban/instabilitas nyata bila dipanggil tiap 5 menit pada AP ramai.

**Cara mengukur sendiri (wajib dilakukan sebelum menetapkan anggaran CPU):**
```
time ubus call iwinfo assoclist '{"device":"phy0-ap0"}' >/dev/null
time /usr/libexec/cuanku-wifi-pass.sh < /dev/null     # atau ukur skrip extend
top -b -n1 | head -20 ; logread -e snmpd | tail
# beban poller: pada server, ukur wall-time & CPU satu siklus penuh
time snmpwalk -v2c -c <c> <ip> 1.3.6.1.2.1.31.1.1.1 ; time snmpwalk -v2c -c <c> <ip> 1.3.6.1.4.1.2021
```

### 6. Keterjangkauan AP di balik NAT/CGNAT (inti ADR-0002)

ADR-0002 menyebut *"TR-069/ACS (push) atau reverse tunnel adalah alternatif yang mungkin diperlukan"*. Hasil verifikasi:

**(a) Reverse SSH tunnel — TIDAK CUKUP untuk SNMP apa adanya. [TERVERIFIKASI]** SSH Connection Protocol (RFC 4254) hanya mendefinisikan port forwarding **TCP**: tabel isinya "7. TCP/IP Port Forwarding", "7.1. Requesting Port Forwarding" (`tcpip-forward`), "7.2. TCP/IP Forwarding Channels" dengan channel type `forwarded-tcpip` (remote forwarding) dan `direct-tcpip` (local forwarding) [S17]. Tidak ada channel UDP. SNMP berjalan di **UDP/161** [S4: `option agentaddress 'UDP:161'`]. Jadi `ssh -R 161:127.0.0.1:161` **tidak akan** meneruskan SNMP — perlu relay UDP↔TCP (mis. `socat UDP4-RECVFROM:161,fork TCP4:127.0.0.1:1161` di AP dan cerminnya di server), yang berarti: komponen tambahan di setiap AP, dua titik gagal baru, dan kebutuhan skrip/init tambahan yang harus dipelihara di setiap firmware. (Alternatifnya: tunnel dipertahankan untuk tujuan lain, bukan untuk SNMP.)
Ketersediaan paket resmi untuk sisi tunnel: **`autossh`** (`net/autossh`, punya UCI `autossh.config` dengan opsi `ssh`, `gatetime`, `monitorport`, `poll`) dan **`sshtunnel`** (`net/sshtunnel`, dependensi OpenSSH client) — keduanya **terverifikasi ada** di pohon feed `packages` resmi [S1]. **[INDIKATIF]** halaman wiki OpenWrt juga menyebut keduanya untuk kebutuhan "AP di balik NAT" [S22].

**(b) TR-069/CWMP via paket OpenWrt resmi — TIDAK ADA. [TERVERIFIKASI]** Telusuri penuh `git/trees/master?recursive=1` dari `openwrt/packages` @ commit `4a69a0b` (2026-09-25): tidak ada direktori `net/icwmp`, tidak ada kecocokan untuk `cwmp`, `tr069`, maupun `icwmp` di seluruh pohon [S1]. Yang ada hanya sebagai referensi di wiki OpenWrt [S18]:
- **icwmp** (BSD-3-Clause, dev.iopsys.eu) — hanya tersedia via **feed Iopsys**, bukan feed resmi OpenWrt.
- **EasyCwmp** (GPLv2) — wiki mencatat *"Last updated to support OpenWrt 19.07. There haven't been any updates since December 2019. Last checked June 2024."*
- **FreeCWMP** (GPLv2) — proyek lama (materi 2012).
Konsekuensi arsitektur: mengadopsi TR-069 berarti menambahkan feed pihak ketiga yang tidak dikelola OpenWrt + menjalankan ACS sendiri (mis. GenieACS) — biaya operasional baru yang justru bertentangan dengan semangat "anti lock-in" ADR-0002, karena TR-069 membawa model data TR-098/TR-181 sendiri (lapisan pemetaan data tambahan).

**(c) TR-369/USP — TIDAK ADA di feed resmi. [TERVERIFIKASI]** Di pohon paket yang sama tidak ditemukan `net/usp`, `obuspa`, `uspd`, maupun paket `usp` [S1]. Implementasi yang ada: **OB-USPA** (`BroadbandForum/obuspa`, daemon USP Agent) [S21], dan feed pihak ketiga seperti **feed_usp SoftAtHome** (berisi `uspagent`, `usp-endpoint`, `tr181-localagent`, `libimtp`, `libusp`) serta varian prpl [S18][S21]. Wiki OpenWrt hanya mendaftarkannya sebagai "Free implementations of TR-369" (OktopUSP, BroadbandForum/usp) tanpa paket resmi [S18]. Jadi TR-369 = pekerjaan build/integrasi tersendiri (dan, untuk sisi controller, produk/OSS lain lagi) — bukan opsi MVP.

**(d) Agent push sederhana — TIDAK ADA paket khusus, tapi paling mudah dibangun dan tidak bergantung paket baru. [INFERENSI peneliti]** AP sudah **wajib** menginisiasi koneksi keluar dalam arsitektur yang sudah diputuskan: `0001-mvp-scope.md` menetapkan transport RADIUS via "tunnel/radsecproxy" (hostapd hanya UDP) [S20], dan ADR-0001 sudah hidup dengan pembedaan "AccessPoint terjangkau langsung" vs "di balik NAT/CGNAT" untuk Disconnect-Request [S20]. Artinya pola "AP menghubungi server" **sudah** ada di sistem; menambahkan job push (mis. skrip + `curl` HTTPS POST ke endpoint adapter, token unik per `AccessPoint`, interval sama 5 menit) tidak menambah komponen jaringan baru. Kelemahannya: harus dibangun & didistribusikan sendiri (provisioning token, rollback, anti-replay, penanganan antrean saat offline), dan tidak ada standar yang bisa dibeli/diwarisi.

**Verdict keterjangkauan (tegas):** untuk `AccessPoint` di balik NAT/CGNAT, **SNMP pull tidak layak di MVP** — bukan karena protokolnya, tetapi karena (i) tidak ada jalur inbound, dan (ii) satu-satunya "solusi murah" yang biasanya disebut (reverse SSH) ternyata harus ditambah relay UDP↔TCP. Karena T3 di `0001-mvp-scope.md` **sudah** membatasi janji ke "AP terjangkau SNMP", tidak ada pelanggaran kriteria sukses bila AP di balik NAT berstatus `unreachable` pada MVP; tetapi konsekuensi operasionalnya harus eksplisit di ADR-0002/issue #19: **mayoritas `AccessPoint` Customer (perangkat rumah) kemungkinan besar akan permanen `unreachable`, sehingga T3 hanya terpenuhi untuk sub-populasi kecil.** Bila pilot membuktikan itu, agent push (d) naik ke MVP dan SNMP pull turun menjadi adapter opsional untuk AP yang memang punya IP routable. TR-069/TR-369 tetap post-MVP.

### 7. Beban polling dan jumlah OID yang wajar

**[TIDAK TERVERIFIKASI / tidak ditemukan sumber primer]** Tidak ditemukan dokumentasi resmi OpenWrt, net-snmp, maupun IETF yang menetapkan ambang jumlah OID atau besaran beban `snmpd` yang wajar untuk router kelas bawah. Klaim forum/blog tentang topik ini **tidak dipakai** sebagai bukti di sini ([INDIKATIF] dan sengaja tidak dikutip).

Yang bisa dipertanggungjawabkan hanyalah perkiraan berbasis fakta protokol:
- Satu siklus polling 5 menit per AP dengan kumpulan metrik di §10 (≈ 4 skalar + 1–2 walk `ifXTable` + ≈ 9 skalar UCD + 2 skalar HR + beberapa baris `pass_persist`) = **puluhan varbind**, yang dengan SNMPv2c `GetBulk` dapat diambil dalam **1–3 round-trip** ⇒ trafik ratusan byte–beberapa KB per AP per 5 menit. **[INFERENSI peneliti]**
- Karena intervalnya 5 menit dan polling bersifat sekuensial per AP, risiko utamanya bukan bandwidth melainkan **jumlah AP × latensi timeout**: dengan timeout 1 detik dan 1 retry, AP yang mati menyita ~2 detik poller; 500 AP dengan 20% selalu timeout ≈ 200 detik per siklus — masih di bawah 300 detik, tapi marjinal. **[INFERENSI peneliti]** Implikasi desain: **kecualikan AP berstatus `unreachable` dari siklus polling normal** (lihat §8), dan ukur waktu siklus penuh di server (bukan hanya di AP).

**Cara mengukur sendiri (wajib, karena tidak ada acuan primer):**
```
# di sisi AccessPoint
top -b -n1 ; free ; logread -e snmpd | tail -20
time ubus call iwinfo assoclist '{"device":"phy0-ap0"}' >/dev/null
# di sisi poller (server)
/usr/bin/time -v snmpbulkwalk -v2c -c <c> -Cn0 -Cr10 <ip-ap> 1.3.6.1.2.1.31.1.1
# dan: korelasi CPU AP vs jumlah client (uji pada 0, 5, 50 client)
```

### 8. Semantik status: offline vs unreachable (dampak ke #23)

**[TERVERIFIKASI dari kode klien]** Aplikasi net-snmp hanya membedakan dua kelas kegagalan. Di `apps/snmpstatus.c` [S12b]:

```c
} else if (status == STAT_TIMEOUT) {
    fprintf(stderr, "Timeout: No Response from %s\n", session.peername);
    ...
} else { /* status == STAT_ERROR */
    snmp_sess_perror(...);
}
```

Artinya:
- **Host mati**, **jaringan tidak bisa ditembus** (NAT/firewall drop), **community salah/diblokir VACM source**, dan **paket hilang** semuanya muncul ke poller sebagai `STAT_TIMEOUT` → *"Timeout: No Response from …"* + exit code non-nol. **Tidak ada bedanya.** [TERVERIFIKASI untuk sisi klien; mekanisme agent membuang/menjawab pesan ber-community salah [PERLU UJI LOKAL] — verifikasi: `snmpget -v2c -c salah <ip> .1.3.6.1.2.1.1.3.0 ; echo $?` vs community benar.]
- **`STAT_ERROR`** muncul justru ketika **ada jawaban**: mis. objek tidak ada → nilai `noSuchObject`/`noSuchInstance` (RFC 3416 §4.2.1: *"if the variable binding's name does not have an OBJECT IDENTIFIER prefix which exactly matches… its value field is set to noSuchObject"*, lalu *"Otherwise… noSuchInstance"*) [S16b], atau error-status/authorizationError lain dari agent.
- Jadi "**agent hidup tapi OID kosong**" **bisa** dibedakan (poller mendapat respons dengan `noSuchInstance`) — dan itu penting: `IEEE802dot11-MIB` yang mati akan muncul sebagai respons `noSuchObject`, **bukan** timeout, sehingga tidak salah diklasifikasikan sebagai AP mati.

**Implikasi langsung untuk #23 ("Alert saat AP offline; tidak alert saat AP unreachable"):** membedakan keduanya **tidak mungkin** dari satu hasil SNMP. Karena itu klasifikasinya harus diputuskan dari **kondisi yang diketahui di luar protokol**, bukan ditebak dari kegagalan:

1. **Basis data topologi (otoritatif):** simpan per `AccessPoint` fakta "punya IP routable/port-forward?" (mis. flag `reachableByPolling`, diisi saat onboarding dan diverifikasi saat aktivasi). AP tanpa jalur inbound **tidak pernah** masuk jalur SNMP → statusnya `unreachable` secara definitif, bukan hasil polling.
2. **Pembedaan untuk AP yang seharusnya terjangkau:** gunakan bukti berlapis sebelum menyatakan `offline`: (a) ICMP ping + (b) percobaan koneksi TCP ke port layanan AP yang pasti terbuka (mis. port RADIUS/SSH/tunnel) — bila ICMP+TCP gagal ⇒ `offline`; bila ICMP/TCP **berhasil** tetapi SNMP timeout ⇒ `unreachable` (atau community/VACM salah → masuk kategori "konfigurasi bermasalah", bukan offline). Ini juga melindungi #23 dari alert palsu ketika hanya `snmpd` yang mati sementara AP hidup — karena kasus itu menghasilkan **timeout** yang sama. **[INFERENSI peneliti]**
3. **Deredam (flapping):** butuh N siklus berturut-turut gagal (mis. 3× 5 menit = 15 menit) sebelum menyatakan `offline`; ini juga menghindari alert dari reboot/topologi berubah. Selaras dengan sifat polling 5 menit. **[INFERENSI peneliti]**
4. **Sinyal pendukung yang tersedia dan murah:** AP yang hidup mengirim **accounting RADIUS** (start/interim/stop) ke FreeRADIUS. `Session`/accounting adalah indikator "AP masih hidup" yang **independen dari SNMP** dan bekerja untuk AP di balik NAT — sangat berguna untuk menekan alert palsu dan untuk memenuhi #23 pada AP yang tidak dipoll. **[INFERENSI peneliti, berbasis `0001-mvp-scope.md` yang memang sudah mencatat interval interim accounting 5 menit]**

Konsekuensinya untuk model data adapter (ADR-0002): status `AccessPoint` sebaiknya bukan hasil tunggal poller, melainkan hasil kebijakan yang menggabungkan (a) metadata reachability, (b) hasil polling bila relevan, (c) liveness berlapis (ICMP/TCP), (d) sinyal accounting RADIUS. Detail protokol (timeout/exit code SNMP) tetap tidak boleh bocor ke domain layer — sesuai Consequences ADR-0002.

### 9. Trade-off push vs pull untuk MVP

| Dimensi | Pull (SNMP v2c) | Push (agent/HTTPS) | TR-069 (CWMP) | TR-369 (USP) |
|---|---|---|---|---|
| Berfungsi di balik NAT/CGNAT | **Tidak** | **Ya** (AP menginisiasi keluar) | Ya | Ya |
| Paket resmi OpenWrt | **Ya** (`snmpd`) [S3] | tidak ada (rakitan sendiri) | **tidak ada** [S1][S18] | **tidak ada** [S1][S18][S21] |
| Kematangan ekosistem | sangat matang (MIB standar) | sepenuhnya tanggungan kita | matang di industri CPE, tapi lewat feed pihak ketiga + ACS | belum matang untuk OpenWrt umum |
| Biaya implementasi MVP | rendah (poller + UCI) | sedang (agent, token, endpoint, retry) | tinggi (feed Iopsys + ACS + TR-181) | tinggi (OB-USPA + controller) |
| Keamanan | v2c cleartext; community per-AP + batas IP (sudah di `0001-mvp-scope.md`) | TLS + token per-AP (lebih baik) | TLS + kredensial ACS | TLS + MTP/WebSocket |
| Cakupan metrik WiFi | butuh `pass_persist`/`extend` | sama, tapi bisa kirim JSON kaya (per-client) tanpa batas tipe MIB | sama, tapi via TR-181 | sama, tapi via TR-181 |
| Beban server | 1 sesi UDP cepat per AP per 5 menit | 1 request HTTP per AP per 5 menit (header lebih berat) | sesi CWMP lengkap | sesi USP/WebSocket |
| Kesesuaian dengan ADR-0002 | implementasi pertama (sesuai) | adapter kedua (sesuai) | adapter lain (sesuai secara abstraksi) | adapter lain (sesuai secara abstraksi) |

**Rekomendasi (tegas, untuk MVP):**

1. **Pertahankan SNMP v2c pull sebagai adapter pertama** sesuai ADR-0002 dan #19 — tetapi **ruang lingkupnya dibatasi** pada `AccessPoint` dengan jalur IP inbound, dan poller **wajib** memakai metadata reachability untuk menandai sisanya sebagai `unreachable` **tanpa** menghabiskan waktu timeout (§8). Ini konsisten dengan T3 yang sudah menyebut "untuk AP terjangkau SNMP".
2. **Turunkan cakupan metrik** (§10 + daftar risiko): `signal/noise per radio` menjadi **`signal`/`signal_avg` per client + `noise` floor kanal per interface + jumlah client per interface**; `CPU/memori` dari `HOST-RESOURCES-MIB` dipindahkan ke **UCD-SNMP-MIB** sebagai sumber utama (HR sebagai cadangan), karena `hrProcessorLoad` rapuh (§3). Ini **rekomendasi**, bukan perubahan ADR — ADR-0002 tetap accepted.
3. **Siapkan agent push sebagai adapter kedua dan putuskan promosinya dengan data pilot**, bukan dengan argumen: pada 3–5 `AccessPoint` percontohan, ukur berapa persen yang punya jalur IP inbound (termasuk opsi port-forward di sisi Customer). Jika < 50%, promosikan push ke MVP (dan SNMP pull jadi opsional); jika ≥ 50%, SNMP pull tetap MVP dan push ditunda ke post-MVP. Alasan yang bisa dipertahankan: ADR-0002 sudah menjanjikan netralitas adapter, jadi promosi push **tidak** mengubah model data; yang berubah hanya jumlah adapter yang dibangun.
4. **Jangan tempuh reverse SSH tunnel sebagai solusi standar** untuk SNMP (§6a). Kalau tunnel tetap dibutuhkan demi tujuan lain, itu keputusan terpisah dan harus mempertimbangkan relay UDP↔TCP.

### 10. Daftar OID konkret untuk #20

Status: **[TERSEDIA]** = tersedia di build `snmpd` OpenWrt, cukup di-GET/GETBULK · **[BUTUH SCRIPT]** = harus diekspos via `pass_persist`/`extend` · **[TIDAK TERSEDIA]** = tidak ada di perangkat/MIB.

| Metrik | OID (nama numerik) | MIB / sumber | Status | Catatan |
|---|---|---|---|---|
| Uptime agent (TimeTicks 1/100 s) | `.1.3.6.1.2.1.1.3.0` `sysUpTime.0` | SNMPv2-MIB (RFC 3418) | **TERSEDIA** | pembanding lintas perangkat |
| Uptime host | `.1.3.6.1.2.1.25.1.1.0` `hrSystemUptime.0` | HOST-RESOURCES-MIB (RFC 2790) | **TERSEDIA** | berbeda dari `sysUpTime` [S13] |
| Identitas perangkat | `.1.3.6.1.2.1.1.1.0` `sysDescr.0` | SNMPv2-MIB | **TERSEDIA** | untuk deteksi model/firmware |
| Nama perangkat | `.1.3.6.1.2.1.1.5.0` `sysName.0` | SNMPv2-MIB | **TERSEDIA** | diisi via UCI `config system` |
| Jumlah proses | `.1.3.6.1.2.1.25.1.6.0` `hrSystemProcesses.0` | HOST-RESOURCES-MIB | **TERSEDIA** (uji lokal) | Gauge32 [S13] |
| CPU% (metode utama: delta counter) | `.1.3.6.1.4.1.2021.11.53.0` `ssCpuRawIdle.0`, `.50.0` `ssCpuRawUser.0`, `.51.0` `ssCpuRawNice.0`, `.52.0` `ssCpuRawSystem.0`, `.54.0` `ssCpuRawWait.0` | UCD-SNMP-MIB (`systemStats 50..54` = `ucdavis.11`) | **TERSEDIA** | hitung persentase dari selisih dua sampel 5 menit [S12] |
| CPU% (objek lama, 1 menit) | `.1.3.6.1.4.1.2021.11.9.0` `ssCpuUser.0`, `.10.0` `ssCpuSystem.0`, `.11.0` `ssCpuIdle.0` | UCD-SNMP-MIB (deprecated) | **TERSEDIA** | *deprecated*; jangan dijadikan basis [S12] |
| CPU load rata-rata 1 menit | `.1.3.6.1.4.1.2021.10.1.3.1` `laLoad.1` | UCD-SNMP-MIB (`ucd-snmp/loadave`) | **TERSEDIA** | murah, tapi bukan CPU% benar |
| CPU% per prosesor (HR) | `.1.3.6.1.2.1.25.3.3.1.2.1` `hrProcessorLoad.1` | HOST-RESOURCES-MIB / `hr_proc.c` | **TERSEDIA (rapuh)** | bisa `noSuchInstance`; resolusi 1 menit [S11][S13] |
| Memori total/terpakai | `.1.3.6.1.4.1.2021.4.5.0` `memTotalReal.0`, `.4.6.0` `memAvailReal.0`, `.4.11.0` `memTotalFree.0`, `.4.14.0` `memBuffer.0`, `.4.15.0` `memCached.0` | UCD-SNMP-MIB (`memory` = `ucdavis.4`) | **TERSEDIA** | Integer32 kB; risiko overflow > 2 GiB [S12] |
| Memori (64-bit, bila ada) | `.1.3.6.1.4.1.2021.4.20.0` `memTotalRealX.0`, `.4.21.0` `memAvailRealX.0`, `.4.22.0` `memTotalFreeX.0` | UCD-SNMP-MIB | **TERSEDIA (uji lokal)** | CounterBasedGauge64 [S12] |
| RAM fisik (HR) | `.1.3.6.1.2.1.25.2.2.0` `hrMemorySize.0` | HOST-RESOURCES-MIB | **TERSEDIA** (uji lokal) | KBytes [S13] |
| Storage/overlay | `.1.3.6.1.2.1.25.2.3.1.4/.5/.6` (`hrStorageAllocationUnits/Size/Used`) | HOST-RESOURCES-MIB | **TERSEDIA (uji lokal)** | apakah tabel terisi di OpenWrt **belum terverifikasi** [S13] |
| Daftar interface | `.1.3.6.1.2.1.2.2.1.1` `ifIndex`, `.2` `ifDescr`, `.8` `ifOperStatus` | IF-MIB (RFC 2863) | **TERSEDIA** | [S16] |
| Nama interface | `.1.3.6.1.2.1.31.1.1.1.1` `ifName` | IF-MIB ifXTable | **TERSEDIA** | pakai ini untuk memetakan `br-lan`/`phy0-ap0` [S16] |
| Traffic RX/TX per interface (64-bit) | `.1.3.6.1.2.1.31.1.1.1.6` `ifHCInOctets`, `.1.3.6.1.2.1.31.1.1.1.10` `ifHCOutOctets` | IF-MIB ifXTable | **TERSEDIA** | **pakai versi HC**, bukan `.2.2.1.10/.16` (Counter32, overflow pada WAN cepat) [S16] |
| Traffic RX/TX (32-bit, fallback) | `.1.3.6.1.2.1.2.2.1.10` `ifInOctets`, `.1.3.6.1.2.1.2.2.1.16` `ifOutOctets` | IF-MIB ifTable | **TERSEDIA** | hanya bila HC tidak ada [S16] |
| Kecepatan link | `.1.3.6.1.2.1.31.1.1.1.15` `ifHighSpeed` | IF-MIB ifXTable | **TERSEDIA** | Mb/s [S16] |
| Jumlah client per interface | — | `iwinfo assoclist` (jumlah `results[]`) / `iw dev station dump` | **BUTUH SCRIPT** | tidak ada di MIB; risiko buffer 24 KB pada AP ramai [S10][S12b] |
| Signal terlemah/rata-rata/terbaik per interface (agregat dari client) | — | `assoclist[].signal`, `assoclist[].signal_avg` / `station dump signal` | **BUTUH SCRIPT** | agregasi adalah keputusan poller, bukan sifat perangkat |
| Signal & signal_avg per client (detail) | — | idem, satu baris per MAC | **BUTUH SCRIPT** | volume data naik linear dengan jumlah client |
| Bitrate rx/tx per client | — | `assoclist[].rx.rate` / `tx.rate` (rateinfo `rate`+`mhz`) / `station dump tx bitrate` | **BUTUH SCRIPT** | opsional untuk MVP |
| Noise floor kanal per radio/interface | — | `iwinfo info.noise`, `iwinfo survey[].noise`, `iw survey` | **BUTUH SCRIPT** | nilai sama untuk semua client di interface itu |
| Noise per client | — | — | **TIDAK TERSEDIA** | tidak ada `NL80211_STA_INFO_NOISE` [S10b] |
| `dot11*` (IEEE802dot11-MIB) | `.1.3.6.1.2.1.34` | IEEE802dot11-MIB (patch OpenWrt, berbasis WEXT) | **TIDAK TERSEDIA (efektif)** | WEXT dimatikan di kernel OpenWrt [S8][S14] |
| Kanal tempat kerja (bantu) | `ifOperStatus` + `ifName` pada interface AP | IF-MIB | **TERSEDIA** | tidak memberi kanal; kanal dari `iwinfo info.channel` (**BUTUH SCRIPT**) |
| Status polling (bukan MIB) | exit code + stderr `snmpget`/`snmpbulkwalk` | net-snmp apps | **TERSEDIA** | `Timeout: No Response from <host>` vs respons ber-`noSuchObject` [S12b][S16b] |

**Jalur ekspos skrip (nama OID NET-SNMP-EXTEND-MIB)** — indeks `nsExtendToken` adalah string, jadi sufiks OID = panjang + kode ASCII (mis. token `wifi` → `5.119.105.102.105`):

| Objek | OID | Isi |
|---|---|---|
| `nsExtendNumEntries.0` | `.1.3.6.1.4.1.8072.1.3.2.1.0` | jumlah entri `extend` |
| `nsExtendOutput1Line.<token>` | `.1.3.6.1.4.1.8072.1.3.2.3.1.1.<token>` | **baris pertama** output |
| `nsExtendOutputFull.<token>` | `.1.3.6.1.4.1.8072.1.3.2.3.1.2.<token>` | seluruh output sebagai satu string |
| `nsExtendOutNumLines.<token>` | `.1.3.6.1.4.1.8072.1.3.2.3.1.3.<token>` | jumlah baris |
| `nsExtendResult.<token>` | `.1.3.6.1.4.1.8072.1.3.2.3.1.4.<token>` | exit status perintah |
| `nsExtendOutLine.<token>.<n>` | `.1.3.6.1.4.1.8072.1.3.2.4.1.2.<token>.<n>` | **baris ke-n** — paling nyaman untuk parsing per-metrik |

Struktur itu berasal dari `nsExtendObjects ::= { nsExtensions 2 }`, `nsExtendOutput1Table ::= { nsExtendObjects 3 }`, `nsExtendOutput2Table ::= { nsExtendObjects 4 }` dengan `nsExtensions ::= { netSnmpObjects 3 }` pada pohon `.1.3.6.1.4.1.8072` [S15b][S15c]. Verifikasi lokal untuk setiap OID sebelum masuk kode #20:

```
snmptranslate -On -m /usr/share/snmp/mibs/NET-SNMP-EXTEND-MIB.txt nsExtendOutputFull
snmpwalk -v2c -c <community> <ip-ap> 1.3.6.1.4.1.8072.1.3
```

---

## Risiko & ketidakpastian

1. **Janji metrik T3/ADR-0002 tidak bisa dipenuhi apa adanya (risiko utama).** "signal/noise per radio" bukan konsep yang ada di perangkat; "noise per client" tidak ada sama sekali; "jumlah client" tidak ada di MIB dan hanya bisa dihitung skrip (dengan risiko rpcd crash pada AP ramai). **Rekomendasi: turunkan/ubah istilah metrik** di ADR-0002 (atau di #20) menjadi *signal·signal_avg per client + noise floor kanal per interface + jumlah client per interface* — tanpa mengubah keputusan arsitektur ADR-0002. Bila istilah dibiarkan, "metrik inti tercatat konsisten dengan interval polling" (kriteria #20) tidak akan pernah bisa diverifikasi.
2. **`IEEE802dot11-MIB` adalah jebakan.** Modulnya ada dan ter-kompilasi, sehingga mudah disimpulkan "wireless sudah dapat dari SNMP". Implementasinya berbasis WEXT yang dimatikan di kernel OpenWrt. Risiko: pekerjaan integrasi MIB ini sia-sia. **Mitigasi:** larang penggunaannya di #20; uji cepat `cat /proc/net/wireless` di perangkat.
3. **HOST-RESOURCES-MIB rapuh.** `hrProcessorLoad` bisa `noSuchInstance`; `hrStorageTable` belum terbukti terisi di OpenWrt. Risiko: grafik CPU/memori kosong tanpa error. **Mitigasi:** UCD-SNMP-MIB sebagai sumber utama; jalankan uji keberadaan OID (bukan hanya uji koneksi) pada setiap model AP sebelum rilis.
4. **AP di balik NAT = mayoritas `unreachable` permanen.** Bila asumsi ini benar (sangat mungkin untuk perangkat rumah), maka T3 hanya berlaku untuk minoritas kecil dan #23 kehilangan sinyalnya (tidak ada AP "offline" yang pernah terdeteksi). **Mitigasi:** metadata reachability + liveness dari accounting RADIUS; pilot untuk mengukur proporsi AP dengan IP inbound; keputusan push vs pull berbasis data pilot.
5. **Tunnel SSH bukan solusi langsung.** Kesalahan desain yang mudah terjadi: mengasumsikan `ssh -R` bisa mem-forward SNMP. RFC 4254 hanya mendefinisikan forwarding TCP. Risiko: jadwal dan estimasi implementasi melebar saat relay UDP↔TCP ketahuan belakangan.
6. **Batasan `pass`/`pass_persist`.** Tidak ada `counter64` dan `noSuchObject`. Risiko: desain metrik byte-per-client yang mengasumsikan 64-bit akan mentok. **Mitigasi:** semua byte counter kumulatif dari IF-MIB (native 64-bit); metrik skrip pakai `integer`/`gauge`/`counter` 32-bit atau string.
7. **Ambiguitas tanda pada `signal`/`noise` dari ubus.** Kode rpcd memakai `blobmsg_add_u32` atas nilai `int8_t`, sedangkan contoh keluaran yang beredar menunjukkan angka negatif. **Kontradiksi eksplisit** (lihat bawah) ⇒ poller **harus** menormalkan dan menolak nilai di luar rentang wajar (−100…0 dBm) sebagai anomali, bukan menyimpannya mentah.
8. **Tidak ada acuan primer untuk anggaran beban.** Semua angka beban di dokumen ini adalah perkiraan; tanpa pengukuran lapangan, penetapan interval/thread poller bisa salah. **Mitigasi:** jadikan pengukuran (§7) sebagai *acceptance* sebelum menutup #19.
9. **Batas sesi riset ini:** verifikasi runtime hanya bisa dilakukan pada perangkat nyata. Semua butir **[PERLU UJI LOKAL]** (isi `hrStorageTable`, keberadaan `ssCpuRaw*`/`mem*` di firmware spesifik, nilai asli `noise`/`signal` dari ubus, `/proc/net/wireless`, keberadaan `hostapd_cli`) belum terverifikasi dan tidak boleh dianggap benar sebelum diuji.

## Kontradiksi / perbedaan bukti

- **Tanda nilai `signal`/`noise`.** Sumber primer (kode `rpcd`/`iwinfo.c`): `blobmsg_add_u32(&buf, "signal", a->signal)` — `a->signal` bertipe `int8_t`, jadi nilainya dikonversi ke `uint32_t` (untuk −45 menjadi 4294967251) [S7]. Contoh keluaran yang diposting pengguna di forum OpenWrt menunjukkan nilai negatif langsung (`"signal": -32, "noise": -89`) [S12b]. **Tidak diselesaikan** — kemungkinan perbedaan versi rpcd/`blobmsg_add_u32`, atau perbedaan cara pengambilan. **Tindakan:** uji di perangkat (`ubus call iwinfo assoclist '{"device":"phy0-ap0"}'`) dan normalkan di poller.
- **`mini-snmpd` masih disebut dokumentasi resmi OpenWrt.** Halaman wiki "SNMPD" menyebut *"There are two options: mini-snmpd or snmpd"* dan mendokumentasikan `/etc/config/mini_snmpd` [S5b]. Namun tidak ada `net/mini-snmpd` di pohon feed `packages` `master` (2026-09-25) maupun di branch `openwrt-24.10` [S1][S23]. **Kesimpulan: dokumentasi wiki itu usang; `mini-snmpd` bukan opsi.** (Selain itu mini-snmpd hanya 32-bit counter [S5b].)
- **Versi paket pada indeks paket OpenWrt vs feed.** Halaman `openwrt.org/packages/pkgdata/snmpd` menampilkan `5.9.1-7` [S5], sedangkan Makefile feed menunjukkan `5.9.4-6` (24.10/25.12) dan `5.9.5.2-2` (master) [S2][S3]. **Versi di Makefile feed adalah yang benar**; indeks wiki tidak diperbarui. Verifikasi otoritatif di perangkat: `opkg info snmpd`.

## Missing evidence

- **Runtime di perangkat nyata**: apakah `snmpwalk 1.3.6.1.2.1.25` benar-benar mengembalikan `hrProcessorLoad`, `hrStorageTable`, `hrSystemUptime` pada build `snmpd` OpenWrt 24.10/25.12 untuk hardware target (uji ada/tidaknya nilai, bukan sekadar konektivitas).
- **Apakah `ucd-snmp/vmstat` menghasilkan `ssCpuRaw*`** pada arsitektur target (mis. mips/mipsel, arm) — MIB-nya sendiri memperingatkan dukungan per-sistem [S12].
- **Ukuran terpasang (flash) paket `snmpd` + `libnetsnmp` + `libpci` + `libpcre2`** pada rilis 25.12.x. (Indeks `Packages` per-arsitektur tidak berhasil diambil dalam sesi ini; ambil lokal dengan `opkg info`/`df`.)
- **Format keluaran CLI `iwinfo <dev> info`, `iwinfo <dev> assoclist`, dan `iw dev <dev> survey dump`** (yang akan dipakai bila `rpcd-mod-iwinfo` tidak dipasang) — belum diverifikasi kata-per-kata.
- **Ketersediaan `hostapd_cli` (dan subperintah `all_sta`) pada image OpenWrt target** — belum diverifikasi.
- **Ambang jumlah client** di mana `ubus call iwinfo assoclist` benar-benar gagal pada versi iwinfo sekarang (isu upstream menyebut ±150 untuk versi 2019 dengan struct yang lebih kecil) [S12b].
- **Perilaku agent saat community salah** di `snmpd` OpenWrt (dibuang diam-diam vs dicatat) — belum diverifikasi dari kode agent; efek yang dipastikan hanyalah dari sisi klien (timeout).
- **Isi issue GitHub #19/#20/#23** hanya dibaca sebagai judul + cuplikan body lewat API publik (bukan dibaca penuh); bila ada acceptance criteria tambahan di luar cuplikan, dokumen ini belum menilainya.

---

## Sumber

**Dipertahankan — primer (kode/config upstream, dokumen IETF, man page upstream, indeks rilis resmi):**

1. [S1] `openwrt/packages` — pohon Git lengkap branch `master`, commit `4a69a0b76759dcd6e2164c81d7260b5280d833c1` (2026-09-25): https://api.github.com/repos/openwrt/packages/git/trees/master?recursive=1 (tanggal commit dari https://api.github.com/repos/openwrt/packages/commits/master). Dipakai untuk membuktikan ada/tidaknya `net/net-snmp`, `net/autossh`, `net/sshtunnel`, `net/icwmp`, `net/usp`, `net/mini-snmpd`, dan `net/net-snmp/patches/750-ieee802dot11.patch`.
2. [S2] `net/net-snmp/Makefile`, branch `openwrt-24.10` dan `openwrt-25.12`: https://raw.githubusercontent.com/openwrt/packages/openwrt-24.10/net/net-snmp/Makefile · https://raw.githubusercontent.com/openwrt/packages/openwrt-25.12/net/net-snmp/Makefile — `PKG_VERSION:=5.9.4`, `PKG_RELEASE:=6`.
3. [S3] `net/net-snmp/Makefile`, branch `master`: https://raw.githubusercontent.com/openwrt/packages/master/net/net-snmp/Makefile — `PKG_VERSION:=5.9.5.2`, `PKG_RELEASE:=2`, daftar `SNMP_MIB_MODULES_INCLUDED/EXCLUDED`, `CONFIGURE_ARGS`, dependensi `+libnl-tiny +libpci +libpcre2`, `SNMP_TRANSPORTS_*`, varian `nossl`/`ssl`.
4. [S4] `net/net-snmp/files/snmpd.conf` (konfigurasi UCI pabrik) dan `net/net-snmp/files/snmpd.init` (generator `/var/run/snmpd.conf`, section & field UCI, pembentukan baris `com2sec`/`group`/`view`/`access`/`pass`/`extend`, firewall rule untuk `list network`): https://raw.githubusercontent.com/openwrt/packages/master/net/net-snmp/files/snmpd.conf · https://raw.githubusercontent.com/openwrt/packages/master/net/net-snmp/files/snmpd.init
5. [S5] Halaman indeks paket OpenWrt untuk `snmpd` (usang; hanya untuk pembanding versi): https://openwrt.org/packages/pkgdata/snmpd
6. [S5b] Dokumentasi OpenWrt: "SNMPD" (`opkg install snmpd`, `/etc/config/snmpd`, penyebutan `mini-snmpd` dan batas 32-bit counter-nya): https://openwrt.org/docs/guide-user/services/snmp/server
7. [S6] Dokumentasi OpenWrt: "docs:guide-user:services:snmp:snmpd" (UCI `config 'exec'` → baris `exec` di `snmpd.conf`; menegaskan UCI sebagai sumber konfigurasi): https://openwrt.org/docs/guide-user/services/snmp/snmpd
8. [S7] `openwrt/rpcd` — `iwinfo.c` (plugin `rpcd-mod-iwinfo`): method ubus (`devices`, `info`, `scan`, `assoclist`, `freqlist`, `txpowerlist`, `countrylist`, `survey`, `phyname`) dan daftar field JSON `info`/`assoclist`/`survey`: https://raw.githubusercontent.com/openwrt/rpcd/master/iwinfo.c
9. [S8] OpenWrt kernel config generik: https://raw.githubusercontent.com/openwrt/openwrt/main/target/linux/generic/config-6.12 — `# CONFIG_WEXT_CORE is not set`, `# CONFIG_WEXT_PROC is not set`, `# CONFIG_WEXT_PRIV is not set`, `# CONFIG_WEXT_SPY is not set`, `# CONFIG_WIRELESS_EXT is not set`.
10. [S9] `openwrt/openwrt` — `package/network/utils/iw/Makefile` (iw 6.17; varian `iw`=tiny default, `iw-full`): https://raw.githubusercontent.com/openwrt/openwrt/main/package/network/utils/iw/Makefile
11. [S10] `openwrt/iwinfo` — `include/iwinfo.h` (`IWINFO_BUFSIZE 24 * 1024`, `struct iwinfo_assoclist_entry`, `struct iwinfo_survey_entry`) dan `iwinfo_nl80211.c` (pemetaan atribut nl80211; `e->noise = 0; /* filled in by caller */`; noise kanal disalin ke semua baris): https://raw.githubusercontent.com/openwrt/iwinfo/master/include/iwinfo.h · https://raw.githubusercontent.com/openwrt/iwinfo/master/iwinfo_nl80211.c
12. [S10b] `openwrt/iwinfo` — `api/nl80211.h` (uapi kernel yang divendorkan): `enum nl80211_sta_info` (SIGNAL, SIGNAL_AVG, CHAIN_SIGNAL, CHAIN_SIGNAL_AVG, ACK_SIGNAL…; **tidak ada** `NL80211_STA_INFO_NOISE`) vs `enum nl80211_survey_info` (`NL80211_SURVEY_INFO_NOISE` = *"noise level of channel (u8, dBm)"*): https://raw.githubusercontent.com/openwrt/iwinfo/master/api/nl80211.h
13. [S11] net-snmp — `agent/mibgroup/host/hr_proc.c` (`hrProcessorLoad` dihitung dari history CPU; `return NULL` bila history/tick tidak valid; `hrProcessorFrwID` → `nullOid`): https://raw.githubusercontent.com/net-snmp/net-snmp/master/agent/mibgroup/host/hr_proc.c
14. [S11b] Sumber `iw` — `station.c` (`iw dev <dev> station dump`; daftar field yang dicetak; tidak ada noise per stasiun): https://git.kernel.org/pub/scm/linux/kernel/git/jberg/iw.git/plain/station.c
15. [S12] net-snmp — `mibs/UCD-SNMP-MIB.txt` (`memory ::= { ucdavis 4 }`: `memTotalReal(5)`, `memAvailReal(6)`, `memTotalFree(11)`, `memShared(13)`, `memBuffer(14)`, `memCached(15)`, `memSwapError(100)`, varian `*X` 64-bit; `systemStats ::= { ucdavis 11 }`: `ssCpuUser(9)/ssCpuSystem(10)/ssCpuIdle(11)` deprecated, `ssCpuRawUser(50)`…`ssCpuRawIdle(53)`; peringatan *"Not supported on all systems!"*): https://raw.githubusercontent.com/net-snmp/net-snmp/master/mibs/UCD-SNMP-MIB.txt
16. [S12b] Dua sumber sekunder-teknis untuk dua klaim berbeda: (i) net-snmp `apps/snmpstatus.c` — hanya `STAT_TIMEOUT` → *"Timeout: No Response from %s"*, selain itu `snmp_sess_perror`: https://fossies.org/dox/net-snmp-5.9.5.2/snmpstatus_8c_source.html ; (ii) `openwrt/iwinfo` issue #15 — buffer 24 KB, laporan crash rpcd pada ±150 client: https://github.com/openwrt/iwinfo/issues/15 ; dan contoh keluaran `ubus call iwinfo assoclist` dari pengguna (**[INDIKATIF]**): https://forum.openwrt.org/t/iwinfo-assoclist-bug/151889
17. [S13] RFC 2790 — *Host Resources MIB*: `hrSystemUptime`, `hrSystemNumUsers(5)`, `hrSystemProcesses(6)`, `hrMemorySize` (*"physical read-write main memory, typically RAM"*), `hrStorageTable` + `hrStorageAllocationUnits(4)/hrStorageSize(5)/hrStorageUsed(6)`, `hrProcessorLoad` (*"average, over the last minute, of the percentage of time that this processor was not idle"*): https://www.rfc-editor.org/rfc/rfc2790.txt
18. [S14] OpenWrt patch net-snmp `750-ieee802dot11.patch` (menambah `agent/mibgroup/ieee802dot11.c/.h` + `iwlib.h`; memakai `SIOCGIWNAME`/`SIOCGIWRANGE` + `/proc/net/wireless`): https://raw.githubusercontent.com/openwrt/packages/master/net/net-snmp/patches/750-ieee802dot11.patch
19. [S15] net-snmp — `man/snmpd.conf.5.def`: sintaks `extend`/`pass`/`pass_persist`, format 3 baris OID/TYPE/VALUE, batasan *"counter64 and SNMPv2 noSuchObject exception are not supported"*, protokol `PING`/`PONG`, sintaks `com2sec [-Cn CONTEXT] SECNAME SOURCE COMMUNITY` beserta bentuk SOURCE, serta catatan bahwa data SNMP bisa dikirim lewat notification untuk perangkat di balik NAT **bila** modul `deliver/deliverByNotify` dikompilasi (modul itu **tidak** ada di daftar modul OpenWrt): https://raw.githubusercontent.com/net-snmp/net-snmp/master/man/snmpd.conf.5.def
20. [S15b] net-snmp — `mibs/NET-SNMP-EXTEND-MIB.txt` (`nsExtendObjects ::= { nsExtensions 2 }`, `nsExtendOutput1Table ::= { nsExtendObjects 3 }`, `nsExtendOutput2Table ::= { nsExtendObjects 4 }`, `nsExtendCacheTime` *"The length of time for which the output of this command will be cached…"*, `nsExtendOutLine`): https://raw.githubusercontent.com/net-snmp/net-snmp/master/mibs/NET-SNMP-EXTEND-MIB.txt
21. [S15c] net-snmp — `mibs/NET-SNMP-AGENT-MIB.txt` (`nsExtensions ::= { netSnmpObjects 3 }`): https://raw.githubusercontent.com/net-snmp/net-snmp/master/mibs/NET-SNMP-AGENT-MIB.txt
22. [S16] RFC 2863 — *The Interfaces Group MIB*: `ifIndex(1)`, `ifDescr(2)`, `ifAdminStatus(7)`, `ifOperStatus(8)`, `ifInOctets(10)`, `ifOutOctets(16)`; pada ifXEntry: `ifName(1)`, `ifHCInOctets(6)`, `ifHCOutOctets(10)`, `ifHighSpeed(15)`: https://www.rfc-editor.org/rfc/rfc2863.txt
23. [S16b] RFC 3416 — *Protocol Operations for SNMPv2* (`noSuchObject`/`noSuchInstance`/`endOfMibView`, `genErr`, `snmpSilentDrops`): https://www.rfc-editor.org/rfc/rfc3416.txt
24. [S17] RFC 4254 — *The SSH Connection Protocol*, §7 "TCP/IP Port Forwarding" / §7.1 `tcpip-forward` / §7.2 channel type `forwarded-tcpip` dan `direct-tcpip` (tidak ada forwarding UDP): https://www.rfc-editor.org/rfc/rfc4254.txt
25. [S18] Wiki OpenWrt — "TR-069 / CWMP" (daftar implementasi gratis TR-069 dan TR-369; icwmp hanya lewat feed Iopsys; EasyCwmp *"Last updated to support OpenWrt 19.07. There haven't been any updates since December 2019. Last checked June 2024."*; OktopUSP & BroadbandForum/usp untuk TR-369): https://openwrt.org/docs/guide-user/network/wan/tr-069
26. [S19] Indeks rilis resmi OpenWrt (versi stabil terkini: 25.12.5; 24.10.8 sebagai 24.10 terakhir): https://downloads.openwrt.org/releases/
27. [S20] `docs/plans/0001-mvp-scope.md` (kriteria T3, transport RADIUS via tunnel/radsecproxy, syarat keamanan SNMP v2c) dan `docs/adr/0001-suspend-via-dynamic-vlan.md` (pembedaan AP terjangkau langsung vs di balik NAT/CGNAT) — di repo ini.
28. [S21] OB-USPA (BroadbandForum/obuspa) — Agent USP: https://github.com/BroadbandForum/obuspa ; feed pihak ketiga untuk USP (SoftAtHome `feed_usp`): https://gitlab.com/soft.at.home/buildsystems/openwrt/feed_usp
29. [S22] **[INDIKATIF]** Wiki OpenWrt — Autossh (`config autossh option ssh '-i /etc/dropbear/id_rsa -N -T -R 2222:localhost:22 user@host'`) dan SSH tunnel (`sshtunnel`); dipakai hanya sebagai konfirmasi silang keberadaan paket `net/autossh/files/autossh.config`: https://openwrt.org/docs/guide-user/services/ssh/autossh · https://openwrt.org/docs/guide-user/services/ssh/sshtunnel
30. [S23] Konfirmasi ketiadaan `mini-snmpd` pada branch rilis: https://raw.githubusercontent.com/openwrt/packages/openwrt-24.10/net/mini-snmpd/Makefile → `404: Not Found`.

**Ditolak/dideprioritaskan (sekunder, tidak dipakai sebagai bukti klaim):**
- `openwrt.org/packages/pkgdata/snmpd` (menampilkan `5.9.1-7`) — indeks usang, bertentangan dengan Makefile feed.
- Halaman wiki OpenWrt "SNMPD" bagian `mini-snmpd` — paketnya sudah tidak ada di feed resmi; hanya dipakai untuk menunjukkan dokumentasi usang.
- Forum OpenWrt, Stack Exchange, `snmp-monitoring.info`, blog/Medium tentang "cara monitoring WiFi lewat SNMP" dan ambang beban SNMP — sekunder, sering tanpa versi; hanya dipakai sebagai petunjuk arah (kecuali contoh keluaran assoclist yang dikutip eksplisit sebagai **[INDIKATIF]** di [S12b]).
- Reddit / arsip milis net-snmp-users — forum; tidak dipakai.
- Mirror `openwrt-packages` pihak ketiga (mis. `AutomationD/openwrt-packages`, `hanetzer/openwrt-packages`, `DimmKirr/openwrt-packages`) dan `openwrt-xburst` (EOL) — muncul di hasil pencarian tetapi bukan sumber kanonik; **semua klaim versi/MIB di dokumen ini memakai repo `openwrt/packages` resmi**.
- Halaman wiki lama `oldwiki.archive.openwrt.org` — arsip, tidak dipakai.

---

## Langkah berikutnya

1. **Uji lokal "keberadaan metrik" pada 2–3 model `AccessPoint` target (blokir untuk #19/#20).** Di setiap perangkat: pasang `snmpd` + `snmp-utils` + `iwinfo` (atau `rpcd-mod-iwinfo`), lalu jalankan `snmpwalk` atas `.1.3.6.1.2.1.1`, `.1.3.6.1.2.1.31.1.1`, `.1.3.6.1.2.1.25`, `.1.3.6.1.4.1.2021`, `.1.3.6.1.2.1.34`; catat OID mana yang **kembali nilai**, mana yang `noSuchObject`, dan mana yang **timeout**. Output ini langsung mengisi kolom status tabel §10 dengan fakta, bukan asumsi. Sekaligus: `cat /proc/net/wireless` (membuktikan WEXT mati), `ubus list iwinfo`, `which hostapd_cli`, dan `opkg info snmpd libnetsnmp libpci libpcre2` + `df -h /` (anggaran flash).
2. **Tentukan arti ulang metrik WiFi dan mintakan keputusan produk.** Ubah rumusan "signal/noise per radio" di ADR-0002/#20 menjadi *signal·signal_avg per client (agregat terlemah/rata-rata per interface) + noise floor kanal per interface + jumlah client per interface*, dan **coret** noise-per-client sebagai mustahil. Ini keputusan yang memengaruhi model data time-series; hasilkan sebagai usulan revisi ADR/issue, bukan perubahan langsung.
3. **Pilot keterjangkauan (keputusan push vs pull).** Pada 3–5 lokasi nyata, ukur: berapa `AccessPoint` yang bisa di-poll SNMP dari server (termasuk opsi port-forward), berapa yang hanya bisa keluar. Bila mayoritas tidak terjangkau, naikkan agent push ke MVP. Sekalian ukur waktu satu siklus polling penuh pada server (termasuk AP yang selalu timeout) untuk menetapkan timeout/retry/concurrency poller #19.
4. **Rancang dan uji prototipe jalur skrip (§5) sebelum menulis adapter.** Bikin satu skrip `pass_persist` + satu `extend` di perangkat pilot, ukur CPU/waktu (`time`, `top`) pada 0/5/50 client, dan verifikasi (a) tanda nilai `signal`/`noise` dari ubus, (b) perilaku saat `ubus call iwinfo` gagal (skrip harus tetap menjawab `NONE`, bukan menggantung), (c) apakah `iwinfo assoclist` stabil pada AP dengan client banyak atau harus diganti `iw ... station dump`. Hasilnya jadi kontrak eksplisit antara adapter SNMP dan skrip perangkat.
