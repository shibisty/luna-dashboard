package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"fullstack"
	auth "fullstack-passport"
	"fullstack-validation"
	"fullstack/web"

	"luna-dashboard/app/filesapi"
	"luna-dashboard/app/locale"
	"luna-dashboard/app/models"
)

// Files serves an application's files, kept by luna-files: a browser of
// folders and files (upload, rename, move, visibility, links, delete) and,
// through api/, the uploads of the browser and the page editor's assets.
type Files struct{ App *fullstack.App }

const filesPerPage = 50

func (h Files) client() (*filesapi.Client, error) { return fullstack.Make[*filesapi.Client](h.App) }

// current is the developer's application of {app} (404 for anyone else's)
// and the luna-files client.
func (h Files) current(c *web.Ctx) (*filesapi.Client, *models.Application, error) {
	_, a, err := Apps(h).current(c)
	if err != nil {
		return nil, nil, err
	}
	cl, err := h.client()
	return cl, a, err
}

func filesURL(a *models.Application) string { return "/apps/" + a.PublicID + "/files" }

// folderURL is the browser at a folder ("" the root).
func folderURL(a *models.Application, folder string) string {
	if folder == "" {
		return filesURL(a)
	}
	return filesURL(a) + "?" + url.Values{"folder": {folder}}.Encode()
}

func fileURL(a *models.Application, id string) string {
	return filesURL(a) + "/file/" + url.PathEscape(id)
}

// unavailable is the page shown when luna-files cannot be asked.
func (h Files) unavailable(c *web.Ctx, a *models.Application, err error) error {
	msg := "luna-files cannot be reached right now. Try again in a moment."
	if errors.Is(err, filesapi.ErrNotConfigured) {
		msg = "luna-files is not connected to this dashboard (LUNA_FILES_URL and LUNA_FILES_TOKEN)."
	}
	h.App.Log.Error("files: luna-files", "error", err)
	return c.HTML(http.StatusBadGateway, "pages/apps/files/unavailable", page(h.App, c, map[string]any{"Application": a, "Message": msg}))
}

// problem is what a refused change says: luna-files' validation errors by
// field, or its message.
func problem(err error) (validation.Errors, string, bool) {
	if errs, ok := validation.ErrorsOf(err); ok {
		return errs, "", true
	}
	var e *filesapi.Error
	if !errors.As(err, &e) || e.Status >= 500 {
		return nil, "", false
	}
	if len(e.Fields) > 0 {
		return validation.Errors(e.Fields), "", true
	}
	switch e.Status {
	case http.StatusConflict:
		return nil, e.Message, true // the name is taken, the folder is not empty
	case http.StatusInsufficientStorage:
		return nil, "The application's storage is full.", true
	case http.StatusRequestEntityTooLarge:
		return nil, "The file is too large.", true
	}
	return nil, e.Message, true
}

// folderOption is a folder in the "move to" lists, by its whole path.
type folderOption struct {
	ID, Label string
	Depth     int
}

// folderOptions are every folder by path, without skip and its subfolders
// (a folder cannot move into itself).
func folderOptions(all []filesapi.Folder, skip string) []folderOption {
	byID := make(map[string]filesapi.Folder, len(all))
	for _, f := range all {
		byID[f.ID] = f
	}
	var out []folderOption
	for _, f := range all {
		names, depth, skipped := []string{}, 0, false
		for cur, ok := f, true; ok; {
			if cur.ID == skip {
				skipped = true
				break
			}
			names = append([]string{cur.Name}, names...)
			depth++
			if cur.ParentID == nil || depth > 64 {
				break
			}
			cur, ok = byID[*cur.ParentID]
		}
		if !skipped {
			out = append(out, folderOption{ID: f.ID, Label: strings.Join(names, " / "), Depth: depth})
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label) })
	return out
}

// Index is the browser: a folder's subfolders and files (or a search in
// every folder), breadcrumbs, the storage used, and the forms.
func (h Files) Index(c *web.Ctx) error { return h.index(c, http.StatusOK, nil, "") }

