package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginPolicyAllowsSameHost(t *testing.T) {
	policy := newOriginPolicy(nil)
	req := httptest.NewRequest(http.MethodGet, "http://api.example.com/api/health", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://api.example.com")

	origin, ok := policy.allowedOrigin(req)
	if !ok {
		t.Fatal("allowedOrigin denied same host origin")
	}
	if origin != "https://api.example.com" {
		t.Fatalf("origin = %q, want original origin", origin)
	}
}

func TestOriginPolicyAllowsConfiguredCrossOrigin(t *testing.T) {
	policy := newOriginPolicy([]string{"https://homelab.example.com"})
	req := httptest.NewRequest(http.MethodGet, "http://api.example.com/api/health", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://homelab.example.com")

	origin, ok := policy.allowedOrigin(req)
	if !ok {
		t.Fatal("allowedOrigin denied configured origin")
	}
	if origin != "https://homelab.example.com" {
		t.Fatalf("origin = %q, want original origin", origin)
	}
}

func TestOriginPolicyDeniesUnconfiguredCrossOrigin(t *testing.T) {
	policy := newOriginPolicy(nil)
	req := httptest.NewRequest(http.MethodGet, "http://api.example.com/api/health", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://other.example.com")

	if _, ok := policy.allowedOrigin(req); ok {
		t.Fatal("allowedOrigin allowed unconfigured cross origin")
	}
}

func TestOriginPolicyDeniesMalformedOrigin(t *testing.T) {
	policy := newOriginPolicy([]string{"*"})
	req := httptest.NewRequest(http.MethodGet, "http://api.example.com/api/health", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://api.example.com/path")

	if _, ok := policy.allowedOrigin(req); ok {
		t.Fatal("allowedOrigin allowed malformed origin")
	}
}

func TestCORSPreflightEchoesConfiguredOrigin(t *testing.T) {
	s := &Server{originPolicy: newOriginPolicy([]string{"https://homelab.example.com"})}
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not run for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "http://api.example.com/api/health", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://homelab.example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://homelab.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want true", got)
	}
}

func TestCORSPreflightRejectsUnconfiguredOrigin(t *testing.T) {
	s := &Server{originPolicy: newOriginPolicy(nil)}
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not run for rejected preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "http://api.example.com/api/health", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://other.example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestCORSRequestRejectsUnconfiguredOrigin(t *testing.T) {
	s := &Server{originPolicy: newOriginPolicy(nil)}
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not run for rejected origin")
	}))

	req := httptest.NewRequest(http.MethodPost, "http://api.example.com/api/hosts", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://other.example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}
