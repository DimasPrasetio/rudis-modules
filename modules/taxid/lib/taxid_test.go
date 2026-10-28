package taxid

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeNPWP(t *testing.T) {
	for in, want := range map[string]string{
		"01.234.567.8-901.000": "0012345678901000", // the old 15 digits get a leading 0
		"012345678901000":      "0012345678901000",
		"3201011508900001":     "3201011508900001", // a resident's NIK used as NPWP
		"3201-0115-0890-0001":  "3201011508900001",
		" 0012 3456 7890 1000": "0012345678901000",
	} {
		if got, err := NormalizeNPWP(in); err != nil || got != want {
			t.Errorf("NormalizeNPWP(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "123", "01234567890100", "00123456789010001", "0123456789O1000", "01.234.567.8-901.00a",
		"0000000000000000", "000000000000000", // Coretax's placeholder for a buyer without an NPWP
		"3201234567890001", // 16 digits not starting with 0 that are no NIK (month 67)
		"9901011508900001", // …nor a province
	} {
		if _, err := NormalizeNPWP(in); !errors.Is(err, ErrFormat) {
			t.Errorf("NormalizeNPWP(%q) = %v; want ErrFormat", in, err)
		}
	}
}

func TestCheckNIK(t *testing.T) {
	for _, in := range []string{
		"3201011508900001", // 15 Aug 1990
		"3201015508900001", // a woman: day 15 + 40
		"3201012902000001", // 29 Feb 2000
		"3201-0115-0890-0001",
	} {
		if _, err := CheckNIK(in); err != nil {
			t.Errorf("CheckNIK(%q) = %v", in, err)
		}
	}
	for _, in := range []string{
		"320101150890000",  // 15 digits
		"0001011508900001", // no province
		"9901011508900001", // province 99 does not exist
		"2001011508900001", // …nor 20
		"3200011508900001", // no kabupaten/kota
		"3201001508900001", // no kecamatan
		"3201011508900000", // serial 0000
		"3201013208900001", // day 32
		"3201017208900001", // a woman's day 32
		"3201011513900001", // month 13
		"3201013102900001", // 31 Feb
		"3201011508900O01",
	} {
		if _, err := CheckNIK(in); !errors.Is(err, ErrFormat) {
			t.Errorf("CheckNIK(%q) = %v; want ErrFormat", in, err)
		}
	}
}

func TestCheckNITKU(t *testing.T) {
	if got, err := CheckNITKU("0012345678901000000001", "0012345678901000"); err != nil || got != "0012345678901000000001" {
		t.Errorf("a branch of the NPWP = %q, %v", got, err)
	}
	if _, err := CheckNITKU("0012345678901000000001", ""); err != nil {
		t.Errorf("without an NPWP to compare: %v", err)
	}
	for _, c := range [][2]string{
		{"001234567890100000000", ""},                  // 21 digits
		{"0099999999999999000000", "0012345678901000"}, // another NPWP's place
	} {
		if _, err := CheckNITKU(c[0], c[1]); !errors.Is(err, ErrFormat) {
			t.Errorf("CheckNITKU(%q, %q) = %v; want ErrFormat", c[0], c[1], err)
		}
	}
}

func TestCheckPassport(t *testing.T) {
	if got, err := CheckPassport(" e1234567 "); err != nil || got != "E1234567" {
		t.Errorf("CheckPassport = %q, %v", got, err)
	}
	for _, in := range []string{"A12", "A1234-567", "ABCDEFGHIJKLMNOPQRSTU"} {
		if _, err := CheckPassport(in); !errors.Is(err, ErrFormat) {
			t.Errorf("CheckPassport(%q) = %v; want ErrFormat", in, err)
		}
	}
}

func TestSuspectNPWP(t *testing.T) {
	// 01.302.442.7-…: Luhn over 013024427 holds.
	if SuspectNPWP("0013024427092000") {
		t.Error("a matching check digit is suspect")
	}
	if !SuspectNPWP("0013024420092000") {
		t.Error("a wrong check digit is not suspect")
	}
	if SuspectNPWP("3201011508900001") {
		t.Error("an NPWP that is a NIK has no check digit")
	}
}

func TestMask(t *testing.T) {
	for in, want := range map[string]string{
		"3201011508900001": "3201********0001",
		"1234567890":       "1234**7890",
		"E1234567":         "******67",
		"12":               "**",
		"":                 "",
	} {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestKindValid(t *testing.T) {
	for _, k := range []Kind{NPWP, NIK, Passport, None} {
		if !k.Valid() {
			t.Errorf("%q not valid", k)
		}
	}
	if Kind("ktp").Valid() {
		t.Error("an unknown kind is valid")
	}
}

func TestCheckNIKProvinces(t *testing.T) {
	for _, p := range []string{"11", "21", "36", "65", "76", "82", "91", "96"} {
		if _, err := CheckNIK(p + "01011508900001"); err != nil {
			t.Errorf("province %s: %v", p, err)
		}
	}
	if len(provinces) != 38 {
		t.Errorf("provinces = %d, want the 38 of Kepmendagri 300.2.2-2138/2025", len(provinces))
	}
}

func TestKinds(t *testing.T) {
	for _, k := range []Kind{NPWP, NIK, Passport, Other, None} {
		if !k.Valid() {
			t.Errorf("%q not valid", k)
		}
	}
	if Kind("tin").Valid() {
		t.Error("unknown kind valid")
	}
	for k, want := range map[Kind]bool{NPWP: true, NIK: true, Passport: false, Other: false, None: false} {
		if k.TakesNITKU() != want {
			t.Errorf("%q.TakesNITKU() = %v", k, !want)
		}
	}
}

func TestIdentifiesPerson(t *testing.T) {
	for _, c := range []struct {
		k    Kind
		id   string
		want bool
	}{
		{NPWP, "0013024427092000", false}, // a company's NPWP is no personal data
		{NPWP, "3201011508900001", true},  // …a NIK used as NPWP is
		{NIK, "3201011508900001", true},
		{Passport, "A1234567", true},
		{Other, "KITAS123", true},
		{None, "", false},
	} {
		if got := IdentifiesPerson(c.k, c.id); got != c.want {
			t.Errorf("IdentifiesPerson(%q, %q) = %v", c.k, c.id, got)
		}
	}
}

func TestNITKUHelpers(t *testing.T) {
	if got := DefaultNITKU("0013024427092000"); got != "0013024427092000000000" {
		t.Errorf("DefaultNITKU = %q", got)
	}
	if !NITKUOf("0013024427092000000001", "0013024427092000") || NITKUOf("0099999999999999000001", "0013024427092000") || NITKUOf("001302442709200000000", "0013024427092000") {
		t.Error("NITKUOf")
	}
}

func TestCheckOther(t *testing.T) {
	if got, err := CheckOther(" kitas 2c11/ab-123 "); err != nil || got != "KITAS2C11/AB-123" {
		t.Errorf("CheckOther = %q, %v", got, err)
	}
	for _, in := range []string{"", "ab", strings.Repeat("A", 31), "AB#12"} {
		if _, err := CheckOther(in); !errors.Is(err, ErrFormat) {
			t.Errorf("CheckOther(%q) = %v; want ErrFormat", in, err)
		}
	}
}

func TestSearchForm(t *testing.T) {
	for in, want := range map[string]string{
		"01.302.442.7-092.000": "0013024427092000",
		"01.302.442.7":         "0013024427", // the start of an old NPWP, written the old way
		"3201.0115":            "32010115",   // a NIK in groups is no old NPWP
		"0013":                 "0013",
		" a1234567 ":           "A1234567",
		"":                     "",
		"Toko Maju":            "TOKOMAJU",
	} {
		if got := SearchForm(in); got != want {
			t.Errorf("SearchForm(%q) = %q, want %q", in, got, want)
		}
	}
}
