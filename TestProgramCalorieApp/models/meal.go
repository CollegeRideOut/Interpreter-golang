package models

type Meal string

const (
	Breakfast Meal = "breakfast"
	Lunch     Meal = "lunch"
	Dinner    Meal = "dinner"
	Snacks    Meal = "snacks"
)

func (meal Meal) Valid() bool {
	switch meal {
	case Breakfast, Lunch, Dinner, Snacks:
		return true
	default:
		return false
	}
}
