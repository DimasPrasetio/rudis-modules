# Rencana: persediaan — `costing`, `items`, `stock`

Status: **disetujui maintainer** (Dimas Prasetio, 30 Oktober 2026, semua keputusan D1–D13 sesuai rekomendasi) · branch `persediaan` · ROADMAP #44, #37, #47.

## 1. Tujuan dan batas

Aplikasi pencatatan barang masuk, barang keluar, dan stok saat ini per gudang — untuk UMKM sampai
menengah — dirakit dari tiga bagian:

| Modul | Jenis | Untuk | Bukan untuk (milik modul lain) |
|---|---|---|---|
| `costing` 0.1.0 | gokit | Menilai persediaan: rata-rata bergerak dan FIFO, nilai pengambilan sebagian, pemulihan saat dibatalkan, konversi satuan | Tabel, rute, aturan dokumen (milik `stock`); identifikasi khusus per serial (bersama lot di `stock` 0.2) |
| `items` 0.1.0 | fullstack | Data barang & jasa: kode, nama, jenis, kategori, satuan dasar dan satuan alternatif dengan faktor konversi, barcode, kode barang Coretax opsional; kontrak `ItemCatalog` | Harga jual (`price-lists`), varian (`items` 0.2), gambar (`items` 0.2 lewat `FileStore`), stok (`stock`) |
| `stock` 0.1.0 | fullstack | Gudang; dokumen **penerimaan**, **pengeluaran**, **penyesuaian** (draf → diposting → dibatalkan); buku besar mutasi dengan nilai; saldo per barang × gudang; kartu stok; laporan mutasi (awal, masuk, keluar, akhir); kontrak `StockLedger` | Lot/serial/kedaluwarsa & FEFO (`stock` 0.2), lokasi rak (`stock` 0.3), reservasi (`stock` bersama `sales`), transfer (`stock-transfer`), opname (`stock-count`), pembelian/penjualan (`purchasing`, `sales`), jurnal akuntansi (`ledger`) |

ROADMAP menempatkan `items` di gelombang 4 dan `costing`/`stock` di gelombang 5, sementara
gelombang 1 masih terbuka. Ketiganya tidak memakai modul atau kit dari gelombang 1–3 (port
`stock` hanya ke `items`, `numbering`, `period`, yang sudah ada), jadi dibangun lebih dulu atas
permintaan maintainer (D1).

## 2. Keputusan yang perlu disetujui

