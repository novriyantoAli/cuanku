# Riset: Provisi Kredensial `RadiusAccount` & Skema radcheck FreeRADIUS (WPA2-Enterprise, PEAP-MSCHAPv2)

Riset ini menjawab pertanyaan teknis: **bagaimana aplikasi Go menulis kredensial `RadiusAccount` ke FreeRADIUS sehingga EAP-PEAP/MSCHAPv2 berfungsi, dengan hanya menyimpan NT-hash (md4) dan tanpa cleartext?**

Pertanyaan ini melayani:
- **Issue #6** — "provisi + tulis radcheck (NT-hash)": model data `RadiusAccount`, atribut radcheck yang benar, cara menghasilkan NT-hash di Go, dan jalur tulis yang direkomendasikan.
- **Issue #7** — kebijakan VLAN: butuh jaminan bahwa `Tunnel-*` di radreply memang jalur yang benar (status ringkas di §10, detail di riset 0003).
- **Issue #10** — ganti kredensial self-service: latensi penerapan (§5) dan semantik ganti `username` (§6).
- **Issue #12** — `Session`/accounting: kolom radacct yang bisa dipakai (§7).
- **Issue #14** — risiko operasional dua penulis pada satu DB (§9).

Batasan keamanan yang sudah diputuskan di `docs/plans/0001-mvp-scope.md` (*"Kredensial WiFi: tidak pernah disimpan cleartext; NT-hash (MD4) diakui lemah/tanpa salt — risiko didokumentasikan"*) diperlakukan sebagai **premis**, bukan pertanyaan: riset ini tidak mencari alternatif penyimpanan cleartext, melainkan memverifikasi bahwa NT-hash-saja memang cukup secara teknis.

**Versi rujukan yang dipakai**: FreeRADIUS **3.0.x** (branch `v3.0.x`) dan **3.2.x** (branch `v3.2.x`) pada repo `github.com/FreeRADIUS/freeradius-server`; dokumentasi resmi **4.0.0** (untuk perbandingan 3.x vs 4.x); hostapd (mirror sumber upstream); OpenWrt **25.12** (stable saat ini) dan **24.10** (old stable). Setiap klaim diberi label **[TERVERIFIKASI]** (kode/dokumentasi primer yang dibaca langsung), **[INDIKATIF]** (hanya forum/mailing list/blog atau kutipan tidak langsung), **[TIDAK TERVERIFIKASI]** (tidak ditemukan bukti), dan ditandai **[Bukti langsung]** vs **[Inferensi peneliti]**.

## Ringkasan

Untuk EAP-PEAP/MSCHAPv2, FreeRADIUS **tidak butuh cleartext password**: server menyimpan **`NT-Password`** di `radcheck` (16 byte biner, ditulis sebagai 32 digit hex), dan modul `mschap` menurunkan seluruh data kriptografis yang diperlukan dari NT-hash tersebut. Modul `rlm_eap_mschapv2` sendiri **tidak melakukan kripto MSCHAPv2** — ia menyusun atribut `MS-CHAP-Challenge`/`MS-CHAP2-Response`/`MS-CHAP-User-Name` lalu memanggil bagian `Auth-Type MS-CHAP` (yaitu modul `mschap`) melalui `process_authenticate()` [TERVERIFIKASI, kode]. Di dalam `mschap`, *cleartext* hanya dipakai sebagai **fallback** ketika `NT-Password` tidak ada, dan kunci MPPE (`MS-MPPE-Recv-Key`/`MS-MPPE-Send-Key`) dihitung dari `nthashhash = MD4(NT-Password)` — bukan dari cleartext. Karena itu model data `RadiusAccount` **cukup menyimpan NT-hash**, asalkan `use_mppe` tidak dimatikan (default kode: `yes`) dan modul `pap` aktif di `authorize` untuk menormalisasi hex menjadi 16 byte biner.

Jalur tulis yang direkomendasikan adalah **(a) tulis langsung ke tabel SQL `radcheck`/`radreply`** dalam satu transaksi PostgreSQL (skema PostgreSQL resmi disediakan upstream di `raddb/mods-config/sql/main/postgresql/schema.sql`), bukan `rlm_rest` dan bukan berkas `users`. FreeRADIUS **tidak meng-cache** hasil authorize dari `radcheck`: query `authorize_check_query` dijalankan pada setiap Access-Request, sehingga kredensial baru langsung berlaku untuk autentikasi berikutnya; batas SLA "≤ 1 interval re-auth" sepenuhnya ditentukan oleh perilaku klien/`AccessPoint` (`eap_reauth_period` hostapd, default 3600 s; atau `Session-Timeout` dari radreply) dan oleh ketersediaan CoA/Disconnect (RFC 5176), bukan oleh FreeRADIUS. Target versi yang dianjurkan: **FreeRADIUS 3.2.x (3.2.10)** — selaras dengan paket `freeradius3` di OpenWrt 25.12 — bukan 3.0.x (EOL) dan bukan 4.x (experimental, dan dokumentasi resminya sendiri menganjurkan versi 3 untuk produksi).

## Tabel ringkasan temuan

| # | Topik | Verdict | Basis bukti |
|---|---|---|---|
| 1 | Atribut radcheck untuk PEAP-MSCHAPv2 | **`NT-Password`** (hex 32 digit) cukup dan yang benar. Konversi ke biner dilakukan modul `pap` (`normalise = yes`, default). `Cleartext-Password` hanya fallback; `Password-With-Header` (`{nt}`/`{nthash}`/`{md4}`) valid tetapi bukan bentuk kanonik di 3.x. Jangan pakai kombinasi. | Kode `rlm_mschap.c`, `rlm_pap.c`; dokumen 4.0 PAP; `mods-available/mschap` |
| 2 | Server hanya menyimpan NT-hash | **Bisa.** `nthashhash = MD4(NT-Password)` → `MasterKey = SHA1(nthashhash‖NTResponse‖magic1)` → kunci send/recv. Cleartext tidak diperlukan untuk MSCHAPv2 maupun MPPE. Tanpa kunci MPPE, sesi WPA2 gagal di hostapd. | Kode `rlm_mschap.c` (`do_mschap`, `mppe_chap2_gen_keys128`), `rlm_eap_mschapv2.c`, hostapd `ieee802_1x.c` |
| 3 | Menghasilkan NT-hash di Go | `MD4(UTF-16LE(password))`; pakai `golang.org/x/crypto/md4` + `unicode/utf16` + `encoding/binary`. Jebakan: UTF-8 vs UTF-16LE, karakter non-BMP (FreeRADIUS memakai konversi **UCS-2**, bukan UTF-16), dan kasus huruf pada hex. | Samba `E_md4hash`; FreeRADIUS `pap_auth_nt` + `fr_utf8_to_ucs2`; MS-NLMP §3.3.1 |
| 4 | Jalur tulis aplikasi Go | **Tulis langsung ke SQL** (skema PostgreSQL resmi ada di upstream). `rlm_rest` = opsi valid tapi menambah hop + tidak ada transaksi lintas-cek; berkas `users` tidak layak untuk skala per-`Customer`. | `schema.sql`, `queries.conf`, `mods-available/sql`, `mods-available/rest`, dokumen SQL 4.0 |
| 5 | Latensi penerapan | **Tidak ada cache authorize.** Berlaku pada Access-Request berikutnya. Batas nyata = `eap_reauth_period`/`Session-Timeout`/PMKSA caching/CoA. | `authorize_check_query` (queries.conf), `mods-available/sql`, hostapd `hostapd.conf` + `ieee802_1x.c` |
| 6 | Ganti `username` | `radcheck` berkunci `id` (serial) + indeks **non-unik** `(UserName, Attribute)` → aplikasi **wajib** menegakkan keunikan. Ubah aman = `UPDATE` dalam satu transaksi (bukan delete+insert). | `schema.sql` PostgreSQL (3.0.x), `queries.conf` |
| 7 | Skema radacct untuk #12 | `acctuniqueid` (UNIQUE), `acctsessionid`, `username`, `nasipaddress` (NOT NULL), `acctstarttime`, `acctupdatetime`, `acctstoptime`, `acctinterval`, `acctinput/octets`, `acctterminatecause`, `framedipaddress`, `callingstationid`, `class`. **VLAN (`Tunnel-*`) dan `Session-Timeout` tidak tersimpan** di radacct/radpostauth. | `schema.sql` PostgreSQL, `queries.conf` |
| 8 | Versi FreeRADIUS 2026 | **3.2.10** (3.2.x). 3.0.28 = "likely the last release of 3.0.x"; 4.x = experiment/devel, "you should use version 3". OpenWrt 25.12 & master: `freeradius3` **3.2.10**; OpenWrt 24.10: 3.2.8. | GitHub releases FreeRADIUS, docs 4.0 (releases/installation), Makefile `openwrt/packages` |
| 9 | Dua penulis pada satu DB | Aman secara MVCC PostgreSQL, tapi tanpa FK/constraint unik → risiko baris yatim & duplikat. Perlu grant sempit + transaksi pendek + indeks unik tambahan (opsional). | `schema.sql`, alasan DB (lihat §9; **[Inferensi peneliti]** untuk rekomendasi) |
| 10 | Atribut tambahan di radreply | **Ya** — model data harus mampu mematerialkan `Tunnel-Type`/`Tunnel-Medium-Type`/`Tunnel-Private-Group-Id` (dan opsi `Class`) untuk VLAN dinamis. Detail & verifikasi di riset 0003. | hostapd `ieee802_1x.c` (jalur Access-Accept → `ap_sta_bind_vlan`) |

## Detail per pertanyaan

### 1. Atribut radcheck yang benar-benar berfungsi untuk EAP-PEAP/MSCHAPv2

