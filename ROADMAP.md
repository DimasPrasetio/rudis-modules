# Roadmap modul rudis-modules

Acuan pengembangan pustaka: **89 modul baru** yang akan dibangun, di luar 21 modul dan 1 preset
yang sudah rilis. Tujuannya memperkaya modul reusable agar aplikasi apa pun yang diminta pengguna
nanti — ERP, HRMIS, gudang, POS, buku tamu, booking, helpdesk, website, sekolah, klinik — bisa
dirakit dari modul yang sudah ada, dan Rudis-SDD hanya membuat bagian yang benar-benar khusus
proyek.

Daftar ini disusun dari riset internet (produk yang paling banyak dipakai dan regulasi Indonesia;
sumber di [Rujukan](#rujukan)) dan dari arsitektur yang terkunci di [AGENTS.md](AGENTS.md).

**Ini juga checkpoint.** Saat sebuah modul dirilis (tag `<id>/v<versi>` ada di `main`), ubah
statusnya di tabel, tulis versinya, dan perbarui kolom *Selesai* di ringkasan. Urutan pengerjaan
mengikuti gelombang: ambil dari gelombang paling awal yang masih terbuka.

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

## Seberapa besar satu modul

Satu modul adalah **satu unit pasang**: proyek yang memasangnya menerima semua tabel, menu, izin,
dan migrasinya. Bagian yang tidak dibutuhkan klien tetap ikut terpasang, jadi batas modul
ditentukan dengan uji ini.

**Pisahkan** bagian menjadi modul sendiri bila salah satu benar:

1. Ada klien yang wajar membutuhkan A **tanpa** B (distributor butuh daftar harga tanpa promo).
2. Bagian itu punya **data dan tabelnya sendiri** (transfer stok, kasbon, basis pengetahuan).
3. Bagian itu dipakai **jenis aplikasi yang berbeda** (layar dapur dipakai POS dan pesanan online).

**Satukan** hanya bila keduanya selalu dipakai bersama atau yang satu tidak bermakna tanpa yang
lain (shift dalam absensi; lot, kedaluwarsa, dan valuasi dalam buku besar stok; QR dan barcode
dalam satu kit tanpa tabel).

Jangan memecah lebih halus dari *fitur yang wajar dipakai terpisah*: setiap pecahan menambah port
dan adaptor yang harus dirakit di setiap proyek.

## Batas kit aturan Indonesia

Satu aturan tinggal di **satu** kit. Modul yang butuh aturan itu memakai kitnya; tidak ada tarif,
tabel, atau format resmi yang ditulis ulang di modul atau di kit lain.

| Kit | Menjawab | Isi | Berubah karena | Tidak memuat |
|---|---|---|---|---|
| `taxid` ✅ | **Siapa** — apakah nomor identitas ini sah formatnya? | NPWP 16 digit, NIK, NITKU, paspor, Other ID, penyamaran, pencarian aman | struktur nomor (jarang) | tarif, perhitungan, format XML |
| `taxrules` | **Berapa pajaknya?** | PPN & DPP, PPh 4(2)/15/22/23, PPh 21 (TER, PTKP, Pasal 17), PPh final pesangon, bea meterai | PMK/PP tarif (sering) | identitas, iuran BPJS, format XML |
| `payrollrules` | **Berapa hak & kewajiban ketenagakerjaannya?** | iuran BPJS & batas upah, upah lembur, THR, pesangon/UPMK/UPH, UMK | PP/Permenaker, ketetapan BPJS, UMK tahunan | pajak apa pun (PPh 21 milik `taxrules`) |
| `coretax` | **Bagaimana menyerahkannya ke DJP?** | XML faktur & bukti potong, kode transaksi, aturan ID TKU | versi templat Coretax | tarif (dipakai dari `taxrules`), validasi identitas (dari `taxid`) |

Pemakaian: `invoicing`, `purchasing`, `pos` → `taxrules`; `efaktur`, `ebupot` → `taxrules` +
`coretax` + `taxid`; `payroll` → `taxrules` + `payrollrules`; `overtime` → `payrollrules`;
`offboarding` → `payrollrules` + `taxrules`. Kit boleh memakai kit lain (`gokit-pure`), tidak
pernah modul.

## Syarat wajib setiap modul — pelajaran audit 28 Oktober 2026

Audit 21 modul pertama (fungsional, UI/UX, arsitektur) menemukan kesalahan yang mudah terulang.
Setiap modul di roadmap **wajib** memenuhi daftar ini. Rencana modul di `docs/plans/` memuat
bagian "Syarat wajib" yang menjawab setiap butir (✓ atau alasan tidak berlaku), dan tinjauan
rilis memeriksanya.

### A. Data milik perusahaan dan data pribadi

1. **Scoped sejak awal.** Modul yang menyimpan data operasional memakai `"scoped": true`;
   tidak ada tabel atau daftar global yang bisa dibaca semua perusahaan. *(audit: mailer,
   audit-log)*
2. **Catatan tingkat instalasi disimpan sebagai perusahaan 0** dan hanya terlihat oleh izin
   tingkat instalasi yang terpisah — tidak pernah dititipkan ke perusahaan 1. *(audit-log)*
3. **Identitas orang tidak bisa ditebak lewat pencarian.** NIK, paspor, NPWP orang pribadi,
   nomor rekening, nomor BPJS: hanya cocok persis, tidak pernah `LIKE 'xxx%'`. *(customers,
   suppliers)*
4. **Samarkan berdasarkan "apakah ini orang"**, bukan berdasarkan jenis kolom; buka hanya
   dengan izin `*.sensitive` dan selalu diaudit.
5. **Akses berkas mengikuti modul pemiliknya.** Berkas milik modul lain tidak bisa diunduh
   lewat izin umum. *(files)*
6. **`LIKE` selalu di-escape** (`%`, `_`).

### B. Keamanan

7. **Pembatas laju menghitung kegagalan, bukan keberhasilan**; gunakan penundaan bertahap,
   bukan kunci keras yang bisa dipakai penyerang untuk mengunci korban. *(auth)*
8. **Perubahan sensitif pada akun sendiri** (email, sandi, rekening, peran) lewat jalur
   mandiri dengan autentikasi ulang, tidak lewat endpoint admin. *(users)*
9. **Pemberian hak dan perubahan peran adalah kejadian keamanan**: diaudit sebagai
   `security`, di dalam transaksi yang sama. *(rbac)*
10. **Perubahan rekening bank, alamat tagih, atau data pembayaran** butuh persetujuan orang
    kedua atau masa tunggu (anti-fraud).

### C. Ketepatan

11. **Baca-ubah-tulis dalam satu transaksi terkunci** (`SELECT … FOR UPDATE` lalu tulis,
    audit di transaksi yang sama). *(users, numbering)*
12. **Pembaruan sebagian tidak menimpa field yang tidak dikirim** (zona waktu, logo, NITKU).
    *(settings)*
13. **Efek samping keluar sistem idempoten** (email, webhook, pembayaran, kurir): pengiriman
    ulang tidak menggandakan; error setelah pihak lain menerima diperlakukan sebagai sukses;
    salah konfigurasi dicoba ulang dan diperingatkan, bukan "gagal permanen". *(mailer)*
14. **Byte di disk mengikuti transaksi**: berkas yang barisnya batal disapu. *(files)*
15. **Data turunan diperiksa ulang saat sumbernya berubah** (NITKU saat NPWP berubah).
    *(customers, suppliers)*

### D. Regulasi Indonesia

16. Aturan pajak, BPJS, upah, kode wilayah, hari libur **disimpan sebagai data bertanggal
    berlaku** di kit (`taxrules`, `payrollrules`, `coretax`, `taxid`), dicek ke **teks resmi** (pajak.go.id,
    peraturan.bpk.go.id, JDIH) dan diberi sumber di README.
17. Validasi identitas lewat kit `taxid` (NPWP 16 digit, NIK-sebagai-NPWP, NITKU 22 digit,
    jenis "Other ID"), tidak ditulis ulang di modul.

### E. Arsitektur

18. **Data master menerbitkan event siklus hidup** (`<id>.<benda>.created/changed/deactivated`)
    agar modul lain bisa bereaksi tanpa polling. *(audit: hanya satu modul yang menerbitkan event)*
19. **Tawarkan kontrak yang akan dibutuhkan modul lain** sejak versi pertama (mis. `RoleHolders`
    di rbac untuk approvals).
20. Kebutuhan lintas modul tetap lewat port — dan bila satu modul berubah status (penempatan
    dicabut), modul terkait diberi tahu lewat event, bukan dibiarkan basi. *(organization ↔ rbac)*

### F. UI/UX — riset dulu, bangun kemudian

Setiap modul yang punya layar **wajib melewati gerbang riset desain** sebelum kode UI ditulis:

21. **Riset** bagaimana layar sejenis dirancang dengan baik saat ini (produk acuan + pedoman:
    WCAG 2.2 AA, Carbon, NN/g, GOV.UK Design System, Baymard; untuk sentuh: Material/HIG) dan
    tulis **brief desain bersumber** di rencana modul (`docs/plans/PLAN-<NAME>.md` › Desain).
22. **Pakai kit, jangan buat ulang** (`kits-first`): bila komponen generik belum ada, tambahkan
    dulu ke kit + `ui-gallery` dalam perubahan sendiri.
23. **Daftar periksa aksesibilitas** (dibuktikan di bagian Hasil rencana):
    - fokus keyboard terlihat ≥3:1 pada setiap kontrol (WCAG 1.4.11, 2.4.7) dan tidak
      tertutup header/footer lengket (2.4.11);
    - judul halaman lewat `PageHeader`, satu `<h1>`, skip link dari app-shell (2.4.1, 2.4.2);
    - nama aksesibel = label; tidak ada elemen interaktif bersarang;
    - `autocomplete` pada data pribadi (1.3.5); OTP/sandi bisa ditempel (3.3.8);
    - error: ringkasan di atas + pesan di samping field + ikon, divalidasi saat kirim
      (GOV.UK/NN/g); penjaga perubahan belum disimpan;
    - status diumumkan tanpa memindah fokus (4.1.3);
    - target ≥24 px (2.5.8); **≥44–48 px untuk modul sentuh/kiosk** (POS, absensi, buku tamu,
      antrean, KDS) memakai density sentuh;
    - `prefers-reduced-motion` dihormati.
24. **Format Indonesia lewat kit**: uang dengan `MoneyInput` (titik ribuan, koma desimal),
    tanggal `dd/mm/yyyy` dan nama bulan Indonesia, telepon +62, NPWP/NIK — tidak pernah
    `Number()` atau `parseFloat` pada uang.
25. **Tabel**: identitas baris adalah tautan sungguhan (bisa dibuka di tab baru), state
    kosong/memuat/error tidak menghilangkan header, tata letak HP tidak sekadar geser
    horizontal.
26. **Verifikasi visual wajib sebelum rilis**: screenshot desktop (≥1280 px) dan HP (375 px),
    navigasi keyboard penuh, konsol bebas galat — dilampirkan di bagian Hasil rencana.

### G. Pengujian

27. Setiap perbaikan dan setiap aturan di atas yang berlaku punya **cek testbed**; modul scoped
    wajib punya cek isolasi dua perusahaan.
28. Testbed penuh dijalankan sebelum rilis; hasilnya dicatat di STATUS.md › Last verified.

Fakta regulasi di kolom Catatan diberi tanda **[V]** terverifikasi di sumber resmi, **[S]** dari
sumber sekunder, **[U]** belum terverifikasi. Semua wajib dicek ulang ke teks resmi saat rencana
modulnya ditulis.

## Modul yang sudah ada

| Status | Modul | Jenis | Menyediakan |
|---|---|---|---|
| ✅ 0.8.0 | `auth` | fullstack | login email + sandi, sesi server |
| ✅ 0.9.0 | `users` | fullstack | kontrak `UserClient` |
| ✅ 0.7.0 | `rbac` | fullstack | peran & izin per perusahaan/cabang; kontrak `RoleHolders` |
| ✅ 0.2.0 | `organization` | fullstack | `OrganizationClient`; event `organization.company.created`, `organization.user.unplaced` |
| ✅ 0.3.1 | `settings` | fullstack | `SettingsClient` (profil perusahaan, NPWP, logo) |
| ✅ 0.3.0 | `audit-log` | fullstack | jejak audit `d.Audit` |
| ✅ 0.3.0 | `files` | fullstack | `FileClient` (unggah ke `DATA_DIR`) |
| ✅ 0.3.1 | `mailer` | fullstack | `MailClient` (antrean email per perusahaan) |
| ✅ 0.1.0 | `numbering` | fullstack | `NumberingClient` (nomor dokumen) |
| ✅ 0.1.0 | `period` | fullstack | `PeriodClient` (tutup buku per area) |
| ✅ 0.3.0 | `customers` | fullstack | `CustomerClient` (identitas pajak Coretax); event siklus hidup |
| ✅ 0.3.0 | `suppliers` | fullstack | `SupplierClient`; event siklus hidup |
| ✅ 0.1.0 | `money` | gokit | Rupiah, kuantitas, tarif presisi |
| ✅ 0.1.0 | `docstate` | gokit | draft → posted → cancelled → amended |
| ✅ 0.2.0 | `taxid` | gokit | NPWP 16 digit, NIK |
| ✅ 0.7.0 | `app-shell` | web | bingkai halaman, sidebar dari nav modul, breadcrumb dari halaman |
| ✅ 0.6.0 | `ui-core` | ui | primitif & layout, zona waktu tampilan, density, breadcrumb |
| ✅ 0.3.0 | `ui-data` | ui | DataTable (tautan identitas, tata letak HP, pilih semua lintas halaman) |
| ✅ 0.4.0 | `ui-forms` | ui | field, pencarian, alamat, `MoneyInput`, `ErrorSummary`, pemilih tanggal/periode |
| ✅ 0.2.0 | `ui-feedback` | ui | toast (dengan Urungkan), dialog konfirmasi, `Banner` |
| ✅ 0.2.0 | `ui-gallery` | web | galeri komponen kit |
| ✅ 0.2.0 | `bingkai-navy` | preset | tema back-office, density sentuh/kiosk |

## Ringkasan 89 modul baru

| Gelombang | Fokus | Jumlah | Selesai |
|---|---|---|---|
| 0 | Perbaikan fondasi (audit) — bukan modul baru | 20 butir | 19 (P1 selesai, 0.18 lewat kernel 7; 0.20 P2 terbuka) |
| 1 | Kit dasar | 10 | 0 |
| 2 | Platform lintas bidang | 14 | 0 |
| 3 | Layanan, kantor & publik | 11 | 0 |
| 4 | Penjualan & pembelian | 8 | 0 |
| 5 | Gudang, POS & toko | 17 | 0 |
| 6 | Keuangan & pajak | 9 | 0 |
| 7 | SDM | 10 | 0 |
| 8 | Vertikal & lanjutan | 10 | 0 |
| | **Total** | **89** | **0** |

Gelombang mengikuti arah port: modul di gelombang N hanya memakai port yang penyedianya ada di
gelombang yang sama atau sebelumnya (atau modul yang sudah ada). Gelombang 3 sengaja sebelum ERP:
modul layanan memakai paling sedikit port dan paling cepat berguna untuk aplikasi yang jenisnya
belum diketahui. Gelombang 8 berisi modul domain sempit dan pelengkap yang bernilai tinggi bila
dibutuhkan, tetapi jarang menjadi kebutuhan pertama.

## Gelombang 0 — Perbaikan fondasi (audit 28 Oktober 2026)

Audit fungsional, UI/UX, dan arsitektur atas modul yang sudah ada sebelum roadmap dimulai.
**P0** dan **P1** sudah diperbaiki (29 Oktober 2026); 0.18 selesai lewat kernel 7 atas issue
[rudis-modules#1](https://github.com/DimasPrasetio/rudis-modules/issues/1) (teksnya di `docs/kernel-requests/`); **P2** (0.20) terbuka dan diambil bila sebuah
gelombang membutuhkannya.
Akar masalahnya kini menjadi [syarat wajib](#syarat-wajib-setiap-modul--pelajaran-audit-28-oktober-2026)
untuk setiap modul berikutnya. Kolom *Syarat wajib* menunjuk kelompok dan nomor butirnya
(mis. `B7` = kelompok B, butir 7).

| # | Status | Perbaikan | Modul (versi) | Syarat wajib |
|---|---|---|---|---|
| 0.1 | ✅ | Catatan tanpa perusahaan disimpan sebagai perusahaan 0, hanya untuk izin `audit-log.system`; disalin ke perusahaan tempat orangnya ditempatkan; lingkup baca per cabang | `audit-log` 0.3.0 | A1, A2 |
| 0.2 | ✅ | Outbox email per perusahaan (`company_id`), email tingkat instalasi untuk `mailer.system`, nama pengirim per perusahaan lewat port opsional `SenderIdentity` | `mailer` 0.3.0 | A1, A2 |
| 0.3 | ✅ | Pencarian identitas orang hanya cocok utuh (tanpa oracle prefix); semua identitas orang disamarkan; 409 duplikat hanya untuk penulis | `customers` 0.2.0, `suppliers` 0.2.0 | A3, A4 |
| 0.4 | ✅ | Pembatas login hanya menghitung kegagalan, penundaan bertahap + 429 `Retry-After`, LRU O(1) per jenis kunci; idle timeout `SESSION_IDLE_TIMEOUT` (60 menit); sesi lama dicabut saat login ulang; akun nonaktif tidak lagi menjadi oracle sandi | `auth` 0.8.0 | B7 |
| 0.5 | ✅ | Berkas milik modul lain tidak terjangkau izin umum `files.read`; sapu byte yatim harian; ekstensi mengikuti tipe terdeteksi; cek paket Office | `files` 0.3.0 | A5, C14 |
| 0.6 | ✅ | `Update` pengguna satu transaksi terkunci; email/sandi akun sendiri tidak lewat endpoint admin; `LIKE` di-escape | `users` 0.8.0 | B8, C11 |
| 0.7 | ✅ | Zona waktu tidak tertimpa saat simpan profil, audit di dalam transaksi; email tidak terkirim ganda saat QUIT gagal, salah kredensial SMTP dicoba ulang, Message-ID stabil | `settings` 0.3.1, `mailer` 0.3.0 | C12, C13 |
| 0.8 | ✅ | NPWP tak berawalan 0 = NIK dan diperiksa sebagai NIK, nilai serba-nol ditolak, kode provinsi NIK, jenis "Other ID", `DefaultNITKU`; NITKU default ikut pindah saat NPWP berubah | `taxid` 0.2.0, `customers` 0.2.0, `suppliers` 0.2.0 | C15, D17 |
| 0.9 | ✅ | `MoneyInput` (titik ribuan, koma desimal, tanpa floating point), dipakai di batas kredit | `ui-forms` 0.3.0, `customers` 0.2.0 | F24 |
| 0.10 | ✅ | Fokus keyboard ≥3:1 di semua kontrol (cincin `--ring` 4,3–6,8:1), opsi pilihan terlihat saat fokus, tanpa tombol bersarang, FileDropzone, `autocomplete`, skip link, judul per halaman, `aria-current`, reduced-motion | `ui-core` 0.5.0, `ui-forms` 0.3.0, `ui-data` 0.2.1, `app-shell` 0.6.0, `ui-gallery` 0.1.2, `bingkai-navy` 0.1.2 | F23 |
| 0.11 | ✅ | Peran per perusahaan atau templat peran; audit pemberian admin saat start; peran dicatat `security` di dalam transaksi; kontrak `RoleHolders` | `rbac` 0.7.0 | B9, E19 |
| 0.12 | ✅ | Penempatan dicabut → peran di perusahaan itu ikut dicabut (event organization, didengar rbac) | `organization` 0.2.0, `rbac` 0.7.0 | E20 |
| 0.13 | ✅ | Event siklus hidup data master (`customers`, `suppliers`, `users`) | `customers` 0.3.0, `suppliers` 0.3.0, `users` 0.9.0 | E18 |
| 0.14 | ✅ | `ErrorSummary` + penjaga perubahan belum disimpan; `DatePicker` id-ID + rentang dengan preset + `PeriodPicker`; toast dengan Urungkan; banner | `ui-forms` 0.4.0, `ui-feedback` 0.2.0, `ui-gallery` 0.2.0 | F23, F24 |
| 0.15 | ✅ | Tabel: identitas baris sebagai tautan, state tanpa menghilangkan header, tata letak HP, header mengikuti density, pilih semua lintas halaman, pengumuman jumlah terpilih | `ui-data` 0.3.0 | F25 |
| 0.16 | ✅ | Breadcrumb yang bisa kembali ke daftar (API crumbs); tanggal mengikuti zona perusahaan/cabang | `app-shell` 0.7.0, `ui-core` 0.6.0, `organization` 0.2.0 | F23 |
| 0.17 | ✅ | Density sentuh/kiosk (target 44–48 px, tanpa hover, layar penuh + reset idle) | `bingkai-navy` 0.2.0 (density `touch`), `ui-core` 0.6.0 (layar penuh, reset idle) | F23 |
| 0.18 | ✅ kernel 7 (issue [rudis-modules#1](https://github.com/DimasPrasetio/rudis-modules/issues/1)) | Kernel request: cincin fokus Button/FIELD ≥3:1 di kernel (`--ring` solid); `kernel.CompanyOf` dan `d.Scope.Several()` untuk "satu atau banyak perusahaan" | kernel 7 (`rudis-monorepo/kernel/v7.0`), `mailer` 0.3.1, `bingkai-navy` 0.2.1 — teks: `docs/kernel-requests/0001-focus-ring-and-company-count.md` | F23 |
| 0.19 | ✅ | Alat: `npm run sandbox` gagal bila `organization` dipasang bersama `users` (siklus port terlambat); `testbed --only documents` gagal bila `.testbed/` belum ada | `scripts/` (cek di skenario `lifecycle`) | G27 |
| 0.20 | ⬜ P2 | MFA TOTP + kode pemulihan, daftar sesi aktif & cabut, undangan pengguna lewat email, wajib ganti sandi, daftar sandi bocor + 15 karakter; hard lock periode; bounce email; deteksi duplikat pelanggan; rantai hash audit; mode gelap | `auth`, `users`, `period`, `mailer`, `customers`, `audit-log`, tema | B, C |

## Gelombang 1 — Kit dasar

Tanpa tabel, tanpa port. Dipakai banyak modul di gelombang berikutnya.

| # | Status | Modul | Jenis | Fungsi | Catatan | Rujukan |
|---|---|---|---|---|---|---|
| 1 | ⬜ | `fieldcrypt` | gokit | Enkripsi kolom data pribadi (AES-GCM, rotasi kunci, indeks buta untuk cari NIK, masking) | UU PDP mewajibkan langkah keamanan teknis [S]; kunci dari `config` | S30 |
| 2 | ⬜ | `pdfdoc` | gokit | Bangun PDF murni Go: kop surat, tabel, nomor halaman, QR, format tanggal & angka Indonesia, terbilang | Tanpa Chromium (batas container) | S42 |
| 3 | ⬜ | `barcode` | gokit | Buat & cek QR dan barcode 1D (EAN-13, Code128), GS1 lot/kedaluwarsa, token QR bertanda tangan HMAC berbatas waktu | Dekode kamera di browser, bukan di server (tanpa CGO) | S10 |
| 4 | ⬜ | `geo` | gokit | Jarak haversine, titik dalam poligon, geofence radius | Untuk absensi GPS dan check-in lokasi | S4 |
| 5 | ⬜ | `provider-http` | gokit | Klien HTTP untuk penyedia luar: timeout, retry dengan backoff, batas laju, mode sandbox/live, log dengan rahasia tersamar | Dipakai payments, shipping, whatsapp, marketplace-sync, efaktur | S16, S17 |
| 6 | ⬜ | `webhook-inbox` | gokit | Verifikasi webhook masuk (HMAC, SHA512 Midtrans, token Xendit), kunci idempotensi, jendela replay; tabel inbox tetap milik modul pemakai | Midtrans SHA512 dan Xendit `x-callback-token` [V] | S16, S17, S18 |
| 7 | ⬜ | `ui-print` | ui | Cetak dari browser: tata letak cetak A4, struk thermal 58/80 mm (ESC/POS lewat WebUSB/Bluetooth/Serial), fallback `window.print` | Server tidak bisa menjangkau printer toko; dipakai POS, dapur, badge buku tamu | S9, S36 |
| 8 | ⬜ | `ui-scan` | ui | Input scanner barcode (keyboard wedge), scan kamera (BarcodeDetector), scan-ke-field | Dipakai gudang, POS, buku tamu, aset | S9 |
| 9 | ⬜ | `ui-live` | ui | Hook pembaruan realtime (SSE) untuk layar dapur, antrean, status pesanan | Memakai streaming respons kernel; WebSocket bila kernel mendukung [U] | S9 |
| 10 | ⬜ | `ui-charts` | ui | Grafik bertema token (batang, garis, donat, sparkline, KPI) | Mengikuti `semantic-colors`; dipakai reports dan dasbor modul | S6, S7 |

## Gelombang 2 — Platform lintas bidang

Blok yang dipakai hampir semua jenis aplikasi.

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · kontrak · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 11 | ⬜ | `notifications` | fullstack | Lonceng & kotak masuk, preferensi kanal per pengguna, satu router ke email/WhatsApp | `UserDirectory` (→ users) | `ChannelSender` (→ mailer, whatsapp); kontrak `Notifier` · emits `notifications.message.sent` | | S1, S31 |
| 12 | ⬜ | `whatsapp` | fullstack | Kirim & terima WhatsApp (Cloud API / BSP): registri templat per kategori, opt-in, webhook masuk, status kirim, jendela 24 jam | — | kontrak `ChannelSender`, `Conversations` · kit `provider-http`, `webhook-inbox` · emits `whatsapp.message.received` | Harga per pesan sejak 1 Jul 2025; utility dalam jendela berbayar mulai 1 Okt 2026 [V] | S20 |
| 13 | ⬜ | `approvals` | fullstack | Persetujuan bertingkat untuk dokumen modul lain: ambang nilai, peran, delegasi, SLA, kotak masuk | — | `ApproverResolver` (→ rbac, employees), `Notifier`; kontrak `Approvals` · emits `approvals.request.approved` / `rejected` | Status dokumen tetap milik modul pemiliknya (`docstate`) | S1, S5, S41 |
| 14 | ⬜ | `data-io` | fullstack | Impor/ekspor CSV & XLSX di latar belakang: pemetaan kolom, pratinjau validasi, laporan galat | `FileStore` (→ files) | `Notifier`; registri `ImportTarget`/`ExportSource` dari tiap modul · emits `data-io.job.completed` | File sementara di `DATA_DIR` | S6, S9 |
| 15 | ⬜ | `attachments` | fullstack | Lampiran file pada record modul mana pun, dengan pratinjau dan riwayat versi | `FileStore` (→ files) | `RecordAccess` dari tiap modul; kontrak `Attachments` · emits `attachments.file.attached` | Izin mengikuti record induk | S1, S33 |
| 16 | ⬜ | `comments` | fullstack | Komentar berutas, @mention, linimasa aktivitas pada record modul mana pun | — | `RecordAccess`, `Notifier` · emits `comments.comment.created` | Izin mengikuti record induk | S33 |
| 17 | ⬜ | `tags` | fullstack | Kamus tag per perusahaan dan penanda tag pada record mana pun, filter berdasarkan tag | — | `RecordTypes` dari tiap modul; kontrak `TagLookup` | | S1 |
| 18 | ⬜ | `custom-fields` | fullstack | Field tambahan per jenis record (teks, angka, tanggal, pilihan, wilayah, file) dengan validasi, tampil di form & tabel | — | `RecordTypes`, `RegionLookup`, `FileStore` | Disimpan JSON tervalidasi | S1 |
| 19 | ⬜ | `work-calendar` | fullstack | Libur nasional & cuti bersama per tahun, kalender perusahaan, pola kerja 5/6 hari, jam kerja, hitung hari kerja | — | `Units` (→ organization, zona WIB/WITA/WIT); kontrak `WorkCalendar` | Ditetapkan SKB 3 Menteri tiap tahun [V ada; jumlah hari S] | S28 |
| 20 | ⬜ | `id-regions` | fullstack | Provinsi → kab/kota → kecamatan → desa/kelurahan dengan kode Kemendagri dan kode pos, versi dataset per keputusan, pemilih bertingkat | — | kontrak `RegionLookup` | Kepmendagri 300.2.2-2138/2025 (diubah 2430/2025) [S]; kode desa 10 digit [S] | S29 |
| 21 | ⬜ | `payments` | fullstack | Tagihan bayar lewat gateway: QRIS dinamis, virtual account, e-wallet, link bayar, refund, rekonsiliasi; adaptor per penyedia | — | `PayableResolver`; kontrak `PaymentGateway` · kit `provider-http`, `webhook-inbox` · emits `payments.intent.succeeded` / `expired` / `refunded` | MDR QRIS ditanggung merchant, tidak dibebankan ke konsumen [V] | S15, S16, S17 |
| 22 | ⬜ | `integrations` | fullstack | Kunci API ber-scope (hash + prefix), langganan webhook keluar bertanda tangan HMAC dengan retry, log & replay | — | `EventCatalog` · kit `provider-http` · emits `integrations.webhook.delivered` / `failed` | | S16, S17 |
| 23 | ⬜ | `reports` | fullstack | Dasbor widget (KPI, grafik, tabel), laporan tersimpan, kirim terjadwal via email/PDF | — | `MetricSource` dari tiap modul, `Mailer` · kit `ui-charts`, `pdfdoc` | | S6, S7 |
| 24 | ⬜ | `data-privacy` | fullstack | Catatan persetujuan, kotak masuk permintaan subjek data, register pemrosesan & masa simpan, log insiden kebocoran, purge terjadwal | — | `PersonalDataProvider` dari tiap modul · emits `data-privacy.request.received` | PP 33/2026 berlaku 16 Jan 2027: respons & notifikasi 3×24 jam [S] | S30 |

## Gelombang 3 — Layanan, kantor & publik

Modul siap pakai untuk kantor, sekolah, klinik, komunitas, dan usaha jasa.

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · kontrak · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 25 | ⬜ | `forms` | fullstack | Pembuat formulir & survei: logika, validasi, tautan publik & QR, skor kuis, ekspor jawaban, bisa disematkan di modul lain | — | `RegionLookup`, `FileStore`; kontrak `FormSource` · emits `forms.response.submitted` | Untuk pendaftaran, PPDB, intake | S1 |
| 26 | ⬜ | `booking` | fullstack | Satu mesin reservasi: ruang, lapangan, layanan, dokter — aturan ketersediaan, slot, kapasitas, buffer, DP, pengingat, daftar tunggu | — | `WorkCalendar`, `PaymentGateway`, `Notifier`; kontrak `BookingSource` · emits `booking.reservation.confirmed` / `cancelled` | | S1 |
| 27 | ⬜ | `queue` | fullstack | Antrean: layanan, loket, tiket (kiosk/online/WhatsApp), panggilan + layar + suara, transfer, statistik waktu tunggu | — | `Notifier`, `BookingSource`; kontrak `QueueSource` · kit `ui-live` · emits `queue.ticket.called` | Jembatan antrean Mobile JKN sebagai adaptor nanti [U] | S39 |
| 28 | ⬜ | `visitor-log` | fullstack | Buku tamu digital: check-in kiosk/QR, undangan pra-daftar, foto KTP/selfie terenkripsi, notifikasi tuan rumah, badge, daftar pantau, check-out | `HostDirectory` (→ users, employees) | `Notifier` (→ notifications; MVP: → mailer), `FileStore` (→ files); kontrak `VisitorPersonalData` (dibaca `PersonalDataProvider` milik data-privacy) · kit `barcode`, `fieldcrypt`, `ui-scan`, `ui-print` (badge) · emits `visitor-log.visit.checked-in` | Foto KTP = data pribadi (UU PDP): persetujuan dicatat saat check-in, masa simpan dengan purge terjadwal milik modul sendiri sampai `data-privacy` dipasang | S31, S32 |
| 29 | ⬜ | `helpdesk` | fullstack | Tiket dari portal/email/WhatsApp, tim & penugasan, kebijakan SLA, balasan siap pakai, survei kepuasan | — | `WorkCalendar`, `Mailer`, `ConversationSource` (→ whatsapp), `ArticleSource` (→ knowledge-base); kontrak `TicketSink` · emits `helpdesk.ticket.created` | | S33 |
| 30 | ⬜ | `knowledge-base` | fullstack | Artikel internal & publik, kategori, versi, pencarian, umpan balik "membantu?" | — | `FileStore`, `Approvals`; kontrak `ArticleSource` · emits `knowledge-base.article.published` | SOP internal tanpa website | S33 |
| 31 | ⬜ | `cms` | fullstack | Halaman & seksi, berita/blog, menu, media, SEO & slug, alur terbit | — | `FileStore`, `Approvals`; kontrak `ContentSource` · emits `cms.content.published` | Situs sekolah, desa, klinik, perusahaan | S1 |
| 32 | ⬜ | `crm` | fullstack | Prospek, pipeline, aktivitas & tindak lanjut, konversi ke pelanggan | — | `CustomerDirectory` (→ customers), `Notifier` · emits `crm.deal.won` | | S1, S2, S41 |
| 33 | ⬜ | `events-ticketing` | fullstack | Acara & sesi, jenis & kuota tiket, pendaftaran, tiket berbayar, e-tiket QR, check-in pintu, sertifikat PDF | — | `PaymentGateway`, `FormSource`, `Notifier` · kit `barcode`, `pdfdoc` · emits `events-ticketing.attendee.checked-in` | | S1 |
| 34 | ⬜ | `membership` | fullstack | Paket keanggotaan & perpanjangan, kartu member QR, paket kelas & kuota kunjungan, beku & pengingat kedaluwarsa | — | `CustomerDirectory`, `PaymentGateway`, `BookingSource`, `Notifier` · kit `barcode` · emits `membership.membership.renewed` | Gym, klub, kursus | S1 |
| 35 | ⬜ | `assets` | fullstack | Register aset, label QR, pemegang & lokasi, peminjaman & pengembalian, jadwal & catatan perawatan | — | `HolderDirectory` (→ users, employees), `Notifier`, `FileStore`; kontrak `AssetRegister` · kit `barcode`, `ui-scan` · emits `assets.asset.lent` / `returned`, `assets.maintenance.scheduled` | Peminjaman aset kantor & sekolah tanpa akuntansi | S13 |

## Gelombang 4 — Penjualan & pembelian

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · kontrak · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 36 | ⬜ | `taxrules` | gokit | Semua tarif & perhitungan pajak Indonesia, bertanggal berlaku: PPN (12% × DPP 11/12, kode transaksi), PPh 4(2)/15/22/23 (tarif lebih tinggi tanpa NPWP), PPh 21 (kategori TER A/B/C & tabelnya, PTKP, biaya jabatan, hitung tahunan Pasal 17 untuk Desember/bulan keluar), PPh final pesangon, bea meterai | — | kit `money` | PMK 131/2024 [V]; PMK 168/2023 kategori TER A/B/C [V]; PPh 23 & meterai Rp10.000 di atas Rp5 juta [S]; tabel TER lengkap & PPh final pesangon (PP 68/2009) [U] | S21, S24 |
| 37 | 🟡 | `items` | fullstack | Produk & jasa: varian, kategori, satuan & konversi, SKU/barcode, gambar, kode barang/jasa Coretax | — | `FileStore`, `DocNumbers` (→ numbering); kontrak `ItemCatalog` · emits `items.item.changed` / `archived` | Kode barang wajib di faktur (PER-11/PJ/2025) [S] | S7, S9, S35 |
| 38 | ⬜ | `pricing` | fullstack | Daftar harga per cabang/kanal/kelompok pelanggan, harga bertingkat per jumlah, harga terjadwal, harga pokok vs jual | `ItemCatalog` | `CustomerDirectory`; kontrak `PriceResolver` · emits `pricing.list.changed` | | S9 |
| 39 | ⬜ | `promotions` | fullstack | Promo (%, Rp, beli X gratis Y, bundel, minimum belanja, happy hour) dan voucher/kupon berbatas pemakaian | `ItemCatalog` | `CustomerDirectory`, `LoyaltyTier`; kontrak `PromotionEngine` · emits `promotions.voucher.redeemed` | | S9, S14 |
| 40 | ⬜ | `sales` | fullstack | Penawaran, pesanan penjualan, pengiriman (surat jalan), retur | `CustomerDirectory`, `ItemCatalog` | `PriceResolver`, `PromotionEngine`, `StockReservation`, `Approvals`, `DocNumbers`, `PeriodLock`; kontrak `OrderIntake`, `OrderFulfilment` · emits `sales.order.confirmed`, `sales.delivery.confirmed` | | S1, S2, S7 |
| 41 | ⬜ | `purchasing` | fullstack | Permintaan pembelian, RFQ, PO, penerimaan barang (lot & kedaluwarsa), tagihan pemasok, retur, saran pemesanan ulang | `SupplierDirectory` (→ suppliers), `ItemCatalog` | `Approvals`, `BudgetCheck`, `DocNumbers`, `PeriodLock`; kontrak `PurchaseDocuments` · kit `taxrules` · emits `purchasing.receipt.confirmed`, `purchasing.bill.posted` | PPN masukan, PPh 23 jasa | S1, S41 |
| 42 | ⬜ | `invoicing` | fullstack | Faktur penjualan, nota kredit, faktur uang muka, baris PPN, penanda meterai, cetak PDF | `CustomerDirectory` | `ItemCatalog`, `DocNumbers`, `PeriodLock`; kontrak `InvoiceSource` · kit `taxrules`, `pdfdoc` · emits `invoicing.invoice.posted` / `cancelled` | | S1, S8 |
| 43 | ⬜ | `settlements` | fullstack | Penerimaan & pembayaran piutang/hutang, alokasi ke faktur/tagihan, umur piutang/hutang, termin pembayaran (net-N, cicilan) | `OpenDocuments` (→ invoicing, purchasing) | `CashAccounts`, `PaymentGateway`; kontrak `PaymentTerms` · emits `settlements.payment.received` / `made` | | S2, S7 |

## Gelombang 5 — Gudang, POS & toko

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · kontrak · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 44 | 🟡 | `costing` | gokit | Lapisan FIFO, rata-rata bergerak, konversi satuan di atas `money`/`Qty` | — | — | Dipakai stock, production, bom | S2, S12 |
| 45 | ⬜ | `offline-sync` | gokit | Penerimaan batch dari perangkat offline: UUID klien, kunci idempotensi, urutan per perangkat, kebijakan konflik | — | — | Dipakai pos dan aplikasi lapangan | S9 |
| 46 | ⬜ | `ui-offline` | ui | Antrean kirim IndexedDB, cache katalog & harga, lencana status sinkron | — | — | Service worker PWA mungkin butuh dukungan shell kernel (kernel request) [U] | S9 |
| 47 | 🟡 | `stock` | fullstack | Multi-gudang, lokasi rak (lantai/baris/kolom/rak), buku besar mutasi stok, stok tersedia & reservasi, lot/serial & kedaluwarsa (FEFO), valuasi rata-rata/FIFO, peringatan stok minimum | `ItemCatalog` | `PeriodLock`; kontrak `StockLedger`, `StockReservation` · kit `costing` · emits `stock.level.changed`, `stock.lot.expiry-flagged` | | S9, S10, S12 |
| 48 | ⬜ | `stock-transfer` | fullstack | Transfer antar gudang/cabang dengan status dalam perjalanan, terima sebagian, selisih, penyesuaian & barang rusak | `StockLedger` | `Approvals`, `DocNumbers` · emits `stock-transfer.transfer.shipped` / `received` | Toko satu gudang tidak perlu memasangnya | S9, S10 |
| 49 | ⬜ | `stock-count` | fullstack | Stok opname (penuh/siklus/buta), scan, impor Excel, tinjau selisih, posting penyesuaian | `StockLedger` | menyumbang target impor ke `data-io` · kit `ui-scan` · emits `stock-count.count.posted` | | S9, S12 |
| 50 | ⬜ | `wms` | fullstack | Saran putaway, daftar ambil (per pesanan/SKU/rak; batch/zona), packing, ambil FEFO, isi ulang rak, layar scanner | `StockLedger` | `OrderFulfilment` (→ sales), `ShipmentBooking` (→ shipping) · kit `ui-scan` · emits `wms.order.packed` | | S10 |
| 51 | ⬜ | `pos` | fullstack | Kasir: perangkat, shift (modal awal, kas masuk/keluar, hitung tutup, selisih), transaksi, multi/split pembayaran (tunai, QRIS, kartu), void/refund, struk, sinkron offline | `ItemCatalog` | `PriceResolver`, `PromotionEngine`, `StockLedger`, `PaymentGateway`, `LoyaltyLedger`, `WalletTender`, `CustomerDirectory`; kontrak `PosSales` · kit `taxrules`, `offline-sync`, `ui-print`, `ui-scan`, `ui-offline` · emits `pos.sale.completed` / `voided`, `pos.shift.closed` | QRIS wajib ada; MDR tidak dibebankan ke konsumen [V] | S9, S15, S36 |
| 52 | ⬜ | `pos-fnb` | fullstack | Restoran: denah meja, open bill, gabung/pisah meja & bill, dine-in/takeaway/delivery, self-order QR meja | `PosSales` (→ pos), `ItemCatalog` | · kit `barcode` · emits `pos-fnb.bill.sent-to-kitchen` | Service charge & PBJT restoran [U] | S9, S11 |
| 53 | ⬜ | `kitchen-display` | fullstack | Layar dapur: stasiun per kategori, status per item, bump, layar status pesanan pelanggan, cetak tiket dapur | `ItemCatalog` | listens `pos-fnb.bill.sent-to-kitchen`, `sales.order.confirmed` · kit `ui-live`, `ui-print` · emits `kitchen-display.item.prepared` | Juga untuk pesanan online | S9, S11 |
| 54 | ⬜ | `loyalty` | fullstack | Member, poin (dapat & tukar), tingkat, kedaluwarsa, hadiah (% / Rp / produk gratis) | `CustomerDirectory` | `ItemCatalog`; kontrak `LoyaltyLedger`, `LoyaltyTier` · listens `pos.sale.completed` · emits `loyalty.points.earned` / `redeemed` | | S9, S14 |
| 55 | ⬜ | `customer-wallet` | fullstack | Saldo deposit pelanggan, kartu hadiah, kredit toko, kasbon pelanggan | `CustomerDirectory` | kontrak `WalletTender` · emits `customer-wallet.balance.debited` / `credited` | Laundry, kantin, toko langganan | S14 |
| 56 | ⬜ | `shipping` | fullstack | Ongkir kurir, booking resi, label, lacak, COD; adaptor agregator | — | `OrderFulfilment`, `RegionLookup`; kontrak `ShippingQuote`, `ShipmentBooking` · kit `provider-http`, `webhook-inbox` · emits `shipping.shipment.delivered` | Biteship & RajaOngkir (Komerce) [V] | S18, S19 |
| 57 | ⬜ | `bom` | fullstack | Resep & bill of materials: kit, bertingkat, rendemen, pemotongan bahan otomatis saat penjualan | `ItemCatalog` | `StockLedger`; kontrak `BomSource` · listens `pos.sale.completed` · kit `costing` · emits `bom.materials.consumed` | Kebutuhan utama F&B | S1, S9 |
| 58 | ⬜ | `production` | fullstack | Perintah produksi, operasi & work center, pengeluaran bahan, penerimaan barang jadi, scrap, biaya sederhana | `BomSource` (→ bom), `StockLedger` | `WorkCalendar` · kit `costing` · emits `production.order.completed` | Untuk pabrik & konveksi | S1 |
| 59 | ⬜ | `storefront` | fullstack | Toko & pemesanan online: keranjang, checkout, akun pembeli, halaman produk | `ItemCatalog` | `PriceResolver`, `PromotionEngine`, `PaymentGateway`, `ShippingQuote`, `OrderIntake` (→ sales) · emits `storefront.checkout.completed` | | S9 |
| 60 | ⬜ | `marketplace-sync` | fullstack | Sinkron produk, harga, stok, dan pesanan dengan marketplace & layanan pesan-antar makanan | `ItemCatalog` | `StockLedger`, `OrderIntake` · kit `provider-http`, `webhook-inbox` · emits `marketplace-sync.order.imported` | Tokopedia bergabung ke TikTok Shop Partner Center [S]; GrabFood punya API mitra [S] | S10 |

## Gelombang 6 — Keuangan & pajak

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · kontrak · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 61 | ⬜ | `ledger` | fullstack | Bagan akun, jurnal manual & otomatis (aturan pemetaan dari event `*.posted`), buku besar, neraca saldo, laba rugi, neraca | — | `PeriodLock`, `DocNumbers`; kontrak `Posting`, `LedgerBalances` · emits `ledger.journal.posted` | | S1, S2, S6 |
| 62 | ⬜ | `cash-bank` | fullstack | Akun kas & bank, kas kecil, transfer, impor mutasi rekening, rekonsiliasi bank | — | `Posting` (→ ledger); kontrak `CashAccounts` · emits `cash-bank.statement.reconciled` | | S6, S7, S8 |
| 63 | ⬜ | `budgets` | fullstack | Anggaran per akun/pusat biaya/proyek & periode, realisasi vs anggaran, pengecekan anggaran | — | `ActualsSource` (→ ledger); kontrak `BudgetCheck` | | S6, S41 |
| 64 | ⬜ | `expenses` | fullstack | Biaya perusahaan, klaim reimburse karyawan dengan bukti, uang muka & pertanggungjawabannya | `FileStore` | `EmployeeDirectory`, `Approvals`; kontrak `ClaimsPayable` · emits `expenses.claim.approved` | | S1, S3, S4 |
| 65 | ⬜ | `fixed-assets` | fullstack | Aset tetap akuntansi: buku penyusutan komersial & fiskal, penyusutan bulanan, revaluasi, pelepasan | — | `AssetRegister` (→ assets), `Posting` · emits `fixed-assets.depreciation.posted` | Kelompok 1–4 masa manfaat 4/8/16/20 tahun (PMK 72/2023) [V] | S13, S27 |
| 66 | ⬜ | `coretax` | gokit | Format Coretax DJP: membangun & membaca XML impor faktur pajak dan bukti potong (BPPU, BPMP, BP21, BPA1), kode transaksi, aturan ID TKU per jenis identitas, versi templat bertanggal berlaku | — | kit `taxid`, `taxrules`, `money` | XML menggantikan CSV di Coretax [S]; panjang ID TKU untuk pembeli non-TIN belum pasti [U] | S22, S23 |
| 67 | ⬜ | `efaktur` | fullstack | Register faktur keluaran & masukan, impor/ekspor XML Coretax, status persetujuan, retur/batal | `InvoiceSource` (→ invoicing, purchasing), `CompanyTaxIdentity` (→ settings) | `CounterpartyTaxId` (→ customers, suppliers) · kit `taxrules`, `coretax`, `taxid` · emits `efaktur.faktur.approved` | Nomor otomatis dari Coretax; unggah paling lambat tgl 20 bulan berikutnya [S] | S22 |
| 68 | ⬜ | `ebupot` | fullstack | Register bukti potong & ekspor XML Coretax: BPPU (PPh 4(2)/15/22/23), BPMP (TER bulanan), BP21, BPA1 | — | `WithholdingSource` (→ purchasing, payroll), `EmployeeDirectory` · kit `taxrules`, `coretax`, `taxid` | BPA1 butuh NIK tervalidasi [S] | S23 |
| 69 | ⬜ | `emeterai` | fullstack | Deteksi dokumen wajib meterai, ledger meterai, pembubuhan lewat distributor resmi | `StampProvider` (adaptor distributor Peruri), `FileStore` | `InvoiceSource` · emits `emeterai.document.stamped` | Rp10.000 untuk nilai di atas Rp5 juta (UU 10/2020) [S] | S7, S8 |

## Gelombang 7 — SDM

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · kontrak · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 70 | ⬜ | `payrollrules` | gokit | Aturan ketenagakerjaan bertanggal berlaku: iuran & batas upah BPJS Kesehatan dan Ketenagakerjaan (JHT, JKK, JKM, JP, JKP), upah lembur 1/173 dan pengalinya, THR proporsional, pesangon/UPMK/UPH PP 35/2021, UMK sebagai data | — | kit `money` | Iuran BPJS TK & batas JP [V]; BPJS Kesehatan 4%+1% batas Rp12 juta [S]; tabel pesangon & tarif JKP [U] | S25, S26, S37 |
| 71 | ⬜ | `employees` | fullstack | Data karyawan, unit & jabatan, grade, kontrak PKWT/PKWTT, atasan langsung, NIK/NPWP/PTKP/BPJS/rekening terenkripsi, profil mandiri | — | `Units` (→ organization), `UserAccounts` (→ users), `FileStore`; kontrak `EmployeeDirectory` · kit `taxid`, `fieldcrypt` · emits `employees.employee.hired` / `terminated` | Lingkup "atasan melihat timnya" perlu desain lingkup data (rbac/kernel) | S3, S4, S5 |
| 72 | ⬜ | `attendance` | fullstack | Absen masuk/pulang (GPS geofence, selfie), shift & jadwal, rekap, koreksi absen | `EmployeeDirectory` | `WorkCalendar`, `Approvals`, `FileStore`; kontrak `AttendanceSummary`, `AttendanceHours` · kit `geo` · emits `attendance.period.closed` | | S4, S5 |
| 73 | ⬜ | `overtime` | fullstack | Pengajuan & persetujuan lembur, jam aktual, hitung upah lembur | `EmployeeDirectory` | `AttendanceHours`, `WorkCalendar`, `Approvals`; kontrak `OvertimeHours` · kit `payrollrules` · emits `overtime.request.approved` | Maks. 4 jam/hari, 18 jam/minggu (PP 35/2021) [S] | S26, S37 |
| 74 | ⬜ | `leave` | fullstack | Jenis cuti, akrual, saldo, pengajuan, sisa dibawa ke tahun berikutnya, kalender tim | `EmployeeDirectory` | `WorkCalendar`, `Approvals`, `FileStore` (surat dokter); kontrak `LeaveSummary`, `LeaveBalance` · emits `leave.request.approved` | Cuti tahunan min. 12 hari setelah 12 bulan kerja [S] | S1, S4, S38 |
| 75 | ⬜ | `payroll` | fullstack | Komponen & struktur gaji, periode gaji terkunci, PPh 21 TER, BPJS, THR, slip gaji PDF terenkripsi, file transfer bank, ekspor jurnal | `EmployeeDirectory` | `AttendanceSummary`, `OvertimeHours`, `LeaveSummary`, `ClaimsPayable`, `LoanDeductions`, `PeriodLock`, `Posting`; kontrak `PayrollResults` · kit `taxrules`, `payrollrules`, `money`, `fieldcrypt`, `pdfdoc` · emits `payroll.run.closed` | THR paling lambat 7 hari sebelum hari raya [S] | S3, S4, S5, S24 |
| 76 | ⬜ | `employee-loans` | fullstack | Kasbon & pinjaman karyawan dengan jadwal cicilan potong gaji | `EmployeeDirectory` | `Approvals`; kontrak `LoanDeductions` · listens `payroll.run.closed` · emits `employee-loans.loan.disbursed` | | S4, S5 |
| 77 | ⬜ | `recruitment` | fullstack | Lowongan, pelamar, tahapan seleksi, jadwal wawancara, penawaran kerja | — | `FormSource`, `Notifier`, `EmployeeOnboarding` (→ employees) · emits `recruitment.candidate.hired` | | S1, S4 |
| 78 | ⬜ | `performance` | fullstack | KPI, siklus penilaian, penilaian atasan & 360, rencana pengembangan | `EmployeeDirectory` | `Approvals` · emits `performance.review.completed` | | S1, S4, S5 |
| 79 | ⬜ | `offboarding` | fullstack | Resign/PHK, checklist keluar, pengembalian aset, hitung pesangon & uang pisah | `EmployeeDirectory` | `LeaveBalance`, `AssetReturn` (→ assets) · kit `payrollrules`, `taxrules` · emits `offboarding.exit.completed` | PP 35/2021 Pasal 40 dst. [V ada; tabel U] | S26 |

## Gelombang 8 — Vertikal & lanjutan

| # | Status | Modul | Jenis | Fungsi | Port wajib | Port opsional · kontrak · event | Catatan | Rujukan |
|---|---|---|---|---|---|---|---|---|
| 80 | ⬜ | `school` | fullstack | Peserta didik, rombel, mapel, guru, tahun ajaran & semester, absensi siswa, nilai & rapor PDF, kontak orang tua | — | `Notifier`, `FormSource` (PPDB) · kit `pdfdoc` · emits `school.attendance.recorded` | Adaptor impor Dapodik (web service) [V ada; rincian S] | S40 |
| 81 | ⬜ | `clinic` | fullstack | Pasien (NIK, BPJS), pendaftaran & kunjungan, rekam medis SOAP, diagnosis ICD-10, resep, tenaga medis | — | `QueueSource`, `BookingSource` · kit `fieldcrypt` · emits `clinic.encounter.closed` | Adaptor SATUSEHAT: OAuth2, satu client per Organization [V]; PMK 24/2022 [V] | S39 |
| 82 | ⬜ | `koperasi` | fullstack | Anggota, simpanan pokok/wajib/sukarela, pinjaman & cicilan, SHU | — | `PaymentGateway`, `Approvals`, `Posting` · emits `koperasi.loan.disbursed` | Batas Permenkop 8/2023 & pelaporan PPATK [S] | — |
| 83 | ⬜ | `rental` | fullstack | Sewa barang/alat: kalender ketersediaan, harga per waktu, deposit, ambil & kembali dengan kondisi, denda telat | — | `RentableSource` (→ items, assets), `PaymentGateway`, `WalletTender` · emits `rental.order.returned` | | S1 |
| 84 | ⬜ | `property-rent` | fullstack | Properti/kos: unit & kamar, penyewa (KTP terenkripsi), kontrak, tagihan bulanan, deposit, permintaan perbaikan | — | `PaymentGateway`, `Notifier`, `TicketSink` (→ helpdesk) · kit `fieldcrypt` · emits `property-rent.bill.issued` | | S1 |
| 85 | ⬜ | `projects` | fullstack | Proyek, tugas, papan kanban, timesheet, biaya proyek | — | `EmployeeDirectory`, `CustomerDirectory`, `BudgetCheck` · emits `projects.task.completed` | | S1, S2 |
| 86 | ⬜ | `fleet` | fullstack | Kendaraan, pengemudi, masa berlaku STNK/KIR/asuransi, odometer, BBM, jadwal servis | — | `AssetRegister`, `EmployeeDirectory`, `Notifier` · emits `fleet.document.expiry-flagged` | | S1 |
| 87 | ⬜ | `donations` | fullstack | Kampanye, donatur, donasi, kuitansi PDF, halaman progres, ekspor laporan | — | `PaymentGateway`, `ContentSource` (→ cms) · kit `pdfdoc` · emits `donations.donation.received` | QRIS donasi sosial MDR 0% [V]; laporan Kemensos (Permensos 8/2021) [S] | S15 |
| 88 | ⬜ | `search` | fullstack | Pencarian global lintas modul (Cmd-K), hasil tersaring izin | — | `SearchProvider` dari tiap modul | MySQL FULLTEXT parser ngram | S1 |
| 89 | ⬜ | `esign` | fullstack | Tanda tangan elektronik sederhana (gambar + hash audit) dan port adaptor PSrE | — | `SignatureProvider` (Privy, Peruri, VIDA), `FileStore` · emits `esign.document.signed` | PP 71/2019, Permenkominfo 11/2022 [S] | S1 |

## Menambah modul ke roadmap

Kebutuhan baru yang tidak tertampung modul di atas — dari proyek atau dari riset berikutnya —
ditambahkan sebagai baris baru di gelombang yang sesuai dengan nomor berikutnya (90, 91, …),
setelah lolos uji [Seberapa besar satu modul](#seberapa-besar-satu-modul). Perbarui ringkasan.
Modul yang ternyata tidak dibutuhkan tidak dihapus dari tabel: tandai `✖ dibatalkan` dengan
alasannya.

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
- **S14** Square Loyalty & Gift Cards API — https://developer.squareup.com/docs/loyalty-api/overview · https://developer.squareup.com/docs/gift-cards/using-gift-cards-api
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
