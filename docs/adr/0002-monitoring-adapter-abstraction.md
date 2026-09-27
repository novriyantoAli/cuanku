# Abstraksi adapter monitoring (SNMP default, siap agent push)

Metrik `AccessPoint` dikoleksi lewat antarmuka adapter yang dapat diganti. Implementasi MVP adalah polling SNMP v2c tiap 5 menit untuk `AccessPoint` yang punya jalur IP inbound, dengan cakupan: status online/offline/unreachable, uptime, CPU/memori, traffic RX/TX, dan metrik WiFi agregat per interface (jumlah klien, signal terlemah/rata-rata, noise floor kanal). Reachability tidak merata — sebagian `AccessPoint` berada di balik NAT/CGNAT sehingga SNMP (pull) tidak mungkin — maka adapter diabstraksikan agar mekanisme push dapat ditambahkan tanpa mengubah model data.

Status: accepted

## Alasan

- **Reachability tidak merata**: SNMP adalah protokol pull; server tidak bisa menarik metrik dari `AccessPoint` di balik NAT. Reverse tunnel bukan jalan keluar — SSH hanya mem-forward TCP sedangkan SNMP berjalan di UDP/161 — dan TR-069/TR-369 tidak tersedia sebagai paket resmi OpenWrt. Mekanisme push yang realistis adalah agent yang menginisiasi koneksi keluar dari `AccessPoint`, pola yang sudah ada karena `AccessPoint` memang menginisiasi tunnel RADIUS keluar.
- **Anti lock-in**: mengganti mekanisme monitoring tidak boleh mengubah domain model atau skema data.

## Konsekuensi

- Antarmuka adapter monitoring harus netral — detail protokol (OID SNMP, dsb.) tidak boleh bocor ke domain layer.
- Kontrak adapter harus **siap-push sejak awal**: metrik dapat di-ingest lewat API, bukan hanya ditarik oleh poller.
- Poller berjalan sebagai job/worker terpisah, bukan sinkron di request path.
- Penyimpanan metrik time-series harus independen dari mekanisme koleksi.
- Beban polling harus diukur di perangkat nyata; tidak ada sumber primer yang menetapkan ambang jumlah OID yang aman untuk router kelas bawah.

## Revisi (2026-09-27) — hasil riset net-snmp/OpenWrt

Keputusan inti tetap berdiri (adapter netral, poller terpisah, time-series independen). Yang dikoreksi:

1. **Kosakata metrik.** Frasa lama "signal/noise per radio" tidak punya padanan di nl80211. Yang benar-benar tersedia: `signal` dan `signal_avg` **per klien**, dan `noise` sebagai **noise floor kanal per interface** (satu nilai yang dipakai ulang untuk semua klien). **Noise per klien tidak ada sama sekali.** Metrik disimpan sebagai agregat per interface (jumlah klien, signal terlemah, signal rata-rata, noise floor kanal), bukan sampel per klien, agar kardinalitas time-series tetap terbatas.
2. **CPU/memori pindah ke UCD-SNMP-MIB** (`ssCpuRaw*`, `memTotalReal`/`memAvailReal`/`memTotalFree`). HOST-RESOURCES-MIB memang terkompilasi, tetapi `hrProcessorLoad` dihitung dari riwayat CPU internal dan dapat mengembalikan `noSuchInstance`.
3. **`IEEE802dot11-MIB` efektif mati.** OpenWrt menambalnya masuk, tetapi implementasinya bergantung pada Wireless Extensions, sementara kernel OpenWrt mematikan WEXT.
4. **Jumlah klien dan metrik WiFi hanya lewat skrip perangkat** (`extend`/`pass_persist` membaca `ubus call iwinfo assoclist` atau `iw dev <dev> station dump`) — tidak ada di MIB mana pun. Catatan kontrak: `pass_persist` tidak dapat mengembalikan `counter64` maupun `noSuchObject`, sedangkan `extend` mengembalikan `DisplayString` dan punya `cacheTime`. Seluruh metrik WiFi adalah gauge, jadi jalur `extend` dipilih.
5. **Koreksi klaim alternatif.** "Reverse tunnel" tidak menyelesaikan masalah (dua relay UDP↔TCP per `AccessPoint`), dan "TR-069/ACS" tidak tersedia di feed resmi OpenWrt (implementasi yang ada berada di feed pihak ketiga; EasyCwmp berhenti di 19.07). Karena T3 di `docs/plans/0001-mvp-scope.md` sudah membatasi janji ke "AP terjangkau SNMP", `AccessPoint` di balik NAT berstatus `unreachable` pada MVP tanpa melanggar kriteria sukses.
6. **Offline vs unreachable tidak dapat dibedakan dari lapisan SNMP saja** — klien protokol hanya membedakan timeout dari error, dan community salah menghasilkan timeout yang identik dengan host mati. Pembedaan harus berlapis (ICMP/TCP, riwayat, metadata apakah `AccessPoint` di balik NAT).
7. **Tersedia standar dan aman digunakan:** `sysUpTime` (SNMPv2-MIB), `ifHCInOctets`/`ifHCOutOctets`/`ifOperStatus`/`ifName` (IF-MIB + ifXTable) — interface `wlan*`/`phyX-apY` adalah netdev biasa sehingga ikut ter-enumerasi.

Detail bukti dan tabel OID: `docs/research/0004-openwrt-snmp-monitoring.md`.
