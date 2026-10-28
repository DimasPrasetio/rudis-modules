package domain

import "time"

// Stored is a file's row.
type Stored struct {
	ID    int64
	Owner string
	// ReadPermission lets someone open the file outside its owner module (the administration
	// page); empty: only through the owner module.
	ReadPermission string
	Name           string
	ContentType    string
	Size           int64
	SHA256         string
	Key            string // where the bytes are, relative to the store's root
	CreatedAt      time.Time
	CreatedBy      int64
	Locked         bool // attached to a posted document: never deleted
}
