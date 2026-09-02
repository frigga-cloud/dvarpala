// Package users provides user API response structures
package users

import (
	"time"

	"dvarpala/internal/database/models"
)

// UserResponse is the public shape of a user.
//
// Models are deliberately not returned directly: they carry internal fields
// (DeletedAt), unloaded associations that serialise as null noise, and - in
// other models - secrets such as private keys. Every endpoint should map to a
// response type it controls.
type UserResponse struct {
	ID         uint       `json:"id"`
	Email      string     `json:"email"`
	FullName   string     `json:"full_name"`
	Department string     `json:"department"`
	Status     string     `json:"status"`
	Groups     []string   `json:"groups"`
	LastLogin  *time.Time `json:"last_login"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ListResponse wraps a page of users.
type ListResponse struct {
	Users []UserResponse `json:"users"`
	Total int            `json:"total"`
}

// ErrorResponse is the shape of every error this package returns.
type ErrorResponse struct {
	Error string `json:"error"`
}

// toResponse converts a model to its public form.
func toResponse(u models.User) UserResponse {
	groups := make([]string, 0, len(u.Groups))
	for _, g := range u.Groups {
		groups = append(groups, g.Name)
	}

	return UserResponse{
		ID:         u.ID,
		Email:      u.Email,
		FullName:   u.FullName,
		Department: u.Department,
		Status:     string(u.Status),
		Groups:     groups,
		LastLogin:  u.LastLogin,
		CreatedAt:  u.CreatedAt,
	}
}

// toResponseList converts a slice of models.
func toResponseList(list []models.User) []UserResponse {
	out := make([]UserResponse, 0, len(list))
	for _, u := range list {
		out = append(out, toResponse(u))
	}
	return out
}
