package loom

import (
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	contractsdb "github.com/goravel/framework/contracts/database/db"
	contractshttp "github.com/goravel/framework/contracts/http"
	contractsvalidation "github.com/goravel/framework/contracts/validation"

	"goravel/packages/loom/form"
	"goravel/packages/loom/table"
)

// crudHandler serves the generic list/create/edit/delete screens of one resource.
type crudHandler struct {
	panel *Panel
	res   *resourceEntry
}

// listState is the current table state taken from the query string.
type listState struct {
	Search  string
	Sort    string
	Dir     string
	Page    int
	PerPage int
	Filters map[string]string
}

func (h *crudHandler) readState(ctx contractshttp.Context) listState {
	req := ctx.Request()
	t := h.res.table

	st := listState{
		Search:  strings.TrimSpace(req.Query("q")),
		Page:    max(req.QueryInt("page", 1), 1),
		PerPage: t.GetPerPage(),
		Filters: map[string]string{},
	}
	if n := req.QueryInt("per_page", 0); n == 10 || n == 25 || n == 50 || n == 100 {
		st.PerPage = n
	}

	// Only declared sortable columns are accepted (they are interpolated in ORDER BY).
	if c := t.Column(req.Query("sort")); c != nil && c.IsSortable() {
		st.Sort = c.Name()
		st.Dir = "asc"
		if req.Query("dir") == "desc" {
			st.Dir = "desc"
		}
	}

	for _, f := range t.GetFilters() {
		if v := req.Query("filter_" + f.Name()); f.HasOption(v) {
			st.Filters[f.Name()] = v
		}
	}
	return st
}

// URL builds the list URL with the given overrides applied to the current state.
func (st listState) URL(base string, overrides map[string]string) string {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("q", st.Search)
	set("sort", st.Sort)
	set("dir", st.Dir)
	if st.Page > 1 {
		q.Set("page", strconv.Itoa(st.Page))
	}
	for k, v := range st.Filters {
		set("filter_"+k, v)
	}
	for k, v := range overrides {
		if v == "" {
			q.Del(k)
		} else {
			q.Set(k, v)
		}
	}
	if len(q) == 0 {
		return base
	}
	return base + "?" + q.Encode()
}

func (h *crudHandler) index(ctx contractshttp.Context) contractshttp.Response {
	p, res := h.panel, h.res
	st := h.readState(ctx)

	// Count() mutates the query's select list, so build a fresh query for each use.
	build := func() contractsdb.Query {
		query := p.svc.DB.WithContext(ctx).Table(res.TableName())
		if m, ok := res.Resource.(ModifiesListQuery); ok {
			query = m.ModifyListQuery(Context{HTTP: ctx, Panel: p}, query)
		}
		return h.applyFilters(query, st)
	}

	total, err := build().Count()
	if err != nil {
		return p.serverError(ctx, err)
	}
	lastPage := max(int(math.Ceil(float64(total)/float64(st.PerPage))), 1)
	if st.Page > lastPage {
		st.Page = lastPage
	}

	sortCol, sortDir := st.Sort, st.Dir
	if sortCol == "" {
		sortCol, sortDir = res.table.GetDefaultSort()
		if sortCol == "" {
			sortCol = res.PrimaryKey()
		}
	}
	if sortDir != "asc" {
		sortDir = "desc"
	}

	listQuery := build()
	if cols := res.listSelect(); cols != nil {
		listQuery = listQuery.Select(cols...)
	}
	rows, err := fetchRows(listQuery.OrderBy(sortCol, sortDir).
		Offset(uint64((st.Page - 1) * st.PerPage)).
		Limit(uint64(st.PerPage)))
	if err != nil {
		return p.serverError(ctx, err)
	}

	type headerCell struct {
		Column   *table.Column
		SortURL  string
		SortedBy string // "", "asc", "desc"
	}
	base := p.URL(res.Slug())
	headers := make([]headerCell, 0, len(res.table.GetColumns()))
	for _, c := range res.table.GetColumns() {
		hc := headerCell{Column: c}
		if c.IsSortable() {
			next := "asc"
			if st.Sort == c.Name() {
				hc.SortedBy = st.Dir
				if st.Dir == "asc" {
					next = "desc"
				}
			}
			hc.SortURL = st.URL(base, map[string]string{"sort": c.Name(), "dir": next, "page": ""})
		}
		headers = append(headers, hc)
	}

	type pageLink struct {
		Number  int
		URL     string
		Current bool
		Gap     bool
	}
	var pages []pageLink
	for i := 1; i <= lastPage; i++ {
		if i == 1 || i == lastPage || (i >= st.Page-2 && i <= st.Page+2) {
			pages = append(pages, pageLink{Number: i, URL: st.URL(base, map[string]string{"page": strconv.Itoa(i)}), Current: i == st.Page})
		} else if len(pages) > 0 && !pages[len(pages)-1].Gap {
			pages = append(pages, pageLink{Gap: true})
		}
	}

	from, to := 0, 0
	if total > 0 {
		from = (st.Page-1)*st.PerPage + 1
		to = from + len(rows) - 1
	}

	hasSearch := false
	for _, c := range res.table.GetColumns() {
		hasSearch = hasSearch || c.IsSearchable()
	}

	pk := res.PrimaryKey()
	type rowView struct {
		ID  string
		Row table.Row
	}
	views := make([]rowView, 0, len(rows))
	for _, r := range rows {
		views = append(views, rowView{ID: table.Stringify(r[pk]), Row: r})
	}

	return p.page(ctx, "index", 200, map[string]any{
		"Title":     res.PluralLabel(),
		"Resource":  res,
		"Headers":   headers,
		"Columns":   res.table.GetColumns(),
		"Filters":   res.table.GetFilters(),
		"Rows":      views,
		"State":     st,
		"Total":     total,
		"From":      from,
		"To":        to,
		"Pages":     pages,
		"HasSearch": hasSearch,
		"BaseURL":   base,
		"PrevURL":   pageURL(st, base, st.Page-1, lastPage),
		"NextURL":   pageURL(st, base, st.Page+1, lastPage),
	})
}

