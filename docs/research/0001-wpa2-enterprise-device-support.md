# Riset: Dukungan Perangkat WPA2-Enterprise (802.1X, EAP-PEAP/MSCHAPv2)

Riset ini menjawab asumsi pemblokir di `docs/plans/0001-mvp-scope.md`: *"Perangkat klien Customer mendukung WPA2-Enterprise (802.1X/PEAP-MSCHAPv2). Smart TV/IoT mungkin TIDAK mendukung — perlu fallback (PSK per-Customer / MAC-auth) bila populasi perangkat tidak kompatibel."*

Fokus: autentikasi **EAP-PEAP/MSCHAPv2 dengan username + password** (bukan EAP-TLS berbasis sertifikat, bukan captive portal).

## Ringkasan

Perangkat komputasi umum (ponsel/tablet Android & iOS, laptop Windows & macOS, serta Apple TV) **mendukung penuh** PEAP-MSCHAPv2. Sebaliknya, mayoritas perangkat *non-BYOD* — Smart TV (Samsung/LG), streaming stick/box (Fire TV, Chromecast/Google TV, Roku), konsol game (PlayStation, Xbox, Nintendo Switch), speaker/pintar (Amazon Echo, Google Nest), dan perangkat IoT/smart-home umum — **tidak mendukung** 802.1X berbasis username+password. Dengan demikian asumsi pemblokir **TERKONFIRMASI**: tanpa fallback (PSK per-Customer atau MAC-auth), sebagian besar perangkat rumah pelanggan tidak akan bisa bergabung ke SSID WPA2-Enterprise PEAP-MSCHAPv2.

## Tabel ringkasan

| Kategori | Verdict (PEAP-MSCHAPv2) | Basis bukti |
|---|---|---|
| Android (ponsel/tablet) | **Didukung** | Sumber primer Android |
| iOS / iPadOS | **Didukung** | Sumber primer Apple |
| Windows | **Didukung** | Sumber primer Microsoft |
| macOS | **Didukung** | Sumber primer Apple |
| Apple TV | **Didukung** (Apple TV 4K gen-3; model lama perlu verifikasi) | Sumber primer Apple |
| Google TV / Chromecast | **Tidak didukung** | Sumber primer Google |
| Amazon Fire TV | **Tidak didukung** | Sumber primer Amazon |
| Roku | **Tidak didukung** (inferensi dari dokumentasi resmi) | Roku Support |
| Samsung Tizen TV (konsumen) | **Tidak didukung** (inferensi dari manual resmi) | Manual resmi Samsung |
| LG webOS TV (konsumen) | **Tidak terverifikasi — perlu uji lapangan** | Manual/developer LG |
| PlayStation (PS4/PS5) | **Tidak didukung** | Sumber primer Sony |
| Xbox (One/Series) | **Tidak didukung** | Pesan resmi konsol / Microsoft |
| Nintendo Switch | **Tidak didukung** | Sumber primer Nintendo |
| Amazon Echo / Alexa | **Sebagian** — WPA2-Enterprise hanya EAP-TLS (sertifikat), *bukan* PEAP-MSCHAPv2 | Sumber primer Amazon |
| Google Nest (speaker/display) | **Tidak didukung** | Sumber primer Google |
| Printer (HP/Brother/Epson) | **Sebagian** — PEAP-MSCHAPv2 didukung, tergantung model & konfigurasi Web | Sumber primer vendor |
| IoT / smart-home umum (mis. Tuya) | **Tidak didukung** (representatif) | Dokumen developer Tuya |

## Catatan per kategori

Legenda bukti: **[Bukti langsung]** = sumber menyatakan klaim secara eksplisit · **[Interpretasi sumber]** = klaim disimpulkan dari isi sumber · **[Inferensi peneliti]** = kesimpulan peneliti, bukan teks sumber. Kepercayaan: tinggi/sedang/rendah.