| # | Keputusan | Rekomendasi | Alternatif | Sumber |
|---|---|---|---|---|
| D1 | Urutan gelombang | Bangun sekarang, di luar urutan gelombang | Tunggu gelombang 1–4 | Permintaan maintainer; tidak ada ketergantungan ke gelombang 1–3 |
| D2 | Metode biaya | Rata-rata bergerak (default) dan FIFO, **per perusahaan**, terkunci setelah mutasi pertama | Per barang (ERPNext) — lebih rumit tanpa kebutuhan UMKM | PSAK 202 (d/h PSAK 14): FIFO atau rata-rata tertimbang, LIFO dilarang [S]; UU PPh Pasal 10(6) dan penjelasannya: metode dipakai konsisten, ganti butuh persetujuan DJP [S]; ERPNext mengunci metode setelah transaksi [V] |
| D3 | Stok minus | **Dilarang** di 0.1.0 (409 dengan nama barang dan saldonya) | Opsi per perusahaan/barang dengan koreksi nilai saat penerimaan (Odoo) | ERPNext menolak secara default [V]; penilaian stok minus butuh koreksi ulang yang rawan |
| D4 | Posting mundur | Tanggal dokumen **tidak boleh lebih awal** dari mutasi terakhir barang itu di gudang itu; tanpa hitung ulang valuasi | Hitung ulang mutasi setelahnya (Repost Item Valuation ERPNext) — tugas latar besar, ditunda | ERPNext repost [V]; urutan tanggal = urutan posting menjaga saldo berjalan dan nilai tetap benar |
| D5 | Pembatalan dokumen yang sudah diposting | Boleh hanya bila setiap baris masih **mutasi terakhir** barang × gudang itu; menulis baris pembalik (tidak menghapus), lapisan FIFO dipulihkan persis; selain itu diarahkan ke dokumen penyesuaian | Pembatalan bebas + hitung ulang (ikut D4) | ERPNext `is_cancelled` + baris pembalik [V]; Odoo tidak membatalkan transfer selesai, memakai retur [S] |
| D6 | Nomor dokumen | Port **wajib** `Numbers` → `numbering` (`BM/{YYYY}/{MM}/{SEQ:4}`, `BK/…`, `PS/…`), diambil di dalam transaksi posting — tanpa celah | Port opsional dengan penomoran sendiri | Resep posting `docstate` |
| D7 | Tutup buku | Port **opsional** `Periods` → `period`, area `inventory` ("Persediaan") | Wajib | Resep posting `docstate`; port opsional diutamakan (ROADMAP › Aturan) |
| D8 | Penerimaan berbiaya 0 | Ditolak kecuali baris ditandai **tanpa biaya** (bonus/hibah); dicatat di audit | Bebas | ERPNext "Allow Zero Valuation Rate" per baris [V] |
| D9 | Satuan | Satuan dasar per barang **tidak bisa diubah** setelah barang dibuat; satuan alternatif dengan faktor ke satuan dasar, faktor tidak bisa diubah (nonaktifkan lalu buat baru); dokumen menyalin faktor per baris | Boleh diubah selama belum dipakai — butuh port `items` → `stock` (port terlambat) | ERPNext: satuan stok tidak bisa diubah setelah ada transaksi [V]; ledger selalu dalam satuan dasar [V] |
| D10 | Coretax | `items` menyimpan jenis (barang = A, jasa = B), **kode barang/jasa 6 digit opsional** (default `000000`), dan kode satuan `UM.nnnn` opsional per satuan — hanya **diperiksa formatnya**. Daftar resmi kode barang (±1.967) dan satuan (UM.0001–UM.0039) milik kit `coretax` nanti | Menanam daftar kode di `items` — melanggar "satu aturan di satu kit" | PER-11/PJ/2025 Pasal 33 tidak mewajibkan kode barang [V]; DJP: "tidak wajib diisi, jika diisi harus sesuai referensi", `000000` boleh [S]; struktur XML `GoodService` Opt/Code/Unit [V] (kode Odoo `l10n_id_efaktur_coretax`) |
| D11 | Asal/tujuan barang | Teks bebas **Dari/Kepada** dan **Referensi** (no. surat jalan/PO) di 0.1.0 | Port opsional ke `suppliers`/`customers` | Menjaga 0.1.0 kecil; port bisa ditambah di versi minor |
| D12 | Gudang dan cabang | Gudang boleh terikat ke satu cabang (`branch_id`, 0 = seluruh perusahaan); izin dokumen diperiksa untuk cabang gudangnya (`d.Authz.Units`) | Gudang tanpa cabang | Pola `numbering`/`period` |
| D13 | Varian dan gambar barang | Tidak di 0.1.0 (masuk `items` 0.2) | Sekarang | Zoho item groups [V] — fitur besar tersendiri |

## 3. Desain

### 3.1 `costing` (gokit, `lib/` → `internal/shared/costing`)

Murni hitung, hanya standard library dan kit `money`. Nilai selalu `money.Amount`, kuantitas
`money.Qty` dalam satuan dasar.

```go
type Method string // "average" | "fifo"

// Position adalah saldo satu barang × gudang: rata-rata memakai Qty/Value; FIFO memakai Layers.
type Position struct {
	Qty    money.Qty
	Value  money.Amount
	Layers []Layer // FIFO, tertua dulu
}
type Layer struct{ ID int64; Qty, Remaining money.Qty; Value, RemainingValue money.Amount }

// Take adalah pengambilan: nilai yang keluar dan lapisan yang tersentuh (untuk pemulihan).
type Take struct{ LayerID int64; Qty money.Qty; Value money.Amount }

func Receive(p Position, m Method, qty money.Qty, value money.Amount) (Position, error)
func Issue(p Position, m Method, qty money.Qty) (Position, money.Amount, []Take, error) // ErrInsufficient
func Convert(qty money.Qty, factor money.Rate) (money.Qty, error)                    // ke satuan dasar
func UnitCost(p Position) (money.Amount, bool)                                       // nilai/qty, untuk tampilan dan penyesuaian +
```

