package loom

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/mail"
	"net/url"
	"strings"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"
	contractsmail "github.com/goravel/framework/contracts/mail"
	contractsvalidation "github.com/goravel/framework/contracts/validation"
	"github.com/goravel/framework/session"

	"goravel/packages/loom/table"
)

const ctxUserKey = "loom_user"

func (p *Panel) sessionUserKey() string { return "loom_" + p.id + "_user_id" }

// csrfMiddleware rejects state-changing requests without the session token.
// Forms send it as "_token"; htmx sends it as the X-CSRF-Token header.
type csrfMiddleware struct{}

func (*csrfMiddleware) Signature() string { return "loom:csrf" }

func (*csrfMiddleware) Handle(ctx contractshttp.Context) {
	req := ctx.Request()
	if req.Method() == "GET" || req.Method() == "HEAD" || req.Method() == "OPTIONS" {
		req.Next()
		return
	}
	if !req.HasSession() {
		req.AbortWithStatus(419)
		return
	}
	expected := req.Session().Token()
	got := req.Header("X-CSRF-Token")
	if got == "" {
		got = req.Input("_token")
	}
	if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(got)) != 1 {
		req.AbortWithStatus(419)
		return
	}
	req.Next()
}

// authMiddleware requires a logged-in user allowed by Panel.CanAccess.
type authMiddleware struct{ panel *Panel }

func (*authMiddleware) Signature() string { return "loom:auth" }

func (m *authMiddleware) Handle(ctx contractshttp.Context) {
	p := m.panel
	sess := ctx.Request().Session()
	id := sess.Get(p.sessionUserKey())

	var user table.Row
	if id != nil {
		u, err := p.findRow(ctx, p.authTable, "id", normalizeID(table.Stringify(id)))
		if err == nil && u != nil && p.canAccess(u) {
			user = u
		}
	}
	if user == nil {
		sess.Forget(p.sessionUserKey())
		_ = redirect(ctx, p.URL("login")).Abort()
		return
	}

	ctx.WithValue(ctxUserKey, user)
	ctx.Request().Next()
}

func currentUser(ctx contractshttp.Context) table.Row {
	if u, ok := ctx.Value(ctxUserKey).(table.Row); ok {
		return u
	}
	return nil
}

// loginAs starts an authenticated session for the user.
func (p *Panel) loginAs(ctx contractshttp.Context, user table.Row) error {
	sess := ctx.Request().Session()
	// New session id after login prevents session fixation.
	if err := sess.Regenerate(); err != nil {
		return err
	}
	// StartSession already wrote the cookie with the old id; send the new one.
	session.WriteCookie(ctx, sess)
	sess.Put(p.sessionUserKey(), table.Stringify(user["id"]))
	return nil
}

func (p *Panel) authPage(ctx contractshttp.Context, name, title string, status int, data map[string]any) contractshttp.Response {
	if data == nil {
		data = map[string]any{}
	}
	data["Title"] = title
	data["Errors"] = orEmpty(data["Errors"])
	data["Old"] = orEmpty(data["Old"])
	return p.page(ctx, name, status, data)
}

func orEmpty(v any) map[string]string {
	if m, ok := v.(map[string]string); ok {
		return m
	}
	return map[string]string{}
}

func errorStatus(ctx contractshttp.Context) int {
	if isHtmx(ctx) {
		return 200 // htmx does not swap error responses by default
	}
	return 422
}

func (p *Panel) showLogin(ctx contractshttp.Context) contractshttp.Response {
	return p.authPage(ctx, "login", "Entrar", 200, nil)
}

func (p *Panel) login(ctx contractshttp.Context) contractshttp.Response {
	email := strings.TrimSpace(ctx.Request().Input("email"))
	password := ctx.Request().Input("password")

	user, err := p.findRow(ctx, p.authTable, p.emailColumn, email)
	if err != nil {
		return p.serverError(ctx, err)
	}

	ok := user != nil &&
		p.svc.Hash.Check(password, table.Stringify(user[p.passwordColumn])) &&
		p.canAccess(user)
	if !ok {
		return p.authPage(ctx, "login", "Entrar", errorStatus(ctx), map[string]any{
			"Old":   map[string]string{"email": email},
			"Error": "Credenciais inválidas ou sem acesso ao painel.",
		})
	}

	if err := p.loginAs(ctx, user); err != nil {
		return p.serverError(ctx, err)
	}
	return redirect(ctx, p.URL())
}

