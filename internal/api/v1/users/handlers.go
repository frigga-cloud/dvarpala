// Package users provides user management API handlers
package users

import (
	"errors"
	"net/http"

	"dvarpala/internal/services"

	"github.com/gin-gonic/gin"
)

// Handler serves the /users endpoints.
type Handler struct {
	users *services.UserService
}

// NewHandler creates a handler backed by the given service.
func NewHandler(users *services.UserService) *Handler {
	return &Handler{users: users}
}

// Register attaches this handler's routes to the given group.
func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/users")
	g.GET("", h.List)
	g.POST("", h.Create)
}

// List returns every user.
//
// TODO(phase-2): paginate. The requirement doc specifies page/per_page.
func (h *Handler) List(c *gin.Context) {
	list, err := h.users.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListResponse{
		Users: toResponseList(list),
		Total: len(list),
	})
}

// Create adds a user.
func (h *Handler) Create(c *gin.Context) {
	var req services.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	user, err := h.users.CreateUser(c.Request.Context(), req)
	switch {
	case errors.Is(err, services.ErrUserExists):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
	case errors.Is(err, services.ErrInvalidEmail):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	default:
		c.JSON(http.StatusCreated, toResponse(*user))
	}
}