### 1. Android (ponsel & tablet)
- **(a)** Mendukung PEAP-MSCHAPv2: **Ya**. Stack wpa_supplicant Android mencantumkan `EAP-PEAP/MSCHAPv2` (PEAPv0 dan PEAPv1) sebagai metode autentikasi yang didukung; API platform menyediakan konstanta `PEAP` (EAP luar) dan `MSCHAPV2` (fase-2). **[Bukti langsung]** — Kepercayaan: tinggi.
- **(b)** Catatan:
  - Sejak **Android 11 QPR1**, sistem mewajibkan konfigurasi ketat untuk metode Enterprise berbasis TLS (PEAP/TLS/TTLS): wajib memasang **Root CA certificate** DAN **domain suffix match / alternate subject match**. Konfigurasi tanpa keduanya ditolak/tidak tersimpan (untuk jalur konfigurasi via aplikasi/`addNetwork`). **[Bukti langsung]** — implikasi praktis bagi WISP: sertifikat server RADIUS harus dipercaya (CA) dan cocok domain; bukan sekadar "do not validate".
  - UI menawarkan pilihan keamanan "WPA/WPA2/WPA3-Enterprise" / "802.1X EAP" pada dialog tambah jaringan; SSID tersembunyi harus ditambahkan manual. OEM skin tertentu dapat menyembunyikan opsi ini — perlu uji pada perangkat target. **[Interpretasi sumber + inferensi peneliti]**.