// applyFilters adds the search term and the select filters to the query.
func (h *crudHandler) applyFilters(query contractsdb.Query, st listState) contractsdb.Query {
	res := h.res
	if st.Search != "" {
		var searchable []string
		for _, c := range res.table.GetColumns() {
			if c.IsSearchable() {
				searchable = append(searchable, c.Name())
			}
		}
		if len(searchable) > 0 {
			term := "%" + strings.ToLower(escapeLike(st.Search)) + "%"
			query = query.Where(func(q contractsdb.Query) contractsdb.Query {
				for i, col := range searchable {
					raw := "LOWER(" + col + ") LIKE ?"
					if i == 0 {
						q = q.WhereRaw(raw, []any{term})
					} else {
						q = q.OrWhereRaw(raw, []any{term})
					}
				}
				return q
			})
		}
	}
	for col, v := range st.Filters {
		switch v {
		case "true", "false":
			query = query.Where(col, v == "true") // bind as a real boolean
		default:
			query = query.Where(col, v)
		}
	}

	return query
}

func pageURL(st listState, base string, page, last int) string {
	if page < 1 || page > last {
		return ""
	}
	return st.URL(base, map[string]string{"page": strconv.Itoa(page)})
}

func (h *crudHandler) create(ctx contractshttp.Context) contractshttp.Response {
	values := map[string]string{}
	for _, f := range h.res.form.Fields(false) {
		values[f.Name()] = f.DefaultValue()
	}
	return h.renderForm(ctx, "", values, nil, 200)
}

func (h *crudHandler) edit(ctx contractshttp.Context) contractshttp.Response {
	id := ctx.Request().Route("id")
	row, err := h.panel.findRow(ctx, h.res.TableName(), h.res.PrimaryKey(), normalizeID(id))
	if err != nil {
		return h.panel.serverError(ctx, err)
	}
	if row == nil {
		return h.panel.notFound(ctx)
	}

	values := map[string]string{}
	for _, f := range h.res.form.Fields(true) {
		switch f.Kind() {
		case form.KindPassword:
			values[f.Name()] = ""
		case form.KindFile:
			if hasFileValue(row[f.Name()]) {
				values[f.Name()] = "1"
			}
		case form.KindToggle:
			values[f.Name()] = strconv.FormatBool(table.Boolean(f.Name()).Truthy(row))
		case form.KindDate:
			if t, ok := row[f.Name()].(time.Time); ok {
				values[f.Name()] = t.Format("2006-01-02")
			} else {
				values[f.Name()] = table.Stringify(row[f.Name()])
			}
		default:
			values[f.Name()] = table.Stringify(row[f.Name()])
		}
	}
	return h.renderForm(ctx, id, values, nil, 200)
}

