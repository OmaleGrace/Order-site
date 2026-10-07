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
