CREATE TABLE IF NOT EXISTS menu_items (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    price_kobo INTEGER NOT NULL,
    image_url TEXT
);
