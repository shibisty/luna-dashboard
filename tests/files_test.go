package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	apptest "fullstack-testing"
)

const filesToken = "files-internal-token"

// fakeFiles is luna-files' internal API over memory, for one application
// at a time: folders, files with their bytes, usage, links.
type fakeFiles struct {
	mu      sync.Mutex
	n       int
	folders map[string]map[string]any
	files   map[string]map[string]any
	bytes   map[string][]byte
	history map[string][]map[string]any
	calls   []fakeCall
}

func newFakeFiles(t *testing.T) *fakeFiles {
	f := &fakeFiles{folders: map[string]map[string]any{}, files: map[string]map[string]any{}, bytes: map[string][]byte{}, history: map[string][]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	t.Setenv("LUNA_FILES_URL", srv.URL)
	t.Setenv("LUNA_FILES_TOKEN", filesToken)
	t.Setenv("LUNA_FILES_MAX_UPLOAD", "1024")
	return f
}

func (f *fakeFiles) id() string {
	f.n++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.n)
}

func (f *fakeFiles) last() fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[len(f.calls)-1]
}

func (f *fakeFiles) byName(name string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.files {
		if v["name"] == name {
			return v
		}
	}
	for _, v := range f.folders {
		if v["name"] == name {
			return v
		}
	}
	return nil
}

func strOrNil(v any) any {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return nil
}