- **Tanpa sisa sen:** nilai yang keluar = `Value × qty / Qty` (rata-rata) atau
  `RemainingValue × qty / Remaining` (lapisan FIFO), dibulatkan sekali half-up; mengambil seluruh
  sisa mengambil seluruh nilai — saldo qty 0 selalu bernilai 0.
- Perkalian rasio dihitung eksak dengan `math/big` dari bentuk kanonik `money`.

### 3.2 `items` (fullstack, scoped)

Tabel (migrasi `001_create_items`):

```sql
CREATE TABLE itm_units (            -- satuan milik perusahaan: PCS, KG, DUS …
  id BIGINT NOT NULL AUTO_INCREMENT, company_id BIGINT NOT NULL,
  code VARCHAR(10) NOT NULL, name VARCHAR(60) NOT NULL,
  tax_unit_code CHAR(7) NOT NULL DEFAULT '',          -- 'UM.0021' atau ''
  is_active BOOLEAN NOT NULL DEFAULT TRUE, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id), UNIQUE KEY itm_units_code (company_id, code));
CREATE TABLE itm_categories (
  id BIGINT NOT NULL AUTO_INCREMENT, company_id BIGINT NOT NULL,
  name VARCHAR(100) NOT NULL, is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id), UNIQUE KEY itm_categories_name (company_id, name));
CREATE TABLE itm_items (
  id BIGINT NOT NULL AUTO_INCREMENT, company_id BIGINT NOT NULL,
  code VARCHAR(30) NOT NULL, name VARCHAR(150) NOT NULL,
  kind VARCHAR(8) NOT NULL,                            -- goods | service (tetap)
  is_stocked BOOLEAN NOT NULL,                         -- dilacak stoknya (tetap; jasa selalu FALSE)
  category_id BIGINT NULL, base_unit_id BIGINT NOT NULL,  -- satuan dasar (tetap)
  barcode VARCHAR(64) NULL, tax_code CHAR(6) NOT NULL DEFAULT '000000',
  description TEXT NULL, is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_by BIGINT NOT NULL, created_at DATETIME(6) NOT NULL, updated_by BIGINT NOT NULL, updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id), UNIQUE KEY itm_items_code (company_id, code), UNIQUE KEY itm_items_barcode (company_id, barcode),
  KEY itm_items_name (company_id, is_active, name));
CREATE TABLE itm_item_units (       -- satuan alternatif: 1 DUS = 12 PCS
  id BIGINT NOT NULL AUTO_INCREMENT, company_id BIGINT NOT NULL, item_id BIGINT NOT NULL, unit_id BIGINT NOT NULL,
  factor DECIMAL(19,10) NOT NULL,                      -- berapa satuan dasar (tetap)
  is_active BOOLEAN NOT NULL DEFAULT TRUE, created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id), UNIQUE KEY itm_item_units_unit (company_id, item_id, unit_id));
CREATE TABLE itm_settings (company_id BIGINT NOT NULL, code_prefix VARCHAR(10) NOT NULL DEFAULT 'BRG-',
  last_number BIGINT NOT NULL DEFAULT 0, updated_at DATETIME(6) NOT NULL, PRIMARY KEY (company_id));
```

Kontrak:

```go
type Item struct { ID int64; Code, Name, Kind string; IsStocked bool; BaseUnit string; IsActive bool }
type Unit struct { Code string; Factor money.Rate } // Factor 1 untuk satuan dasar
type ItemCatalog interface {
	Get(ctx context.Context, id int64) (Item, error)                         // aktif atau tidak
	Search(ctx context.Context, q string, limit int) ([]Item, error)        // aktif, untuk pemilih
	// Usable dipanggil dokumen di dalam d.Tx-nya: nil, ErrNotFound, ErrInactive. Baris barang
	// ditahan shared sampai transaksi selesai (nonaktifkan menunggu posting, dan sebaliknya).
	Usable(ctx context.Context, id int64) error
	// Unit adalah faktor satuan code ("" = satuan dasar) ke satuan dasar: ErrUnknownUnit bila
	// bukan satuan barang itu atau sudah nonaktif.
	Unit(ctx context.Context, id int64, code string) (Unit, error)
}
```

