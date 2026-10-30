// Package presentation is the items module's HTTP API.
package presentation

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/items/application"
	"__GO_MODULE__/internal/modules/items/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
	"__GO_MODULE__/internal/shared/validator"
)

// Handler serves /api/v1/items.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

// Routes mounts the handlers. Reading needs items.read, changes items.manage; the code prefix is
// checked across the company by the service.
func (h *Handler) Routes(mux *http.ServeMux) {
	read := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermRead, f) }
	manage := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermManage, f) }
	const base = "/api/v1/items"
	mux.Handle("GET "+base, read(h.list))
	mux.Handle("GET "+base+"/lookup", read(h.lookup))
	mux.Handle("POST "+base, manage(h.create))
	mux.Handle("GET "+base+"/settings", read(h.settings))
	mux.Handle("PUT "+base+"/settings", manage(h.setSettings))
	mux.Handle("GET "+base+"/units", read(h.units))
	mux.Handle("POST "+base+"/units", manage(h.createUnit))
	mux.Handle("POST "+base+"/units/defaults", manage(h.addCommonUnits))
	mux.Handle("PUT "+base+"/units/{id}", manage(h.updateUnit))
	mux.Handle("GET "+base+"/categories", read(h.categories))
	mux.Handle("POST "+base+"/categories", manage(h.createCategory))
	mux.Handle("PUT "+base+"/categories/{id}", manage(h.updateCategory))
	mux.Handle("GET "+base+"/{id}", read(h.detail))
	mux.Handle("PUT "+base+"/{id}", manage(h.update))
	mux.Handle("POST "+base+"/{id}/deactivate", manage(h.setActive(false)))
	mux.Handle("POST "+base+"/{id}/activate", manage(h.setActive(true)))
	mux.Handle("POST "+base+"/{id}/units", manage(h.addItemUnit))
	mux.Handle("POST "+base+"/{id}/units/{uid}/deactivate", manage(h.deactivateItemUnit))
}

type itemDTO struct {
	ID           int64     `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	IsStocked    bool      `json:"is_stocked"`
	CategoryID   *int64    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	BaseUnitID   int64     `json:"base_unit_id"`
	BaseUnitCode string    `json:"base_unit_code"`
	BaseUnitName string    `json:"base_unit_name"`
	Barcode      string    `json:"barcode"`
	TaxCode      string    `json:"tax_code"`
	Description  string    `json:"description"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toDTO(it domain.Item) itemDTO {
	d := itemDTO{
		ID: it.ID, Code: it.Code, Name: it.Name, Kind: string(it.Kind), IsStocked: it.IsStocked, CategoryName: it.CategoryName,
		BaseUnitID: it.BaseUnitID, BaseUnitCode: it.BaseUnitCode, BaseUnitName: it.BaseUnitName, Barcode: it.Barcode,
		TaxCode: it.TaxCode, Description: it.Description, IsActive: it.IsActive, CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt,
	}
	if it.CategoryID != 0 {
		id := it.CategoryID
		d.CategoryID = &id
	}
	return d
}

func toDTOs(items []domain.Item) []itemDTO {
	out := make([]itemDTO, len(items))
	for i, it := range items {
		out[i] = toDTO(it)
	}
	return out
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := request.PathInt64(r, name)
	if err != nil || id <= 0 {
		response.Error(w, r, apperror.NotFound("Tidak ditemukan"))
		return 0, false
	}
	return id, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := pagination.FromRequest(r)
	q := r.URL.Query()
	f := application.ListFilter{Search: q.Get("q"), Kind: q.Get("kind"), Status: q.Get("status"), Limit: p.Limit, Offset: p.Offset()}
	if c := q.Get("category_id"); c != "" {
		id, err := strconv.ParseInt(c, 10, 64)
		if err != nil || id < 0 {
			response.Error(w, r, apperror.Validation([]validator.FieldError{{Field: "category_id", Message: "kategori tidak dikenal"}}))
			return
		}
		f.CategoryID = id
	}
	rows, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Paginated(w, "Daftar barang", toDTOs(rows), pagination.NewMeta(p, total))
}

type lookupDTO struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	IsStocked bool   `json:"is_stocked"`
	BaseUnit  string `json:"base_unit"`
}

