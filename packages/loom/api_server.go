package loom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	contractsdb "github.com/goravel/framework/contracts/database/db"
	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
	contractsvalidation "github.com/goravel/framework/contracts/validation"

	"goravel/packages/loom/api"
	"goravel/packages/loom/table"
)

// APIResource exposes a database table as a JSON REST resource.
//
// Embed BaseAPIResource and implement Slug, TableName and Fields:
//
//	type Resource struct{ loom.BaseAPIResource }
//	func (Resource) Slug() string             { return "cars" }
//	func (Resource) TableName() string        { return "cars" }
//	func (Resource) Fields(s *api.Schema)     { s.Schema(api.Field("brand").Required()) }
//	func (Resource) Access() api.Access       { return api.ReadOnly() }
//
// The page hooks of panel resources work here too (BeforeCreate, AfterSave,
// BeforeDelete, ModifyListQuery...), plus ModifyQuery, which scopes every
// endpoint (list, show, update and delete).
type APIResource interface {
	Slug() string
	TableName() string
	Fields(s *api.Schema)
	Access() api.Access
	PrimaryKey() string
	Timestamps() bool
}

// BaseAPIResource provides defaults: primary key "id", managed timestamps and
// every endpoint requiring a token.
type BaseAPIResource struct{}

func (BaseAPIResource) Access() api.Access { return api.Access{} }
func (BaseAPIResource) PrimaryKey() string { return "id" }
func (BaseAPIResource) Timestamps() bool   { return true }

// ModifiesQuery scopes every endpoint of an API resource, e.g. to the records
// of the token owner.
type ModifiesQuery interface {
	ModifyQuery(ctx Context, query contractsdb.Query) contractsdb.Query
}

type apiEntry struct {
	APIResource
	schema *api.Schema
}

// API serves JSON resources under a prefix, authenticated by Bearer tokens.
type API struct {
	svc            Services
	prefix         string
	resources      []*apiEntry
	middleware     []contractshttp.Middleware
	routes         []func(r route.Router)
	rateLimit      int
	authTable      string
	emailColumn    string
	passwordColumn string
	nameColumn     string
	tokenTable     string
	canAccess      func(user table.Row) bool
	tokenLogin     bool
	loginAbilities []string
	tokenTTL       time.Duration
	limiter        *rateLimiter

	pgOnce     sync.Once
	isPostgres bool
}

// NewAPI returns an API mounted at /api with a 60 requests/minute limit.
func NewAPI() *API {
	return &API{
		prefix:         "/api",
		rateLimit:      60,
		authTable:      "users",
		emailColumn:    "email",
		passwordColumn: "password",
		nameColumn:     "name",
		tokenTable:     "api_tokens",
	}
}

// Prefix sets the URL prefix, e.g. "/api/v1".
func (a *API) Prefix(prefix string) *API { a.prefix = "/" + strings.Trim(prefix, "/"); return a }

// Resources registers API resources.
func (a *API) Resources(resources ...APIResource) *API {
	for _, r := range resources {
		s := api.NewSchema()
		r.Fields(s)
		a.resources = append(a.resources, &apiEntry{APIResource: r, schema: s})
	}
	return a
}

// RateLimit sets the requests per minute per client (token, or IP without one). 0 disables it.
func (a *API) RateLimit(perMinute int) *API { a.rateLimit = perMinute; return a }

// Middleware adds Goravel middleware to every API route.
func (a *API) Middleware(m ...contractshttp.Middleware) *API {
	a.middleware = append(a.middleware, m...)
	return a
}

// Routes adds custom routes under the prefix. Protect them with API.Guard.
func (a *API) Routes(fn func(r route.Router)) *API { a.routes = append(a.routes, fn); return a }

// AuthTable sets the users table and its e-mail and password columns.
func (a *API) AuthTable(table, emailColumn, passwordColumn string) *API {
	a.authTable, a.emailColumn, a.passwordColumn = table, emailColumn, passwordColumn
	return a
}

// TokenTable sets the table that stores the tokens (default "api_tokens").
func (a *API) TokenTable(table string) *API { a.tokenTable = table; return a }

