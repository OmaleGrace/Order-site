// Command move-images copies pictures that are stored inside the database to Cloudinary.
//
// Run it once after turning Cloudinary on (it needs DATABASE_URL and the three CLOUDINARY_*
// settings). It is safe to run again: pictures already copied are skipped.
//
//	go run ./cmd/move-images            copy everything, keep the database copy
//	go run ./cmd/move-images -free      also empty the database copy once Cloudinary has it
//	go run ./cmd/move-images -limit 20  only do 20 pictures this time
//
// Old /media/<id> addresses keep working: the site forwards them to Cloudinary.
// After using -free, run  VACUUM FULL uploaded_images;  in your database to give the space back.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"Order-site/database"
	"Order-site/imagestore"
)

func main() {
	limit := flag.Int("limit", 0, "stop after this many pictures (0 = all)")
	free := flag.Bool("free", false, "empty the database copy of each picture once Cloudinary has it")
	flag.Parse()

	if !imagestore.Configured() {
		fmt.Println("Set CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY and CLOUDINARY_API_SECRET first.")
		os.Exit(1)
	}
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	moved, failed := 0, 0
	var afterID int64 // pictures are taken in id order, so a failed one is never retried in the same run
	for *limit == 0 || moved+failed < *limit {
		batch := 25
		if *limit > 0 && *limit-moved-failed < batch {
			batch = *limit - moved - failed
		}
		type image struct {
			id   int64
			data []byte
		}
		var todo []image
		rows, err := db.QueryContext(ctx, `SELECT id, data FROM uploaded_images WHERE cloud_url IS NULL AND data IS NOT NULL AND id > $1 ORDER BY id LIMIT $2`, afterID, batch)
		if err != nil {
			log.Fatalf("reading pictures: %v", err)
		}
		for rows.Next() {
			var im image
			if err := rows.Scan(&im.id, &im.data); err != nil {
				rows.Close()
				log.Fatalf("reading a picture: %v", err)
			}
			todo = append(todo, im)
		}
		rows.Close()
		if len(todo) == 0 {
			break
		}
		for _, im := range todo {
			afterID = im.id
			url, err := imagestore.Upload(ctx, im.data)
			if err != nil {
				failed++
				fmt.Printf("picture %d: upload failed: %v\n", im.id, err)
				continue
			}
			q := `UPDATE uploaded_images SET cloud_url = $1 WHERE id = $2`
			if *free {
				q = `UPDATE uploaded_images SET cloud_url = $1, data = NULL WHERE id = $2`
			}
			if _, err := db.ExecContext(ctx, q, url, im.id); err != nil {
				log.Fatalf("picture %d: saved to Cloudinary but could not record it: %v", im.id, err)
			}
			moved++
			fmt.Printf("picture %d -> %s\n", im.id, url)
		}
	}
	fmt.Printf("Done. Moved %d, failed %d.\n", moved, failed)
	if *free && moved > 0 {
		fmt.Println("Now run  VACUUM FULL uploaded_images;  in your database to give the space back.")
	}
	if failed > 0 {
		os.Exit(2)
	}
}
