package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"Order-site/imagestore"
)

// maxUploadBytes is the biggest picture the server accepts. The browser shrinks phone
// photos to well under this before uploading, so it only stops oversized or odd files.
const maxUploadBytes = 3 << 20

// allowedImageTypes are the formats a vendor may upload. SVG is left out on purpose,
// because an SVG can carry scripts.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// parseFormOrMultipart reads a normal form or one that carries a file.
func parseFormOrMultipart(w http.ResponseWriter, r *http.Request) error {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+(1<<20))
		return r.ParseMultipartForm(4 << 20)
	}
	return r.ParseForm()
}

// uploadedImageURL stores the picture chosen in the form field (if any) and returns the
// address it is served from. It returns "" with no error when no file was chosen.
func (h *Handlers) uploadedImageURL(r *http.Request, field string) (string, error) {
	file, _, err := r.FormFile(field)
	if err == http.ErrMissingFile {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("We couldn't read that picture. Please try a smaller one")
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		return "", fmt.Errorf("We couldn't read that picture. Please try again")
	}
	if len(data) == 0 {
		return "", nil
	}
	if len(data) > maxUploadBytes {
		return "", fmt.Errorf("That picture is too big. Please choose one under 3 MB")
	}
	contentType := http.DetectContentType(data)
	if !allowedImageTypes[contentType] {
		return "", fmt.Errorf("Please upload a JPG, PNG, WebP or GIF picture")
	}

	// Preferred: keep the picture in Cloudinary, so it does not fill the database and is served
	// from a fast image network. If Cloudinary is not set up, or is down for a moment, fall back
	// to the database so an upload never fails just because of that.
	if imagestore.Configured() {
		cloudURL, cerr := imagestore.Upload(r.Context(), data)
		if cerr == nil {
			return cloudURL, nil
		}
		log.Printf("upload image: Cloudinary failed, keeping it in the database instead: %v", cerr)
	}

	var id int64
	if err := h.DB.QueryRow(
		`INSERT INTO uploaded_images (content_type, data) VALUES ($1, $2) RETURNING id`,
		contentType, data,
	).Scan(&id); err != nil {
		log.Printf("upload image: save failed: %v", err)
		return "", fmt.Errorf("We couldn't save that picture. Please try again")
	}
	return "/media/" + strconv.FormatInt(id, 10), nil
}

// ServeUploadedImage serves a stored picture. Pictures never change once saved (a new
// upload gets a new id), so browsers may keep them for a long time.
func (h *Handlers) ServeUploadedImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	var contentType string
	var data []byte
	var cloudURL sql.NullString
	if err := h.DB.QueryRow(`SELECT content_type, data, cloud_url FROM uploaded_images WHERE id = $1`, id).Scan(&contentType, &data, &cloudURL); err != nil {
		http.NotFound(w, r)
		return
	}
	// A picture that has been moved to Cloudinary keeps its old /media/<id> address working by
	// forwarding to the new one, so nothing that saved the old address needs rewriting.
	if cloudURL.Valid && strings.HasPrefix(cloudURL.String, "https://") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.Redirect(w, r, cloudURL.String, http.StatusMovedPermanently)
		return
	}
	if len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}