**Verdict: `NT-Password` dengan nilai hex 16 byte. Bukan kombinasi.**

Alur pembuktian (semuanya **[Bukti langsung]**, **[TERVERIFIKASI]**):

1. **`rlm_eap_mschapv2` delegasikan kripto ke modul `mschap`.** Setelah menyusun atribut request, kode memanggil:
   > `rcode = process_authenticate(inst->auth_type_mschap, request);`

   `inst->auth_type_mschap` di-resolve dari `dict_valbyname(PW_AUTH_TYPE, 0, "MSCHAP")` / `"MS-CHAP"`, dan modul gagal start bila bagian itu tidak ada:
   > `cf_log_err_cs(cs, "Failed to find 'Auth-Type MS-CHAP' section.  Cannot authenticate users.");`

   Bagian tersebut ada di site default:
   > `Auth-Type MS-CHAP { mschap }`
   Sumber: `src/modules/rlm_eap/types/rlm_eap_mschapv2/rlm_eap_mschapv2.c` (v3.0.x); `raddb/sites-available/default` (v3.0.x). *Implikasi: modul `mschap` wajib aktif di bagian `authenticate` — termasuk di virtual server inner-tunnel yang dipakai PEAP.*

2. **`mschap` menerima NT-hash ATAU cleartext, dengan NT-hash diprioritaskan.**
   ```c
   password    = fr_pair_find_by_num(request->config, PW_CLEARTEXT_PASSWORD, 0, TAG_ANY);
   nt_password = fr_pair_find_by_num(request->config, PW_NT_PASSWORD, 0, TAG_ANY);
   ...
   if (!nt_password) {
       if (password) {
           RDEBUG2("Found Cleartext-Password, hashing to create NT-Password");
           ...
           if (mschap_ntpwdhash(p, password->vp_strvalue) < 0) { ... }
   ```
   Sumber: `src/modules/rlm_mschap/rlm_mschap.c` (v3.0.x). Artinya: bila `NT-Password` ada, cleartext diabaikan sepenuhnya.

3. **Nilai hex dari SQL harus dinormalisasi lebih dulu oleh modul `pap`.** Jika tidak, `mschap` menolak diam-diam:
   ```c
   case 34:
   case 32:
       RWDEBUG("NT-Password has not been normalized by the 'pap' module (likely still in hex format).  "
               "Authentication may fail");
       nt_password = NULL;
       break;
   ```
   Normalisasi dilakukan `normify()` di `rlm_pap.c`, yang dijalankan bila `normalise` (default `"yes"`):
   ```c
   { "normalise", FR_CONF_OFFSET(PW_TYPE_BOOLEAN, rlm_pap_t, normify), "yes" },
   ```
   Sumber: `src/modules/rlm_pap/rlm_pap.c` (v3.0.x). *Konsekuensi operasional: modul `sql` dan `pap` harus aktif di `authorize` site default **dan** di virtual server inner-tunnel PEAP (lihat §5 dan Risiko).* **[Inferensi peneliti]** berdasarkan mekanisme di atas.

4. **`Password-With-Header` valid secara konsep, tetapi bukan bentuk kanonik di 3.x.** Tabel header dokumentasi resmi modul `pap` memuat `{nt}` / `{nthash}` / `{md4}` / `{x-nthash}` → **`Password.NT`**. Namun `rlm_pap.c` hanya memproses `PW_PASSWORD_WITH_HEADER` bila `Cleartext-Password` belum ada, dan menghapus atribut aslinya lalu menggantinya:
   > `if (fr_pair_find_by_num(request->config, PW_CLEARTEXT_PASSWORD, 0, TAG_ANY)) { RWDEBUG("Config already contains a \"known good\" password ..."); break; }`
   Sumber: `raddb/mods-available/pap`/kode `rlm_pap.c` (v3.0.x); dokumentasi 4.0 modul PAP. *Rekomendasi (untuk proyek ini): pakai `NT-Password` langsung — satu atribut, tanpa header, perilaku sama di 3.0.x–3.2.x.*

5. **Jangan pakai kombinasi.** Dokumentasi PAP 4.0 menyatakan eksplisit:
   > "Only one control attribute should be set, otherwise the behaviour is undefined as to which one is used for authentication."
   Sumber: dokumentasi resmi 4.0 `reference/raddb/mods-available/pap.html`.

6. Modul `mschap` juga tegas menyatakan sumber kredensial yang mungkin:
   > "MS-CHAP authentication requires access to either the Password.Cleartext or Password.NT attribute for the user. Due to the limitations of MS-CHAP, no other password 'encryption' methods are possible."
   Sumber: dokumentasi resmi 4.0 `reference/raddb/mods-available/mschap.html`.

**Contoh konkret baris `radcheck` (kasus PEAP-MSCHAPv2, hanya NT-hash):**

```sql
-- Satu baris, satu kredensial. op '=' , ':=' , atau '==' (default skema) — lihat catatan di bawah.
INSERT INTO radcheck (username, attribute, op, value)
VALUES ('budi01', 'NT-Password', ':=', '8846F7EAEE8FB117AD06BDD830B7586C');
```

Bentuk tabel:

| username | attribute | op | value |
|---|---|---|---|
| `budi01` | `NT-Password` | `:=` | `8846F7EAEE8FB117AD06BDD830B7586C` |

Nilai contoh di atas adalah NT-hash dari password `password` yang dipublikasikan sebagai nilai contoh umum (Hashcat example hashes) — **[INDIKATIF]**: nilai hash-nya wajib **diverifikasi lokal** (lihat §Langkah berikutnya), bukan dianggap bukti. Tiga hal tentang nilai/`op` yang perlu dicatat apa adanya:

- **Panjang & bentuk nilai.** `normify()` di `rlm_pap.c` memilih encoding secara otomatis: kandidat hex bila panjang **genap** dan **≥ 2×16 = 32** karakter dan dapat di-decode menjadi tepat 16 byte; jika tidak, dicoba base64; jika tidak, nilai dibiarkan. Karena itu 32 digit hex (tanpa prefiks) memenuhi kriteria hex. Komentar di kode juga menyatakan perilaku prefiks `0x` **berubah** antar versi:
  > "Earlier versions used a 0x prefix as a hard indicator that the string was hex encoded, and would fail if the 0x was present but the string didn't consist of hexits. ... That's why min_len (and decodability) are used as the only heuristics now."
  **[TERVERIFIKASI]** untuk mekanisme; **[TIDAK TERVERIFIKASI]** untuk pertanyaan spesifik "apakah `0x` + 32 hex masih diterima di 3.2.10" — saya tidak berhasil membaca implementasi `fr_hex2bin` pada pass ini. **Anjuran aman: tulis 32 digit hex tanpa prefiks `0x`, huruf besar** (konsisten dengan contoh-contoh FreeRADIUS), dan verifikasi lokal dengan `radtest`.
- **Huruf besar/kecil hex.** Tidak terverifikasi apakah `fr_hex2bin` menerima `a-f` dan `A-F`. Pakai huruf besar (bentuk yang dipublikasikan) dan verifikasi lokal.
- **`op`.** Skema PostgreSQL resmi memberi default `op VARCHAR(2) NOT NULL DEFAULT '=='` untuk `radcheck` (dan `'='` untuk `radreply`). Nilai manakah yang semantiknya tepat untuk atribut password di radcheck **belum saya verifikasi dari sumber primer** (dokumen SQL module menyatakan skema SQL "mirrors the functionality of the `files` module ... see the users file documentation"). Pakai `:=` (praktik umum) dan uji dengan `radtest`; jangan mengandalkan asumsi bahwa `==` dan `:=` identik. **[TIDAK TERVERIFIKASI]**

### 2. HANYA NT-hash: apa yang rusak, khususnya MPPE

**Verdict: hanya NT-hash tersimpan aman — dan wajib, karena tanpanya kunci MPPE tidak dibangkitkan.**

Bukti dari `src/modules/rlm_mschap/rlm_mschap.c` (v3.0.x), fungsi `do_mschap()` **[Bukti langsung]**:

```c
case AUTH_INTERNAL:
    /*
     *	No password: can't do authentication.
     */
    if (!password) {
        REDEBUG("FAILED: No NT-Password.  Cannot perform authentication");
        return -1;
    }

    smbdes_mschap(password->vp_octets, challenge, calculated);
    if (rad_digest_cmp(response, calculated, 24) != 0) { return -1; }

    /*
     *	If the password exists, and is an NT-Password,
     *	then calculate the hash of the NT hash.  Doing this
     *	here minimizes work for later.
     */
    if (!password->da->vendor &&
        (password->da->attr == PW_NT_PASSWORD)) {
        fr_md4_calc(nthashhash, password->vp_octets, MD4_DIGEST_LENGTH);
    }
    break;
```

- Verifikasi respons MSCHAPv2 hanya butuh NT-hash (`smbdes_mschap` atas NT-hash + challenge).
- `nthashhash` = `MD4(NT-hash)`, dihitung **khusus ketika** sumber password adalah `NT-Password`. Ini bukti eksplisit bahwa desain dimaksudkan untuk bekerja tanpa cleartext.

Kunci MPPE juga hanya dari `nthashhash` **[Bukti langsung]**:

```c
static void mppe_chap2_get_keys128(uint8_t const *nt_hashhash, uint8_t const *nt_response,
                                   uint8_t *sendkey, uint8_t *recvkey)
{
       uint8_t masterkey[16];
       mppe_GetMasterKey(nt_hashhash,nt_response,masterkey);
       mppe_GetAsymmetricStartKey(masterkey,sendkey,16,1);
       mppe_GetAsymmetricStartKey(masterkey,recvkey,16,0);
}
```
dengan `mppe_GetMasterKey()` = `SHA1(nthashhash ‖ nt_response(24 byte) ‖ magic1)`.

