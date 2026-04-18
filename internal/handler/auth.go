package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/sreagent/sreagent/internal/service"
)

type AuthHandler struct {
	svc     *service.AuthService
	userSvc *service.UserService
}

// SetUserService wires the user service for /me endpoints.
func (h *AuthHandler) SetUserService(svc *service.UserService) {
	h.userSvc = svc
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	token, expiresIn, err := h.svc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, LoginResponse{
		Token:     token,
		ExpiresIn: expiresIn,
	})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := GetCurrentUserID(c)
	user, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, user)
}

// UpdateMe updates the current user's own profile (display_name, email, phone, avatar).
func (h *AuthHandler) UpdateMe(c *gin.Context) {
	if h.userSvc == nil {
		ErrorWithMessage(c, 50000, "user service not available")
		return
	}
	userID := GetCurrentUserID(c)

	var req struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		Avatar      string `json:"avatar"` // base64 data URL or preset key
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	// Validate avatar size: base64 data URLs must not exceed 200 KB.
	// A 200 KB binary file encodes to ~272,000 base64 characters.
	if len(req.Avatar) > 0 && req.Avatar[:5] == "data:" && len(req.Avatar) > 272000 {
		ErrorWithMessage(c, 10001, "avatar image must not exceed 200 KB")
		return
	}

	if err := h.userSvc.UpdateProfile(c.Request.Context(), userID, req.DisplayName, req.Email, req.Phone, req.Avatar); err != nil {
		Error(c, err)
		return
	}
	Success(c, nil)
}

// Refresh issues a new JWT token given a valid or recently-expired token.
// POST /api/v1/auth/refresh  — no JWTAuth middleware (token may be expired).
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	token, expiresIn, err := h.svc.RefreshToken(c.Request.Context(), req.Token)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, LoginResponse{
		Token:     token,
		ExpiresIn: expiresIn,
	})
}

// BindLark saves the current user's Lark open_id for bot command identity mapping.
// PUT /me/lark-bind   body: {"lark_open_id": "ou_xxx"}
func (h *AuthHandler) BindLark(c *gin.Context) {
	if h.userSvc == nil {
		ErrorWithMessage(c, 50000, "user service not available")
		return
	}
	userID := GetCurrentUserID(c)

	var req struct {
		LarkOpenID string `json:"lark_open_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	if err := h.userSvc.BindLarkOpenID(c.Request.Context(), userID, req.LarkOpenID); err != nil {
		Error(c, err)
		return
	}
	Success(c, nil)
}

// ChangeMyPassword changes the current user's own password.
func (h *AuthHandler) ChangeMyPassword(c *gin.Context) {
	if h.userSvc == nil {
		ErrorWithMessage(c, 50000, "user service not available")
		return
	}
	userID := GetCurrentUserID(c)

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	if err := h.userSvc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		Error(c, err)
		return
	}
	Success(c, nil)
}