func (h Files) index(c *web.Ctx, status int, errs validation.Errors, message string) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	if !cl.Configured() {
		return h.unavailable(c, a, filesapi.ErrNotConfigured)
	}
	ctx := c.Context()
	folder := strings.TrimSpace(c.Query("folder"))
	if folder == "" {
		folder = strings.TrimSpace(c.R.PostFormValue("folder_id"))
	}
	q := strings.TrimSpace(c.Query("q"))
	n, _ := strconv.Atoi(c.Query("page"))
	if n < 1 {
		n = 1
	}
	var path []filesapi.Folder
	if folder != "" {
		path, err = cl.Path(ctx, a.PublicID, folder)
		if errors.Is(err, filesapi.ErrNotFound) {
			return web.ErrNotFound
		}
		if err != nil {
			return h.unavailable(c, a, err)
		}
	}
	var folders []filesapi.Folder
	if q == "" {
		if folders, err = cl.Folders(ctx, a.PublicID, folder); err != nil {
			return h.unavailable(c, a, err)
		}
	}
	list, err := cl.Files(ctx, a.PublicID, filesapi.Query{Folder: folder, Search: q, Limit: filesPerPage, Offset: (n - 1) * filesPerPage})
	if err != nil {
		return h.unavailable(c, a, err)
	}
	usage, err := cl.Usage(ctx, a.PublicID)
	if err != nil {
		return h.unavailable(c, a, err)
	}
	all, err := cl.AllFolders(ctx, a.PublicID)
	if err != nil {
		return h.unavailable(c, a, err)
	}
	last := (list.Total + filesPerPage - 1) / filesPerPage
	if last < 1 {
		last = 1
	}
	link := func(p int) string {
		v := url.Values{"page": {strconv.Itoa(p)}}
		if folder != "" {
			v.Set("folder", folder)
		}
		if q != "" {
			v.Set("q", q)
		}
		return filesURL(a) + "?" + v.Encode()
	}
	data := map[string]any{
		"Application": a, "URL": filesURL(a), "Folder": folder, "Path": path, "Query": q,
		"Folders": folders, "Files": list, "Usage": usage, "Page": n, "LastPage": last,
		"Moves": folderOptions(all, ""), "Errors": errs, "Message": message,
		"MaxUpload": cl.MaxUpload(), "CSRFToken": auth.CSRFToken(c),
		"Done": doneNotice(c.Query("done")), "UploadWords": uploadWords(locale.Detect(c.R)),
	}
	if len(path) > 0 {
		data["Current"] = path[len(path)-1]
		data["FolderMoves"] = folderOptions(all, folder)
		if len(path) > 1 {
			data["Up"] = folderURL(a, path[len(path)-2].ID)
		} else {
			data["Up"] = filesURL(a)
		}
	}
	if n > 1 {
		data["PrevURL"] = link(n - 1)
	}
	if n < last {
		data["NextURL"] = link(n + 1)
	}
	return c.HTML(status, "pages/apps/files/index", page(h.App, c, data))
}

// uploadMessages are luna-files' answers the upload script may show.
var uploadMessages = []string{
	"Something of that name is already there.", "The application's storage is full.", "The file is too large.",
	"The uploaded bytes do not match the announced size or SHA-256.", "The given data was invalid.",
	"luna-files cannot be reached right now.", "luna-files is not connected to this dashboard.",
}

// uploadWords are the upload script's words in the page's language, as
// JSON for a <script type="application/json">.
func uploadWords(l *locale.Lang) template.JS {
	msgs := map[string]string{}
	for _, m := range uploadMessages {
		msgs[m] = l.Msg(m)
	}
	b, _ := json.Marshal(map[string]any{
		"done": l.T("uploaded"), "direct": l.T("straight to the storage…"), "checking": l.T("checking…"),
		"network": l.T("The connection failed."), "messages": msgs,
	})
	return template.JS(b)
}

// doneNotice is what a redirect after a change says (?done=).
func doneNotice(done string) string {
	switch done {
	case "uploaded":
		return "Uploaded."
	case "folder":
		return "Folder created."
	case "deleted":
		return "Deleted."
	case "saved":
		return "Saved."
	case "replaced":
		return "The new content is the current version; the old one is in the history."
	case "restored":
		return "Restored: that content is the current version again."
	case "version-deleted":
		return "The version was deleted."
	}
	return ""
}

func back(a *models.Application, folder, done string) string {
	u := folderURL(a, folder)
	if done == "" {
		return u
	}
	if strings.Contains(u, "?") {
		return u + "&done=" + done
	}
	return u + "?done=" + done
}