Pemuatan kunci ke reply **[Bukti langsung]**:

```c
	/* now create MPPE attributes */
	if (inst->use_mppe) {
		...
		} else if (mschap_version == 2) {
			RDEBUG2("Adding MS-CHAPv2 MPPE keys");
			mppe_chap2_gen_keys128(nthashhash, response->vp_octets + 26, mppe_sendkey, mppe_recvkey);
			mppe_add_reply(request, "MS-MPPE-Recv-Key", mppe_recvkey, 16);
			mppe_add_reply(request, "MS-MPPE-Send-Key", mppe_sendkey, 16);
		}
		pair_make_reply("MS-MPPE-Encryption-Policy", ...);
		pair_make_reply("MS-MPPE-Encryption-Types", ...);
	}
```

**Apa yang rusak bila kunci MPPE tidak ada** — ini di sisi `AccessPoint` (hostapd) **[Bukti langsung]**:

```c
static void ieee802_1x_get_keys(struct hostapd_data *hapd, struct sta_info *sta, ...)
{
	keys = radius_msg_get_ms_keys(msg, req, shared_secret, shared_secret_len);
	if (keys && keys->send && keys->recv) {
		len = keys->send_len + keys->recv_len;
		...
		os_memcpy(sm->eap_if->aaaEapKeyData, keys->recv, keys->recv_len);
		os_memcpy(sm->eap_if->aaaEapKeyData + keys->recv_len, keys->send, keys->send_len);
		sm->eap_if->aaaEapKeyAvailable = true;
	} else {
		wpa_printf(MSG_DEBUG, "MS-MPPE: 1x_get_keys, could not get keys: %p send: %p recv: %p", ...);
	}
```
Sumber: hostapd `src/ap/ieee802_1x.c`. **[Inferensi peneliti]**: tanpa `MS-MPPE-Recv-Key`/`Send-Key` di Access-Accept, `aaaEapKeyAvailable` tidak pernah di-set → tidak ada material kunci bagi hostapd → handshake WPA2 tidak dapat diselesaikan. Artinya: **`use_mppe` TIDAK BOLEH dimatikan** untuk skenario ini.

**Default `use_mppe` — ada kontradiksi dokumentasi vs kode.** Kode 3.0.x/3.2.x menetapkan default `yes`:
```c
static const CONF_PARSER module_config[] = {
	/*
	 * Cache the password by default.
	 */
	{ "use_mppe", FR_CONF_OFFSET(PW_TYPE_BOOLEAN, rlm_mschap_t, use_mppe), "yes" },
```
Sumber: `src/modules/rlm_mschap/rlm_mschap.c` (v3.0.x); file `raddb/mods-available/mschap` hanya mengomentari baris `# use_mppe = no` dan berkomentar *"If use_mppe is not set to no mschap, will add MS-CHAP-MPPE-Keys ... and MS-MPPE-Recv-Key/MS-MPPE-Send-Key for MS-CHAPv2"* → konsisten dengan default **aktif**. Sedangkan dokumentasi HTML resmi (networkradius/FreeRADIUS 3.0.20) menyebut `use_mppe` **Default: no**. **Kesimpulan: percayai kode (default `yes`) dan jangan menyentuh direktif ini**; verifikasi lokal dengan `radiusd -X` sambil mencari baris log `Adding MS-CHAPv2 MPPE keys`. **[TERVERIFIKASI]** untuk isi kode & kontradiksi; **[Inferensi peneliti]** untuk "kode menang".

**Apakah radreply perlu diisi?** Tidak untuk `MS-MPPE-*`. Atribut itu dibangun per-sesi oleh modul (`pair_make_reply`), dan `rlm_eap_mschapv2.c` secara eksplisit **memindahkan** kunci MPPE keluar dari reply inner lalu mengembalikannya pada saat EAP Success:
> `/* Delete MPPE keys & encryption policy. We don't want these here. */ fix_mppe_keys(handler, data);`
> `fr_pair_list_mcopy_by_num(request->reply, &request->reply->vps, &data->mppe_keys, 0, 0, TAG_ANY);`
Sumber: `src/modules/rlm_eap/types/rlm_eap_mschapv2/rlm_eap_mschapv2.c`. **[Inferensi peneliti]**: menulis `MS-MPPE-Recv-Key`/`Send-Key` statis ke `radreply` akan menghasilkan kunci yang salah/berbahaya dan tidak diperlukan.

**Hal yang memang hilang bila cleartext tidak disimpan** (perlu diketahui, bukan blocker) **[TERVERIFIKASI, kecuali ditandai lain]**:
- Autentikasi PAP terhadap akun yang sama **tetap bekerja** — `pap_auth_nt()` membandingkan `MD4` dari `User-Password` dengan NT-Password tersimpan, jadi `radtest` (PAP) bisa dipakai sebagai alat uji lokal untuk memverifikasi hash. Sumber: `rlm_pap.c` (`pap_auth_nt`), `sites-available/default` (`Auth-Type PAP { pap }`).
- Fitur ganti-password via MS-CHAPv2 yang mengekspos cleartext (`MS-CHAP-New-Cleartext-Password`) **tidak layak dipercaya** untuk non-ASCII; dokumentasi proyek sendiri menyatakan: *"cleartext passwords have undergone unicode transformation ... in a very ad-hoc way. ... when the server reads Password.Cleartext out of files/database, it assumes US-ASCII and thus international characters will fail."* Sumber: wiki proyek `MS-CHAP` (freeradius.org/documentation/.../howto/modules/mschap). *Untuk issue #10, tulis ulang NT-hash dari aplikasi, jangan mengandalkan fitur change-password FreeRADIUS.*
- MS-CHAPv1/LM tidak dipakai (dan LM-hash legacy akan tidak tersedia) — tidak relevan untuk WPA2-Enterprise.

### 3. Menghasilkan NT-hash yang benar di Go

**Definisi algoritma** (dikonfirmasi dari dua implementasi acuan):

- Samba — `libcli/auth/smbencrypt.c`:
  > `/** Creates the MD4 Hash of the users password in NT UNICODE. */`
  > `bool E_md4hash(const char *passwd, uint8_t p16[16]) { ... ret = push_ucs2_talloc(NULL, &wpwd, passwd, &len); ... len -= 2; mdfour(p16, (const uint8_t *)wpwd, len); ... }`
  **[Bukti langsung]**. Perhatikan detail penting: `len -= 2` → **terminator UCS-2 tidak diikutkan** dalam MD4.
- FreeRADIUS — `rlm_pap.c` → `pap_auth_nt()`:
  ```c
  len = fr_utf8_to_ucs2(ucs2_password, sizeof(ucs2_password), request->password->vp_strvalue, request->password->vp_length);
  fr_md4_calc(digest, (uint8_t *) ucs2_password, len);
  ```
  **[Bukti langsung]**. `fr_utf8_to_ucs2()` didefinisikan di `src/lib/misc.c` dengan komentar *"Convert UTF8 string to UCS2 encoding ... Borrowed from src/crypto/ms_funcs.c of wpa_supplicant project"* (**[INDIKATIF]**: saya hanya melihat komentar/signature lewat hasil pencarian dokumentasi doxygen, bukan membaca badan fungsinya).
- Spesifikasi: MS-NLMP §3.3.1 mendefinisikan `NTOWFv1` = MD4 atas password Unicode (**[INDIKATIF]**: URL resmi `learn.microsoft.com/en-us/openspecs/windows_protocols/ms-nlmp/464551a8-9fc4-428e-b3d3-bc5bfb2e73a5` tidak berhasil di-fetch pada pass ini; formula-nya konsisten dengan dua implementasi acuan di atas).

**Implementasi Go yang direkomendasikan (mandiri, tanpa dependensi NTLM pihak ketiga):**

```go
import (
	"encoding/hex"
	"strings"
	"unicode/utf16"

	"golang.org/x/crypto/md4"
)

// NTHash mengembalikan 16 byte NT-hash = MD4(UTF-16LE(password)).
func NTHash(password string) []byte {
	u := utf16.Encode([]rune(password)) // UTF-16 code units
	buf := make([]byte, 2*len(u))
	for i, c := range u {
		buf[2*i] = byte(c)        // little-endian: low byte dulu
		buf[2*i+1] = byte(c >> 8)
	}
	h := md4.New()
	h.Write(buf)
	return h.Sum(nil)
}

// Nilai untuk kolom radcheck.value: 32 digit hex huruf besar.
func NTHashHex(password string) string {
	return strings.ToUpper(hex.EncodeToString(NTHash(password)))
}
```

Catatan paket: `golang.org/x/crypto/md4` menyediakan `md4.New()` (dan mendokumentasikan sendiri bahwa MD4 rusak secara kriptografis dan hanya dipakai untuk kompatibilitas legacy — persis kasus kita). Sumber: pkg.go.dev/golang.org/x/crypto/md4. **[TERVERIFIKASI]**

**Paket Go "siap pakai" untuk NT-hash — hasil penelusuran:**
- `github.com/Azure/go-ntlmssp` (v0.1.1) **tidak mengekspor** fungsi NTOWFv1/NT-hash; API publiknya hanya `NewAuthenticateMessage`, `NewNegotiateMessage`, `Negotiator`, dan opsi `AuthenticateMessageOptions.PasswordHashed` (menerima hash yang **sudah** dihitung, dalam format hex). Sumber: pkg.go.dev/github.com/Azure/go-ntlmssp. **[TERVERIFIKASI]** → tidak cocok sebagai *penghasil* hash.
- Pustaka lain (`github.com/bodgit/ntlmssp`, `github.com/staaldraad/go-ntlm`, contoh di `blackhat-go/bhg`) memiliki fungsi `ntowfV1`/`Ntowfv1`, tetapi sebagian di antaranya **tidak diekspor** dan semuanya adalah pustaka untuk keperluan serangan/klien SMB — **[INDIKATIF]** (hanya dari isi kode yang terlihat di hasil pencarian, bukan dari halaman API resminya). *Anjuran: implementasi sendiri 12 baris di atas; permukaan dependensi lebih kecil dan tidak ada risiko menyalahgunakan lib ofensif.* **[Inferensi peneliti]**

