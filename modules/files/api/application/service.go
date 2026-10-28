// Package application holds the files module's use cases.
package application

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/files/contracts"
	"__GO_MODULE__/internal/modules/files/domain"
	"__GO_MODULE__/internal/shared/pagination"
)

// keepDeleted is how long a deleted file's bytes stay on disk (and in the backups).
const keepDeleted = 30 * 24 * time.Hour

// orphanGrace is how old stored bytes must be before the sweep may remove them for having no
// row: a save writes the bytes first and the row in the caller's transaction, which commits —
// or rolls back, leaving the bytes behind — long before this.
const orphanGrace = 24 * time.Hour

// ErrTooLarge is what a Storage reports when the content passes the limit.
var ErrTooLarge = errors.New("too large")

// Purgeable is a deleted file whose bytes can go.
type Purgeable struct {
	ID  int64
	Key string
}

// Repository is the files table.
type Repository interface {
	Create(ctx context.Context, f domain.Stored) (int64, error)
	// Get returns an undeleted file, or contracts.ErrNotFound.
	Get(ctx context.Context, id int64) (domain.Stored, error)
	// SoftDelete deletes an unlocked file; false when there is none to delete.
	SoftDelete(ctx context.Context, id int64, at time.Time) (bool, error)
	Lock(ctx context.Context, id int64, at time.Time) error
	// ListVisible lists the files the administration page may show: stored by this module, or
	// with a read permission in readable.
	ListVisible(ctx context.Context, readable []string, owner, search string, p pagination.Params) ([]domain.Stored, int64, error)
	// ReadPermissions lists the read permissions owner modules named in the company.
	ReadPermissions(ctx context.Context) ([]string, error)
	SystemPurgeCandidates(ctx context.Context, before time.Time) ([]Purgeable, error)
	DeleteRow(ctx context.Context, id int64) error
	// SystemKnownKeys reports which storage keys a row (of any company, deleted or not) refers to.
	SystemKnownKeys(ctx context.Context, keys []string) (map[string]bool, error)
}

// Storage keeps the bytes.
type Storage interface {
	// Write streams r into a new object, refusing (ErrTooLarge) past limit bytes.
	Write(ctx context.Context, r io.Reader, limit int64) (key string, size int64, sha256 string, err error)
	Open(key string) (io.ReadSeekCloser, error)
	Remove(key string) error
	// CleanPartials removes writes abandoned half-way (a crash, a cut connection).
	CleanPartials(olderThan time.Duration) error
	// Keys calls fn with batches of the keys of complete objects older than olderThan.
	Keys(olderThan time.Duration, batch int, fn func(keys []string) error) error
}

// Permissions tells what the signed-in user may do — the kernel's d.Authz.
type Permissions interface {
	Can(ctx context.Context, permission string) bool
}

// Auditor records deletions — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Identity tells who is signed in — the kernel's d.Auth.
type Identity interface {
	UserID(ctx context.Context) (int64, bool)
}

// Service stores and serves files.
type Service struct {
	repo     Repository
	store    Storage
	audit    Auditor
	who      Identity
	maxBytes int64
	now      func() time.Time
}

func NewService(repo Repository, store Storage, audit Auditor, who Identity, maxBytes int64) *Service {
	return &Service{repo: repo, store: store, audit: audit, who: who, maxBytes: maxBytes, now: func() time.Time { return time.Now().UTC() }}
}