Izin: `items.read` (melihat barang), `items.manage` (menambah, mengubah, menonaktifkan barang,
satuan, dan kategori). Event: `items.item.created`, `.changed`, `.deactivated`, `.activated` —
`{id, company_id, code, name}` (+ `kind` saat dibuat).

API: `GET/POST /api/v1/items`, `GET /api/v1/items/lookup`, `GET/PUT /api/v1/items/{id}`,
`POST …/{id}/deactivate|activate`, `POST …/{id}/units`, `POST …/{id}/units/{uid}/deactivate`,
`GET/POST /api/v1/items/units`, `PUT /api/v1/items/units/{id}`, `GET/POST /api/v1/items/categories`,
`PUT /api/v1/items/categories/{id}`, `GET/PUT /api/v1/items/settings`.

### 3.3 `stock` (fullstack, scoped)

Tabel (migrasi `001_create_stock`):

```sql
CREATE TABLE stk_settings (company_id BIGINT NOT NULL, cost_method VARCHAR(8) NOT NULL DEFAULT 'average',
  updated_at DATETIME(6) NOT NULL, PRIMARY KEY (company_id));
CREATE TABLE stk_warehouses (id, company_id, branch_id BIGINT NOT NULL DEFAULT 0, code VARCHAR(20), name VARCHAR(100),
  is_active, created_at, updated_at, UNIQUE (company_id, code));
CREATE TABLE stk_documents (id, company_id, branch_id, kind VARCHAR(10) /* receipt|issue|adjustment */,
  warehouse_id, doc_date DATE, number VARCHAR(40) NULL, counterparty VARCHAR(150), reference VARCHAR(60),
  notes TEXT NULL, <docstate.Fields>, created_by/at, updated_by/at,
  UNIQUE (company_id, number), UNIQUE (company_id, amended_from), KEY (company_id, kind, status, doc_date));
CREATE TABLE stk_document_lines (id, company_id, document_id, line_no INT, item_id, item_code, item_name, -- disalin saat posting
  unit_code VARCHAR(10), factor DECIMAL(19,10), qty DECIMAL(19,4) /* bertanda untuk penyesuaian */,
  base_qty DECIMAL(19,4), unit_cost DECIMAL(19,2) NULL, zero_cost BOOLEAN, value DECIMAL(19,2) NULL /* diisi saat posting */);
CREATE TABLE stk_moves (            -- buku besar mutasi: hanya ditambah
  id BIGINT AUTO_INCREMENT /* urutan */, company_id, warehouse_id, item_id, document_id, line_id, move_date DATE,
  qty DECIMAL(19,4) /* + masuk, − keluar */, value DECIMAL(19,2), qty_after DECIMAL(19,4), value_after DECIMAL(19,2),
  reverses BIGINT NULL /* mutasi yang dibalik */, created_at,
  KEY (company_id, item_id, warehouse_id, id), KEY (company_id, warehouse_id, move_date));
CREATE TABLE stk_balances (company_id, warehouse_id, item_id, qty DECIMAL(19,4), value DECIMAL(19,2),
  last_move_id BIGINT, last_date DATE, PRIMARY KEY (company_id, warehouse_id, item_id));
CREATE TABLE stk_layers (id, company_id, warehouse_id, item_id, move_id, layer_date DATE,
  qty DECIMAL(19,4), value DECIMAL(19,2), remaining DECIMAL(19,4), remaining_value DECIMAL(19,2),
  KEY (company_id, warehouse_id, item_id, remaining, id));   -- FIFO
CREATE TABLE stk_layer_takes (company_id, move_id, layer_id, qty DECIMAL(19,4), value DECIMAL(19,2),
  PRIMARY KEY (company_id, move_id, layer_id));             -- untuk memulihkan lapisan saat dibatalkan
```