**Jebakan encoding yang wajib diantisipasi** (semuanya berdampak langsung ke issue #6/#10):

1. **UTF-16LE, bukan UTF-8, dan tanpa NUL terminator.** MD4 atas `[]byte(password)` (UTF-8) menghasilkan hash yang salah secara senyap → customer tidak bisa login tanpa error yang jelas di sisi FreeRADIUS.
2. **Karakter non-BMP (mis. emoji, sebagian CJK ekstensi).** FreeRADIUS memakai konversi **UCS-2** (2 byte/karakter, "borrowed from wpa_supplicant ms_funcs.c") sementara Go/`utf16.Encode` menghasilkan **surrogate pair** (4 byte di UTF-16LE) — seperti Windows. Untuk password dengan karakter > U+FFFF, hash FreeRADIUS dan hash Go **akan berbeda**. **[Inferensi peneliti]** dari nama/komentar fungsi `fr_utf8_to_ucs2` + pernyataan resmi proyek bahwa penanganan unicode "ad-hoc". **Mitigasi yang direkomendasikan**: validasi password `RadiusAccount` pada rentang karakter aman (mis. ASCII/Latin-1 tercetak + simbol), tolak karakter non-BMP dengan pesan jelas, dan uji kasus non-ASCII terhadap FreeRADIUS nyata. **[Inferensi peneliti]**
3. **Normalisasi Unicode.** Password yang tampak sama tetapi berbeda bentuk (NFC vs NFD) menghasilkan hash berbeda. Tidak ada bukti FreeRADIUS menormalkan; terapkan kebijakan normalisasi aplikasi + dokumentasikan. **[TIDAK TERVERIFIKASI]** apakah klien supplicant melakukan normalisasi apa pun.
4. **Hex huruf besar/kecil & prefiks `0x`** — lihat §1 (butir "Panjang & bentuk nilai"). **[TIDAK TERVERIFIKASI]**

### 4. Jalur tulis yang direkomendasikan untuk aplikasi Go

| Jalur | Atomisitas | Validasi | Dukungan FreeRADIUS | Penilaian |
|---|---|---|---|---|
| **(a) Tulis langsung ke tabel SQL `radcheck`/`radreply`** | Penuh (transaksi PostgreSQL) | Aplikasi (DB tidak punya constraint domain) | Skema resmi disediakan upstream; query authorize resmi tersedia | **Direkomendasikan** |
| (b) API aplikasi + `rlm_rest` | Transaksi di sisi aplikasi; cek & tulis terpisah di sisi RADIUS | Aplikasi | Modul resmi, konfigurasi per-bagian | Layak, tapi menambah hop & kompleksitas; tidak menghapus kebutuhan API aplikasi |
| (c) Berkas `users` (modul `files`) | Rewrite berkas (non-transaksional di aplikasi) | Aplikasi | Modul resmi | Tidak layak untuk ribuan `RadiusAccount` per-`Customer` |

**Skema resmi & dukungan PostgreSQL** **[TERVERIFIKASI]**:
- Skema PostgreSQL **disediakan resmi** upstream: `raddb/mods-config/sql/main/postgresql/schema.sql` (+ `queries.conf`) di repo FreeRADIUS. Daftar dialek pada `raddb/mods-available/sql` (3.0.x): `mssql, mysql, oracle, postgresql, sqlite, mongo` (mongo "experimental"); dokumentasi 4.0 menambah `cassandra`, `firebird` dan menghapus `mongo`. Dokumentasi 4.0 juga mencatat: *"Not all drivers ship with query.conf or schema.sql files. For those which don't, please create them and contribute them back to the project."* → **PostgreSQL termasuk yang disediakan** (bukan kasus "tidak didukung").
- Di OpenWrt, driver PostgreSQL dipaketkan terpisah: `freeradius3-mod-sql-postgresql` (`DEPENDS:=freeradius3-mod-sql +libpq`) — jadi **didukung dan tersedia** di feed. Sumber: `openwrt/packages` `net/freeradius3/Makefile`. **[TERVERIFIKASI]**
- Tabel yang relevan (kutipan verbatim skema PostgreSQL 3.0.x):
  ```sql
  CREATE TABLE IF NOT EXISTS radcheck (
  	id			serial PRIMARY KEY,
  	UserName		text NOT NULL DEFAULT '',
  	Attribute		text NOT NULL DEFAULT '',
  	op			VARCHAR(2) NOT NULL DEFAULT '==',
  	Value			text NOT NULL DEFAULT ''
  );
  create index radcheck_UserName on radcheck (UserName,Attribute);
  ```
  ```sql
  CREATE TABLE IF NOT EXISTS radreply (
  	id			serial PRIMARY KEY,
  	UserName		text NOT NULL DEFAULT '',
  	Attribute		text NOT NULL DEFAULT '',
  	op			VARCHAR(2) NOT NULL DEFAULT '=',
  	Value			text NOT NULL DEFAULT ''
  );
  create index radreply_UserName on radreply (UserName,Attribute);
  ```
  Catatan: komentar skema juga menyediakan varian indeks untuk query case-insensitive (`lower(UserName)`), yang *dinonaktifkan* secara default.

**`rlm_rest` — detail yang terverifikasi** (sebagai perbandingan, bukan rekomendasi) **[Bukti langsung]**, sumber `raddb/mods-available/rest` (v3.0.x):
- Bagian di modul ini "reflect the sections in the server": `authorize`, `authenticate`, `preacct`, `accounting`, `post-auth`, dst.
- Opsi per bagian: `uri`, `method` (`get|post|put|patch|delete`), `body` (`none`|`post`|`json`), `attr_num`, `raw_value`, `data`.
- Pemetaan kode HTTP → return code modul bersifat formal, mis. untuk authorize: `404 → notfound`, `410 → notfound`, `403 → userlock`, `401 → reject`, `204 → ok`, `2xx → ok/updated`, `5xx → fail`.
- **[Inferensi peneliti]**: karena `rlm_rest` adalah *klien* HTTP, "tulis ke radcheck" tetap dilakukan oleh API aplikasi Anda; `rlm_rest` hanya menggantikan pembacaan SQL dengan pemanggilan API. Keuntungannya (aplikasi menjadi satu-satunya penulis DB) harus dibayar dengan: latensi tambahan pada setiap Access-Request, tidak ada transaksi DB bersama, dan ketergantungan FreeRADIUS pada ketersediaan API internal saat autentikasi (titik gagal baru pada jalur kritis).

**Berkas `users`/modul `files`** **[Inferensi peneliti] + [TIDAK TERVERIFIKASI]**: saya tidak memverifikasi semantik reload (`HUP`) maupun atomisitas penulisan berkas `users` pada pass ini. Untuk kebutuhan "ganti kredensial self-service ≤ 2 menit" (kriteria T1 di `0001-mvp-scope.md`), satu berkas monolitik tanpa indeks per-`Customer` dan tanpa transaksi bukan pilihan yang aman; jangan dipilih. **Rekomendasi tegas: opsi (a).**

**Ringkas 3.x vs 4.x untuk jalur tulis**: di 3.x, `NT-Password` (hex → dinormalisasi `pap`) adalah bentuk kanonik dan didukung penuh; di 4.x dokumentasi lebih mendorong `Password-With-Header` dan menegaskan bahwa skema SQL "mirrors the functionality of the `files` module". Kalau proyek memilih 3.2.x (§8), desain tabel tetap valid dan portabel ke 4.x selama kita menyimpan `NT-Password` (bukan header) dan menghindari ketergantungan pada perilaku order-of-attributes. **[Inferensi peneliti]**

### 5. Latensi penerapan: apakah FreeRADIUS meng-cache hasil authorize?

**Verdict: tidak ada cache hasil authorize pada 3.x; perubahan berlaku pada Access-Request berikutnya. Batas SLA ditentukan oleh re-auth di sisi `AccessPoint`/klien, plus CoA.**

**[Bukti langsung]** Query authorize resmi (tidak ada cache, dijalankan per-request):
```
authorize_check_query = "\
	SELECT id, UserName, Attribute, Value, Op \
	FROM ${authcheck_table} \
	WHERE Username = '%{SQL-User-Name}' \
	ORDER BY id"
authorize_reply_query = "\
	SELECT id, UserName, Attribute, Value, Op \
	FROM ${authreply_table} \
	WHERE Username = '%{SQL-User-Name}' \
	ORDER BY id"
```
Sumber: `raddb/mods-config/sql/main/postgresql/queries.conf` (v3.0.x; `${authcheck_table}` = `radcheck`, `${authreply_table}` = `radreply`). Varian case-insensitive ada tetapi **dikomentari** ("WARNING: Slower queries!").

**[Bukti langsung]** Direktif di modul `sql` 3.0.x yang relevan dengan caching/efisiensi: `read_groups` (default `yes`), `read_profiles` (default `yes`), `read_clients` ("Clients will ONLY be read on server startup"), `delete_stale_sessions = yes`, `query_timeout`, dan `pool { ... }`. **Tidak ada** direktif `cache` untuk hasil authorize. Sumber: `raddb/mods-available/sql` (v3.0.x). Di 4.0 ditambahkan `cache_groups` — *"whether or not we cache the list of SQL groups ... Default is `no`"* — yaitu cache keanggotaan grup saja, **bukan** `radcheck`. Sumber: dokumentasi SQL 4.0.

**[Bukti langsung] Batas nyata "interval re-auth" ada di sisi `AccessPoint`:**

1. **`eap_reauth_period`** — `hostapd/hostapd.conf`:
   > `# EAP reauthentication period in seconds (default: 3600 seconds; 0 = disable # reauthentication).`
   Artinya default satu jam.
2. **`Session-Timeout` dari Access-Accept** — hostapd `src/ap/ieee802_1x.c`:
   ```c
   session_timeout_set = !radius_msg_get_attr_int32(msg, RADIUS_ATTR_SESSION_TIMEOUT, &session_timeout);
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
   **[Inferensi peneliti]**: inilah lever paling presisi untuk janji SLA. Mengirim `Session-Timeout` (+ `Termination-Action = RADIUS-Request`, RFC 3580) membuat AP me-*re-auth* tepat pada interval itu, sehingga "kredensial baru berlaku ≤ 1 interval re-auth" menjadi sifat yang bisa dikonfigurasi, bukan harapan.
3. **PMKSA caching / `okc` / pre-authentication** dapat melewati EAP penuh saat klien berpindah/associate ulang (`disable_pmksa_caching` ada di hostapd.conf) **— [Bukti langsung untuk keberadaan opsi; Inferensi peneliti untuk dampaknya terhadap SLA]**: cache PMK dapat membuat klien lama tetap terhubung tanpa autentikasi ulang.
4. **CoA/Disconnect (RFC 5176)** — jalur untuk efek segera pada `AccessPoint` terjangkau (sesuai kriteria T1 di `0001-mvp-scope.md`: "Disconnect untuk AccessPoint terjangkau; ≤ 30 menit untuk AccessPoint di balik NAT"). FreeRADIUS 3.2.9 menambahkan `Error-Cause` pada CoA-NAK/Disconnect-NAK. **[TERVERIFIKASI]** dari release notes. *Implementasi pengiriman Disconnect dari aplikasi Go (paket Go RADIUS, shared-secret per-`AccessPoint`) belum diriset di sini.*

**Implikasi konkret untuk issue #10 (dan #6)**: cukup `UPDATE radcheck SET value=... WHERE username=... AND attribute='NT-Password'` lalu commit; Access-Request berikutnya langsung memakai kredensial baru. Untuk memenuhi SLA ≤ 30 menit di AP di balik NAT, pasang `Session-Timeout` ≈ 1800 detik (atau `eap_reauth_period` di AP) sebagai kebijakan, dan dokumentasikan konsekuensinya (frekuensi re-auth meningkat). **Angka ini keputusan desain, bukan temuan riset — perlu uji lokal.**

### 6. Semantik ganti `username`

**[Bukti langsung]** Kunci tabel (skema PostgreSQL resmi):
```sql
CREATE TABLE IF NOT EXISTS radcheck (
	id			serial PRIMARY KEY,
	UserName		text NOT NULL DEFAULT '',
	Attribute		text NOT NULL DEFAULT '',
	op			VARCHAR(2) NOT NULL DEFAULT '==',
	Value			text NOT NULL DEFAULT ''
);
create index radcheck_UserName on radcheck (UserName,Attribute);
```
- Kunci baris = `id` (serial). `UserName` **hanya indeks non-unik** `(UserName, Attribute)` — **tidak ada UNIQUE constraint** untuk `(UserName, Attribute)` di skema PostgreSQL resmi. (Skema MySQL 2.x/3.x juga: `PRIMARY KEY (id)`, `KEY username (username(32))`.) → **Aplikasi wajib menegakkan keunikan `username` + `attribute='NT-Password'` sendiri.**
- `authorize_check_query` memakai perbandingan **case-sensitive** (`WHERE Username = '%{SQL-User-Name}'`); varian `LOWER()` ada tetapi dikomentari. → tetapkan kebijakan kanonikalisasi `username` di aplikasi (mis. trim + casefold ke huruf kecil) dan simpan sudah ternormalisasi, karena `AccessPoint` akan mengirim apa pun yang diketik pengguna. **[Inferensi peneliti]** — perlu uji lokal karena beberapa supplicant melakukan normalisasi sendiri (di kode EAP-MSCHAPv2 FreeRADIUS bahkan ada peringatan ketidakcocokan `User-Name` vs `MS-CHAP-Name`).

**Langkah aman mengubah username** **[Inferensi peneliti, berdasarkan skema & query di atas]**:

```sql
BEGIN;
-- 1) Ambil baris milik akun lama dan pastikan jumlahnya sesuai harapan (hindari salah sasaran).
SELECT id, attribute, op, value FROM radcheck WHERE username = $old FOR UPDATE;
-- 2) Pastikan username baru belum dipakai.
SELECT 1 FROM radcheck WHERE username = $new LIMIT 1;   -- harus kosong
-- 3) Ubah kepemilikan baris (mempertahankan `id`, sehingga urutan ORDER BY id tidak berubah).
UPDATE radcheck SET username = $new WHERE username = $old;
UPDATE radreply SET username = $new WHERE username = $old;
-- 4) (bila dipakai) radusergroup username mapping.
UPDATE radusergroup SET username = $new WHERE username = $old;
COMMIT;
```

- **`UPDATE` lebih diutamakan daripada `DELETE`+`INSERT`**: (i) satu transaksi tetap atomik untuk keduanya, tetapi `UPDATE` tidak pernah meninggalkan akun tanpa baris bila proses terputus di tengah rencana, dan (ii) `ORDER BY id` pada query authorize berarti urutan baris memengaruhi presedensi atribut — `UPDATE` mempertahankan `id`.
- **Baris yatim**: karena tidak ada foreign key, `radcheck`/`radreply`/`radusergroup` dapat "yatim" bila aplikasi menghapus `RadiusAccount` tanpa membersihkan ketiganya. Aturan: hapus/tulis ketiga tabel dalam **satu transaksi**. `radacct`/`radpostauth` **jangan** diubah (arsip historis; `username` di sana adalah jejak audit).
- **Sesi yang sedang berjalan** tidak terpengaruh oleh rename: kunci accounting adalah `AcctUniqueId` (`text NOT NULL UNIQUE`), bukan `username`. Jadi rename tidak memutus sesi aktif; efeknya baru terasa pada autentikasi berikutnya (§5). **[Inferensi peneliti]** dari skema + query.
- **Opsional (hardening, menyimpang dari skema resmi)**: tambahkan indeks unik untuk nilai yang secara semantik tunggal, mis. `CREATE UNIQUE INDEX radcheck_ntpwd_uniq ON radcheck (UserName) WHERE Attribute = 'NT-Password';`. Ini adalah keputusan desain; pastikan tidak mematahkan atribut multi-nilai lain yang (mungkin) ditambahkan untuk #12. **[Inferensi peneliti]**
- **Migrasi identitas**: kalau `username` `RadiusAccount` dipakai juga sebagai referensi di tabel aplikasi (opsional: tabel milik aplikasi sendiri, mis. `radius_account`), lakukan rename di DB aplikasi pada transaksi yang sama atau lewat tabel pemetaan `(radius_account_id, username)` agar tidak ada dua sumber kebenaran. **[Inferensi peneliti]**

### 7. Skema accounting (radacct) untuk issue #12

**[Bukti langsung]** — skema PostgreSQL resmi (`raddb/mods-config/sql/main/postgresql/schema.sql`, v3.0.x):

```sql
CREATE TABLE IF NOT EXISTS radacct (
	RadAcctId		bigserial PRIMARY KEY,
	AcctSessionId		text NOT NULL,
	AcctUniqueId		text NOT NULL UNIQUE,
	UserName		text,
	Realm			text,
	NASIPAddress		inet NOT NULL,
	NASPortId		text,
	NASPortType		text,
	AcctStartTime		timestamp with time zone,
	AcctUpdateTime		timestamp with time zone,
	AcctStopTime		timestamp with time zone,
	AcctInterval		bigint,
	AcctSessionTime		bigint,
	AcctAuthentic		text,
	ConnectInfo_start	text,
	ConnectInfo_stop	text,
	AcctInputOctets		bigint,
	AcctOutputOctets	bigint,
	CalledStationId		text,
	CallingStationId	text,
	AcctTerminateCause	text,
	ServiceType		text,
	FramedProtocol		text,
	FramedIPAddress		inet,
	FramedIPv6Address	inet,
	FramedIPv6Prefix	inet,
	FramedInterfaceId	text,
	DelegatedIPv6Prefix	inet,
	Class			text
);

