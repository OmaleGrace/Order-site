# Food Ordering App

A full-stack food ordering website for a single restaurant, built with Go on the backend. Users can browse the menu, create an account, add items to a cart, place an order with real payment, and track order status.

## Status

🚀 **Live and deployed** at [grace-ordering-site.onrender.com](https://grace-ordering-site.onrender.com) — menu browsing, signup, login, cart, and checkout (order creation) are all working in production.

✅ **Production-readiness pass complete** — password hashing, persistent cart storage, clean handler structure, request logging, and graceful shutdown are all in place.

## Planned Features

- [x] Public menu browsing
- [x] User accounts (signup/login)
- [x] Cart (persisted to database)
- [x] Checkout — creates a real order and order items in the database
- [ ] Real payment integration
- [ ] Order status tracking (pending → preparing → out for delivery → delivered)
- [x] Deployed publicly (Render, free tier)
- [ ] Custom domain (currently using Render's default `.onrender.com` domain, which is already HTTPS)

## Tech Stack

- **Backend:** Go
- **Database:** PostgreSQL 16
- **Local Dev Database:** Dockerized PostgreSQL container
- **Production Database:** Render PostgreSQL (Frankfurt region)
- **Config Management:** `.env` file + `godotenv` (git-ignored, never committed)
- **Hosting:** Render (Web Service)
- **Frontend:** HTML templates
- **Styling:** CSS, dedicated stylesheet per page (`home.css`, `login.css`/`auth.css`, `signup.css`/`auth.css`, `style.css` shared base)
- **Payments:** TBD (likely Stripe)

## Current Progress

### Frontend Foundation ✅

- [x] Homepage created, with its own dedicated stylesheet (`home.css`) for a proper landing-page feel
- [x] Restaurant welcome section
- [x] Food item display
- [x] Login page — with show/hide password toggle
- [x] Signup page — with show/hide password toggle
- [x] Cart page — shows quantity per item and running total, with a checkout button
- [x] Dedicated CSS stylesheets linked per page
- [x] HTML templates connected to Go routes

### Database Foundation ✅

- [x] PostgreSQL installed locally
- [x] PostgreSQL 16 running in Docker for local dev
- [x] `food_ordering` local database created
- [x] `food_app` local PostgreSQL user created
- [x] `users`, `menu_items`, `cart_items`, `orders`, and `order_items` tables created (local + production)
- [x] Sample menu items added (local + production)
- [x] Practiced basic SQL: `CREATE`, `INSERT`, `SELECT`, `UPDATE`, `DELETE`, `WHERE`, `ORDER BY`
- [x] PostgreSQL Go driver (`lib/pq`) installed

### Backend Connection ✅

- [x] Go database package created (`database.Connect()`)
- [x] Go successfully connects to Postgres via `DATABASE_URL`
- [x] `.env` support added via `godotenv`, loaded at app startup
- [x] `.gitignore` added — `.env` never committed
- [x] Local setup uses `sslmode=disable` (Docker Postgres has no SSL); production uses Render's internal connection with SSL enabled by default
- [x] Same code works in both environments — only the `DATABASE_URL` value changes per environment
- [x] Fixed static file serving bug (`/static` → `/static/` prefix mismatch was silently breaking all CSS/JS)

### Deployment ✅

- [x] Render PostgreSQL database created (Frankfurt region)
- [x] Render Web Service created and connected to GitHub repo
- [x] `DATABASE_URL` (Internal Render URL) set as environment variable on the web service
- [x] Production schema applied (dumped from local via `pg_dump`, applied via `psql`)
- [x] Production menu data seeded
- [x] Live site verified end-to-end: menu, signup, login, cart, and checkout all confirmed working in production

### Production Readiness ✅ (complete)

- [x] **Password hashing with bcrypt** — passwords are hashed with `golang.org/x/crypto/bcrypt` on signup and verified with `bcrypt.CompareHashAndPassword` on login; no plain-text passwords stored
- [x] **Cart persisted to the database** — replaced the in-memory `map[int]*cart.Cart` with a `cart_items` table (tracks quantity, price at time of adding, timestamps), so carts survive restarts/redeploys
- [x] **Split into a `handlers` package** — routing logic now lives in `main.go` only; `homeHandler`, `loginHandler`/`signupHandler`, `menuHandler`, cart handlers, and `checkoutHandler` each moved into their own file under `handlers/`, as methods on a shared `Handlers` struct holding the DB connection
- [x] **Request logging middleware** — every route is wrapped with a `middleware.Logging` handler that logs method, path, and duration for each request, visible locally and in Render's logs
- [x] **Static asset paths audited** — confirmed every template's `<link>`/`<script>` tag correctly uses the `/static/...` path
- [x] **Graceful shutdown** — replaced the blocking `http.ListenAndServe` call with an `http.Server` + signal handling, so the app finishes in-flight requests (up to a 10s timeout) instead of dying instantly on redeploy/restart

### Known Gaps

- [ ] No real payment integration yet — checkout currently creates an order and order items in the database, but doesn't process actual payment
- [ ] No order status tracking/updates yet (orders are created with status `pending` and don't move forward)
- [ ] Free-tier Render instance spins down after inactivity (first request after idle can take ~50s)
- [ ] Free-tier Render Postgres database has a limited lifespan unless upgraded

## Database Structure

```text
food_ordering
│
├── users
│   ├── id (serial, primary key)
│   ├── name
│   ├── email (unique)
│   └── password (bcrypt hash)
│
├── menu_items
│   ├── id (serial, primary key)
│   ├── name
│   ├── description
│   └── price_kobo
│
├── cart_items
│   ├── id (serial, primary key)
│   ├── user_id → users.id
│   ├── menu_item_id → menu_items.id
│   ├── quantity
│   ├── price_kobo_at_addition
│   ├── special_instructions
│   ├── created_at
│   └── updated_at
│   (unique per user_id + menu_item_id)
│
├── orders
│   ├── id (serial, primary key)
│   ├── user_id → users.id
│   ├── total_kobo
│   └── status
│
└── order_items
    ├── order_id → orders.id
    ├── menu_item_id → menu_items.id
    ├── quantity
    └── price_kobo
```

## Local Development Setup

1. Start the local Postgres Docker container:
   ```bash
   docker start food-ordering-db
   ```
2. Ensure `.env` exists in the project root with:
   ```
   DATABASE_URL=postgres://food_app:food_password@localhost:5432/food_ordering?sslmode=disable
   ```
3. Run the app:
   ```bash
   go run .
   ```
4. To stop it cleanly, press `Ctrl+C` — the app will finish in-flight requests and shut down gracefully instead of dying instantly.

## Deployment Notes

- Hosted on Render as a **Web Service**, connected directly to this GitHub repo (`main` branch)
- Build command: `go build -o app .`
- Start command: `./app`
- Production `DATABASE_URL` is set via Render's Environment Variables (Internal Database URL), separate from the local `.env` file
- Schema and seed data were applied to production manually via `pg_dump` (local) → `psql` (Render, External Database URL)