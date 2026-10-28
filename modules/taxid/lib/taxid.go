// Package taxid checks and normalizes Indonesian tax identities: the 16-digit NPWP, the NIK that
// serves as an individual's NPWP, the 22-digit NITKU of a place of business, and passport
// numbers for foreign parties. It checks formats only — whether a number is registered, or a
// party is a PKP, can be confirmed only by DJP (Coretax) and Dukcapil.
package taxid

import (
	"errors"
	"strings"
	"time"
)

// Kind is the type of a party's tax identity. Coretax names a buyer's ID TIN (an NPWP or a NIK
// used as one), National ID (a NIK), Passport, or Other; None is a party with no ID at all.
type Kind string

const (
	NPWP     Kind = "npwp"     // 16 digits: companies, government bodies, non-resident individuals — or a resident's NIK used as NPWP
	NIK      Kind = "nik"      // 16 digits: resident individuals (the NIK is their NPWP, PMK 112/2022 as amended by PMK 136/2023)
	Passport Kind = "passport" // foreign individuals
	Other    Kind = "other"    // another identity document Coretax accepts as "Other" (e.g. a KITAS number)
	None     Kind = "none"     // foreign companies, or no tax ID: name and address only
)

// Valid reports whether k is one of the kinds above.
func (k Kind) Valid() bool { return k == NPWP || k == NIK || k == Passport || k == Other || k == None }

// TakesNITKU reports whether a party of kind k has places of business with a NITKU: an NPWP or a
// NIK used as one (Coretax's TIN). The other kinds have none; Coretax fills the buyer's ID TKU
// with zeros for them (an e-Faktur export writes that placeholder, never this kit).
func (k Kind) TakesNITKU() bool { return k == NPWP || k == NIK }

// ErrFormat is a number that does not have the shape its kind requires.
var ErrFormat = errors.New("taxid: wrong format")

// Digits keeps only the digits of s — "01.234.567.8-901.000" becomes "012345678901000".
func Digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// onlyDigitsAndPunctuation is s holding nothing but digits and the separators people write in
// an NPWP — so "12a3…" is refused rather than read as "123…".
func onlyDigitsAndPunctuation(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9') && !strings.ContainsRune(" .-/", r) {
			return false
		}
	}
	return true
}

// NormalizeNPWP returns the 16-digit NPWP of s: 16 digits as given, or a 15-digit NPWP of the
// old format prefixed with "0" (PMK 112/2022). Dots, dashes, and spaces are ignored.
//
// A 16-digit NPWP that does not start with "0" is a resident individual's NIK used as NPWP
// (PMK 112/2022 as amended by PMK 136/2023), so it must pass CheckNIK. All zeros is refused: it is
// the placeholder Coretax writes for a buyer without an NPWP, not a tax ID. Anything else is
// ErrFormat.
func NormalizeNPWP(s string) (string, error) {
	if !onlyDigitsAndPunctuation(s) {
		return "", ErrFormat
	}
	d := Digits(s)
	switch len(d) {
	case 15:
		d = "0" + d
	case 16:
	default:
		return "", ErrFormat
	}
	if strings.Trim(d, "0") == "" {
		return "", ErrFormat
	}
	if IsNIKAsNPWP(d) {
		if _, err := CheckNIK(d); err != nil {
			return "", ErrFormat
		}
	}
	return d, nil
}

// IsNIKAsNPWP reports whether a 16-digit NPWP is a resident individual's NIK: an NPWP of a
// company, a government body, or a non-resident is the old 15 digits with a leading "0".
func IsNIKAsNPWP(npwp16 string) bool { return len(npwp16) == 16 && npwp16[0] != '0' }

// IdentifiesPerson reports whether a tax ID of kind k names a natural person — personal data
// under UU 27/2022 that lists and responses mask: a NIK, a passport, another identity document,
// or an NPWP that is a NIK.
func IdentifiesPerson(k Kind, id string) bool {
	switch k {
	case NIK, Passport, Other:
		return true
	case NPWP:
		return IsNIKAsNPWP(id)
	}
	return false
}

// provinces are the 38 province codes of Kepmendagri 300.2.2-2138/2025 as amended by
// 300.2.2-2430/2025 (checked 28 Oct 2026 against the cahyadsn/wilayah dataset that follows it).
// A NIK keeps the province it was issued in, so 91–96 all occur: a NIK from before Papua was
// divided (2022) still starts with 91 or 92.
var provinces = map[string]bool{
	"11": true, "12": true, "13": true, "14": true, "15": true, "16": true, "17": true, "18": true, "19": true,
	"21": true, "31": true, "32": true, "33": true, "34": true, "35": true, "36": true,
	"51": true, "52": true, "53": true, "61": true, "62": true, "63": true, "64": true, "65": true,
	"71": true, "72": true, "73": true, "74": true, "75": true, "76": true, "81": true, "82": true,
	"91": true, "92": true, "93": true, "94": true, "95": true, "96": true,
}

