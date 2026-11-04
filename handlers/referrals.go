package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

const referralCookie = "ref"

var referralCodePattern = regexp.MustCompile(`^[A-Z2-7]{6,12}$`)

// referralBonusPoints is what both people get (1 point = N1 off food). Default 300.
func referralBonusPoints() int {
	if v, err := strconv.Atoi(os.Getenv("REFERRAL_BONUS_POINTS")); err == nil && v > 0 && v <= 5000 {
		return v
	}
	return 300
}

// referralMinOrderKobo is the food total a friend's first order must reach before the
// bonus is paid. It makes throw-away accounts not worth creating. Default N1,500.
func referralMinOrderKobo() int { return envKobo("REFERRAL_MIN_ORDER_KOBO", 150000) }

func newReferralCode() (string, error) {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil // 8 characters
}

// rememberReferral keeps a friend's code in a cookie while they browse, so it is still there
// when they finish signing up.
func rememberReferral(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("ref")))
	if !referralCodePattern.MatchString(code) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: referralCookie, Value: code, Path: "/", HttpOnly: true,
		Secure: middleware.SecureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 30,
	})
}

// linkReferral records who invited a brand-new account. It never fails the signup.
func (h *Handlers) linkReferral(w http.ResponseWriter, r *http.Request, newUserID int) {
	// A code typed on the signup form wins over one remembered from an invite link.
	code := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(r.FormValue("referral_code")), " ", ""))
	if !referralCodePattern.MatchString(code) {
		c, err := r.Cookie(referralCookie)
		if err != nil || !referralCodePattern.MatchString(c.Value) {
			return
		}
		code = c.Value
	}
	if _, err := h.DB.Exec(`
		UPDATE users SET referred_by = (SELECT id FROM users WHERE referral_code = $1 AND id <> $2 AND NOT is_guest)
		WHERE id = $2 AND referred_by IS NULL`, code, newUserID); err != nil {
		log.Printf("referral: link failed: %v", err)
	}
	http.SetCookie(w, &http.Cookie{Name: referralCookie, Value: "", Path: "/", MaxAge: -1})
}

// awardReferral pays the bonus to both people when a referred customer's first food order of
// at least the minimum is completed. The table's primary key makes it happen once only.
func (h *Handlers) awardReferral(userID, orderID, foodKobo int) {
	if foodKobo < referralMinOrderKobo() {
		return
	}
	var referrer sql.NullInt64
	if err := h.DB.QueryRow(`SELECT referred_by FROM users WHERE id = $1`, userID).Scan(&referrer); err != nil || !referrer.Valid {
		return
	}
	tx, err := h.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		INSERT INTO referral_rewards (referred_user_id, referrer_user_id, order_id, points)
		VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, userID, referrer.Int64, orderID, referralBonusPoints())
	if err != nil {
		log.Printf("referral: reward insert failed: %v", err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return // already paid for this friend
	}
	for _, uid := range []int64{referrer.Int64, int64(userID)} {
		desc := "Referral bonus: a friend you invited completed their first order"
		if uid == int64(userID) {
			desc = "Referral bonus: welcome gift from the friend who invited you"
		}
		if _, err := tx.Exec(`INSERT INTO loyalty_transactions (user_id, points, type, description) VALUES ($1, $2, 'earned', $3)`,
			uid, referralBonusPoints(), desc); err != nil {
			log.Printf("referral: bonus insert failed: %v", err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("referral: commit failed: %v", err)
	}
}

type ReferralPage struct {
	Link       string
	Code       string
	Bonus      int
	MinOrder   int
	Invited    int
	Rewarded   int
	PointsEarn int
}

// Referrals shows a customer their invite link and how many friends have joined.
func (h *Handlers) Referrals(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromRequest(r)
	if err != nil {
		errors.Render(w, "Please log in to invite friends.", http.StatusUnauthorized)
		return
	}
	var code sql.NullString
	if err := h.DB.QueryRow(`SELECT referral_code FROM users WHERE id = $1`, userID).Scan(&code); err != nil {
		errors.Render(w, "Could not load your invite link", http.StatusInternalServerError)
		return
	}
	for tries := 0; !code.Valid && tries < 5; tries++ {
		fresh, gerr := newReferralCode()
		if gerr != nil {
			break
		}
		var saved sql.NullString
		if err := h.DB.QueryRow(`UPDATE users SET referral_code = $1 WHERE id = $2 AND referral_code IS NULL RETURNING referral_code`, fresh, userID).Scan(&saved); err == nil {
			code = saved
		} else if err == sql.ErrNoRows {
			_ = h.DB.QueryRow(`SELECT referral_code FROM users WHERE id = $1`, userID).Scan(&code)
		}
		// a unique-code clash just tries again with a new code
	}
	if !code.Valid {
		errors.Render(w, "Could not create your invite link. Please try again.", http.StatusInternalServerError)
		return
	}

	page := ReferralPage{Code: code.String, Bonus: referralBonusPoints(), MinOrder: referralMinOrderKobo() / 100}
	base := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/")
	page.Link = base + "/signup?ref=" + code.String
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE referred_by = $1`, userID).Scan(&page.Invited)
	_ = h.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(points), 0) FROM referral_rewards WHERE referrer_user_id = $1`, userID).Scan(&page.Rewarded, &page.PointsEarn)

	tmpl, err := render.Page("referrals.html", template.FuncMap{})
	if err != nil {
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, page)
}
