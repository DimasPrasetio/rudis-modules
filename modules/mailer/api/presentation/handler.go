// Package presentation is the mailer's HTTP API: the outbox, for whoever looks after email.
package presentation

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/mailer/application"
	"__GO_MODULE__/internal/modules/mailer/domain"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

// Permissions this module declares (mailer.module.go) and guards with.
const (
	PermRead   = "mailer.read"
	PermManage = "mailer.manage"
	// PermSystem opens the installation-level outbox (email sent before any company is chosen)
	// — held across the home company (company 1), where the installation is administered.
	PermSystem = "mailer.system"
)

type messageDTO struct {
	ID            int64      `json:"id"`
	To            string     `json:"to"`
	ToName        string     `json:"to_name"`
	Subject       string     `json:"subject"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	NextAttemptAt time.Time  `json:"next_attempt_at"`
	LastError     string     `json:"last_error"`
	CreatedAt     time.Time  `json:"created_at"`
	SentAt        *time.Time `json:"sent_at"`
}

func toDTO(m domain.Message) messageDTO {
	d := messageDTO{ID: m.ID, To: m.To, ToName: m.ToName, Subject: m.Subject, Status: m.Status, Attempts: m.Attempts,
		NextAttemptAt: m.NextAttemptAt, LastError: m.LastError, CreatedAt: m.CreatedAt}
	if !m.SentAt.IsZero() {
		d.SentAt = &m.SentAt
	}
	return d
}

// Handler serves /api/v1/mailer.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

// Routes mounts the outbox endpoints.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/mailer/messages", h.authz.Require(PermRead, http.HandlerFunc(h.list)))
	mux.Handle("POST /api/v1/mailer/messages/{id}/retry", h.authz.Require(PermManage, http.HandlerFunc(h.retry)))
}

// reach is whose outbox the request works on: the installation-level one with ?scope=system
// (mailer.system across the home company), else the active company's (perm across it).
func (h *Handler) reach(r *http.Request, perm string) (application.Reach, error) {
	ctx := r.Context()
	if r.URL.Query().Get("scope") == "system" {
		sc := kernel.ScopeFrom(ctx)
		u, err := h.authz.Units(ctx, PermSystem)
		if err != nil {
			return application.Reach{}, err
		}
		return application.Reach{System: true, SystemAllowed: sc.CompanyID == kernel.DefaultCompany && sc.BranchID == 0 && u.AllBranches}, nil
	}
	u, err := h.authz.Units(ctx, perm)
	if err != nil {
		return application.Reach{}, err
	}
	return application.Reach{AllBranches: u.AllBranches}, nil
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, application.ErrSystemDenied):
		response.Fail(w, http.StatusForbidden, "Kotak keluar tingkat instalasi hanya untuk pemegang izin mailer.system di seluruh perusahaan induk", nil)
	case errors.Is(err, application.ErrNotEverywhere):
		response.Fail(w, http.StatusForbidden, "Kotak keluar milik seluruh perusahaan: izin ini harus berlaku di seluruh perusahaan", nil)
	default:
		response.Error(w, r, err)
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	reach, err := h.reach(r, PermRead)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	p := pagination.FromRequest(r)
	q := r.URL.Query()
	msgs, total, err := h.svc.List(r.Context(), reach, q.Get("status"), strings.TrimSpace(q.Get("q")), p)
	if err != nil {
		fail(w, r, err)
		return
	}
	out := make([]messageDTO, len(msgs))
	for i, m := range msgs {
		out[i] = toDTO(m)
	}
	response.Paginated(w, "Kotak keluar", out, pagination.NewMeta(p, total))
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	reach, err := h.reach(r, PermManage)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.Retry(r.Context(), reach, id); err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Email dijadwalkan untuk dikirim ulang", nil)
}
