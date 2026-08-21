# Food Ordering App

A full-stack food ordering website for a single restaurant, built with Go on the backend. Users can browse the menu, create an account, add items to a cart, place an order with real payment, and track order status.

## Status

🚀 **Live and deployed** at [grace-ordering-site.onrender.com](https://grace-ordering-site.onrender.com) — menu browsing, signup, and login are working in production. Cart, checkout, payments, and order tracking are still to come.

🔧 **Currently hardening the codebase** — working through a list of production-readiness improvements, starting with password hashing.

## Planned Features

- [x] Public menu browsing
- [x] User accounts (signup/login)
- [ ] Cart and checkout
- [ ] Real payment integration
- [ ] Order status tracking (pending → preparing → out for delivery → delivered)
- [x] Deployed publicly (Render, free tier)
- [ ] Custom domain + HTTPS (currently using Render's default `.onrender.com` domain, which is already HTTPS)

## Tech Stack

- **Backend:** Go
- **Database:** PostgreSQL 16
- **Local Dev Database:** Dockerized PostgreSQL container
- **Production Database:** Render PostgreSQL (Frankfurt region)
- **Config Management:** `.env` file + `godotenv` (git-ignored, never committed)
- **Hosting:** Render (Web Service)
- **Frontend:** HTML templates
- **Styling:** CSS (per-page stylesheets, e.g. `login.css`, `signup.css`)
- **Payments:** TBD (likely Stripe)

## Current Progress

### Frontend Foundation ✅

- [x] Homepage created
- [x] Restaurant welcome section
- [x] Food item display
- [x] Login page — with show/hide password toggle
- [x] Signup page — with show/hide password toggle
- [x] Dedicated CSS stylesheets linked per page
- [x] HTML templates connected to Go routes

### Database Foundation ✅

- [x] PostgreSQL installed locally
- [x] PostgreSQL 16 running in Docker for local dev
- [x] `food_ordering` local database created
- [x] `food_app` local PostgreSQL user created
- [x] `users` table created (local + production)
- [x] `menu_items` table created (local + production)
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
- [x] Live site verified: menu loads, signup creates real users in the production database

### Production Readiness — In Progress 🔧

- [ ] **Password hashing with bcrypt** *(in progress)* — passwords are currently stored in plain text; `golang.org/x/crypto/bcrypt` has been installed and hashing is being wired into signup/login
- [ ] Move cart storage from in-memory (`map[int]*cart.Cart` in `main.go`) to the database, so carts survive restarts/redeploys
- [ ] Split `main.go` into separate handler files (e.g. `handlers/auth.go`, `handlers/menu.go`, `handlers/cart.go`) as the app grows
- [ ] Add request logging middleware for visibility into production traffic and easier debugging
- [ ] Audit all static asset paths for consistency now that the `/static/` routing bug is fixed
- [ ] Add graceful shutdown handling to `http.ListenAndServe`

### Other Known Gaps

- [ ] No checkout or payment flow yet
- [ ] No order tracking/status system yet
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
│   └── password (⚠️ currently plain text — bcrypt hashing in progress)
│
└── menu_items
    ├── id (serial, primary key)
    ├── name
    ├── description
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

## Deployment Notes

- Hosted on Render as a **Web Service**, connected directly to this GitHub repo (`main` branch)
- Build command: `go build -o app .`
- Start command: `./app`
- Production `DATABASE_URL` is set via Render's Environment Variables (Internal Database URL), separate from the local `.env` file
- Schema and seed data were applied to production manually via `pg_dump` (local) → `psql` (Render, External Database URL)