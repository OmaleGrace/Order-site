// Package imagestore keeps uploaded pictures in Cloudinary instead of inside the database.
package imagestore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// Folder is where site pictures are kept inside the Cloudinary account.
const Folder = "laikihub/images"

// Shrink is applied by Cloudinary when the picture is stored: no side longer than 1200 px,
// automatic quality, and the best format each browser can show.
const Shrink = "c_limit,w_1200,h_1200/q_auto,f_auto"

// Configured reports whether the three Cloudinary settings are present.
func Configured() bool {
	return os.Getenv("CLOUDINARY_CLOUD_NAME") != "" && os.Getenv("CLOUDINARY_API_KEY") != "" && os.Getenv("CLOUDINARY_API_SECRET") != ""
}

// Upload stores one picture and returns its https address.
func Upload(ctx context.Context, data []byte) (string, error) {
	if !Configured() {
		return "", fmt.Errorf("Cloudinary is not configured")
	}
	cld, err := cloudinary.NewFromURL(fmt.Sprintf("cloudinary://%s:%s@%s",
		os.Getenv("CLOUDINARY_API_KEY"), os.Getenv("CLOUDINARY_API_SECRET"), os.Getenv("CLOUDINARY_CLOUD_NAME")))
	if err != nil {
		return "", fmt.Errorf("Cloudinary settings are invalid: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	res, err := cld.Upload.Upload(ctx, bytes.NewReader(data), uploader.UploadParams{Folder: Folder, Transformation: Shrink})
	if err != nil {
		return "", err
	}
	if res == nil || res.SecureURL == "" {
		return "", fmt.Errorf("Cloudinary did not return a picture address")
	}
	return res.SecureURL, nil
}
