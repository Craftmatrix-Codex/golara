package project

import "time"

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Current   bool      `json:"current"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	Error     string    `json:"error,omitempty"`
}
