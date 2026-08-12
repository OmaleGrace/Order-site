# Food Ordering App

A full-stack food ordering website for a single restaurant, built with Go on the backend. Users will be able to browse the menu, create an account, add items to a cart, place an order with real payment, and track order status.

## Status

🚧 **Early development** — the initial Go web server, HTML templates, CSS setup, and basic authentication pages are complete. PostgreSQL setup is the next step.

## Completed So Far

* [x] Initialized Go project
* [x] Created basic Go HTTP server
* [x] Added HTML templates
* [x] Added static file serving
* [x] Added CSS stylesheet
* [x] Created restaurant homepage
* [x] Added `/login` route and login page
* [x] Added `/signup` route and signup page
* [x] Added navigation between authentication pages

> Authentication is currently only the frontend structure. User accounts and database-backed authentication will be implemented later.

## Planned Features

* Public menu browsing
* User accounts (signup/login)
* Cart and checkout
* Real payment integration
* Order status tracking (pending → preparing → out for delivery → delivered)
* Deployed publicly with a real domain and HTTPS

## Tech Stack

* **Backend:** Go
* **Database:** PostgreSQL
* **Frontend:** HTML templates + CSS
* **Payments:** TBD (likely Stripe)
* **Deployment:** TBD

## Roadmap

* [ ] **Milestone 0 — Database fundamentals**

  * [x] Initialize Go project
  * [x] Create basic Go HTTP server
  * [x] Learn HTML template rendering
  * [x] Serve static files
  * [x] Create basic website pages
  * [x] Create login and signup pages
  * [ ] Install and configure PostgreSQL
  * [ ] Learn Go + SQL basics
  * [ ] Connect Go application to PostgreSQL
  * [ ] Create initial database schema

* [ ] **Milestone 1 — Menu data model + read-only API**

  * [ ] Create menu database tables
  * [ ] Add menu items
  * [ ] Create menu page
  * [ ] Read menu data from PostgreSQL

* [ ] **Milestone 2 — User accounts**

  * [ ] Create users table
  * [ ] Implement signup
  * [ ] Hash passwords
  * [ ] Implement login
  * [ ] Add sessions/authentication
  * [ ] Protect authenticated routes

* [ ] **Milestone 3 — Cart + orders**

  * [ ] Create cart functionality
  * [ ] Add/remove menu items
  * [ ] Create orders
  * [ ] Store order items
  * [ ] Display order history

* [ ] **Milestone 4 — Real payment integration**

  * [ ] Choose payment provider
  * [ ] Create checkout flow
  * [ ] Process payments securely
  * [ ] Handle payment confirmation

* [ ] **Milestone 5 — Public deployment**

  * [ ] Prepare production configuration
  * [ ] Deploy application
  * [ ] Configure PostgreSQL in production
  * [ ] Add real domain
  * [ ] Configure HTTPS
  * [ ] Test production application

## Project Structure

```text
food-ordering/
├── go.mod
├── main.go
├── templates/
│   ├── home.html
│   ├── login.html
│   └── signup.html
└── static/
    └── css/
        └── style.css
```

## Getting Started

The project is currently under active development.

### Run the application

```bash
go run main.go
```

Then open:

```text
http://localhost:8080
```

Available pages:

```text
/          → Homepage
/login     → Login page
/signup    → Signup page
```

## License

TBD
