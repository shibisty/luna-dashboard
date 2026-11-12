package tests

import (
	"io/fs"
	"net/url"
	"regexp"
	"strconv"
	"testing"

	apptest "fullstack-testing"

	"luna-dashboard/app/locale"
	"luna-dashboard/resources"
)

// Every text the templates translate has a Russian translation.
func TestEveryTemplateTextIsTranslated(t *testing.T) {
	text := regexp.MustCompile(`\.(?:T|H) ("(?:[^"\\]|\\.)*")`)
	n := 0
	err := fs.WalkDir(resources.Views, "views", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(resources.Views, path)
		if err != nil {
			return err
		}
		for _, m := range text.FindAllStringSubmatch(string(b), -1) {
			s, err := strconv.Unquote(m[1])
			if err != nil {
				t.Errorf("%s: %s", path, m[1])
				continue
			}
			n++
			if !locale.HasRU(s) {
				t.Errorf("%s: no Russian for %q", path, s)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 100 {
		t.Fatalf("only %d texts found", n)
	}
}

func TestLanguageAndTheme(t *testing.T) {
	app := newApp(t)

	t.Run("English by default, Russian from the browser", func(t *testing.T) {
		apptest.New(t, app).Get("/login").AssertOK().
			AssertSee(`<html lang="en" data-theme="system">`, "Log in", "Remember me", `content="light dark"`)
		apptest.New(t, app).WithHeader("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8").Get("/login").AssertOK().
			AssertSee(`<html lang="ru" data-theme="system">`, "Войти", "Запомнить меня", "Регистрация")
	})

	t.Run("/prefs keeps the choice and goes back", func(t *testing.T) {
		c := apptest.New(t, app)
		r := c.Get("/prefs?lang=ru&theme=dark&back=" + url.QueryEscape("/login?redirect=/apps")).AssertRedirect("/login?redirect=/apps")
		for _, name := range []string{locale.LangCookie, locale.ThemeCookie} {
			ck := cookieOf(r, name)
			if ck == nil || ck.MaxAge < 300*24*3600 || !ck.HttpOnly || ck.Path != "/" {
				t.Fatalf("%s: %+v", name, ck)
			}
		}
		c.WithHeader("Accept-Language", "en").Get("/login").
			AssertSee(`<html lang="ru" data-theme="dark">`, `content="dark"`, "Войти", `aria-current="true">RU`)
		c.Get("/prefs?lang=en&theme=light&back=/").AssertRedirect("/")
		c.Get("/login").AssertSee(`<html lang="en" data-theme="light">`, "Log in")
	})

	t.Run("/prefs goes back only to this site", func(t *testing.T) {
		c := apptest.New(t, app)
		for _, back := range []string{"//evil.example/x", "https://evil.example", `/\evil.example`, "evil", ""} {
			c.Get("/prefs?theme=dark&back=" + url.QueryEscape(back)).AssertRedirect("/")
		}
		r := c.Get("/prefs?lang=de&theme=pink&back=/")
		if cookieOf(r, locale.LangCookie) != nil || cookieOf(r, locale.ThemeCookie) != nil {
			t.Fatal("unknown values are not kept")
		}
	})

	t.Run("form errors in Russian", func(t *testing.T) {
		c := apptest.New(t, app).WithHeader("Accept-Language", "ru")
		c.Get("/register")
		c.Post("/register", url.Values{"name": {""}, "email": {"nope"}, "password": {"short"}, "password_confirmation": {"other"}}).
			AssertUnprocessable().
			AssertSee("Поле «Название» обязательно.", "Поле «Email» должно быть корректным email-адресом.",
				"Поле «Пароль» должно быть не короче 8 символов.", "Поле «Повтор пароля» должно совпадать с полем «Пароль».")
	})

	t.Run("signed-in pages in Russian, the switch comes back to them", func(t *testing.T) {
		c := developer(t, apptest.New(t, app), "lang@example.com")
		id := createApp(t, c, "Магазин")
		c = c.WithHeader("Accept-Language", "ru")
		c.Get("/apps").AssertSee("Приложения", "Новое приложение", "Магазин", "Выйти")
		c.Get("/apps/"+id).AssertSee("API-ключи", "Настройки", "Удалить Магазин", "back=%2fapps%2f"+id)
		c.Get("/apps/"+id+"/keys/new").AssertSee("Новый API-ключ", "Регистрация, вход и выход пользователей, сброс паролей")
		c.Post("/apps/"+id+"/keys", url.Values{"kind": {"browser"}, "scopes": {"auth:login"}}).AssertUnprocessable().
			AssertSee("Браузерному ключу нужен хотя бы один разрешённый источник.")
	})

	t.Run("error pages in the dashboard's look", func(t *testing.T) {
		c := developer(t, apptest.New(t, app), "errors@example.com").WithHeader("Accept", "text/html").WithHeader("Accept-Language", "ru")
		c.Get("/apps/nope").AssertNotFound().AssertSee(`data-theme="system"`, "Не найдено", "Приложения")
		apptest.New(t, app).WithHeader("Accept", "application/json").Get("/apps/nope/pages/api/editor").
			AssertStatus(401)
	})
}
