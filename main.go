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

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	db, err := database.Connect()
	if err != nil {
		fmt.Println("Database connection failed:", err)
		return
	}
	defer db.Close()

	h := handlers.New(db)

	http.HandleFunc("/", middleware.Logging(h.Home))
	http.HandleFunc("/login", middleware.Logging(h.Login))
	http.HandleFunc("/signup", middleware.Logging(h.Signup))

	http.HandleFunc("/menu", middleware.Logging(h.Menu))

	http.HandleFunc(
		"/cart/add",
		middleware.Logging(h.AddToCart),
	)

	http.HandleFunc(
		"/cart",
		middleware.Logging(h.Cart),
	)

	http.HandleFunc(
		"/cart/remove",
		middleware.Logging(h.RemoveFromCart),
	)

	http.HandleFunc(
		"/checkout",
		middleware.Logging(h.Checkout),
	)

	http.HandleFunc(
		"/payment/callback",
		middleware.Logging(h.PaymentCallback),
	)

	http.HandleFunc(
		"/order-success",
		middleware.Logging(h.OrderSuccess),
	)

	// Admin dashboard
	http.HandleFunc(
		"/admin/orders",
		middleware.Logging(
			middleware.Admin(db, h.AdminOrders),
		),
	)

	// Admin order status update
	http.HandleFunc(
		"/admin/orders/status",
		middleware.Logging(
			middleware.Admin(db, h.UpdateOrderStatus),
		),
	)

	srv := &http.Server{
		Addr: ":8080",
	}

	go func() {
		fmt.Println("Server Running on https://localhost:8080")

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