CREATE INDEX radacct_active_session_idx ON radacct (AcctUniqueId) WHERE AcctStopTime IS NULL;
CREATE INDEX radacct_bulk_close ON radacct (NASIPAddress, AcctStartTime) WHERE AcctStopTime IS NULL;
CREATE INDEX radacct_start_user_idx ON radacct (AcctStartTime, UserName);
CREATE INDEX radacct_calss_idx ON radacct (Class);   -- (ejaan asli upstream)
```

Catatan kolom yang penting untuk `Session` (issue #12):
- `AcctUniqueId` = **kunci alami** yang dipakai semua UPDATE (start/interim/stop). UNIQUE + ada partial index khusus sesi aktif (`WHERE AcctStopTime IS NULL`) — pola "sesi aktif" = `AcctStopTime IS NULL`.
- `AcctInterval` **bukan** "interval interim yang dikonfigurasi", melainkan **selisih waktu sejak update terakhir**:
  ```sql
  AcctInterval = (${....event_timestamp_epoch} - EXTRACT(EPOCH FROM (COALESCE(AcctUpdateTime, AcctStartTime))))
  ```
  **[Bukti langsung]** dari `interim-update { -query = ... }` di `queries.conf`. → Untuk aturan *stale session* "2× interval interim" di `0001-mvp-scope.md` (yang defaultnya bilang 5 menit), pakai **`AcctUpdateTime`**, bukan `AcctInterval`. **[Inferensi peneliti]** — kalau interim hilang, `AcctInterval` justru membesar; membandingkannya dengan 2× interval akan salah.
- `start` memakai `INSERT ... ON CONFLICT (AcctUniqueId) DO UPDATE ...` dan `interim-update`/`stop` memakai `UPDATE ... WHERE AcctUniqueId = '...' AND AcctStopTime IS NULL` → duplikat paket start tidak membuat baris ganda, dan stop bersifat idempoten. **[Bukti langsung]**
- `AcctInputOctets`/`AcctOutputOctets` dihitung sebagai `(Gigawords << 32) + Octets` → sudah 64-bit. **[Bukti langsung]**
- `Class` tersedia (dan diisi lewat `class.column_name`/`packet_xlat` yang di-generate dari policy `insert_acct_class`). Release 3.2.9 menyebut *"Amend policy insert_acct_class/acct_unique to work in environments with multiple Class attributes"* → `Class` memang jalur resmi untuk menandai sesi. **[Bukti langsung]**

**Apakah atribut VLAN (`Tunnel-*`) dan `Session-Timeout` tampil di radacct/radpostauth? — Tidak.**

- `column_list` akunting di `queries.conf` **tidak memuat satu pun kolom Tunnel-\*** maupun Session-Timeout (lihat daftar `AcctSessionId, AcctUniqueId, UserName, Realm, NASIPAddress, NASPortId, NASPortType, AcctStartTime, AcctUpdateTime, AcctStopTime, AcctSessionTime, AcctAuthentic, ConnectInfo_start, ConnectInfo_Stop, AcctInputOctets, AcctOutputOctets, CalledStationId, CallingStationId, AcctTerminateCause, ServiceType, FramedProtocol, FramedIpAddress, FramedIpv6Address, FramedIpv6Prefix, FramedInterfaceId, DelegatedIpv6Prefix`). **[Bukti langsung]**
- `radpostauth` hanya menyimpan: `username, pass, reply, CalledStationId, CallingStationId, authdate, Class`, dengan
  ```sql
  VALUES('%{User-Name}', '%{%{User-Password}:-%{Chap-Password}}', '%{reply:Packet-Type}', '%S.%M' ${..class.reply_xlat})
  ```
  → kolom `reply` berisi `Access-Accept`/`Access-Reject` (tipe paket), **bukan** VLAN. **[Bukti langsung]**
- Jalur yang **tersedia** untuk mengaitkan VLAN/`SubscriptionState` ke `Session`: atribut RADIUS **`Class`**. hostapd menyimpan `Class` dari Access-Accept
  > `static void ieee802_1x_store_radius_class(...) { if (!hapd->conf->radius->acct_server || !hapd->radius || !sm) return; ... }`
  dan menyediakannya kembali lewat `ieee802_1x_get_radius_class()` untuk dipakai pada paket akunting (`ap/accounting.c`). **[TERVERIFIKASI]** untuk penyimpanan + getter (kode dibaca); **[Inferensi peneliti]** untuk pemakaian di sisi accounting (saya tidak membaca `ap/accounting.c` pada pass ini). Karena hostapd menyimpan `Class` **hanya jika `acct_server` dikonfigurasi**, jalur ini gratis untuk setup kita. **[Inferensi peneliti]**
- **Kesimpulan praktis untuk #12**: (i) dukung `Vlan`/`SubscriptionState` per `Session` lewat `Class` (mis. `Class = "vlan=123;state=active"`) sehingga nilainya ikut terbawa ke `radacct.Class`; atau (ii) jangan bergantung pada DB RADIUS untuk VLAN — simpan `Vlan` pada `Session` di DB aplikasi dan rekonsiliasi lewat `AcctUniqueId` + waktu. Opsi (i) lebih murah karena berjalan di jalur yang sudah ada (need verifikasi lokal end-to-end dengan hostapd nyata).

### 8. Versi FreeRADIUS yang ditargetkan pada 2026

**Verdict: FreeRADIUS 3.2.x — pakai 3.2.10. Bukan 3.0.x, bukan 4.0.x.**

**[Bukti langsung]** GitHub Releases `FreeRADIUS/freeradius-server` (rilis teratas saat riset):
> `## 3.2.10` … `## 3.0.28 — This is likely the last release of 3.0.x - everyone should migrate to 3.2.9 or later.` … `## 3.2.9`