type folderForm struct {
	Name     string `form:"name" validate:"required,max=255"`
	FolderID string `form:"folder_id"`
}

// CreateFolder makes a folder in the open one.
func (h Files) CreateFolder(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	var in folderForm
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := validation.ErrorsOf(err); ok {
			return h.index(c, http.StatusUnprocessableEntity, errs, "")
		}
		return err
	}
	f, err := cl.CreateFolder(c.Context(), a.PublicID, in.FolderID, strings.TrimSpace(in.Name))
	if errs, msg, ok := problem(err); ok {
		return h.index(c, http.StatusUnprocessableEntity, errs, msg)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, folderURL(a, f.ID))
}

// UpdateFolder renames the open folder and/or moves it (move_to: a folder
// id, "root").
func (h Files) UpdateFolder(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	id := c.Param("folder")
	p := filesapi.FolderPatch{}
	if name := strings.TrimSpace(c.R.PostFormValue("name")); name != "" {
		p.Name = &name
	}
	if to := c.R.PostFormValue("move_to"); to != "" {
		if to == "root" {
			to = ""
		}
		p.ParentID = &to
	}
	_, err = cl.UpdateFolder(c.Context(), a.PublicID, id, p)
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if errs, msg, ok := problem(err); ok {
		c.R.PostForm.Set("folder_id", id) // back to the folder
		return h.index(c, http.StatusUnprocessableEntity, errs, msg)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, back(a, id, "saved"))
}

// DeleteFolder deletes a folder: an empty one, or with recursive=1
// everything in it.
func (h Files) DeleteFolder(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	id := c.Param("folder")
	path, err := cl.Path(c.Context(), a.PublicID, id)
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	_, err = cl.DeleteFolder(c.Context(), a.PublicID, id, c.R.PostFormValue("recursive") == "1")
	if errs, msg, ok := problem(err); ok {
		if msg == "The folder is not empty." {
			msg = "The folder is not empty: tick “with everything in it” to delete it all."
		}
		c.R.PostForm.Set("folder_id", id)
		return h.index(c, http.StatusUnprocessableEntity, errs, msg)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	parent := ""
	if len(path) > 1 {
		parent = path[len(path)-2].ID
	}
	return c.Redirect(http.StatusSeeOther, back(a, parent, "deleted"))
}

// Upload stores the files of the upload form in the open folder (the
// form's way without JavaScript; the page's script sends each file through
// api/ instead, large ones straight to the store). Names that are taken get
// a free one.
func (h Files) Upload(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	if c.R.MultipartForm == nil {
		if err := c.R.ParseMultipartForm(32 << 20); err != nil {
			return h.index(c, http.StatusUnprocessableEntity, validation.Errors{"file": {"Choose a file."}}, "")
		}
	}
	folder := c.R.PostFormValue("folder_id")
	vis := c.R.PostFormValue("visibility")
	var parts []*multipart.FileHeader
	if c.R.MultipartForm != nil {
		parts = c.R.MultipartForm.File["file"]
	}
	if len(parts) == 0 {
		return h.index(c, http.StatusUnprocessableEntity, validation.Errors{"file": {"Choose a file."}}, "")
	}
	for _, p := range parts {
		if p.Size > cl.MaxUpload() {
			return h.index(c, http.StatusRequestEntityTooLarge, nil,
				fmt.Sprintf("%s is larger than %s: without JavaScript files go through the dashboard, which takes up to that size.", p.Filename, sizeText(cl.MaxUpload(), locale.Detect(c.R))))
		}
	}
	for _, p := range parts {
		f, err := p.Open()
		if err != nil {
			return err
		}
		_, err = cl.Upload(c.Context(), a.PublicID, filesapi.Upload{
			Folder: folder, Name: p.Filename, Mime: p.Header.Get("Content-Type"), Visibility: vis, Rename: true,
		}, f)
		f.Close()
		if errs, msg, ok := problem(err); ok {
			return h.index(c, http.StatusUnprocessableEntity, errs, msg)
		}
		if err != nil {
			return h.unavailable(c, a, err)
		}
	}
	return c.Redirect(http.StatusSeeOther, back(a, folder, "uploaded"))
}

// Show is a file: a preview, its details and link, and the forms.
func (h Files) Show(c *web.Ctx) error { return h.show(c, http.StatusOK, nil, "") }

func (h Files) show(c *web.Ctx, status int, errs validation.Errors, message string) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	ctx := c.Context()
	f, err := cl.File(ctx, a.PublicID, c.Param("file"))
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	var path []filesapi.Folder
	if f.FolderID != nil {
		if path, err = cl.Path(ctx, a.PublicID, *f.FolderID); err != nil {
			return h.unavailable(c, a, err)
		}
	}
	all, err := cl.AllFolders(ctx, a.PublicID)
	if err != nil {
		return h.unavailable(c, a, err)
	}
	versions, err := cl.Versions(ctx, a.PublicID, f.ID)
	if err != nil {
		return h.unavailable(c, a, err)
	}
	data := map[string]any{
		"Application": a, "URL": filesURL(a), "File": f, "Path": path,
		"Moves": folderOptions(all, ""), "Errors": errs, "Message": message,
		"Self": fileURL(a, f.ID), "Done": doneNotice(c.Query("done")),
		"Versions": versions, "MaxUpload": cl.MaxUpload(), "CSRFToken": auth.CSRFToken(c),
		"API": filesURL(a) + "/api", "UploadWords": uploadWords(locale.Detect(c.R)),
	}
	folder := ""
	if f.FolderID != nil {
		folder = *f.FolderID
	}
	data["Back"] = folderURL(a, folder)
	if f.Status == "ready" {
		link, err := cl.URL(ctx, a.PublicID, f.ID, false)
		if err != nil {
			return h.unavailable(c, a, err)
		}
		data["Link"] = link
	}
	c.W.Header().Set("Cache-Control", "no-store") // the signed link
	return c.HTML(status, "pages/apps/files/show", page(h.App, c, data))
}