// lookup is the active items matching ?q=, for pickers in other modules' pages.
func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.Search(r.Context(), r.URL.Query().Get("q"), 20)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]lookupDTO, len(rows))
	for i, it := range rows {
		out[i] = lookupDTO{ID: it.ID, Code: it.Code, Name: it.Name, Kind: string(it.Kind), IsStocked: it.IsStocked, BaseUnit: it.BaseUnitCode}
	}
	response.OK(w, "Barang", out)
}

type itemUnitDTO struct {
	ID       int64      `json:"id"`
	UnitID   int64      `json:"unit_id"`
	Code     string     `json:"code"`
	Name     string     `json:"name"`
	Factor   money.Rate `json:"factor"`
	IsActive bool       `json:"is_active"`
}

func toItemUnitDTO(iu domain.ItemUnit) itemUnitDTO {
	return itemUnitDTO{ID: iu.ID, UnitID: iu.UnitID, Code: iu.UnitCode, Name: iu.UnitName, Factor: iu.Factor, IsActive: iu.IsActive}
}

// canDTO tells the web what needs a permission across the company, which useCan cannot answer.
type canDTO struct {
	Settings bool `json:"settings"`
}

type detailDTO struct {
	Item  itemDTO       `json:"item"`
	Units []itemUnitDTO `json:"units"`
	Can   canDTO        `json:"can"`
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	d, err := h.svc.Detail(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := detailDTO{Item: toDTO(d.Item), Units: make([]itemUnitDTO, len(d.Units)), Can: canDTO{Settings: h.svc.CanCompanyWide(r.Context(), application.PermManage)}}
	for i, iu := range d.Units {
		out.Units[i] = toItemUnitDTO(iu)
	}
	response.OK(w, "Barang", out)
}

// itemInput is an item as the form sends it. On PUT, a field left out is left as it is.
type itemInput struct {
	Code        *string `json:"code"`
	Name        *string `json:"name"`
	Kind        *string `json:"kind"`
	IsStocked   *bool   `json:"is_stocked"`
	CategoryID  *int64  `json:"category_id"`
	BaseUnitID  *int64  `json:"base_unit_id"`
	Barcode     *string `json:"barcode"`
	TaxCode     *string `json:"tax_code"`
	Description *string `json:"description"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func idOf(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in itemInput
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	it, err := h.svc.Create(r.Context(), application.Input{
		Item: domain.Item{Code: str(in.Code), Name: str(in.Name), Kind: domain.Kind(str(in.Kind)), CategoryID: idOf(in.CategoryID),
			BaseUnitID: idOf(in.BaseUnitID), Barcode: str(in.Barcode), TaxCode: str(in.TaxCode), Description: str(in.Description)},
		IsStocked: in.IsStocked,
	})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Barang ditambahkan", toDTO(it))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	itemID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in itemInput
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	it, err := h.svc.Update(r.Context(), itemID, application.Patch{Code: in.Code, Name: in.Name, Kind: in.Kind, IsStocked: in.IsStocked,
		CategoryID: in.CategoryID, BaseUnitID: in.BaseUnitID, Barcode: in.Barcode, TaxCode: in.TaxCode, Description: in.Description})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Barang disimpan", toDTO(it))
}

func (h *Handler) setActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemID, ok := pathID(w, r, "id")
		if !ok {
			return
		}
		if err := h.svc.SetActive(r.Context(), itemID, active); err != nil {
			response.Error(w, r, err)
			return
		}
		response.OK(w, map[bool]string{true: "Barang diaktifkan", false: "Barang dinonaktifkan"}[active], nil)
	}
}

// itemUnitInput is an alternative unit as the form sends it. The factor is a string, parsed here
// so a bad one gets a field message.
type itemUnitInput struct {
	UnitID int64  `json:"unit_id"`
	Factor string `json:"factor"`
}

func (h *Handler) addItemUnit(w http.ResponseWriter, r *http.Request) {
	itemID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in itemUnitInput
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	factor, err := money.ParseRate(strings.TrimSpace(in.Factor))
	if err != nil {
		response.Error(w, r, apperror.Validation([]validator.FieldError{{Field: "factor",
			Message: "angka lebih dari 0 dengan paling banyak 9 digit bulat dan 10 desimal, mis. 12 atau 0.5"}}))
		return
	}
	iu, err := h.svc.AddUnit(r.Context(), itemID, in.UnitID, factor)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Satuan ditambahkan", toItemUnitDTO(iu))
}

func (h *Handler) deactivateItemUnit(w http.ResponseWriter, r *http.Request) {
	itemID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := pathID(w, r, "uid")
	if !ok {
		return
	}
	if err := h.svc.DeactivateUnit(r.Context(), itemID, uid); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Satuan dinonaktifkan", nil)
}

type unitDTO struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	TaxUnitCode string `json:"tax_unit_code"`
	IsActive    bool   `json:"is_active"`
}

func toUnitDTO(u domain.Unit) unitDTO {
	return unitDTO{ID: u.ID, Code: u.Code, Name: u.Name, TaxUnitCode: u.TaxUnitCode, IsActive: u.IsActive}
}

func (h *Handler) units(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.Units(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]unitDTO, len(rows))
	for i, u := range rows {
		out[i] = toUnitDTO(u)
	}
	response.OK(w, "Satuan", out)
}

// unitInput is a unit as the form sends it; on PUT the code is ignored (it is fixed) and a field
// left out is left as it is.
type unitInput struct {
	Code        *string `json:"code"`
	Name        *string `json:"name"`
	TaxUnitCode *string `json:"tax_unit_code"`
	IsActive    *bool   `json:"is_active"`
}

func (h *Handler) createUnit(w http.ResponseWriter, r *http.Request) {
	var in unitInput
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	u, err := h.svc.CreateUnit(r.Context(), domain.Unit{Code: str(in.Code), Name: str(in.Name), TaxUnitCode: str(in.TaxUnitCode)})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Satuan ditambahkan", toUnitDTO(u))
}

func (h *Handler) updateUnit(w http.ResponseWriter, r *http.Request) {
	unitID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in unitInput
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	u, err := h.svc.UpdateUnit(r.Context(), unitID, application.UnitPatch{Name: in.Name, TaxUnitCode: in.TaxUnitCode, IsActive: in.IsActive})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Satuan disimpan", toUnitDTO(u))
}

func (h *Handler) addCommonUnits(w http.ResponseWriter, r *http.Request) {
	added, err := h.svc.AddCommonUnits(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	msg := "Semua satuan umum sudah ada"
	if len(added) > 0 {
		msg = strconv.Itoa(len(added)) + " satuan umum ditambahkan"
	}
	response.OK(w, msg, map[string][]string{"added": added})
}

type categoryDTO struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

func toCategoryDTO(c domain.Category) categoryDTO {
	return categoryDTO{ID: c.ID, Name: c.Name, IsActive: c.IsActive}
}

func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.Categories(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]categoryDTO, len(rows))
	for i, c := range rows {
		out[i] = toCategoryDTO(c)
	}
	response.OK(w, "Kategori", out)
}

type categoryInput struct {
	Name     *string `json:"name"`
	IsActive *bool   `json:"is_active"`
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var in categoryInput
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), domain.Category{Name: str(in.Name)})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Kategori ditambahkan", toCategoryDTO(c))
}

func (h *Handler) updateCategory(w http.ResponseWriter, r *http.Request) {
	catID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in categoryInput
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	c, err := h.svc.UpdateCategory(r.Context(), catID, application.CategoryPatch{Name: in.Name, IsActive: in.IsActive})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Kategori disimpan", toCategoryDTO(c))
}

type settingsDTO struct {
	CodePrefix string `json:"code_prefix"`
	NextCode   string `json:"next_code"`
	CanManage  bool   `json:"can_manage"`
}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Settings(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Pengaturan barang", settingsDTO{CodePrefix: st.CodePrefix, NextCode: domain.GeneratedCode(st.CodePrefix, st.LastNumber+1),
		CanManage: h.svc.CanCompanyWide(r.Context(), application.PermManage)})
}

func (h *Handler) setSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsDTO
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.SetPrefix(r.Context(), in.CodePrefix); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Pengaturan barang disimpan", nil)
}
