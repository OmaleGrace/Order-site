package imagestore

import "testing"

func TestConfigured(t *testing.T) {
	for _, k := range []string{"CLOUDINARY_CLOUD_NAME", "CLOUDINARY_API_KEY", "CLOUDINARY_API_SECRET"} {
		t.Setenv(k, "")
	}
	if Configured() {
		t.Fatal("nothing set: must not be configured")
	}
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	if Configured() {
		t.Fatal("a missing secret must not count as configured")
	}
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	if !Configured() {
		t.Fatal("all three set: must be configured")
	}
}

func TestUploadWithoutSettingsFails(t *testing.T) {
	for _, k := range []string{"CLOUDINARY_CLOUD_NAME", "CLOUDINARY_API_KEY", "CLOUDINARY_API_SECRET"} {
		t.Setenv(k, "")
	}
	if _, err := Upload(t.Context(), []byte("x")); err == nil {
		t.Fatal("expected an error when Cloudinary is not configured")
	}
}
