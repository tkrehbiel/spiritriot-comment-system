package writeComments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"spiritriot-comment-services/internal/common"
)

func TestCheckAkismet_Bypass(t *testing.T) {
	os.Setenv("AKISMET_API_KEY", "")
	defer os.Unsetenv("AKISMET_API_KEY")

	data := common.CommentEntryData{
		Name:    "Test User",
		Comment: "A normal comment",
	}

	spam, err := CheckAkismet(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spam {
		t.Error("expected comment to not be marked as spam when bypassed")
	}
}

func TestCheckAkismet_Spam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and content type
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("expected urlencoded content type, got %s", r.Header.Get("Content-Type"))
		}

		// Verify form values
		if r.FormValue("comment_author") != "Spammer" {
			t.Errorf("expected author Spammer, got %s", r.FormValue("comment_author"))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("true"))
	}))
	defer server.Close()

	os.Setenv("AKISMET_API_KEY", "test-key")
	os.Setenv("AKISMET_API_URL", server.URL)
	defer os.Unsetenv("AKISMET_API_KEY")
	defer os.Unsetenv("AKISMET_API_URL")

	data := common.CommentEntryData{
		Name:      "Spammer",
		Email:     "spam@spam.com",
		Comment:   "Buy links here!",
		ClientIP:  "1.2.3.4",
		UserAgent: "Mozilla/5.0",
		Referrer:  "http://referrer.com",
		Page:      "/test-page",
	}

	spam, err := CheckAkismet(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spam {
		t.Error("expected comment to be flagged as spam")
	}
}

func TestCheckAkismet_Ham(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("false"))
	}))
	defer server.Close()

	os.Setenv("AKISMET_API_KEY", "test-key")
	os.Setenv("AKISMET_API_URL", server.URL)
	defer os.Unsetenv("AKISMET_API_KEY")
	defer os.Unsetenv("AKISMET_API_URL")

	data := common.CommentEntryData{
		Name:    "Nice Reader",
		Comment: "Great article!",
	}

	spam, err := CheckAkismet(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spam {
		t.Error("expected comment to not be flagged as spam")
	}
}

func TestCheckAkismet_Error(t *testing.T) {
	os.Setenv("AKISMET_API_KEY", "test-key")
	os.Setenv("AKISMET_API_URL", "http://invalid-localhost-url-that-fails")
	defer os.Unsetenv("AKISMET_API_KEY")
	defer os.Unsetenv("AKISMET_API_URL")

	data := common.CommentEntryData{
		Name:    "Test User",
		Comment: "A normal comment",
	}

	_, err := CheckAkismet(context.Background(), data)
	if err == nil {
		t.Error("expected connection error, got nil")
	}
}
