# Abstraksi payment gateway (provider ditunda)

Integrasi pembayaran online diabstraksikan di balik antarmuka `PaymentGateway`, sehingga provider konkret (Midtrans/Xendit/Duitku/Tripay/DOKU) menjadi detail implementasi yang dapat diganti tanpa mengubah domain. Provider MVP belum ditetapkan — alur inti (`TopUp` → `Payment` → kredit saldo → aktivasi VLAN) tidak bergantung pada provider.

Status: accepted

## Alasan

- Provider belum final saat desain; memilih sekarang akan memaksakan detail provider ke domain.
- Alur bisnis (top-up → `Balance` → aktivasi) identik untuk semua gateway; hanya mekanisme charge & webhook yang berbeda.

## Konsekuensi

- Webhook/notifikasi gateway dinormalisasi menjadi satu kontrak `Payment` di domain.
- MVP membutuhkan satu implementasi adapter nyata (dipilih belakangan) + adapter `mock` untuk pengembangan & test.
- `GraceAccess` **wajib bertenggat**: notifikasi sukses menyalakan layanan tanpa mengkredit `Balance`, tetapi hanya sampai tenggat yang ditetapkan saat notifikasi diterima. Tanpa tenggat, settlement yang hilang berarti layanan menyala selamanya — yaitu internet gratis. Rekonsiliasi settlement/reversal tetap berjalan terhadap `Payment`, bukan terhadap tenggat ini.
