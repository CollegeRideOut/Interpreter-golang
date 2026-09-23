package server

import (
	"net/http"

	"calorieapp/controllers"
)

type Server struct {
	mux      *http.ServeMux
	foods    *controllers.FoodController
	calories *controllers.CaloriesController
	users    *controllers.UserController
}

func New() *Server {
	users := controllers.NewUserController()
	foods := controllers.NewFoodController(2200, users.Authenticate)
	calories := controllers.NewCaloriesController(foods)
	mux := http.NewServeMux()
	server := &Server{mux: mux, foods: foods, calories: calories, users: users}
	mux.HandleFunc("GET /api/summary", foods.Summary)
	mux.HandleFunc("POST /api/foods", foods.AddFood)
	mux.HandleFunc("GET /api/calories/remaining", calories.Remaining)
	mux.HandleFunc("POST /api/register", users.Register)
	mux.HandleFunc("POST /api/login", users.Login)
	mux.Handle("/", http.FileServer(http.Dir(".")))
	return server
}

func (server *Server) ListenAndServe(address string) error {
	return http.ListenAndServe(address, server.mux)
}