Posting (satu `d.Tx.Do`, resep `docstate`): kunci dokumen `FOR UPDATE` → `docstate.Check` →
izin untuk cabang gudang → `Periods.EnsureOpen` (bila disambung) → per baris (urut item_id agar
kunci berurutan): `ItemCatalog.Usable`, `Unit` (faktor), kunci saldo (`INSERT IGNORE` lalu
`SELECT … FOR UPDATE`), tolak tanggal < `last_date` (D4), `costing.Receive/Issue`, tulis
`stk_moves`, lapisan dan pengambilan FIFO, saldo → kunci metode biaya (baris `stk_settings`
dibuat bila belum ada) → `Numbers.Next` (terakhir) → `UPDATE … AND status = 'draft'` +
`docstate.Guard` → audit → event `stock.document.posted` dan `stock.level.changed` per saldo.

Pembatalan: kunci, `Check(Cancel)`, `CancelReason`, `EnsureOpen` tanggal dokumen, per baris
saldo `FOR UPDATE` dan `last_move_id` harus mutasi baris itu (D5), tulis mutasi pembalik
(`reverses`), pulihkan saldo dan lapisan, `stock.document.cancelled`. Amend dari `docstate`.

Kontrak `StockLedger` (untuk `purchasing`, `sales`, `pos` nanti — dipanggil di dalam `d.Tx` mereka):

```go
type Movement struct { WarehouseID, ItemID int64; Date kernel.Date; Qty money.Qty /* satuan dasar, bertanda */;
	UnitCost *money.Amount /* masuk; nil = biaya rata-rata saat ini */; Source string /* "sales.delivery:42" */ }
type StockLedger interface {
	OnHand(ctx context.Context, warehouseID, itemID int64) (money.Qty, error)
	Warehouses(ctx context.Context) ([]Warehouse, error)
	Post(ctx context.Context, m Movement) (Posted, error)   // nilai keluar dihitung costing; ErrInsufficient, ErrBackdated
	Reverse(ctx context.Context, source string) error        // aturan D5
}
```

Izin: `stock.read` (melihat stok, dokumen, kartu stok, laporan), `stock.draft` (membuat dan mengubah
draf), `stock.post` (memposting), `stock.cancel` (membatalkan dokumen terposting, dengan alasan),
`stock.manage` (gudang dan metode biaya — seluruh perusahaan). Event: `stock.document.posted`,
`stock.document.cancelled` (`{id, company_id, kind, number, warehouse_id}`), `stock.level.changed`
(`{company_id, warehouse_id, item_id, qty}`).

### 3.4 Halaman dan brief desain

Grup navigasi **"Persediaan"** (urutan 300–340) dan satu halaman di **"Pengaturan"** (830):

| Rute | Halaman | Nav |
|---|---|---|
| `/items` | Barang & jasa: daftar, cari kode/nama/barcode, filter jenis/kategori/status; lembar form; dialog Satuan dan Kategori | Persediaan › Barang, 300 |
| `/items/:id` | Detail barang: data, satuan alternatif, tautan ke kartu stok (slot `items:detail-actions` tidak dipakai — tautan biasa) | — |
| `/stock` | Stok saat ini: barang × gudang, qty dalam satuan dasar, nilai, biaya rata-rata; filter gudang/kategori, "hanya yang ada stok" | Persediaan › Stok, 310 |
| `/stock/documents` | Dokumen stok: tab Penerimaan / Pengeluaran / Penyesuaian, filter status/gudang/tanggal | Persediaan › Dokumen stok, 320 |
| `/stock/documents/new?kind=…`, `/stock/documents/:id` | Form dokumen: kepala (gudang, tanggal, dari/kepada, referensi, catatan) + `LineItemsEditor` (barang, satuan, qty, biaya satuan untuk masuk); Simpan draf / Posting / Batalkan (alasan) / Koreksi | — |
| `/stock/items/:id` | Kartu stok: saldo awal, mutasi (tanggal, no. dokumen sebagai tautan, masuk, keluar, saldo qty, nilai, saldo nilai), filter gudang dan periode | — |
| `/stock/report` | Laporan mutasi: per barang × gudang untuk periode — awal, masuk, keluar, akhir (qty dan nilai) | Persediaan › Laporan mutasi, 330 |
| `/stock/warehouses` | Gudang dan metode biaya (terkunci setelah mutasi pertama) | Pengaturan › Gudang, 830 |

