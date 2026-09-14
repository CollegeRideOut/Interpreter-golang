package server

import (
	"net/http"

	"calorieapp/controllers"
)

type Server struct {
	mux      *http.ServeMux
	foods    *controllers.FoodController
	calories *controllers.CaloriesController
}

func New() *Server {
	foods := controllers.NewFoodController(2200)
	calories := controllers.NewCaloriesController(foods)
	mux := http.NewServeMux()
	server := &Server{mux: mux, foods: foods, calories: calories}
	mux.HandleFunc("GET /api/summary", foods.Summary)
	mux.HandleFunc("POST /api/foods", foods.AddFood)
	mux.HandleFunc("GET /api/calories/remaining", calories.Remaining)
	mux.Handle("/", http.FileServer(http.Dir(".")))
	return server
}

func (server *Server) ListenAndServe(address string) error {
	return http.ListenAndServe(address, server.mux)
}
