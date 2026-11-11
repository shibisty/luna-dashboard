package controllers

import (
	"encoding/json"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"fullstack-validation"

	"luna-dashboard/app/pagesapi"
)

// An entry's quick form: one input per template field, named after the
// value's path ("v.title", "v.photo.url", "v.specs.0.name"), so that the
// form can be read back into the values grapesjs-storage keeps.

// FieldInput is one field of the form.
type FieldInput struct {
	Field   pagesapi.Field
	Name    string // the form name, "v.title"
	ID      string
	Value   string // text, rich text, number, date, select
	Checked bool   // boolean
	URL     string // image, link
	Alt     string // image
	Label   string // link
	Items   []ItemInput
	Error   string
}

// ItemInput is one item of a list field; the last one is empty, for a new item.
type ItemInput struct {
	Index  int
	Number int    // Index + 1, for people
	Name   string // "v.specs.0"
	Fields []FieldInput
	New    bool
}

var idUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// errKey is the key of a value's errors: the form name without "v.".
func errKey(name string) string { return strings.TrimPrefix(name, "v.") }

func fieldInputs(fields []pagesapi.Field, values map[string]json.RawMessage, errs validation.Errors, prefix string) []FieldInput {
	out := make([]FieldInput, 0, len(fields))
	for _, f := range fields {
		name := prefix + f.Name
		in := FieldInput{Field: f, Name: name, ID: "f-" + idUnsafe.ReplaceAllString(name, "-")}
		if msgs := errs[errKey(name)]; len(msgs) > 0 {
			in.Error = msgs[0]
		}
		raw := values[f.Name]
		switch f.Type {
		case "boolean":
			var b bool
			in.Checked = json.Unmarshal(raw, &b) == nil && b
		case "image":
			in.URL, in.Alt = pair(raw, "alt")
		case "link":
			in.URL, in.Label = pair(raw, "label")
		case "date":
			in.Value = asText(raw)
			if len(in.Value) > 10 {
				in.Value = in.Value[:10]
			}
		case "list":
			var items []map[string]json.RawMessage
			_ = json.Unmarshal(raw, &items)
			for i, item := range items {
				p := name + "." + strconv.Itoa(i)
				in.Items = append(in.Items, ItemInput{Index: i, Number: i + 1, Name: p, Fields: fieldInputs(f.Fields, item, errs, p+".")})
			}
			n := len(items)
			p := name + "." + strconv.Itoa(n)
			in.Items = append(in.Items, ItemInput{Index: n, Number: n + 1, Name: p, Fields: fieldInputs(f.Fields, nil, nil, p+"."), New: true})
			if in.Error == "" { // an item's problem, shown at the list too
				for k, msgs := range errs {
					if strings.HasPrefix(k, errKey(name)+".") && len(msgs) > 0 {
						in.Error = msgs[0]
						break
					}
				}
			}
		default:
			in.Value = asText(raw)
		}
		out = append(out, in)
	}
	return out
}

// asText is a value as an input shows it.
func asText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// pair reads an image ({url, alt}) or a link ({url, label}); a plain
// string is the URL.
func pair(raw json.RawMessage, second string) (string, string) {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		u, _ := obj["url"].(string)
		s, _ := obj[second].(string)
		return u, s
	}
	return asText(raw), ""
}

var null = json.RawMessage("null")

func marshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// formValues reads the form back into values; an empty input is null
// (luna-pages then drops the value). List items left empty, or ticked for
// removal, are dropped and the rest renumbered.
func formValues(fields []pagesapi.Field, form url.Values, prefix string) (map[string]json.RawMessage, validation.Errors) {
	values := map[string]json.RawMessage{}
	var errs validation.Errors
	for _, f := range fields {
		name := prefix + f.Name
		get := func(k string) string { return form.Get(k) }
		switch f.Type {
		case "boolean":
			values[f.Name] = marshal(get(name) != "")
		case "number":
			s := strings.TrimSpace(get(name))
			if s == "" {
				values[f.Name] = null
				continue
			}
			n, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
			if err != nil {
				errs = add(errs, errKey(name), "Enter a number.")
				values[f.Name] = marshal(s)
				continue
			}
			values[f.Name] = json.RawMessage(strconv.FormatFloat(n, 'f', -1, 64))
		case "image", "link":
			second := map[string]string{"image": "alt", "link": "label"}[f.Type]
			u, s := strings.TrimSpace(get(name+".url")), strings.TrimSpace(get(name+"."+second))
			if u == "" && s == "" {
				values[f.Name] = null
				continue
			}
			values[f.Name] = marshal(map[string]string{"url": u, second: s})
		case "select":
			s := get(name)
			if s == "" {
				values[f.Name] = null
				continue
			}
			if !slices.Contains(f.Options, s) {
				errs = add(errs, errKey(name), "Choose one of the options.")
			}
			values[f.Name] = marshal(s)
		case "list":
			items, itemErrs := formItems(f, form, name)
			for k, v := range itemErrs {
				errs = add(errs, k, v[0])
			}
			values[f.Name] = marshal(items)
		case "richtext":
			if s := get(name); strings.TrimSpace(s) != "" {
				values[f.Name] = marshal(s)
			} else {
				values[f.Name] = null
			}
		default: // text, date, unknown types
			if s := strings.TrimSpace(get(name)); s != "" {
				values[f.Name] = marshal(s)
			} else {
				values[f.Name] = null
			}
		}
	}
	return values, errs
}

func formItems(f pagesapi.Field, form url.Values, name string) ([]map[string]json.RawMessage, validation.Errors) {
	seen := map[int]bool{}
	for k := range form {
		rest, ok := strings.CutPrefix(k, name+".")
		if !ok {
			continue
		}
		idx, _, _ := strings.Cut(rest, ".")
		if i, err := strconv.Atoi(idx); err == nil && i >= 0 && i < 1000 {
			seen[i] = true
		}
	}
	order := make([]int, 0, len(seen))
	for i := range seen {
		order = append(order, i)
	}
	sort.Ints(order)
	items := []map[string]json.RawMessage{}
	var errs validation.Errors
	for _, i := range order {
		p := name + "." + strconv.Itoa(i)
		if form.Get(p+".remove") != "" {
			continue
		}
		vals, itemErrs := formValues(f.Fields, form, p+".")
		item := map[string]json.RawMessage{}
		for k, v := range vals {
			if string(v) != "null" && string(v) != "false" {
				item[k] = v
			}
		}
		if len(item) == 0 {
			continue
		}
		for k, v := range vals { // a false checkbox is a value, once the item exists
			if string(v) == "false" {
				item[k] = v
			}
		}
		n := len(items)
		from, to := errKey(p)+".", errKey(name)+"."+strconv.Itoa(n)+"."
		for k, msgs := range itemErrs {
			errs = add(errs, to+strings.TrimPrefix(k, from), msgs[0])
		}
		items = append(items, item)
	}
	return items, errs
}
