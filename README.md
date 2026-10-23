# ENIE — Food Marketplace Platform

A Go and PostgreSQL food marketplace connecting customers with participating food vendors. The application supports customer accounts (including Google sign-in), vendor shops and menus, search and category filters, cart management, Paystack payments, email notifications, order tracking, vendor tools, and protected admin management.

## Live Project

**Production:** Deployed on Render

## Features

### Customer

* Create an account and log in securely
* Sign up / log in manually, or continue with Google (OAuth)
* Password hashing with bcrypt for manual accounts
* Forgot password / reset password via emailed, time-limited reset link
* Browse the restaurant menu (login required)
* Search menu items by name or description
* Filter menu items by category (Rice & Swallow, Proteins & Sides, Soups & Stews, Drinks)
* View food images, descriptions, and prices
* Add menu items to a cart
* Increase or decrease item quantities
* Remove items from the cart
* Cart shows item images alongside details
* Checkout and create orders
* Pay using Paystack test-mode payments
* Server-side payment verification
* Email notifications: welcome email on signup, payment confirmation, order status updates
* View previous orders with a visual order-status tracker
* Track order status
* Secure logout

### Admin

* Responsive admin overview with platform totals and organised navigation cards
* View and search all registered users, including join date, role, shop count, and order count
* Main-admin-only user deletion; the main admin account is protected from deletion
* Deleting a user suspends and detaches any shop they owned, while related account data is removed according to database relationships
* Manage orders, vendors, menu items, earnings, payouts, and vendor applications
* View ordered items and quantities, totals, dates, and delivery information
* Update order status (customer is notified by email automatically)

### Vendor AI writing assistant

* Optional AI-powered menu descriptions and social media promotional captions
* Uses `OPENAI_API_KEY` on the server; no key is sent to the browser
* Vendors must review and manually publish drafts; generation does not alter live menu content
* Configure `OPENAI_MODEL` optionally; defaults to `gpt-4o-mini`

### Vendor delivery areas

* Vendors configure delivery coverage as one `State | LGA` entry per line in Shop Profile
* Nigerian states are validated and canonicalised
* Delivery checkout rejects state/LGA combinations that the selected vendor has not configured
* Vendors must configure at least one delivery area before enabling delivery

## Main administrator setup

The main administrator email configured for user-management access is `omalegrace2009@gmail.com`. For safety, that account must already have `users.is_admin = TRUE` in the production database; the app does not promote an account based on email alone. If it is not already an admin, set the flag for the existing account through your database provider's SQL console:

```sql
UPDATE users SET is_admin = TRUE WHERE LOWER(email) = 'omalegrace2009@gmail.com';
```

Keep database credentials private and do not commit a `.env` file. The app creates the `vendor_delivery_areas` table automatically when it connects to PostgreSQL. See [`DATABASE_SETUP.md`](DATABASE_SETUP.md) for instructions to use hosted PostgreSQL without installing it locally, and copy `.env.example` to `.env` as a starting point.

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
* Consistent branded design system across home, menu, cart, orders, order-success, auth, and error pages

### Authentication

* Manual signup/login with bcrypt password hashing
* Google OAuth 2.0 ("Continue with Google") via `golang.org/x/oauth2`
* Forgot password / reset password flow with time-limited, single-use tokens
* Server-side session tokens

### Payments

* Paystack API
* Payment callback verification
* Payment webhook verification
* Server-side amount validation

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
   ├── Login (manual or Google) / Forgot Password
   │
   ├── Browse Menu (search + category filters)
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
   └── Track Orders (status stepper)


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


## Authentication & Sessions

User passwords are hashed with bcrypt before being stored, for accounts created manually.

Customers can alternatively sign up or log in with **Google OAuth** — Google-authenticated accounts are created with `auth_provider = 'google'` and no password, since Google handles authentication. Existing accounts are matched by email, so a customer who originally signed up manually and later uses Google (or vice versa) with the same email is logged into the same account. A Google-only customer can also set a password via the forgot-password flow, giving them a manual login option too.

