package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not reach PostgreSQL using DATABASE_URL: %w", err)
	}

	// Keep a warm pool so normal page requests reuse established PostgreSQL
	// connections instead of repeatedly paying connection setup latency.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// A newly-created hosted PostgreSQL database has no application tables.
	// Bootstrap the checked-in schema only when the core users table is absent;
	// never reapply the schema dump to an existing database.
	var usersTable sql.NullString
	if err := db.QueryRow(`SELECT to_regclass('public.users')`).Scan(&usersTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not inspect database schema: %w", err)
	}
	if !usersTable.Valid {
		if err := bootstrapSchema(db); err != nil {
			db.Close()
			return nil, fmt.Errorf("database is empty and schema bootstrap failed: %w", err)
		}
	}

	// Create cart_items table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cart_items (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			menu_item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
			quantity INTEGER NOT NULL DEFAULT 1,
			price_kobo_at_addition INTEGER NOT NULL,
			UNIQUE (user_id, menu_item_id)
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Add image_url column to menu_items if it doesn't exist
	_, err = db.Exec(`
		ALTER TABLE menu_items
		ADD COLUMN IF NOT EXISTS image_url TEXT
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Create orders table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			total_kobo INTEGER NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			payment_reference VARCHAR(100),
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Create order_items table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS order_items (
			id SERIAL PRIMARY KEY,
			order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			menu_item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
			quantity INTEGER NOT NULL,
			price_kobo INTEGER NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Keep existing databases compatible with the current user model.
	_, err = db.Exec(`
		ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_provider TEXT NOT NULL DEFAULT 'local';
		ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture TEXT;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS phone TEXT;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name TEXT NOT NULL DEFAULT '';
		ALTER TABLE users ADD COLUMN IF NOT EXISTS middle_name TEXT NOT NULL DEFAULT '';
		ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name TEXT NOT NULL DEFAULT '';
		ALTER TABLE users ADD COLUMN IF NOT EXISTS country TEXT NOT NULL DEFAULT 'Nigeria';
		ALTER TABLE users ADD COLUMN IF NOT EXISTS state TEXT NOT NULL DEFAULT '';
		ALTER TABLE users ADD COLUMN IF NOT EXISTS terms_accepted_at TIMESTAMPTZ;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS delivery_area TEXT;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS preferred_contact TEXT NOT NULL DEFAULT 'email';
		ALTER TABLE users ADD COLUMN IF NOT EXISTS marketing_opt_in BOOLEAN NOT NULL DEFAULT FALSE;
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Vendor coupons and promo codes. Discounts apply to food only.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS coupons (
			id SERIAL PRIMARY KEY,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			code TEXT NOT NULL,
			discount_type TEXT NOT NULL CHECK (discount_type IN ('percentage','flat')),
			discount_value INTEGER NOT NULL CHECK (discount_value > 0),
			min_order_kobo INTEGER NOT NULL DEFAULT 0 CHECK (min_order_kobo >= 0),
			max_discount_kobo INTEGER NOT NULL DEFAULT 0 CHECK (max_discount_kobo >= 0),
			usage_limit INTEGER NOT NULL DEFAULT 0 CHECK (usage_limit >= 0),
			used_count INTEGER NOT NULL DEFAULT 0 CHECK (used_count >= 0),
			reserved_count INTEGER NOT NULL DEFAULT 0 CHECK (reserved_count >= 0),
			active BOOLEAN NOT NULL DEFAULT TRUE,
			starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(vendor_id, code)
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_coupons_vendor_active ON coupons(vendor_id, active, expires_at)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS coupon_id INTEGER REFERENCES coupons(id) ON DELETE SET NULL;
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS coupon_code TEXT NOT NULL DEFAULT '';
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS coupon_discount_kobo INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS coupon_reserved BOOLEAN NOT NULL DEFAULT FALSE;
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// In-app notifications for customers.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			type TEXT NOT NULL DEFAULT 'general',
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			link TEXT NOT NULL DEFAULT '',
			read_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Loyalty rewards: 1 point per ₦100 of food on completed orders.
	// Existing databases are upgraded safely; redeemed and earned records may
	// reference the same order, while only earned records are unique per order.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS loyalty_transactions (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			order_id INTEGER REFERENCES orders(id) ON DELETE SET NULL,
			points INTEGER NOT NULL CHECK (points > 0),
			type TEXT NOT NULL DEFAULT 'earned' CHECK (type IN ('earned', 'redeemed')),
			description TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(`ALTER TABLE loyalty_transactions DROP CONSTRAINT IF EXISTS loyalty_transactions_order_id_key`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_loyalty_transactions_user_created ON loyalty_transactions(user_id, created_at DESC)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_loyalty_earned_order ON loyalty_transactions(order_id) WHERE type = 'earned' AND order_id IS NOT NULL`); err != nil {
		db.Close()
		return nil, err
	}

	// Ensure scheduled-order columns exist before creating the partial index that references them.
	// Existing databases are upgraded safely with IF NOT EXISTS.
	_, err = db.Exec(`
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS scheduled_for TIMESTAMPTZ;
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Hot-path indexes used by the home page, cart/header counters and order history.
	// IF NOT EXISTS makes this safe for existing installations.
	for _, indexSQL := range []string{
		`CREATE INDEX IF NOT EXISTS idx_menu_items_vendor_id ON menu_items(vendor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_menu_item_id ON order_items(menu_item_id)`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_user_created ON orders(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_vendor_status ON orders(vendor_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_scheduled_for ON orders(status, scheduled_for) WHERE scheduled_for IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_cart_items_user_id ON cart_items(user_id)`,
	} {
		if _, err = db.Exec(indexSQL); err != nil {
			db.Close()
			return nil, err
		}
	}

	// Create sessions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`); err != nil {
		db.Close()
		return nil, err
	}

	// Keep existing order records compatible with checkout, vendor orders and delivery.
	// vendor_id is nullable so old records that cannot be mapped safely are not
	// assigned to the wrong shop. New orders always set it explicitly.
	_, err = db.Exec(`
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS vendor_id INTEGER REFERENCES vendors(id);
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_method TEXT NOT NULL DEFAULT 'pickup';
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_address TEXT NOT NULL DEFAULT '';
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_phone TEXT NOT NULL DEFAULT '';
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_fee_kobo INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_state TEXT NOT NULL DEFAULT '';
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_lga TEXT NOT NULL DEFAULT '';
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS commission_kobo INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS loyalty_points_redeemed INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_loyalty_points_redeemed_check;
		ALTER TABLE orders ADD CONSTRAINT orders_loyalty_points_redeemed_check CHECK (loyalty_points_redeemed >= 0);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Money sent to vendors from their sales, recorded by the admin
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vendor_payouts (
			id SERIAL PRIMARY KEY,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id),
			amount_kobo INTEGER NOT NULL CHECK (amount_kobo > 0),
			note TEXT NOT NULL DEFAULT '',
			paid_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			recorded_by INTEGER
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Vendor payout account details. Verification is performed through Paystack
	// and the account is never exposed on public vendor pages.
	_, err = db.Exec(`
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS payout_bank_name TEXT NOT NULL DEFAULT '';
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS payout_bank_code TEXT NOT NULL DEFAULT '';
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS payout_account_number TEXT NOT NULL DEFAULT '';
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS payout_account_name TEXT NOT NULL DEFAULT '';
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS payout_account_verified BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS payout_account_verified_at TIMESTAMPTZ;
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Short-lived server-side records used between Paystack account verification
	// and the vendor's explicit Save action. Raw verification tokens are never
	// exposed as permanent vendor data.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vendor_payout_verifications (
			token TEXT PRIMARY KEY,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			bank_name TEXT NOT NULL,
			bank_code TEXT NOT NULL,
			account_number TEXT NOT NULL,
			account_name TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Store the exact Nigerian state/LGA combinations each vendor serves.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vendor_delivery_areas (
			id SERIAL PRIMARY KEY,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			state TEXT NOT NULL,
			lga TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (vendor_id, state, lga)
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_vendor_delivery_areas_vendor ON vendor_delivery_areas(vendor_id)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Private customer-to-vendor conversations. A customer has one thread per shop;
	// every read/write handler independently checks that the requester is a participant.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS message_threads (
			id SERIAL PRIMARY KEY,
			customer_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (customer_id, vendor_id)
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id BIGSERIAL PRIMARY KEY,
			thread_id INTEGER NOT NULL REFERENCES message_threads(id) ON DELETE CASCADE,
			sender_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			read_at TIMESTAMPTZ
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	// delivered_at powers the chat ticks: sent (neither set), delivered (delivered_at set), read (read_at set).
	_, err = db.Exec(`ALTER TABLE messages ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ`)
	if err != nil {
		db.Close()
		return nil, err
	}
	// Messages that were already read before this column existed must also count as delivered.
	_, err = db.Exec(`UPDATE messages SET delivered_at = read_at WHERE delivered_at IS NULL AND read_at IS NOT NULL`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_message_threads_customer_updated ON message_threads(customer_id, updated_at DESC)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_message_threads_vendor_updated ON message_threads(vendor_id, updated_at DESC)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id, id)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Password reset tokens are part of the authentication flow and may be absent
	// from older databases created before password reset was introduced.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS password_reset_tokens (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMPTZ NOT NULL,
			used BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Remembers which phone number a text-message code was sent to, and for whom,
	// so a code can only verify the number it was really sent to.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS phone_verifications (
			pin_id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			phone TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Loyalty rewards for completed customer orders. This additive migration
	// also creates the table for existing databases that predate Stage 12.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS loyalty_transactions (
			id BIGSERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			order_id INTEGER UNIQUE REFERENCES orders(id) ON DELETE SET NULL,
			points INTEGER NOT NULL CHECK (points > 0),
			type TEXT NOT NULL DEFAULT 'earned' CHECK (type IN ('earned', 'redeemed')),
			description TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_loyalty_transactions_user_created ON loyalty_transactions(user_id, created_at DESC)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Verified-purchase reviews. A customer can review a completed order once,
	// with optional per-dish ratings. This is additive for existing databases.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS order_reviews (
			id BIGSERIAL PRIMARY KEY,
			order_id INTEGER NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
			comment TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS order_item_reviews (
			id BIGSERIAL PRIMARY KEY,
			order_review_id BIGINT NOT NULL REFERENCES order_reviews(id) ON DELETE CASCADE,
			order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			menu_item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
			rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
			UNIQUE (order_review_id, menu_item_id)
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_order_reviews_vendor_created ON order_reviews(vendor_id, created_at DESC)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_order_item_reviews_menu_item ON order_item_reviews(menu_item_id)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// bootstrapSchema applies schema.sql to a truly empty database. The checked-in
// file is a pg_dump schema export, so psql-only meta commands and session setup
// statements are removed before sending the SQL through database/sql.
func bootstrapSchema(db *sql.DB) error {
	data, err := os.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema.sql from the project root: %w", err)
	}
	schemaSQL := cleanSchemaDump(string(data))
	if schemaSQL == "" {
		return fmt.Errorf("schema.sql contains no SQL")
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return err
	}
	return nil
}

func cleanSchemaDump(dump string) string {
	var statements []string
	for _, line := range strings.Split(dump, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, `\`),
			strings.HasPrefix(trimmed, "--"),
			strings.HasPrefix(trimmed, "SET "),
			strings.HasPrefix(trimmed, "SELECT pg_catalog.set_config"),
			strings.HasPrefix(trimmed, "CREATE SCHEMA public"),
			strings.HasPrefix(trimmed, "COMMENT ON SCHEMA public"):
			continue
		}
		statements = append(statements, line)
	}
	return strings.TrimSpace(strings.Join(statements, "\n"))
}
