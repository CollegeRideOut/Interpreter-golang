package models

type User struct {
	Username string `json:"username"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Session struct {
	User  User   `json:"user"`
	Token string `json:"token"`
}
