package models

type User struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Age       int    `json:"age"`
	City      string `json:"city"`
	CreatedAt string `json:"created_at"`
}
