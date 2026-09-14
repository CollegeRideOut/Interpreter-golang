package main

import (
	"log"

	"calorieapp/server"
)

func main() {
	app := server.New()
	log.Println("calorie app listening on http://localhost:8080")
	log.Fatal(app.ListenAndServe(":8080"))
}
