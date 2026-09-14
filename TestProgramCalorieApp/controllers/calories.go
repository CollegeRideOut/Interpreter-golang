package controllers

import (
	"encoding/json"
	"net/http"
)

type CaloriesController struct {
	foodController *FoodController
}

func NewCaloriesController(foodController *FoodController) *CaloriesController {
	return &CaloriesController{foodController: foodController}
}

func (controller *CaloriesController) Remaining(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(controller.foodController.CurrentSummary())
}
