package controllers

import (
	"errors"
	"net/http"
	"strings"

	"fullstack"
	auth "fullstack-passport"
	"fullstack-validation"
	"fullstack/web"
	"luna-keys"

	"luna-dashboard/app/apps"
	"luna-dashboard/app/models"
)

// Apps serves the developer's applications and their keys.
type Apps struct{ App *fullstack.App }

type appForm struct {
	Name             string `form:"name" validate:"required,max=100"`
	PasswordResetURL string `form:"password_reset_url" validate:"omitempty,url,max=2048"`
}

type keyForm struct {
	Name    string   `form:"name" validate:"max=100"`
	Kind    string   `form:"kind" validate:"required,oneof=server browser"`
	Scopes  []string `form:"scopes"`
	Origins string   `form:"origins" validate:"max=4000"`
	IPs     string   `form:"ips" validate:"max=4000"`
}

func (h Apps) svc() (*apps.Service, error) { return fullstack.Make[*apps.Service](h.App) }

func userID(c *web.Ctx) (int64, error) {
	u, ok := auth.User(c)
	if !ok {
		return 0, web.Error(http.StatusUnauthorized, "Log in first.")
	}
	return apps.UserID(u.ProviderUserID)
}

// current loads the application of the {app} parameter for the logged-in
// developer; someone else's application is a 404.
func (h Apps) current(c *web.Ctx) (*apps.Service, *models.Application, error) {
	svc, err := h.svc()
	if err != nil {
		return nil, nil, err
	}
	uid, err := userID(c)
	if err != nil {
		return nil, nil, err
	}
	a, err := svc.AppOf(c.Context(), uid, c.Param("app"))
	if errors.Is(err, apps.ErrNotFound) {
		return nil, nil, web.ErrNotFound
	}
	return svc, a, err
}

func fieldErrors(err error) (validation.Errors, bool) {
	if errs, ok := validation.ErrorsOf(err); ok {
		return errs, true
	}
	var fe *apps.FieldError
	if errors.As(err, &fe) {
		return validation.Errors{fe.Field: {fe.Message}}, true
	}
	return nil, false
}

// Index lists the applications with a form for a new one.
func (h Apps) Index(c *web.Ctx) error { return h.index(c, 200, nil, appForm{}) }

func (h Apps) index(c *web.Ctx, status int, errs validation.Errors, in appForm) error {
	svc, err := h.svc()
	if err != nil {
		return err
	}
	uid, err := userID(c)
	if err != nil {
		return err
	}
	list, err := svc.AppsOf(c.Context(), uid)
	if err != nil {
		return err
	}
	return c.HTML(status, "pages/apps/index", page(h.App, c, map[string]any{"Apps": list, "Errors": errs, "Form": in}))
}

// Create makes an application and opens it.
func (h Apps) Create(c *web.Ctx) error {
	var in appForm
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := fieldErrors(err); ok {
			return h.index(c, 422, errs, in)
		}
		return err
	}
	svc, err := h.svc()
	if err != nil {
		return err
	}
	uid, err := userID(c)
	if err != nil {
		return err
	}
	a, err := svc.CreateApp(c.Context(), uid, apps.AppInput{Name: in.Name, PasswordResetURL: in.PasswordResetURL})
	if errs, ok := fieldErrors(err); ok {
		return h.index(c, 422, errs, in)
	}
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/apps/"+a.PublicID)
}

// Show is an application: its settings and keys.
func (h Apps) Show(c *web.Ctx) error {
	svc, a, err := h.current(c)
	if err != nil {
		return err
	}
	return h.show(c, svc, a, 200, nil, appForm{Name: a.Name, PasswordResetURL: a.PasswordResetURL})
}

func (h Apps) show(c *web.Ctx, svc *apps.Service, a *models.Application, status int, errs validation.Errors, in appForm) error {
	list, err := svc.KeysOf(c.Context(), a)
	if err != nil {
		return err
	}
	return c.HTML(status, "pages/apps/show", page(h.App, c, map[string]any{"Application": a, "Keys": list, "Errors": errs, "Form": in}))
}