// CanAccess decides which users may use the API (log in and use their tokens).
func (a *API) CanAccess(fn func(user table.Row) bool) *API { a.canAccess = fn; return a }

// TokenLogin enables POST {prefix}/auth/token (e-mail and password => token),
// DELETE {prefix}/auth/token and GET {prefix}/auth/me. Tokens issued by login
// get the given abilities; with none they only reach Authenticated() endpoints.
func (a *API) TokenLogin(abilities ...string) *API {
	a.tokenLogin = true
	a.loginAbilities = abilities
	return a
}

// TokenExpiry sets how long tokens issued by login last (0 = never expire).
func (a *API) TokenExpiry(d time.Duration) *API { a.tokenTTL = d; return a }

// Services returns the services the API was registered with.
func (a *API) Services() Services { return a.svc }

// Register validates the API and mounts its routes.
func (a *API) Register(svc Services) error {
	a.svc = svc
	if err := a.validate(); err != nil {
		return err
	}
	a.limiter = newRateLimiter(a.rateLimit)

	router := svc.Route.Prefix(a.prefix).Middleware(append([]contractshttp.Middleware{&apiMiddleware{api: a}}, a.middleware...)...)
	router.Group(func(r route.Router) {
		if a.tokenLogin {
			r.Post("/auth/token", a.issueTokenHandler)
			r.Delete("/auth/token", a.Guard(api.Authenticated(), a.revokeTokenHandler))
			r.Get("/auth/me", a.Guard(api.Authenticated(), a.meHandler))
		}
		for _, res := range a.resources {
			h := &apiHandler{api: a, res: res}
			access := res.Access()
			slug := "/" + res.Slug()
			r.Get(slug, a.Guard(access.Index, h.index))
			r.Post(slug, a.Guard(access.Store, h.store))
			r.Get(slug+"/{id}", a.Guard(access.Show, h.show))
			r.Put(slug+"/{id}", a.Guard(access.Update, h.update))
			r.Patch(slug+"/{id}", a.Guard(access.Update, h.update))
			r.Delete(slug+"/{id}", a.Guard(access.Destroy, h.destroy))
		}
		for _, fn := range a.routes {
			fn(r)
		}
	})
	return nil
}

func (a *API) validate() error {
	if svcMissing := a.svc.Route == nil || a.svc.DB == nil || a.svc.Validation == nil || a.svc.Hash == nil; svcMissing {
		return fmt.Errorf("loom: the API needs Services.Route, DB, Validation and Hash")
	}
	for _, name := range []string{a.authTable, a.emailColumn, a.passwordColumn, a.nameColumn, a.tokenTable} {
		if !identifier.MatchString(name) {
			return fmt.Errorf("loom: invalid API table/column name %q", name)
		}
	}
	used := map[string]bool{}
	if a.tokenLogin {
		used["auth"] = true
	}
	for _, res := range a.resources {
		slug := res.Slug()
		if !slugRule.MatchString(slug) {
			return fmt.Errorf("loom: invalid API slug %q", slug)
		}
		if used[slug] {
			return fmt.Errorf("loom: duplicated API slug %q", slug)
		}
		used[slug] = true
		if !identifier.MatchString(res.TableName()) || !identifier.MatchString(res.PrimaryKey()) {
			return fmt.Errorf("loom: API resource %q: invalid table or primary key", slug)
		}
		seen := map[string]bool{}
		for _, f := range res.schema.Fields() {
			if !identifier.MatchString(f.Name()) {
				return fmt.Errorf("loom: API resource %q: invalid field name %q", slug, f.Name())
			}
			if seen[f.Name()] {
				return fmt.Errorf("loom: API resource %q: duplicated field %q", slug, f.Name())
			}
			seen[f.Name()] = true
		}
	}
	return nil
}

// Guard wraps a handler with an access rule (use it in API.Routes).
func (a *API) Guard(rule api.Rule, next contractshttp.HandlerFunc) contractshttp.HandlerFunc {
	return func(ctx contractshttp.Context) contractshttp.Response {
		if rule.IsPublic() {
			return next(ctx)
		}
		tok := currentToken(ctx)
		if tok == nil {
			msg := "Envie o token no header Authorization: Bearer SEU_TOKEN."
			if ctx.Value(ctxTokenInvalidKey) != nil {
				msg = "Token inválido ou expirado."
			}
			ctx.Response().Header("WWW-Authenticate", "Bearer")
			return apiError(ctx, 401, msg)
		}
		if ab := rule.Ability(); ab != "" && !tok.Can(ab) {
			return apiError(ctx, 403, "O token não tem a permissão "+ab+".")
		}
		return next(ctx)
	}
}

