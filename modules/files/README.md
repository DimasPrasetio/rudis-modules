# files

Stores uploaded files for other modules. Fullstack (Go + React), owns `files`.

**How a module uses it.** It declares its own port (e.g. settings' `FileStore`) and an adapter in
the composition root maps it to `contracts.FileClient` — `Save`, `Get`, `Open`, `Delete`. The
module serves its files through **its own endpoint**, deciding who may see them; files has no
public URL of its own.

**Storing.** `Save` reads the first 512 bytes and decides the type from the **content**, not the
name — a `.png` holding HTML is HTML, and refused. Allowed by default: PNG, JPEG, WebP, GIF, PDF,
Word/Excel/PowerPoint (.docx/.xlsx/.pptx), plain text and CSV. **Never SVG or HTML** (script in the
app's origin). A module narrows the types and the size for its own uploads (`Upload.Allowed`,
`MaxBytes`). The bytes stream to `DATA_DIR/files/<yyyy>/<mm>/<random>` — never into memory or
`/tmp`, which is RAM in the container — through a `.part` file renamed on completion; size and
SHA-256 are computed on the way. A refused upload leaves nothing behind. Names are cleaned (no
path, no control characters) and given the extension of the type the bytes are (a PDF named
`laporan.html` is stored as `laporan.pdf`). An Office document must be an Office package: a ZIP
named `.docx` without one inside is refused.

**Who sees a file.** The module that stored it — it serves the file through its own endpoint and
decides who may. The administration page (`/files`, `files.read`) shows only files this module
stores itself and files whose owner named a **read permission** the viewer also holds
(`Upload.ReadPermission`, e.g. `"expenses.read"`). A file stored without one — the default, and
right for personal data such as an ID scan or a payslip — is never listed, downloaded, or deleted
there: the same 404 as a file that does not exist.

**Deleting** takes a file out of use at once; its bytes stay 30 days (recoverable from disk and
backups), then a daily job (`d.Jobs`) removes them, abandoned partial writes, and bytes older than
a day that no row refers to — a save writes the bytes first and its row in the caller's
transaction, so a rollback leaves them behind. Bytes any row refers to, deleted or not, are only
removed on the 30-day schedule.

**Server.** `DATA_DIR` is the volume `~/<slug>/data` (deploy.sh hands it to uid 10001); the
server's nightly backup snapshots it with the database. The API does not start if the folder is
not writable. The server's read and write timeouts (30 s / 60 s) are sized for API calls: the
download route gives itself time for the file's size, and a module that accepts uploads or
serves its own files does the same — `request.AllowTransfer(w, maxBytes)` (kernel 5) at the top
of the handler.

**Kernel 5:** `Config.DataDir`, `d.Jobs`, `d.Audit` (administrator deletions), `request.AllowTransfer`.

**Web:** `/files` — "Berkas": the files its viewer may read (above) with their module, type,
size; download (as an attachment — never rendered in the app) and delete (`files.manage`). Nav
entry in "Pengaturan" (`files.read`).

**Not in this version:** thumbnails or previews, object storage/CDN, quotas, virus scanning.

## Changes

- **0.3.0** — the administration page no longer reaches every module's files: only the files
  module's own and those whose owner set the new `Upload.ReadPermission` that the viewer holds
  (migration 003; existing files have none, so they are reached only through their owner
  module). A daily sweep removes stored bytes no row refers to (a save rolled back with its
  transaction). Stored names take the extension of the detected type; a ZIP named
  .docx/.xlsx/.pptx without an Office package is refused; the search matches `%` and `_`
  literally. `FileClient` is unchanged for existing callers (a new optional field). Grounds:
  OWASP File Upload Cheat Sheet (validate type by content, align the extension, keep access with
  the owner — https://cheatsheetseries.owasp.org/cheatsheets/File_Upload_Cheat_Sheet.html) and
  UU PDP 27/2022 (personal data seen only by those entitled to it).

- **0.2.0** — needs kernel 6. Scoped: files belong to a company (migration 002), and every
  call works in the ctx's company. `FileClient.Lock` keeps a file attached to a posted document
  for good — `Delete` then answers `ErrLocked`; the admin page marks locked files.
- **0.1.0** — first version.