// CheckNIK reports a NIK that is not 16 digits; whose region — province (one of the 38 codes),
// kabupaten/kota, kecamatan, two digits each — is missing; whose serial is 0000; or whose birth
// date (DDMMYY, day + 40 for women) is no real date. It cannot tell whether the NIK exists: only
// Dukcapil can. Returns the NIK's digits.
func CheckNIK(s string) (string, error) {
	if !onlyDigitsAndPunctuation(s) {
		return "", ErrFormat
	}
	d := Digits(s)
	if len(d) != 16 || !provinces[d[:2]] || d[2:4] == "00" || d[4:6] == "00" || d[12:] == "0000" {
		return "", ErrFormat
	}
	day, month := num(d[6:8]), num(d[8:10])
	if day > 40 {
		day -= 40
	}
	// The year has two digits; 2000 is a leap year, so 29 February is a real date for "00".
	year := 2000 + num(d[10:12])
	if month < 1 || month > 12 || day < 1 || day > time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		return "", ErrFormat
	}
	return d, nil
}

func num(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// CheckNITKU reports a NITKU that is not 22 digits, or — when npwp16 is given — does not start
// with that NPWP. The last six digits are the place's sequence; "000000" is the head office.
// Returns the NITKU's digits.
func CheckNITKU(nitku, npwp16 string) (string, error) {
	if !onlyDigitsAndPunctuation(nitku) {
		return "", ErrFormat
	}
	d := Digits(nitku)
	if len(d) != 22 {
		return "", ErrFormat
	}
	if npwp16 != "" && d[:16] != npwp16 {
		return "", ErrFormat
	}
	return d, nil
}

// DefaultNITKU is the NITKU of the head office of npwp16: the NPWP followed by "000000".
func DefaultNITKU(npwp16 string) string { return npwp16 + "000000" }

// NITKUOf reports whether nitku (22 digits) is a place of business of npwp16.
func NITKUOf(nitku, npwp16 string) bool {
	return len(nitku) == 22 && len(npwp16) == 16 && nitku[:16] == npwp16
}

// CheckPassport reports a passport number that is not 5 to 20 letters and digits. Returns it in
// capitals.
func CheckPassport(s string) (string, error) {
	p := strings.ToUpper(strings.TrimSpace(s))
	if len(p) < 5 || len(p) > 20 {
		return "", ErrFormat
	}
	for _, r := range p {
		if !(r >= '0' && r <= '9') && !(r >= 'A' && r <= 'Z') {
			return "", ErrFormat
		}
	}
	return p, nil
}

// CheckOther reports an "Other" identity document number that is not 3 to 30 letters, digits,
// and the separators . / - (spaces are dropped). Returns it in capitals.
func CheckOther(s string) (string, error) {
	p := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	if len(p) < 3 || len(p) > 30 {
		return "", ErrFormat
	}
	for _, r := range p {
		if !(r >= '0' && r <= '9') && !(r >= 'A' && r <= 'Z') && !strings.ContainsRune("./-", r) {
			return "", ErrFormat
		}
	}
	return p, nil
}

// SuspectNPWP reports an NPWP of the old format (16 digits starting with "0") whose check digit
// does not match: a Luhn digit over its first nine digits, the way python-stdnum checks it. DJP
// has not published the algorithm, so this is a warning to show, never a reason to refuse. An
// NPWP that is a NIK has no check digit and is never suspect.
func SuspectNPWP(npwp16 string) bool {
	if len(npwp16) != 16 || npwp16[0] != '0' || Digits(npwp16) != npwp16 {
		return false
	}
	return !luhn(npwp16[1:10])
}

// luhn reports whether the last digit of d is the Luhn check digit of the ones before it.
func luhn(d string) bool {
	sum := 0
	for i := len(d) - 1; i >= 0; i-- {
		n := int(d[i] - '0')
		if (len(d)-1-i)%2 == 1 {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
	}
	return sum%10 == 0
}

// SearchForm is how a search for a tax ID is compared with stored ones: the digits of an NPWP or
// NIK as typed, or the capitals of a passport or other document number. An old NPWP — 15 digits,
// or the start of one written the old way ("01.302.442.7") — gets the leading "0" it is stored
// with. "" when q holds nothing to search a tax ID by.
func SearchForm(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	if onlyDigitsAndPunctuation(q) {
		d := Digits(q)
		if len(d) == 15 || (len(d) < 15 && oldNPWPWritten(q)) {
			return "0" + d
		}
		return d
	}
	return strings.ToUpper(strings.ReplaceAll(q, " ", ""))
}

// oldNPWPWritten reports q written as an old NPWP is: two digits, then a dot (99.999.999.9-999.999).
func oldNPWPWritten(q string) bool {
	return len(q) >= 3 && q[0] >= '0' && q[0] <= '9' && q[1] >= '0' && q[1] <= '9' && q[2] == '.'
}

// Mask hides all but the first and last four characters: "3201********0001". Shorter values
// keep only their last two.
func Mask(s string) string {
	r := []rune(s)
	switch {
	case len(r) == 0:
		return ""
	case len(r) <= 8:
		if len(r) <= 2 {
			return strings.Repeat("*", len(r))
		}
		return strings.Repeat("*", len(r)-2) + string(r[len(r)-2:])
	}
	return string(r[:4]) + strings.Repeat("*", len(r)-8) + string(r[len(r)-4:])
}
