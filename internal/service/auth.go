package service

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/sreagent/sreagent/internal/config"
	"github.com/sreagent/sreagent/internal/middleware"
	"github.com/sreagent/sreagent/internal/model"
	apperr "github.com/sreagent/sreagent/internal/pkg/errors"
	"github.com/sreagent/sreagent/internal/repository"
)

type AuthService struct {
	userRepo   *repository.UserRepository
	jwtCfg     *config.JWTConfig
	settingSvc *SystemSettingService
	logger     *zap.Logger
}

func NewAuthService(userRepo *repository.UserRepository, jwtCfg *config.JWTConfig, settingSvc *SystemSettingService, logger *zap.Logger) *AuthService {
	return &AuthService{userRepo: userRepo, jwtCfg: jwtCfg, settingSvc: settingSvc, logger: logger}
}

// getExpireSeconds returns the effective JWT expiration in seconds.
// It reads from the system settings DB first, falling back to the config file value.
func (s *AuthService) getExpireSeconds(ctx context.Context) int {
	if s.settingSvc != nil {
		cfg, err := s.settingSvc.GetSecurityConfig(ctx, s.jwtCfg.Expire)
		if err == nil && cfg.JWTExpireSeconds > 0 {
			return cfg.JWTExpireSeconds
		}
	}
	return s.jwtCfg.Expire
}

func (s *AuthService) Login(ctx context.Context, username, password string) (string, int, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return "", 0, apperr.ErrInvalidCreds
	}

	if !user.IsActive {
		return "", 0, apperr.WithMessage(apperr.ErrForbidden, "account is disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", 0, apperr.ErrInvalidCreds
	}

	expire := s.getExpireSeconds(ctx)
	token, err := middleware.GenerateToken(user.ID, user.Username, string(user.Role), s.jwtCfg.Secret, expire)
	if err != nil {
		s.logger.Error("failed to generate token", zap.Error(err))
		return "", 0, apperr.Wrap(apperr.ErrInternal, err)
	}

	return token, expire, nil
}

func (s *AuthService) GetProfile(ctx context.Context, userID uint) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperr.ErrUserNotFound
	}
	return user, nil
}

// RefreshToken validates an existing token (which may be recently expired) and issues a new one.
// The old token is accepted if:
//  1. Its signature is valid.
//  2. It was issued no more than refreshGraceDays ago (default 7 days).
//
// This avoids storing refresh tokens in the DB while still limiting the refresh window.
func (s *AuthService) RefreshToken(ctx context.Context, tokenString string) (string, int, error) {
	const refreshGraceDays = 7

	claims, err := middleware.ParseTokenIgnoreExpiry(tokenString, s.jwtCfg.Secret)
	if err != nil {
		return "", 0, apperr.ErrInvalidCreds
	}

	// Reject tokens issued more than refreshGraceDays days ago
	if claims.IssuedAt == nil || time.Since(claims.IssuedAt.Time) > time.Duration(refreshGraceDays)*24*time.Hour {
		return "", 0, apperr.WithMessage(apperr.ErrInvalidCreds, "token is too old to refresh, please log in again")
	}

	// Re-validate the user still exists and is active
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return "", 0, apperr.ErrUserNotFound
	}
	if !user.IsActive {
		return "", 0, apperr.WithMessage(apperr.ErrForbidden, "account is disabled")
	}

	expire := s.getExpireSeconds(ctx)
	newToken, err := middleware.GenerateToken(user.ID, user.Username, string(user.Role), s.jwtCfg.Secret, expire)
	if err != nil {
		s.logger.Error("failed to generate refresh token", zap.Error(err))
		return "", 0, apperr.Wrap(apperr.ErrInternal, err)
	}

	return newToken, expire, nil
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
