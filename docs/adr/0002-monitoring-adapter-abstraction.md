# Abstraksi adapter monitoring (SNMP default, siap TR-069)

Metrik `AccessPoint` dikoleksi lewat antarmuka adapter yang dapat diganti. Implementasi MVP adalah polling SNMP v2c tiap 5 menit dengan cakupan lengkap (status online/offline, uptime, CPU/memori, signal/noise per radio, jumlah client, traffic RX/TX). Reachability AccessPoint belum pasti — AccessPoint rumahan sering di balik NAT/CGNAT sehingga SNMP (pull) bisa gagal — maka adapter diabstraksikan agar bisa diganti ke TR-069/ACS atau agent push tanpa mengubah model data.

Status: accepted

## Alasan

- **Reachability tidak pasti**: SNMP adalah protokol pull; server tidak bisa polling AccessPoint di balik NAT. TR-069/ACS (push) atau reverse tunnel adalah alternatif yang mungkin diperlukan.
- **Anti lock-in**: mengganti mekanisme monitoring tidak boleh mengubah domain model atau skema data.

## Konsekuensi

- Antarmuka adapter monitoring harus netral — detail protokol (OID SNMP, dsb.) tidak boleh bocor ke domain layer.
- Poller berjalan sebagai job/worker terpisah, bukan sinkron di request path.
- Penyimpanan metrik time-series harus independen dari mekanisme koleksi.
- Metrik WiFi (signal/noise per radio, jumlah client) TIDAK tersedia di net-snmp standar OpenWrt; butuh skrip net-snmp extend/pass (baca ubus/hostapd) atau custom agent. CPU/memori butuh HOST-RESOURCES-MIB aktif. Traffic RX/TX (IF-MIB) dan uptime (sysUpTime) tersedia standar.