func (h *crudHandler) store(ctx contractshttp.Context) contractshttp.Response {
	return h.save(ctx, "")
}

func (h *crudHandler) update(ctx contractshttp.Context) contractshttp.Response {
	id := ctx.Request().Route("id")
	exists, err := h.panel.svc.DB.WithContext(ctx).Table(h.res.TableName()).Where(h.res.PrimaryKey(), normalizeID(id)).Exists()
	if err != nil {
		return h.panel.serverError(ctx, err)
	}
	if !exists {
		return h.panel.notFound(ctx)
	}
	return h.save(ctx, id)
}

// save validates the submitted form and inserts (id == "") or updates the record.
func (h *crudHandler) save(ctx contractshttp.Context, id string) contractshttp.Response {
	p, res := h.panel, h.res
	isEdit := id != ""
	fields := res.form.Fields(isEdit)
	req := ctx.Request()

	raw := map[string]any{}
	values := map[string]string{}
	rules := map[string]any{}
	hasFiles := false
	for _, f := range fields {
		if f.Kind() == form.KindFile {
			hasFiles = true
			continue
		}
		v := req.Input(f.Name())
		if f.Kind() != form.KindPassword && f.Kind() != form.KindTextarea {
			v = strings.TrimSpace(v)
		}
		if f.Kind() == form.KindToggle {
			v = strconv.FormatBool(v == "1" || v == "on" || v == "true")
		}
		values[f.Name()] = v
		if v != "" {
			raw[f.Name()] = v
		}
		if r := f.ValidationRules(res.TableName(), id); r != "" && f.Kind() != form.KindToggle {
			rules[f.Name()] = r
		}
	}

	errs := map[string]string{}
	if len(rules) > 0 {
		attributes := map[string]string{}
		for _, f := range fields {
			attributes[f.Name()] = f.LabelText()
		}
		validator, err := p.svc.Validation.Make(ctx, raw, rules, func(o *contractsvalidation.Options) {
			o.Attributes = attributes // "The Nome field..." instead of "The name field..."
		})
		if err != nil {
			return p.serverError(ctx, err)
		}
		if validator.Fails() {
			for _, f := range fields {
				if msg := validator.Errors().One(f.Name()); msg != "" {
					errs[f.Name()] = msg
				}
			}
		}
	}

	// Files: validate every upload before storing anything.
	var current table.Row
	if hasFiles && isEdit {
		row, err := p.findRow(ctx, res.TableName(), res.PrimaryKey(), normalizeID(id))
		if err != nil {
			return p.serverError(ctx, err)
		}
		current = row
	}
	uploads, fileErrs := h.collectUploads(ctx, fields, current)
	defer uploads.cleanup()
	for k, v := range fileErrs {
		errs[k] = v
	}
	for _, f := range fields {
		if f.Kind() == form.KindFile && current != nil && hasFileValue(current[f.Name()]) {
			values[f.Name()] = "1" // keep showing the current file after a validation error
		}
	}

	if len(errs) > 0 {
		for _, f := range fields {
			if f.Kind() == form.KindPassword {
				values[f.Name()] = ""
			}
		}
		status := 422
		if isHtmx(ctx) {
			status = 200 // htmx does not swap error responses by default
		}
		return h.renderForm(ctx, id, values, errs, status)
	}

	data := map[string]any{}
	for _, f := range fields {
		v := values[f.Name()]
		switch f.Kind() {
		case form.KindFile:
			// handled by uploads.apply below
		case form.KindPassword:
			if v == "" {
				continue // keep current password on edit
			}
			hashed, err := p.svc.Hash.Make(v)
			if err != nil {
				return p.serverError(ctx, err)
			}
			data[f.Name()] = hashed
		case form.KindToggle:
			data[f.Name()] = v == "true"
		case form.KindNumber:
			if v == "" {
				data[f.Name()] = nil
			} else if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				data[f.Name()] = n
			} else {
				fl, _ := strconv.ParseFloat(v, 64)
				data[f.Name()] = fl
			}
		case form.KindSelect, form.KindDate:
			if v == "" {
				data[f.Name()] = nil
			} else {
				data[f.Name()] = v
			}
		default:
			if v == "" && f.IsNullable() {
				data[f.Name()] = nil
			} else {
				data[f.Name()] = v
			}
		}
	}

	// Page hooks may change data or stop with Halt("message").
	hookCtx := Context{HTTP: ctx, Panel: p}
	var hookErr error
	if hook, ok := res.Resource.(BeforeSaveHook); ok && isEdit {
		hookErr = hook.BeforeSave(hookCtx, id, data)
	} else if hook, ok := res.Resource.(BeforeCreateHook); ok && !isEdit {
		hookErr = hook.BeforeCreate(hookCtx, data)
	}
	if hookErr != nil {
		msg, halted := haltMessage(hookErr)
		if !halted {
			return p.serverError(ctx, hookErr)
		}
		for _, f := range fields {
			if f.Kind() == form.KindPassword {
				values[f.Name()] = ""
			}
		}
		status := 422
		if isHtmx(ctx) {
			status = 200
		}
		return h.renderFormWith(ctx, id, values, errs, status, msg)
	}

	if err := uploads.apply(ctx, p, data); err != nil {
		uploads.rollback(p)
		return p.serverError(ctx, err)
	}

	now := time.Now()
	query := p.svc.DB.WithContext(ctx).Table(res.TableName())
	if res.Timestamps() {
		data["updated_at"] = now
	}
	if isEdit {
		if _, err := query.Where(res.PrimaryKey(), normalizeID(id)).Update(data); err != nil {
			uploads.rollback(p)
			return p.serverError(ctx, err)
		}
		uploads.commit(p)
		if hook, ok := res.Resource.(AfterSaveHook); ok {
			if err := hook.AfterSave(hookCtx, id, data); err != nil {
				return p.serverError(ctx, err)
			}
		}
		if ctx.Value(ctxNotifiedKey) == nil {
			flash(ctx, "success", res.Singular()+" atualizado com sucesso.")
		}
	} else {
		if res.Timestamps() {
			data["created_at"] = now
		}
		if _, err := query.Insert(data); err != nil {
			uploads.rollback(p)
			return p.serverError(ctx, err)
		}
		uploads.commit(p)
		if hook, ok := res.Resource.(AfterCreateHook); ok {
			if err := hook.AfterCreate(hookCtx, data); err != nil {
				return p.serverError(ctx, err)
			}
		}
		if ctx.Value(ctxNotifiedKey) == nil {
			flash(ctx, "success", res.Singular()+" criado com sucesso.")
		}
	}
	return redirect(ctx, p.URL(res.Slug()))
}

