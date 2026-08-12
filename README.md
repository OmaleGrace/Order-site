# Food Ordering App

A full-stack food ordering website for a single restaurant, built with Go on the backend. Users can browse the menu, create an account, add items to a cart, place an order with real payment, and track order status.

## Status

🚧 Early development — database foundation completed. Currently connecting the Go backend to PostgreSQL.

## Planned Features

- Public menu browsing
- User accounts (signup/login)
- Cart and checkout
- Real payment integration
- Order status tracking (pending → preparing → out for delivery → delivered)
- Deployed publicly with a real domain and HTTPS

## Tech Stack

- **Backend:** Go
- **Database:** PostgreSQL 16
- **Database Environment:** Docker
- **Frontend:** HTML templates
- **Styling:** CSS
- **Payments:** TBD (likely Stripe)

## Current Progress

### Frontend Foundation ✅

- [x] Homepage created
- [x] Restaurant welcome section
- [x] Food item display
- [x] Login page
- [x] Signup page
- [x] CSS stylesheet linked
- [x] HTML templates connected to Go routes

### Database Foundation ✅

- [x] PostgreSQL installed
- [x] PostgreSQL 16 running in Docker
- [x] `food_ordering` database created
- [x] `food_app` PostgreSQL user created
- [x] `users` table created
- [x] `menu_items` table created
- [x] Sample menu items added
- [x] Practiced basic SQL:
  - `CREATE`
  - `INSERT`
  - `SELECT`
  - `UPDATE`
  - `DELETE`
  - `WHERE`
  - `ORDER BY`
- [x] PostgreSQL Go driver (`lib/pq`) installed
- [x] Initial Go database package created
- [ ] Complete Go → PostgreSQL connection

## Database Structure

```text
food_ordering
│
├── users
│   ├── id
│   ├── name
│   ├── email
│   └── password
│
└── menu_items
    ├── id
    ├── name
    ├── description
    └── price_kobo
