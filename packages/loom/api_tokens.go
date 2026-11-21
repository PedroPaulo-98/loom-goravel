package loom

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
	contractshttp "github.com/goravel/framework/contracts/http"

	"goravel/packages/loom/form"
	"goravel/packages/loom/table"
)

const (
	ctxTokenKey        = "loom_api_token"
	ctxTokenInvalidKey = "loom_api_token_invalid"
	ctxNotifiedKey     = "loom_notified"
)

// Token is an API token. Only its SHA-256 hash is stored.
type Token struct {
	ID        any
	UserID    any
	Name      string
	Abilities []string
	ExpiresAt *time.Time
}

// Can reports whether the token has the ability. "*" grants everything and
// "cars:*" grants every "cars:..." ability.
func (t *Token) Can(ability string) bool {
	if t == nil {
		return false
	}
	for _, a := range t.Abilities {
		if a == "*" || a == ability || (strings.HasSuffix(a, ":*") && strings.HasPrefix(ability, strings.TrimSuffix(a, "*"))) {
			return true
		}
	}
	return false
}

func currentToken(ctx contractshttp.Context) *Token {
	if t, ok := ctx.Value(ctxTokenKey).(*Token); ok {
		return t
	}
	return nil
}

func bearerToken(ctx contractshttp.Context) (string, bool) {
	h := strings.TrimSpace(ctx.Request().Header("Authorization"))
	if len(h) < 7 || !strings.EqualFold(h[:7], "bearer ") {
		return "", false
	}
	t := strings.TrimSpace(h[7:])
	return t, t != ""
}

func newPlainToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "lm_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// ParseAbilities turns "cars:read, cars:write" into ["cars:read", "cars:write"].
func ParseAbilities(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }) {
		if !seen[part] {
			seen[part] = true
			out = append(out, part)
		}
	}
	return out
}

// CreateToken stores a new token for the user and returns its plain text,
// which cannot be recovered later. expiresAt nil means it never expires.
func (a *API) CreateToken(ctx context.Context, userID any, name string, abilities []string, expiresAt *time.Time) (string, error) {
	plain, err := newPlainToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	data := map[string]any{
		"user_id":    userID,
		"name":       name,
		"token":      hashToken(plain),
		"abilities":  strings.Join(abilities, ","),
		"expires_at": nil,
		"created_at": now,
		"updated_at": now,
	}
	if expiresAt != nil {
		data["expires_at"] = *expiresAt
	}
	if _, err := a.svc.DB.WithContext(ctx).Table(a.tokenTable).Insert(data); err != nil {
		return "", err
	}
	return plain, nil
}

// findToken returns the valid token and its user, or nils when the token is
// unknown, expired or its user can no longer use the API.
func (a *API) findToken(ctx contractshttp.Context, plain string) (*Token, table.Row, error) {
	rows, err := fetchRows(a.svc.DB.WithContext(ctx).Table(a.tokenTable).Where("token", hashToken(plain)).Limit(1))
	if err != nil || len(rows) == 0 {
		return nil, nil, err
	}
	row := rows[0]
	tok := &Token{
		ID:        row["id"],
		UserID:    row["user_id"],
		Name:      table.Stringify(row["name"]),
		Abilities: ParseAbilities(table.Stringify(row["abilities"])),
	}
	if t, ok := row["expires_at"].(time.Time); ok {
		if time.Now().After(t) {
			return nil, nil, nil
		}
		tok.ExpiresAt = &t
	}

	users, err := fetchRows(a.svc.DB.WithContext(ctx).Table(a.authTable).Where("id", row["user_id"]).Limit(1))
	if err != nil || len(users) == 0 {
		return nil, nil, err
	}
	user := users[0]
	if a.canAccess != nil && !a.canAccess(user) {
		return nil, nil, nil
	}

	if last, ok := row["last_used_at"].(time.Time); !ok || time.Since(last) > time.Minute {
		_, _ = a.svc.DB.WithContext(ctx).Table(a.tokenTable).Where("id", row["id"]).Update(map[string]any{"last_used_at": time.Now()})
	}
	return tok, user, nil
}

