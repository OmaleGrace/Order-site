-- GPS accuracy (in metres) reported by the rider's phone, shown as a circle on the admin live map.
-- Idempotent, so it is safe whether or not the column already exists.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS rider_location_accuracy DOUBLE PRECISION;