// apiMiddleware applies the rate limit and resolves the Bearer token, if any.
type apiMiddleware struct{ api *API }

func (m *apiMiddleware) Signature() string { return "loom-api" }

func (m *apiMiddleware) Handle(ctx contractshttp.Context) {
	a := m.api
	if plain, ok := bearerToken(ctx); ok {
		tok, user, err := a.findToken(ctx, plain)
		switch {
		case err != nil:
			logError(ctx, err)
			_ = ctx.Response().Json(500, map[string]any{"message": "Erro interno."}).Abort()
			return
		case tok == nil:
			ctx.WithValue(ctxTokenInvalidKey, true)
		default:
			ctx.WithValue(ctxTokenKey, tok)
			ctx.WithValue(ctxUserKey, user)
		}
	}

	key := "ip:" + ctx.Request().Ip()
	if tok := currentToken(ctx); tok != nil {
		key = "token:" + table.Stringify(tok.ID)
	}
	if allowed, remaining, retry := a.limiter.allow(key); !allowed {
		ctx.Response().Header("Retry-After", strconv.Itoa(retry))
		ctx.Response().Header("X-RateLimit-Limit", strconv.Itoa(a.rateLimit))
		ctx.Response().Header("X-RateLimit-Remaining", "0")
		_ = ctx.Response().Json(429, map[string]any{"message": "Muitas requisições. Tente novamente em instantes."}).Abort()
		return
	} else if a.rateLimit > 0 {
		ctx.Response().Header("X-RateLimit-Limit", strconv.Itoa(a.rateLimit))
		ctx.Response().Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
	}
	ctx.Request().Next()
}

// apiHandler serves the REST endpoints of one resource.
type apiHandler struct {
	api *API
	res *apiEntry
}

func (h *apiHandler) hookCtx(ctx contractshttp.Context) Context {
	return Context{HTTP: ctx, API: h.api}
}

// query returns the table query with the resource scope (ModifyQuery).
func (h *apiHandler) query(ctx contractshttp.Context) contractsdb.Query {
	q := h.api.svc.DB.WithContext(ctx).Table(h.res.TableName())
	if m, ok := h.res.APIResource.(ModifiesQuery); ok {
		q = m.ModifyQuery(h.hookCtx(ctx), q)
	}
	return q
}

// columns are the selected columns: the primary key and every visible field.
func (h *apiHandler) columns() []string {
	pk := h.res.PrimaryKey()
	cols := []string{pk}
	for _, f := range h.res.schema.Fields() {
		if !f.IsHidden() && f.Name() != pk {
			cols = append(cols, f.Name())
		}
	}
	return cols
}

