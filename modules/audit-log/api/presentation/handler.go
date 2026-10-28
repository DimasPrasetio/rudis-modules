// Package presentation is the audit trail's HTTP API — read-only: nothing changes or removes an
// entry except the retention job.
package presentation

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/audit-log/application"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/response"
)

// Permissions this module declares and guards with.
const (
	PermRead = "audit-log.read"
	// PermSystem reads the installation-level trail (company 0) — held across the home company
	// (company 1), where the installation is administered.
	PermSystem = "audit-log.system"
)

type entryDTO struct {
	ID         int64           `json:"id"`
	OccurredAt time.Time       `json:"occurred_at"`
	ActorID    int64           `json:"actor_id"`
	ActorName  string          `json:"actor_name"`
	Kind       string          `json:"kind"`
	BranchID   int64           `json:"branch_id"`
	Module     string          `json:"module"`
	Action     string          `json:"action"`
	Target     string          `json:"target"`
	Summary    string          `json:"summary"`
	Data       json.RawMessage `json:"data"`
	IP         string          `json:"ip"`
	RequestID  string          `json:"request_id"`
}

// Handler serves /api/v1/audit.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
	clock kernel.Clock
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot, clock kernel.Clock) *Handler {
	return &Handler{svc: svc, authz: authz, clock: clock}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/audit", h.authz.Require(PermRead, http.HandlerFunc(h.list)))
	mux.Handle("GET /api/v1/audit/modules", h.authz.Require(PermRead, http.HandlerFunc(h.modules)))
}

// reach is how far the signed-in reader sees: the installation-level trail with ?scope=system,
// else the active company's branches their audit-log.read reaches.
func (h *Handler) reach(r *http.Request) (application.Reach, error) {
	ctx := r.Context()
	if r.URL.Query().Get("scope") == "system" {
		sc := kernel.ScopeFrom(ctx)
		home := sc.CompanyID == kernel.DefaultCompany && sc.BranchID == 0
		u, err := h.authz.Units(ctx, PermSystem)
		if err != nil {
			return application.Reach{}, err
		}
		return application.Reach{System: true, SystemAllowed: home && u.AllBranches}, nil
	}
	u, err := h.authz.Units(ctx, PermRead)
	if err != nil {
		return application.Reach{}, err
	}
	return application.Reach{AllBranches: u.AllBranches, Branches: u.BranchIDs}, nil
}

// fail answers the refusals of a reach the reader does not have; anything else as an error.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, application.ErrSystemDenied):
		response.Fail(w, http.StatusForbidden, "Log tingkat instalasi hanya untuk pemegang izin audit-log.system di seluruh perusahaan induk", nil)
	case errors.Is(err, application.ErrNoBranches):
		response.Fail(w, http.StatusForbidden, "Anda tidak memiliki izin untuk melihat log aktivitas", nil)
	default:
		response.Error(w, r, err)
	}
}

// day parses YYYY-MM-DD as the start of that day where the request works (its branch's time
// zone, else APP_TIMEZONE).
func (h *Handler) day(r *http.Request, v string) (time.Time, error) {
	loc := kernel.ScopeFrom(r.Context()).Location
	if loc == nil {
		loc = h.clock.Location()
	}
	return time.ParseInLocation("2006-01-02", v, loc)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := application.Filter{Kind: q.Get("kind"), Module: q.Get("module"), Target: strings.TrimSpace(q.Get("target"))}
	switch f.Kind {
	case "", "change", "access", "export", "security":
	default:
		response.Error(w, r, apperror.BadRequest("Parameter kind tidak dikenal"))
		return
	}
	if v := q.Get("actor"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.Error(w, r, apperror.BadRequest("Parameter actor tidak valid"))
			return
		}
		f.Actor = id
	}
	if v := q.Get("from"); v != "" {
		t, err := h.day(r, v)
		if err != nil {
			response.Error(w, r, apperror.BadRequest("Parameter from harus berformat YYYY-MM-DD"))
			return
		}
		f.From = t.UTC()
	}
	if v := q.Get("to"); v != "" {
		t, err := h.day(r, v)
		if err != nil {
			response.Error(w, r, apperror.BadRequest("Parameter to harus berformat YYYY-MM-DD"))
			return
		}
		f.To = t.AddDate(0, 0, 1).UTC() // the whole "to" day
	}
	reach, err := h.reach(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	p := pagination.FromRequest(r)
	entries, total, err := h.svc.List(r.Context(), reach, f, p)
	if err != nil {
		fail(w, r, err)
		return
	}
	out := make([]entryDTO, len(entries))
	for i, e := range entries {
		out[i] = entryDTO{ID: e.ID, OccurredAt: e.OccurredAt, ActorID: e.ActorID, ActorName: e.ActorName, Kind: e.Kind, BranchID: e.BranchID,
			Module: e.Module, Action: e.Action,
			Target: e.Target, Summary: e.Summary, Data: e.Data, IP: e.IP, RequestID: e.RequestID}
		if len(out[i].Data) == 0 || string(out[i].Data) == "null" {
			out[i].Data = nil
		}
	}
	response.Paginated(w, "Log aktivitas", out, pagination.NewMeta(p, total))
}

func (h *Handler) modules(w http.ResponseWriter, r *http.Request) {
	reach, err := h.reach(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	mods, err := h.svc.Modules(r.Context(), reach)
	if err != nil {
		fail(w, r, err)
		return
	}
	if mods == nil {
		mods = []string{}
	}
	response.OK(w, "Modul", mods)
}