**[Bukti langsung]** Dokumentasi resmi 4.0:
- *Release Management*: rilik v4 dibagi dua alur — minor **genap** = stabil/dukung, minor **ganjil** = eksperimental/feature; *"Experimental versions must not be packaged by OS vendors."*
- *Install and Upgrade (4.0)*: *"These instructions cover installing FreeRADIUS 4.x, which is still in heavy development. Other than exceptional circumstances, you should use version 3."*
- Paket 4.0 dari Network RADIUS: *"the upcoming version that is still in development ... we highly recommend only using these packages for testing purposes only."*

**[Bukti langsung]** Paket OpenWrt (`openwrt/packages`, `net/freeradius3/Makefile`):
| Branch | `PKG_VERSION` | Catatan |
|---|---|---|
| `openwrt-25.12` (stable saat ini) | **3.2.10**, `PKG_RELEASE:=2` | selaras dengan rilis FreeRADIUS terbaru |
| `master` | **3.2.10**, `PKG_RELEASE:=2` | — |
| `openwrt-24.10` (old stable) | **3.2.8**, `PKG_RELEASE:=1` | versi lebih lama |

**[Bukti langsung]** Status rilis OpenWrt (downloads.openwrt.org): stable saat ini **25.12** (25.12.5, 2026-06-29); **24.10** adalah "old stable" (24.10.8, 2026-07-24).

**[Bukti langsung]** Paket plugin OpenWrt yang relevan tersedia: `freeradius3-mod-sql`, `freeradius3-mod-sql-postgresql` (`+libpq`), `freeradius3-mod-rest`, `freeradius3-mod-cache*`, `freeradius3-mod-mschap`/`freeradius3-mod-pap` (bagian dari daftar plugin), dsb.

**Rekomendasi tegas (beserta alasan)** **[Inferensi peneliti]**:
1. **Gunakan FreeRADIUS 3.2.x (3.2.10)** di atas **OpenWrt 25.12** untuk lingkungan `AccessPoint` bila FreeRADIUS dijalankan on-device; sebaliknya, untuk server RADIUS terpusat (arsitektur ADR-0001 + transport tunnel/radsecproxy di `0001-mvp-scope.md`), pakai 3.2.10 dari paket distro/dari sumber.
2. **Alasan menolak 3.0.x**: rilis 3.0.28 secara eksplisit dinyatakan "likely the last release of 3.0.x"; memulai proyek baru di atas cabang EOL berarti tidak ada perbaikan keamanan (termasuk kelas serangan BlastRADIUS yang mitigasinya dirilis di 3.2.5).
3. **Alasan menolak 4.x**: (a) dokumentasi resmi proyek sendiri mengarahkan produksi ke versi 3; (b) kebijakan rilis menyatakan versi eksperimental "must not be packaged by OS vendors", sehingga distribusi ke `AccessPoint` OpenWrt menjadi rumit; (c) model konfigurasi dan atribut kredensial bergeser (`Password-With-Header`, struktur `unlang`) → desain provisi di dokumen ini harus ditinjau ulang; (d) 4.x bukan target paket OpenWrt 25.12.
4. **Konsekuensi desain**: dokumen ini dan skema tabelnya merujuk `radcheck`/`radreply`/`radacct` 3.x + `NT-Password` (hex). Bila kelak pindah ke 4.x, `NT-Password` tetap didukung, tetapi mekanisme normalisasi (`pap normalise`) dan pemetaan `Password-With-Header` perlu diverifikasi ulang di versi target. **[Inferensi peneliti]**

### 9. Risiko operasional: dua penulis (aplikasi + FreeRADIUS) pada DB yang sama

**[Inferensi peneliti]**, dengan basis mekanisme yang terverifikasi (skema tanpa FK/UNIQUE, query per-request):

