package handlers

import (
	"Order-site/errors"
	"net/http"
)

// SaveActiveStatus updates whether other chat participants may see this user's active state.
func (h *Handlers) SaveActiveStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := requestUserID(h, r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	enabled := r.FormValue("active_status") == "on"
	if _, err := h.DB.Exec(`UPDATE users SET active_status_enabled=$1 WHERE id=$2`, enabled, userID); err != nil {
		errors.Render(w, "Could not save your privacy setting", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/account?updated=true", http.StatusSeeOther)
}
