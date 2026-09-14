package models

type Food struct {
	Name     string `json:"name"`
	Calories int    `json:"calories"`
	Meal     Meal   `json:"meal"`
}

type DailyPlan struct {
	TargetCalories int    `json:"targetCalories"`
	Consumed       int    `json:"consumed"`
	Remaining      int    `json:"remaining"`
	Foods          []Food `json:"foods"`
}