- **Baca-sambil-tulis**: `radcheck` dibaca dengan `SELECT ... WHERE Username = ...` pada setiap Access-Request. Di PostgreSQL (MVCC) pembacaan tidak memblokir penulisan dan sebaliknya; penulis aplikasi tidak akan memblokir autentikasi selama transaksinya pendek. Risiko nyata bukan *locking*, melainkan **snapshot per-statement**: FreeRADIUS menjalankan `authorize_check_query` dan `authorize_reply_query` sebagai dua statement terpisah, sehingga secara teori sebuah request bisa melihat `radcheck` versi lama dan `radreply` versi baru. **[Inferensi peneliti]** → desain tabel harus "tahan campuran": jangan pernah punya state antar-tabel yang membuat kombinasi lama/baru berbahaya, dan selalu jaga tepat satu baris `NT-Password` yang valid.
- **Tanpa constraint unik** pada `(UserName, Attribute)` → duplikat baris sangat mungkin bila aplikasi melakukan insert tanpa cek. Akibat: perilaku `op`/urutan (`ORDER BY id`) menentukan nilai mana yang "menang" — sulit didiagnosis. Mitigasi: invariant aplikasi (satu baris NT-Password per `username`) + (opsional) indeks unik parsial.
- **Tanpa FK/CASCADE** → baris yatim pada penghapusan `RadiusAccount`. Mitigasi: hapus dalam satu transaksi lintas `radcheck`/`radreply`/`radusergroup`.
- **Beban akunting** menulis `UPDATE radacct` berfrekuensi tinggi (`WHERE AcctUniqueId = ... AND AcctStopTime IS NULL`), plus `INSERT ... ON CONFLICT` pada start. Ini menghasilkan *row churn* pada `radacct`; bagi `radcheck`/`radreply` (volume sangat kecil) beban ini tidak relevan, tetapi **jangan** menaruh data aplikasi pada `radacct` dan jangan melakukan query analitik berat pada DB yang sama tanpa replika. **[Inferensi peneliti]**
- **`delete_stale_sessions = yes` (default 3.x)** dapat menambah pekerjaan pada jalur authorize (cek login ganda). Bila `simul_*`/checkrad tidak dipakai, pertimbangkan `delete_stale_sessions = no` untuk mengurangi query di jalur kritis — **perlu verifikasi lokal** bahwa tidak ada fitur lain yang bergantung padanya. **[TERVERIFIKASI]** untuk keberadaan direktif & default; **[Inferensi peneliti]** untuk rekomendasi.
- **Hak akses**: beri peran DB aplikasi grant **hanya** pada `radcheck`, `radreply`, (`radusergroup` bila dipakai); FreeRADIUS memakai peran terpisah dengan grant `SELECT` pada `radcheck`/`radreply` + tulis pada `radacct`/`radpostauth`. Ini mengurangi dampak bug aplikasi terhadap data akunting. **[Inferensi peneliti]**
- **Jangan** memakai koneksi/transaksi panjang atau `LOCK TABLE`; dan hindari `TRUNCATE`/`ALTER` pada tabel yang dibaca FreeRADIUS (DDL memblokir pembaca). **[Inferensi peneliti]**

### 10. Kebutuhan atribut tambahan di radreply (status ringkas — detail di riset 0003)

**Status: ya, model data `RadiusAccount` harus mampu mematerialkan atribut reply VLAN.**

- Jalur Access-Accept hostapd memang memanggil penanganan VLAN: pada blok pemrosesan Access-Accept di `src/ap/ieee802_1x.c` terdapat `ap_sta_bind_vlan(hapd, sta);` (di sekitar penanganan `Session-Timeout`). **[Bukti langsung]** untuk pemanggilan; **[INDIKATIF]** untuk pernyataan bahwa `ap_sta_bind_vlan()` mengambil `Tunnel-Type`/`Tunnel-Medium-Type`/`Tunnel-Private-Group-Id` + `Egress-VLAN-*` (saya tidak membaca fungsi itu pada pass ini). Detail lengkap → **riset 0003**.
- Karena itu, selain `NT-Password` di `radcheck`, model data harus menyediakan materialisasi baris `radreply` (atau `radgroupreply` + `radusergroup` bila memakai grup) untuk atribut seperti `Tunnel-Type`, `Tunnel-Medium-Type`, `Tunnel-Private-Group-Id` — yang per ADR-0001 nilainya ditentukan oleh `SubscriptionState` dan `Vlan` (`RadiusAccount` aktif → VLAN unik per `Customer`; suspended → satu VLAN suspended bersama).
- Satu tambahan yang berbiaya rendah dan berdampak besar untuk issue #12: baris `radreply` untuk **`Class`** (terbawa ke `radacct.Class` tempat hostapd menyimpannya, §7) dan/atau **`Session-Timeout`** (+ `Termination-Action`) untuk mengendalikan interval re-auth (§5).
- **Belum diputuskan di sini**: apakah `Vlan`/`SubscriptionState` dinyatakan lewat `radreply` per-`RadiusAccount` (baris per akun) atau lewat `radgroupreply` + `radusergroup` (baris per grup, lebih hemat untuk VLAN yang dibagi antar-`RadiusAccount` milik satu `Customer`). Konsekuensi tulis-baca, dan risiko `read_groups`/`Fall-Through`, dibahas di riset 0003.

## Risiko & ketidakpastian

1. **`use_mppe` default kontradiktif di dokumentasi.** Kode 3.0.x/3.2.x = `yes`; dokumentasi HTML networkradius/3.0 menyatakan default `no`. Jika evaluator/konfigurasi lain mengikuti dokumentasi dan menuliskan `use_mppe = no`, `MS-MPPE-Recv-Key`/`Send-Key` tidak akan dikirim → **semua sesi WPA2 akan gagal** tanpa error yang jelas di sisi FreeRADIUS (hanya debug hostapd "MS-MPPE: 1x_get_keys, could not get keys"). **Mitigasi**: jalankan `radiusd -X`, cari `Adding MS-CHAPv2 MPPE keys`, dan jadikan ini bagian dari smoke test deployment.
2. **Modul `pap`/`sql` harus aktif di inner-tunnel PEAP.** Bukti kuat untuk `sql` + `pap` di authorize site default ada (kode + dokumentasi); untuk virtual server `inner-tunnel` saya baru menemukan pernyataan dokumentasi proyek (*"edit /etc/raddb/sites-available/inner-tunnel dan uncomment baris yang berisi 'sql' di bawah `authorize {}`"*, wiki SQL-HOWTO) — **belum saya verifikasi dari berkas `raddb/sites-available/inner-tunnel`**. Ini titik gagal paling sering untuk setup SQL+PEAP: bila normalisasi `pap` tidak berjalan di jalur inner, `NT-Password` tetap dalam bentuk hex dan `mschap` mengembalikan `invalid`.
3. **Perilaku prefiks `0x` dan kasus huruf pada hex tidak terverifikasi** untuk 3.2.10. Sumber kode menyatakan heuristik encoding *berubah* antar versi. Mitigasi: tulis 32 hex huruf besar tanpa `0x`, lalu uji dengan `radtest`.
4. **Semantik `op` di radcheck** (`==` default skema vs `:=`) tidak terverifikasi dari sumber primer untuk atribut password. Mitigasi: uji kedua nilai dengan `radtest` pada staging.
5. **Divergensi Unicode.** FreeRADIUS memakai konversi UTF-8→**UCS-2** (2 byte/karakter) untuk NT-hash, sedangkan Go/Windows memakai **UTF-16 dengan surrogate pair**. Password dengan karakter di luar BMP berpotensi menghasilkan hash berbeda → customer tidak bisa login. Mitigasi: batasi charset password (validasi server-side) dan tulis test case non-ASCII.
6. **`Session-Timeout` vs `eap_reauth_period` tidak sinkron.** Kalau AP mengirim/memakai `Session-Timeout` yang lebih panjang daripada `eap_reauth_period` (atau sebaliknya), SLA "≤ 1 interval re-auth" tidak dapat diprediksi. Butuh uji lapangan pada `AccessPoint` OpenWrt nyata (dan ingat PMKSA caching dapat menyembunyikan perilaku sebenarnya).
7. **VLAN per `Session` tidak tersedia langsung dari DB RADIUS.** Rencana "accounting yang mencatat VLAN/`SubscriptionState` per sesi" (`0001-mvp-scope.md`) tidak bisa dipenuhi oleh `radacct` bawaan; harus lewat `Class` atau melalui tabel aplikasi. Risiko: bila `Class` tidak terbawa (mis. `acct_server` tidak dikonfigurasi di AP, atau `Class` lebih dari satu), data VLAN per `Session` hilang. **Butuh uji lapangan.**
8. **Skala/kinerja DB bersama.** Semua keputusan di sini mengasumsikan satu PostgreSQL dengan beban moderat. Bila populasi `Customer` tumbuh besar (interim 5 menit × banyak sesi), `radacct` menjadi tabel panas dan strategi retensi/partisi harus ditinjau (di luar lingkup dokumen ini).
9. **Perubahan username vs sesi aktif.** Rename tidak memutus sesi (kunci `AcctUniqueId`), tetapi billing usage-based menyimpan `username` di `radacct`. Rekonsiliasi pasca-rename harus memakai `AcctUniqueId`/waktu, bukan `username`. **[Inferensi peneliti]** — perlu dipastikan bahwa `Session` di aplikasi memang dikunci oleh `AcctUniqueId`, bukan `username`.
10. **Tidak ada pengujian hardware/lingkungan pada pass ini.** Semua bukti adalah kode/dokumentasi upstream; tidak ada verifikasi end-to-end pada FreeRADIUS 3.2.10 nyata, hostapd nyata, atau PostgreSQL nyata. Lihat "Langkah berikutnya".

## Sumber

