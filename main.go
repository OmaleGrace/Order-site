package main

import (
	"Order-site/alerts"
	"Order-site/push"
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
	"Order-site/safe"

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

	if err := database.CleanupGuestAccounts(db); err != nil {
		fmt.Println("Guest cleanup failed:", err)
	}

	if err := database.CleanupUnusedImages(db); err != nil {
		fmt.Println("Image cleanup failed:", err)
	}

	h := handlers.New(db)

	// Email the owner when something breaks (set ALERT_EMAIL; see alerts/alerts.go).
	alerts.Install()
	if err := push.Init(); err != nil {
		fmt.Println("Push notifications are off:", err, "(run `go run ./cmd/vapid-keys` and set the keys to turn them on)")
	} else {
		fmt.Println("Push notifications are on")
	}
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	alerts.WatchDB(db, stopWatch)

	// Lightweight in-process scheduler for scheduled customer orders. The database
	// update is atomic, so repeated ticks cannot activate the same order twice.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			// One bad tick must not stop scheduled orders for good, or crash the site.
			func() {
				defer safe.Recover("scheduled order activation")
				h.ActivateScheduledOrders()
			}()
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

	mux.HandleFunc("GET /healthz", h.Health)
	mux.HandleFunc("GET /push/key", middleware.Logging(h.PushKey))
	mux.HandleFunc("GET /push/status", middleware.Logging(h.PushStatus))
	mux.HandleFunc("POST /push/subscribe", middleware.Logging(middleware.RateLimit(middleware.PushLimiter, h.PushSubscribe)))
	mux.HandleFunc("POST /push/unsubscribe", middleware.Logging(h.PushUnsubscribe))
	mux.HandleFunc("POST /push/test", middleware.Logging(middleware.RateLimit(middleware.PushLimiter, h.PushTest)))
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
	mux.HandleFunc("GET /vendor/delivery", middleware.Logging(h.VendorDeliveryPricing))
	mux.HandleFunc("POST /vendor/delivery/save", middleware.Logging(h.VendorDeliverySave))
	mux.HandleFunc("GET /vendor/statements", middleware.Logging(h.VendorStatements))
	mux.HandleFunc("GET /vendor/statements/export", middleware.Logging(h.VendorStatementExport))
	mux.HandleFunc("GET /vendor/profile", middleware.Logging(h.VendorProfile))
	mux.HandleFunc("GET /vendor/ai", middleware.Logging(middleware.RequireLogin(db, h.VendorAI)))
	mux.HandleFunc("GET /messages", middleware.Logging(middleware.RequireLogin(db, h.MessagesInbox)))
	mux.HandleFunc("POST /account/presence", middleware.Logging(middleware.RequireLogin(db, h.SaveActiveStatus)))
	mux.HandleFunc("GET /help", middleware.Logging(middleware.RequireLogin(db, h.Help)))
	mux.HandleFunc("POST /help/ask", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimitByUser(middleware.MessageAILimiter, h.HelpAssistant))))
	mux.HandleFunc("POST /help/tickets", middleware.Logging(middleware.RequireLogin(db, h.CreateSupportTicket)))
	mux.HandleFunc("GET /notifications", middleware.Logging(middleware.RequireLogin(db, h.Notifications)))
	mux.HandleFunc("GET /notifications/unread-count", middleware.Logging(middleware.RequireLogin(db, h.NotificationUnreadCount)))
	mux.HandleFunc("GET /header/counts", middleware.Logging(h.HeaderCounts))
	mux.HandleFunc("POST /notifications/read", middleware.Logging(middleware.RequireLogin(db, h.NotificationRead)))
	mux.HandleFunc("POST /notifications/read-all", middleware.Logging(middleware.RequireLogin(db, h.NotificationsReadAll)))
	mux.HandleFunc("POST /messages/start", middleware.Logging(middleware.RequireLogin(db, h.StartMessageThread)))
	mux.HandleFunc("GET /messages/{threadID}", middleware.Logging(middleware.RequireLogin(db, h.MessageThread)))
	mux.HandleFunc("GET /messages/{threadID}/updates", middleware.Logging(middleware.RequireLogin(db, h.MessageUpdates)))
	mux.HandleFunc("POST /messages/{threadID}/ai-draft", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimitByUser(middleware.MessageAILimiter, h.MessageAIDraft))))
	mux.HandleFunc("POST /messages/{threadID}/send", middleware.Logging(middleware.RequireLogin(db, h.SendMessage)))
	mux.HandleFunc("POST /vendor/ai/generate", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimitByUser(middleware.VendorAILimiter, h.VendorAIGenerate))))
	mux.HandleFunc("POST /vendor/profile/save", middleware.Logging(h.VendorProfileSave))
	mux.HandleFunc("GET /vendor/payout-banks", middleware.Logging(middleware.RequireLogin(db, h.VendorPayoutBanks)))
	mux.HandleFunc("POST /vendor/payout-account/verify", middleware.Logging(middleware.RequireLogin(db, h.VendorPayoutAccountVerify)))
	mux.HandleFunc("POST /vendor/payout-account/save", middleware.Logging(middleware.RequireLogin(db, h.VendorPayoutAccountSave)))
	mux.HandleFunc("GET /signup", middleware.Logging(h.Signup))
	mux.HandleFunc("POST /signup", middleware.Logging(middleware.RateLimit(middleware.SignupIPLimiter, h.Signup)))
	mux.HandleFunc("GET /vendor/orders", middleware.Logging(h.VendorOrders))
	mux.HandleFunc("POST /vendor/orders/status", middleware.Logging(h.VendorUpdateOrderStatus))
	mux.HandleFunc("POST /vendor/orders/rider", middleware.Logging(middleware.RequireLogin(db, h.VendorAssignRider)))
	// Rider URLs are bearer credentials; do not write their tokens into application request logs.
	mux.HandleFunc("GET /rider/{token}", h.RiderPortal)
	mux.HandleFunc("POST /rider/{token}/status", h.RiderUpdateStatus)
	mux.HandleFunc("POST /rider/{token}/location", h.RiderLocation)
	mux.HandleFunc("POST /rider/{token}/stop-location", h.RiderStopLocation)
	mux.HandleFunc("GET /logout", middleware.Logging(h.Logout))
	mux.HandleFunc("GET /vendor/menu", middleware.Logging(h.VendorMenu))
	mux.HandleFunc("GET /vendor/menu/customizations", middleware.Logging(middleware.RequireLogin(db, h.VendorCustomizationPage)))
	mux.HandleFunc("POST /vendor/menu/customizations/group/save", middleware.Logging(middleware.RequireLogin(db, h.VendorCustomizationGroupSave)))
	mux.HandleFunc("POST /vendor/menu/customizations/option/save", middleware.Logging(middleware.RequireLogin(db, h.VendorCustomizationOptionSave)))
	mux.HandleFunc("POST /vendor/menu/customizations/toggle", middleware.Logging(middleware.RequireLogin(db, h.VendorCustomizationToggle)))
	mux.HandleFunc("GET /vendor/coupons", middleware.Logging(middleware.RequireLogin(db, h.VendorCoupons)))
	mux.HandleFunc("POST /vendor/coupons/save", middleware.Logging(middleware.RequireLogin(db, h.VendorCouponSave)))
	mux.HandleFunc("POST /vendor/coupons/toggle", middleware.Logging(middleware.RequireLogin(db, h.VendorCouponToggle)))
	mux.HandleFunc("POST /vendor/coupons/delete", middleware.Logging(middleware.RequireLogin(db, h.VendorCouponDelete)))
	mux.HandleFunc("POST /vendor/menu/save", middleware.Logging(h.VendorMenuSave))
	mux.HandleFunc("POST /vendor/menu/delete", middleware.Logging(h.VendorMenuDelete))
	mux.HandleFunc("POST /vendor/menu/availability", middleware.Logging(middleware.RequireLogin(db, h.VendorMenuAvailability)))
	mux.HandleFunc("POST /vendor/menu/duplicate", middleware.Logging(middleware.RequireLogin(db, h.VendorMenuDuplicate)))
	mux.HandleFunc("POST /vendor/menu/bulk", middleware.Logging(middleware.RequireLogin(db, h.VendorMenuBulkUpdate)))
	// Menu browsing is public so visitors and search crawlers can discover
	// available dishes. Cart and checkout actions remain login-protected.
	mux.HandleFunc("GET /menu", middleware.Logging(h.Menu))
	mux.HandleFunc("GET /menu/options", middleware.Logging(h.MenuItemOptions))
	mux.HandleFunc("GET /sw.js", h.ServiceWorker)
	mux.HandleFunc("GET /manifest.webmanifest", h.WebManifest)
	mux.HandleFunc("GET /offline", h.OfflinePage)
	mux.HandleFunc("GET /robots.txt", middleware.Logging(h.RobotsTxt))
	mux.HandleFunc("GET /sitemap.xml", middleware.Logging(h.SitemapXML))
	mux.HandleFunc("GET /ads.txt", middleware.Logging(h.AdsTxt))
	// Cart and checkout work for guests too: each handler resolves a logged-in customer or a
	// guest cart session itself. Everything under /account, /orders and so on still needs a login.
	mux.HandleFunc("GET /checkout/details", middleware.Logging(h.CheckoutDetails))
	mux.HandleFunc("GET /checkout/coupon", middleware.Logging(h.CheckoutCoupon))
	mux.HandleFunc("GET /media/{id}", h.ServeUploadedImage)
	mux.HandleFunc("GET /wallet", middleware.Logging(middleware.RequireLogin(db, h.Wallet)))
	mux.HandleFunc("POST /wallet/pay", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimit(middleware.PayLimiter, h.WalletPay))))
	mux.HandleFunc("POST /wallet/redeem", middleware.Logging(middleware.RequireLogin(db, h.WalletRedeem)))
	mux.HandleFunc("GET /wallet/callback", middleware.Logging(h.WalletCallback))
	mux.HandleFunc("GET /group-order", middleware.Logging(middleware.RequireLogin(db, h.GroupOrderForm)))
	mux.HandleFunc("POST /group-order", middleware.Logging(middleware.RequireLogin(db, h.GroupOrderSubmit)))
	mux.HandleFunc("GET /group-orders", middleware.Logging(middleware.RequireLogin(db, h.GroupOrders)))
	mux.HandleFunc("POST /group-orders/pay", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimit(middleware.PayLimiter, h.GroupOrderPay))))
	mux.HandleFunc("POST /group-orders/cancel", middleware.Logging(middleware.RequireLogin(db, h.GroupOrderCancel)))
	mux.HandleFunc("GET /vendor/group-orders", middleware.Logging(h.VendorGroupOrders))
	mux.HandleFunc("POST /vendor/group-orders/quote", middleware.Logging(h.VendorGroupOrderQuote))
	mux.HandleFunc("GET /referrals", middleware.Logging(middleware.RequireLogin(db, h.Referrals)))
	mux.HandleFunc("GET /feedback", middleware.Logging(h.Feedback))
	mux.HandleFunc("POST /feedback", middleware.Logging(h.SubmitFeedback))
	mux.HandleFunc("GET /admin/feedback", middleware.Logging(middleware.Admin(db, h.AdminFeedback)))
	mux.HandleFunc("POST /admin/feedback/update", middleware.Logging(middleware.Admin(db, h.AdminFeedbackUpdate)))
	mux.HandleFunc("GET /track/{orderID}/{token}", middleware.Logging(h.TrackOrder))
	mux.HandleFunc("GET /track/{orderID}/{token}/status", middleware.Logging(h.TrackOrderStatus))
	mux.HandleFunc("GET /v/{slug}", middleware.Logging(h.VendorShop))
	mux.HandleFunc(
		"POST /cart/add",
		middleware.Logging(
			h.AddToCart,
		),
	)
	mux.HandleFunc("GET /vendors", middleware.Logging(h.VendorsList))
	mux.HandleFunc("GET /cart", middleware.Logging(h.Cart))
	mux.HandleFunc("GET /admin", middleware.Logging(middleware.Admin(db, h.AdminDashboard)))
	mux.HandleFunc("GET /admin/support", middleware.Logging(middleware.Admin(db, h.AdminSupport)))
	mux.HandleFunc("GET /admin/riders", middleware.Logging(middleware.Admin(db, h.AdminRiders)))
	mux.HandleFunc("POST /admin/riders/save", middleware.Logging(middleware.Admin(db, h.AdminRiderSave)))
	mux.HandleFunc("POST /admin/riders/toggle", middleware.Logging(middleware.Admin(db, h.AdminRiderToggle)))
	mux.HandleFunc("POST /admin/orders/rider", middleware.Logging(middleware.Admin(db, h.AdminAssignRider)))
	mux.HandleFunc("POST /admin/support/ai-reply", middleware.Logging(middleware.Admin(db, middleware.RateLimitByUser(middleware.MessageAILimiter, h.AdminSupportAIReply))))
	mux.HandleFunc("POST /admin/support/update", middleware.Logging(middleware.Admin(db, h.AdminSupportUpdate)))
	mux.HandleFunc("GET /admin/users", middleware.Logging(middleware.SuperAdmin(db, handlers.MainAdminEmail(), h.AdminUsers)))
	mux.HandleFunc("POST /admin/users/delete", middleware.Logging(middleware.SuperAdmin(db, handlers.MainAdminEmail(), h.AdminUserDelete)))
	mux.HandleFunc("GET /admin/applications", middleware.Logging(middleware.Admin(db, h.AdminApplications)))
	mux.HandleFunc("POST /admin/applications/review", middleware.Logging(middleware.Admin(db, h.AdminApplicationReview)))
	mux.HandleFunc("GET /vendor-agreement", middleware.Logging(h.VendorAgreement))
	mux.HandleFunc("POST /become-a-vendor/agree", middleware.Logging(middleware.RequireLogin(db, h.VendorAgreementAccept)))
	mux.HandleFunc("GET /become-a-vendor", middleware.Logging(middleware.RequireLogin(db, h.VendorApply)))
	mux.HandleFunc("POST /become-a-vendor", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimit(middleware.ApplyLimiter, h.VendorApplySubmit))))
	mux.HandleFunc("POST /cart/remove", middleware.Logging(h.RemoveFromCart))

	mux.HandleFunc("POST /cart/update", middleware.Logging(h.UpdateCartQuantity))
	mux.HandleFunc("POST /favorites/toggle", middleware.Logging(middleware.RequireLogin(db, h.ToggleFavorite)))
	mux.HandleFunc("GET /favorites", middleware.Logging(middleware.RequireLogin(db, h.Favorites)))
	mux.HandleFunc("POST /checkout", middleware.Logging(h.Checkout))
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
	mux.HandleFunc("POST /vendor/buy", middleware.Logging(middleware.RateLimit(middleware.PayLimiter, h.VendorBuyExtra)))
	mux.HandleFunc("GET /vendor/payment/callback", middleware.Logging(h.VendorPaymentCallback))
	mux.HandleFunc("GET /orders/{orderID}/rider-location", middleware.Logging(middleware.RequireLogin(db, h.CustomerRiderLocation)))
	mux.HandleFunc(
		"GET /orders",
		middleware.Logging(
			middleware.RequireLogin(db, h.MyOrders),
		),
	)
	mux.HandleFunc("POST /orders/reorder", middleware.Logging(middleware.RequireLogin(db, h.Reorder)))
	mux.HandleFunc("GET /orders/{orderID}/review", middleware.Logging(middleware.RequireLogin(db, h.ReviewOrder)))
	mux.HandleFunc("POST /orders/{orderID}/review", middleware.Logging(middleware.RequireLogin(db, h.ReviewOrder)))
	mux.HandleFunc("GET /admin/orders/{orderID}/rider-location", middleware.Logging(middleware.Admin(db, h.AdminRiderLocation)))
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
	mux.HandleFunc("POST /admin/earnings/transfer", middleware.Logging(middleware.SuperAdmin(db, handlers.MainAdminEmail(), h.AdminTransferPayout)))
	mux.HandleFunc("GET /admin/settlements", middleware.Logging(middleware.SuperAdmin(db, handlers.MainAdminEmail(), h.AdminSettlements)))
	mux.HandleFunc("GET /admin/settlements/export", middleware.Logging(middleware.SuperAdmin(db, handlers.MainAdminEmail(), h.AdminSettlementsExport)))
	mux.HandleFunc("GET /admin/vendors", middleware.Logging(middleware.Admin(db, h.AdminVendors)))
	mux.HandleFunc("POST /admin/vendors/save", middleware.Logging(middleware.Admin(db, h.AdminVendorSave)))
	mux.HandleFunc("POST /admin/vendors/growth", middleware.Logging(middleware.Admin(db, h.AdminVendorGrowth)))
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

	mux.HandleFunc("POST /account/photo/remove", middleware.Logging(middleware.RequireLogin(db, h.RemoveProfilePhoto)))
	mux.HandleFunc("POST /account/delete", middleware.Logging(middleware.RequireLogin(db, h.DeleteAccount)))
	mux.HandleFunc("GET /account/deleted", middleware.Logging(h.AccountDeleted))
	mux.HandleFunc("POST /account/theme", middleware.Logging(middleware.RequireLogin(db, h.SaveTheme)))
	mux.HandleFunc("POST /account/password", middleware.Logging(middleware.RequireLogin(db, middleware.RateLimit(middleware.LoginIPLimiter, h.ChangePassword))))

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
	mux.HandleFunc(
		"POST /account/phone/firebase-verify",
		middleware.Logging(
			middleware.RequireLogin(db, h.FirebaseVerifyPhone),
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

	// Order matters: the old-address notice first, then headers, then the size limit, then the forged-request check.
	// The timeouts stop slow or stuck connections from tying up the server.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           alerts.Middleware(middleware.LegacyHost(middleware.SecurityHeaders(middleware.LimitBody(middleware.SameOrigin(mux))))),
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
			// Without this the program would wait for a shutdown signal while serving nothing
			// (for example when the port is already taken), and the host would never restart it.
			os.Exit(1)
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
