package locale

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"fullstack-validation"
)

func TestFromAcceptLanguage(t *testing.T) {
	for header, want := range map[string]string{
		"":                                "en",
		"ru-RU,ru;q=0.9,en-US;q=0.8":      "ru",
		"en-US,en;q=0.9,ru;q=0.8":         "en",
		"de-DE,de;q=0.9,ru;q=0.8,en;q=.7": "ru",
		"fr, en;q=0.1, ru;q=0.5":          "ru",
		"ru;q=0, en":                      "en",
		"uk":                              "en",
		"RU":                              "ru",
	} {
		if got := FromAcceptLanguage(header).Code(); got != want {
			t.Errorf("%q: %s, want %s", header, got, want)
		}
	}
}

func TestDetectPrefersTheCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Language", "ru")
	if Detect(r) != RU {
		t.Fatal("Accept-Language")
	}
	r.AddCookie(&http.Cookie{Name: LangCookie, Value: "en"})
	if Detect(r) != EN {
		t.Fatal("the cookie should win")
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: LangCookie, Value: "xx"})
	r.AddCookie(&http.Cookie{Name: ThemeCookie, Value: "pink"})
	if Detect(r) != EN || Theme(r) != "system" {
		t.Fatal("unknown cookie values are ignored")
	}
}

func TestTranslate(t *testing.T) {
	if got := RU.T("Delete %s", "Shop"); got != "Удалить Shop" {
		t.Fatal(got)
	}
	if got := EN.T("Delete %s", "Shop"); got != "Delete Shop" {
		t.Fatal(got)
	}
	if got := RU.T("Untranslated"); got != "Untranslated" {
		t.Fatal(got)
	}
	if got := string(RU.H("It works only from %s.", "<b>")); got != "Он работает только с &lt;b&gt;." {
		t.Fatal(got)
	}
	for n, want := range map[int]string{0: "0 пользователей", 1: "1 пользователь", 2: "2 пользователя", 5: "5 пользователей",
		11: "11 пользователей", 21: "21 пользователь", 22: "22 пользователя", 112: "112 пользователей"} {
		if got := RU.N(n, "user"); got != want {
			t.Errorf("%d: %s", n, got)
		}
	}
	if EN.N(1, "user") != "1 user" || EN.N(2, "user") != "2 users" || RU.N(2, "entry") != "2 entries" {
		t.Fatal("English plurals")
	}
}

func TestValidationMessages(t *testing.T) {
	for en, ru := range map[string]string{
		"The email field is required.":                            "Поле «Email» обязательно.",
		"The password field must be at least 8 characters.":       "Поле «Пароль» должно быть не короче 8 символов.",
		"The name field must not be greater than 100 characters.": "Поле «Название» должно быть не длиннее 100 символов.",
		"The password confirmation field must match password.":    "Поле «Повтор пароля» должно совпадать с полем «Пароль».",
		"The selected kind is invalid (one of server, browser).":  "Недопустимое значение поля «Тип» (одно из: server, browser).",
		"The Price field is required.":                            "Поле «Price» обязательно.",
		"These credentials do not match our records.":             "Неверный email или пароль.",
		"Something new.": "Something new.",
	} {
		if got := RU.Msg(en); got != ru {
			t.Errorf("%q: %q", en, got)
		}
	}
	errs := RU.Errors(validation.Errors{"email": {"The email field is required."}})
	if errs["email"][0] != "Поле «Email» обязательно." {
		t.Fatal(errs)
	}
	if EN.Msg("The email field is required.") != "The email field is required." {
		t.Fatal("English")
	}
}