Brief (dari riset 30 Oktober 2026):

- **Daftar** mengikuti NN/g data tables dan Carbon: header tetap, kolom identitas = tautan,
  filter aktif terlihat, toolbar ≤ 5 aksi, pagination di bawah, tata letak HP berupa kartu (kit
  `ui-data` sudah melakukannya).
- **Stok saat ini dan laporan mutasi** meniru kolom ERPNext Stock Balance dan format lokal
  Accurate/Jurnal: awal, masuk, keluar, akhir, nilai; kartu stok meniru ERPNext Stock Ledger
  (masuk, keluar, saldo qty, nilai, no. dokumen sebagai tautan).
- **Dokumen** mengikuti ERPNext Stock Entry dan status Odoo: badge status Draf/Diposting/
  Dibatalkan, posting dengan konfirmasi, batal dengan `ReasonDialog`, tidak pernah hapus setelah
  posting.
- **Form**: satu kolom untuk kepala, label di atas field, opsional ditandai, `ErrorSummary` di atas
  dan pesan di samping field (GOV.UK, NN/g), penjaga perubahan belum disimpan.
- **Angka**: qty memakai `MoneyInput` tanpa awalan dengan 4 desimal; biaya `MoneyInput` Rupiah;
  tanggal `DatePicker` dd/mm/yyyy.
- Komponen kit yang dipakai: `Page`, `PageHeader`, `SectionCard`, `DataTable`, `FilterBar`,
  `NavTabs`, `StatusBadge`, `FormSheet`, `FormField`, `ErrorSummary`, `LineItemsEditor`,
  `AsyncSearchSelect`, `MoneyInput`, `DatePicker`, `DateRangePicker`, `ReasonDialog`,
  `ConfirmDialog`, `Notice`, `EmptyState`. **Tidak ada komponen kit baru** yang dibutuhkan; bila
  ternyata kurang saat membangun, ditambahkan ke kit dan `ui-gallery` dalam perubahan sendiri.
- Dihindari: menghapus dokumen terposting (Zoho), lokasi sebagai pohon folder (Sortly), edit
  sebaris di tabel lebar dan modal yang menutupi baris (NN/g).

## 4. Syarat wajib

