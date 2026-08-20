package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebsiteTitle(t *testing.T) {
	tests := []struct {
		name                    string
		status                  int
		body                    string
		wantTitle               string
		wantErr                 bool
		wantErrTitleUnavailable bool
	}{
		{
			name:      "OK with title",
			status:    http.StatusOK,
			body:      `<html><head><title>  Hello   World  </title></head></html>`,
			wantTitle: "Hello World",
			wantErr:   false,
		},
		{
			name:                    "OK with empty title",
			status:                  http.StatusOK,
			body:                    `<html><head><title>   </title></head></html>`,
			wantTitle:               "",
			wantErr:                 true,
			wantErrTitleUnavailable: true,
		},
		{
			name:                    "OK with missing title tag",
			status:                  http.StatusOK,
			body:                    `<html><head></head><body>hi</body></html>`,
			wantTitle:               "",
			wantErr:                 true,
			wantErrTitleUnavailable: true,
		},
		{
			name:                    "Forbidden",
			status:                  http.StatusForbidden,
			body:                    "",
			wantTitle:               "",
			wantErr:                 true,
			wantErrTitleUnavailable: true,
		},
		{
			name:      "NotFound",
			status:    http.StatusNotFound,
			body:      "",
			wantTitle: "",
			wantErr:   true,
		},
		{
			name:      "ServerError",
			status:    http.StatusInternalServerError,
			body:      "",
			wantTitle: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			defer server.Close()

			gotTitle, gotErr := WebsiteTitle(server.URL)
			if gotTitle != tt.wantTitle {
				t.Errorf("title mismatch: got %q, want %q", gotTitle, tt.wantTitle)
			}
			if tt.wantErr && gotErr == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && gotErr != nil {
				t.Errorf("unexpected error: %v", gotErr)
			}
			if tt.wantErrTitleUnavailable && !errors.Is(gotErr, ErrTitleUnavailable) {
				t.Errorf("expected ErrTitleUnavailable, got %v", gotErr)
			}
		})
	}
}
