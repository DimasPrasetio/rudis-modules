# Roadmap modul rudis-modules

Acuan pengembangan pustaka: **50 modul baru** yang akan dibangun, di luar 21 modul dan 1 preset
yang sudah rilis. Tujuannya memperkaya modul reusable agar aplikasi apa pun yang diminta pengguna
nanti — ERP, HRMIS, gudang, POS, buku tamu, booking, helpdesk, website — bisa dirakit dari modul
yang sudah ada, dan Rudis-SDD hanya membuat bagian yang benar-benar khusus proyek.

Daftar ini disusun dari riset internet (produk yang paling banyak dipakai dan regulasi Indonesia;
sumber di [Rujukan](#rujukan)) dan dari arsitektur yang terkunci di [AGENTS.md](AGENTS.md).

**Ini juga checkpoint.** Saat sebuah modul dirilis (tag `<id>/v<versi>` ada di `main`), ubah
statusnya di tabel dan tulis versinya. Urutan pengerjaan mengikuti gelombang: ambil dari gelombang
paling awal yang masih terbuka.

| Status | Arti |
|---|---|
| ⬜ | belum dimulai |
| 🟡 | sedang dikerjakan — rencananya ada di `docs/plans/` dan tercatat di STATUS.md › In progress |
| ✅ | dirilis — tulis versinya, mis. `✅ 0.1.0` |

## Aturan yang berlaku untuk setiap modul di daftar ini

Diringkas dari [AGENTS.md](AGENTS.md) dan [docs/MODULE_FORMAT.md](docs/MODULE_FORMAT.md):

- **Tidak ada modul yang mengimpor modul lain** (`no-module-imports`). Kebutuhan dari luar ditulis
  sebagai **port** milik modul itu sendiri; yang ditawarkan ditulis sebagai **kontrak**; keduanya
  disambung hanya di composition root lewat adaptor. Kolom *Port wajib* dan *Port opsional* di
  bawah adalah port milik modul itu — penyedia yang disebut dalam kurung hanyalah adaptor yang
  disiapkan, bukan import.
- **Port opsional lebih diutamakan.** Port wajib hanya bila modul tidak bermakna tanpanya; port
  opsional yang tidak disambung berarti fiturnya mati, modulnya tetap jalan.
- **Kode bersama tanpa tabel = kit Go** (`gokit`: hanya standard library, `go_dependencies`, dan
  kit Go lain); **UI generik = kit UI** (`ui`, `kits-first`). Kit dibangun lebih dulu di
  gelombangnya.
- **Tabel milik sendiri** (`own-tables-only`): relasi ke data modul lain hanya berupa ID, tanpa
  foreign key atau join; modul multi-perusahaan memakai `"scoped": true` (`company_id` di setiap
  tabel dan query).
- **Angka pasti** (`exact-numbers`): uang `DECIMAL(19,2)`, kuantitas `DECIMAL(19,4)`, tarif
  `DECIMAL(19,10)` lewat kit `money`.
- **Event** dideklarasikan di `emits` dengan nama `<id>.<benda>.<kata-kerja-lampau>` (`events-declared`).
- **Tarif dan aturan regulasi disimpan sebagai data bertanggal berlaku**, bukan konstanta di kode
  (TER, PTKP, iuran BPJS, PPN, kode wilayah, hari libur berubah tiap tahun).
- **Batas kernel 6**: container Alpine tanpa CGO, memori 512 MB, filesystem read-only kecuali
  `DATA_DIR` — PDF dibuat murni Go, tanpa Chromium; pustaka yang butuh CGO tidak boleh.
- Setiap modul tetap **dimulai sebagai rencana** di `docs/plans/` yang disetujui maintainer
  ([docs/BUILDING_MODULES.md](docs/BUILDING_MODULES.md)), lalu diuji di testbed sebelum rilis.

Fakta regulasi di kolom Catatan diberi tanda **[V]** terverifikasi di sumber resmi, **[S]** dari
sumber sekunder, **[U]** belum terverifikasi. Semua wajib dicek ulang ke teks resmi saat rencana
modulnya ditulis.

## Modul yang sudah ada

| Status | Modul | Jenis | Menyediakan |
|---|---|---|---|
| ✅ 0.7.0 | `auth` | fullstack | login email + sandi, sesi server |
| ✅ 0.7.0 | `users` | fullstack | kontrak `UserClient` |
| ✅ 0.6.0 | `rbac` | fullstack | peran & izin per perusahaan/cabang |
| ✅ 0.1.0 | `organization` | fullstack | `OrganizationClient`; event `organization.company.created` |
| ✅ 0.3.0 | `settings` | fullstack | `SettingsClient` (profil perusahaan, NPWP, logo) |
| ✅ 0.2.0 | `audit-log` | fullstack | jejak audit `d.Audit` |
| ✅ 0.2.0 | `files` | fullstack | `FileClient` (unggah ke `DATA_DIR`) |
| ✅ 0.2.0 | `mailer` | fullstack | `MailClient` (antrean email) |
| ✅ 0.1.0 | `numbering` | fullstack | `NumberingClient` (nomor dokumen) |
| ✅ 0.1.0 | `period` | fullstack | `PeriodClient` (tutup buku per area) |
| ✅ 0.1.0 | `customers` | fullstack | `CustomerClient` (identitas pajak Coretax) |
| ✅ 0.1.0 | `suppliers` | fullstack | `SupplierClient` |
| ✅ 0.1.0 | `money` | gokit | Rupiah, kuantitas, tarif presisi |
| ✅ 0.1.0 | `docstate` | gokit | draft → posted → cancelled → amended |
| ✅ 0.1.0 | `taxid` | gokit | NPWP 16 digit, NIK |
| ✅ 0.5.0 | `app-shell` | web | bingkai halaman, sidebar dari nav modul |
| ✅ 0.4.0 | `ui-core` | ui | primitif & layout Bingkai Navy |
| ✅ 0.2.0 | `ui-data` | ui | DataTable |
| ✅ 0.2.0 | `ui-forms` | ui | field, pencarian, alamat |
| ✅ 0.1.0 | `ui-feedback` | ui | toast, dialog konfirmasi |
| ✅ 0.1.1 | `ui-gallery` | web | galeri komponen kit |
| ✅ 0.1.1 | `bingkai-navy` | preset | tema back-office |

## Ringkasan 50 modul baru

| Gelombang | Fokus | Jumlah | Selesai |
|---|---|---|---|
| 1 | Kit dasar | 6 | 0 |
| 2 | Platform lintas bidang | 12 | 0 |
| 3 | Layanan & publik | 7 | 0 |
| 4 | Penjualan & pembelian | 7 | 0 |
| 5 | Gudang & POS | 7 | 0 |
| 6 | Keuangan & pajak | 6 | 0 |
| 7 | SDM | 5 | 0 |
| | **Total** | **50** | **0** |

Gelombang 3 sengaja sebelum ERP: modul layanan memakai paling sedikit port dan paling cepat
dipakai di aplikasi yang jenisnya belum diketahui.

## Gelombang 1 — Kit dasar

Tanpa tabel, tanpa port. Dipakai oleh banyak modul di gelombang berikutnya.

| # | Status | Modul | Jenis | Fungsi | Catatan | Rujukan |
|---|---|---|---|---|---|---|
| 1 | ⬜ | `fieldcrypt` | gokit | Enkripsi kolom data pribadi (AES-GCM, rotasi kunci, indeks buta untuk cari NIK, masking) | UU PDP mewajibkan langkah keamanan teknis [S]; kunci dari `config` | S30 |
| 2 | ⬜ | `pdfdoc` | gokit | Bangun PDF murni Go: kop surat, tabel, nomor halaman, QR, format tanggal & angka Indonesia, terbilang | Tanpa Chromium (batas container) | S42 |
| 3 | ⬜ | `barcode` | gokit | Buat & cek QR dan barcode 1D (EAN-13, Code128), GS1 lot/kedaluwarsa, token QR bertanda tangan HMAC berbatas waktu | Dekode kamera di browser, bukan di server (tanpa CGO) | S10 |
| 4 | ⬜ | `geo` | gokit | Jarak haversine, titik dalam poligon, geofence radius | Untuk absensi GPS dan check-in lokasi | S4 |
| 5 | ⬜ | `ui-print` | ui | Cetak dari browser: tata letak cetak A4, struk thermal 58/80 mm (ESC/POS lewat WebUSB/Bluetooth/Serial), fallback `window.print` | Server tidak bisa menjangkau printer toko | S9, S36 |
| 6 | ⬜ | `ui-scan` | ui | Input scanner barcode (keyboard wedge), scan kamera (BarcodeDetector), scan-ke-field | Dipakai gudang, POS, buku tamu | S9 |

## Gelombang 2 — Platform lintas bidang

Blok yang dipakai hampir semua jenis aplikasi.

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 7 | ⬜ | `notifications` | fullstack | Lonceng & kotak masuk, preferensi kanal per pengguna, satu router ke email/WhatsApp | `UserDirectory` (→ users) | `ChannelSender` (→ mailer, whatsapp); kontrak `Notifier` · emits `notifications.message.sent` | | S1, S31 |
| 8 | ⬜ | `whatsapp` | fullstack | Kirim & terima WhatsApp (Cloud API / BSP): registri templat per kategori, opt-in, webhook masuk, status kirim, jendela 24 jam | — | kontrak `ChannelSender` · emits `whatsapp.message.received` | Harga per pesan sejak 1 Jul 2025; pesan utility dalam jendela berbayar mulai 1 Okt 2026 [V] | S20 |
| 9 | ⬜ | `approvals` | fullstack | Persetujuan bertingkat untuk dokumen modul lain: ambang nilai, peran, delegasi, SLA, kotak masuk | — | `ApproverResolver` (→ rbac, employees), `Notifier`; kontrak `Approvals` · emits `approvals.request.approved` / `rejected` | Status dokumen tetap milik modul pemiliknya (`docstate`) | S1, S5, S41 |
| 10 | ⬜ | `data-io` | fullstack | Impor/ekspor CSV & XLSX di latar belakang: pemetaan kolom, pratinjau validasi, laporan galat | `FileStore` (→ files) | `Notifier`; registri `ImportTarget`/`ExportSource` dari tiap modul · emits `data-io.job.completed` | File sementara di `DATA_DIR` | S6, S9 |
| 11 | ⬜ | `comments` | fullstack | Komentar berutas, @mention, linimasa aktivitas, lampiran pada record modul mana pun | — | `FileStore`, `Notifier`, `RecordAccess` · emits `comments.comment.created` | Izin mengikuti record induk | S33 |
| 12 | ⬜ | `work-calendar` | fullstack | Libur nasional & cuti bersama per tahun, kalender perusahaan, pola kerja 5/6 hari, jam kerja, hitung hari kerja | — | `Units` (→ organization, zona WIB/WITA/WIT); kontrak `WorkCalendar` | Ditetapkan SKB 3 Menteri tiap tahun [V ada; jumlah hari S] | S28 |
| 13 | ⬜ | `id-regions` | fullstack | Provinsi → kab/kota → kecamatan → desa/kelurahan dengan kode Kemendagri dan kode pos, versi dataset per keputusan, pemilih bertingkat | — | kontrak `RegionLookup` | Kepmendagri 300.2.2-2138/2025 (diubah 2430/2025) [S]; kode desa 10 digit [S] | S29 |
| 14 | ⬜ | `custom-fields` | fullstack | Field tambahan per jenis record (teks, angka, tanggal, pilihan, wilayah, file) + kamus tag | — | `RecordTypes` dari tiap modul | Disimpan JSON tervalidasi | S1 |
| 15 | ⬜ | `payments` | fullstack | Tagihan bayar lewat gateway: QRIS dinamis, virtual account, e-wallet, link bayar, webhook bertanda tangan & idempoten, refund, rekonsiliasi | — | `PayableResolver` · emits `payments.intent.succeeded` / `expired` / `refunded` | MDR QRIS ditanggung merchant, tidak boleh dibebankan ke konsumen [V]; Midtrans SHA512, Xendit `x-callback-token` [V] | S15, S16, S17 |
| 16 | ⬜ | `integrations` | fullstack | Kunci API ber-scope (hash + prefix), langganan webhook keluar bertanda tangan HMAC dengan retry, log & replay | — | `EventCatalog` · emits `integrations.webhook.delivered` / `failed` | | S16, S17 |
| 17 | ⬜ | `reports` | fullstack | Dasbor widget (KPI, grafik, tabel), laporan tersimpan, kirim terjadwal via email/PDF | — | `MetricSource` dari tiap modul, `Mailer` · kit `pdfdoc` | Grafik masuk kit UI bila belum ada | S6, S7 |
| 18 | ⬜ | `data-privacy` | fullstack | Catatan persetujuan, kotak masuk permintaan subjek data, register pemrosesan & masa simpan, log insiden kebocoran, purge terjadwal | — | `PersonalDataProvider` dari tiap modul · emits `data-privacy.request.received` | PP 33/2026 berlaku 16 Jan 2027: respons & notifikasi 3×24 jam [S] | S30 |

## Gelombang 3 — Layanan & publik

Modul siap pakai untuk kantor, sekolah, klinik, komunitas, usaha jasa.

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 19 | ⬜ | `forms` | fullstack | Pembuat formulir & survei: logika, validasi, tautan publik & QR, skor kuis, ekspor jawaban, bisa disematkan di modul lain | — | `RegionLookup`, `FileStore` · emits `forms.response.submitted` | Untuk pendaftaran, PPDB, intake | S1 |
| 20 | ⬜ | `booking` | fullstack | Satu mesin reservasi: ruang, lapangan, layanan, dokter — aturan ketersediaan, slot, kapasitas, buffer, DP, pengingat, daftar tunggu | — | `WorkCalendar`, `PaymentGateway`, `Notifier` · emits `booking.reservation.confirmed` / `cancelled` | | S1 |
| 21 | ⬜ | `queue` | fullstack | Antrean: layanan, loket, tiket (kiosk/online/WhatsApp), panggilan + layar + suara, transfer, statistik waktu tunggu | — | `Notifier`, `BookingSource` · emits `queue.ticket.called` | Jembatan antrean Mobile JKN sebagai adaptor nanti [U] | S39 |
| 22 | ⬜ | `visitor-log` | fullstack | Buku tamu digital: check-in kiosk/QR, undangan pra-daftar, foto KTP/selfie terenkripsi, notifikasi tuan rumah, badge, daftar pantau, check-out | `HostDirectory` (→ users, employees) | `Notifier`, `FileStore` · kit `barcode`, `fieldcrypt` · emits `visitor-log.visit.checked-in` | Foto KTP = data pribadi (UU PDP): persetujuan & masa simpan | S31, S32 |
| 23 | ⬜ | `helpdesk` | fullstack | Tiket dari portal/email/WhatsApp, tim & penugasan, kebijakan SLA, balasan siap pakai, survei kepuasan | — | `WorkCalendar`, `Mailer`, `ConversationSource` (→ whatsapp), `ArticleSource` (→ cms) · emits `helpdesk.ticket.created` | | S33 |
| 24 | ⬜ | `cms` | fullstack | Halaman & seksi, berita/blog, artikel basis pengetahuan, menu, media, SEO & slug, alur terbit | — | `FileStore`, `Approvals` · emits `cms.content.published` | Situs sekolah, desa, klinik, perusahaan | S1 |
| 25 | ⬜ | `crm` | fullstack | Prospek, pipeline, aktivitas & tindak lanjut, konversi ke pelanggan | — | `CustomerDirectory` (→ customers), `Notifier` · emits `crm.deal.won` | | S1, S2, S41 |

## Gelombang 4 — Penjualan & pembelian

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 26 | ⬜ | `idtax` | gokit | Aturan pajak Indonesia bertanggal berlaku: PPN (12% × DPP 11/12), tarif PPh 4(2)/15/22/23, bea meterai, pembangun XML Coretax | — | — | PMK 131/2024 [V]; PPh 23 & meterai Rp10.000 di atas Rp5 juta [S] | S21, S22 |
| 27 | ⬜ | `items` | fullstack | Produk & jasa: varian, kategori, satuan & konversi, SKU/barcode, gambar, kode barang/jasa Coretax | — | `FileStore`, `DocNumbers` (→ numbering); kontrak `ItemCatalog` · emits `items.item.changed` / `archived` | Kode barang wajib di faktur (PER-11/PJ/2025) [S] | S7, S9, S35 |
| 28 | ⬜ | `pricing` | fullstack | Daftar harga per cabang/kanal/kelompok pelanggan, harga bertingkat & terjadwal, promo (%, Rp, beli X gratis Y, bundel, happy hour), voucher berbatas | `ItemCatalog` | `CustomerDirectory`; kontrak `PriceResolver`, `PromotionEngine` · emits `pricing.voucher.redeemed` | | S9, S14 |
| 29 | ⬜ | `sales` | fullstack | Penawaran, pesanan penjualan, pengiriman, retur | `CustomerDirectory`, `ItemCatalog` | `PriceResolver`, `StockReservation`, `Approvals`, `DocNumbers`, `PeriodLock` · emits `sales.order.confirmed`, `sales.delivery.confirmed` | | S1, S2, S7 |
| 30 | ⬜ | `purchasing` | fullstack | Permintaan pembelian, RFQ, PO, penerimaan barang (lot & kedaluwarsa), tagihan pemasok, retur, saran pemesanan ulang | `SupplierDirectory` (→ suppliers), `ItemCatalog` | `Approvals`, `BudgetCheck`, `DocNumbers`, `PeriodLock` · emits `purchasing.receipt.confirmed`, `purchasing.bill.posted` | PPN masukan, PPh 23 jasa | S1, S41 |
| 31 | ⬜ | `invoicing` | fullstack | Faktur penjualan, nota kredit, faktur uang muka, baris PPN, penanda meterai, cetak PDF | `CustomerDirectory` | `ItemCatalog`, `DocNumbers`, `PeriodLock` · kit `idtax`, `pdfdoc` · emits `invoicing.invoice.posted` / `cancelled` | | S1, S8 |
| 32 | ⬜ | `settlements` | fullstack | Penerimaan & pembayaran piutang/hutang, alokasi ke faktur/tagihan, umur piutang/hutang, termin pembayaran (net-N, cicilan) | `OpenDocuments` (→ invoicing, purchasing) | `CashAccounts`, `PaymentGateway`; kontrak `PaymentTerms` · emits `settlements.payment.received` / `made` | | S2, S7 |

## Gelombang 5 — Gudang & POS

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 33 | ⬜ | `stock` | fullstack | Multi-gudang, lokasi rak (lantai/baris/kolom/rak), buku besar mutasi stok, stok tersedia & reservasi, transfer antar gudang (dalam perjalanan), lot/serial & kedaluwarsa (FEFO), valuasi rata-rata/FIFO, peringatan stok minimum | `ItemCatalog` | `PeriodLock`; kontrak `StockLedger`, `StockReservation` · emits `stock.level.changed`, `stock.lot.expiry-flagged` | | S9, S10, S12 |
| 34 | ⬜ | `stock-count` | fullstack | Stok opname (penuh/siklus/buta), scan, impor Excel, tinjau selisih, posting penyesuaian | `StockLedger` | `ImportTarget` (→ data-io) · emits `stock-count.count.posted` | | S9, S12 |
| 35 | ⬜ | `pos` | fullstack | Kasir: perangkat, shift (modal awal, kas masuk/keluar, hitung tutup, selisih), transaksi, multi/split pembayaran (tunai, QRIS, kartu), void/refund, struk, sinkron offline idempoten | `ItemCatalog` | `PriceResolver`, `PromotionEngine`, `StockLedger`, `PaymentGateway`, `LoyaltyLedger`, `CustomerDirectory` · kit `ui-print`, `ui-scan` · emits `pos.sale.completed` / `voided`, `pos.shift.closed` | QRIS wajib ada; MDR tidak dibebankan ke konsumen [V] | S9, S15, S36 |
| 36 | ⬜ | `pos-fnb` | fullstack | Restoran: denah meja, open bill, gabung/pisah meja & bill, dine-in/takeaway, self-order QR meja, layar dapur (KDS) per stasiun | `PosSales` (→ pos), `ItemCatalog` | · emits `pos-fnb.bill.sent-to-kitchen`, `pos-fnb.item.prepared` | Service charge & PBJT restoran [U] | S9, S11 |
| 37 | ⬜ | `loyalty` | fullstack | Member, poin (dapat & tukar), tingkat, kedaluwarsa, hadiah, saldo deposit/kartu hadiah | `CustomerDirectory` | `ItemCatalog`; kontrak `LoyaltyLedger` · listens `pos.sale.completed` · emits `loyalty.points.earned` | | S9, S14 |
| 38 | ⬜ | `shipping` | fullstack | Ongkir kurir, booking resi, label, lacak, COD, webhook; adaptor agregator | — | `OrderFulfilment`, `RegionLookup` · emits `shipping.shipment.delivered` | Biteship & RajaOngkir (Komerce) [V] | S18, S19 |
| 39 | ⬜ | `bom` | fullstack | Resep & bill of materials: kit, bertingkat, rendemen, pemotongan bahan otomatis saat penjualan, perintah produksi sederhana | `ItemCatalog` | `StockLedger` · listens `pos.sale.completed` · emits `bom.materials.consumed` | Kebutuhan utama F&B | S1, S9 |

## Gelombang 6 — Keuangan & pajak

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 40 | ⬜ | `ledger` | fullstack | Bagan akun, jurnal manual & otomatis (aturan pemetaan dari event `*.posted`), buku besar, neraca saldo, laba rugi, neraca | — | `PeriodLock`, `DocNumbers`; kontrak `Posting` · emits `ledger.journal.posted` | | S1, S2, S6 |
| 41 | ⬜ | `cash-bank` | fullstack | Akun kas & bank, kas kecil, transfer, impor mutasi rekening, rekonsiliasi bank | — | `Posting` (→ ledger); kontrak `CashAccounts` · emits `cash-bank.statement.reconciled` | | S6, S7, S8 |
| 42 | ⬜ | `expenses` | fullstack | Biaya perusahaan, klaim reimburse karyawan dengan bukti, uang muka & pertanggungjawabannya | `FileStore` | `EmployeeDirectory`, `Approvals` · emits `expenses.claim.approved` | | S1, S3, S4 |
| 43 | ⬜ | `assets` | fullstack | Register aset, pemegang & lokasi, peminjaman, jadwal & catatan perawatan, penyusutan komersial & fiskal, pelepasan | — | `EmployeeDirectory`, `Posting` · emits `assets.depreciation.posted`, `assets.maintenance.scheduled` | Kelompok 1–4 masa manfaat 4/8/16/20 tahun (PMK 72/2023) [V] | S13, S27 |
| 44 | ⬜ | `efaktur` | fullstack | Register faktur keluaran & masukan, impor/ekspor XML Coretax, status persetujuan, retur/batal | `InvoiceSource` (→ invoicing, purchasing), `CompanyTaxIdentity` (→ settings) | `CounterpartyTaxId` (→ customers, suppliers) · kit `idtax` · emits `efaktur.faktur.approved` | Nomor otomatis dari Coretax; unggah paling lambat tgl 20 bulan berikutnya [S] | S22 |
| 45 | ⬜ | `ebupot` | fullstack | Register bukti potong & ekspor XML Coretax: BPPU (PPh 4(2)/15/22/23), BPMP (TER bulanan), BP21, BPA1 | — | `WithholdingSource` (→ purchasing, payroll), `EmployeeDirectory` · kit `idtax` | BPA1 butuh NIK tervalidasi [S] | S23 |

## Gelombang 7 — SDM

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 46 | ⬜ | `idpayroll` | gokit | Aturan penggajian bertanggal berlaku: kategori TER A/B/C & tabelnya, PTKP, hitung tahunan Desember/bulan keluar (Pasal 17), iuran & batas BPJS, upah lembur 1/173, THR proporsional, pesangon PP 35/2021 | — | — | TER A/B/C [V]; iuran BPJS TK & batas JP [V]; BPJS Kesehatan 4%+1% batas Rp12 juta [S]; tabel pesangon [U] | S24, S25, S26 |
| 47 | ⬜ | `employees` | fullstack | Data karyawan, unit & jabatan, grade, kontrak PKWT/PKWTT, atasan langsung, NIK/NPWP/PTKP/BPJS/rekening terenkripsi, profil mandiri | — | `Units` (→ organization), `UserAccounts` (→ users), `FileStore` · kit `taxid`, `fieldcrypt`; kontrak `EmployeeDirectory` · emits `employees.employee.hired` / `terminated` | Lingkup "atasan melihat timnya" perlu desain lingkup data (rbac/kernel) | S3, S4, S5 |
| 48 | ⬜ | `attendance` | fullstack | Absen masuk/pulang (GPS geofence, selfie), shift & jadwal, rekap, koreksi, pengajuan & persetujuan lembur | `EmployeeDirectory` | `WorkCalendar`, `Approvals`, `FileStore` · kit `geo` · emits `attendance.period.closed`, `attendance.overtime.approved` | Lembur maks. 4 jam/hari, 18 jam/minggu (PP 35/2021) [S] | S4, S5, S37 |
| 49 | ⬜ | `leave` | fullstack | Jenis cuti, akrual, saldo, pengajuan, sisa dibawa ke tahun berikutnya, kalender tim | `EmployeeDirectory` | `WorkCalendar`, `Approvals`, `FileStore` (surat dokter) · emits `leave.request.approved` | Cuti tahunan min. 12 hari setelah 12 bulan kerja [S] | S1, S4, S38 |
| 50 | ⬜ | `payroll` | fullstack | Komponen & struktur gaji, periode gaji terkunci, PPh 21 TER, BPJS, THR, kasbon & cicilan, slip gaji PDF terenkripsi, file transfer bank, ekspor jurnal | `EmployeeDirectory` | `AttendanceSummary`, `LeaveSummary`, `ClaimsPayable`, `PeriodLock`, `Posting` · kit `idpayroll`, `money`, `fieldcrypt`, `pdfdoc` · emits `payroll.run.closed` | THR paling lambat 7 hari sebelum hari raya [S] | S3, S4, S5, S24 |

## Kandidat berikutnya

Ditemukan dalam riset tetapi di luar 50. Bila ada slot atau permintaan proyek, pindahkan ke tabel
gelombang yang sesuai dengan nomor berikutnya.

| Modul | Jenis | Fungsi singkat | Rujukan |
|---|---|---|---|
| `events-ticketing` | fullstack | Acara, jenis & kuota tiket, e-tiket QR, check-in pintu, sertifikat PDF | S1 |
| `membership` | fullstack | Paket langganan & perpanjangan, kartu member QR, paket kelas | S1 |
| `school` | fullstack | Peserta didik, rombel, mapel, absensi, nilai & rapor; adaptor impor Dapodik | S40 |
| `clinic` | fullstack | Pasien, kunjungan, rekam medis SOAP, ICD-10, resep; adaptor SATUSEHAT (OAuth2, satu client per Organization) [V] | S39 |
| `koperasi` | fullstack | Anggota, simpanan pokok/wajib/sukarela, pinjaman & cicilan, SHU; batas Permenkop 8/2023 [S] | — |
| `rental` | fullstack | Sewa barang & properti/kos: ketersediaan, kontrak, tagihan bulanan, deposit, denda | S1 |
| `projects` | fullstack | Proyek, tugas, timesheet, biaya proyek | S1, S2 |
| `budgets` | fullstack | Anggaran per akun/pusat biaya/proyek, realisasi vs anggaran, kontrak `BudgetCheck` | S6 |
| `wms` | fullstack | Saran putaway, daftar ambil (batch/zona), packing, pengisian ulang rak, layar scanner | S10 |
| `storefront` | web | Toko & pemesanan online di atas `items`, `pricing`, `payments`, `shipping` | S9 |
| `marketplace-sync` | fullstack | Sinkron produk, stok, pesanan dengan marketplace & GrabFood | S10 |
| `emeterai` | fullstack | Deteksi dokumen wajib meterai, ledger meterai, adaptor distributor resmi Peruri | S7, S8 |
| `esign` | fullstack | Tanda tangan elektronik sederhana + port adaptor PSrE (Privy, Peruri, VIDA) | — |
| `search` | fullstack | Pencarian global lintas modul (MySQL FULLTEXT ngram), hasil tersaring izin | — |
| `recruitment` | fullstack | Lowongan, pelamar, tahapan seleksi | S4 |
| `performance` | fullstack | KPI & penilaian kinerja | S4, S5 |
| `offboarding` | fullstack | Resign/PHK, checklist keluar, hitung pesangon | S26 |
| `fleet` | fullstack | Kendaraan, pengemudi, masa berlaku STNK/KIR, servis, BBM | — |
| `donations` | fullstack | Kampanye, donatur, kuitansi PDF, laporan Kemensos [S]; QRIS donasi sosial MDR 0% [V] | S15 |

## Rujukan

Produk dan modul yang menjadi dasar daftar:

- **S1** Odoo, daftar aplikasi — https://www.odoo.com/page/all-apps
- **S2** ERPNext, modul inti — https://github.com/frappe/erpnext/tree/develop/erpnext
- **S3** Frappe HR — https://github.com/frappe/hrms
- **S4** Talenta (Mekari), fitur — https://www.talenta.co/fitur/
- **S5** Gadjian — https://www.gadjian.com/
- **S6** Jurnal (Mekari), fitur — https://www.jurnal.id/id/fitur/
- **S7** Accurate, fitur — https://accurate.id/fitur/
- **S8** Kledo, fitur — https://kledo.com/fitur
- **S9** majoo, panduan pengguna (meja, resep, loyalitas, stok opname) — https://majoo.id/panduan-pengguna
- **S10** Jubelio, warehouse system — https://jubelio.com/en/product/omnichannel/warehouse-system/
- **S11** Odoo POS restoran — https://www.odoo.com/documentation/18.0/applications/sales/point_of_sale/restaurant.html
- **S12** ERPNext stock reconciliation — https://docs.frappe.io/erpnext/stock-reconciliation.md
- **S13** ERPNext asset movement & maintenance — https://docs.frappe.io/erpnext/asset-movement.md · https://docs.frappe.io/erpnext/asset-maintenance.md
- **S14** Square Loyalty API — https://developer.squareup.com/docs/loyalty-api/overview
- **S31** Envoy, visitor management — https://envoy.com/features/visitor-badges
- **S32** Buku tamu digital QR + notifikasi WhatsApp (jurnal) — https://jurnal.polgan.ac.id/index.php/remik/article/view/15661
- **S33** Freshdesk, fitur — https://www.freshworks.com/freshdesk/support-desk/features/
- **S36** Kasir Pintar, printer & struk — https://help.kasirpintar.co.id/knowledge-base/category/aplikasi-kasir-pintar-pos/pengaturan-aplikasi-kasir-pintar-pos/printer-dan-struk-pengaturan-aplikasi-kasir-pintar-pos
- **S40** Frappe Education, absensi siswa — https://docs.frappe.io/education/student-attendance.md
- **S41** HashMicro — https://www.hashmicro.com/id/
- **S42** PDF murni Go (gpdf) — https://pkg.go.dev/github.com/gpdf-dev/gpdf

Regulasi dan integrasi Indonesia:

- **S15** Bank Indonesia, QRIS (MPM statis/dinamis, MDR, PADG) — https://www.bi.go.id/id/fungsi-utama/sistem-pembayaran/ritel/kanal-layanan/QRIS/default.aspx
- **S16** Midtrans, notifikasi pembayaran — https://docs.midtrans.com/reference/handle-notifications
- **S17** Xendit, webhook — https://docs.xendit.co/docs/handling-webhooks
- **S18** Biteship, webhook — https://biteship.com/docs/api/webhook/overview
- **S19** RajaOngkir (Komerce), endpoint ongkir — https://rajaongkir.com/docs/shipping-cost/getting_started/endpoint
- **S20** WhatsApp Business Platform, harga — https://developers.facebook.com/docs/whatsapp/pricing
- **S21** PMK 131/2024, PPN DPP 11/12 — https://pajak.go.id/index.php/id/artikel/pmk-1312024-tarif-ppn-sebelas-dua-belas
- **S22** PER-8/PJ/2025 & PER-11/PJ/2025, Coretax — https://www.alvarezandmarsal.com/thought-leadership/coretax-regulation-update-per-8-pj-2025-and-per-11-pj-2025
- **S23** Jenis bukti potong di Coretax — https://news.ddtc.co.id/berita/nasional/1818044/ada-beragam-menu-bupot-di-coretax-apa-bedanya
- **S24** PPh 21 TER (PMK 168/2023) — https://pajak.go.id/id/artikel/penghitungan-pph-pasal-21-jangan-sampai-ter-koreksi
- **S25** Iuran BPJS Ketenagakerjaan — https://www.bpjsketenagakerjaan.go.id/artikel/18913/artikel-berapa-besaran-iuran-jht,-jkk,-jkm,-jp-dan-jkp
- **S26** PP 35/2021 (PKWT, alih daya, waktu kerja, PHK) — https://peraturan.bpk.go.id/Details/161904/pp-no-35-tahun-2021
- **S27** PMK 72/2023, penyusutan — https://pajak.go.id/id/siaran-pers/pmk-72-tahun-2023-terbit-berikut-pokok-aturannya
- **S28** SKB 3 Menteri libur nasional & cuti bersama 2026 — https://www.setneg.go.id/baca/index/inilah_skb_3_menteri_libur_nasional_dan_cuti_bersama_2026
- **S29** Kode wilayah Kemendagri 2025 — https://infopublik.id/kategori/nasional-politik-hukum/919675/ · data: https://github.com/cahyadsn/wilayah
- **S30** UU 27/2022 Pelindungan Data Pribadi — https://peraturan.bpk.go.id/Details/229798 · PP 33/2026 (sekunder): https://siplawfirm.id/resources/pp-33-2026-pelindungan-data-pribadi
- **S35** Kode barang di faktur (PER-11/PJ/2025) — https://ortax.org/bagaimana-ketentuan-pencantuman-kode-barang-di-per-11-2025
- **S37** Upah lembur per jam — https://www.hukumonline.com/klinik/a/cara-hitung-upah-lembur-per-jam-ini-rumusnya!-lt4b7e3965556d3/
- **S38** Cuti tahunan — https://www.hukumonline.com/klinik/a/cuti-tahunan-lebih-dari-12-hari-lt5cda7b8923c44
- **S39** SATUSEHAT, playbook integrasi — https://satusehat.kemkes.go.id/platform/docs/id/playbook/

Riset dilakukan 28 Oktober 2026. Harga, tarif, dan peraturan berubah; verifikasi ulang saat
rencana modul ditulis.
