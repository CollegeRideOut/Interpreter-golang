package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"calorieapp/models"
)

type UserController struct {
	mu        sync.RWMutex
	passwords map[string]string
	sessions  map[string]string
	nextToken atomic.Uint64
}

func NewUserController() *UserController {
	return &UserController{passwords: make(map[string]string), sessions: make(map[string]string)}
}

func (controller *UserController) Register(writer http.ResponseWriter, request *http.Request) {
	var credentials models.Credentials
	if err := json.NewDecoder(request.Body).Decode(&credentials); err != nil {
		http.Error(writer, "invalid credentials JSON", http.StatusBadRequest)
		return
	}
	credentials.Username = strings.TrimSpace(credentials.Username)
	if credentials.Username == "" || credentials.Password == "" {
		http.Error(writer, "username and password are required", http.StatusBadRequest)
		return
	}

	controller.mu.Lock()
	defer controller.mu.Unlock()
	if _, exists := controller.passwords[credentials.Username]; exists {
		http.Error(writer, "username already exists", http.StatusConflict)
		return
	}
	controller.passwords[credentials.Username] = credentials.Password
	writeSession(writer, controller.createSessionLocked(credentials.Username))
}

func (controller *UserController) Login(writer http.ResponseWriter, request *http.Request) {
	var credentials models.Credentials
	if err := json.NewDecoder(request.Body).Decode(&credentials); err != nil {
		http.Error(writer, "invalid credentials JSON", http.StatusBadRequest)
		return
	}
	credentials.Username = strings.TrimSpace(credentials.Username)

	controller.mu.Lock()
	defer controller.mu.Unlock()
	if controller.passwords[credentials.Username] != credentials.Password {
		http.Error(writer, "invalid username or password", http.StatusUnauthorized)
		return
	}
	writeSession(writer, controller.createSessionLocked(credentials.Username))
}

func (controller *UserController) Authenticate(request *http.Request) (string, bool) {
	const prefix = "Bearer "
	header := request.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	controller.mu.RLock()
	username, ok := controller.sessions[strings.TrimSpace(strings.TrimPrefix(header, prefix))]
	controller.mu.RUnlock()
	return username, ok
}

func (controller *UserController) createSessionLocked(username string) models.Session {
	token := fmt.Sprintf("session-%d", controller.nextToken.Add(1))
	controller.sessions[token] = username
	return models.Session{User: models.User{Username: username}, Token: token}
}

func writeSession(writer http.ResponseWriter, session models.Session) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(session)
}
