
LaikiHub — Food Marketplace Platform

LaikiHub is a Go and PostgreSQL-powered food marketplace connecting customers with local food vendors in Nigeria. It features secure customer accounts (with Google Sign-In), custom vendor shops, dynamic menu management, cart processing, Paystack payments, Brevo transactional emails, and admin control panels.

🚀 Live Project

• Production Deployment: Render

🔥 Core Features


👤 Customer Experience

• Authentication: Manual signup/login (bcrypt) or Google OAuth 2.0. Time-limited email password resets.
• Browsing & Discovery: Search and filter menus by categories (Rice & Swallow, Proteins & Sides, Soups & Stews, Drinks).
• Checkout & Guest Mode: Order via an account or use Guest Checkout (uses secure guest_token cookie and signed HMAC tracking links).
• Payments: Integrated Paystack test-mode flow with server-side amount validation and webhook verification.
• Order Tracking: Live visual status stepper with automated email updates.
• Messaging: Real-time customer-to-vendor text chat at /messages (polls every 5 seconds).

🏪 Vendor & Shop Tools

• Delivery Rules: Configure custom delivery prices and minimum order limits mapped to specific Nigerian State | LGA boundaries.
• AI Writing Studio: Generate menu descriptions, social media drafts, and customer replies using OpenAI or Gemini APIs.
• Financial Reporting: Track performance via period statements and download transaction history as CSV files.
• Payout Account Setup: Verify Nigerian bank accounts directly using Paystack's bank directory API.

🛡️ Administration Portal

• Overview Metrics: View platform totals, manage user rosters, and supervise active shops.
• Order Supervision: Track processing payloads and manually move order statuses.
• Financial Settlements: Review platform-wide revenue reports with administrative CSV exports.
• Security Control: Only the main platform administrator (omalegrace2009@gmail.com) can see full payout bank details.

🛠️ Tech Stack

• Backend: Go (net/http, database/sql, embedded HTML templates)
• Database: PostgreSQL
• Frontend: HTML5, CSS3, Go Templates (Mobile-responsive utility system, installable PWA manifest)
• Integrations: Paystack (Payments/Banks), Brevo (Email SMTP), Termii (SMS alerts), OpenAI/Gemini (AI Copywriting)

📂 Project Structure

text
├── main.go            # App router & server bootstrap
├── handlers/          # Modular feature controllers (1 file per domain)
├── render/            # Template rendering middleware & engine configuration
├── templates/         # UI layout structures (*.html with /partials component architecture)
├── static/            # Public compiled styling and scripts (/css, /js, /img)
├── db/                # Schema blueprints (schema.sql) and dataset migrations
├── middleware/        # Rate limiters, security headers, and active session verifications
└── errors/            # Uniform branded client-error presentation package
Use code with caution.

💻 Local Development


1. Project Initialization

bash
git clone https://github.com/OmaleGrace/Order-site
cd Order-site
Use code with caution.

2. Environment Configuration

Copy .env.example to a new .env file. Provide your database parameters and API credentials.
• Note: If DATABASE_URL targets an empty schema, LaikiHub boots up table architectures automatically from db/schema.sql.

3. Run the App

bash

# Production mode (Parses templates once on start)
go run .


# UI-Hot Reload mode (Reads templates from disk dynamically)
TEMPLATES_DIR=templates go run .
Use code with caution.
🌐 Local Endpoint: http://localhost:8080

🧪 Testing Protocol

Integration profiles clean and rebuild tables using a dedicated development engine. Never run tests against a live production database.
bash

# Execute suite sequentially (Prevents shared-database write collision)
go test -p 1 ./...

# View coverage reports
go test ./... -cover

# Execute linting / static analysis
go vet ./...
Use code with caution.

🛡️ Key Operations & Security Notes

• Database PgBouncer Errors: If you see unnamed prepared statement does not exist, append ?binary_parameters=yes to your connection string.
• App Branding: Identifiers start with LAIKI- instead of old project variables.
• Secret Rules: Do not archive or commit your .env configuration. Generate distribution zips safely using: git archive -o release.zip HEAD.



Paid insight reports.
Reminders and win-back messages, recurring orders, WhatsApp and push alerts, daily summaries: these need a scheduler and a messaging provider (for example WhatsApp Business), which don't exist in the site yet.
Photo reviews and live tracking: photo reviews are doable next, since photo upload now exists. Live tracking depends on how your riders work.


## Rider tracking and project archive note

The rider workflow uses a one-time, high-entropy tracking link for each assignment. Riders open it in a mobile browser, explicitly grant browser geolocation permission to share their location, and can stop sharing at any time. The rider token is stored as a SHA-256 hash and is revoked when the rider is deactivated or the assignment is removed.

The uploaded project archive used for this update did not include the original `db/schema.sql` dump even though `db/embed.go` embeds it. A placeholder file is present only so the embedded source path exists. Existing installations with an initialized database can use the incremental migrations in `database.Connect`; restore the real schema dump before using this archive to initialize a brand-new database.
