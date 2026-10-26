package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"Order-site/database"
	"Order-site/errors"
	"Order-site/handlers"
	"Order-site/middleware"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found, using system environment")
	}

	db, err := database.Connect()
	if err != nil {
		fmt.Println("Database connection failed:", err)
		return
	}
	defer db.Close()

	if err := middleware.CleanupExpiredSessions(db); err != nil {
		fmt.Println("Session cleanup failed:", err)
	}

	h := handlers.New(db)

	// Lightweight in-process scheduler for scheduled customer orders. The database
	// update is atomic, so repeated ticks cannot activate the same order twice.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			h.ActivateScheduledOrders()
		}
	}()

	mux := http.NewServeMux()

	// Static files
	// Folder listings are switched off: only named files can be fetched.
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
				errors.Render(w, "We couldn't find the page you were looking for.", http.StatusNotFound)
				return
			}
			w.Header().Set("Cache-Control", "public, max-age=3600")
			fs.ServeHTTP(w, r)
		})),
	)

	mux.HandleFunc("GET /{$}", middleware.Logging(h.Home))
	mux.HandleFunc("GET /about", middleware.Logging(h.PublicPage))
	mux.HandleFunc("GET /guides", middleware.Logging(h.PublicPage))
	mux.HandleFunc("GET /guides/nigerian-rice-dishes", middleware.Logging(h.PublicPage))
	mux.HandleFunc("GET /guides/ordering-food-safely", middleware.Logging(h.PublicPage))
	mux.HandleFunc("GET /guides/pickup-or-delivery", middleware.Logging(h.PublicPage))
	mux.HandleFunc("GET /contact", middleware.Logging(h.PublicPage))
	mux.HandleFunc("GET /privacy", middleware.Logging(h.PublicPage))
	mux.HandleFunc("GET /terms", middleware.Logging(h.PublicPage))
	mux.HandleFunc("GET /login", middleware.Logging(h.Login))
	mux.HandleFunc("POST /login", middleware.Logging(middleware.RateLimit(middleware.LoginIPLimiter, h.Login)))
	mux.HandleFunc("GET /vendor/dashboard", middleware.Logging(h.VendorDashboard))
	mux.HandleFunc("GET /vendor/analytics", middleware.Logging(h.VendorAnalytics))
	mux.HandleFunc("GET /vendor/profile", middleware.Logging(h.VendorProfile))
	mux.HandleFunc("GET /vendor/ai", middleware.Logging(middleware.RequireLogin(db, h.VendorAI)))
	mux.HandleFunc("GET /messages", middleware.Logging(middleware.RequireLogin(db, h.MessagesInbox)))
	mux.HandleFunc("GET /notifications", middleware.Logging(middleware.RequireLogin(db, h.Notifications)))
	mux.HandleFunc("GET /notifications/unread-count", middleware.Logging(middleware.RequireLogin(db, h.NotificationUnreadCount)))
	mux.HandleFunc("GET /header/counts", middleware.Logging(h.HeaderCounts))
	mux.HandleFunc("POST /notifications/read", middleware.Logging(middleware.RequireLogin(db, h.NotificationRead)))
	mux.HandleFunc("POST /notifications/read-all", middleware.Logging(middleware.RequireLogin(db, h.NotificationsReadAll)))
	mux.HandleFunc("POST /messages/start", middleware.Logging(middleware.RequireLogin(db, h.StartMessageThread)))
	mux.HandleFunc("GET /messages/{threadID}", middleware.Logging(middleware.RequireLogin(db, h.MessageThread)))
	mux.HandleFunc("GET /messages/{threadID}/updates", middleware.Logging(middleware.RequireLogin(db, h.MessageUpdates)))
	mux.HandleFunc("POST /messages/{threadID}/ai-draft", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimit(middleware.MessageAILimiter, h.MessageAIDraft))))
	mux.HandleFunc("POST /messages/{threadID}/send", middleware.Logging(middleware.RequireLogin(db, h.SendMessage)))
	mux.HandleFunc("POST /vendor/ai/generate", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimit(middleware.VendorAILimiter, h.VendorAIGenerate))))
	mux.HandleFunc("POST /vendor/profile/save", middleware.Logging(h.VendorProfileSave))
	mux.HandleFunc("GET /vendor/payout-banks", middleware.Logging(middleware.RequireLogin(db, h.VendorPayoutBanks)))
	mux.HandleFunc("POST /vendor/payout-account/verify", middleware.Logging(middleware.RequireLogin(db, h.VendorPayoutAccountVerify)))
	mux.HandleFunc("POST /vendor/payout-account/save", middleware.Logging(middleware.RequireLogin(db, h.VendorPayoutAccountSave)))
	mux.HandleFunc("GET /signup", middleware.Logging(h.Signup))
	mux.HandleFunc("POST /signup", middleware.Logging(middleware.RateLimit(middleware.SignupIPLimiter, h.Signup)))
	mux.HandleFunc("GET /vendor/orders", middleware.Logging(h.VendorOrders))
	mux.HandleFunc("POST /vendor/orders/status", middleware.Logging(h.VendorUpdateOrderStatus))
	mux.HandleFunc("GET /logout", middleware.Logging(h.Logout))
	mux.HandleFunc("GET /vendor/menu", middleware.Logging(h.VendorMenu))
	mux.HandleFunc("GET /vendor/coupons", middleware.Logging(middleware.RequireLogin(db, h.VendorCoupons)))
	mux.HandleFunc("POST /vendor/coupons/save", middleware.Logging(middleware.RequireLogin(db, h.VendorCouponSave)))
	mux.HandleFunc("POST /vendor/coupons/toggle", middleware.Logging(middleware.RequireLogin(db, h.VendorCouponToggle)))
	mux.HandleFunc("POST /vendor/coupons/delete", middleware.Logging(middleware.RequireLogin(db, h.VendorCouponDelete)))
	mux.HandleFunc("POST /vendor/menu/save", middleware.Logging(h.VendorMenuSave))
	mux.HandleFunc("POST /vendor/menu/delete", middleware.Logging(h.VendorMenuDelete))
	// Menu browsing is public so visitors and search crawlers can discover
	// available dishes. Cart and checkout actions remain login-protected.
	mux.HandleFunc("GET /menu", middleware.Logging(h.Menu))
	mux.HandleFunc("GET /robots.txt", middleware.Logging(h.RobotsTxt))
	mux.HandleFunc("GET /sitemap.xml", middleware.Logging(h.SitemapXML))
	mux.HandleFunc("GET /ads.txt", middleware.Logging(h.AdsTxt))
	mux.HandleFunc("GET /checkout/details", middleware.Logging(middleware.RequireLogin(db, h.CheckoutDetails)))
	mux.HandleFunc("GET /checkout/coupon", middleware.Logging(middleware.RequireLogin(db, h.CheckoutCoupon)))
	mux.HandleFunc("GET /v/{slug}", middleware.Logging(h.VendorShop))
	mux.HandleFunc(
		"POST /cart/add",
		middleware.Logging(
			middleware.RequireLogin(db, h.AddToCart),
		),
	)
	mux.HandleFunc("GET /vendors", middleware.Logging(h.VendorsList))
	mux.HandleFunc(
		"GET /cart",
		middleware.Logging(
			middleware.RequireLogin(db, h.Cart),
		),
	)
	mux.HandleFunc("GET /admin", middleware.Logging(middleware.Admin(db, h.AdminDashboard)))
	mux.HandleFunc("GET /admin/users", middleware.Logging(middleware.SuperAdmin(db, handlers.MainAdminEmail(), h.AdminUsers)))
	mux.HandleFunc("POST /admin/users/delete", middleware.Logging(middleware.SuperAdmin(db, handlers.MainAdminEmail(), h.AdminUserDelete)))
	mux.HandleFunc("GET /admin/applications", middleware.Logging(middleware.Admin(db, h.AdminApplications)))
	mux.HandleFunc("POST /admin/applications/review", middleware.Logging(middleware.Admin(db, h.AdminApplicationReview)))
	mux.HandleFunc("GET /become-a-vendor", middleware.Logging(middleware.RequireLogin(db, h.VendorApply)))
	mux.HandleFunc("POST /become-a-vendor", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimit(middleware.ApplyLimiter, h.VendorApplySubmit))))
	mux.HandleFunc(
		"POST /cart/remove",
		middleware.Logging(
			middleware.RequireLogin(db, h.RemoveFromCart),
		),
	)

	mux.HandleFunc(
		"POST /cart/update",
		middleware.Logging(
			middleware.RequireLogin(db, h.UpdateCartQuantity),
		),
	)
	mux.HandleFunc("POST /favorites/toggle", middleware.Logging(middleware.RequireLogin(db, h.ToggleFavorite)))
	mux.HandleFunc("GET /favorites", middleware.Logging(middleware.RequireLogin(db, h.Favorites)))
	mux.HandleFunc(
		"POST /checkout",
		middleware.Logging(
			middleware.RequireLogin(db, h.Checkout),
		),
	)
	mux.HandleFunc(
		"GET /payment/callback",
		middleware.Logging(h.PaymentCallback),
	)
	mux.HandleFunc(
		"POST /payment/webhook",
		middleware.Logging(h.PaymentWebhook),
	)

	mux.HandleFunc(
		"GET /order-success",
		middleware.Logging(h.OrderSuccess),
	)
	mux.HandleFunc("GET /vendor/billing", middleware.Logging(h.VendorBilling))
	mux.HandleFunc("POST /vendor/pay", middleware.Logging(middleware.RateLimit(middleware.PayLimiter, h.VendorPay)))
	mux.HandleFunc("GET /vendor/payment/callback", middleware.Logging(h.VendorPaymentCallback))
	mux.HandleFunc(
		"GET /orders",
		middleware.Logging(
			middleware.RequireLogin(db, h.MyOrders),
		),
	)
	mux.HandleFunc("POST /orders/reorder", middleware.Logging(middleware.RequireLogin(db, h.Reorder)))
	mux.HandleFunc("GET /orders/{orderID}/review", middleware.Logging(middleware.RequireLogin(db, h.ReviewOrder)))
	mux.HandleFunc("POST /orders/{orderID}/review", middleware.Logging(middleware.RequireLogin(db, h.ReviewOrder)))
	mux.HandleFunc(
		"GET /admin/orders",
		middleware.Logging(
			middleware.Admin(db, h.AdminOrders),
		),
	)
	mux.HandleFunc("GET /vendor/billing/status", middleware.Logging(h.VendorBillingStatus))
	mux.HandleFunc(
		"POST /admin/orders/status",
		middleware.Logging(
			middleware.Admin(db, h.UpdateOrderStatus),
		),
	)
	mux.HandleFunc("GET /admin/earnings", middleware.Logging(middleware.SuperAdmin(db, handlers.MainAdminEmail(), h.AdminEarnings)))
	mux.HandleFunc("POST /admin/earnings/payout", middleware.Logging(middleware.SuperAdmin(db, handlers.MainAdminEmail(), h.AdminRecordPayout)))
	mux.HandleFunc("GET /admin/vendors", middleware.Logging(middleware.Admin(db, h.AdminVendors)))
	mux.HandleFunc("POST /admin/vendors/save", middleware.Logging(middleware.Admin(db, h.AdminVendorSave)))
	mux.HandleFunc("POST /admin/vendors/status", middleware.Logging(middleware.Admin(db, h.AdminVendorStatus)))
	mux.HandleFunc("GET /cart/count", middleware.Logging(h.CartCount))
	mux.HandleFunc("GET /admin/menu", middleware.Logging(middleware.Admin(db, h.AdminMenu)))
	mux.HandleFunc("POST /admin/menu/save", middleware.Logging(middleware.Admin(db, h.AdminMenuSave)))
	mux.HandleFunc("POST /admin/menu/delete", middleware.Logging(middleware.Admin(db, h.AdminMenuDelete)))

	mux.HandleFunc(
		"/auth/google",
		middleware.Logging(h.GoogleLogin),
	)

	mux.HandleFunc(
		"/auth/google/callback",
		middleware.Logging(
			h.GoogleCallback,
		),
	)

	mux.HandleFunc(
		"/account",
		middleware.Logging(middleware.RequireLogin(db, h.Account)),
	)

	mux.HandleFunc(
		"/loyalty", middleware.Logging(middleware.RequireLogin(db, h.Loyalty)),
	)

	mux.HandleFunc(
		"/account/edit",
		middleware.Logging(middleware.RequireLogin(db, h.EditProfile)),
	)

	mux.HandleFunc("GET /forgot-password", middleware.Logging(h.ForgotPassword))
	mux.HandleFunc("POST /forgot-password", middleware.Logging(middleware.RateLimit(middleware.ForgotIPLimiter, h.ForgotPassword)))
	mux.HandleFunc("GET /reset-password", middleware.Logging(h.ResetPassword))
	mux.HandleFunc("POST /reset-password", middleware.Logging(middleware.RateLimit(middleware.ResetIPLimiter, h.ResetPassword)))

	mux.HandleFunc(
		"POST /account/phone/send-otp",
		middleware.Logging(
			middleware.RequireLogin(db, h.SendPhoneOTP),
		),
	)

	mux.HandleFunc(
		"POST /account/phone/verify-otp",
		middleware.Logging(
			middleware.RequireLogin(db, h.VerifyPhoneOTP),
		),
	)
	// Any address that matches no other route gets the styled 404 page
	mux.HandleFunc("/", middleware.Logging(func(w http.ResponseWriter, r *http.Request) {
		errors.Render(w, "We couldn't find the page you were looking for.", http.StatusNotFound)
	}))
	// HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Order matters: headers first, then the size limit, then the forged-request check.
	// The timeouts stop slow or stuck connections from tying up the server.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           middleware.SecurityHeaders(middleware.LimitBody(middleware.SameOrigin(mux))),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	go func() {
		fmt.Println("Server Running on port", port)

		if err := srv.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			fmt.Println("Server error:", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	fmt.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Println("Forced shutdown:", err)
	}

	fmt.Println("Server stopped cleanly")
}
