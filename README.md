# Grace's Kitchen – Full-Stack Food Ordering App

A full-stack food ordering website for a restaurant, built with Go and PostgreSQL.

Customers can create an account, browse the menu, add meals to their cart, make payments through Paystack, and track their orders. An admin dashboard allows authorized users to view and manage customer orders.

## Features

### Customer Features

- User registration
- Secure password hashing with bcrypt
- User login and logout
- Restaurant menu
- Food images
- Add items to cart
- Remove items from cart
- Checkout
- Paystack payment integration
- Payment verification
- Order history
- Order status tracking
- Automatic order-status updates

### Admin Features

- Automatic admin detection after login
- Admin-only order dashboard
- View customer information
- View ordered items
- View order totals
- View order dates
- Update order status
- Protected admin routes

## Tech Stack

### Backend
- Go
- net/http
- PostgreSQL
- database/sql
- bcrypt

### Frontend
- HTML
- CSS
- Go Templates
- JavaScript

### APIs & Services
- Paystack API
- Render

### Development Tools
- Git
- GitHub
- GitHub Codespaces

## Payment Flow

1. Customer adds meals to the cart.
2. Customer proceeds to checkout.
3. The server creates the order and order items.
4. Paystack payment is initialized.
5. Customer completes payment through Paystack.
6. The server verifies the transaction.
7. The payment amount is checked against the order total.
8. The order is marked as paid.
9. The customer's cart is cleared.
10. The customer can view and track the order.

## Order Statuses

Orders can move through the following statuses:

- Pending
- Paid
- Preparing
- Ready
- Completed
- Cancelled

## Database

The application uses PostgreSQL with tables for:

- Users
- Menu items
- Cart items
- Orders
- Order items

Order creation uses a database transaction so an order and its items are saved together.

## Security

The application includes:

- Password hashing with bcrypt
- Login-protected customer routes
- Admin-only routes
- HTTP-only authentication cookies
- SameSite cookie protection
- Server-side payment verification
- Paystack amount verification
- Unique payment references
- Server-side order totals

## Deployment

The application is deployed using Render.

The production application uses environment variables for sensitive configuration such as:

- `DATABASE_URL`
- `PAYSTACK_SECRET_KEY`
- `PAYSTACK_CALLBACK_URL`

Sensitive credentials are not stored in the source code.

## Project Status

🚀 Core functionality completed and deployed.

The application has been tested locally and on the deployed Render version, including the customer ordering flow, Paystack test payment flow, order tracking, admin dashboard, and menu images.

## Future Improvements

Possible future improvements include:

- Server-side session tokens
- Paystack webhooks
- Email order notifications
- Better automated test coverage
- Advanced admin analytics
- Order search and filtering
- Database migrations
- Improved error pages
- Production payment mode