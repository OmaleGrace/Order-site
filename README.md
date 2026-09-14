# Grace's Kitchen — Full-Stack Food Ordering App

A full-stack food ordering web application for a single restaurant, built with Go and PostgreSQL. The application supports customer accounts (including Google sign-in), menu browsing, cart management, Paystack payments, email notifications, order tracking, and protected admin order management.

## Live Project

**Production:** Deployed on Render

## Features

### Customer

* Create an account and log in securely
* Sign up / log in manually, or continue with Google (OAuth)
* Password hashing with bcrypt for manual accounts
* Browse the restaurant menu
* View food images, descriptions, and prices
* Add menu items to a cart
* Increase or decrease item quantities
* Remove items from the cart
* Checkout and create orders
* Pay using Paystack test-mode payments
* Server-side payment verification
* Email notifications: welcome email on signup, payment confirmation, order status updates
* View previous orders
* Track order status
* Secure logout

### Admin

* Automatic admin detection after login
* Protected admin-only dashboard
* View customer information and orders
* View ordered items and quantities
* View order totals and dates
* Update order status (customer is notified by email automatically)

## Tech Stack

### Backend

* Go
* `net/http`
* `database/sql`
* HTML templates

### Database

* PostgreSQL

### Frontend

* HTML5
* CSS3
* Go templates
* Responsive design

### Authentication

* Manual signup/login with bcrypt password hashing
* Google OAuth 2.0 ("Continue with Google") via `golang.org/x/oauth2`
* Server-side session tokens

### Payments

* Paystack API
* Payment callback verification
* Payment webhook verification
* Server-side amount validation

### Email

* Brevo transactional email API
* Asynchronous sending (background goroutines) so email delivery never blocks the user-facing request
* Triggers: signup welcome email, payment confirmation, order status change

### Security

* bcrypt password hashing (manual accounts)
* Google OAuth for password-less sign-in
* Server-side session tokens
* Secure, HttpOnly cookies
* SameSite cookie protection
* Protected customer routes
* Protected admin routes
* Security headers
* HTTP method-specific routing

### Deployment & Tools

* Render
* Docker
* Git
* GitHub
* Linux
* GitHub Codespaces

## Application Flow

```text
Customer
   │
   ├── Sign Up (manual or Google) ── Welcome email sent
   │
   ├── Login (manual or Google)
   │
   ├── Browse Menu
   │
   ├── Add Items to Cart
   │
   ├── Manage Quantities
   │
   ├── Checkout
   │
   ├── Pay with Paystack
   │
   ├── Payment Verification ── Payment confirmation email sent
   │
   └── Track Orders


Admin
   │
   ├── Login
   │
   ├── Automatic Admin Detection
   │
   ├── View Orders
   │
   └── Update Order Status ── Status update email sent to customer
```

## Database Structure

The application uses PostgreSQL with separate tables for the main application data:

* `users` (includes `auth_provider` to distinguish manual vs. Google accounts; `password` is nullable for Google accounts)
* `menu_items`
* `cart_items`
* `orders`
* `order_items`
* `sessions`

Orders and order items are stored separately so that an order keeps a record of the items and prices associated with that purchase.

## Authentication & Sessions

User passwords are hashed with bcrypt before being stored, for accounts created manually.

Customers can alternatively sign up or log in with **Google OAuth** — Google-authenticated accounts are created with `auth_provider = 'google'` and no password, since Google handles authentication. Existing accounts are matched by email, so a customer who originally signed up manually and later uses Google (or vice versa) with the same email is logged into the same account.

After login (either method), the application creates a random server-side session token. The browser receives the token through a secure cookie, while the actual user identity remains stored on the server.

Sessions also have an expiration time and expired sessions are cleaned up.

## Payment Processing

The application integrates Paystack for payment processing.

The payment flow includes:

1. Create a pending order.
2. Generate a unique payment reference.
3. Initialize the Paystack transaction.
4. Redirect the customer to Paystack.
5. Receive the payment callback.
6. Verify the transaction server-side.
7. Confirm the payment amount matches the order total.
8. Mark the order as paid.
9. Clear the customer's cart.
10. Send a payment confirmation email to the customer.

A Paystack webhook is also implemented for payment event handling, and triggers the same email confirmation.

## Email Notifications

Transactional emails are sent via the Brevo API at three points in the customer journey:

* **Signup** — a welcome email, sent regardless of whether the account was created manually or via Google
* **Payment confirmation** — sent once a payment is verified and the order is marked as paid
* **Order status updates** — sent whenever an admin changes an order's status from the admin dashboard

Emails are sent in background goroutines so a slow or failed email delivery never blocks or delays the underlying signup, payment, or status-update action. Send failures are logged server-side.

## Testing

The project includes automated tests covering major application areas, including:

* Authentication (manual)
* Signup
* Login
* Logout
* Sessions
* Admin authorization
* Cart operations (add, remove, update quantity)
* Menu loading
* Checkout
* Payment callbacks
* Payment webhooks
* Orders
* Admin order management
* HTTP handlers
* Middleware

Run the complete test suite with:

```bash
go test ./...
```

Run with coverage:

```bash
go test ./... -cover
```

Run static analysis with:

```bash
go vet ./...
```

## Local Development

### 1. Clone the repository

```bash
git clone https://github.com/OmaleGrace/Order-site
cd Order-site
```

### 2. Set up environment variables

Create a `.env` file with:

```
DATABASE_URL=postgres://food_app:food_password@localhost:5432/food_ordering?sslmode=disable
PAYSTACK_SECRET_KEY=your_paystack_secret_key
PAYSTACK_CALLBACK_URL=http://localhost:8080/payment/callback
BREVO_API_KEY=your_brevo_api_key
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
```

### 3. Run the application

```bash
go run .
```

The server runs locally on:

```text
http://localhost:8080
```

## Deployment

The application is deployed on Render with:

* Production PostgreSQL
* Environment-based configuration
* Render-provided `PORT`
* HTTPS
* Paystack integration
* Brevo email integration
* Google OAuth integration (separate authorized redirect URI registered for production)
* Production menu data
* Remote food image URLs

The same Go application can run locally or in production through environment-based configuration.

## Project Status

**Completed and deployed.**

The core customer ordering flow, payment processing, email notifications, Google OAuth sign-in, order tracking, admin management, authentication, security improvements, testing, and production deployment are all implemented.

## What I Learned

This project provided hands-on experience with:

* Building web applications with Go
* HTTP routing and middleware
* PostgreSQL database design
* SQL queries and relationships
* Authentication and session management
* Password hashing
* OAuth 2.0 (Google Sign-In) integration
* Payment API integration
* Webhooks
* Transactional email integration
* Transactional order creation
* Automated testing
* Docker-based development
* Environment configuration
* Cloud deployment with Render
* Debugging production issues (including OAuth redirect URI and browser-caching pitfalls)
* Basic web application security