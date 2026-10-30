// Package presentation is the stock module's HTTP API.
package presentation

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/stock/application"
	"__GO_MODULE__/internal/modules/stock/domain"
	"__GO_MODULE__/internal/modules/stock/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/docstate"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
	"__GO_MODULE__/internal/shared/validator"
)

// Handler serves /api/v1/stock.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
	clock kernel.Clock
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot, clock kernel.Clock) *Handler {
	return &Handler{svc: svc, authz: authz, clock: clock}
}

// Routes mounts the handlers. Each route needs its permission somewhere in the company; the
// service then checks it for the branch of the document's warehouse, and stock.manage across
// the whole company.
func (h *Handler) Routes(mux *http.ServeMux) {
	need := func(p string, f http.HandlerFunc) http.Handler { return h.authz.Require(p, f) }
	const base = "/api/v1/stock"
	mux.Handle("GET "+base+"/settings", need(application.PermRead, h.settings))
	mux.Handle("PUT "+base+"/settings", need(application.PermManage, h.setSettings))
	mux.Handle("GET "+base+"/warehouses", need(application.PermRead, h.warehouses))
	mux.Handle("GET "+base+"/branches", need(application.PermRead, h.branches))
	mux.Handle("POST "+base+"/warehouses", need(application.PermManage, h.saveWarehouse))
	mux.Handle("PUT "+base+"/warehouses/{id}", need(application.PermManage, h.saveWarehouse))
	mux.Handle("POST "+base+"/warehouses/{id}/deactivate", need(application.PermManage, h.setWarehouseActive(false)))
	mux.Handle("POST "+base+"/warehouses/{id}/activate", need(application.PermManage, h.setWarehouseActive(true)))
	mux.Handle("GET "+base+"/items/lookup", need(application.PermRead, h.lookupItems))
	mux.Handle("GET "+base+"/items/{id}/units", need(application.PermRead, h.itemUnits))
	mux.Handle("GET "+base+"/items/{id}/card", need(application.PermRead, h.card))
	mux.Handle("GET "+base+"/balances", need(application.PermRead, h.balances))
	mux.Handle("GET "+base+"/report", need(application.PermRead, h.report))
	mux.Handle("GET "+base+"/documents", need(application.PermRead, h.documents))
	mux.Handle("POST "+base+"/documents", need(application.PermDraft, h.createDocument))
	mux.Handle("GET "+base+"/documents/{id}", need(application.PermRead, h.document))
	mux.Handle("PUT "+base+"/documents/{id}", need(application.PermDraft, h.updateDocument))
	mux.Handle("DELETE "+base+"/documents/{id}", need(application.PermDraft, h.deleteDocument))
	mux.Handle("POST "+base+"/documents/{id}/post", need(application.PermPost, h.post))
	mux.Handle("POST "+base+"/documents/{id}/cancel", need(application.PermCancel, h.cancel))
	mux.Handle("POST "+base+"/documents/{id}/amend", need(application.PermDraft, h.amend))
}

// fail maps the module's errors to responses: a refused movement is a 409 with its message, a
// lifecycle error too, a missing row a 404.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var refused *application.MoveError
	switch {
	case errors.As(err, &refused):
		err = apperror.Conflict(refused.Message)
	case errors.Is(err, application.ErrNotFound):
		err = apperror.NotFound("Tidak ditemukan")
	case errors.Is(err, docstate.ErrTransition), errors.Is(err, docstate.ErrStale):
		err = apperror.Conflict(err.Error())
	case errors.Is(err, docstate.ErrReason):
		err = apperror.Validation([]validator.FieldError{{Field: "reason", Message: err.Error()}})
	case errors.Is(err, money.ErrOverflow):
		err = apperror.Validation([]validator.FieldError{{Field: "lines", Message: "jumlah atau nilai terlalu besar"}})
	}
	response.Error(w, r, err)
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := request.PathInt64(r, "id")
	if err != nil || id <= 0 {
		response.Error(w, r, apperror.NotFound("Tidak ditemukan"))
		return 0, false
	}
	return id, true
}

func queryInt(r *http.Request, name string) int64 {
	n, _ := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	if n < 0 {
		return 0
	}
	return n
}

// parseDate reads "2006-01-02"; ok is false for anything else.
func parseDate(s string) (kernel.Date, bool) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return kernel.Date{}, false
	}
	return kernel.DateOf(t, time.UTC), true
}