**Sumber primer yang dibaca/diperlakukan sebagai bukti:**
1. FreeRADIUS `freeradius-server`, branch `v3.0.x` — `src/modules/rlm_mschap/rlm_mschap.c` — https://github.com/FreeRADIUS/freeradius-server/blob/v3.0.x/src/modules/rlm_mschap/rlm_mschap.c — sumber utama §1/§2 (pemilihan `NT-Password`, `nthashhash`, MPPE, `use_mppe` default, `Auth-Type MS-CHAP`).
2. FreeRADIUS `v3.0.x` — `src/modules/rlm_eap/types/rlm_eap_mschapv2/rlm_eap_mschapv2.c` — https://github.com/FreeRADIUS/freeradius-server/blob/v3.0.x/src/modules/rlm_eap/types/rlm_eap_mschapv2/rlm_eap_mschapv2.c — bukti bahwa EAP-MSCHAPv2 mendelegasikan ke `Auth-Type MS-CHAP` dan memindahkan kunci MPPE.
3. FreeRADIUS `v3.0.x` — `src/modules/rlm_pap/rlm_pap.c` — https://github.com/FreeRADIUS/freeradius-server/blob/v3.0.x/src/modules/rlm_pap/rlm_pap.c — `normalise` default `yes`, `normify()` (hex/base64/biner), `pap_auth_nt()` (UCS-2 + MD4).
4. FreeRADIUS `v3.0.x` — `raddb/mods-config/sql/main/postgresql/schema.sql` — https://github.com/FreeRADIUS/freeradius-server/blob/v3.0.x/raddb/mods-config/sql/main/postgresql/schema.sql — skema resmi PostgreSQL `radcheck`/`radreply`/`radacct`/`radpostauth`/`nas`.
5. FreeRADIUS `v3.0.x` — `raddb/mods-config/sql/main/postgresql/queries.conf` — https://github.com/FreeRADIUS/freeradius-server/blob/v3.0.x/raddb/mods-config/sql/main/postgresql/queries.conf — `authorize_check_query`/`authorize_reply_query`, semantik start/interim/stop, `AcctInterval`, `postauth_query`.
6. FreeRADIUS `v3.0.x` — `raddb/mods-available/sql` — https://github.com/FreeRADIUS/freeradius-server/blob/v3.0.x/raddb/mods-available/sql — daftar dialek, `read_groups`/`read_profiles`/`read_clients`/`delete_stale_sessions`, pool (tidak ada cache authorize).
7. FreeRADIUS `v3.0.x`/`v3.2.x` — `raddb/mods-available/mschap` — https://github.com/FreeRADIUS/freeradius-server/blob/v3.2.x/raddb/mods-available/mschap — komentar `use_mppe`, peringatan hanya pakai MS-CHAP di dalam tunnel, contoh `local_cpw` ke `radcheck` (`attribute='NT-Password'`).
8. FreeRADIUS `v3.0.x` — `raddb/sites-available/default` — https://github.com/FreeRADIUS/freeradius-server/blob/v3.0.x/raddb/sites-available/default — `Auth-Type MS-CHAP { mschap }` dan `Auth-Type PAP { pap }`.
9. FreeRADIUS `v3.0.x` — `raddb/mods-available/rest` — https://github.com/FreeRADIUS/freeradius-server/blob/v3.0.x/raddb/mods-available/rest — pemetaan kode HTTP → return code, opsi per-bagian (basis perbandingan §4).
10. Dokumentasi resmi FreeRADIUS 4.0 — modul `mschap` (https://www.freeradius.org/documentation/freeradius-server/4.0.0/reference/raddb/mods-available/mschap.html), modul `pap` header table (…/pap.html), modul `sql` (`cache_groups`, dialek, catatan schema.sql tidak untuk semua driver), how-to MS-CHAP (`…/howto/modules/mschap/index.html` — dua mode kredensial, peringatan Unicode cleartext), *Release Management* (`…/releases.html`), *Install and Upgrade* ("you should use version 3").
11. Wiki proyek FreeRADIUS — `guide/SQL-HOWTO` (https://wiki.freeradius.org/guide/SQL-HOWTO) dan `guide/SQL-HOWTO-for-freeradius-3.x-on-Debian-Ubuntu` — instruksi mengaktifkan `sql` di `sites-available/inner-tunnel`; `MS-CHAP` how-to (password change, `local_cpw`, peringatan Unicode).
12. Man page FreeRADIUS — `rlm_mschap(5)`/`radcrypt(8)` — https://www.freeradius.org/radiusd/man/rlm_mschap.html , https://www.freeradius.org/radiusd/man/radcrypt.html — `rlm_mschap` menerima cleartext atau NT-Password; `radcrypt` **hanya DES/MD5** (tidak bisa menghasilkan NT-hash).
13. GitHub Releases `FreeRADIUS/freeradius-server` — https://github.com/FreeRADIUS/freeradius-server/releases — 3.2.10 terbaru; 3.0.28 "likely the last release of 3.0.x".
14. hostapd (mirror sumber upstream) — `src/ap/ieee802_1x.c` — https://github.com/nikescar/hostapd-mirror/blob/b01c4843/src/ap/ieee802_1x.c — `ieee802_1x_get_keys()` (wajib `MS-MPPE-Recv-Key`+`Send-Key`), `Session-Timeout`/`Termination-Action` → `reAuthPeriod`, `ieee802_1x_store_radius_class()` + `ieee802_1x_get_radius_class()`, `ap_sta_bind_vlan()`.
15. hostapd — `hostapd/hostapd.conf` (mirror yang sama) — `eap_reauth_period` default 3600 s; `disable_pmksa_caching`.
16. Samba — `libcli/auth/smbencrypt.c` — https://github.com/samba-team/samba/blob/master/libcli/auth/smbencrypt.c — `E_md4hash()`: "MD4 Hash of the users password in NT UNICODE", terminator UCS-2 tidak diikutkan.
17. OpenWrt packages — `net/freeradius3/Makefile`, branch `openwrt-25.12` dan `master` (3.2.10) serta `openwrt-24.10` (3.2.8) — https://github.com/openwrt/packages/blob/openwrt-25.12/net/freeradius3/Makefile — daftar subpaket `freeradius3-mod-sql`, `-mod-sql-postgresql`, `-mod-rest`, `-mod-cache*`.
18. OpenWrt — halaman Downloads/Releases (https://downloads.openwrt.org/, https://openwrt.org/releases/25.12/start) — 25.12.5 stable (2026-06-29), 24.10.8 old stable (2026-07-24).
19. Go — `golang.org/x/crypto/md4` (https://pkg.go.dev/golang.org/x/crypto/md4) dan `github.com/Azure/go-ntlmssp` (https://pkg.go.dev/github.com/Azure/go-ntlmssp) — `md4.New()`; go-ntlmssp tidak mengekspor NTOWFv1.
20. Network RADIUS — dokumentasi `mschap` (`use_mppe` default `no` di dokumen) dan halaman paket 4.0 ("testing purposes only") — https://networkradius.org/doc/current/raddb/mods-available/mschap.html , https://networkradius.com/packages/4.0/ — dipakai sebagai pihak yang dikontraskan pada §2/§8.

**Sumber sekunder/langsung-tidak-lengkap — dipakai hanya sebagai penunjuk, bukan bukti klaim:**
21. MS-NLMP §3.3.1 (NTOWFv1) — https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-nlmp/464551a8-9fc4-428e-b3d3-bc5bfb2e73a5 — **[INDIKATIF]**: halaman tidak berhasil di-fetch pada pass ini; formula NT-hash didasarkan pada dua implementasi acuan (Samba #16, FreeRADIUS #3).
22. Doxygen/dokumentasi mirror FreeRADIUS untuk `fr_utf8_to_ucs2` (`src/lib/misc.c`, `src/include/libradius.h`) — diperoleh lewat cuplikan hasil pencarian (doc.freeradius.org, fossies) — **[INDIKATIF]**.
23. Mailing list `freeradius-users` (thread "Inserting NT-Passwords in MySQL database", "Creating an NT-Password value with python") — **[INDIKATIF]**: hanya dipakai untuk mengonfirmasi bahwa pertanyaan ini sering muncul; **bukan** bukti klaim teknis. Salah satu balasan menyarankan `smbpasswd` untuk membuat NT-hash — saya **tidak memverifikasi** apakah utilitas itu ada di paket FreeRADIUS/OpenWrt.

**Ditolak/dideprioritaskan:** blog & tutorial pihak ketiga tentang "insert NT-Password ke MySQL" (tanpa versi/rujukan), jawaban Stack Overflow/ServerFault (tidak ada sitasi kode), halaman agregator konfigurasi (memuat konfigurasi FreeRADIUS 2.x yang sudah tidak relevan), repositori kode pihak ketiga yang menduplikasi NTLM tanpa keterangan versi (`blackhat-go/bhg`, `staaldraad/go-ntlm`) — dipakai hanya sebagai petunjuk bahwa fungsi NTOWFv1 sering tidak diekspor.

## Langkah berikutnya (riset/verifikasi paling berguna)

1. **Uji end-to-end lokal FreeRADIUS 3.2.10 + PostgreSQL + hostapd/eapol_test, hanya dengan NT-hash.** Bangun satu baris `radcheck` lewat jalur tulis aplikasi, lalu: (a) `radtest` (PAP) untuk memvalidasi nilai hash (menguji `pap_auth_nt` tanpa perlu EAP); (b) `eapol_test -c peap-mschapv2.conf` untuk PEAP-MSCHAPv2 penuh; (c) di `radiusd -X` pastikan muncul `Found NT-Password` dan `Adding MS-CHAPv2 MPPE keys`, **bukan** `NT-Password has not been normalized by the 'pap' module`. Sekaligus jawab pertanyaan terbuka: prefiks `0x`, kasus huruf hex, dan `op` (`==` vs `:=`).
2. **Verifikasi jalur inner-tunnel PEAP** dengan membaca `raddb/sites-available/inner-tunnel` pada 3.2.10 dan memastikan `sql`+`pap` aktif di `authorize`, `mschap` di `authorize`/`authenticate`, dan bahwa tidak ada `files` yang "mencuri" hasil lebih dulu.
3. **Uji Unicode NT-hash**: bandingkan hasil `NTHashHex()` (Go) dengan `%{mschap:NT-Hash <password>}` di FreeRADIUS untuk set password uji (ASCII, Latin-1 (mis. `ü`), karakter non-BMP) → konfirmasi hipotesis divergensi UCS-2 vs UTF-16, dan tetapkan charset password yang diizinkan untuk `RadiusAccount`.
4. **Uji lapangan SLA & VLAN pada `AccessPoint` OpenWrt**: (a) ukur kapan kredensial baru benar-benar berlaku dengan `eap_reauth_period` default vs `Session-Timeout`+`Termination-Action` di radreply; (b) uji apakah `Class` dari Access-Accept muncul di `radacct.Class` (butuh `acct_server`); (c) catat apakah PMKSA caching membuat klien tetap online melewati interval. Hasil ini menutup celah untuk issue #10 dan #12.
