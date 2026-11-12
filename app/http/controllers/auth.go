package controllers

import (
	"errors"

	"fullstack"
	auth "fullstack-passport"
	"fullstack-validation"
	"fullstack/web"

	"luna-dashboard/app/sessions"
	"luna-dashboard/app/users"
)

// Auth serves login, registration and logout.
type Auth struct {
	App    *fullstack.App
	Auth   *auth.Auth
	Keeper *sessions.Keeper
}

type loginForm struct {
	Email    string `form:"email" validate:"required,email"`
	Password string `form:"password" validate:"required"`
	Remember bool   `form:"remember"`
}

type registerForm struct {
	Name     string `form:"name" validate:"required,max=100"`
	Email    string `form:"email" validate:"required,email,max=254"`
	Password string `form:"password" validate:"required,min=8,max=200"`
	Confirm  string `form:"password_confirmation" validate:"same=Password"`
}

// ShowLogin is the login form.
func (h Auth) ShowLogin(c *web.Ctx) error {
	return c.HTML(200, "pages/login", page(h.App, c, map[string]any{"Redirect": c.Query("redirect")}))
}

// Login checks the credentials and starts the session.
func (h Auth) Login(c *web.Ctx) error {
	var in loginForm
	fail := func(errs validation.Errors) error {
		return c.HTML(422, "pages/login", page(h.App, c, map[string]any{"Errors": errs, "Email": in.Email, "Remember": in.Remember, "Redirect": c.Query("redirect")}))
	}
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := validation.ErrorsOf(err); ok {
			return fail(errs)
		}
		return err
	}
	id, err := h.Auth.Credential(c.Context(), "local", map[string]string{"email": in.Email, "password": in.Password})
	if errors.Is(err, auth.ErrInvalidCredentials) {
		return fail(validation.Errors{"email": {"These credentials do not match our records."}})
	}
	if err != nil {
		return err
	}
	if err := h.Keeper.Login(c, id, in.Remember); err != nil {
		return err
	}
	return c.Redirect(0, h.Auth.Intended(c, "/dashboard"))
}

// ShowRegister is the registration form.
func (h Auth) ShowRegister(c *web.Ctx) error {
	return c.HTML(200, "pages/register", page(h.App, c, nil))
}

// Register creates the user and logs them in.
func (h Auth) Register(c *web.Ctx) error {
	var in registerForm
	fail := func(errs validation.Errors) error {
		return c.HTML(422, "pages/register", page(h.App, c, map[string]any{"Errors": errs, "Name": in.Name, "Email": in.Email}))
	}
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := validation.ErrorsOf(err); ok {
			return fail(errs)
		}
		return err
	}
	svc, err := fullstack.Make[*users.Service](h.App)
	if err != nil {
		return err
	}
	u, err := svc.Register(c.Context(), in.Name, in.Email, in.Password)
	if errors.Is(err, users.ErrEmailTaken) {
		return fail(validation.Errors{"email": {"The email has already been taken."}})
	}
	if err != nil {
		return err
	}
	if err := h.Keeper.Login(c, users.Identity(u), false); err != nil {
		return err
	}
	return c.Redirect(0, "/dashboard")
}

// Logout ends the session.
func (h Auth) Logout(c *web.Ctx) error {
	if err := h.Keeper.Logout(c); err != nil {
		return err
	}
	return c.Redirect(0, "/")
}
