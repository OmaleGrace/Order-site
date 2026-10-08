CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password TEXT,
    auth_provider TEXT NOT NULL DEFAULT 'local',
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS menu_items (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    price_kobo INTEGER NOT NULL,
    image_url TEXT
);
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    token TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE
);
ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone TEXT UNIQUE;
CREATE TABLE IF NOT EXISTS vendors (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    owner_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'active', 'suspended')),
    setup_paid_at TIMESTAMPTZ,
    paid_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS vendor_id INTEGER REFERENCES vendors(id);
CREATE INDEX IF NOT EXISTS idx_menu_items_vendor ON menu_items(vendor_id);

INSERT INTO vendors (name, slug, status, setup_paid_at, paid_until)
VALUES ('Grace''s Kitchen', 'graces-kitchen', 'active', NOW(), NOW() + INTERVAL '100 years')
ON CONFLICT (slug) DO NOTHING;

UPDATE menu_items
SET vendor_id = (SELECT id FROM vendors WHERE slug = 'graces-kitchen')
WHERE vendor_id IS NULL;
CREATE TABLE IF NOT EXISTS favorites (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    menu_item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, menu_item_id)
);
