// Package domain holds the files module's rules: which content is accepted, and what a stored
// name may contain.
package domain

import (
	"bytes"
	"net/http"
	"path"
	"strings"
	"unicode"
)

// DefaultTypes are what an upload may be when its module does not narrow it. Never SVG or HTML:
// served from the app's origin they could run script.
var DefaultTypes = []string{
	"image/png", "image/jpeg", "image/webp", "image/gif",
	"application/pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"text/plain", "text/csv",
}

var officeByExt = map[string]string{
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

// Detect decides a file's content type from its first bytes. A name only narrows what the
// bytes already say: an Office document is a ZIP whose extension names the kind, and plain
// text is text/plain or (.csv) text/csv. Anything else the bytes do not identify is
// application/octet-stream, which no default allows.
func Detect(head []byte, name string) string {
	sniffed := http.DetectContentType(head)
	if i := strings.IndexByte(sniffed, ';'); i >= 0 {
		sniffed = sniffed[:i]
	}
	ext := strings.ToLower(path.Ext(name))
	switch {
	case sniffed == "application/zip" && officeByExt[ext] != "" && ooxml(head):
		return officeByExt[ext]
	case sniffed == "application/zip" && officeByExt[ext] != "":
		// Named .docx/.xlsx/.pptx, but no Office package inside: refused, not trusted.
		return "application/octet-stream"
	case sniffed == "text/plain" && ext == ".csv":
		return "text/csv"
	case sniffed == "text/plain" && (ext == ".txt" || ext == ""):
		return "text/plain"
	case sniffed == "text/plain":
		// Text with another extension (.svg, .html, .js, …) is refused rather than trusted.
		return "application/octet-stream"
	}
	return sniffed
}

// ooxml reports whether a ZIP's first entry is an Office Open XML package part: the first local
// file header (at offset 0) names its file at offset 30, and Office writes [Content_Types].xml,
// _rels/, or docProps/ first (ECMA-376 Part 2, Open Packaging Conventions).
func ooxml(head []byte) bool {
	if len(head) < 30 || !bytes.HasPrefix(head, []byte("PK\x03\x04")) {
		return false
	}
	name := head[30:]
	for _, part := range []string{"[Content_Types].xml", "_rels/", "docProps/", "word/", "xl/", "ppt/"} {
		if bytes.HasPrefix(name, []byte(part)) {
			return true
		}
	}
	return false
}

// extensions are the names a stored file of each type is given: its own extension is kept when
// it is one of these, else the first replaces it.
var extensions = map[string][]string{
	"image/png": {".png"}, "image/jpeg": {".jpg", ".jpeg"}, "image/webp": {".webp"}, "image/gif": {".gif"},
	"application/pdf": {".pdf"},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   {".docx"},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {".xlsx"},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {".pptx"},
	"text/plain": {".txt"}, "text/csv": {".csv"},
}

// NameFor gives name the extension of the content type the bytes were found to be, so a
// download is opened by the program that matches its content (OWASP: "the file extension must
// match the content"). "laporan.html" holding a PDF becomes "laporan.pdf".
func NameFor(kind, name string) string {
	exts := extensions[kind]
	if len(exts) == 0 {
		return name
	}
	ext := path.Ext(name)
	for _, e := range exts {
		if strings.EqualFold(ext, e) {
			return name
		}
	}
	base := strings.TrimSuffix(name, ext)
	if base == "" {
		base = "berkas"
	}
	return base + exts[0]
}

// Allowed reports whether kind is in the list.
func Allowed(kind string, list []string) bool {
	for _, k := range list {
		if k == kind {
			return true
		}
	}
	return false
}

// CleanName keeps a file name safe to store and to put in a download header: no path, no
// control characters, at most 200 characters.
func CleanName(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		name = "berkas"
	}
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	return name
}
