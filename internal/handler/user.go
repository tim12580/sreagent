package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/service"
)

type UserHandler struct {
	svc      *service.UserService
	auditSvc *service.AuditLogService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) SetAuditService(svc *service.AuditLogService) {
	h.auditSvc = svc
}

// CreateUserRequest is the request body for creating a user.
type CreateUserRequest struct {
	Username    string     `json:"username" binding:"required"`
	Password    string     `json:"password" binding:"required,min=8"`
	DisplayName string     `json:"display_name"`
	Email       string     `json:"email" binding:"omitempty,email"`
	Phone       string     `json:"phone"`
	LarkUserID  string     `json:"lark_user_id"`
	Avatar      string     `json:"avatar"`
	Role        model.Role `json:"role"`
}

// UpdateUserRequest is the request body for updating a user.
type UpdateUserRequest struct {
	DisplayName string     `json:"display_name"`
	Email       string     `json:"email" binding:"omitempty,email"`
	Phone       string     `json:"phone"`
	LarkUserID  string     `json:"lark_user_id"`
	Avatar      string     `json:"avatar"`
	Role        model.Role `json:"role"`
}

// ToggleActiveRequest is the request body for enabling/disabling a user.
type ToggleActiveRequest struct {
	IsActive bool `json:"is_active"`
}

// ChangePasswordRequest is the request body for changing a user's password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// Create creates a new user.
func (h *UserHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	role := req.Role
	if role == "" {
		role = model.RoleMember
	}

	user := &model.User{
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Phone:       req.Phone,
		LarkUserID:  req.LarkUserID,
		Avatar:      req.Avatar,
		Role:        role,
		IsActive:    true,
	}

	if err := h.svc.Create(c.Request.Context(), user); err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		uid := GetCurrentUserID(c)
		h.auditSvc.Record(&model.AuditLog{
			UserID: &uid, Username: GetCurrentUsername(c),
			Action: model.AuditActionCreate, ResourceType: model.AuditResourceUser,
			ResourceID: &user.ID, ResourceName: user.Username, IP: c.ClientIP(),
		})
	}
	Success(c, user)
}

// Get returns a user by ID.
func (h *UserHandler) Get(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	user, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, user)
}

// List returns a paginated list of users.
// Supports optional ?user_type=human|bot|channel query param.
func (h *UserHandler) List(c *gin.Context) {
	pq := GetPageQuery(c)
	userType := c.Query("user_type")

	list, total, err := h.svc.ListByType(c.Request.Context(), userType, pq.Page, pq.PageSize)
	if err != nil {
		Error(c, err)
		return
	}

	SuccessPage(c, list, total, pq.Page, pq.PageSize)
}

// Update updates a user's profile.
func (h *UserHandler) Update(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	user := &model.User{
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Phone:       req.Phone,
		LarkUserID:  req.LarkUserID,
		Avatar:      req.Avatar,
		Role:        req.Role,
	}
	user.ID = id

	if err := h.svc.Update(c.Request.Context(), user); err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		uid := GetCurrentUserID(c)
		h.auditSvc.Record(&model.AuditLog{
			UserID: &uid, Username: GetCurrentUsername(c),
			Action: model.AuditActionUpdate, ResourceType: model.AuditResourceUser,
			ResourceID: &id, IP: c.ClientIP(),
		})
	}
	Success(c, user)
}

// ToggleActive enables or disables a user account.
func (h *UserHandler) ToggleActive(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req ToggleActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	if err := h.svc.ToggleActive(c.Request.Context(), id, req.IsActive); err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		uid := GetCurrentUserID(c)
		detail := "deactivated"
		if req.IsActive {
			detail = "activated"
		}
		h.auditSvc.Record(&model.AuditLog{
			UserID: &uid, Username: GetCurrentUsername(c),
			Action: model.AuditActionToggle, ResourceType: model.AuditResourceUser,
			ResourceID: &id, Detail: detail, IP: c.ClientIP(),
		})
	}
	Success(c, nil)
}

// ChangePassword allows an admin to reset a user's password.
// PATCH /users/:id/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	if err := h.svc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		Error(c, err)
		return
	}

	Success(c, nil)
}

// CreateVirtualUserRequest is the request body for creating a virtual (non-human) user.
type CreateVirtualUserRequest struct {
	Username     string         `json:"username" binding:"required"`
	DisplayName  string         `json:"display_name"`
	UserType     model.UserType `json:"user_type" binding:"required"`
	NotifyTarget string         `json:"notify_target"`
	Description  string         `json:"description"`
	Role         model.Role     `json:"role"`
}

// CreateVirtual creates a non-human user (bot or channel type).
// POST /users/virtual
func (h *UserHandler) CreateVirtual(c *gin.Context) {
	var req CreateVirtualUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	role := req.Role
	if role == "" {
		role = model.RoleMember
	}

	user := &model.User{
		Username:     req.Username,
		DisplayName:  req.DisplayName,
		UserType:     req.UserType,
		NotifyTarget: req.NotifyTarget,
		Role:         role,
	}

	if err := h.svc.CreateVirtual(c.Request.Context(), user); err != nil {
		Error(c, err)
		return
	}

	Success(c, user)
}

// DeleteUser permanently deletes a user by ID.
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		uid := GetCurrentUserID(c)
		h.auditSvc.Record(&model.AuditLog{
			UserID: &uid, Username: GetCurrentUsername(c),
			Action: model.AuditActionDelete, ResourceType: model.AuditResourceUser,
			ResourceID: &id, IP: c.ClientIP(),
		})
	}
	Success(c, nil)
}
