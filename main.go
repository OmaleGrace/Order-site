package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Order-site/database"
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

	// Create our own router.
	mux := http.NewServeMux()

	// Static files
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", fs),
	)

	mux.HandleFunc("GET /{$}", middleware.Logging(h.Home))
	mux.HandleFunc("GET /login", middleware.Logging(h.Login))
	mux.HandleFunc("POST /login", middleware.Logging(h.Login))

	mux.HandleFunc("GET /signup", middleware.Logging(h.Signup))
	mux.HandleFunc("POST /signup", middleware.Logging(h.Signup))

	mux.HandleFunc("GET /logout", middleware.Logging(h.Logout))

	mux.HandleFunc("GET /menu", middleware.Logging(h.Menu))

	mux.HandleFunc(
		"POST /cart/add",
		middleware.Logging(
			middleware.RequireLogin(db, h.AddToCart),
		),
	)

	mux.HandleFunc(
		"GET /cart",
		middleware.Logging(
			middleware.RequireLogin(db, h.Cart),
		),
	)

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

	mux.HandleFunc(
		"GET /orders",
		middleware.Logging(
			middleware.RequireLogin(db, h.MyOrders),
		),
	)

	mux.HandleFunc(
		"GET /admin/orders",
		middleware.Logging(
			middleware.Admin(db, h.AdminOrders),
		),
	)

	mux.HandleFunc(
		"POST /admin/orders/status",
		middleware.Logging(
			middleware.Admin(db, h.UpdateOrderStatus),
		),
	)

	// HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":8080",
		Handler: middleware.SecurityHeaders(mux),
	}

	go func() {
		fmt.Println("Server Running on http://localhost:8080")

		if err := srv.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			fmt.Println("Server error:", err)
		}
	}()

	// Wait for shutdown signal
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
