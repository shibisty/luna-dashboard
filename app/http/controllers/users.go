package controllers

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"fullstack"
	"fullstack-validation"
	"fullstack/web"

	"luna-dashboard/app/authapi"
	"luna-dashboard/app/models"
)

// Users serves an application's end users and the application's data
// about them, kept by luna-auth (authapi).
type Users struct{ App *fullstack.App }

const usersPerPage = 25

type newUserForm struct {
	Username      string `form:"username" validate:"max=32"`
	Email         string `form:"email" validate:"required,email,max=254"`
	Password      string `form:"password" validate:"required,min=8,max=200"`
	EmailVerified bool   `form:"email_verified"`
}

type userForm struct {
	Username      string `form:"username" validate:"max=32"`
	Email         string `form:"email" validate:"required,email,max=254"`
	EmailVerified bool   `form:"email_verified"`
	Password      string `form:"password" validate:"omitempty,min=8,max=200"`
}

type dataForm struct {
	Key   string `form:"key" validate:"required,max=64"`
	Value string `form:"value" validate:"max=16384"`
}

func (h Users) client() (*authapi.Client, error) { return fullstack.Make[*authapi.Client](h.App) }

// current is the developer's application of {app} (404 for anyone else's)
// and the luna-auth client.
func (h Users) current(c *web.Ctx) (*authapi.Client, *models.Application, error) {
	_, a, err := Apps(h).current(c)
	if err != nil {
		return nil, nil, err
	}
	cl, err := h.client()
	return cl, a, err
}

// apiErrors turns luna-auth's 422 into form errors.
func apiErrors(err error) (validation.Errors, bool) {
	if errs, ok := validation.ErrorsOf(err); ok {
		return errs, true
	}
	var e *authapi.Error
	if errors.As(err, &e) && e.Status == http.StatusUnprocessableEntity && len(e.Fields) > 0 {
		return validation.Errors(e.Fields), true
	}
	return nil, false
}

// unavailable is the page shown when luna-auth cannot be asked.
func (h Users) unavailable(c *web.Ctx, a *models.Application, err error) error {
	msg := "luna-auth cannot be reached right now. Try again in a moment."
	if errors.Is(err, authapi.ErrNotConfigured) {
		msg = "luna-auth is not connected to this dashboard (LUNA_AUTH_URL and LUNA_AUTH_TOKEN)."
	}
	h.App.Log.Error("users: luna-auth", "error", err)
	return c.HTML(http.StatusBadGateway, "pages/apps/users/unavailable", page(h.App, c, map[string]any{"Application": a, "Message": msg}))
}

// Index lists the users: search, pages, and a form for a new user.
func (h Users) Index(c *web.Ctx) error {
	return h.index(c, http.StatusOK, nil, newUserForm{})
}

func (h Users) index(c *web.Ctx, status int, errs validation.Errors, in newUserForm) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	q := strings.TrimSpace(c.Query("q"))
	n, _ := strconv.Atoi(c.Query("page"))
	if n < 1 {
		n = 1
	}
	p, err := cl.Users(c.Context(), a.PublicID, q, n, usersPerPage)
	if err != nil {
		return h.unavailable(c, a, err)
	}
	link := func(page int) string {
		v := url.Values{"page": {strconv.Itoa(page)}}
		if q != "" {
			v.Set("q", q)
		}
		return "/apps/" + a.PublicID + "/users?" + v.Encode()
	}
	data := map[string]any{"Application": a, "Users": p, "Query": q, "Errors": errs, "Form": in}
	if p.Page > 1 {
		data["PrevURL"] = link(p.Page - 1)
	}
	if p.Page < p.LastPage {
		data["NextURL"] = link(p.Page + 1)
	}
	return c.HTML(status, "pages/apps/users/index", page(h.App, c, data))
}

// Create adds a user.
func (h Users) Create(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	var in newUserForm
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := apiErrors(err); ok {
			in.Password = ""
			return h.index(c, http.StatusUnprocessableEntity, errs, in)
		}
		return err
	}
	u, err := cl.CreateUser(c.Context(), a.PublicID, authapi.NewUser{Username: in.Username, Email: in.Email, Password: in.Password, EmailVerified: in.EmailVerified})
	if errs, ok := apiErrors(err); ok {
		in.Password = ""
		return h.index(c, http.StatusUnprocessableEntity, errs, in)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, userURL(a, u.ID))
}

func userURL(a *models.Application, id string) string {
	return "/apps/" + a.PublicID + "/users/" + url.PathEscape(id)
}

// Show is a user: their account and the application's data about them.
func (h Users) Show(c *web.Ctx) error { return h.show(c, http.StatusOK, nil, nil, nil, nil) }

// show renders the user page; form (the account) and data (the new value)
// keep what was typed when errs or dataErrs refused it.
func (h Users) show(c *web.Ctx, status int, errs validation.Errors, form *userForm, dataErrs validation.Errors, data *dataForm) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	id := c.Param("user")
	u, err := cl.User(c.Context(), a.PublicID, id)
	if errors.Is(err, authapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	values, err := cl.Data(c.Context(), a.PublicID, id)
	if err != nil {
		return h.unavailable(c, a, err)
	}
	if form == nil {
		form = &userForm{Username: u.Name(), Email: u.Email, EmailVerified: u.EmailVerified}
	}
	if data == nil {
		data = &dataForm{}
	}
	return c.HTML(status, "pages/apps/users/show", page(h.App, c, map[string]any{
		"Application": a, "User": u, "Data": values, "Form": form, "Errors": errs,
		"DataForm": data, "DataErrors": dataErrs, "URL": userURL(a, u.ID),
	}))
}

// Update saves the account: username, email, verification, a new password.
func (h Users) Update(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	var in userForm
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := apiErrors(err); ok {
			in.Password = ""
			return h.show(c, http.StatusUnprocessableEntity, errs, &in, nil, nil)
		}
		return err
	}
	ch := authapi.Changes{Username: &in.Username, Email: &in.Email, EmailVerified: &in.EmailVerified}
	if in.Password != "" {
		ch.Password = &in.Password
	}
	_, err = cl.UpdateUser(c.Context(), a.PublicID, c.Param("user"), ch)
	if errors.Is(err, authapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if errs, ok := apiErrors(err); ok {
		in.Password = ""
		return h.show(c, http.StatusUnprocessableEntity, errs, &in, nil, nil)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, userURL(a, c.Param("user")))
}

// Delete deletes the user, their data and their logins.
func (h Users) Delete(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	err = cl.DeleteUser(c.Context(), a.PublicID, c.Param("user"))
	if err != nil && !errors.Is(err, authapi.ErrNotFound) {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, "/apps/"+a.PublicID+"/users")
}

// SetData adds a key or changes its value.
func (h Users) SetData(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	var in dataForm
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := apiErrors(err); ok {
			return h.show(c, http.StatusUnprocessableEntity, nil, nil, errs, &in)
		}
		return err
	}
	err = cl.SetData(c.Context(), a.PublicID, c.Param("user"), strings.TrimSpace(in.Key), in.Value)
	if errors.Is(err, authapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if errs, ok := apiErrors(err); ok {
		return h.show(c, http.StatusUnprocessableEntity, nil, nil, errs, &in)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, userURL(a, c.Param("user"))+"#data")
}

// DeleteData deletes a key.
func (h Users) DeleteData(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	err = cl.DeleteData(c.Context(), a.PublicID, c.Param("user"), c.Param("key"))
	if err != nil && !errors.Is(err, authapi.ErrNotFound) {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, userURL(a, c.Param("user"))+"#data")
}
