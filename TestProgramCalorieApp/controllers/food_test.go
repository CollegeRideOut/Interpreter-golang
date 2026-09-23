package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"calorieapp/models"
)

func TestFoodSummaryIsScopedToLoggedInUser(t *testing.T) {
	users := NewUserController()
	foods := NewFoodController(2200, users.Authenticate)
	samToken := registerUser(t, users, "sam")
	leeToken := registerUser(t, users, "lee")

	request := httptest.NewRequest(http.MethodPost, "/api/foods", bytes.NewBufferString(`{"name":"Lunch","calories":600,"meal":"lunch"}`))
	request.Header.Set("Authorization", "Bearer "+samToken)
	response := httptest.NewRecorder()
	foods.AddFood(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("add food status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/summary", nil)
	request.Header.Set("Authorization", "Bearer "+leeToken)
	response = httptest.NewRecorder()
	foods.Summary(response, request)
	var summary models.DailyPlan
	if err := json.NewDecoder(response.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.Consumed != 0 || len(summary.Foods) != 0 {
		t.Fatalf("lee summary = %+v", summary)
	}
}

func registerUser(t *testing.T, users *UserController, username string) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"username":"`+username+`","password":"secret"}`))
	response := httptest.NewRecorder()
	users.Register(response, request)
	var session models.Session
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	return session.Token
}
