// Package infrastructure stores the files table in MySQL (the module's own sqlc package) and
// the bytes on disk.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/files/application"
	"__GO_MODULE__/internal/modules/files/contracts"
	"__GO_MODULE__/internal/modules/files/domain"
	"__GO_MODULE__/internal/modules/files/infrastructure/sqlc"
	"__GO_MODULE__/internal/shared/pagination"
)

var _ application.Repository = (*Repository)(nil)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository {
	return &Repository{tx: tx}
}

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

// company is the company the request works in (kernel 6 scope).
func company(ctx context.Context) int64 { return kernel.ScopeFrom(ctx).CompanyID }

func (r *Repository) Create(ctx context.Context, f domain.Stored) (int64, error) {
	return r.q(ctx).CreateFile(ctx, sqlc.CreateFileParams{
		CompanyID: company(ctx), Owner: f.Owner, ReadPermission: f.ReadPermission, Name: f.Name, ContentType: f.ContentType, Size: f.Size, Sha256: f.SHA256,
		StorageKey: f.Key, CreatedAt: f.CreatedAt, CreatedBy: sql.NullInt64{Int64: f.CreatedBy, Valid: f.CreatedBy != 0},
	})
}

func (r *Repository) Get(ctx context.Context, id int64) (domain.Stored, error) {
	row, err := r.q(ctx).GetFile(ctx, sqlc.GetFileParams{ID: id, CompanyID: company(ctx)})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Stored{}, contracts.ErrNotFound
	}
	if err != nil {
		return domain.Stored{}, err
	}
	return stored(sqlc.ListVisibleFilesRow(row)), nil
}

func stored(row sqlc.ListVisibleFilesRow) domain.Stored {
	return domain.Stored{ID: row.ID, Owner: row.Owner, ReadPermission: row.ReadPermission, Name: row.Name, ContentType: row.ContentType, Size: row.Size,
		SHA256: row.Sha256, Key: row.StorageKey, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy.Int64, Locked: row.LockedAt.Valid}
}

func (r *Repository) SoftDelete(ctx context.Context, id int64, at time.Time) (bool, error) {
	n, err := r.q(ctx).SoftDeleteFile(ctx, sqlc.SoftDeleteFileParams{DeletedAt: sql.NullTime{Time: at, Valid: true}, ID: id, CompanyID: company(ctx)})
	return n > 0, err
}

func (r *Repository) Lock(ctx context.Context, id int64, at time.Time) error {
	return r.q(ctx).LockFile(ctx, sqlc.LockFileParams{LockedAt: sql.NullTime{Time: at, Valid: true}, ID: id, CompanyID: company(ctx)})
}

func (r *Repository) ListVisible(ctx context.Context, readable []string, owner, search string, p pagination.Params) ([]domain.Stored, int64, error) {
	pattern := likeEscape(search)
	rows, err := r.q(ctx).ListVisibleFiles(ctx, sqlc.ListVisibleFilesParams{CompanyID: company(ctx), Readable: readable, Owner: owner, Search: pattern,
		Limit: int32(p.Limit), Offset: int32(p.Offset())})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountVisibleFiles(ctx, sqlc.CountVisibleFilesParams{CompanyID: company(ctx), Readable: readable, Owner: owner, Search: pattern})
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Stored, len(rows))
	for i, row := range rows {
		out[i] = stored(row)
	}
	return out, total, nil
}

func (r *Repository) ReadPermissions(ctx context.Context) ([]string, error) {
	return r.q(ctx).ReadPermissions(ctx, company(ctx))
}

func (r *Repository) SystemPurgeCandidates(ctx context.Context, before time.Time) ([]application.Purgeable, error) {
	rows, err := r.q(ctx).SystemPurgeCandidates(ctx, sql.NullTime{Time: before, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]application.Purgeable, len(rows))
	for i, row := range rows {
		out[i] = application.Purgeable{ID: row.ID, Key: row.StorageKey}
	}
	return out, nil
}

func (r *Repository) DeleteRow(ctx context.Context, id int64) error {
	return r.q(ctx).SystemDeleteFileRow(ctx, id)
}

func (r *Repository) SystemKnownKeys(ctx context.Context, keys []string) (map[string]bool, error) {
	got, err := r.q(ctx).SystemKnownKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(got))
	for _, k := range got {
		out[k] = true
	}
	return out, nil
}

// likeEscape makes a search term match literally inside LIKE '%…%': "%" and "_" typed by a
// user are characters, not wildcards (MySQL's default escape character is the backslash).
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
