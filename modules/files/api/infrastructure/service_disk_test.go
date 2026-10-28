package infrastructure_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/files/application"
	"__GO_MODULE__/internal/modules/files/contracts"
	"__GO_MODULE__/internal/modules/files/domain"
	"__GO_MODULE__/internal/modules/files/infrastructure"
	"__GO_MODULE__/internal/shared/pagination"
)

type memRepo struct {
	rows    map[int64]*domain.Stored
	deleted map[int64]time.Time
	locked  map[int64]bool
	next    int64
}

func (m *memRepo) Create(_ context.Context, f domain.Stored) (int64, error) {
	m.next++
	f.ID = m.next
	m.rows[f.ID] = &f
	return f.ID, nil
}

func (m *memRepo) Get(_ context.Context, id int64) (domain.Stored, error) {
	f, ok := m.rows[id]
	if _, gone := m.deleted[id]; !ok || gone {
		return domain.Stored{}, contracts.ErrNotFound
	}
	out := *f
	out.Locked = m.locked[id]
	return out, nil
}

func (m *memRepo) SoftDelete(_ context.Context, id int64, at time.Time) (bool, error) {
	if _, gone := m.deleted[id]; gone || m.locked[id] {
		return false, nil
	}
	m.deleted[id] = at
	return true, nil
}

func (m *memRepo) Lock(_ context.Context, id int64, _ time.Time) error {
	if m.locked == nil {
		m.locked = map[int64]bool{}
	}
	m.locked[id] = true
	return nil
}

func (m *memRepo) ListVisible(_ context.Context, readable []string, _, _ string, _ pagination.Params) ([]domain.Stored, int64, error) {
	var out []domain.Stored
	for id, f := range m.rows {
		if _, gone := m.deleted[id]; !gone && (f.Owner == application.AdminOwner || (f.ReadPermission != "" && slices.Contains(readable, f.ReadPermission))) {
			out = append(out, *f)
		}
	}
	return out, int64(len(out)), nil
}

func (m *memRepo) ReadPermissions(context.Context) ([]string, error) {
	var out []string
	for _, f := range m.rows {
		if f.ReadPermission != "" && !slices.Contains(out, f.ReadPermission) {
			out = append(out, f.ReadPermission)
		}
	}
	return out, nil
}

func (m *memRepo) SystemKnownKeys(_ context.Context, keys []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, f := range m.rows {
		if slices.Contains(keys, f.Key) {
			out[f.Key] = true
		}
	}
	return out, nil
}

func (m *memRepo) SystemPurgeCandidates(_ context.Context, before time.Time) ([]application.Purgeable, error) {
	var out []application.Purgeable
	for id, at := range m.deleted {
		if at.Before(before) {
			out = append(out, application.Purgeable{ID: id, Key: m.rows[id].Key})
		}
	}
	return out, nil
}

func (m *memRepo) DeleteRow(_ context.Context, id int64) error {
	delete(m.rows, id)
	delete(m.deleted, id)
	return nil
}

type noAudit struct{}

func (noAudit) Record(context.Context, kernel.AuditEntry) {}

type nobody struct{}

func (nobody) UserID(context.Context) (int64, bool) { return 7, true }

var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")

func setup(t *testing.T) (*application.Service, *memRepo, string) {
	t.Helper()
	root := t.TempDir()
	disk := infrastructure.Disk{Root: root}
	if err := disk.Check(); err != nil {
		t.Fatal(err)
	}
	repo := &memRepo{rows: map[int64]*domain.Stored{}, deleted: map[int64]time.Time{}}
	return application.NewService(repo, disk, noAudit{}, nobody{}, 1<<20), repo, root
}