func (f *fakeFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body []byte
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		body, _ = io.ReadAll(r.Body)
	}
	f.calls = append(f.calls, fakeCall{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Cookie: r.Header.Get("Cookie"), CSRF: r.Header.Get("X-CSRF-Token"), Body: string(body)})
	if r.Header.Get("Authorization") != "Bearer "+filesToken {
		reply(w, 401, map[string]string{"error": "missing or invalid service token"})
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/internal/v1/apps/")
	parts := strings.Split(rest, "/")
	if !ok || len(parts) < 2 {
		reply(w, 404, map[string]string{"error": "Not found."})
		return
	}
	parts = parts[1:]
	now := time.Now().UTC().Format(time.RFC3339)
	var in map[string]any
	_ = json.Unmarshal(body, &in)
	notFound := func() { reply(w, 404, map[string]string{"error": "Not found."}) }
	taken := func(parent any, name string) bool {
		for _, v := range f.folders {
			if v["parent_id"] == parent && strings.EqualFold(v["name"].(string), name) {
				return true
			}
		}
		return false
	}
	switch {
	case parts[0] == "usage":
		var used int64
		for _, v := range f.files {
			used += v["size"].(int64)
		}
		reply(w, 200, map[string]any{"used": used, "limit": 1 << 30, "files": len(f.files)})
	case parts[0] == "folders" && len(parts) == 1 && r.Method == "GET":
		list := []map[string]any{}
		parent := strOrNil(r.URL.Query().Get("parent_id"))
		for _, v := range f.folders {
			if r.URL.Query().Get("all") == "1" || v["parent_id"] == parent {
				list = append(list, v)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i]["name"].(string) < list[j]["name"].(string) })
		reply(w, 200, map[string]any{"data": list})
	case parts[0] == "folders" && len(parts) == 1 && r.Method == "POST":
		name, _ := in["name"].(string)
		if taken(in["parent_id"], name) {
			reply(w, 409, map[string]string{"error": "Something of that name is already there."})
			return
		}
		v := map[string]any{"id": f.id(), "parent_id": in["parent_id"], "name": name, "created_at": now, "updated_at": now}
		f.folders[v["id"].(string)] = v
		reply(w, 201, v)
	case parts[0] == "folders" && len(parts) == 2:
		v, ok := f.folders[parts[1]]
		if !ok {
			notFound()
			return
		}
		switch r.Method {
		case "GET":
			path := []map[string]any{v}
			for p := v["parent_id"]; p != nil; p = f.folders[p.(string)]["parent_id"] {
				path = append([]map[string]any{f.folders[p.(string)]}, path...)
			}
			reply(w, 200, map[string]any{"folder": v, "path": path})
		case "PATCH":
			if n, ok := in["name"].(string); ok {
				v["name"] = n
			}
			if p, ok := in["parent_id"]; ok {
				v["parent_id"] = p
			}
			reply(w, 200, v)
		case "DELETE":
			for _, x := range f.files {
				if x["folder_id"] == v["id"] && r.URL.Query().Get("recursive") != "1" {
					reply(w, 409, map[string]string{"error": "The folder is not empty."})
					return
				}
			}
			n := 0
			for k, x := range f.files {
				if x["folder_id"] == v["id"] {
					delete(f.files, k)
					n++
				}
			}
			delete(f.folders, parts[1])
			reply(w, 200, map[string]any{"deleted_files": n})
		}
	case parts[0] == "files" && len(parts) == 1 && r.Method == "GET":
		list := []map[string]any{}
		folder := strOrNil(r.URL.Query().Get("folder_id"))
		q := r.URL.Query().Get("q")
		for _, v := range f.files {
			if (q != "" && strings.Contains(v["name"].(string), q) && folder == nil) || (q == "" && v["folder_id"] == folder) {
				if r.URL.Query().Get("images") == "1" && !strings.HasPrefix(v["mime"].(string), "image/") {
					continue
				}
				list = append(list, v)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i]["name"].(string) < list[j]["name"].(string) })
		reply(w, 200, map[string]any{"data": list, "total": len(list), "limit": 50, "offset": 0})
	case parts[0] == "files" && len(parts) == 1 && r.Method == "POST":
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		fields := map[string]string{}
		for {
			p, err := mr.NextPart()
			if err != nil {
				reply(w, 422, map[string]any{"error": "The given data was invalid.", "errors": map[string][]string{"file": {"Send the file in the part \"file\"."}}})
				return
			}
			if p.FormName() != "file" {
				b, _ := io.ReadAll(p)
				fields[p.FormName()] = string(b)
				continue
			}
			data, _ := io.ReadAll(p)
			if r := fields["replace_id"]; r != "" {
				v, ok := f.files[r]
				if !ok {
					notFound()
					return
				}
				old := map[string]any{"number": v["version"], "mime": v["mime"], "size": v["size"], "content_at": now, "archived_at": now}
				f.history[r] = append([]map[string]any{old}, f.history[r]...)
				v["version"] = v["version"].(int) + 1
				v["size"], v["mime"] = int64(len(data)), p.Header.Get("Content-Type")
				f.bytes[r] = data
				reply(w, 200, v)
				return
			}
			name := fields["name"]
			if name == "" {
				name = p.FileName()
			}
			vis := fields["visibility"]
			if vis == "" {
				vis = "public"
			}
			v := map[string]any{"id": f.id(), "folder_id": strOrNil(fields["folder_id"]), "name": name, "mime": p.Header.Get("Content-Type"),
				"size": int64(len(data)), "visibility": vis, "status": "ready", "created_at": now, "updated_at": now, "rename": fields["rename"],
				"version": 1, "content_at": now}
			if vis == "public" {
				v["url"] = "https://cdn.test/apps/x/" + v["id"].(string)
			}
			f.files[v["id"].(string)] = v
			f.bytes[v["id"].(string)] = data
			reply(w, 201, v)
			return
		}
	case parts[0] == "files" && len(parts) >= 2:
		v, ok := f.files[parts[1]]
		if !ok {
			notFound()
			return
		}
		switch {
		case len(parts) == 3 && parts[2] == "versions":
			list := f.history[parts[1]]
			if list == nil {
				list = []map[string]any{}
			}
			reply(w, 200, map[string]any{"data": list})
		case len(parts) >= 4 && parts[2] == "versions":
			var hit map[string]any
			for _, x := range f.history[parts[1]] {
				if fmt.Sprint(x["number"]) == parts[3] {
					hit = x
				}
			}
			if hit == nil {
				notFound()
				return
			}
			switch {
			case len(parts) == 5 && parts[4] == "url":
				reply(w, 200, map[string]any{"url": "https://bucket.test/v/" + parts[3] + "?" + r.URL.RawQuery, "expires_at": "2026-11-13T12:15:00Z"})
			case len(parts) == 5 && parts[4] == "restore":
				v["version"] = v["version"].(int) + 1
				v["size"] = hit["size"]
				reply(w, 200, v)
			case r.Method == "DELETE":
				var keep []map[string]any
				for _, x := range f.history[parts[1]] {
					if x["number"] != hit["number"] {
						keep = append(keep, x)
					}
				}
				f.history[parts[1]] = keep
				w.WriteHeader(204)
			}
		case len(parts) == 3 && parts[2] == "url":
			if v["visibility"] == "public" {
				reply(w, 200, map[string]any{"url": v["url"]})
				return
			}
			reply(w, 200, map[string]any{"url": "https://bucket.test/apps/x/" + v["id"].(string) + "?sig=1&" + r.URL.RawQuery, "expires_at": "2026-11-13T12:15:00Z"})
		case len(parts) == 3 && parts[2] == "complete":
			reply(w, 200, v)
		case r.Method == "GET":
			reply(w, 200, v)
		case r.Method == "PATCH":
			if n, ok := in["name"].(string); ok {
				v["name"] = n
			}
			if p, ok := in["folder_id"]; ok {
				v["folder_id"] = p
			}
			if vis, ok := in["visibility"].(string); ok {
				v["visibility"] = vis
				if vis == "public" {
					v["url"] = "https://cdn.test/apps/x/" + v["id"].(string)
				} else {
					delete(v, "url")
				}
			}
			reply(w, 200, v)
		case r.Method == "DELETE":
			delete(f.files, parts[1])
			w.WriteHeader(204)
		}
	case parts[0] == "uploads":
		reply(w, 201, map[string]any{"file": map[string]any{"id": "u1"}, "upload": map[string]any{"method": "PUT", "url": "https://bucket.test/x", "headers": map[string]string{}}})
	default:
		notFound()
	}
}

