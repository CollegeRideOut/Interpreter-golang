package controllers

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"calorieapp/models"
)

type FoodController struct {
	mu             sync.RWMutex
	targetCalories int
	foods          map[string][]models.Food
	authenticate   func(*http.Request) (string, bool)
}

func NewFoodController(targetCalories int, authenticate func(*http.Request) (string, bool)) *FoodController {
	return &FoodController{targetCalories: targetCalories, foods: make(map[string][]models.Food), authenticate: authenticate}
}

func (controller *FoodController) AddFood(writer http.ResponseWriter, request *http.Request) {
	username, ok := controller.authenticate(request)
	if !ok {
		http.Error(writer, "login required", http.StatusUnauthorized)
		return
	}

	var food models.Food
	if err := json.NewDecoder(request.Body).Decode(&food); err != nil {
		http.Error(writer, "invalid food JSON", http.StatusBadRequest)
		return
	}
	food.Name = strings.TrimSpace(food.Name)
	if food.Name == "" || food.Calories <= 0 || !food.Meal.Valid() {
		http.Error(writer, "name, positive calories, and a valid meal are required", http.StatusBadRequest)
		return
	}

	controller.mu.Lock()
	controller.foods[username] = append(controller.foods[username], food)
	controller.mu.Unlock()

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(food)
}

func (controller *FoodController) Summary(writer http.ResponseWriter, request *http.Request) {
	username, ok := controller.authenticate(request)
	if !ok {
		http.Error(writer, "login required", http.StatusUnauthorized)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(controller.CurrentSummary(username))
}

func (controller *FoodController) CurrentSummary(username string) models.DailyPlan {
	controller.mu.RLock()
	defer controller.mu.RUnlock()

	consumed := 0
	for _, food := range controller.foods[username] {
		consumed += food.Calories
	}
	foods := append([]models.Food(nil), controller.foods[username]...)
	summary := models.DailyPlan{
		TargetCalories: controller.targetCalories,
		Consumed:       consumed,
		Remaining:      controller.targetCalories - consumed,
		Foods:          foods,
	}
	return summary
}
