INSERT INTO menu_items
    (id, name, description, price_kobo, image_url)
VALUES
    (
        1,
        'Jollof Rice',
        'Delicious Nigerian jollof rice',
        350000,
        'https://images.unsplash.com/photo-1665556899022-9761f95769e5?auto=format&fit=crop&fm=jpg&q=80&w=1200'
    ),
    (
        2,
        'Fried Rice',
        'Fried rice with vegetables and chicken',
        400000,
        'https://images.unsplash.com/photo-1772729440931-e8efd3adc748?auto=format&fit=crop&fm=jpg&q=80&w=1200'
    ),
    (
        3,
        'Spaghetti',
        'Spaghetti with tomato sauce and chicken',
        300000,
        'https://images.unsplash.com/photo-1713561058969-793049b01712?auto=format&fit=crop&fm=jpg&q=80&w=1200'
    )
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    price_kobo = EXCLUDED.price_kobo,
    image_url = EXCLUDED.image_url;

SELECT setval(
    'menu_items_id_seq',
    COALESCE((SELECT MAX(id) FROM menu_items), 1),
    true
);