func filesUnder(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err == nil && !e.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestSaveOpenDelete(t *testing.T) {
	svc, _, root := setup(t)
	f, err := svc.Save(context.Background(), contracts.Upload{Owner: "settings", Name: "../logo.png", Reader: bytes.NewReader(png)})
	if err != nil {
		t.Fatal(err)
	}
	if f.ContentType != "image/png" || f.Name != "logo.png" || f.Size != int64(len(png)) || len(f.SHA256) != 64 {
		t.Fatalf("file = %+v", f)
	}
	_, rc, err := svc.Open(context.Background(), f.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, png) {
		t.Fatal("content differs")
	}
	if err := svc.Delete(context.Background(), f.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Open(context.Background(), f.ID); !errors.Is(err, contracts.ErrNotFound) {
		t.Fatalf("a deleted file opened: %v", err)
	}
	if err := svc.Purge(context.Background()); err != nil || len(filesUnder(t, root)) != 1 {
		t.Fatal("the bytes must stay for 30 days after a delete")
	}
}

func TestRefusedUploadsLeaveNothingBehind(t *testing.T) {
	svc, repo, root := setup(t)
	for name, u := range map[string]contracts.Upload{
		"svg":                         {Owner: "settings", Name: "x.svg", Reader: strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
		"html as png":                 {Owner: "settings", Name: "x.png", Reader: strings.NewReader("<html><script>alert(1)</script></html>")},
		"too large":                   {Owner: "settings", Name: "big.png", Reader: io.MultiReader(bytes.NewReader(png), bytes.NewReader(make([]byte, 2<<20)))},
		"over the module's own limit": {Owner: "settings", Name: "a.png", Reader: io.MultiReader(bytes.NewReader(png), bytes.NewReader(make([]byte, 600))), MaxBytes: 100},
		"not in the module's list":    {Owner: "settings", Name: "a.png", Reader: bytes.NewReader(png), Allowed: []string{"application/pdf"}},
		"empty":                       {Owner: "settings", Name: "a.png", Reader: strings.NewReader("")},
	} {
		if _, err := svc.Save(context.Background(), u); !errors.Is(err, contracts.ErrRejected) {
			t.Errorf("%s: want ErrRejected, got %v", name, err)
		}
	}
	if left := filesUnder(t, root); len(left) != 0 || len(repo.rows) != 0 {
		t.Fatalf("refused uploads left files %v / rows %d", left, len(repo.rows))
	}
	if _, err := svc.Save(context.Background(), contracts.Upload{Owner: "../etc", Name: "a.png", Reader: bytes.NewReader(png)}); err == nil {
		t.Fatal("an owner that is not a module id was accepted")
	}
}

func TestALockedFileCannotBeDeleted(t *testing.T) {
	svc, _, _ := setup(t)
	f, err := svc.Save(context.Background(), contracts.Upload{Owner: "sales", Name: "faktur.pdf", Reader: strings.NewReader("%PDF-1.7\n%…\n")})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Lock(context.Background(), f.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), f.ID); !errors.Is(err, contracts.ErrLocked) {
		t.Fatalf("a locked file was deleted: %v", err)
	}
	if got, _ := svc.Get(context.Background(), f.ID); !got.Locked {
		t.Fatal("the file does not say it is locked")
	}
}

// holds is a viewer holding exactly these permissions.
type holds []string

func (h holds) Can(_ context.Context, p string) bool { return slices.Contains(h, p) }

func TestTheAdministrationPageShowsOnlyWhatItsViewerMayRead(t *testing.T) {
	svc, _, _ := setup(t)
	ctx := context.Background()
	save := func(owner, perm string) contracts.File {
		f, err := svc.Save(ctx, contracts.Upload{Owner: owner, ReadPermission: perm, Name: "a.png", Reader: bytes.NewReader(png)})
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	ktp := save("visitor-log", "")               // personal data: only through its module
	receipt := save("expenses", "expenses.read") // readable by holders of expenses.read
	page := pagination.Params{Page: 1, Limit: 10}
	if _, n, _ := svc.AdminList(ctx, holds{"files.read"}, "", "", page); n != 0 {
		t.Fatalf("files.read alone lists %d files of other modules", n)
	}
	if _, _, err := svc.AdminOpen(ctx, holds{"files.read"}, ktp.ID); !errors.Is(err, contracts.ErrNotFound) {
		t.Fatalf("an ID scan opened on the administration page: %v", err)
	}
	if err := svc.AdminDelete(ctx, holds{"files.read", "files.manage"}, ktp.ID); !errors.Is(err, contracts.ErrNotFound) {
		t.Fatalf("another module's file deleted from the administration page: %v", err)
	}
	got, n, _ := svc.AdminList(ctx, holds{"files.read", "expenses.read"}, "", "", page)
	if n != 1 || got[0].ID != receipt.ID {
		t.Fatalf("a holder of expenses.read lists %d: %+v", n, got)
	}
	if _, rc, err := svc.AdminOpen(ctx, holds{"expenses.read"}, receipt.ID); err != nil {
		t.Fatalf("the receipt did not open for a holder of its read permission: %v", err)
	} else {
		rc.Close()
	}
	if _, err := svc.Save(ctx, contracts.Upload{Owner: "expenses", ReadPermission: "not a permission", Name: "a.png", Reader: bytes.NewReader(png)}); err == nil {
		t.Fatal("a malformed read permission was accepted")
	}
}

func TestStoredNamesMatchTheirContent(t *testing.T) {
	svc, _, _ := setup(t)
	f, err := svc.Save(context.Background(), contracts.Upload{Owner: "sales", Name: "faktur.html", Reader: strings.NewReader("%PDF-1.7\n%…\n")})
	if err != nil || f.Name != "faktur.pdf" {
		t.Fatalf("a PDF named .html is stored as %q (%v)", f.Name, err)
	}
}

func TestTheSweepRemovesOnlyBytesNoRowRefersTo(t *testing.T) {
	svc, _, root := setup(t)
	kept, err := svc.Save(context.Background(), contracts.Upload{Owner: "settings", Name: "logo.png", Reader: bytes.NewReader(png)})
	if err != nil {
		t.Fatal(err)
	}
	// Bytes whose row was rolled back with the caller's transaction, and one written just now.
	disk := infrastructure.Disk{Root: root}
	orphan, _, _, err := disk.Write(context.Background(), bytes.NewReader(png), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	fresh, _, _, _ := disk.Write(context.Background(), bytes.NewReader(png), 1<<20)
	old := time.Now().Add(-48 * time.Hour)
	for _, p := range filesUnder(t, root) {
		if !strings.HasSuffix(filepath.ToSlash(p), fresh) {
			_ = os.Chtimes(p, old, old)
		}
	}
	if err := svc.SweepOrphans(context.Background()); err != nil {
		t.Fatal(err)
	}
	left := strings.Join(filesUnder(t, root), "\n")
	if strings.Contains(left, filepath.FromSlash(orphan)) {
		t.Fatal("unreferenced bytes older than a day were kept")
	}
	if !strings.Contains(left, filepath.FromSlash(fresh)) {
		t.Fatal("bytes younger than a day were removed — their row may still be committing")
	}
	if _, rc, err := svc.Open(context.Background(), kept.ID); err != nil {
		t.Fatalf("a referenced file lost its bytes: %v", err)
	} else {
		rc.Close()
	}
}