// period reads ?from=&to=, defaulting to the month so far.
func (h *Handler) period(r *http.Request) (kernel.Date, kernel.Date, error) {
	today := kernel.Today(r.Context(), h.clock)
	from, to := kernel.Date{Year: today.Year, Month: today.Month, Day: 1}, today
	q := r.URL.Query()
	if s := q.Get("from"); s != "" {
		d, ok := parseDate(s)
		if !ok {
			return from, to, apperror.Validation([]validator.FieldError{{Field: "from", Message: "tanggal tidak valid"}})
		}
		from = d
	}
	if s := q.Get("to"); s != "" {
		d, ok := parseDate(s)
		if !ok {
			return from, to, apperror.Validation([]validator.FieldError{{Field: "to", Message: "tanggal tidak valid"}})
		}
		to = d
	}
	return from, to, nil
}

// --- settings and warehouses -----------------------------------------------------------------

type settingsDTO struct {
	CostMethod string `json:"cost_method"`
	Locked     bool   `json:"locked"`
	CanManage  bool   `json:"can_manage"`
}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Settings(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Pengaturan persediaan", settingsDTO{CostMethod: string(st.Method), Locked: st.Locked, CanManage: h.svc.Can(r.Context(), application.PermManage, 0)})
}

func (h *Handler) setSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CostMethod string `json:"cost_method"`
	}
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	st, err := h.svc.SetCostMethod(r.Context(), in.CostMethod)
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Metode biaya disimpan", settingsDTO{CostMethod: string(st.Method), Locked: st.Locked, CanManage: true})
}