// multipartBody is an upload form: fields, then files by name.
func multipartBody(fields map[string]string, files map[string][]byte) (*bytes.Buffer, string) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="`+n+`"`)
		ct := "application/octet-stream"
		if strings.HasSuffix(n, ".png") {
			ct = "image/png"
		}
		h.Set("Content-Type", ct)
		p, _ := mw.CreatePart(h)
		_, _ = p.Write(files[n])
	}
	_ = mw.Close()
	return &b, mw.FormDataContentType()
}

func TestFileBrowser(t *testing.T) {
	fs := newFakeFiles(t)
	app := newApp(t)
	c := developer(t, apptest.New(t, app), "ann@example.com")
	id := createApp(t, c, "Shop")
	base := "/apps/" + id + "/files"

	c.Get("/apps/"+id).AssertOK().AssertSee(base)
	c.Get(base).AssertOK().AssertSee("No files yet.", "0 B of 1 GB used", `data-max="1024"`, `id="upload-words"`)

	// A folder; its page has breadcrumbs and its own forms.
	c.Post(base+"/folders", map[string][]string{"name": {"Images"}}).AssertRedirect()
	img := fs.byName("Images")["id"].(string)
	c.Get(base+"?folder="+img).AssertOK().AssertSee("This folder is empty.", "This folder", `action="`+base+`/folders/`+img+`/delete"`, "All files")
	c.Post(base+"/folders", map[string][]string{"name": {"images"}}).AssertUnprocessable().AssertSee("Something of that name is already there.")

	// Uploads through the dashboard go to luna-files as multipart, a free
	// name asked for; larger files are refused without JavaScript.
	body, ct := multipartBody(map[string]string{"folder_id": img, "visibility": "public"}, map[string][]byte{"logo.png": []byte("\x89PNG fake"), "notes.txt": []byte("hello")})
	c.Do("POST", base+"/upload", body, ct).AssertRedirect(base + "?folder=" + img + "&done=uploaded")
	logo := fs.byName("logo.png")
	if logo == nil || logo["folder_id"] != img || logo["rename"] != "1" || logo["mime"] != "image/png" || string(fs.bytes[logo["id"].(string)]) != "\x89PNG fake" {
		t.Fatalf("uploaded: %v", logo)
	}
	body, ct = multipartBody(map[string]string{"folder_id": img}, map[string][]byte{"big.bin": bytes.Repeat([]byte("x"), 2048)})
	c.Do("POST", base+"/upload", body, ct).AssertStatus(http.StatusRequestEntityTooLarge).AssertSee("big.bin is larger than 1 KB")
	c.Get(base+"?folder="+img+"&done=uploaded").AssertOK().AssertSee("Uploaded.", "logo.png", "notes.txt", `src="https://cdn.test/apps/x/`, "2 files")

	// A file: the link, then private (a signed link), renamed, moved.
	fileURL := base + "/file/" + logo["id"].(string)
	c.Get(fileURL).AssertOK().AssertSee("https://cdn.test/apps/x/", "Anyone with this link").AssertHeader("Cache-Control", "no-store")
	c.Post(fileURL, map[string][]string{"name": {"brand.png"}, "visibility": {"private"}, "move_to": {"root"}}).AssertRedirect(fileURL + "?done=saved")
	if logo["name"] != "brand.png" || logo["visibility"] != "private" || logo["folder_id"] != nil {
		t.Fatalf("changed: %v", logo)
	}
	c.Get(fileURL+"?done=saved").AssertOK().AssertSee("https://bucket.test/apps/x/", "works until", "Saved.")
	r := c.Get(fileURL + "/open?download=1").AssertRedirect()
	if loc := r.Header().Get("Location"); !strings.HasPrefix(loc, "https://bucket.test/") || !strings.Contains(loc, "download=1") {
		t.Fatalf("open: %s", loc)
	}

	// A folder that is not empty needs "with everything in it".
	c.Post(base+"/folders/"+img+"/delete", nil).AssertUnprocessable().AssertSee("tick")
	c.Post(base+"/folders/"+img+"/delete", map[string][]string{"recursive": {"1"}}).AssertRedirect(base + "?done=deleted")
	c.Post(fileURL+"/delete", nil).AssertRedirect(base + "?done=deleted")
	if len(fs.files) != 0 || len(fs.folders) != 0 {
		t.Fatalf("left: %v %v", fs.files, fs.folders)
	}

	// Search everywhere.
	body, ct = multipartBody(nil, map[string][]byte{"cat.png": []byte("c")})
	c.Do("POST", base+"/upload", body, ct).AssertRedirect()
	c.Get(base+"?q=cat").AssertOK().AssertSee("cat.png").AssertDontSee("New folder")
	c.Get(base+"?q=dog").AssertOK().AssertSee("No files match")

	// Another developer sees none of it.
	other := developer(t, apptest.New(t, app), "bob@example.com")
	other.Get(base).AssertNotFound()
	other.Get(base + "/file/" + fs.byName("cat.png")["id"].(string)).AssertNotFound()
}

