# Grace's Kitchen — Full-Stack Food Ordering App

A full-stack food ordering web application for a single restaurant, built with Go and PostgreSQL. The application supports customer accounts, menu browsing, cart management, Paystack payments, order tracking, and protected admin order management.

## Live Project

**Production:** Deployed on Render

## Features

### Customer

* Create an account and log in securely
* Password hashing with bcrypt
* Browse the restaurant menu
* View food images, descriptions, and prices
* Add menu items to a cart
* Increase or decrease item quantities
* Remove items from the cart
* Checkout and create orders
* Pay using Paystack test-mode payments
* Server-side payment verification
* View previous orders
* Track order status
* Secure logout

### Admin

* Automatic admin detection after login
* Protected admin-only dashboard
* View customer information and orders
* View ordered items and quantities
* View order totals and dates
* Update order status

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

### Payments

* Paystack API
* Payment callback verification
* Payment webhook verification
* Server-side amount validation

### Security

* bcrypt password hashing
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
   ├── Sign Up
   │
   ├── Login
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
   ├── Payment Verification
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
   └── Update Order Status
```

## Database Structure

The application uses PostgreSQL with separate tables for the main application data:

* `users`
* `menu_items`
* `cart_items`
* `orders`
* `order_items`
* `sessions`

Orders and order items are stored separately so that an order keeps a record of the items and prices associated with that purchase.

## Authentication & Sessions

User passwords are hashed with bcrypt before being stored.

After login, the application creates a random server-side session token. The browser receives the token through a secure cookie, while the actual user identity remains stored on the server.

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

A Paystack webhook is also implemented for payment event handling.

## Testing

The project includes automated tests covering major application areas, including:

* Authentication
* Signup
* Login
* Logout
* Sessions
* Admin authorization
* Cart operations
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

Run static analysis with:

```bash
go vet ./...
```

## Local Development

### 1. Clone the repository

```bash
git clone <https://github.com/OmaleGrace/Order-site>
cd Order-site
```


### 2. Run the application

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
* Production menu data
* Remote food image URLs

The same Go application can run locally or in production through environment-based configuration.

## Project Status

**Completed and deployed.**

The core customer ordering flow, payment processing, order tracking, admin management, authentication, security improvements, testing, and production deployment are implemented.

## What I Learned

This project provided hands-on experience with:

* Building web applications with Go
* HTTP routing and middleware
* PostgreSQL database design
* SQL queries and relationships
* Authentication and session management
* Password hashing
* Payment API integration
* Webhooks
* Transactional order creation
* Automated testing
* Docker-based development
* Environment configuration
* Cloud deployment with Render
* Debugging production issues
* Basic web application security