// issueTokenHandler: POST {prefix}/auth/token {"email", "password", "name"}.
func (a *API) issueTokenHandler(ctx contractshttp.Context) contractshttp.Response {
	body, msg := readJSONBody(ctx)
	if msg != "" {
		return apiError(ctx, 400, msg)
	}
	email, _ := body["email"].(string)
	password, _ := body["password"].(string)
	name, _ := body["name"].(string)
	if strings.TrimSpace(email) == "" || password == "" {
		return ctx.Response().Json(422, map[string]any{
			"message": "Informe e-mail e senha.",
			"errors":  map[string][]string{"email": {"Obrigatório."}, "password": {"Obrigatório."}},
		})
	}
	if name = strings.TrimSpace(name); name == "" {
		name = "api"
	}

	users, err := fetchRows(a.svc.DB.WithContext(ctx).Table(a.authTable).Where(a.emailColumn, strings.TrimSpace(email)).Limit(1))
	if err != nil {
		return apiServerError(ctx, err)
	}
	if len(users) == 0 || !a.svc.Hash.Check(password, table.Stringify(users[0][a.passwordColumn])) ||
		(a.canAccess != nil && !a.canAccess(users[0])) {
		return apiError(ctx, 401, "Credenciais inválidas ou sem acesso à API.")
	}

	var expires *time.Time
	if a.tokenTTL > 0 {
		t := time.Now().Add(a.tokenTTL)
		expires = &t
	}
	plain, err := a.CreateToken(ctx, users[0]["id"], name, a.loginAbilities, expires)
	if err != nil {
		return apiServerError(ctx, err)
	}
	resp := map[string]any{
		"token":      plain,
		"token_type": "Bearer",
		"abilities":  nonNil(a.loginAbilities),
		"expires_at": nil,
	}
	if expires != nil {
		resp["expires_at"] = expires.UTC().Format(time.RFC3339)
	}
	return ctx.Response().Json(201, resp)
}

// revokeTokenHandler: DELETE {prefix}/auth/token revokes the token in use.
func (a *API) revokeTokenHandler(ctx contractshttp.Context) contractshttp.Response {
	if _, err := a.svc.DB.WithContext(ctx).Table(a.tokenTable).Where("id", currentToken(ctx).ID).Delete(); err != nil {
		return apiServerError(ctx, err)
	}
	return ctx.Response().NoContent(204)
}

