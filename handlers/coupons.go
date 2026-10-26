package handlers

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Order-site/brand"
	appErrors "Order-site/errors"
	"Order-site/middleware"
	"errors"
)

type Coupon struct {
	ID              int
	Code            string
	DiscountType    string
	DiscountValue   int
	MinOrderKobo    int
	MaxDiscountKobo int
	UsageLimit      int
	UsedCount       int
	ReservedCount   int
	Active          bool
	StartsAt        string
	ExpiresAt       string
}

type CouponPage struct {
	VendorName string
	Coupons    []Coupon
	Notice     string
}

func normalizeCouponCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) > 40 {
		code = code[:40]
	}
	return code
}

func (h *Handlers) CheckoutCoupon(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, `{"ok":false,"message":"Please log in"}`, http.StatusUnauthorized)
		return
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Error(w, `{"ok":false,"message":"Invalid session"}`, http.StatusUnauthorized)
		return
	}
	code := normalizeCouponCode(r.URL.Query().Get("code"))
	var vendorID, subtotal int
	err = h.DB.QueryRow(`SELECT m.vendor_id, COALESCE(SUM(ci.quantity*ci.price_kobo_at_addition),0) FROM cart_items ci JOIN menu_items m ON m.id=ci.menu_item_id WHERE ci.user_id=$1 GROUP BY m.vendor_id`, userID).Scan(&vendorID, &subtotal)
	if err != nil || vendorID == 0 {
		http.Error(w, `{"ok":false,"message":"Could not check your cart"}`, http.StatusBadRequest)
		return
	}
	_, discount, err := h.validateCoupon(vendorID, code, subtotal)
	resp := map[string]any{"ok": err == nil, "discount_kobo": discount}
	if err != nil {
		resp["message"] = err.Error()
	} else {
		resp["message"] = "Coupon applied"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) VendorCoupons(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorName, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	page := CouponPage{VendorName: vendorName, Notice: r.URL.Query().Get("msg")}
	rows, err := h.DB.Query(`
        SELECT id, code, discount_type, discount_value, min_order_kobo, max_discount_kobo,
               usage_limit, used_count, reserved_count, active,
               TO_CHAR(starts_at, 'YYYY-MM-DD HH24:MI'), TO_CHAR(expires_at, 'YYYY-MM-DD HH24:MI')
        FROM coupons WHERE vendor_id = $1 ORDER BY created_at DESC, id DESC`, vendorID)
	if err != nil {
		log.Printf("vendor coupons query: %v", err)
		appErrors.Render(w, "Could not load coupons", 500)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var c Coupon
		if err := rows.Scan(&c.ID, &c.Code, &c.DiscountType, &c.DiscountValue, &c.MinOrderKobo, &c.MaxDiscountKobo, &c.UsageLimit, &c.UsedCount, &c.ReservedCount, &c.Active, &c.StartsAt, &c.ExpiresAt); err != nil {
			log.Printf("vendor coupons scan: %v", err)
			appErrors.Render(w, "Could not read coupons", 500)
			return
		}
		page.Coupons = append(page.Coupons, c)
	}
	if err := rows.Err(); err != nil {
		log.Printf("vendor coupons rows: %v", err)
		appErrors.Render(w, "Could not read coupons", 500)
		return
	}
	tmpl, err := template.New("vendor-coupons.html").Funcs(template.FuncMap{"money": money, "naira": naira}).Funcs(brand.Funcs()).ParseFiles("templates/vendor-coupons.html")
	if err != nil {
		log.Printf("vendor coupons template: %v", err)
		appErrors.Render(w, "Could not display coupons", 500)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendor coupons execute: %v", err)
	}
}