var (
	ownerPattern      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
	permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}\.[a-z][a-z0-9.-]{0,38}$`)
)

// AdminOwner is the owner of a file this module stores itself: shown on the administration page
// to any holder of files.read.
const AdminOwner = "files"

func rejected(format string, args ...any) error {
	return fmt.Errorf("%w: %s", contracts.ErrRejected, fmt.Sprintf(format, args...))
}

// Save checks what the upload is from its first bytes, then streams it to disk — nothing is
// written for a refused type, and a too-large file is removed as soon as it passes the limit.
func (s *Service) Save(ctx context.Context, u contracts.Upload) (contracts.File, error) {
	if !ownerPattern.MatchString(u.Owner) {
		return contracts.File{}, fmt.Errorf("files: owner %q is not a module id", u.Owner)
	}
	if u.ReadPermission != "" && !permissionPattern.MatchString(u.ReadPermission) {
		return contracts.File{}, fmt.Errorf("files: read permission %q is not \"<module>.<action>\"", u.ReadPermission)
	}
	limit := s.maxBytes
	if u.MaxBytes > 0 && u.MaxBytes < limit {
		limit = u.MaxBytes
	}
	allowed := u.Allowed
	if allowed == nil {
		allowed = domain.DefaultTypes
	}
	name := domain.CleanName(u.Name)
	// The name's extension may not contradict the content: decided below from the bytes, then
	// the stored name gets that type's extension (a PDF named x.html is downloaded as x.pdf).
	br := bufio.NewReaderSize(u.Reader, 512)
	head, _ := br.Peek(512)
	if len(head) == 0 {
		return contracts.File{}, rejected("berkas kosong")
	}
	kind := domain.Detect(head, name)
	if !domain.Allowed(kind, allowed) || !domain.Allowed(kind, domain.DefaultTypes) {
		return contracts.File{}, rejected("jenis berkas tidak diizinkan (%s)", kind)
	}
	name = domain.NameFor(kind, name)
	key, size, sum, err := s.store.Write(ctx, br, limit)
	if errors.Is(err, ErrTooLarge) {
		return contracts.File{}, rejected("berkas lebih besar dari %s", humanSize(limit))
	}
	if err != nil {
		return contracts.File{}, err
	}
	actor, _ := s.who.UserID(ctx)
	f := domain.Stored{Owner: u.Owner, ReadPermission: u.ReadPermission, Name: name, ContentType: kind, Size: size, SHA256: sum, Key: key, CreatedAt: s.now(), CreatedBy: actor}
	if f.ID, err = s.repo.Create(ctx, f); err != nil {
		_ = s.store.Remove(key)
		return contracts.File{}, err
	}
	return toContract(f), nil
}

func (s *Service) Get(ctx context.Context, id int64) (contracts.File, error) {
	f, err := s.repo.Get(ctx, id)
	if err != nil {
		return contracts.File{}, err
	}
	return toContract(f), nil
}

// Open returns the file and its content, ready for http.ServeContent.
func (s *Service) Open(ctx context.Context, id int64) (contracts.File, io.ReadSeekCloser, error) {
	f, err := s.repo.Get(ctx, id)
	if err != nil {
		return contracts.File{}, nil, err
	}
	rc, err := s.store.Open(f.Key)
	if err != nil {
		return contracts.File{}, nil, err
	}
	return toContract(f), rc, nil
}

// Delete takes the file out of use now; Purge removes its bytes later.
func (s *Service) Delete(ctx context.Context, id int64) error {
	f, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if f.Locked {
		return contracts.ErrLocked
	}
	ok, err := s.repo.SoftDelete(ctx, id, s.now())
	if err != nil {
		return err
	}
	if !ok {
		return contracts.ErrNotFound
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "files", Action: "file.deleted", Target: fmt.Sprintf("file:%d", id), Summary: "Menghapus berkas " + f.Name,
		Data: map[string]any{"owner": f.Owner, "name": f.Name, "size": f.Size}})
	return nil
}

// Lock keeps the file for good: the document it is attached to was posted (a record kept 10
// years). Locking a locked file is a no-op.
func (s *Service) Lock(ctx context.Context, id int64) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.Lock(ctx, id, s.now())
}

// readable is which of the company's file read permissions the signed-in user holds.
func (s *Service) readable(ctx context.Context, can Permissions) ([]string, error) {
	perms, err := s.repo.ReadPermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if can.Can(ctx, p) {
			out = append(out, p)
		}
	}
	return out, nil
}

// visible reports whether the administration page may show f: a file of this module's own, or
// one whose owner named a read permission the viewer holds. Anything else — an ID scan, a
// payslip — is reached only through its owner module, which decides who may see it.
func visible(f domain.Stored, readable []string) bool {
	return f.Owner == AdminOwner || (f.ReadPermission != "" && slices.Contains(readable, f.ReadPermission))
}

// AdminGet is a file for the administration page, or ErrNotFound when it may not show it — the
// same answer as for a file that does not exist, so its existence is not told either.
func (s *Service) AdminGet(ctx context.Context, can Permissions, id int64) (domain.Stored, error) {
	f, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Stored{}, err
	}
	readable, err := s.readable(ctx, can)
	if err != nil {
		return domain.Stored{}, err
	}
	if !visible(f, readable) {
		return domain.Stored{}, contracts.ErrNotFound
	}
	return f, nil
}

// AdminOpen opens a file the administration page may show.
func (s *Service) AdminOpen(ctx context.Context, can Permissions, id int64) (contracts.File, io.ReadSeekCloser, error) {
	if _, err := s.AdminGet(ctx, can, id); err != nil {
		return contracts.File{}, nil, err
	}
	return s.Open(ctx, id)
}

// AdminDelete deletes a file the administration page may show.
func (s *Service) AdminDelete(ctx context.Context, can Permissions, id int64) error {
	if _, err := s.AdminGet(ctx, can, id); err != nil {
		return err
	}
	return s.Delete(ctx, id)
}

// AdminList lists the files the administration page may show.
func (s *Service) AdminList(ctx context.Context, can Permissions, owner, search string, p pagination.Params) ([]contracts.File, int64, error) {
	readable, err := s.readable(ctx, can)
	if err != nil {
		return nil, 0, err
	}
	rows, total, err := s.repo.ListVisible(ctx, readable, owner, search, p)
	if err != nil {
		return nil, 0, err
	}
	out := make([]contracts.File, len(rows))
	for i, f := range rows {
		out[i] = toContract(f)
	}
	return out, total, nil
}

// Purge removes the bytes of files deleted more than 30 days ago, abandoned partial writes, and
// stored bytes no row refers to (a save whose transaction rolled back) — the job the module
// schedules.
func (s *Service) Purge(ctx context.Context) error {
	if err := s.store.CleanPartials(24 * time.Hour); err != nil {
		return err
	}
	if err := s.SweepOrphans(ctx); err != nil {
		return err
	}
	gone, err := s.repo.SystemPurgeCandidates(ctx, s.now().Add(-keepDeleted))
	if err != nil {
		return err
	}
	for _, g := range gone {
		if err := s.store.Remove(g.Key); err != nil {
			return err
		}
		if err := s.repo.DeleteRow(ctx, g.ID); err != nil {
			return err
		}
	}
	return nil
}

// SweepOrphans removes stored bytes older than a day that no row refers to. A row of any
// company, deleted or not, keeps its bytes: only the purge above removes those, on schedule.
func (s *Service) SweepOrphans(ctx context.Context) error {
	return s.store.Keys(orphanGrace, 500, func(keys []string) error {
		known, err := s.repo.SystemKnownKeys(ctx, keys)
		if err != nil {
			return err
		}
		for _, k := range keys {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !known[k] {
				if err := s.store.Remove(k); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func toContract(f domain.Stored) contracts.File {
	return contracts.File{ID: f.ID, Owner: f.Owner, Name: f.Name, ContentType: f.ContentType, Size: f.Size, SHA256: f.SHA256, CreatedAt: f.CreatedAt, Locked: f.Locked}
}

func humanSize(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%d MB", n>>20)
	}
	return fmt.Sprintf("%d KB", n>>10)
}
