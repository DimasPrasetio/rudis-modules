# taxid

Go kit: Indonesian tax identities. Installed into `apps/api/internal/shared/taxid`; a module that
uses it lists `"go_kits": ["taxid"]`. Standard library only.

It checks **formats**. Whether an NPWP or NIK is registered, or a party is a PKP, only DJP
(Coretax) and Dukcapil can say.

| Function | Does |
|---|---|
| `taxid.Kind` | `npwp`, `nik`, `passport`, `other`, `none` — Coretax's buyer ID types (TIN, National ID, Passport, Other) and no ID; `Valid()`, `TakesNITKU()` (NPWP and NIK: Coretax's TIN) |
| `NormalizeNPWP(s)` | The 16-digit NPWP: 16 digits as given, or a 15-digit old NPWP with a leading `0` (PMK 112/2022). An NPWP not starting with `0` is a resident's NIK and must pass `CheckNIK`; all zeros — Coretax's placeholder for a buyer without an NPWP — is refused. Dots, dashes, and spaces are ignored; letters are refused |
| `IsNIKAsNPWP(npwp16)` | The NPWP is a resident individual's NIK (does not start with `0`) |
| `IdentifiesPerson(kind, id)` | The ID names a natural person — NIK, passport, other document, or an NPWP that is a NIK: mask it (UU 27/2022) |
| `CheckNIK(s)` | 16 digits; a province among the 38 of Kepmendagri 300.2.2-2138/2025 (as amended by 2430/2025), a kabupaten/kota and kecamatan other than `00`; a serial other than `0000`; a real birth date (`DDMMYY`, day + 40 for women). Returns the digits |
| `CheckNITKU(nitku, npwp16)` | 22 digits; with `npwp16`, must start with it. The last six digits are the place's sequence, `000000` the head office |
| `DefaultNITKU(npwp16)` · `NITKUOf(nitku, npwp16)` | The head office's NITKU (NPWP + `000000`) · whether a NITKU belongs to an NPWP |
| `CheckPassport(s)` | 5–20 letters and digits, returned in capitals |
| `CheckOther(s)` | An "Other" document number: 3–30 letters, digits, `. / -`, returned in capitals |
| `SearchForm(q)` | A typed search as stored tax IDs are compared: digits — an old NPWP, whole or its start written the old way (`01.302.442.7`), gets its `0` — or capitals |
| `SuspectNPWP(npwp16)` | An old-format NPWP whose check digit (Luhn over its first nine digits, as python-stdnum checks it) does not match. DJP has not published the algorithm: show a warning, never refuse |
| `Mask(s)` | `3201********0001` — for lists and responses that need not show the whole number (UU PDP) |
| `Digits(s)` | Only the digits of `s` |

Every check returns `taxid.ErrFormat`; the caller turns it into its own field message.

**ID TKU for parties without a NITKU.** Coretax requires a 22-digit NITKU as the buyer's ID TKU
for a TIN buyer; for National ID, Passport, and Other it takes a zero placeholder. Sources differ
on its length (`0000` or `000000`), so the kit leaves it to the e-Faktur export, which must follow
the import template DJP publishes at the time.

Sources: PMK 112/2022 and its amendment PMK 136/2023 (NIK as NPWP, the 16-digit NPWP, NITKU);
PER-11/PJ/2025 art. 33 (buyer identity on a tax invoice); DDTC News on Coretax buyer ID types and
ID TKU (https://news.ddtc.co.id/review/konsultasi-coretax/1808368/muncul-error-nitku-pembeli-tidak-valid-apa-yang-harus-dilakukan,
https://news.ddtc.co.id/berita/nasional/1821714/ini-ketentuan-pengisian-npwpnik-saat-perekaman-fp-untuk-orang-pribadi);
province codes: Kepmendagri 300.2.2-2138/2025 and 300.2.2-2430/2025 via https://github.com/cahyadsn/wilayah.

## Changes

- **0.2.0** — stricter checks and Coretax's "Other" ID: an NPWP not starting with `0` must be a
  valid NIK; all zeros is refused; a NIK's province must be one of the 38, its kabupaten/kota and
  kecamatan not `00`. New: `Kind` `other` with `CheckOther`, `TakesNITKU`, `IsNIKAsNPWP`,
  `IdentifiesPerson`, `DefaultNITKU`, `NITKUOf`, `SearchForm`. Values already stored are not
  rechecked; a module applies the new checks the next time a record is saved.
- **0.1.0** — first version: NPWP, NIK, NITKU, passport checks; the NPWP check digit as a
  warning; masking.
