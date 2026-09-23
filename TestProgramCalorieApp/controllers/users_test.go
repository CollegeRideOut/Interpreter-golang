package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"calorieapp/models"
)

func TestRegisterAndLoginCreateSessions(t *testing.T) {
	controller := NewUserController()
	register := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"username":"sam","password":"secret"}`))
	registered := httptest.NewRecorder()
	controller.Register(registered, register)
	if registered.Code != http.StatusCreated {
		t.Fatalf("register status = %d", registered.Code)
	}
	var session models.Session
	if err := json.NewDecoder(registered.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.Token == "" || session.User.Username != "sam" {
		t.Fatalf("session = %+v", session)
	}

	login := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"username":"sam","password":"secret"}`))
	loggedIn := httptest.NewRecorder()
	controller.Login(loggedIn, login)
	if loggedIn.Code != http.StatusCreated {
		t.Fatalf("login status = %d", loggedIn.Code)
	}
}
