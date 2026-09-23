package controllers

import "net/http"

type CaloriesController struct {
	foodController *FoodController
}

func NewCaloriesController(foodController *FoodController) *CaloriesController {
	return &CaloriesController{foodController: foodController}
}

func (controller *CaloriesController) Remaining(writer http.ResponseWriter, request *http.Request) {
	controller.foodController.Summary(writer, request)
}