// meHandler: GET {prefix}/auth/me returns the token owner and the token.
func (a *API) meHandler(ctx contractshttp.Context) contractshttp.Response {
	user, tok := currentUser(ctx), currentToken(ctx)
	expires := any(nil)
	if tok.ExpiresAt != nil {
		expires = tok.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return ctx.Response().Json(200, map[string]any{
		"data": map[string]any{
			"id":    normalizeID(table.Stringify(user["id"])),
			"name":  table.Stringify(user[a.nameColumn]),
			"email": table.Stringify(user[a.emailColumn]),
		},
		"token": map[string]any{
			"name":       tok.Name,
			"abilities":  nonNil(tok.Abilities),
			"expires_at": expires,
		},
	})
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// APITokenResource is a panel resource to create and revoke API tokens.
// The token is shown once, right after it is created.
//
//	Resources(&user.Resource{}, &loom.APITokenResource{})
type APITokenResource struct {
	BaseResource
	// TokenTable defaults to "api_tokens".
	TokenTable string
	// Group is the navigation group (optional).
	Group string
}

func (r APITokenResource) Slug() string          { return "api-tokens" }
func (r APITokenResource) Label() string         { return "Tokens de API" }
func (r APITokenResource) SingularLabel() string { return "Token de API" }
func (r APITokenResource) Icon() string          { return "key" }
func (r APITokenResource) NavigationGroup() string {
	return r.Group
}

func (r APITokenResource) TableName() string {
	if r.TokenTable == "" {
		return "api_tokens"
	}
	return r.TokenTable
}

func (r APITokenResource) Form(f *form.Form) {
	f.Schema(
		form.Text("name").Label("Nome").Required().Rules("max:255").
			Placeholder("App mobile, integração ERP..."),
		form.Text("abilities").Label("Permissões").Nullable().
			Placeholder("cars:read, cars:write").
			Help("Separe por vírgula. * libera tudo e cars:* libera todas as de cars. Vazio: só rotas que pedem apenas login."),
		form.Date("expires_at").Label("Expira em").Help("Vazio: não expira."),
	)
}

func (r APITokenResource) Table(t *table.Table) {
	t.Columns(
		table.Text("name").Label("Nome").Sortable().Searchable(),
		table.Text("abilities").Label("Permissões").FormatUsing(func(v any, _ table.Row) string {
			if s := table.Stringify(v); s != "" {
				return strings.ReplaceAll(s, ",", ", ")
			}
			return "—"
		}),
		table.DateTime("last_used_at").Label("Último uso").Sortable(),
		table.DateTime("expires_at").Label("Expira em").Sortable(),
		table.DateTime("created_at").Label("Criado em").Sortable(),
	).DefaultSort("created_at", "desc")
}

// BeforeCreate generates the token, links it to the logged-in user and shows it once.
func (r APITokenResource) BeforeCreate(ctx Context, data map[string]any) error {
	plain, err := newPlainToken()
	if err != nil {
		return err
	}
	data["token"] = hashToken(plain)
	data["user_id"] = ctx.User()["id"]
	data["abilities"] = normalizeAbilities(data["abilities"])
	ctx.Notify("Token criado. Copie agora, ele não será mostrado de novo: " + plain)
	return nil
}

func (r APITokenResource) BeforeSave(ctx Context, id string, data map[string]any) error {
	data["abilities"] = normalizeAbilities(data["abilities"])
	return nil
}

func normalizeAbilities(v any) string {
	return strings.Join(ParseAbilities(table.Stringify(v)), ",")
}

// MakeTokenCommand creates an API token from the terminal:
//
//	./artisan loom:make-token --email=admin@example.com --abilities="cars:read,cars:write" --days=30
type MakeTokenCommand struct{ api *API }

// NewMakeTokenCommand builds the command for the API (registered or not yet).
func NewMakeTokenCommand(a *API) *MakeTokenCommand { return &MakeTokenCommand{api: a} }

func (c *MakeTokenCommand) Signature() string   { return "loom:make-token" }
func (c *MakeTokenCommand) Description() string { return "Create an API token for a user" }

func (c *MakeTokenCommand) Extend() command.Extend {
	return command.Extend{
		Category: "loom",
		Flags: []command.Flag{
			&command.StringFlag{Name: "email", Usage: "Token owner e-mail"},
			&command.StringFlag{Name: "name", Usage: "Token name", Value: "cli"},
			&command.StringFlag{Name: "abilities", Usage: `Abilities, e.g. "cars:read,cars:write" or "*"`},
			&command.StringFlag{Name: "days", Usage: "Expire after N days (empty = never)"},
		},
	}
}

func (c *MakeTokenCommand) Handle(ctx console.Context) error {
	a := c.api
	if a.svc.DB == nil {
		return errors.New("loom: the API is not registered")
	}
	email := ctx.Option("email")
	if email == "" {
		var err error
		if email, err = ctx.Ask("Owner e-mail", console.AskOption{}); err != nil {
			return err
		}
	}
	users, err := fetchRows(a.svc.DB.Table(a.authTable).Where(a.emailColumn, strings.TrimSpace(email)).Limit(1))
	if err != nil {
		return err
	}
	if len(users) == 0 {
		ctx.Error("No user with e-mail " + email + ".")
		return nil
	}

	var expires *time.Time
	if d := ctx.Option("days"); d != "" {
		n, err := strconv.Atoi(d)
		if err != nil || n < 1 {
			ctx.Error("--days must be a positive number.")
			return nil
		}
		t := time.Now().AddDate(0, 0, n)
		expires = &t
	}
	abilities := ParseAbilities(ctx.Option("abilities"))
	plain, err := a.CreateToken(context.Background(), users[0]["id"], ctx.Option("name"), abilities, expires)
	if err != nil {
		return err
	}
	ctx.Success("Token created. Copy it now, it will not be shown again:")
	ctx.Line(plain)
	return nil
}