func TestFilesAPIForwarding(t *testing.T) {
	fs := newFakeFiles(t)
	newFakePages(t)
	app := newApp(t)
	c := developer(t, apptest.New(t, app), "ann@example.com")
	id := createApp(t, c, "Shop")
	api := "/apps/" + id + "/files/api"

	// The script's multipart upload streams through, with the service
	// token and without the developer's cookies.
	body, ct := multipartBody(map[string]string{"visibility": "public", "rename": "1"}, map[string][]byte{"a.png": []byte("png")})
	c.Do("POST", api+"/files", body, ct).AssertCreated().AssertJSONPath("name", "a.png").AssertHeader("Cache-Control", "no-store")
	if last := fs.last(); last.Cookie != "" || last.CSRF != "" || last.Path != "/internal/v1/apps/"+id+"/files" {
		t.Fatalf("forwarded: %+v", last)
	}
	c.GetJSON(api + "/files?images=1&all=1").AssertOK().AssertJSONCount("data", 1)
	c.PostJSON(api+"/uploads", map[string]any{"name": "big.mov", "size": 5000}).AssertCreated().AssertJSONPath("upload.method", "PUT")
	c.PostJSON(api+"/files/"+fs.byName("a.png")["id"].(string)+"/complete", map[string]any{}).AssertOK()

	// Only the browser's paths; nothing without the CSRF header.
	c.PutJSON(api+"/quota", map[string]any{"limit": "5GiB"}).AssertNotFound()
	c.Do("DELETE", api+"/files/"+fs.byName("a.png")["id"].(string), nil, "").AssertNotFound()
	c.PostJSON(api+"/folders", map[string]any{"name": "x"}).AssertNotFound()
	c.WithoutCSRF().PostJSON(api+"/uploads", map[string]any{"name": "x", "size": 1}).AssertForbidden()
	developer(t, apptest.New(t, app), "bob@example.com").GetJSON(api + "/files").AssertNotFound()

	// The page editor's asset manager uses it.
	c.Get("/apps/"+id+"/pages").AssertOK().AssertSee(`"files":{"api":"`+api+`"`, `"max":1024`, "uploadFile: uploadFiles")
}

