package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"redchef/db"
)

// AdminExport streams a full backup as a zip archive: a consistent SQLite
// snapshot (via VACUUM INTO) plus every file under the uploads dir. The
// temp snapshot is removed on success and error paths.
func AdminExport(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		jsonError(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	tmpDB := filepath.Join(os.TempDir(), fmt.Sprintf("redchef_export_%d.db", generateID()))
	defer os.Remove(tmpDB)

	if _, err := db.DB.Exec("VACUUM INTO ?", tmpDB); err != nil {
		jsonError(w, "failed to create DB snapshot: "+err.Error(), http.StatusInternalServerError)
		return
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="redchef-backup-%s.zip"`, stamp))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	zw := zip.NewWriter(w)

	// 1. Database snapshot
	if err := addFileToZip(zw, tmpDB, "redchef.db"); err != nil {
		// Headers are already sent; aborting mid-stream is all we can do.
		zw.Close()
		return
	}

	// 2. Media files as media/<relative-path>
	root := uploadDir
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil // skip unreadable/dirs; best-effort backup
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "_raw_") && !strings.Contains(rel, string(filepath.Separator)) {
			return nil
		}
		name := "media/" + filepath.ToSlash(rel)
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return nil
		}
		hdr.Name = name
		hdr.Method = zip.Deflate
		entry, err := zw.CreateHeader(hdr)
		if err != nil {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		io.Copy(entry, f)
		f.Close()
		return nil
	})

	zw.Close()
}

func addFileToZip(zw *zip.Writer, path, name string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate

	entry, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, f)
	return err
}
