package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/sreagent/sreagent/internal/model"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) GetByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Preload("Teams").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail looks up a user by email address. Returns gorm.ErrRecordNotFound if not found.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByLarkUserID looks up a user by their Lark open_id stored in lark_user_id.
func (r *UserRepository) GetByLarkUserID(ctx context.Context, larkUserID string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("lark_user_id = ? AND lark_user_id != ''", larkUserID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByOIDCSubject looks up a user by OIDC subject identifier. Returns gorm.ErrRecordNotFound if not found.
func (r *UserRepository) GetByOIDCSubject(ctx context.Context, sub string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("oidc_subject = ?", sub).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) List(ctx context.Context, page, pageSize int) ([]model.User, int64, error) {
	return r.ListByType(ctx, "", page, pageSize)
}

// ListByType returns users filtered by type. Pass "" to list all types.
func (r *UserRepository) ListByType(ctx context.Context, userType string, page, pageSize int) ([]model.User, int64, error) {
	var list []model.User
	var total int64

	query := r.db.WithContext(ctx).Model(&model.User{})
	if userType != "" {
		query = query.Where("user_type = ?", userType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id ASC").Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete permanently removes a user by ID.
func (r *UserRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, id).Error
}
