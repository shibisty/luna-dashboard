// Package apps manages developers' applications and their API keys, and
// answers the other Luna services about keys (Lookup).
package apps

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fullstack"
	database "fullstack-orm"
	"luna-keys"
	"orm"

	"luna-dashboard/app/models"
)

// ErrNotFound is returned for an application or key that does not exist
// or belongs to someone else (the two are not told apart).
var ErrNotFound = errors.New("not found")

// Service is in the container (Provider).
type Service struct {
	app *fullstack.App
	now func() time.Time
}

// Provider registers *apps.Service.
type Provider struct{}

// Register implements fullstack.Provider.
func (Provider) Register(app *fullstack.App) error {
	fullstack.Provide(app, func(app *fullstack.App) (*Service, error) { return &Service{app: app, now: time.Now}, nil })
	return nil
}

func (s *Service) appsRepo() (*orm.Repository[models.Application], error) {
	return database.Repo[models.Application](s.app)
}

func (s *Service) keysRepo() (*orm.Repository[models.APIKey], error) {
	return database.Repo[models.APIKey](s.app)
}

// AppInput is what a developer types for an application.
type AppInput struct {
	Name             string
	PasswordResetURL string
}

// FieldError is an invalid input field; controllers show it next to the field.
type FieldError struct{ Field, Message string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func (in *AppInput) clean() error {
	in.Name = strings.TrimSpace(in.Name)
	in.PasswordResetURL = strings.TrimSpace(in.PasswordResetURL)
	if in.PasswordResetURL != "" {
		u, err := url.Parse(in.PasswordResetURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return &FieldError{"password_reset_url", "The password reset URL must be an http or https address."}
		}
	}
	return nil
}

// CreateApp creates an application of the user.
func (s *Service) CreateApp(ctx context.Context, userID int64, in AppInput) (*models.Application, error) {
	if err := in.clean(); err != nil {
		return nil, err
	}
	repo, err := s.appsRepo()
	if err != nil {
		return nil, err
	}
	a := &models.Application{PublicID: newUUID(), UserID: userID, Name: in.Name, PasswordResetURL: in.PasswordResetURL}
	if err := repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// AppsOf lists the user's applications, newest first.
func (s *Service) AppsOf(ctx context.Context, userID int64) ([]models.Application, error) {
	repo, err := s.appsRepo()
	if err != nil {
		return nil, err
	}
	return repo.Where("user_id", "=", userID).OrderByDesc("id").Get(ctx)
}

// AppOf returns the user's application with the public id.
func (s *Service) AppOf(ctx context.Context, userID int64, publicID string) (*models.Application, error) {
	if !validUUID(publicID) {
		return nil, ErrNotFound
	}
	repo, err := s.appsRepo()
	if err != nil {
		return nil, err
	}
	a, err := repo.Where("public_id", "=", publicID).Where("user_id", "=", userID).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return nil, ErrNotFound
	}
	return a, err
}

// UpdateApp changes an application's settings.
func (s *Service) UpdateApp(ctx context.Context, a *models.Application, in AppInput) error {
	if err := in.clean(); err != nil {
		return err
	}
	repo, err := s.appsRepo()
	if err != nil {
		return err
	}
	a.Name, a.PasswordResetURL = in.Name, in.PasswordResetURL
	return repo.Update(ctx, a)
}

// DeleteApp deletes an application with its keys.
func (s *Service) DeleteApp(ctx context.Context, a *models.Application) error {
	repo, err := s.appsRepo()
	if err != nil {
		return err
	}
	return repo.Delete(ctx, a.ID)
}

// KeyInput is what a developer chooses for a key.
type KeyInput struct {
	Name    string
	Kind    keys.Kind
	Scopes  []string
	Origins string // one per line (browser keys)
	IPs     string // one per line, CIDRs or addresses; empty: any
}

type cleanKey struct {
	name         string
	kind         keys.Kind
	scopes       []keys.Scope
	origins, ips []string
}

func (in KeyInput) clean() (cleanKey, error) {
	out := cleanKey{name: strings.TrimSpace(in.Name), kind: in.Kind}
	if out.kind != keys.Server && out.kind != keys.Browser {
		return out, &FieldError{"kind", "Choose a server or a browser key."}
	}
	scopes, err := keys.ParseScopes(in.Scopes)
	if err != nil {
		return out, &FieldError{"scopes", "Unknown scope."}
	}
	if err := keys.CheckScopes(out.kind, scopes); err != nil {
		if len(scopes) == 0 {
			return out, &FieldError{"scopes", "Choose at least one scope."}
		}
		return out, &FieldError{"scopes", "A browser key may only hold auth:login."}
	}
	out.scopes = scopes
	for _, line := range lines(in.Origins) {
		o, ok := normOrigin(line)
		if !ok {
			return out, &FieldError{"origins", fmt.Sprintf("%q is not an origin such as https://app.example.", line)}
		}
		out.origins = appendNew(out.origins, o)
	}
	if out.kind == keys.Browser && len(out.origins) == 0 {
		return out, &FieldError{"origins", "A browser key needs at least one allowed origin."}
	}
	for _, line := range lines(in.IPs) {
		p, err := keys.ParseNetwork(line)
		if err != nil {
			return out, &FieldError{"ips", fmt.Sprintf("%q is not an address or a network such as 203.0.113.0/24.", line)}
		}
		out.ips = appendNew(out.ips, p.String())
	}
	return out, nil
}

// CreateKey makes a key for the application and returns it in plain text:
// the only time it exists outside the developer's hands.
func (s *Service) CreateKey(ctx context.Context, a *models.Application, in KeyInput) (*models.APIKey, string, error) {
	c, err := in.clean()
	if err != nil {
		return nil, "", err
	}
	repo, err := s.keysRepo()
	if err != nil {
		return nil, "", err
	}
	plain, hash, err := keys.Generate(c.kind)
	if err != nil {
		return nil, "", err
	}
	k := &models.APIKey{PublicID: newUUID(), ApplicationID: a.ID, Name: c.name, Kind: string(c.kind),
		Hash: hash, Hint: keys.Hint(plain), Scopes: scopeStrings(c.scopes), Origins: c.origins, IPs: c.ips}
	if k.Name == "" {
		k.Name = strings.ToUpper(string(c.kind[:1])) + string(c.kind[1:]) + " key"
	}
	if err := repo.Create(ctx, k); err != nil {
		return nil, "", err
	}
	return k, plain, nil
}

// KeysOf lists an application's keys, active first, newest first.
func (s *Service) KeysOf(ctx context.Context, a *models.Application) ([]models.APIKey, error) {
	repo, err := s.keysRepo()
	if err != nil {
		return nil, err
	}
	all, err := repo.Where("application_id", "=", a.ID).OrderByDesc("id").Get(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.APIKey, 0, len(all))
	for _, k := range all {
		if k.Active() {
			out = append(out, k)
		}
	}
	for _, k := range all {
		if !k.Active() {
			out = append(out, k)
		}
	}
	return out, nil
}

// KeyOf returns the application's key with the public id.
func (s *Service) KeyOf(ctx context.Context, a *models.Application, publicID string) (*models.APIKey, error) {
	if !validUUID(publicID) {
		return nil, ErrNotFound
	}
	repo, err := s.keysRepo()
	if err != nil {
		return nil, err
	}
	k, err := repo.Where("public_id", "=", publicID).Where("application_id", "=", a.ID).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return nil, ErrNotFound
	}
	return k, err
}