func (p *Panel) logout(ctx contractshttp.Context) contractshttp.Response {
	sess := ctx.Request().Session()
	sess.Forget(p.sessionUserKey())
	_ = sess.Invalidate()
	session.WriteCookie(ctx, sess)
	return redirect(ctx, p.URL("login"))
}

func (p *Panel) showRegister(ctx contractshttp.Context) contractshttp.Response {
	return p.authPage(ctx, "register", "Criar conta", 200, nil)
}

func (p *Panel) register(ctx contractshttp.Context) contractshttp.Response {
	req := ctx.Request()
	old := map[string]string{
		"name":  strings.TrimSpace(req.Input("name")),
		"email": strings.TrimSpace(req.Input("email")),
	}
	data := map[string]any{
		"name":                  old["name"],
		"email":                 old["email"],
		"password":              req.Input("password"),
		"password_confirmation": req.Input("password_confirmation"),
	}
	rules := map[string]any{
		"name":     "required|max:255",
		"email":    "required|email|max:255|unique:" + p.authTable + "," + p.emailColumn,
		"password": "required|confirmed|" + p.passwordRules,
	}
	if errs, err := p.validateInput(ctx, data, rules, map[string]string{
		"name": "Nome", "email": "E-mail", "password": "Senha",
	}); err != nil {
		return p.serverError(ctx, err)
	} else if len(errs) > 0 {
		return p.authPage(ctx, "register", "Criar conta", errorStatus(ctx), map[string]any{"Errors": errs, "Old": old})
	}

	hashed, err := p.svc.Hash.Make(req.Input("password"))
	if err != nil {
		return p.serverError(ctx, err)
	}
	now := time.Now()
	row := map[string]any{}
	for k, v := range p.newUserDefaults {
		row[k] = v
	}
	row[p.nameColumn] = old["name"]
	row[p.emailColumn] = old["email"]
	row[p.passwordColumn] = hashed
	row["created_at"] = now
	row["updated_at"] = now

	if _, err := p.svc.DB.WithContext(ctx).Table(p.authTable).Insert(row); err != nil {
		// Most likely a concurrent registration with the same e-mail (unique index).
		log.Printf("loom: register: %v", err)
		return p.authPage(ctx, "register", "Criar conta", errorStatus(ctx), map[string]any{
			"Errors": map[string]string{"email": "Não foi possível criar a conta com este e-mail."},
			"Old":    old,
		})
	}

	user, err := p.findRow(ctx, p.authTable, p.emailColumn, old["email"])
	if err != nil || user == nil {
		return p.serverError(ctx, err)
	}
	if !p.canAccess(user) {
		flash(ctx, "success", "Conta criada! Um administrador precisa liberar o seu acesso ao painel.")
		return redirect(ctx, p.URL("login"))
	}
	if err := p.loginAs(ctx, user); err != nil {
		return p.serverError(ctx, err)
	}
	flash(ctx, "success", "Bem-vindo! Sua conta foi criada.")
	return redirect(ctx, p.URL())
}

func (p *Panel) showForgotPassword(ctx contractshttp.Context) contractshttp.Response {
	return p.authPage(ctx, "forgot-password", "Recuperar senha", 200, nil)
}

// sendResetLink always answers with the same message, so the page cannot be
// used to discover which e-mails have an account.
func (p *Panel) sendResetLink(ctx contractshttp.Context) contractshttp.Response {
	email := strings.TrimSpace(ctx.Request().Input("email"))
	if _, err := mail.ParseAddress(email); err != nil || email == "" {
		return p.authPage(ctx, "forgot-password", "Recuperar senha", errorStatus(ctx), map[string]any{
			"Errors": map[string]string{"email": "Informe um e-mail válido."},
			"Old":    map[string]string{"email": email},
		})
	}

	if err := p.issueResetToken(ctx, email); err != nil {
		log.Printf("loom: password reset for %s: %v", email, err)
	}

	flash(ctx, "success", "Se existir uma conta com este e-mail, enviamos um link para redefinir a senha.")
	return redirect(ctx, p.URL("login"))
}

