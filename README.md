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
* Uses `OPENAI_API_KEY` on the server (or `GEMINI_API_KEY` as a fallback); no key is sent to the browser
* Vendors must review and manually publish drafts; generation does not alter live menu content
* Configure `OPENAI_MODEL` optionally; defaults to `gpt-4o-mini`

### Vendor delivery areas

* Vendors configure delivery coverage as one `State | LGA` entry per line in Shop Profile
* Nigerian states are validated and canonicalised
* Delivery checkout rejects state/LGA combinations that the selected vendor has not configured
* Vendors must configure at least one delivery area before enabling delivery

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
go test -p 1 ./...
```

`-p 1` runs one package at a time, because every database test resets the same test database. Test databases are built the way the app builds its own at start-up (schema plus every migration), so tests see the real tables. `go test ./render` (which checks that every page template parses) works without a database.

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

Copy `.env.example` to `.env`, then set `DATABASE_URL` to a PostgreSQL connection URL (a hosted database works, so you do not need to install PostgreSQL locally). Fill in only the services you need; payments, email, SMS, image upload and AI are all optional while developing.

When `DATABASE_URL` points to a truly empty database, the application bootstraps the schema from `db/schema.sql` (embedded in the binary) before running additive migrations. Existing databases are not reset or overwritten.

### 3. Run the application

Use the Go version required by `go.mod` (Go 1.26.0):

```bash
go run .
```

The server runs locally on:

```text
http://localhost:8080
```

HTML templates are embedded in the binary, so they are parsed once at start-up and the app runs from any directory. While editing templates, skip the rebuild step by reading them from disk on every request:

```bash
TEMPLATES_DIR=templates go run .
```

## Delivery Pricing, Statements and Settlements

* **Delivery pricing (vendors):** `/vendor/delivery` lets a shop set, for each area it serves, its own delivery fee, a minimum food order, and an amount above which delivery is free. A blank fee means "use the shop's default delivery fee". Prices live in `vendor_delivery_rates`, separate from the list of areas, so editing the shop profile never wipes them.
* **Checkout:** the server applies those rules when an order is placed (`resolveDeliveryFee`); the checkout page previews the same fee and blocks the button when the minimum order is not met.
* **Statements (vendors):** `/vendor/statements` shows a period statement with a CSV export at `/vendor/statements/export`.
* **Settlements (main admin only):** `/admin/settlements` is the platform-wide report with a CSV export at `/admin/settlements/export`. Shops with nothing to report in the period are left out.

## Project Structure

```text
main.go            routes and server start-up
handlers/          HTTP handlers, one file per feature
render/            template loader: parses each page once, with shared partials
templates/         page templates (*.html)
templates/partials shared pieces: <head>, admin/vendor navigation, common scripts
static/css/        stylesheets (page-specific ones live in static/css/pages/)
static/js/         scripts (page-specific ones live in static/js/pages/)
db/                schema.sql (embedded) and seed data
middleware/        security headers, rate limiting, sessions, logging
errors/            the branded error page
```

### Working with templates

* Every page starts with `{{template "head-common"}}` (charset, viewport, icons, theme colour) and ends with `{{template "scripts-common"}}`.
* Private pages (accounts, vendor and admin tools) add `{{template "meta-noindex"}}`; public pages add a description with `{{template "meta-desc" "…"}}`.
* The admin and vendor menus are single partials. To change a menu link, edit `templates/partials/nav.html` once. A page picks its highlighted link with `{{template "admin-nav" dict "Class" "portal-nav" "Active" "/admin/orders"}}`.
* Keep CSS and JavaScript in `static/`, not inline in templates.
* Use `data-confirm="Delete this item?"` (optionally with `data-confirm-title`, `data-confirm-ok`, `data-confirm-tone="danger"`) on a form or submit button for a styled confirmation dialog instead of `confirm()`.
* Forms are protected against double submission automatically. Add `data-allow-resubmit` to a form to opt out.
* For errors on a normal page, call `errors.Render(w, "friendly message", status)` so visitors see the branded error page. Keep plain `http.Error` / JSON responses for background (fetch) endpoints.

## Troubleshooting

* **A page shows an error once, then works when refreshed.** With a hosted database behind a connection pooler (PgBouncer, Supabase, Neon) the log shows `unnamed prepared statement does not exist (26000)`. The app adds `binary_parameters=yes` to `DATABASE_URL` automatically to avoid this. To opt out, put `binary_parameters=no` in the URL yourself.
* **Slow pages with a hosted database.** Each page makes several queries, so a database far from the server adds delay to every one of them. Run the app in the same region as the database when you can.
* **The AI writing studio stops mid-answer.** Gemini 2.5 models count hidden "thinking" against the output limit. The app turns thinking off for the 2.5 Flash models, asks again with a bigger limit if an answer is cut off, and never shows half a sentence. If you set `GEMINI_MODEL` to a Pro model, thinking stays on and the larger limit is used on the retry.

## Security Notes

* Never commit `.env`, and never share it in a zip, chat or screenshot. `.gitignore` and `.dockerignore` exclude it, but a hand-made archive does not respect them: build zips from tracked files only (for example `git archive -o site.zip HEAD`).
* If a secret is ever exposed, rotate it at the provider (Paystack, Google OAuth, Brevo, Termii, Cloudinary, AI keys, database password) rather than just deleting the file.
* Use Paystack test keys outside production.

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


## Vendor AI and vendor payout accounts

- The `/vendor/ai` page and its generation endpoint require a valid login session and an owned vendor shop. The AI assistant includes a customer-chat reply drafter: vendors paste a customer message and confirmed shop facts, then review the generated draft and send it themselves. This is not a live messaging system and does not send messages automatically.
- Vendor AI supports menu descriptions, social captions, menu idea brainstorming, WhatsApp broadcast drafts, three-post campaign plans, customer-review replies, and pricing worksheets. Configure `OPENAI_API_KEY` and optionally `OPENAI_MODEL` in the private environment to enable generation.
- Vendors can add a Nigerian bank account from Shop Profile. ENIE checks the bank code/name against Paystack's bank directory and resolves the account name before marking it verified. Configure `PAYSTACK_SECRET_KEY` privately.
- Only the configured main administrator (`omalegrace2009@gmail.com`) can view full payout account numbers on Admin Earnings. The payout form records a payment after the administrator has sent it; it does **not** automatically transfer money. The administrator must send the transfer through their bank or a supported payout service, then record the amount in ENIE.
- Never commit `.env` or expose `OPENAI_API_KEY` / `PAYSTACK_SECRET_KEY` in frontend JavaScript or public repositories.

## Customer-to-vendor messaging

ENIE includes authenticated customer/vendor conversations at `/messages`. Customers can start a conversation from an active shop's public page; shop owners can reply from their message inbox. Messages are stored in PostgreSQL, the conversation page checks that the signed-in user is either the customer or the owner of the shop, and new messages are fetched every five seconds while the conversation is open. Message bodies are plain text and limited to 2,000 characters. Existing databases receive the `message_threads` and `messages` tables automatically during startup. Keep this feature behind HTTPS in production and never ask customers to share passwords, PINs, OTPs, or complete payment credentials in chat.