// Revoke stops a key; services notice within their cache time (60 s).
func (s *Service) Revoke(ctx context.Context, k *models.APIKey) error {
	if !k.Active() {
		return nil
	}
	repo, err := s.keysRepo()
	if err != nil {
		return err
	}
	now := s.now().UTC()
	k.RevokedAt = &now
	return repo.Update(ctx, k)
}

// Regenerate gives an active key a new secret with the same settings and
// returns it in plain text; the old secret stops working like a revoked key.
func (s *Service) Regenerate(ctx context.Context, k *models.APIKey) (string, error) {
	if !k.Active() {
		return "", ErrNotFound
	}
	repo, err := s.keysRepo()
	if err != nil {
		return "", err
	}
	plain, hash, err := keys.Generate(keys.Kind(k.Kind))
	if err != nil {
		return "", err
	}
	k.Hash, k.Hint, k.LastUsedAt = hash, keys.Hint(plain), nil
	if err := repo.Update(ctx, k); err != nil {
		return "", err
	}
	return plain, nil
}

// Lookup is the introspection lookup (keys.LookupFunc): the active key
// with the hash and its application. It records the key's last use, at
// most once a minute.
func (s *Service) Lookup(ctx context.Context, hash string) (*keys.Key, error) {
	repo, err := s.keysRepo()
	if err != nil {
		return nil, err
	}
	k, err := repo.Where("hash", "=", hash).WhereNull("revoked_at").First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return nil, keys.ErrUnknownKey
	}
	if err != nil {
		return nil, err
	}
	apps, err := s.appsRepo()
	if err != nil {
		return nil, err
	}
	a, err := apps.Find(ctx, k.ApplicationID)
	if errors.Is(err, orm.ErrNotFound) {
		return nil, keys.ErrUnknownKey
	}
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	if k.LastUsedAt == nil || now.Sub(*k.LastUsedAt) > time.Minute {
		// best effort: a failed write must not fail the check
		_, _ = repo.Where("id", "=", k.ID).Update(ctx, map[string]any{"last_used_at": now})
	}
	scopes := make([]keys.Scope, len(k.Scopes))
	for i, sc := range k.Scopes {
		scopes[i] = keys.Scope(sc)
	}
	return &keys.Key{ID: k.PublicID, AppID: a.PublicID, AppName: a.Name, Kind: keys.Kind(k.Kind),
		Scopes: scopes, Origins: []string(k.Origins), IPs: []string(k.IPs)}, nil
}

// UserID reads the owner id from a passport identity's ProviderUserID.
func UserID(providerUserID string) (int64, error) {
	return strconv.ParseInt(providerUserID, 10, 64)
}

func scopeStrings(scopes []keys.Scope) models.StringList {
	out := make(models.StringList, len(scopes))
	for i, s := range scopes {
		out[i] = string(s)
	}
	return out
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' }) {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func appendNew(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// normOrigin turns "https://App.Example/" into "https://app.example"; an
// origin is a scheme, a host and an optional port, nothing else.
func normOrigin(s string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", false
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), true
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return false
			}
		}
	}
	return true
}