func (h *apiHandler) index(ctx contractshttp.Context) contractshttp.Response {
	req := ctx.Request()
	res := h.res

	page := max(req.QueryInt("page", 1), 1)
	perPage := req.QueryInt("per_page", 15)
	if perPage < 1 || perPage > 100 {
		return apiError(ctx, 400, "per_page deve ficar entre 1 e 100.")
	}

	sortCol, sortDir := res.PrimaryKey(), "desc"
	if s := strings.TrimSpace(req.Query("sort")); s != "" {
		sortDir = "asc"
		if strings.HasPrefix(s, "-") {
			s, sortDir = s[1:], "desc"
		}
		f := res.schema.Field(s)
		if s != res.PrimaryKey() && (f == nil || !f.IsSortable() || f.IsHidden()) {
			return apiError(ctx, 400, "Não é possível ordenar por "+s+".")
		}
		sortCol = s
	}

	type filter struct {
		col    string
		values []any
	}
	var filters []filter
	for name, raw := range req.QueryMap("filter") {
		f := res.schema.Field(name)
		if f == nil || !f.IsFilterable() || f.IsHidden() {
			return apiError(ctx, 400, "Não é possível filtrar por "+name+".")
		}
		var values []any
		for _, part := range strings.Split(raw, ",") {
			v, msg := coerce(f, strings.TrimSpace(part), true)
			if msg != "" {
				return apiError(ctx, 400, "Filtro "+name+": "+msg)
			}
			values = append(values, v)
		}
		filters = append(filters, filter{name, values})
	}

	search := strings.TrimSpace(req.Query("search"))
	var searchable []string
	for _, f := range res.schema.Fields() {
		if f.IsSearchable() && !f.IsHidden() {
			searchable = append(searchable, f.Name())
		}
	}

	build := func() contractsdb.Query {
		q := h.query(ctx)
		if m, ok := res.APIResource.(ModifiesListQuery); ok {
			q = m.ModifyListQuery(h.hookCtx(ctx), q)
		}
		if search != "" && len(searchable) > 0 {
			term := "%" + strings.ToLower(escapeLike(search)) + "%"
			q = q.Where(func(sq contractsdb.Query) contractsdb.Query {
				for i, col := range searchable {
					raw := "LOWER(" + col + ") LIKE ?"
					if i == 0 {
						sq = sq.WhereRaw(raw, []any{term})
					} else {
						sq = sq.OrWhereRaw(raw, []any{term})
					}
				}
				return sq
			})
		}
		for _, f := range filters {
			if len(f.values) == 1 {
				q = q.Where(f.col, f.values[0])
			} else {
				q = q.WhereIn(f.col, f.values)
			}
		}
		return q
	}

	total, err := build().Count()
	if err != nil {
		return apiServerError(ctx, err)
	}
	rows, err := fetchRows(build().Select(h.columns()...).OrderBy(sortCol, sortDir).
		Offset(uint64((page - 1) * perPage)).Limit(uint64(perPage)))
	if err != nil {
		return apiServerError(ctx, err)
	}

	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, h.present(r))
	}
	return ctx.Response().Json(200, map[string]any{
		"data": data,
		"meta": map[string]any{
			"page":      page,
			"per_page":  perPage,
			"total":     total,
			"last_page": max(int(math.Ceil(float64(total)/float64(perPage))), 1),
		},
	})
}

func (h *apiHandler) show(ctx contractshttp.Context) contractshttp.Response {
	row, err := h.find(ctx, ctx.Request().Route("id"))
	if err != nil {
		return apiServerError(ctx, err)
	}
	if row == nil {
		return apiError(ctx, 404, "Registro não encontrado.")
	}
	return ctx.Response().Json(200, map[string]any{"data": h.present(row)})
}