// Update saves an application's settings.
func (h Apps) Update(c *web.Ctx) error {
	svc, a, err := h.current(c)
	if err != nil {
		return err
	}
	var in appForm
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := fieldErrors(err); ok {
			return h.show(c, svc, a, 422, errs, in)
		}
		return err
	}
	err = svc.UpdateApp(c.Context(), a, apps.AppInput{Name: in.Name, PasswordResetURL: in.PasswordResetURL})
	if errs, ok := fieldErrors(err); ok {
		return h.show(c, svc, a, 422, errs, in)
	}
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/apps/"+a.PublicID)
}

// Delete deletes an application and its keys.
func (h Apps) Delete(c *web.Ctx) error {
	svc, a, err := h.current(c)
	if err != nil {
		return err
	}
	if err := svc.DeleteApp(c.Context(), a); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/apps")
}

// NewKey is the form for a key.
func (h Apps) NewKey(c *web.Ctx) error {
	_, a, err := h.current(c)
	if err != nil {
		return err
	}
	return h.newKey(c, a, 200, nil, keyForm{Kind: string(keys.Server), Scopes: []string{string(keys.AuthLogin)}})
}

func (h Apps) newKey(c *web.Ctx, a *models.Application, status int, errs validation.Errors, in keyForm) error {
	return c.HTML(status, "pages/apps/key_new", page(h.App, c, map[string]any{
		"Application": a, "Errors": errs, "Form": in, "Scopes": scopeOptions(in.Scopes),
	}))
}

// CreateKey makes a key and shows its secret once.
func (h Apps) CreateKey(c *web.Ctx) error {
	svc, a, err := h.current(c)
	if err != nil {
		return err
	}
	var in keyForm
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := fieldErrors(err); ok {
			return h.newKey(c, a, 422, errs, in)
		}
		return err
	}
	k, plain, err := svc.CreateKey(c.Context(), a, apps.KeyInput{Name: in.Name, Kind: keys.Kind(in.Kind),
		Scopes: in.Scopes, Origins: in.Origins, IPs: in.IPs})
	if errs, ok := fieldErrors(err); ok {
		return h.newKey(c, a, 422, errs, in)
	}
	if err != nil {
		return err
	}
	return h.secret(c, a, k, plain)
}

// Regenerate gives a key a new secret and shows it once.
func (h Apps) Regenerate(c *web.Ctx) error {
	svc, a, k, err := h.key(c)
	if err != nil {
		return err
	}
	plain, err := svc.Regenerate(c.Context(), k)
	if errors.Is(err, apps.ErrNotFound) {
		return web.ErrNotFound
	}
	if err != nil {
		return err
	}
	return h.secret(c, a, k, plain)
}

// Revoke stops a key.
func (h Apps) Revoke(c *web.Ctx) error {
	svc, a, k, err := h.key(c)
	if err != nil {
		return err
	}
	if err := svc.Revoke(c.Context(), k); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/apps/"+a.PublicID)
}

func (h Apps) key(c *web.Ctx) (*apps.Service, *models.Application, *models.APIKey, error) {
	svc, a, err := h.current(c)
	if err != nil {
		return nil, nil, nil, err
	}
	k, err := svc.KeyOf(c.Context(), a, c.Param("key"))
	if errors.Is(err, apps.ErrNotFound) {
		return nil, nil, nil, web.ErrNotFound
	}
	return svc, a, k, err
}

// secret renders the page with a key in plain text. It is never cached and
// never shown again.
func (h Apps) secret(c *web.Ctx, a *models.Application, k *models.APIKey, plain string) error {
	c.W.Header().Set("Cache-Control", "no-store")
	return c.HTML(200, "pages/apps/key_secret", page(h.App, c, map[string]any{"Application": a, "Key": k, "Secret": plain}))
}

type scopeOption struct {
	keys.ScopeInfo
	Checked bool
}

func scopeOptions(checked []string) []scopeOption {
	var out []scopeOption
	for _, info := range keys.Scopes() {
		on := false
		for _, s := range checked {
			if strings.TrimSpace(s) == string(info.Scope) {
				on = true
			}
		}
		out = append(out, scopeOption{info, on})
	}
	return out
}