// Update renames, moves and/or changes the visibility of a file.
func (h Files) Update(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	p := filesapi.FilePatch{}
	if name := strings.TrimSpace(c.R.PostFormValue("name")); name != "" {
		p.Name = &name
	}
	if to := c.R.PostFormValue("move_to"); to != "" {
		if to == "root" {
			to = ""
		}
		p.FolderID = &to
	}
	if v := c.R.PostFormValue("visibility"); v != "" {
		p.Visibility = &v
	}
	f, err := cl.UpdateFile(c.Context(), a.PublicID, c.Param("file"), p)
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if errs, msg, ok := problem(err); ok {
		return h.show(c, http.StatusUnprocessableEntity, errs, msg)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, fileURL(a, f.ID)+"?done=saved")
}

// Delete deletes a file and goes back to its folder.
func (h Files) Delete(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	ctx := c.Context()
	f, err := cl.File(ctx, a.PublicID, c.Param("file"))
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	if err := cl.DeleteFile(ctx, a.PublicID, f.ID); err != nil && !errors.Is(err, filesapi.ErrNotFound) {
		return h.unavailable(c, a, err)
	}
	folder := ""
	if f.FolderID != nil {
		folder = *f.FolderID
	}
	return c.Redirect(http.StatusSeeOther, back(a, folder, "deleted"))
}

// Replace uploads a new content for a file through the dashboard (the
// form's way without JavaScript; the page's script sends it through api/
// with replace_id, large files straight to the store).
func (h Files) Replace(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	if c.R.MultipartForm == nil {
		_ = c.R.ParseMultipartForm(32 << 20)
	}
	var part *multipart.FileHeader
	if c.R.MultipartForm != nil && len(c.R.MultipartForm.File["file"]) > 0 {
		part = c.R.MultipartForm.File["file"][0]
	}
	if part == nil {
		return h.show(c, http.StatusUnprocessableEntity, validation.Errors{"file": {"Choose a file."}}, "")
	}
	if part.Size > cl.MaxUpload() {
		return h.show(c, http.StatusRequestEntityTooLarge, nil,
			fmt.Sprintf("%s is larger than %s: without JavaScript files go through the dashboard, which takes up to that size.", part.Filename, sizeText(cl.MaxUpload(), locale.Detect(c.R))))
	}
	body, err := part.Open()
	if err != nil {
		return err
	}
	defer body.Close()
	id := c.Param("file")
	_, err = cl.Upload(c.Context(), a.PublicID, filesapi.Upload{Name: part.Filename, Mime: part.Header.Get("Content-Type"), Replace: id}, body)
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if errs, msg, ok := problem(err); ok {
		return h.show(c, http.StatusUnprocessableEntity, errs, msg)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, fileURL(a, id)+"?done=replaced")
}

