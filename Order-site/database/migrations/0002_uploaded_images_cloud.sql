-- Pictures now live in Cloudinary. Older pictures that were stored inside the database get a
-- cloud_url once they have been copied (see cmd/move-images); after that the database copy of the
-- bytes can be dropped to free space, so the data column must be allowed to be empty.
ALTER TABLE uploaded_images ADD COLUMN IF NOT EXISTS cloud_url TEXT;
ALTER TABLE uploaded_images ALTER COLUMN data DROP NOT NULL;
