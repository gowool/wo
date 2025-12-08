package wo

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

// Attachment sends a response as attachment, prompting client to save the file.
func (e *Event) Attachment(fsys fs.FS, file, name string) error {
	return e.contentDisposition(fsys, file, name, "attachment")
}

// Inline sends a response as inline, opening the file in the browser.
func (e *Event) Inline(fsys fs.FS, file, name string) error {
	return e.contentDisposition(fsys, file, name, "inline")
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func (e *Event) contentDisposition(fsys fs.FS, file, name, dispositionType string) error {
	e.response.Header().Set(HeaderContentDisposition, fmt.Sprintf(`%s; filename="%s"`, dispositionType, quoteEscaper.Replace(name)))
	return e.FileFS(fsys, file)
}

// FileFS serves the specified filename from fsys.
func (e *Event) FileFS(fsys fs.FS, filename string) error {
	f, err := fsys.Open(filename)
	if err != nil {
		return ErrNotFound.WithInternal(err)
	}
	defer func() {
		_ = f.Close()
	}()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	// if it is a directory try to open its index.html file
	if fi.IsDir() {
		filename = filepath.ToSlash(filepath.Join(filename, IndexPage))
		f, err = fsys.Open(filename)
		if err != nil {
			return ErrNotFound.WithInternal(err)
		}
		defer func() {
			_ = f.Close()
		}()

		fi, err = f.Stat()
		if err != nil {
			return err
		}
	}

	ff, ok := f.(io.ReadSeeker)
	if !ok {
		return errors.New("file does not implement io.ReadSeeker")
	}

	SetHeaderIfMissing(e.response, HeaderContentSecurityPolicy, "default-src 'none'; connect-src 'self'; image-src 'self'; media-src 'self'; style-src 'unsafe-inline'; sandbox")
	SetHeaderIfMissing(e.response, HeaderCacheControl, "max-age=2592000, stale-while-revalidate=86400")
	SetHeaderIfMissing(e.response, HeaderXRobotsTag, "noindex")

	http.ServeContent(e.response, e.request, fi.Name(), fi.ModTime(), ff)
	return nil
}

// StaticFS serve static directory content from fsys.
//
// If a file resource is missing and indexFallback is set, the request
// will be forwarded to the base index.html (useful for SPA with pretty urls).
//
// NB! Expects the route to have a "{path...}" wildcard parameter.
//
// Special redirects:
//   - if "path" is a file that ends in index.html, it is redirected to its non-index.html version (eg. /test/index.html -> /test/)
//   - if "path" is a directory that has index.html, the index.html file is rendered,
//     otherwise if missing - returns 404 or fallback to the root index.html if indexFallback is set
//
// Example:
//
//	fsys := os.DirFS("./public")
//	router.GET("/files/{path...}", StaticFS[*Event](fsys, false))
func (e *Event) StaticFS(fsys fs.FS, indexFallback bool) error {
	filename := e.Param(StaticWildcardParam)
	filename = filepath.ToSlash(filepath.Clean(strings.TrimPrefix(filename, "/")))

	// eagerly check for directory traversal
	//
	// note: this is just out of an abundance of caution because the fs.FS implementation could be non-std,
	// but usually shouldn't be necessary since os.DirFS.Open is expected to fail if the filename starts with dots
	if len(filename) > 2 && filename[0] == '.' && filename[1] == '.' && (filename[2] == '/' || filename[2] == '\\') {
		if indexFallback && filename != IndexPage {
			return e.FileFS(fsys, IndexPage)
		}
		return ErrNotFound.WithMessage("file not found")
	}

	fi, err := fs.Stat(fsys, filename)
	if err != nil {
		if indexFallback && filename != IndexPage {
			return e.FileFS(fsys, IndexPage)
		}
		return ErrNotFound.WithInternal(err)
	}

	if fi.IsDir() {
		// redirect to a canonical dir url, aka. with trailing slash
		if !strings.HasSuffix(e.Request().URL.Path, "/") {
			return e.Redirect(http.StatusMovedPermanently, safeRedirectPath(e.Request().URL.Path+"/"))
		}
	} else {
		urlPath := e.Request().URL.Path
		if strings.HasSuffix(urlPath, "/") {
			// redirect to a non-trailing slash file route
			urlPath = strings.TrimRight(urlPath, "/")
			if len(urlPath) > 0 {
				return e.Redirect(http.StatusMovedPermanently, safeRedirectPath(urlPath))
			}
		} else if stripped, ok := strings.CutSuffix(urlPath, IndexPage); ok {
			// redirect without the index.html
			return e.Redirect(http.StatusMovedPermanently, safeRedirectPath(stripped))
		}
	}

	fileErr := e.FileFS(fsys, filename)

	if fileErr != nil && indexFallback && filename != IndexPage && errors.Is(fileErr, fs.ErrNotExist) {
		return e.FileFS(fsys, IndexPage)
	}

	return fileErr
}

// safeRedirectPath normalizes the path string by replacing all beginning slashes
// (`\\`, `//`, `\/`) with a single forward slash to prevent open redirect attacks
func safeRedirectPath(path string) string {
	if len(path) > 1 && (path[0] == '\\' || path[0] == '/') && (path[1] == '\\' || path[1] == '/') {
		path = "/" + strings.TrimLeft(path, `/\`)
	}
	return path
}