type warehouseDTO struct {
	ID       int64  `json:"id"`
	BranchID int64  `json:"branch_id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

func toWarehouseDTO(w domain.Warehouse) warehouseDTO {
	return warehouseDTO{ID: w.ID, BranchID: w.BranchID, Code: w.Code, Name: w.Name, IsActive: w.IsActive}
}

func (h *Handler) warehouses(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.Warehouses(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	out := make([]warehouseDTO, len(rows))
	for i, wh := range rows {
		out[i] = toWarehouseDTO(wh)
	}
	response.OK(w, "Daftar gudang", out)
}

type branchDTO struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// branches are the branches a warehouse may belong to — empty without an organization module.
func (h *Handler) branches(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.Branches(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	out := make([]branchDTO, len(rows))
	for i, b := range rows {
		out[i] = branchDTO{ID: b.ID, Code: b.Code, Name: b.Name}
	}
	response.OK(w, "Cabang", out)
}

func (h *Handler) saveWarehouse(w http.ResponseWriter, r *http.Request) {
	var id int64
	if r.PathValue("id") != "" {
		var ok bool
		if id, ok = pathID(w, r); !ok {
			return
		}
	}
	var in struct {
		BranchID int64  `json:"branch_id"`
		Code     string `json:"code"`
		Name     string `json:"name"`
	}
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	wh, err := h.svc.SaveWarehouse(r.Context(), id, domain.Warehouse{BranchID: in.BranchID, Code: in.Code, Name: in.Name})
	if err != nil {
		fail(w, r, err)
		return
	}
	if id == 0 {
		response.Created(w, "Gudang ditambahkan", toWarehouseDTO(wh))
		return
	}
	response.OK(w, "Gudang disimpan", toWarehouseDTO(wh))
}

func (h *Handler) setWarehouseActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		wh, err := h.svc.SetWarehouseActive(r.Context(), id, active)
		if err != nil {
			fail(w, r, err)
			return
		}
		msg := "Gudang diaktifkan"
		if !active {
			msg = "Gudang dinonaktifkan"
		}
		response.OK(w, msg, toWarehouseDTO(wh))
	}
}

// --- items -----------------------------------------------------------------------------------

type itemDTO struct {
	ID       int64  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	BaseUnit string `json:"base_unit"`
	IsActive bool   `json:"is_active"`
}

type unitDTO struct {
	Code   string     `json:"code"`
	Name   string     `json:"name"`
	Factor money.Rate `json:"factor"`
	IsBase bool       `json:"is_base"`
}

func toItemDTO(it ports.Item) itemDTO {
	return itemDTO{ID: it.ID, Code: it.Code, Name: it.Name, BaseUnit: it.BaseUnit, IsActive: it.IsActive}
}

// lookupItems is the stocked, active items matching ?q=, for a document line's picker.
func (h *Handler) lookupItems(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.SearchItems(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		fail(w, r, err)
		return
	}
	out := make([]itemDTO, len(rows))
	for i, it := range rows {
		out[i] = toItemDTO(it)
	}
	response.OK(w, "Barang", out)
}

func (h *Handler) itemUnits(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	units, err := h.svc.ItemUnits(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	out := make([]unitDTO, len(units))
	for i, u := range units {
		out[i] = unitDTO{Code: u.Code, Name: u.Name, Factor: u.Factor, IsBase: u.IsBase}
	}
	response.OK(w, "Satuan barang", out)
}

// --- stock, the stock card, and the movement report -----------------------------------------

type balanceDTO struct {
	WarehouseID   int64         `json:"warehouse_id"`
	WarehouseCode string        `json:"warehouse_code"`
	WarehouseName string        `json:"warehouse_name"`
	ItemID        int64         `json:"item_id"`
	ItemCode      string        `json:"item_code"`
	ItemName      string        `json:"item_name"`
	BaseUnit      string        `json:"base_unit"`
	Qty           money.Qty     `json:"qty"`
	Value         money.Amount  `json:"value"`
	UnitCost      *money.Amount `json:"unit_cost"`
	LastDate      string        `json:"last_date"`
}

func (h *Handler) balances(w http.ResponseWriter, r *http.Request) {
	p := pagination.FromRequest(r)
	q := r.URL.Query()
	st, err := h.svc.Balances(r.Context(), application.BalanceFilter{WarehouseID: queryInt(r, "warehouse_id"), ItemID: queryInt(r, "item_id"),
		WithStock: q.Get("with_stock") == "1" || q.Get("with_stock") == "true", Search: application.LikePattern(q.Get("q")), Limit: p.Limit, Offset: p.Offset()})
	if err != nil {
		fail(w, r, err)
		return
	}
	out := make([]balanceDTO, len(st.Rows))
	for i, b := range st.Rows {
		out[i] = balanceDTO{WarehouseID: b.WarehouseID, WarehouseCode: b.WarehouseCode, WarehouseName: b.WarehouseName, ItemID: b.ItemID,
			ItemCode: b.ItemCode, ItemName: b.ItemName, BaseUnit: b.BaseUnit, Qty: b.Qty, Value: b.Value}
		if c, ok := application.UnitCostOf(b.Qty, b.Value); ok {
			out[i].UnitCost = &c
		}
		if b.LastDate != nil {
			out[i].LastDate = b.LastDate.String()
		}
	}
	meta := pagination.NewMeta(p, st.Total)
	response.JSON(w, http.StatusOK, map[string]any{"success": true, "message": "Stok saat ini", "data": out, "meta": meta,
		"total_value": st.Value})
}

type moveDTO struct {
	ID             int64        `json:"id"`
	Date           string       `json:"date"`
	WarehouseID    int64        `json:"warehouse_id"`
	WarehouseCode  string       `json:"warehouse_code"`
	DocumentID     int64        `json:"document_id"`
	DocumentNumber string       `json:"document_number"`
	DocumentKind   string       `json:"document_kind"`
	Source         string       `json:"source"`
	In             money.Qty    `json:"in"`
	Out            money.Qty    `json:"out"`
	Value          money.Amount `json:"value"`
	QtyAfter       money.Qty    `json:"qty_after"`
	ValueAfter     money.Amount `json:"value_after"`
	IsReversal     bool         `json:"is_reversal"`
	IsReversed     bool         `json:"is_reversed"`
}

type openingDTO struct {
	WarehouseID int64        `json:"warehouse_id"`
	Qty         money.Qty    `json:"qty"`
	Value       money.Amount `json:"value"`
}

func (h *Handler) card(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	from, to, err := h.period(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	c, err := h.svc.Card(r.Context(), id, queryInt(r, "warehouse_id"), from, to)
	if err != nil {
		fail(w, r, err)
		return
	}
	openings := make([]openingDTO, len(c.Openings))
	for i, o := range c.Openings {
		openings[i] = openingDTO{WarehouseID: o.WarehouseID, Qty: o.Qty, Value: o.Value}
	}
	moves := make([]moveDTO, len(c.Moves))
	for i, m := range c.Moves {
		d := moveDTO{ID: m.ID, Date: m.Date.String(), WarehouseID: m.WarehouseID, WarehouseCode: m.WarehouseCode, DocumentID: m.DocumentID,
			Source: m.Source, Value: m.Value, QtyAfter: m.QtyAfter, ValueAfter: m.ValueAfter, IsReversal: m.Reverses != 0, IsReversed: m.ReversedBy != 0}
		if doc, ok := c.Docs[m.DocumentID]; ok {
			d.DocumentNumber, d.DocumentKind = doc.Number, string(doc.Kind)
		}
		if m.Qty.Sign() > 0 {
			d.In = m.Qty
		} else {
			d.Out = m.Qty.Neg()
		}
		moves[i] = d
	}
	response.OK(w, "Kartu stok", map[string]any{"item": toItemDTO(c.Item), "from": c.From.String(), "to": c.To.String(),
		"openings": openings, "moves": moves})
}

type reportDTO struct {
	WarehouseID   int64        `json:"warehouse_id"`
	WarehouseCode string       `json:"warehouse_code"`
	ItemID        int64        `json:"item_id"`
	ItemCode      string       `json:"item_code"`
	ItemName      string       `json:"item_name"`
	BaseUnit      string       `json:"base_unit"`
	OpeningQty    money.Qty    `json:"opening_qty"`
	OpeningValue  money.Amount `json:"opening_value"`
	InQty         money.Qty    `json:"in_qty"`
	InValue       money.Amount `json:"in_value"`
	OutQty        money.Qty    `json:"out_qty"`
	OutValue      money.Amount `json:"out_value"`
	ClosingQty    money.Qty    `json:"closing_qty"`
	ClosingValue  money.Amount `json:"closing_value"`
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	from, to, err := h.period(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	rows, err := h.svc.Report(r.Context(), application.ReportFilter{WarehouseID: queryInt(r, "warehouse_id"), From: from, To: to,
		Search: application.LikePattern(r.URL.Query().Get("q"))})
	if err != nil {
		fail(w, r, err)
		return
	}
	out := make([]reportDTO, len(rows))
	for i, x := range rows {
		out[i] = reportDTO(x)
	}
	response.OK(w, "Laporan mutasi stok", map[string]any{"from": from.String(), "to": to.String(), "rows": out})
}

// --- documents -------------------------------------------------------------------------------

type lineDTO struct {
	ID       int64         `json:"id"`
	ItemID   int64         `json:"item_id"`
	ItemCode string        `json:"item_code"`
	ItemName string        `json:"item_name"`
	UnitCode string        `json:"unit_code"`
	Factor   money.Rate    `json:"factor"`
	Qty      money.Qty     `json:"qty"`
	BaseQty  money.Qty     `json:"base_qty"`
	UnitCost *money.Amount `json:"unit_cost"`
	ZeroCost bool          `json:"zero_cost"`
	Value    *money.Amount `json:"value"`
}

type documentDTO struct {
	ID            int64        `json:"id"`
	Kind          string       `json:"kind"`
	KindLabel     string       `json:"kind_label"`
	WarehouseID   int64        `json:"warehouse_id"`
	WarehouseCode string       `json:"warehouse_code"`
	WarehouseName string       `json:"warehouse_name"`
	Date          string       `json:"date"`
	Number        string       `json:"number"`
	Counterparty  string       `json:"counterparty"`
	Reference     string       `json:"reference"`
	Notes         string       `json:"notes"`
	Status        string       `json:"status"`
	PostedAt      *time.Time   `json:"posted_at"`
	CancelledAt   *time.Time   `json:"cancelled_at"`
	CancelReason  string       `json:"cancel_reason"`
	AmendedFrom   int64        `json:"amended_from"`
	Revision      int          `json:"revision"`
	TotalValue    money.Amount `json:"total_value"`
	Lines         []lineDTO    `json:"lines,omitempty"`
	CanEdit       bool         `json:"can_edit"`
	CanPost       bool         `json:"can_post"`
	CanCancel     bool         `json:"can_cancel"`
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func toDocumentDTO(d domain.Document, whCode, whName string) documentDTO {
	out := documentDTO{ID: d.ID, Kind: string(d.Kind), KindLabel: d.Kind.Label(), WarehouseID: d.WarehouseID, WarehouseCode: whCode, WarehouseName: whName,
		Date: d.Date.String(), Number: d.Number, Counterparty: d.Counterparty, Reference: d.Reference, Notes: d.Notes, Status: string(d.Status),
		PostedAt: timePtr(d.PostedAt), CancelledAt: timePtr(d.CancelledAt), CancelReason: d.CancelReason, AmendedFrom: d.AmendedFrom, Revision: d.Revision}
	for _, l := range d.Lines {
		out.Lines = append(out.Lines, lineDTO{ID: l.ID, ItemID: l.ItemID, ItemCode: l.ItemCode, ItemName: l.ItemName, UnitCode: l.UnitCode,
			Factor: l.Factor, Qty: l.Qty, BaseQty: l.BaseQty, UnitCost: l.UnitCost, ZeroCost: l.ZeroCost, Value: l.Value})
		if l.Value != nil {
			out.TotalValue, _ = out.TotalValue.Add(*l.Value)
		}
	}
	return out
}

func toDetailDTO(d application.Detail) documentDTO {
	out := toDocumentDTO(d.Document, d.Warehouse.Code, d.Warehouse.Name)
	if out.Lines == nil {
		out.Lines = []lineDTO{}
	}
	out.CanEdit, out.CanPost, out.CanCancel = d.CanEdit, d.CanPost, d.CanCancel
	return out
}

func (h *Handler) documents(w http.ResponseWriter, r *http.Request) {
	p := pagination.FromRequest(r)
	q := r.URL.Query()
	f := application.DocFilter{Kind: q.Get("kind"), Status: q.Get("status"), WarehouseID: queryInt(r, "warehouse_id"),
		From: kernel.Date{Year: 1000, Month: 1, Day: 1}, To: kernel.Date{Year: 9999, Month: 12, Day: 31},
		Search: application.LikePattern(q.Get("q")), Limit: p.Limit, Offset: p.Offset()}
	if s := q.Get("from"); s != "" {
		d, ok := parseDate(s)
		if !ok {
			fail(w, r, apperror.Validation([]validator.FieldError{{Field: "from", Message: "tanggal tidak valid"}}))
			return
		}
		f.From = d
	}
	if s := q.Get("to"); s != "" {
		d, ok := parseDate(s)
		if !ok {
			fail(w, r, apperror.Validation([]validator.FieldError{{Field: "to", Message: "tanggal tidak valid"}}))
			return
		}
		f.To = d
	}
	rows, total, err := h.svc.Documents(r.Context(), f)
	if err != nil {
		fail(w, r, err)
		return
	}
	out := make([]documentDTO, len(rows))
	for i, row := range rows {
		out[i] = toDocumentDTO(row.Document, row.WarehouseCode, row.WarehouseName)
		out[i].TotalValue = row.TotalValue
	}
	response.Paginated(w, "Dokumen stok", out, pagination.NewMeta(p, total))
}

func (h *Handler) document(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.Document(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Dokumen stok", toDetailDTO(d))
}

type lineRequest struct {
	ItemID   int64         `json:"item_id"`
	UnitCode string        `json:"unit_code"`
	Qty      money.Qty     `json:"qty"`
	UnitCost *money.Amount `json:"unit_cost"`
	ZeroCost bool          `json:"zero_cost"`
}

type documentRequest struct {
	Kind         string        `json:"kind"`
	WarehouseID  int64         `json:"warehouse_id"`
	Date         string        `json:"date"`
	Counterparty string        `json:"counterparty"`
	Reference    string        `json:"reference"`
	Notes        string        `json:"notes"`
	Lines        []lineRequest `json:"lines"`
}

func (in documentRequest) document() (domain.Document, error) {
	d := domain.Document{Kind: domain.Kind(in.Kind), WarehouseID: in.WarehouseID, Counterparty: in.Counterparty, Reference: in.Reference, Notes: in.Notes}
	if in.Date != "" {
		date, ok := parseDate(in.Date)
		if !ok {
			return d, apperror.Validation([]validator.FieldError{{Field: "date", Message: "tanggal tidak valid"}})
		}
		d.Date = date
	}
	for _, l := range in.Lines {
		d.Lines = append(d.Lines, domain.Line{ItemID: l.ItemID, UnitCode: l.UnitCode, Qty: l.Qty, UnitCost: l.UnitCost, ZeroCost: l.ZeroCost})
	}
	return d, nil
}

func (h *Handler) createDocument(w http.ResponseWriter, r *http.Request) {
	var in documentRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	d, err := in.document()
	if err != nil {
		fail(w, r, err)
		return
	}
	saved, err := h.svc.CreateDraft(r.Context(), d)
	if err != nil {
		fail(w, r, err)
		return
	}
	response.Created(w, "Draf disimpan", toDetailDTO(saved))
}

func (h *Handler) updateDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in documentRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	d, err := in.document()
	if err != nil {
		fail(w, r, err)
		return
	}
	saved, err := h.svc.UpdateDraft(r.Context(), id, d)
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Draf disimpan", toDetailDTO(saved))
}

func (h *Handler) deleteDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteDraft(r.Context(), id); err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Draf dihapus", nil)
}

func (h *Handler) post(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.Post(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Dokumen diposting", toDetailDTO(d))
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	d, err := h.svc.Cancel(r.Context(), id, in.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Dokumen dibatalkan", toDetailDTO(d))
}

func (h *Handler) amend(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.Amend(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	response.Created(w, "Draf koreksi dibuat", toDetailDTO(d))
}