func versionParam(c *web.Ctx) (int, error) {
	n, err := strconv.Atoi(c.Param("n"))
	if err != nil || n < 1 {
		return 0, web.ErrNotFound
	}
	return n, nil
}

// OpenVersion sends the developer to an old version (a signed link;
// download=1 as an attachment).
func (h Files) OpenVersion(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	n, err := versionParam(c)
	if err != nil {
		return err
	}
	link, err := cl.VersionURL(c.Context(), a.PublicID, c.Param("file"), n, c.Query("download") == "1")
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	c.W.Header().Set("Cache-Control", "no-store")
	c.W.Header().Set("Referrer-Policy", "no-referrer")
	return c.Redirect(http.StatusSeeOther, link.URL)
}

// RestoreVersion makes an old version the content again.
func (h Files) RestoreVersion(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	n, err := versionParam(c)
	if err != nil {
		return err
	}
	id := c.Param("file")
	_, err = cl.Restore(c.Context(), a.PublicID, id, n)
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if errs, msg, ok := problem(err); ok {
		return h.show(c, http.StatusUnprocessableEntity, errs, msg)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, fileURL(a, id)+"?done=restored")
}

// DeleteVersion removes an old version.
func (h Files) DeleteVersion(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	n, err := versionParam(c)
	if err != nil {
		return err
	}
	id := c.Param("file")
	err = cl.DeleteVersion(c.Context(), a.PublicID, id, n)
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, fileURL(a, id)+"?done=version-deleted")
}

// Open sends the developer to the file: its public URL or a fresh signed
// one (download=1: as an attachment).
func (h Files) Open(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	link, err := cl.URL(c.Context(), a.PublicID, c.Param("file"), c.Query("download") == "1")
	if errors.Is(err, filesapi.ErrNotFound) {
		return web.ErrNotFound
	}
	if _, msg, ok := problem(err); ok {
		return web.Error(http.StatusConflict, msg)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	c.W.Header().Set("Cache-Control", "no-store")
	c.W.Header().Set("Referrer-Policy", "no-referrer")
	return c.Redirect(http.StatusSeeOther, link.URL)
}

// API forwards a request of the page's script or the page editor to
// luna-files' internal API for the application (see filesapi.Allowed). The
// developer's session and CSRF header are checked here; luna-files gets
// the service token.
func (h Files) API(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	path := c.Param("path")
	if !filesapi.Allowed(c.R.Method, path) {
		return web.ErrNotFound
	}
	if err := cl.Forward(c.W, c.R, a.PublicID, path); err != nil {
		var tooBig *http.MaxBytesError
		switch {
		case errors.As(err, &tooBig):
			return web.Error(http.StatusRequestEntityTooLarge, "The file is too large.")
		case c.Written():
			h.App.Log.Error("files: forwarding", "error", err)
			return nil
		case errors.Is(err, filesapi.ErrNotConfigured):
			return web.Error(http.StatusBadGateway, "luna-files is not connected to this dashboard.")
		default:
			h.App.Log.Error("files: forwarding", "error", err)
			return web.Error(http.StatusBadGateway, "luna-files cannot be reached right now.")
		}
	}
	return nil
}

// sizeText is n bytes for people: 512 B, 1.5 MB (units of 1024).
func sizeText(n int64, l *locale.Lang) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v, i := float64(n), 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	unit := units[i]
	if l != nil {
		unit = l.T(unit)
	}
	switch {
	case i == 0:
		return fmt.Sprintf("%d %s", n, unit)
	case v < 10 && v != float64(int64(v)):
		out := fmt.Sprintf("%.1f %s", v, unit)
		if l != nil && l.Code() == "ru" {
			out = strings.Replace(out, ".", ",", 1)
		}
		return out
	}
	return fmt.Sprintf("%.0f %s", v, unit)
}

// SizeText is sizeText for the templates ({{size .Size $.L}}).
func SizeText(n int64, l *locale.Lang) string { return sizeText(n, l) }