| # | Jawaban |
|---|---|
| A1 | ✓ `items` dan `stock` `"scoped": true`; `company_id` di setiap tabel, kunci, dan query |
| A2 | Tidak berlaku — tidak ada catatan tingkat instalasi |
| A3, A4 | Tidak berlaku — tidak menyimpan identitas orang (Dari/Kepada adalah nama pihak, bukan nomor identitas) |
| A5 | Tidak berlaku di 0.1.0 — tanpa berkas |
| A6 | ✓ Pencarian `LIKE` di-escape (`%`, `_`, `\`), seperti `files` |
| B7, B8, B10 | Tidak berlaku |
| B9 | ✓ Mengubah metode biaya dicatat `security` di dalam transaksinya |
| C11 | ✓ Posting, pembatalan, kode otomatis, dan saldo: `SELECT … FOR UPDATE` lalu tulis, audit dan event di transaksi yang sama |
| C12 | ✓ `PUT` barang mengubah hanya field yang dikirim (pointer di DTO); satuan dasar, jenis, dan `is_stocked` tetap |
| C13 | Tidak berlaku — tidak ada efek keluar sistem |
| C14 | Tidak berlaku |
| C15 | ✓ Kode, nama, satuan, dan faktor barang disalin ke baris dokumen saat posting; dokumen lama tetap menampilkan yang berlaku saat itu |
| D16 | ✓ Tidak ada tarif; kode Coretax hanya diperiksa formatnya, daftar resmi milik kit `coretax` (D10) |
| D17 | Tidak berlaku |
| E18 | ✓ `items.item.*`; `stock.document.posted/cancelled`, `stock.level.changed` |
| E19 | ✓ `ItemCatalog` dan `StockLedger` sejak 0.1.0 |
| E20 | ✓ `stock` memeriksa `ItemCatalog.Usable` di dalam transaksi posting (barang nonaktif ditolak, tidak basi) |
| F21 | ✓ Brief bersumber di § 3.4 |
| F22 | ✓ Hanya komponen kit (§ 3.4) |
| F23 | ✓ Dibuktikan di § 9 Hasil: fokus, `PageHeader`, label, `ErrorSummary`, pengumuman status, target ≥24 px |
| F24 | ✓ `MoneyInput` untuk biaya dan qty, `DatePicker`, tanpa `Number()`/`parseFloat` |
| F25 | ✓ `DataTable`: identitas tautan, state tidak menghilangkan header, kartu di HP |
| F26 | ✓ Screenshot desktop ≥1280 px dan HP 375 px, keyboard, konsol — di § 9 |
| G27 | ✓ Cek testbed untuk setiap aturan di atas, termasuk isolasi dua perusahaan |
| G28 | ✓ Testbed penuh sebelum rilis, dicatat di STATUS.md |

## 5. Cara modul lain memakainya

- `stock` → `items` lewat port `Items` (adaptor `adapters/items_items.go` ke `ItemCatalog`).
- `stock` → `numbering` lewat port `Numbers`; → `period` lewat port `Periods` (opsional).
- Modul berikutnya (`purchasing`, `sales`, `pos`, `stock-count`, `stock-transfer`) memakai
  `StockLedger` lewat port masing-masing; `price-lists`, `invoicing`, `pos` memakai `ItemCatalog`.
- Tidak ada import antarmodul.

## 6. Pengujian

- **Unit (`costing`)**: rata-rata bergerak, FIFO lintas lapisan, pengambilan sebagian tanpa sisa
  sen, ambil habis → nilai 0, stok kurang → `ErrInsufficient`, konversi satuan, overflow.
- **Unit (`items`, `stock` dengan `kerneltest`)**: validasi, kode otomatis, field tetap, satuan;
  posting/pembatalan, D3–D5, D8, izin per cabang, event dan audit.
- **Testbed — skenario baru `inventory`** (app-shell, users, auth, rbac, organization, numbering,
  period, items, stock; port 18103–18104):
  - barang: kode otomatis, barcode unik, satuan alternatif, satuan dasar tidak bisa diubah, nonaktif;
  - penerimaan dua harga → rata-rata benar; pengeluaran → nilai keluar dan saldo; FIFO di
    perusahaan kedua;
  - stok minus ditolak (409); tanggal mundur ditolak (409); biaya 0 tanpa tanda ditolak (422);
  - nomor tanpa celah, rollback mengembalikan nomor; periode tertutup menolak (409);
  - pembatalan memulihkan saldo dan lapisan; pembatalan yang bukan mutasi terakhir ditolak;
  - izin: tanpa `stock.post` → 403; cabang lain → 403;
  - isolasi: barang dan gudang perusahaan A tidak terlihat dan tidak bisa diposting dari B;
  - kartu stok dan laporan mutasi cocok dengan isi `stk_moves` di MySQL;
  - event `stock.document.posted` hanya setelah commit.

## 7. Risiko yang diverifikasi saat membangun

- sqlc dan `DECIMAL` (string), `INSERT IGNORE` + `FOR UPDATE` untuk saldo baru, urutan kunci
  antar-baris (deadlock dicoba ulang `d.Tx`, tapi dihindari dengan urut item_id).
- Pembagian rasio eksak di `costing` tanpa melewati 19 digit.
- `LineItemsEditor` cukup untuk baris dengan pemilih barang asinkron dan pilihan satuan per baris.
- Kunci shared `Usable` (items) dan kunci saldo (stock) dalam satu transaksi — urutan tetap.

## 8. Urutan kerja

1. `costing` 0.1.0 (kit, tes unit).
2. `items` 0.1.0 (API, tes unit, web).
3. `stock` 0.1.0 (API, adaptor ke items/numbering/period, tes unit, web).
4. Skenario testbed `inventory`; sandbox dan verifikasi visual.
5. Catalog, STATUS.md, ROADMAP, bagian Hasil di rencana ini; testbed penuh; PR.

## 9. Hasil pembangunan

_Diisi setelah membangun._
