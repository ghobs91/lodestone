package dhtcrawlerhealthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckRemote(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "up", body: `{"details":{"dht":{"status":"up"}}}`, wantErr: false},
		{name: "down", body: `{"details":{"dht":{"status":"down"}}}`, wantErr: true},
		{name: "inactive", body: `{"details":{"dht":{"status":"inactive"}}}`, wantErr: true},
		{name: "missing dht", body: `{"details":{}}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			err := checkRemote(context.Background(), srv.URL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkRemote() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckRemoteUnreachable(t *testing.T) {
	if err := checkRemote(context.Background(), "http://127.0.0.1:1/status"); err == nil {
		t.Fatal("expected an error for an unreachable URL")
	}
}