- **(c)** Sumber:
  - [Secure Wi-Fi Enterprise configuration — Android Developers](https://developer.android.com/guide/topics/connectivity/wifi-enterprise)
  - [wpa_supplicant README — platform/external/wpa_supplicant_8 (Android Git)](https://android.googlesource.com/platform/external/wpa_supplicant_8/+/a5777693d621f768bf7149da381d4856c109e70a/wpa_supplicant/README)
  - [WifiEnterpriseConfig.Phase2 — Android API reference](https://developer.android.com/reference/android/net/wifi/WifiEnterpriseConfig.Phase2)

### 2. iOS / iPadOS
- **(a)** Mendukung PEAP-MSCHAPv2: **Ya**. Apple mendokumentasikan metode EAP untuk WPA/WPA2 Enterprise: `PEAP (EAP-MSCHAPv2, bentuk PEAP paling umum)` dan `TTLS (MSCHAPv2)`, selain TLS/EAP-FAST/EAP-SIM/EAP-AKA. Username/password dapat diisi dalam profil atau diminta ke pengguna. **[Bukti langsung]** — Kepercayaan: tinggi.
- **(b)** Catatan:
  - Sertifikat server RADIUS **harus dipercaya** (diterima manual saat pertama join, atau lewat profil konfigurasi); tidak ada mode "jangan validasi" yang setara untuk deployment. **[Bukti langsung]**.
  - SSID tersembunyi: pilih "Lainnya" lalu ketik SSID. **[Interpretasi sumber]**.
  - Ethernet 802.1X tersedia sejak iOS 17 / iPadOS 17 (untuk adaptor USB-C/Ethernet). **[Bukti langsung]**.
- **(c)** Sumber:
  - [Extensible Authentication Protocol (EAP) device management settings — Apple Support](https://support.apple.com/guide/deployment/eap-settings-dep5d180f86a/web)
  - [Connect Apple devices to 802.1X networks — Apple Support](https://support.apple.com/guide/deployment/connect-to-8021x-networks-depabc994b84/web)
  - [How Apple devices join Wi-Fi networks — Apple Support](https://support.apple.com/guide/deployment/how-apple-devices-join-wi-fi-networks-dep3b0448c58/web)

### 3. Windows
- **(a)** Mendukung PEAP-MSCHAPv2: **Ya**. Microsoft menyediakan contoh profil resmi *"WPA2-Enterprise with PEAP-MSCHAPv2"* yang mengautentikasi dengan `UserName/Password`, dan panduan deployment 802.1X Windows Server. **[Bukti langsung]** — Kepercayaan: tinggi.
- **(b)** Catatan: validasi sertifikat server dikonfigurasi di properti koneksi (ada opsi "Don't validate server certificate" — tidak disarankan); SSID tersembunyi ditambahkan manual. **[Interpretasi sumber]**.
- **(c)** Sumber:
  - [WPA2-Enterprise with PEAP-MSCHAPv2 profile sample — Microsoft (Native Wi-Fi)](https://learn.microsoft.com/en-us/windows/win32/nativewifi/wpa2-enterprise-with-peap-mschapv2-profile-sample)
  - [Deploy 802.1X wireless access — Microsoft Learn](https://learn.microsoft.com/en-us/windows-server/networking/core-network-guide/cncg/wireless/a-deploy-8021x-wireless-access)

### 4. macOS
- **(a)** Mendukung PEAP-MSCHAPv2: **Ya**. "macOS Setup Assistant mendukung autentikasi 802.1X dengan username+password menggunakan TTLS atau PEAP"; mode *System+User* memakai kredensial username/passphrase (EAP-PEAP). **[Bukti langsung]** — Kepercayaan: tinggi.
- **(b)** Catatan: kepercayaan sertifikat RADIUS wajib (accept saat join atau via profil); mode System/Login Window memerlukan MDM. **[Bukti langsung]**.
- **(c)** Sumber:
  - [Connect Apple devices to 802.1X networks — Apple Support](https://support.apple.com/guide/deployment/connect-to-8021x-networks-depabc994b84/web)
  - [Extensible Authentication Protocol (EAP) settings — Apple Support](https://support.apple.com/guide/deployment/eap-settings-dep5d180f86a/web)

### 5. Apple TV
- **(a)** Mendukung PEAP-MSCHAPv2: **Ya** untuk Apple TV yang tercantum. Dokumentasi Apple mencantumkan **Apple TV 4K (generasi ke-3)** sebagai perangkat yang dapat terhubung ke jaringan 802.1X (Wi-Fi; Ethernet 802.1X hanya tvOS 17+). Metode EAP sama dengan platform Apple lain (termasuk PEAP). **[Bukti langsung]** — Kepercayaan: tinggi (untuk model yang tercantum).
- **(b)** Catatan: model Apple TV lama (HD/4K gen 1–2) **tidak tercantum** di dokumen terkini — statusnya perlu verifikasi per model. Konfigurasi umumnya lewat profil (Apple Configurator/MDM), meski tvOS menyediakan entri manual. **[Interpretasi sumber + inferensi peneliti]**.
- **(c)** Sumber:
  - [Connect Apple devices to 802.1X networks — Apple Support](https://support.apple.com/guide/deployment/connect-to-8021x-networks-depabc994b84/web)

### 6. Google TV / Chromecast
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak**. Google menyatakan eksplisit: *"Cast receivers do not support WPA2 Enterprise (WPA 802.1X)"* dan FAQ *"Do cast receivers support Enterprise 802.1x networks? No."* Chromecast with Google TV (4K/HD) termasuk *cast receiver*; koneksi ethernetnya *"does not work with WPA2-Enterprise/802.1X authentication"* dan *"does not support any sort of enterprise certificates"*. **[Bukti langsung]** — Kepercayaan: tinggi.
- **(b)** Catatan: hanya pengirim cast (laptop/ponsel) yang boleh berada di jaringan Enterprise; receiver wajib WPA2-PSK atau Ethernet. **[Bukti langsung]**.
- **(c)** Sumber:
  - [Set up cast moderator — Google Chrome Enterprise & Education](https://support.google.com/chrome/a/answer/11598277?hl=en)
  - [Network requirements for cast moderator — Google Chrome Enterprise & Education](https://support.google.com/chrome/a/answer/12256492?hl=en)

### 7. Amazon Fire TV
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak**. Panduan resmi Fire TV mendaftar jenis jaringan yang didukung: *"Open, WEP, WPA-PSK, and WPA2-PSK encrypted networks"* — tidak ada EAP/802.1X/Enterprise. **[Bukti langsung]** — Kepercayaan: tinggi.
- **(b)** Catatan: Fire TV (FireOS) tidak mendokumentasikan metode Enterprise apa pun; hanya PSK/WEP/open. **[Interpretasi sumber]**.
- **(c)** Sumber:
  - [Amazon Fire TV User Guide (PDF resmi)](https://d1ergij2b6wmg5.cloudfront.net/Amazon+Fire+TV+User+Guides/Amazon+Fire+TV+Device+Documentation/Amazon_Fire_TV_User_Guide.pdf)
  - [Can't Connect Your Fire TV Device to Wi-Fi — Amazon Help](https://www.amazon.com/gp/help/customer/display.html?nodeId=GYMDPRQTTLXQD7PL)

### 8. Roku
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak** (inferensi). Dokumentasi resmi Roku tidak mencantumkan 802.1X/EAP/Enterprise sama sekali; satu-satunya mekanisme untuk jaringan publik/akun adalah **Hotel & Dorm Connect** (login captive-portal via browser ponsel/komputer), yang **bukan** 802.1X. **[Interpretasi sumber + inferensi peneliti]** — Kepercayaan: sedang.
- **(b)** Catatan: tidak ada pernyataan resmi Roku yang eksplisit "tidak mendukung 802.1X" yang ditemukan; kesimpulan didasarkan pada ketiadaan fitur di dokumentasi resmi. **[Inferensi peneliti]**.
- **(c)** Sumber:
  - [How do I use Hotel & Dorm Connect? — Official Roku Support](https://support.roku.com/article/how-do-i-use-hotel-dorm-connect-to-connect-to-the-internet)
  - [Learn about advanced networking features — Official Roku Support](https://support.roku.com/article/advanced-networking-features)

### 9. Samsung Tizen TV (konsumen)
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak** (inferensi kuat). Manual resmi Samsung untuk TV konsumen (banyak model, lintas tahun) menyatakan TV *"hanya mendukung protokol keamanan jaringan nirkabel berikut: Authentication Modes: WEP, WPAPSK, WPA2PSK"* — tidak ada mode Enterprise/802.1X. **[Interpretasi sumber]** — Kepercayaan: sedang-tinggi.
- **(b)** Catatan penting (kontras): **display komersial/signage Samsung** (B2B) secara eksplisit mendukung `802.1x (WPA2 Enterprise): EAP-TLS, EAP-TTLS, EAP-PEAP` — artinya dukungan Enterprise ada di lini signage, **bukan** di TV konsumen Tizen. **[Bukti langsung untuk signage]**.
- **(c)** Sumber:
  - [Manual resmi Samsung TV (Samsung Download Center, PDF)](https://downloadcenter.samsung.com/content/UM/202404/20240426190921443/BN81-24055C-460_EM_OSPDVBEUC_EU_ENG_240417.0.pdf)
  - [Samsung QHB/QMB/QBB Professional Display brochure (802.1x EAP-TLS/TTLS/PEAP)](https://images.samsung.com/is/content/samsung/assets/global/p6-b2b/gro1/ds-com/smart-signage/4k-signage/2023_0313/QBBQMBQHB-Brochure.pdf)

### 10. LG webOS TV (konsumen)
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak terverifikasi — perlu uji lapangan**. Tidak ditemukan pernyataan resmi LG yang eksplisit untuk TV webOS konsumen. Manual online webOS konsumen hanya mendokumentasikan entri password PSK; spesifikasi **signage webOS** mencantumkan `Security: 802.1X EAP` sebagai fitur signage — kontras serupa dengan Samsung. **[Interpretasi sumber + inferensi peneliti]** — Kepercayaan: rendah-sedang.
- **(b)** Catatan: beri status "perlu uji lapangan" untuk TV webOS konsumen sebelum memutuskan fallback per model. **[Inferensi peneliti]**.
- **(c)** Sumber:
  - [webOS Signage hardware specifications (802.1X EAP) — LG Developer](https://webossignage.developer.lge.com/discover/webos-signage/hardware-spec)
  - [LG Smart TV online manual (network settings) — eGuide](https://eguide.lgappstv.com/manual/gb/12003_2.html)

### 11. PlayStation (PS4/PS5)
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak**. Situs resmi PlayStation untuk koneksi Wi-Fi hanya menyebut entri *"Wi-Fi password (WPA, WPA2, WEP, WPA3)"* — tidak ada opsi 802.1X/Enterprise/username+password. **[Interpretasi sumber]** — Kepercayaan: sedang-tinggi.
- **(b)** Catatan: kesimpulan didasarkan pada daftar metode keamanan PSK-only di dokumentasi resmi Sony (absennya 802.1X). **[Inferensi peneliti]**.
- **(c)** Sumber:
  - [How to set up an internet connection on PlayStation consoles — PlayStation Support](https://www.playstation.com/en-us/support/connectivity/internet-connect-playstation/)

### 12. Xbox (One/Series)
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak**. Pesan resmi konsol saat menemui protokol tak dikenal berbunyi: *"This console supports WPA2/WPA3 (personal), WPA/WPA2 (personal), WPA2 (personal), and WEP network security protocols"* — hanya mode *personal* (PSK) + WEP, tanpa Enterprise/802.1X. **[Bukti langsung dari pesan konsol]** — Kepercayaan: tinggi.
- **(b)** Catatan: pesan ini dikutip pada forum resmi Microsoft Answers; halaman resmi support.xbox.com "Troubleshoot a wireless network connection" adalah lokasi kanonik pemecahan masalah. **[Interpretasi sumber]**.
- **(c)** Sumber:
  - [Troubleshoot a wireless network connection — Xbox Support](https://support.xbox.com/en-US/help/hardware-network/connect-network/xbox-one-wireless-connection)
  - [Pesan protokol keamanan Xbox (kutipan konsol) — Microsoft Answers](https://answers.microsoft.com/en-us/xbox/forum/all/how-do-i-fix-security-protocol-error-message-on-my/af64ed81-d945-463b-8fc5-1430d6223730)

### 13. Nintendo Switch
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak**. Tabel resmi Nintendo: Switch 2 mendukung `WPA-PSK(AES), WPA2-PSK(AES), WPA3-SAE(AES)`; Switch/OLED/Lite mendukung `WEP, WPA-PSK(AES), WPA2-PSK(AES)` — tanpa mode Enterprise/802.1X. **[Bukti langsung]** — Kepercayaan: tinggi.
- **(b)** Catatan: hanya PSK/WEP; tidak ada opsi username+password. **[Interpretasi sumber]**.
- **(c)** Sumber:
  - [Compatible Wireless Modes and Wireless Security Types — Nintendo Support](https://en-americas-support.nintendo.com/app/answers/detail/a_id/498)

### 14. Amazon Echo / Alexa
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak**. Dukungan WPA2-Enterprise Echo hanya via **EAP-TLS berbasis sertifikat**, dikelola melalui **Alexa Smart Properties** (atau Alexa for Business): perangkat memperoleh sertifikat dari CA yang dikelola ASP dan dirotasi otomatis — memerlukan server RADIUS yang menandatangani CSR. Tidak ada jalur username+password PEAP-MSCHAPv2 untuk perangkat konsumen. **[Bukti langsung]** — Kepercayaan: tinggi.
- **(b)** Catatan: untuk WISP dengan PEAP-MSCHAPv2 username+password, Echo **tidak** dapat bergabung; jalur EAP-TLS ini juga hanya untuk perangkat "ASP supported" dan skema billing ASP (bukan setup konsumen biasa). **[Interpretasi sumber]**.
- **(c)** Sumber:
  - [WPA2 Enterprise Wi-Fi — Alexa Smart Properties, Amazon Developer](https://developer.amazon.com/en-US/docs/alexa/alexa-smart-properties/wpa2-enterprise-wifi-for-asp.html)
  - [Alexa for Business Adds WPA2 Enterprise Wi-Fi Support — AWS What's New](https://aws.amazon.com/about-aws/whats-new/2018/12/alexa-for-business-adds-wpa2-enterprise-wi-fi-support-for-shared/)

### 15. Google Nest (speaker & display)
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak**. Google menyatakan: *"Enterprise networks in businesses such as 802.1x/RADIUS networks are not compatible with Nest products."* dan spesifikasi perangkat mencantumkan *"WPA2-Enterprise is not supported."* **[Bukti langsung]** — Kepercayaan: tinggi.
- **(b)** Catatan: captive portal juga tidak didukung. **[Bukti langsung]**.
- **(c)** Sumber:
  - [Wi-Fi networks that are incompatible or aren't recommended — Google Nest Help](https://support.google.com/googlehome/answer/9249905?hl=en)
  - [Google Nest and Home device specifications — Google Nest Help](https://support.google.com/googlehome/answer/7072284?hl=en-GB)

### 16. Printer (HP / Brother / Epson)
- **(a)** Mendukung PEAP-MSCHAPv2: **Ya, pada model yang mendukung 802.1X** (umumnya printer bisnis/enterprise). HP Jetdirect mendukung PEAP (+ EAP-TLS); Brother mencantumkan `PEAP/MS-CHAPv2` sebagai metode autentikasi Enterprise; Epson mencantumkan `PEAP/MSCHAPv2` sebagai metode antara printer dan server RADIUS. **[Bukti langsung]** — Kepercayaan: tinggi (untuk model yang didokumentasikan).
- **(b)** Catatan: dukungan **tergantung model & firmware** (tidak semua printer rumahan); konfigurasi dilakukan via **Web Config / Embedded Web Server (EWS)** dengan User ID + Password + verifikasi sertifikat server, bukan UI on-device sederhana — ini menyulitkan konsumen non-teknis. **[Interpretasi sumber]**.
- **(c)** Sumber:
  - [How to Use 802.1X on HP Jetdirect Print Servers (HP)](https://h10032.www1.hp.com/ctg/Manual/c00731218.pdf)
  - [Configure Your Machine for an Enterprise Wireless Network (Brother)](https://support.brother.com/g/s/id/htmldoc/printer/cv_hll2460dw/uke/html/GUID-EE53FF46-6B72-416B-A67A-2E7A16BB8EA3_1.html)
  - [IEEE 802.1X Network Settings (Epson)](https://files.support.epson.com/docid/cpd6/cpd62840/source/printers/source/administration/references/ieee8021x_setting_options.html)

### 17. IoT / smart-home umum (representatif: Tuya)
- **(a)** Mendukung PEAP-MSCHAPv2: **Tidak** (representatif). Mekanisme pairing SDK Tuya (EZ mode / AP mode) hanya meneruskan **SSID + password** dari aplikasi ke perangkat; tidak ada konsep username/password 802.1X ataupun sertifikat dalam alur provisioning standar. **[Interpretasi sumber]** — Kepercayaan: sedang (representatif, bukan enumerasi lengkap).
- **(b)** Catatan: mayoritas smart plug/bulb/sensor konsumen di ekosistem Tuya dan sejenis tidak mendukung WPA2-Enterprise; tidak ada UI username+password di perangkat. **[Inferensi peneliti]**.
- **(c)** Sumber:
  - [Pairing Wireless Gateways in EZ Mode — Tuya Developer](https://developer.tuya.com/en/docs/iot-device-dev/integrated_sdk_ez_commissioning_guide?id=Kb9p8i00u3p6v)
  - [AP Pairing — TuyaOS, Tuya Developer](https://developer.tuya.com/en/docs/iot-device-dev/TuyaOS-iot_abi_network_config_AP?id=Kc67sz8ud0obw)

## Kontradiksi & catatan bukti

- **Samsung konsumen vs signage**: TV konsumen Tizen (manual: hanya WEP/WPAPSK/WPA2PSK) **kontradiktif** dengan display signage Samsung (brosur resmi: 802.1x EAP-TLS/TTLS/PEAP didukung). Jangan menganggap "Samsung" secara umum tidak mendukung — hanya lini konsumen. Sumber: manual resmi Samsung TV vs [brosur Samsung QHB/QMB/QBB](https://images.samsung.com/is/content/samsung/assets/global/p6-b2b/gro1/ds-com/smart-signage/4k-signage/2023_0313/QBBQMBQHB-Brochure.pdf).
- **LG konsumen vs signage**: pola yang sama — spesifikasi signage webOS mencantumkan `802.1X EAP`, sementara manual konsumen tidak. Ini alasan status "Tidak terverifikasi" untuk webOS konsumen.
- **Amazon Echo**: mendukung WPA2-Enterprise, tetapi **hanya EAP-TLS (sertifikat)** via Alexa Smart Properties/Alexa for Business — *bukan* PEAP-MSCHAPv2. Sebagian sumber sekunder lama mengklaim "Echo tidak mendukung WPA2-Enterprise" tanpa nuansa ini; klaim yang benar adalah "mendukung WPA2-Enterprise hanya dalam bentuk EAP-TLS terkelola".
- **Google "tidak direkomendasikan" vs "tidak kompatibel"**: halaman Google membedakan jaringan yang "tidak direkomendasikan" (hotspot, guest) dari yang "tidak kompatibel" (captive portal, enterprise 802.1x/RADIUS). Enterprise masuk kategori **tidak kompatibel**, bukan sekadar tidak disarankan.

## Bukti yang belum terverifikasi (missing evidence)

- **LG webOS TV konsumen**: tidak ada pernyataan resmi LG "mendukung/tidak mendukung 802.1X" untuk TV konsumen — perlu uji lapangan per model.
- **Roku**: tidak ada pernyataan resmi Roku yang eksplisit menolak 802.1X; verdict "Tidak didukung" adalah inferensi dari ketiadaan fitur di dokumentasi resmi.
- **Samsung TV konsumen (model terbaru)**: teks manual "WEP/WPAPSK/WPA2PSK" diverifikasi lewat manual resmi lintas tahun, tetapi belum diverifikasi kata-per-kata pada PDF model 2024 terbaru (canonical: Samsung Download Center).
- **Printer Canon**: tidak selesai diverifikasi pada pass ini (HP/Brother/Epson sudah terverifikasi PEAP-MSCHAPv2); Canon diyakini serupa tetapi belum dikonfirmasi dari sumber primer.
- **IoT umum**: verdict didasarkan pada ekosistem Tuya sebagai perwakilan; cakupan penuh seluruh chip/vendor IoT belum dienumerasi. Beberapa perangkat IoT enterprise khusus (mis. signage, smart plug kelas enterprise) mungkin mendukung — tetapi ini pengecualian, bukan populasi umum pelanggan WISP.
- **Model Apple TV lama (HD, 4K gen 1–2)**: dokumen Apple terkini hanya mencantumkan Apple TV 4K gen-3; status 802.1X model lama belum diverifikasi.

## Implikasi untuk keputusan fallback

**Kelas perangkat berisiko tinggi (TIDAK akan terhubung ke SSID WPA2-Enterprise PEAP-MSCHAPv2), sehingga memaksa fallback PSK-per-Customer atau MAC-auth:**

1. **Streaming stick/box**: Amazon Fire TV, Chromecast/Google TV, Roku — semuanya PSK-only.
2. **Smart TV**: Samsung Tizen (konsumen) — PSK-only; LG webOS (konsumen) — sangat mungkin PSK-only (perlu uji lapangan).
3. **Konsol game**: PlayStation, Xbox, Nintendo Switch — PSK/WEP only.
4. **Speaker/pintar**: Amazon Echo (hanya EAP-TLS terkelola, bukan PEAP-MSCHAPv2) dan Google Nest (802.1x/RADIUS tidak kompatibel).
5. **IoT/smart-home**: smart plug/bulb/sensor (ekosistem Tuya dan sejenis) — provisioning SSID+password saja.

**Kelas perangkat aman (bekerja penuh dengan PEAP-MSCHAPv2):**

- Ponsel/tablet Android & iOS, laptop Windows & macOS, Apple TV (gen terbaru), dan printer bisnis yang mendukung 802.1X.

**Kesimpulan keputusan:** asumsi pemblokir di `0001-mvp-scope.md` **terkonfirmasi benar**. Populasi perangkat rumah pelanggan WISP hampir pasti memuat beberapa perangkat dari kelas berisiko tinggi di atas. Oleh karena itu, arsitektur murni *single-SSID WPA2-Enterprise PEAP-MSCHAPv2* tidak layak untuk pengalaman pelanggan yang utuh. **[Inferensi peneliti]** — Rekomendasi arah (untuk dibahas di ADR, bukan keputusan riset ini):

- **Dual-SSID**: satu SSID WPA2-Enterprise (PEAP-MSCHAPv2) untuk BYOD (ponsel/laptop/tablet), plus **satu SSID PSK unik per-Customer** untuk Smart TV/konsol/IoT — PSK dirotasi/ikatan ke `RadiusAccount` per-Customer.
- Atau **MAC-auth (MAB)** pada SSID terpisah untuk perangkat tanpa UI username+password, dengan pendaftaran MAC per-Customer.
- Kedua fallback harus tetap mengikat sesi ke `Customer`/`RadiusAccount` yang sama agar billing usage-based (`Balance`/`Tariff`) tetap benar.

## Sumber

**Dipertahankan (primer/otoritatif):**
- Android Developers — Secure Wi-Fi Enterprise configuration — syarat CA + domain match Android 11+.
- Android Git (wpa_supplicant README) — daftar EAP-PEAP/MSCHAPv2.
- Apple Support (deployment) — EAP settings & Connect to 802.1X — PEAP/TTLS, Apple TV.
- Microsoft Learn — profil PEAP-MSCHAPv2 & panduan 802.1X.
- Nintendo Support — tabel mode keamanan konsol.
- Google (Chrome Enterprise) — "Cast receivers do not support WPA2 Enterprise".
- Amazon Fire TV User Guide (PDF) — daftar jaringan PSK-only.
- Roku Support — Hotel & Dorm Connect, advanced networking.
- Samsung Download Center + brosur signage — kontras konsumen vs B2B.
- LG Developer (webOS signage) + LG eGuide — dasar "Tidak terverifikasi".
- PlayStation Support — koneksi Wi-Fi PSK-only.
- Xbox Support + Microsoft Answers — pesan protokol keamanan konsol.
- Amazon Developer (Alexa Smart Properties) — WPA2-Enterprise EAP-TLS only.
- Google Nest Help — 802.1x/RADIUS tidak kompatibel; WPA2-Enterprise tidak didukung.
- HP / Brother / Epson — dokumentasi 802.1X printer.
- Tuya Developer — pairing SSID+password.

**Ditolak/dideprioritaskan (sekunder, tidak dipakai sebagai bukti klaim):**
- Reddit (r/Roku, r/fireTV, r/Nest, r/XboxSupport, dsb.) — forum, hanya petunjuk arah riset.
- KB universitas (Andrews, Swarthmore, Norwich, TTU, Shebbear) — sekunder namun konsisten; dipakai sebagai konfirmasi silang, bukan sumber utama.
- Blog/Medium (yingtongli.me, umatechnology.org, trunetto.com) — bukan sumber primer.
- Manual aggregator (manualsdump/manualowl/manualsdir/manualslib) — reproduksi teks manual; canonical-nya adalah situs unduhan resmi vendor.
- Situs SEO/agregasi (find-your-support.com, gamerz-forum.com) — bukan sumber otoritatif.

## Langkah berikutnya (riset lanjutan paling berguna)

1. Uji lapangan LG webOS TV konsumen (2–3 model) terhadap SSID PEAP-MSCHAPv2 untuk menutup satu-satunya kategori "Tidak terverifikasi" pada perangkat utama.
2. Verifikasi kata-per-kata manual Samsung TV 2024 terbaru dari Samsung Download Center (memperkuat verdict konsumen).
3. Enumerasi printer yang umum dipakai pelanggan (termasuk Canon) untuk memastikan cakupan dukungan PEAP-MSCHAPv2.