func (h *crudHandler) renderForm(ctx contractshttp.Context, id string, values, errs map[string]string, status int) contractshttp.Response {
	return h.renderFormWith(ctx, id, values, errs, status, "")
}

// renderFormWith also shows formError (e.g. from a Halt in a page hook) above the form.
func (h *crudHandler) renderFormWith(ctx contractshttp.Context, id string, values, errs map[string]string, status int, formError string) contractshttp.Response {
	res := h.res
	isEdit := id != ""
	title := "Novo " + lowerWords(res.Singular())
	action := h.panel.URL(res.Slug())
	if isEdit {
		title = "Editar " + lowerWords(res.Singular())
		action = h.panel.URL(res.Slug(), id)
	}
	if errs == nil {
		errs = map[string]string{}
	}
	data := map[string]any{
		"Title":    title,
		"Resource": res,
		"Fields":   res.form.Fields(isEdit),
		"Values":   values,
		"Errors":   errs,
		"Action":   action,
		"IsEdit":   isEdit,
		"ID":       id,
		"BackURL":  h.panel.URL(res.Slug()),
		"HasFiles": res.hasFileFields(),
		"FilesURL": h.panel.URL(res.Slug(), id, "files"),
	}
	if formError != "" {
		data["FlashError"] = formError
	}
	return h.panel.page(ctx, "form", status, data)
}

