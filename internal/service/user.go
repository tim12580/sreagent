package service

import (
	"context"
	"fmt"
	"unicode"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/sreagent/sreagent/internal/model"
	apperr "github.com/sreagent/sreagent/internal/pkg/errors"
	"github.com/sreagent/sreagent/internal/repository"
)

// validatePassword checks password complexity: at least 8 chars, with upper, lower, and digit.
func validatePassword(pwd string) error {
	if len(pwd) < 8 {
		return apperr.WithMessage(apperr.ErrInvalidParam, "password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range pwd {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return apperr.WithMessage(apperr.ErrInvalidParam, "password must contain uppercase, lowercase letters and digits")
	}
	return nil
}

type UserService struct {
	repo   *repository.UserRepository
	logger *zap.Logger
}

func NewUserService(repo *repository.UserRepository, logger *zap.Logger) *UserService {
	return &UserService{repo: repo, logger: logger}
}

// Create creates a new user with a hashed password.
func (s *UserService) Create(ctx context.Context, user *model.User) error {
	// Validate password complexity
	if err := validatePassword(user.Password); err != nil {
		return err
	}

	// Check if username already exists
	existing, _ := s.repo.GetByUsername(ctx, user.Username)
	if existing != nil {
		return apperr.WithMessage(apperr.ErrDuplicateName, fmt.Sprintf("username '%s' already exists", user.Username))
	}

	// Hash the password
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash password", zap.Error(err))
		return apperr.Wrap(apperr.ErrInternal, err)
	}
	user.Password = string(hashedPwd)

	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create user", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	return nil
}

// GetByID retrieves a user by their ID.
func (s *UserService) GetByID(ctx context.Context, id uint) (*model.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.ErrUserNotFound
	}
	return user, nil
}

// List returns a paginated list of users.
func (s *UserService) List(ctx context.Context, page, pageSize int) ([]model.User, int64, error) {
	return s.ListByType(ctx, "", page, pageSize)
}

// ListByType returns a paginated list of users filtered by user_type.
// Pass "" to list all types.
func (s *UserService) ListByType(ctx context.Context, userType string, page, pageSize int) ([]model.User, int64, error) {
	list, total, err := s.repo.ListByType(ctx, userType, page, pageSize)
	if err != nil {
		s.logger.Error("failed to list users", zap.Error(err))
		return nil, 0, apperr.Wrap(apperr.ErrDatabase, err)
	}
	return list, total, nil
}

// CreateVirtual creates a non-human (bot or channel) user.
// Virtual users do not require a real password; one is auto-generated.
func (s *UserService) CreateVirtual(ctx context.Context, user *model.User) error {
	if user.UserType != model.UserTypeBot && user.UserType != model.UserTypeChannel {
		return apperr.WithMessage(apperr.ErrBadRequest, "user_type must be 'bot' or 'channel' for virtual users")
	}

	// Check if username already exists
	existing, _ := s.repo.GetByUsername(ctx, user.Username)
	if existing != nil {
		return apperr.WithMessage(apperr.ErrDuplicateName, fmt.Sprintf("username '%s' already exists", user.Username))
	}

	// Generate a random unusable password (virtual users cannot log in)
	randomPwd := fmt.Sprintf("virtual-%s-%d", user.Username, len(user.Username))
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(randomPwd), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash password for virtual user", zap.Error(err))
		return apperr.Wrap(apperr.ErrInternal, err)
	}
	user.Password = string(hashedPwd)
	user.IsActive = true

	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create virtual user", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	s.logger.Info("virtual user created",
		zap.String("username", user.Username),
		zap.String("user_type", string(user.UserType)),
	)
	return nil
}

// UpdateProfile updates only the self-editable fields of a user (display_name, email, phone, avatar).
func (s *UserService) UpdateProfile(ctx context.Context, userID uint, displayName, email, phone, avatar string) error {
	existing, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return apperr.ErrUserNotFound
	}
	if displayName != "" {
		existing.DisplayName = displayName
	}
	existing.Email = email
	existing.Phone = phone
	existing.Avatar = avatar
	if err := s.repo.Update(ctx, existing); err != nil {
		s.logger.Error("failed to update profile", zap.Error(err), zap.Uint("user_id", userID))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// Update updates user profile fields (not password) - used by admin.
func (s *UserService) Update(ctx context.Context, user *model.User) error {
	existing, err := s.repo.GetByID(ctx, user.ID)
	if err != nil {
		return apperr.ErrUserNotFound
	}

	// Update allowed fields
	existing.DisplayName = user.DisplayName
	existing.Email = user.Email
	existing.Phone = user.Phone
	existing.LarkUserID = user.LarkUserID
	existing.Avatar = user.Avatar
	existing.Role = user.Role

	if err := s.repo.Update(ctx, existing); err != nil {
		s.logger.Error("failed to update user", zap.Error(err), zap.Uint("user_id", user.ID))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	return nil
}

// BindLarkOpenID saves the user's Lark open_id (for bot command identity mapping).
// Passing an empty string clears the binding.
func (s *UserService) BindLarkOpenID(ctx context.Context, userID uint, openID string) error {
	existing, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return apperr.ErrUserNotFound
	}
	existing.LarkUserID = openID
	if err := s.repo.Update(ctx, existing); err != nil {
		s.logger.Error("failed to bind lark open_id", zap.Error(err), zap.Uint("user_id", userID))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	s.logger.Info("lark open_id bound", zap.Uint("user_id", userID), zap.String("open_id", openID))
	return nil
}

// ToggleActive enables or disables a user account.
func (s *UserService) ToggleActive(ctx context.Context, id uint, active bool) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return apperr.ErrUserNotFound
	}

	existing.IsActive = active
	if err := s.repo.Update(ctx, existing); err != nil {
		s.logger.Error("failed to toggle user active status", zap.Error(err), zap.Uint("user_id", id))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	s.logger.Info("user active status toggled", zap.Uint("user_id", id), zap.Bool("is_active", active))
	return nil
}

// ChangePassword changes a user's password after verifying the old password.
func (s *UserService) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return apperr.ErrUserNotFound
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return apperr.WithMessage(apperr.ErrInvalidCreds, "old password is incorrect")
	}

	// Hash new password
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash new password", zap.Error(err))
		return apperr.Wrap(apperr.ErrInternal, err)
	}

	user.Password = string(hashedPwd)
	if err := s.repo.Update(ctx, user); err != nil {
		s.logger.Error("failed to change password", zap.Error(err), zap.Uint("user_id", userID))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	s.logger.Info("user password changed", zap.Uint("user_id", userID))
	return nil
}

// Delete permanently removes a user. The built-in admin user (ID=1) cannot be deleted.
func (s *UserService) Delete(ctx context.Context, id uint) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return apperr.ErrUserNotFound
	}

	// Protect the built-in admin account
	if existing.Username == "admin" {
		return apperr.WithMessage(apperr.ErrBadRequest, "cannot delete the built-in admin user")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete user", zap.Error(err), zap.Uint("user_id", id))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	s.logger.Info("user deleted", zap.Uint("user_id", id))
	return nil
}
