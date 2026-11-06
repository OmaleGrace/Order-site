package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"

	"golang.org/x/crypto/bcrypt"

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

	errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	var phone *string

	err = h.DB.QueryRow(`
		SELECT name, email, COALESCE(profile_picture, ''), phone
		FROM users
		WHERE id = $1
	`, userID).Scan(&name, &email, &profilePicture, &phone)

	if err != nil {
		errors.Render(w, "Could not load profile", http.StatusInternalServerError)
		return
	}

	phoneValue := ""
	if phone != nil {
		phoneValue = *phone
	}

	x := h.loadAccountExtras(r.Context(), userID)
	var provider string
	_ = h.DB.QueryRow(`SELECT COALESCE(to_jsonb(users)->>'auth_provider', 'local') FROM users WHERE id = $1`, userID).Scan(&provider)
	initial := "?"
	if rs := []rune(strings.TrimSpace(name)); len(rs) > 0 {
		initial = strings.ToUpper(string(rs[0]))
	}
	q := r.URL.Query()
	data := struct {
		Name, Email, ProfilePicture, Phone, Initial string
		Extras                                      accountExtras
		CanChangePassword                           bool
		Error, Notice                               string
	}{
		Name: name, Email: email, ProfilePicture: profilePicture, Phone: phoneValue, Initial: initial,
		Extras:            x,
		CanChangePassword: provider == "" || provider == "local",
		Error:             profileMessage(q.Get("err")),
		Notice:            profileMessage(q.Get("ok")),
	}

	tmpl, err := render.Page("edit-profile.html")
	if err != nil {
		errors.Render(w, "Could not load profile page", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		errors.Render(w, "Could not render profile page", http.StatusInternalServerError)
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
		errors.Render(w, "Invalid upload", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))

	if name == "" {
		errors.Render(w, "Name cannot be empty", http.StatusBadRequest)
		return
	}

	if len(name) > 100 {
		errors.Render(w, "Name is too long", http.StatusBadRequest)
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
			errors.Render(w, "Only JPG, JPEG, PNG and WEBP images are allowed", http.StatusBadRequest)
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
			errors.Render(w, "Could not configure image storage", http.StatusInternalServerError)
			return
		}

		uploadResult, err := cld.Upload.Upload(
			r.Context(),
			file,
			uploader.UploadParams{
				Folder:         "graces-kitchen/profiles",
				Transformation: "c_limit,w_600,h_600/q_auto,f_auto", // shrink big phone photos before storing
			},
		)

		if err != nil {
			fmt.Println("Cloudinary upload error:", err)
			errors.Render(w, "Could not upload profile picture", http.StatusInternalServerError)
			return
		}

		profilePicture = uploadResult.SecureURL
	} else if err != http.ErrMissingFile {
		errors.Render(w, "Could not process profile picture", http.StatusBadRequest)
		return
	}

	// Saved delivery details. These columns are created by migration 0004 and the baseline, so a
	// failure here must not block the name and photo from saving.
	contact := r.FormValue("preferred_contact")
	if contact != "email" && contact != "sms" {
		contact = "email"
	}
	if _, xerr := h.DB.Exec(`
		UPDATE users SET address_line = $1, delivery_area = $2, state = $3, delivery_note = $4,
		       preferred_contact = $5, marketing_opt_in = $6
		WHERE id = $7`,
		cleanText(r.FormValue("address_line"), 200), cleanText(r.FormValue("delivery_area"), 80),
		cleanText(r.FormValue("state"), 60), cleanText(r.FormValue("delivery_note"), 200),
		contact, r.FormValue("marketing_opt_in") == "on", userID); xerr != nil {
		fmt.Println("Profile: could not save delivery details:", xerr)
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
		errors.Render(w, "Could not update profile", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/account?updated=true", http.StatusSeeOther)
}

// profileMessage maps the short codes used in /account/edit?err=...&ok=... to text. Codes are
// looked up rather than echoed so a crafted link cannot put arbitrary words on the page.
func profileMessage(code string) string {
	switch code {
	case "pw-wrong":
		return "Your current password is not correct."
	case "pw-mismatch":
		return "The new passwords do not match."
	case "pw-weak":
		return "Your new password must be at least 8 characters and mix upper and lower case letters, a number and a symbol."
	case "pw-same":
		return "Choose a password you have not used here before."
	case "pw-error":
		return "We could not change your password. Please try again."
	case "pw-unavailable":
		return "This account signs in with Google or Facebook, so it has no password to change."
	case "phone-saved":
		return "Your phone number is verified and saved."
	}
	return ""
}

// ChangePassword lets a signed-in customer set a new password. Everyone is signed out
// afterwards, which also ends any session an attacker may have opened with the old password.
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := requestUserID(h, r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	back := func(code string) {
		http.Redirect(w, r, "/account/edit?err="+code+"#security", http.StatusSeeOther)
	}
	current := r.FormValue("current_password")
	next := r.FormValue("new_password")
	if next != r.FormValue("confirm_password") {
		back("pw-mismatch")
		return
	}
	var stored sql.NullString
	if err := h.DB.QueryRow(`SELECT password FROM users WHERE id = $1`, userID).Scan(&stored); err != nil {
		back("pw-error")
		return
	}
	if !stored.Valid || stored.String == "" {
		back("pw-unavailable")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(stored.String), []byte(current)) != nil {
		back("pw-wrong")
		return
	}
	if validatePassword(next) != nil {
		back("pw-weak")
		return
	}
	if next == current {
		back("pw-same")
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		back("pw-error")
		return
	}
	if _, err := h.DB.Exec(`UPDATE users SET password = $1 WHERE id = $2`, string(hashed), userID); err != nil {
		back("pw-error")
		return
	}
	if err := middleware.DeleteUserSessions(h.DB, userID); err != nil {
		fmt.Println("Password change: could not end old sessions:", err)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
