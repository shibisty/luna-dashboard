// Package resources embeds the views and the page editor's bundle into the binary.
package resources

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Views are the templates of resources/views.
//
//go:embed views
var embedded embed.FS

// Views is the embedded resources directory.
var Views fs.FS = embedded

// editor is the page editor's bundle (GrapesJS and its plugins, see
// editor/README.md).
//
//go:embed editor/*.js editor/*.css editor/*.woff2 editor/*.woff editor/manifest.json
var editor embed.FS

// Editor holds the editor's files by name (grapes.min.js, manifest.json…).
var Editor fs.FS = sub(editor, "editor")

func sub(f fs.FS, dir string) fs.FS {
	s, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return s
}

// dir is this package's directory in the source tree.
func dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

// OnDisk reports whether the source of the views is here (go run in the
// project), so that development can read them from disk and see edits
// without restarting.
func OnDisk() bool {
	_, err := os.Stat(filepath.Join(dir(), "views"))
	return err == nil
}

// Disk is the resources directory on disk.
func Disk() fs.FS { return os.DirFS(dir()) }