If a customer forgets their password, they can request a reset link from `/forgot-password`. A random, single-use token is generated and emailed with a link valid for 15 minutes. To avoid revealing which emails are registered, the same confirmation message is shown whether or not the email exists in the system.

After login (either method), the application creates a random server-side session token. The browser receives the token through a secure cookie, while the actual user identity remains stored on the server.

Sessions also have an expiration time and expired sessions are cleaned up.

## Menu Search & Filtering

The menu supports searching by name/description and filtering by category, both via URL query parameters (`/menu?search=...&category=...`) so results are shareable and linkable. Search and category filters can be combined. Menu browsing is public for discovery; adding items to the cart and checkout remain login-protected.

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

Transactional emails are sent via the Brevo API at these points in the customer journey:

* **Signup** — a welcome email, sent regardless of whether the account was created manually or via Google
* **Password reset** — a time-limited reset link
* **Payment confirmation** — sent once a payment is verified and the order is marked as paid
* **Order status updates** — sent whenever an admin changes an order's status from the admin dashboard

Emails are sent in background goroutines so a slow or failed email delivery never blocks or delays the underlying signup, payment, or status-update action. Send failures are logged server-side.

## Design

Every customer-facing page (home, menu, cart, orders, order-success, login/signup, forgot/reset password, and error pages) shares a consistent visual identity: a branded header/nav, an orange (`#d35400`) accent color, card-based layouts with hover states, and a responsive layout down to mobile. The homepage features a hero section with a live preview of menu items pulled directly from the database. Errors are rendered through a shared, reusable `errors` package instead of raw browser error text.

## Testing

The project includes automated tests covering major application areas, including:

* Authentication (manual)
* Signup
* Login
* Logout
* Sessions
* Admin authorization
* Cart operations (add, remove, update quantity)
* Menu loading, search, and category filtering
* Checkout and vendor delivery-area restrictions
* Vendor AI draft generation input validation
* Database schema bootstrap helper
* Payment callbacks
* Payment webhooks
* Orders
* Admin order management
* HTTP handlers
* Middleware

The integration tests require a dedicated local PostgreSQL test database (see `TEST_DATABASE_URL` in `testutil/testutil.go`). Never point tests at production data. Run the complete test suite with:

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

### 1. Clone or extract the project

```bash
git clone https://github.com/OmaleGrace/Order-site
cd Order-site
```

### 2. Configure environment variables and PostgreSQL

Copy `.env.example` to `.env`, then set `DATABASE_URL` to a working hosted PostgreSQL connection URL. This lets you develop without installing PostgreSQL or Docker locally. Read [`DATABASE_SETUP.md`](DATABASE_SETUP.md) for the setup steps. Never commit `.env` or share its secrets.

When `DATABASE_URL` points to a truly empty database, the application bootstraps the schema from `schema.sql` before running additive migrations. Existing databases are not reset or overwritten.

### 3. Run the application

Use the Go version required by `go.mod` (Go 1.26.0):

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
* Production menu data (19 menu items across 4 categories, with images)
* Remote food image URLs

The same Go application can run locally or in production through environment-based configuration.

## Project Status

**Actively being improved.** The existing marketplace and ordering flows are in place. Current work includes delivery-area enforcement, stronger first-run database setup, ENIE branding, administrator user management, vendor UX improvements, and the optional AI writing assistant. A full release should only be marked verified after running the complete test suite against a dedicated test database and exercising the deployed checkout/payment flow.

## What I Learned

This project provided hands-on experience with:

* Building web applications with Go
* HTTP routing and middleware
* PostgreSQL database design
* SQL queries and relationships (including dynamic, conditionally-built queries for search/filtering)
* Authentication and session management
* Password hashing
* OAuth 2.0 (Google Sign-In) integration
* Secure password-reset token flows
* Payment API integration
* Webhooks
* Transactional email integration
* Transactional order creation
* Automated testing
* Docker-based development
* Environment configuration
* Cloud deployment with Render
* Debugging production issues (including OAuth redirect URI mismatches, browser-caching pitfalls, and local vs. production database drift)
* Basic web application security
* Building a consistent design system across an entire application