func (h *Handlers) VendorCouponSave(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	code := normalizeCouponCode(r.FormValue("code"))
	dtype := strings.TrimSpace(r.FormValue("discount_type"))
	value, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("discount_value")))
	minOrder, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("min_order")))
	maxDiscount, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("max_discount")))
	usageLimit, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("usage_limit")))
	starts := strings.TrimSpace(r.FormValue("starts_at"))
	expires := strings.TrimSpace(r.FormValue("expires_at"))
	active := r.FormValue("active") == "on"

	if code == "" || len(code) < 3 || dtype != "percentage" && dtype != "flat" || value <= 0 || minOrder < 0 || maxDiscount < 0 || usageLimit < 0 {
		http.Redirect(w, r, "/vendor/coupons?msg=Please+enter+valid+coupon+details", http.StatusSeeOther)
		return
	}
	if dtype == "percentage" && value > 100 {
		http.Redirect(w, r, "/vendor/coupons?msg=Percentage+must+be+100+or+less", http.StatusSeeOther)
		return
	}
	if dtype == "flat" {
		value *= 100
		maxDiscount = maxDiscount * 100
		minOrder *= 100
	} else {
		minOrder *= 100
		maxDiscount *= 100
	}
	if maxDiscount == 0 {
		maxDiscount = 0
	}

	var startTime, endTime any = time.Now(), nil
	if starts != "" {
		if t, err := time.Parse("2006-01-02T15:04", starts); err == nil {
			startTime = t
		}
	}
	if expires != "" {
		if t, err := time.Parse("2006-01-02T15:04", expires); err == nil {
			endTime = t
		}
	}
	_, err := h.DB.Exec(`
        INSERT INTO coupons (vendor_id, code, discount_type, discount_value, min_order_kobo, max_discount_kobo, usage_limit, active, starts_at, expires_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        ON CONFLICT (vendor_id, code) DO UPDATE SET discount_type=EXCLUDED.discount_type, discount_value=EXCLUDED.discount_value,
          min_order_kobo=EXCLUDED.min_order_kobo, max_discount_kobo=EXCLUDED.max_discount_kobo, usage_limit=EXCLUDED.usage_limit,
          active=EXCLUDED.active, starts_at=EXCLUDED.starts_at, expires_at=EXCLUDED.expires_at`, vendorID, code, dtype, value, minOrder, maxDiscount, usageLimit, active, startTime, endTime)
	if err != nil {
		log.Printf("vendor coupon save: %v", err)
		http.Redirect(w, r, "/vendor/coupons?msg=Could+not+save+coupon", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/vendor/coupons?msg=Coupon+saved", http.StatusSeeOther)
}

func (h *Handlers) VendorCouponToggle(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	id, _ := strconv.Atoi(r.FormValue("id"))
	_, err := h.DB.Exec(`UPDATE coupons SET active = NOT active WHERE id=$1 AND vendor_id=$2`, id, vendorID)
	if err != nil {
		log.Printf("vendor coupon toggle: %v", err)
	}
	http.Redirect(w, r, "/vendor/coupons", http.StatusSeeOther)
}

func (h *Handlers) VendorCouponDelete(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	id, _ := strconv.Atoi(r.FormValue("id"))
	_, err := h.DB.Exec(`DELETE FROM coupons WHERE id=$1 AND vendor_id=$2 AND used_count=0 AND reserved_count=0`, id, vendorID)
	if err != nil {
		log.Printf("vendor coupon delete: %v", err)
	}
	http.Redirect(w, r, "/vendor/coupons", http.StatusSeeOther)
}

func (h *Handlers) reserveCoupon(tx interface{ QueryRow(string, ...any) *sql.Row }, couponID int) error {
	if couponID == 0 {
		return nil
	}
	var id int
	err := tx.QueryRow(`UPDATE coupons SET reserved_count = reserved_count + 1 WHERE id=$1 AND (usage_limit=0 OR used_count+reserved_count < usage_limit) RETURNING id`, couponID).Scan(&id)
	return err
}

func settleCoupon(tx interface {
	QueryRow(string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
}, orderID int) error {
	var couponID int
	var reserved bool
	err := tx.QueryRow(`SELECT COALESCE(coupon_id,0), coupon_reserved FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&couponID, &reserved)
	if err != nil || couponID == 0 || !reserved {
		return err
	}
	var couponRow int
	err = tx.QueryRow(`UPDATE coupons SET reserved_count = GREATEST(reserved_count-1,0), used_count = used_count+1 WHERE id=$1 RETURNING id`, couponID).Scan(&couponRow)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE orders SET coupon_reserved=FALSE WHERE id=$1`, orderID)
	return err
}

// validateCoupon returns the discount in kobo for the current food subtotal.
func (h *Handlers) validateCoupon(vendorID int, code string, foodSubtotalKobo int) (int, int, error) {
	code = normalizeCouponCode(code)
	if code == "" {
		return 0, 0, nil
	}
	var id, value, minOrder, maxDiscount, usageLimit, usedCount, reservedCount int
	var dtype string
	var active bool
	err := h.DB.QueryRow(`
        SELECT id, discount_type, discount_value, min_order_kobo, max_discount_kobo, usage_limit, used_count, reserved_count, active
        FROM coupons WHERE vendor_id=$1 AND code=$2 AND starts_at <= NOW() AND (expires_at IS NULL OR expires_at > NOW())
    `, vendorID, code).Scan(&id, &dtype, &value, &minOrder, &maxDiscount, &usageLimit, &usedCount, &reservedCount, &active)
	if err != nil || !active {
		return 0, 0, errors.New("Coupon is invalid or unavailable")
	}
	if usageLimit > 0 && usedCount+reservedCount >= usageLimit {
		return 0, 0, errors.New("This coupon has reached its usage limit")
	}
	if foodSubtotalKobo < minOrder {
		return 0, 0, errors.New("Your food subtotal is below this coupon's minimum order")
	}
	discount := value
	if dtype == "percentage" {
		discount = foodSubtotalKobo * value / 100
	}
	if maxDiscount > 0 && discount > maxDiscount {
		discount = maxDiscount
	}
	if discount > foodSubtotalKobo {
		discount = foodSubtotalKobo
	}
	if discount <= 0 {
		return 0, 0, errors.New("This coupon does not apply to this order")
	}
	return id, discount, nil
}