func (h *crudHandler) destroy(ctx contractshttp.Context) contractshttp.Response {
	id := ctx.Request().Route("id")
	if resp := h.beforeDelete(ctx, []any{normalizeID(id)}); resp != nil {
		return resp
	}
	files, err := h.diskFilesOf(ctx, []any{normalizeID(id)})
	if err != nil {
		return h.panel.serverError(ctx, err)
	}
	result, err := h.panel.svc.DB.WithContext(ctx).Table(h.res.TableName()).Where(h.res.PrimaryKey(), normalizeID(id)).Delete()
	if err != nil {
		return h.panel.serverError(ctx, err)
	}
	if result.RowsAffected == 0 {
		return h.panel.notFound(ctx)
	}
	h.panel.deleteDiskFiles(files)
	flash(ctx, "success", h.res.Singular()+" excluído.")
	return redirect(ctx, h.panel.URL(h.res.Slug()))
}

func (h *crudHandler) bulkDelete(ctx contractshttp.Context) contractshttp.Response {
	ids := ctx.Request().InputArray("ids")
	if len(ids) == 0 {
		flash(ctx, "error", "Nenhum registro selecionado.")
		return redirect(ctx, h.panel.URL(h.res.Slug()))
	}
	values := make([]any, 0, len(ids))
	for _, id := range ids {
		values = append(values, normalizeID(id))
	}
	if resp := h.beforeDelete(ctx, values); resp != nil {
		return resp
	}
	files, err := h.diskFilesOf(ctx, values)
	if err != nil {
		return h.panel.serverError(ctx, err)
	}
	result, err := h.panel.svc.DB.WithContext(ctx).Table(h.res.TableName()).WhereIn(h.res.PrimaryKey(), values).Delete()
	if err != nil {
		return h.panel.serverError(ctx, err)
	}
	h.panel.deleteDiskFiles(files)
	flash(ctx, "success", strconv.FormatInt(result.RowsAffected, 10)+" registro(s) excluído(s).")
	return redirect(ctx, h.panel.URL(h.res.Slug()))
}

// beforeDelete runs the BeforeDelete hook; a non-nil response stops the delete.
func (h *crudHandler) beforeDelete(ctx contractshttp.Context, ids []any) contractshttp.Response {
	hook, ok := h.res.Resource.(BeforeDeleteHook)
	if !ok {
		return nil
	}
	err := hook.BeforeDelete(Context{HTTP: ctx, Panel: h.panel}, ids)
	if err == nil {
		return nil
	}
	if msg, halted := haltMessage(err); halted {
		flash(ctx, "error", msg)
		return redirect(ctx, h.panel.URL(h.res.Slug()))
	}
	return h.panel.serverError(ctx, err)
}

// fetchRows runs the query and returns every row as a column => value map.
func fetchRows(query contractsdb.Query) ([]table.Row, error) {
	var rows []table.Row
	var firstErr error
	for row := range query.Cursor() { // the channel must be drained
		if err := row.Err(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		m := map[string]any{}
		if err := row.Scan(&m); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		rows = append(rows, m)
	}
	return rows, firstErr
}

// findRow returns the first row where column = value, or nil if none.
func (p *Panel) findRow(ctx contractshttp.Context, tableName, column string, value any) (table.Row, error) {
	rows, err := fetchRows(p.svc.DB.WithContext(ctx).Table(tableName).Where(column, value).Limit(1))
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// normalizeID converts numeric ids to int64 so they bind to integer columns;
// anything else (e.g. UUIDs) is kept as a string.
func normalizeID(id string) any {
	if n, err := strconv.ParseInt(id, 10, 64); err == nil {
		return n
	}
	return id
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func isHtmx(ctx contractshttp.Context) bool {
	return ctx.Request().Header("HX-Request") == "true"
}

// redirect works for normal requests, boosted links/forms and plain htmx calls.
func redirect(ctx contractshttp.Context, location string) contractshttp.AbortableResponse {
	if isHtmx(ctx) && ctx.Request().Header("HX-Boosted") != "true" {
		ctx.Response().Header("HX-Redirect", location)
		return ctx.Response().NoContent(200)
	}
	return ctx.Response().Redirect(302, location)
}

func flash(ctx contractshttp.Context, kind, message string) {
	ctx.Request().Session().Flash("flash_"+kind, message)
}
