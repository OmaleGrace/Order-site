package database

import (
	dbfiles "Order-site/db"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// poolerSafe adds binary_parameters=yes to a connection string unless it already sets it.
//
// By default the PostgreSQL driver prepares a statement on one network round trip and runs it
// on the next. Hosted databases usually sit behind a connection pooler (PgBouncer, Supabase,
// Neon), and in its common "transaction" mode the second round trip can land on a different
// server connection, which fails with "unnamed prepared statement does not exist" (26000).
// People see that as a page that errors once and then works when refreshed.
// binary_parameters=yes makes the driver send each statement in a single batch, which is
// correct both with and without a pooler. Set binary_parameters=no yourself to opt out.
func poolerSafe(dsn string) string {
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn
		}
		q := u.Query()
		if q.Get("binary_parameters") != "" {
			return dsn
		}
		q.Set("binary_parameters", "yes")
		u.RawQuery = q.Encode()
		return u.String()
	}
	if strings.Contains(dsn, "binary_parameters") {
		return dsn
	}
	return strings.TrimSpace(dsn) + " binary_parameters=yes"
}

func Connect() (*sql.DB, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	databaseURL = poolerSafe(databaseURL)

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
	// Hosted PostgreSQL services and poolers often close idle connections after a minute or
	// two. A connection that was dropped behind our back fails the first query that uses it
	// (one error page, then it works on refresh), so retire idle ones sooner than that.
	db.SetConnMaxIdleTime(1 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

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

	// Bring the database up to date. Each change is recorded in schema_migrations, so a restart
	// only checks one small table instead of re-running every statement.
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	return db, nil
}

// baselineMigration is migration 1: every schema change made before numbered migrations existed.
// Each statement is idempotent (IF NOT EXISTS and similar), so it is safe on a brand-new database
// and on one that already has some or all of these changes. It runs once per database. All new
// changes go in database/migrations/NNNN_name.sql instead of here.
func baselineMigration(db execer) error {
	var err error
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
		return err
	}

	// Add image_url column to menu_items if it doesn't exist
	_, err = db.Exec(`
		ALTER TABLE menu_items
		ADD COLUMN IF NOT EXISTS image_url TEXT
	`)
	if err != nil {
		return err
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
		return err
	}

	// Older databases may hold payment_reference as a number. References look like
	// "ORDER-14-1730000000", so the column must be text. Convert it once, and index
	// the text form the payment lookups use.
	if _, err = db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'orders' AND column_name = 'payment_reference'
				  AND data_type NOT IN ('character varying', 'text')
			) THEN
				ALTER TABLE orders ALTER COLUMN payment_reference TYPE VARCHAR(100) USING payment_reference::text;
			END IF;
		END $$;
		CREATE INDEX IF NOT EXISTS orders_payment_reference_idx ON orders ((payment_reference::text));
	`); err != nil {
		return err
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
		return err
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
		return err
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
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_coupons_vendor_active ON coupons(vendor_id, active, expires_at)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS coupon_id INTEGER REFERENCES coupons(id) ON DELETE SET NULL;
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS coupon_code TEXT NOT NULL DEFAULT '';
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS coupon_discount_kobo INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS coupon_reserved BOOLEAN NOT NULL DEFAULT FALSE;
	`)
	if err != nil {
		return err
	}

	// Food customization/add-ons. Vendors can define option groups (for example, Size or Extras)
	// and customers can select valid options before adding a dish to the cart. Prices are
	// snapshotted into the cart and order so historical totals remain transparent.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS menu_option_groups (
			id SERIAL PRIMARY KEY,
			menu_item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			min_select INTEGER NOT NULL DEFAULT 0 CHECK (min_select >= 0),
			max_select INTEGER NOT NULL DEFAULT 1 CHECK (max_select >= 1 AND max_select >= min_select),
			position INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT TRUE
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS menu_options (
			id SERIAL PRIMARY KEY,
			group_id INTEGER NOT NULL REFERENCES menu_option_groups(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			price_kobo INTEGER NOT NULL DEFAULT 0 CHECK (price_kobo >= 0),
			position INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT TRUE
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cart_item_options (
			id SERIAL PRIMARY KEY,
			cart_item_id INTEGER NOT NULL REFERENCES cart_items(id) ON DELETE CASCADE,
			option_id INTEGER NOT NULL REFERENCES menu_options(id) ON DELETE RESTRICT,
			name_at_addition TEXT NOT NULL,
			price_kobo_at_addition INTEGER NOT NULL DEFAULT 0 CHECK (price_kobo_at_addition >= 0),
			UNIQUE(cart_item_id, option_id)
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS order_item_options (
			id SERIAL PRIMARY KEY,
			order_item_id INTEGER NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			price_kobo INTEGER NOT NULL DEFAULT 0 CHECK (price_kobo >= 0)
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_menu_option_groups_item ON menu_option_groups(menu_item_id, position); CREATE INDEX IF NOT EXISTS idx_menu_options_group ON menu_options(group_id, position); CREATE INDEX IF NOT EXISTS idx_cart_item_options_cart ON cart_item_options(cart_item_id); CREATE INDEX IF NOT EXISTS idx_order_item_options_item ON order_item_options(order_item_id)`)
	if err != nil {
		return err
	}

	// Delivery riders and order assignment. Existing orders remain unassigned until an admin assigns a rider.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS riders (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			phone TEXT NOT NULL,
			vehicle TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS rider_id INTEGER REFERENCES riders(id) ON DELETE SET NULL;
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_status TEXT NOT NULL DEFAULT 'unassigned';
        ALTER TABLE orders ADD COLUMN IF NOT EXISTS rider_access_token_hash TEXT;
        ALTER TABLE orders ADD COLUMN IF NOT EXISTS rider_latitude DOUBLE PRECISION;
        ALTER TABLE orders ADD COLUMN IF NOT EXISTS rider_longitude DOUBLE PRECISION;
        ALTER TABLE orders ADD COLUMN IF NOT EXISTS rider_location_updated_at TIMESTAMPTZ;
        ALTER TABLE orders ADD COLUMN IF NOT EXISTS rider_sharing_location BOOLEAN NOT NULL DEFAULT FALSE;
        ALTER TABLE orders ADD COLUMN IF NOT EXISTS rider_status_updated_at TIMESTAMPTZ;
		CREATE INDEX IF NOT EXISTS idx_orders_rider_status ON orders(rider_id, status) WHERE rider_id IS NOT NULL;
        CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_rider_token_hash ON orders(rider_access_token_hash) WHERE rider_access_token_hash IS NOT NULL;
	`)
	if err != nil {
		return err
	}

	// Messaging presence is optional and private by default only when a user switches it off.
	_, err = db.Exec(`
		ALTER TABLE users ADD COLUMN IF NOT EXISTS active_status_enabled BOOLEAN NOT NULL DEFAULT TRUE;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;
		CREATE INDEX IF NOT EXISTS idx_users_last_seen ON users(last_seen_at);
	`)
	if err != nil {
		return err
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
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC)`)
	if err != nil {
		return err
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
		return err
	}
	if _, err = db.Exec(`ALTER TABLE loyalty_transactions DROP CONSTRAINT IF EXISTS loyalty_transactions_order_id_key`); err != nil {
		return err
	}
	if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_loyalty_transactions_user_created ON loyalty_transactions(user_id, created_at DESC)`); err != nil {
		return err
	}
	if _, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_loyalty_earned_order ON loyalty_transactions(order_id) WHERE type = 'earned' AND order_id IS NOT NULL`); err != nil {
		return err
	}

	// Ensure scheduled-order columns exist before creating the partial index that references them.
	// Existing databases are upgraded safely with IF NOT EXISTS.
	_, err = db.Exec(`
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS scheduled_for TIMESTAMPTZ;
	`)
	if err != nil {
		return err
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
			return err
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
		return err
	}
	if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`); err != nil {
		return err
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
		return err
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
		return err
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
		return err
	}

	// Held payouts and automatic transfers: an order's completion time starts the hold, and each
	// payout records whether it is still being sent, was paid, or failed.
	_, err = db.Exec(`
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;
		UPDATE orders SET completed_at = created_at WHERE status = 'completed' AND completed_at IS NULL;
		ALTER TABLE vendor_payouts ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'paid';
		ALTER TABLE vendor_payouts ADD COLUMN IF NOT EXISTS method TEXT NOT NULL DEFAULT 'manual';
		ALTER TABLE vendor_payouts ADD COLUMN IF NOT EXISTS transfer_reference TEXT;
		ALTER TABLE vendor_payouts ADD COLUMN IF NOT EXISTS transfer_code TEXT;
		ALTER TABLE vendor_payouts ADD COLUMN IF NOT EXISTS failure_reason TEXT NOT NULL DEFAULT '';
		CREATE UNIQUE INDEX IF NOT EXISTS vendor_payouts_transfer_reference_idx ON vendor_payouts (transfer_reference) WHERE transfer_reference IS NOT NULL;
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS paystack_recipient_code TEXT NOT NULL DEFAULT '';
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS paystack_recipient_account TEXT NOT NULL DEFAULT '';
	`)
	if err != nil {
		return err
	}

	// The vendor agreement a person accepts when applying; admins cannot approve without it.
	_, err = db.Exec(`
		ALTER TABLE vendor_applications ADD COLUMN IF NOT EXISTS agreement_version TEXT NOT NULL DEFAULT '';
		ALTER TABLE vendor_applications ADD COLUMN IF NOT EXISTS agreement_accepted_at TIMESTAMPTZ;
		ALTER TABLE vendor_applications ADD COLUMN IF NOT EXISTS agreement_ip TEXT NOT NULL DEFAULT '';
	`)
	if err != nil {
		return err
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
		return err
	}

	// Weekly opening hours. One row per open day; a vendor with no rows has not published hours.
	// Times are "HH:MM" text read in Lagos time; a closing time earlier than the opening time means
	// the shop closes after midnight.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vendor_hours (
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			weekday SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
			opens TEXT NOT NULL,
			closes TEXT NOT NULL,
			PRIMARY KEY (vendor_id, weekday)
		)
	`)
	if err != nil {
		return err
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
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_vendor_delivery_areas_vendor ON vendor_delivery_areas(vendor_id)`)
	if err != nil {
		return err
	}

	// Per-area delivery pricing (fee, minimum order, free-delivery amount). It has its own
	// table because saving the shop profile rewrites vendor_delivery_areas, which would
	// otherwise wipe the prices. A NULL fee means "use the shop's default delivery fee".
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vendor_delivery_rates (
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			state TEXT NOT NULL,
			lga TEXT NOT NULL,
			fee_kobo INTEGER CHECK (fee_kobo IS NULL OR fee_kobo >= 0),
			min_order_kobo INTEGER NOT NULL DEFAULT 0 CHECK (min_order_kobo >= 0),
			free_over_kobo INTEGER NOT NULL DEFAULT 0 CHECK (free_over_kobo >= 0),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (vendor_id, state, lga)
		)
	`)
	if err != nil {
		return err
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
		return err
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
		return err
	}
	// delivered_at powers the chat ticks: sent (neither set), delivered (delivered_at set), read (read_at set).
	_, err = db.Exec(`ALTER TABLE messages ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ`)
	if err != nil {
		return err
	}
	// Messages that were already read before this column existed must also count as delivered.
	_, err = db.Exec(`UPDATE messages SET delivered_at = read_at WHERE delivered_at IS NULL AND read_at IS NOT NULL`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_message_threads_customer_updated ON message_threads(customer_id, updated_at DESC)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_message_threads_vendor_updated ON message_threads(vendor_id, updated_at DESC)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id, id)`)
	if err != nil {
		return err
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
		return err
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
		return err
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
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_loyalty_transactions_user_created ON loyalty_transactions(user_id, created_at DESC)`)
	if err != nil {
		return err
	}

	// Stage 20 menu availability: vendors can temporarily hide dishes without deleting them.
	_, err = db.Exec(`ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS available BOOLEAN NOT NULL DEFAULT TRUE`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_menu_items_vendor_available ON menu_items(vendor_id, available)`)
	if err != nil {
		return err
	}

	// Stage 19 support tickets. Customer/vendor support requests are retained for admin follow-up.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS support_tickets (
			id BIGSERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			order_id INTEGER REFERENCES orders(id) ON DELETE SET NULL,
			category VARCHAR(40) NOT NULL DEFAULT 'General',
			subject VARCHAR(160) NOT NULL,
			message TEXT NOT NULL,
			admin_reply TEXT NOT NULL DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open','pending','resolved')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_support_tickets_status_updated ON support_tickets(status, updated_at DESC)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_support_tickets_user_created ON support_tickets(user_id, created_at DESC)`)
	if err != nil {
		return err
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
		return err
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
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_order_reviews_vendor_created ON order_reviews(vendor_id, created_at DESC)`)
	if err != nil {
		return err
	}

	// Optional food photo attached to a review.
	_, err = db.Exec(`ALTER TABLE order_reviews ADD COLUMN IF NOT EXISTS photo_url TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_order_item_reviews_menu_item ON order_item_reviews(menu_item_id)`)
	if err != nil {
		return err
	}

	// Guest checkout. A guest gets a lightweight placeholder row in users (flagged
	// is_guest, with an unusable email and no password) so the cart, orders and payment
	// code keep working unchanged. The contact details the guest typed live on the order
	// itself, so they never collide with a real account's unique email or phone.
	_, err = db.Exec(`
		ALTER TABLE users ADD COLUMN IF NOT EXISTS is_guest BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS guest_name TEXT NOT NULL DEFAULT '';
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS guest_email TEXT NOT NULL DEFAULT '';
		ALTER TABLE orders ADD COLUMN IF NOT EXISTS guest_phone TEXT NOT NULL DEFAULT '';
	`)
	if err != nil {
		return err
	}

	// Pictures uploaded from a vendor's device are kept in the database, so they survive
	// redeploys on hosts with temporary disks.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS uploaded_images (
			id BIGSERIAL PRIMARY KEY,
			content_type TEXT NOT NULL,
			data BYTEA NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}

	// Vendor growth: paid "Promoted" placement and a paid Pro plan (lower commission).
	_, err = db.Exec(`
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS promoted_until TIMESTAMPTZ;
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS plan TEXT NOT NULL DEFAULT 'free';
		ALTER TABLE vendors ADD COLUMN IF NOT EXISTS plan_until TIMESTAMPTZ;
	`)
	if err != nil {
		return err
	}

	// Vendors can now pay for the Pro plan and for promotion themselves.
	_, err = db.Exec(`
		ALTER TABLE vendor_payments ADD COLUMN IF NOT EXISTS days INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE vendor_payments DROP CONSTRAINT IF EXISTS vendor_payments_kind_check;
		ALTER TABLE vendor_payments ADD CONSTRAINT vendor_payments_kind_check
			CHECK (kind IN ('setup', 'renewal', 'pro', 'promote'));
	`)
	if err != nil {
		return err
	}

	// Referrals: each customer gets an invite code; the bonus is paid once per invited friend.
	_, err = db.Exec(`
		ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code TEXT;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS referred_by INTEGER REFERENCES users(id) ON DELETE SET NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_users_referral_code ON users(referral_code) WHERE referral_code IS NOT NULL;
		CREATE TABLE IF NOT EXISTS referral_rewards (
			referred_user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			referrer_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			order_id INTEGER REFERENCES orders(id) ON DELETE SET NULL,
			points INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return err
	}

	// Wallet, gift cards and the pass. The wallet is a ledger (see handlers/wallet.go).
	_, err = db.Exec(`
		ALTER TABLE users ADD COLUMN IF NOT EXISTS pass_until TIMESTAMPTZ;
		CREATE TABLE IF NOT EXISTS wallet_transactions (
			id BIGSERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			amount_kobo INTEGER NOT NULL CHECK (amount_kobo <> 0),
			kind TEXT NOT NULL CHECK (kind IN ('topup', 'gift_redeemed', 'order_payment', 'refund', 'pass_payment')),
			reference TEXT UNIQUE,
			order_id INTEGER REFERENCES orders(id) ON DELETE SET NULL,
			note TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_wallet_transactions_user ON wallet_transactions(user_id, id DESC);
		CREATE TABLE IF NOT EXISTS wallet_payments (
			reference TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			kind TEXT NOT NULL CHECK (kind IN ('topup', 'giftcard', 'pass')),
			amount_kobo INTEGER NOT NULL CHECK (amount_kobo > 0),
			recipient_email TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid')),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			paid_at TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS gift_cards (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			amount_kobo INTEGER NOT NULL CHECK (amount_kobo > 0),
			buyer_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			recipient_email TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			payment_reference TEXT UNIQUE,
			redeemed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
			redeemed_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return err
	}

	// Group orders: a customer asks a shop, the shop quotes one price, the customer pays it in full.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS group_requests (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			event_at TIMESTAMPTZ NOT NULL,
			headcount INTEGER NOT NULL CHECK (headcount > 0),
			delivery_method TEXT NOT NULL DEFAULT 'pickup' CHECK (delivery_method IN ('pickup', 'delivery')),
			address TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			budget_kobo INTEGER,
			quote_kobo INTEGER CHECK (quote_kobo IS NULL OR quote_kobo > 0),
			vendor_note TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'requested' CHECK (status IN ('requested', 'quoted', 'accepted', 'declined', 'cancelled')),
			order_id INTEGER REFERENCES orders(id) ON DELETE SET NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_group_requests_user ON group_requests(user_id, id DESC);
		CREATE INDEX IF NOT EXISTS idx_group_requests_vendor ON group_requests(vendor_id, id DESC);
	`)
	if err != nil {
		return err
	}

	// Customer service fee. It is charged on top of the order total and kept by the platform,
	// so it is stored apart from total_kobo: vendor payouts, statements and analytics (which are
	// built on total_kobo) are unaffected.
	_, err = db.Exec(`ALTER TABLE orders ADD COLUMN IF NOT EXISTS service_fee_kobo INTEGER NOT NULL DEFAULT 0`)
	if err != nil {
		return err
	}

	// Site-wide feedback from anyone (not just order reviews).
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS site_feedback (
			id BIGSERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			name TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL DEFAULT 'Other',
			rating SMALLINT CHECK (rating BETWEEN 1 AND 5),
			message TEXT NOT NULL,
			page TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'new',
			admin_note TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_site_feedback_status ON site_feedback(status, created_at DESC);
	`)
	if err != nil {
		return err
	}
	return nil
}

// CleanupUnusedImages deletes uploads that were never saved onto a dish or shop logo
// (for example a vendor who chose a picture and then left the form).
func CleanupUnusedImages(db *sql.DB) error {
	_, err := db.Exec(`
		DELETE FROM uploaded_images i
		WHERE i.created_at < NOW() - INTERVAL '1 day'
		  AND NOT EXISTS (SELECT 1 FROM menu_items m WHERE m.image_url = '/media/' || i.id::text)
		  AND NOT EXISTS (SELECT 1 FROM vendors v WHERE v.logo_url = '/media/' || i.id::text)
		  AND NOT EXISTS (SELECT 1 FROM order_reviews r WHERE r.photo_url = '/media/' || i.id::text)
	`)
	return err
}

// CleanupGuestAccounts removes placeholder guest rows (and their abandoned carts) that
// never placed an order. Guests who did order are kept, because the order belongs to them.
func CleanupGuestAccounts(db *sql.DB) error {
	_, err := db.Exec(`
		DELETE FROM users u
		WHERE u.is_guest
		  AND u.created_at < NOW() - INTERVAL '30 days'
		  AND NOT EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id)
	`)
	return err
}

// bootstrapSchema applies schema.sql to a truly empty database. The checked-in
// file is a pg_dump schema export, so psql-only meta commands and session setup
// statements are removed before sending the SQL through database/sql.
func bootstrapSchema(db *sql.DB) error {
	// The schema is embedded in the binary, so this works from any working directory.
	schemaSQL := cleanSchemaDump(dbfiles.Schema)
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