func (p *Panel) issueResetToken(ctx contractshttp.Context, email string) error {
	db := p.svc.DB.WithContext(ctx)
	user, err := p.findRow(ctx, p.authTable, p.emailColumn, email)
	if err != nil || user == nil {
		return err
	}
	if !p.canAccess(user) && !p.registration {
		return nil // no reason to reset a password that cannot be used here
	}

	// Throttle: at most one e-mail per minute per address.
	recent, err := db.Table(p.resetTable).
		Where("email", email).
		Where("created_at > ?", time.Now().Add(-time.Minute)).
		Exists()
	if err != nil {
		return err
	}
	if recent {
		return nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := hex.EncodeToString(raw)

	if _, err := db.Table(p.resetTable).Where("email", email).Delete(); err != nil {
		return err
	}
	if _, err := db.Table(p.resetTable).Insert(map[string]any{
		"email":      email,
		"token":      hashToken(token), // only the hash is stored
		"created_at": time.Now(),
	}); err != nil {
		return err
	}

	link := p.baseURL + p.URL("reset-password", token) + "?email=" + url.QueryEscape(email)
	html, err := p.render.fragment("email-reset", map[string]any{
		"Brand":   p.brand,
		"Name":    table.Stringify(user[p.nameColumn]),
		"Link":    link,
		"Minutes": int(p.resetExpiry.Minutes()),
		"Color":   p.colors["primary"]["600"],
	})
	if err != nil {
		return err
	}
	return p.svc.Mail.
		To([]string{email}).
		Subject("Redefinição de senha - " + p.brand).
		Content(contractsmail.Content{Html: string(html)}).
		Send()
}

func (p *Panel) showResetPassword(ctx contractshttp.Context) contractshttp.Response {
	return p.authPage(ctx, "reset-password", "Nova senha", 200, map[string]any{
		"Token": ctx.Request().Route("token"),
		"Old":   map[string]string{"email": ctx.Request().Query("email")},
	})
}

func (p *Panel) resetPassword(ctx contractshttp.Context) contractshttp.Response {
	req := ctx.Request()
	email := strings.TrimSpace(req.Input("email"))
	token := req.Input("token")
	fail := func(errs map[string]string) contractshttp.Response {
		return p.authPage(ctx, "reset-password", "Nova senha", errorStatus(ctx), map[string]any{
			"Token": token, "Errors": errs, "Old": map[string]string{"email": email},
		})
	}

	errs, err := p.validateInput(ctx, map[string]any{
		"password":              req.Input("password"),
		"password_confirmation": req.Input("password_confirmation"),
	}, map[string]any{"password": "required|confirmed|" + p.passwordRules}, map[string]string{"password": "Senha"})
	if err != nil {
		return p.serverError(ctx, err)
	}
	if len(errs) > 0 {
		return fail(errs)
	}

	db := p.svc.DB.WithContext(ctx)
	rows, err := fetchRows(db.Table(p.resetTable).Where("email", email).Limit(1))
	if err != nil {
		return p.serverError(ctx, err)
	}
	invalid := map[string]string{"email": "Este link é inválido ou expirou. Peça um novo."}
	if len(rows) == 0 {
		return fail(invalid)
	}
	row := rows[0]
	created, _ := row["created_at"].(time.Time)
	if subtle.ConstantTimeCompare([]byte(table.Stringify(row["token"])), []byte(hashToken(token))) != 1 ||
		time.Since(created) > p.resetExpiry {
		return fail(invalid)
	}

	hashed, err := p.svc.Hash.Make(req.Input("password"))
	if err != nil {
		return p.serverError(ctx, err)
	}
	if _, err := db.Table(p.authTable).Where(p.emailColumn, email).Update(map[string]any{
		p.passwordColumn: hashed,
		"updated_at":     time.Now(),
	}); err != nil {
		return p.serverError(ctx, err)
	}
	if _, err := db.Table(p.resetTable).Where("email", email).Delete(); err != nil {
		return p.serverError(ctx, err)
	}

	flash(ctx, "success", "Senha redefinida! Entre com a nova senha.")
	return redirect(ctx, p.URL("login"))
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// validateInput runs Goravel validation and returns field => first message.
func (p *Panel) validateInput(ctx contractshttp.Context, data map[string]any, rules map[string]any, labels map[string]string) (map[string]string, error) {
	validator, err := p.svc.Validation.Make(ctx, data, rules, func(o *contractsvalidation.Options) {
		o.Attributes = labels
	})
	if err != nil {
		return nil, err
	}
	errs := map[string]string{}
	if validator.Fails() {
		for field := range rules {
			if msg := validator.Errors().One(field); msg != "" {
				errs[field] = msg
			}
		}
	}
	return errs, nil
}
