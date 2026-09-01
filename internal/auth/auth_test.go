package auth

import (
	"net/http/httptest"
	"testing"
)

func TestSessionRoundTripAndTamperRejection(t *testing.T) {
	manager := NewManager("01234567890123456789012345678901", false)
	response := httptest.NewRecorder()
	if err := manager.Issue(response, "Admin@Example.com"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/api/admin/session", nil)
	request.AddCookie(response.Result().Cookies()[0])
	session, err := manager.Read(request)
	if err != nil {
		t.Fatal(err)
	}
	if session.Email != "admin@example.com" {
		t.Fatalf("expected normalized email, got %q", session.Email)
	}

	tampered := response.Result().Cookies()[0]
	tampered.Value += "x"
	request = httptest.NewRequest("GET", "/api/admin/session", nil)
	request.AddCookie(tampered)
	if _, err := manager.Read(request); err == nil {
		t.Fatal("expected tampered session to fail")
	}
}

func TestOAuthStateRoundTrip(t *testing.T) {
	manager := NewManager("01234567890123456789012345678901", false)
	response := httptest.NewRecorder()
	state, err := manager.NewState(response)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/api/admin/google/callback?state="+state, nil)
	request.AddCookie(response.Result().Cookies()[0])
	if err := manager.VerifyState(request, state); err != nil {
		t.Fatal(err)
	}
	if err := manager.VerifyState(request, state+"x"); err == nil {
		t.Fatal("expected mismatched state to fail")
	}
}
