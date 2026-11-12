// Package locale holds the dashboard's languages: English, in which the
// templates and the code are written, and Russian. The English text is the
// key: a string with no translation is shown as written.
//
// The language comes from the luna_lang cookie (set by /prefs), else from
// the browser's Accept-Language; the theme from luna_theme (light, dark, or
// system: the operating system's choice through prefers-color-scheme).
package locale

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"fullstack-validation"
)

// Cookie names and how long a choice is kept.
const (
	LangCookie  = "luna_lang"
	ThemeCookie = "luna_theme"
	CookieAge   = 365 * 24 * time.Hour
)

// Lang is a language of the dashboard.
type Lang struct {
	code     string
	name     string
	dict     map[string]string
	patterns []pattern
	plurals  map[string][]string
	plural   func(n int) int
}

// Code is the language's tag: "en" or "ru".
func (l *Lang) Code() string { return l.code }

// Name is the language's own name: "English", "Русский".
func (l *Lang) Name() string { return l.name }

var (
	// EN is English, the language the dashboard is written in.
	EN = &Lang{code: "en", name: "English", plurals: map[string][]string{}, plural: func(n int) int {
		if n == 1 {
			return 0
		}
		return 1
	}}
	// RU is Russian.
	RU = &Lang{code: "ru", name: "Русский", dict: ru, patterns: ruPatterns(), plurals: ruPlurals, plural: ruPlural}

	// All is every language, in the order the switch shows them.
	All = []*Lang{EN, RU}
)

// Get returns the language with a code; English for any other.
func Get(code string) *Lang {
	for _, l := range All {
		if l.code == code {
			return l
		}
	}
	return EN
}

// Supported reports whether code is a language of the dashboard.
func Supported(code string) bool {
	for _, l := range All {
		if l.code == code {
			return true
		}
	}
	return false
}

// Detect returns the request's language: the cookie, else the first
// supported language of Accept-Language by quality, else English.
func Detect(r *http.Request) *Lang {
	if c, err := r.Cookie(LangCookie); err == nil && Supported(c.Value) {
		return Get(c.Value)
	}
	return FromAcceptLanguage(r.Header.Get("Accept-Language"))
}

// FromAcceptLanguage picks a language from an Accept-Language header
// ("ru-RU,ru;q=0.9,en-US;q=0.8"): the supported one with the highest
// quality, the first listed among equals.
func FromAcceptLanguage(header string) *Lang {
	type tag struct {
		code string
		q    float64
		i    int
	}
	var tags []tag
	for i, part := range strings.Split(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		code := strings.ToLower(strings.TrimSpace(fields[0]))
		if j := strings.IndexByte(code, '-'); j >= 0 {
			code = code[:j]
		}
		q := 1.0
		for _, f := range fields[1:] {
			if v, ok := strings.CutPrefix(strings.TrimSpace(f), "q="); ok {
				if p, err := strconv.ParseFloat(v, 64); err == nil {
					q = p
				}
			}
		}
		if q > 0 && Supported(code) {
			tags = append(tags, tag{code, q, i})
		}
	}
	sort.SliceStable(tags, func(a, b int) bool { return tags[a].q > tags[b].q })
	if len(tags) > 0 {
		return Get(tags[0].code)
	}
	return EN
}

// Themes are the colour schemes: the operating system's, light, dark.
var Themes = []string{"system", "light", "dark"}

// Theme returns the request's theme: the cookie, else "system".
func Theme(r *http.Request) string {
	if c, err := r.Cookie(ThemeCookie); err == nil && ValidTheme(c.Value) {
		return c.Value
	}
	return "system"
}

// ValidTheme reports whether s is one of Themes.
func ValidTheme(s string) bool {
	for _, t := range Themes {
		if t == s {
			return true
		}
	}
	return false
}

// T translates s; with args, the translation is a fmt format ("%s", "%d",
// "%[2]s" to reorder).
func (l *Lang) T(s string, args ...any) string {
	if t, ok := l.dict[s]; ok {
		s = t
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// HasRU reports whether s has a Russian translation (for the tests).
func HasRU(s string) bool { _, ok := ru[s]; return ok }

// H translates s, which holds trusted markup (<code>, <strong>, links),
// and fills in args HTML-escaped.
func (l *Lang) H(s string, args ...any) template.HTML {
	if t, ok := l.dict[s]; ok {
		s = t
	}
	if len(args) > 0 {
		esc := make([]any, len(args))
		for i, a := range args {
			esc[i] = template.HTMLEscapeString(fmt.Sprint(a))
		}
		s = fmt.Sprintf(s, esc...)
	}
	return template.HTML(s)
}

// N is a count with its noun: N 3 "user" → "3 users", "3 пользователя".
// A noun with no forms is shown as written.
func (l *Lang) N(n int, noun string) string {
	forms, ok := l.plurals[noun]
	if !ok {
		forms = englishForms(noun)
	}
	i := l.plural(n)
	if i >= len(forms) {
		i = len(forms) - 1
	}
	return strconv.Itoa(n) + " " + forms[i]
}

func englishForms(noun string) []string {
	if strings.HasSuffix(noun, "y") {
		return []string{noun, noun[:len(noun)-1] + "ies"}
	}
	return []string{noun, noun + "s"}
}

// Msg translates a message from the code or a service: a known sentence,
// or one of the validation messages with its field name.
func (l *Lang) Msg(s string) string {
	if t, ok := l.dict[s]; ok {
		return t
	}
	for _, p := range l.patterns {
		if m := p.re.FindStringSubmatch(s); m != nil {
			args := make([]any, len(m)-1)
			for i, v := range m[1:] {
				if i < len(p.fields) && p.fields[i] {
					v = l.field(v)
				}
				args[i] = v
			}
			return fmt.Sprintf(p.format, args...)
		}
	}
	return s
}

// field translates a humanized field name ("password confirmation").
func (l *Lang) field(name string) string {
	if t, ok := l.dict["field:"+name]; ok {
		return t
	}
	return name
}

// Errors translates form errors (validation.Errors).
func (l *Lang) Errors(errs validation.Errors) validation.Errors {
	if errs == nil || l == EN {
		return errs
	}
	out := make(validation.Errors, len(errs))
	for k, msgs := range errs {
		t := make([]string, len(msgs))
		for i, m := range msgs {
			t[i] = l.Msg(m)
		}
		out[k] = t
	}
	return out
}

// pattern is a translated message with parts: the English message as a
// regexp, the translation as a fmt format, which parts are field names.
type pattern struct {
	re     *regexp.Regexp
	format string
	fields []bool
}

// compile turns "The {field} field must be at least {param} characters."
// into a pattern; {field} parts are translated, {param} and {any} kept.
func compile(en, format string) pattern {
	var fields []bool
	var b strings.Builder
	b.WriteString("^")
	rest := en
	for {
		i := strings.IndexByte(rest, '{')
		if i < 0 {
			b.WriteString(regexp.QuoteMeta(rest))
			break
		}
		j := strings.IndexByte(rest[i:], '}')
		b.WriteString(regexp.QuoteMeta(rest[:i]))
		b.WriteString("(.+?)")
		fields = append(fields, rest[i+1:i+j] == "field")
		rest = rest[i+j+1:]
	}
	b.WriteString("$")
	return pattern{regexp.MustCompile(b.String()), format, fields}
}