func TestFilesWithoutLunaFiles(t *testing.T) {
	t.Setenv("LUNA_FILES_URL", "")
	newFakePages(t)
	app := newApp(t)
	c := developer(t, apptest.New(t, app), "ann@example.com")
	id := createApp(t, c, "Shop")
	c.Get("/apps/"+id+"/files").AssertStatus(http.StatusBadGateway).AssertSee("LUNA_FILES_URL")
	c.GetJSON("/apps/"+id+"/files/api/files").AssertStatus(http.StatusBadGateway)
	c.Get("/apps/"+id+"/pages").AssertOK().AssertSee(`"files":null`)
}

func TestFileVersions(t *testing.T) {
	fs := newFakeFiles(t)
	app := newApp(t)
	c := developer(t, apptest.New(t, app), "ann@example.com")
	id := createApp(t, c, "Shop")
	base := "/apps/" + id + "/files"
	body, ct := multipartBody(map[string]string{"visibility": "public"}, map[string][]byte{"notes.txt": []byte("one")})
	c.Do("POST", base+"/upload", body, ct).AssertRedirect()
	f := fs.byName("notes.txt")
	fileURL := base + "/file/" + f["id"].(string)
	c.Get(fileURL).AssertOK().AssertSee("No old versions yet", `action="`+fileURL+`/replace"`, `data-file="`+f["id"].(string)+`"`)

	// A new content through the dashboard (no JavaScript): replace_id.
	body, ct = multipartBody(nil, map[string][]byte{"notes-v2.txt": []byte("two!")})
	c.Do("POST", fileURL+"/replace", body, ct).AssertRedirect(fileURL + "?done=replaced")
	if f["version"] != 2 || string(fs.bytes[f["id"].(string)]) != "two!" {
		t.Fatalf("replaced: %v", f)
	}
	c.Get(fileURL+"?done=replaced").AssertOK().AssertSee("the old one is in the history", fileURL+"/versions/1/open", fileURL+"/versions/1/restore")

	// An old version: a signed link, restore, delete.
	r := c.Get(fileURL + "/versions/1/open?download=1").AssertRedirect()
	if loc := r.Header().Get("Location"); !strings.HasPrefix(loc, "https://bucket.test/v/1") || !strings.Contains(loc, "download=1") {
		t.Fatal(loc)
	}
	c.Post(fileURL+"/versions/1/restore", nil).AssertRedirect(fileURL + "?done=restored")
	if f["version"] != 3 {
		t.Fatal(f)
	}
	c.Post(fileURL+"/versions/1/delete", nil).AssertRedirect(fileURL + "?done=version-deleted")
	c.Post(fileURL+"/versions/1/delete", nil).AssertNotFound()
	c.Get(fileURL + "/versions/x/open").AssertNotFound()

	// Another developer: nothing.
	developer(t, apptest.New(t, app), "bob@example.com").Post(fileURL+"/versions/2/restore", nil).AssertNotFound()
}