func (h *apiHandler) find(ctx contractshttp.Context, id string) (table.Row, error) {
	rows, err := fetchRows(h.query(ctx).Select(h.columns()...).Where(h.res.PrimaryKey(), normalizeID(id)).Limit(1))
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (h *apiHandler) exists(ctx contractshttp.Context, id string) (bool, error) {
	return h.query(ctx).Where(h.res.PrimaryKey(), normalizeID(id)).Exists()
}

func (h *apiHandler) store(ctx contractshttp.Context) contractshttp.Response {
	return h.save(ctx, "")
}

func (h *apiHandler) update(ctx contractshttp.Context) contractshttp.Response {
	id := ctx.Request().Route("id")
	ok, err := h.exists(ctx, id)
	if err != nil {
		return apiServerError(ctx, err)
	}
	if !ok {
		return apiError(ctx, 404, "Registro não encontrado.")
	}
	return h.save(ctx, id)
}

// save validates the JSON body and inserts (id == "") or updates the record.
// Updates are partial: only the fields sent are changed (PUT and PATCH).
func (h *apiHandler) save(ctx contractshttp.Context, id string) contractshttp.Response {
	a, res := h.api, h.res
	isCreate := id == ""
	body, msg := readJSONBody(ctx)
	if msg != "" {
		return apiError(ctx, 400, msg)
	}

	data := map[string]any{}
	vdata := map[string]any{}
	rules := map[string]any{}
	errs := map[string][]string{}
	for _, f := range res.schema.Fields() {
		name := f.Name()
		if f.IsReadOnly() || name == res.PrimaryKey() {
			continue
		}
		raw, present := body[name]
		if !present {
			if isCreate && f.IsRequired() {
				errs[name] = append(errs[name], "O campo "+name+" é obrigatório.")
			}
			continue
		}
		v, m := coerce(f, raw, false)
		if m != "" {
			errs[name] = append(errs[name], "O campo "+name+" "+m)
			continue
		}
		if v == nil {
			switch {
			case f.IsRequired():
				errs[name] = append(errs[name], "O campo "+name+" é obrigatório.")
			case f.IsNullable():
				data[name] = nil
			case f.Kind() == api.KindString:
				data[name] = ""
			default:
				errs[name] = append(errs[name], "O campo "+name+" não aceita null.")
			}
			continue
		}
		data[name] = v
		vdata[name] = v
		if s, ok := v.(string); ok && f.Kind() == api.KindNumber {
			vdata[name], _ = strconv.ParseFloat(s, 64) // numeric rules (min, max) compare values
		}
		if r := f.ValidationRules(res.TableName(), id); r != "" {
			rules[name] = r
		}
	}

	if len(rules) > 0 {
		validator, err := a.svc.Validation.Make(ctx, vdata, rules, func(o *contractsvalidation.Options) {})
		if err != nil {
			return apiServerError(ctx, err)
		}
		if validator.Fails() {
			for name := range rules {
				if m := validator.Errors().One(name); m != "" {
					errs[name] = append(errs[name], m)
				}
			}
		}
	}
	if len(errs) > 0 {
		return ctx.Response().Json(422, map[string]any{"message": "Os dados enviados são inválidos.", "errors": errs})
	}

	for _, f := range res.schema.Fields() {
		if v, ok := data[f.Name()].(string); ok && f.Kind() == api.KindPassword {
			hashed, err := a.svc.Hash.Make(v)
			if err != nil {
				return apiServerError(ctx, err)
			}
			data[f.Name()] = hashed
		}
	}

	hc := h.hookCtx(ctx)
	var hookErr error
	if hook, ok := res.APIResource.(BeforeCreateHook); ok && isCreate {
		hookErr = hook.BeforeCreate(hc, data)
	} else if hook, ok := res.APIResource.(BeforeSaveHook); ok && !isCreate {
		hookErr = hook.BeforeSave(hc, id, data)
	}
	if hookErr != nil {
		if m, halted := haltMessage(hookErr); halted {
			return apiError(ctx, 422, m)
		}
		return apiServerError(ctx, hookErr)
	}

	now := time.Now()
	if res.Timestamps() {
		data["updated_at"] = now
	}

	if isCreate {
		if res.Timestamps() {
			data["created_at"] = now
		}
		newID, err := a.insertGetID(ctx, res.TableName(), res.PrimaryKey(), data)
		if err != nil {
			return apiServerError(ctx, err)
		}
		if hook, ok := res.APIResource.(AfterCreateHook); ok {
			if err := hook.AfterCreate(hc, data); err != nil {
				return apiServerError(ctx, err)
			}
		}
		if newID == nil {
			return ctx.Response().Json(201, map[string]any{"data": h.present(data)})
		}
		id = table.Stringify(newID)
	} else {
		if len(data) > 0 {
			if _, err := h.query(ctx).Where(res.PrimaryKey(), normalizeID(id)).Update(data); err != nil {
				return apiServerError(ctx, err)
			}
		}
		if hook, ok := res.APIResource.(AfterSaveHook); ok {
			if err := hook.AfterSave(hc, id, data); err != nil {
				return apiServerError(ctx, err)
			}
		}
	}

	row, err := h.find(ctx, id)
	if err != nil {
		return apiServerError(ctx, err)
	}
	status := 200
	if isCreate {
		status = 201
	}
	if row == nil {
		return ctx.Response().Json(status, map[string]any{"data": h.present(data)})
	}
	return ctx.Response().Json(status, map[string]any{"data": h.present(row)})
}

func (h *apiHandler) destroy(ctx contractshttp.Context) contractshttp.Response {
	id := ctx.Request().Route("id")
	ok, err := h.exists(ctx, id)
	if err != nil {
		return apiServerError(ctx, err)
	}
	if !ok {
		return apiError(ctx, 404, "Registro não encontrado.")
	}
	if hook, ok := h.res.APIResource.(BeforeDeleteHook); ok {
		if err := hook.BeforeDelete(h.hookCtx(ctx), []any{normalizeID(id)}); err != nil {
			if m, halted := haltMessage(err); halted {
				return apiError(ctx, 422, m)
			}
			return apiServerError(ctx, err)
		}
	}
	if _, err := h.query(ctx).Where(h.res.PrimaryKey(), normalizeID(id)).Delete(); err != nil {
		return apiServerError(ctx, err)
	}
	return ctx.Response().NoContent(204)
}

// present converts a row to its JSON shape: visible fields with their types.
func (h *apiHandler) present(row map[string]any) map[string]any {
	pk := h.res.PrimaryKey()
	out := map[string]any{}
	if v, ok := row[pk]; ok {
		out[pk] = normalizeID(table.Stringify(v))
	}
	for _, f := range h.res.schema.Fields() {
		if f.IsHidden() || f.Name() == pk {
			continue
		}
		if _, ok := row[f.Name()]; !ok {
			continue
		}
		out[f.Name()] = outputValue(f, row)
	}
	return out
}

func outputValue(f *api.FieldDef, row map[string]any) any {
	v := row[f.Name()]
	if v == nil {
		return nil
	}
	switch f.Kind() {
	case api.KindInteger:
		if n, err := strconv.ParseInt(table.Stringify(v), 10, 64); err == nil {
			return n
		}
	case api.KindNumber:
		if n, err := strconv.ParseFloat(table.Stringify(v), 64); err == nil {
			return n
		}
	case api.KindBoolean:
		return table.Boolean(f.Name()).Truthy(row)
	case api.KindDate:
		if t, ok := v.(time.Time); ok {
			return t.Format("2006-01-02")
		}
		if s := table.Stringify(v); len(s) >= 10 {
			return s[:10]
		}
	case api.KindDateTime:
		if t, ok := v.(time.Time); ok {
			return t.UTC().Format(time.RFC3339)
		}
	}
	switch t := v.(type) {
	case []byte:
		return string(t)
	case time.Time:
		return t.UTC().Format(time.RFC3339)
	}
	return v
}

// coerce converts an input value (JSON or query string) to the field type.
// It returns nil for null/"" and a message when the value has the wrong type.
func coerce(f *api.FieldDef, raw any, fromQuery bool) (any, string) {
	if raw == nil {
		return nil, ""
	}
	if s, ok := raw.(string); ok && s == "" {
		return nil, ""
	}
	str, isString := raw.(string)
	num, isNumber := raw.(json.Number)
	switch f.Kind() {
	case api.KindInteger:
		if isNumber {
			if n, err := num.Int64(); err == nil {
				return n, ""
			}
		} else if isString {
			if n, err := strconv.ParseInt(str, 10, 64); err == nil {
				return n, ""
			}
		}
		return nil, "deve ser um número inteiro."
	case api.KindNumber:
		if isNumber {
			if _, err := num.Float64(); err == nil {
				return num.String(), ""
			}
		} else if isString {
			if _, err := strconv.ParseFloat(str, 64); err == nil {
				return str, ""
			}
		}
		return nil, "deve ser um número."
	case api.KindBoolean:
		switch v := raw.(type) {
		case bool:
			return v, ""
		case json.Number:
			if v.String() == "0" || v.String() == "1" {
				return v.String() == "1", ""
			}
		case string:
			switch strings.ToLower(v) {
			case "true", "1":
				return true, ""
			case "false", "0":
				return false, ""
			}
		}
		return nil, "deve ser true ou false."
	case api.KindDate:
		if isString {
			if _, err := time.Parse("2006-01-02", str); err == nil {
				return str, ""
			}
		}
		return nil, "deve ser uma data no formato AAAA-MM-DD."
	case api.KindDateTime:
		if isString {
			if t, err := time.Parse(time.RFC3339, str); err == nil {
				return t, ""
			}
			if t, err := time.Parse("2006-01-02", str); err == nil {
				return t, ""
			}
		}
		return nil, "deve ser uma data e hora no formato RFC 3339 (2026-01-31T14:00:00Z)."
	default: // string, password
		if isString {
			return str, ""
		}
		if fromQuery {
			return fmt.Sprint(raw), ""
		}
		return nil, "deve ser um texto."
	}
}

func readJSONBody(ctx contractshttp.Context) (map[string]any, string) {
	origin := ctx.Request().Origin()
	if origin.Body == nil {
		return map[string]any{}, ""
	}
	raw, err := io.ReadAll(io.LimitReader(origin.Body, 10<<20))
	if err != nil {
		return nil, "Não foi possível ler o corpo da requisição."
	}
	origin.Body = io.NopCloser(bytes.NewReader(raw))
	if len(bytes.TrimSpace(raw)) == 0 {
		// Form posts (urlencoded/multipart) were already parsed by the HTTP driver.
		body := map[string]any{}
		for k, v := range origin.PostForm {
			if len(v) > 0 {
				body[k] = v[0]
			}
		}
		return body, ""
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var body map[string]any
	if err := dec.Decode(&body); err != nil {
		return nil, "Envie um objeto JSON válido (Content-Type: application/json)."
	}
	return body, ""
}

// insertGetID inserts the row and returns its primary key when it can be known.
func (a *API) insertGetID(ctx contractshttp.Context, tableName, pk string, data map[string]any) (any, error) {
	db := a.svc.DB.WithContext(ctx)
	if v, ok := data[pk]; ok && v != nil {
		_, err := db.Table(tableName).Insert(data)
		return v, err
	}
	if a.postgres(ctx) {
		// Reserve the next value of the serial/identity sequence, then insert with it.
		var seq []struct {
			Seq *string `db:"seq"`
		}
		if err := db.Select(&seq, "SELECT pg_get_serial_sequence($1, $2) AS seq", tableName, pk); err != nil {
			return nil, err
		}
		if len(seq) == 0 || seq[0].Seq == nil {
			_, err := db.Table(tableName).Insert(data)
			return nil, err
		}
		var next []struct {
			ID int64 `db:"id"`
		}
		if err := db.Select(&next, "SELECT nextval($1::regclass) AS id", *seq[0].Seq); err != nil {
			return nil, err
		}
		data[pk] = next[0].ID
		_, err := db.Table(tableName).Insert(data)
		return next[0].ID, err
	}
	return db.Table(tableName).InsertGetID(data)
}

func (a *API) postgres(ctx contractshttp.Context) bool {
	a.pgOnce.Do(func() {
		var v []struct {
			V string `db:"v"`
		}
		a.isPostgres = a.svc.DB.WithContext(ctx).Select(&v, "SELECT current_setting('server_version_num') AS v") == nil
	})
	return a.isPostgres
}

func apiError(ctx contractshttp.Context, status int, message string) contractshttp.Response {
	return ctx.Response().Json(status, map[string]any{"message": message})
}

func logError(ctx contractshttp.Context, err error) {
	log.Printf("loom: %s %s: %v", ctx.Request().Method(), ctx.Request().Path(), err)
}

func apiServerError(ctx contractshttp.Context, err error) contractshttp.Response {
	logError(ctx, err)
	return apiError(ctx, 500, "Erro interno.")
}

// rateLimiter is a fixed one-minute window per client key.
type rateLimiter struct {
	limit int
	mu    sync.Mutex
	hits  map[string]*window
}

type window struct {
	start time.Time
	count int
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{limit: limit, hits: map[string]*window{}}
}

// allow reports whether the request may go on, the remaining requests and,
// when blocked, the seconds until the window resets.
func (l *rateLimiter) allow(key string) (bool, int, int) {
	if l.limit <= 0 {
		return true, 0, 0
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hits) > 10000 {
		for k, w := range l.hits {
			if now.Sub(w.start) >= time.Minute {
				delete(l.hits, k)
			}
		}
	}
	w := l.hits[key]
	if w == nil || now.Sub(w.start) >= time.Minute {
		w = &window{start: now}
		l.hits[key] = w
	}
	if w.count >= l.limit {
		return false, 0, int(time.Minute.Seconds()-now.Sub(w.start).Seconds()) + 1
	}
	w.count++
	return true, l.limit - w.count, 0
}
