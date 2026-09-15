package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"Order-site/middleware"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

func (h *Handlers) EditProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.showEditProfile(w, r)
		return
	}

	if r.Method == http.MethodPost {
		h.updateProfile(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *Handlers) showEditProfile(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var name, email, profilePicture string

	err = h.DB.QueryRow(`
		SELECT name, email, COALESCE(profile_picture, '')
		FROM users
		WHERE id = $1
	`, userID).Scan(&name, &email, &profilePicture)

	if err != nil {
		http.Error(w, "Could not load profile", http.StatusInternalServerError)
		return
	}

	data := struct {
		Name           string
		Email          string
		ProfilePicture string
	}{
		Name:           name,
		Email:          email,
		ProfilePicture: profilePicture,
	}

	tmpl, err := template.ParseFiles("templates/edit-profile.html")
	if err != nil {
		http.Error(w, "Could not load profile page", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Could not render profile page", http.StatusInternalServerError)
	}
}

func (h *Handlers) updateProfile(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)

	err = r.ParseMultipartForm(5 << 20)
	if err != nil {
		http.Error(w, "Invalid upload", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))

	if name == "" {
		http.Error(w, "Name cannot be empty", http.StatusBadRequest)
		return
	}

	if len(name) > 100 {
		http.Error(w, "Name is too long", http.StatusBadRequest)
		return
	}

	profilePicture := ""

	file, header, err := r.FormFile("profile_picture")

	if err == nil {
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))

		allowed := map[string]bool{
			".jpg":  true,
			".jpeg": true,
			".png":  true,
			".webp": true,
		}

		if !allowed[ext] {
			http.Error(w, "Only JPG, JPEG, PNG and WEBP images are allowed", http.StatusBadRequest)
			return
		}

		cloudinaryURL := fmt.Sprintf(
			"cloudinary://%s:%s@%s",
			os.Getenv("CLOUDINARY_API_KEY"),
			os.Getenv("CLOUDINARY_API_SECRET"),
			os.Getenv("CLOUDINARY_CLOUD_NAME"),
		)

		cld, err := cloudinary.NewFromURL(cloudinaryURL)
		if err != nil {
			http.Error(w, "Could not configure image storage", http.StatusInternalServerError)
			return
		}

		uploadResult, err := cld.Upload.Upload(
			r.Context(),
			file,
			uploader.UploadParams{
				Folder: "graces-kitchen/profiles",
			},
		)

		if err != nil {
			fmt.Println("Cloudinary upload error:", err)
			http.Error(w, "Could not upload profile picture", http.StatusInternalServerError)
			return
		}

		profilePicture = uploadResult.SecureURL
	} else if err != http.ErrMissingFile {
		http.Error(w, "Could not process profile picture", http.StatusBadRequest)
		return
	}

	if profilePicture != "" {
		_, err = h.DB.Exec(`
			UPDATE users
			SET name = $1, profile_picture = $2
			WHERE id = $3
		`, name, profilePicture, userID)
	} else {
		_, err = h.DB.Exec(`
			UPDATE users
			SET name = $1
			WHERE id = $2
		`, name, userID)
	}

	if err != nil {
		http.Error(w, "Could not update profile", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/account?updated=true", http.StatusSeeOther)
}
