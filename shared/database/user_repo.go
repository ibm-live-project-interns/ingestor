package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// UserRepository handles user database operations
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		logger.Error("Failed to create user: %v", err)
		return fmt.Errorf("failed to create user: %w", err)
	}
	logger.Info("Created user: %s", user.Username)
	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

// GetByUsername retrieves a user by username
func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return &user, nil
}

// GetByVerificationToken retrieves a user by verification token
func (r *UserRepository) GetByVerificationToken(token string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("verification_token = ?", token).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by verification token: %w", err)
	}
	return &user, nil
}

// GetByResetToken retrieves a user by password reset token
func (r *UserRepository) GetByResetToken(token string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("reset_token = ?", token).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by reset token: %w", err)
	}
	return &user, nil
}

// Update updates a user
func (r *UserRepository) Update(user *models.User) error {
	if err := r.db.Save(user).Error; err != nil {
		logger.Error("Failed to update user %d: %v", user.ID, err)
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

// UpdateLoginAttempt updates failed login attempts
func (r *UserRepository) UpdateLoginAttempt(userID uint, failed bool) error {
	updates := make(map[string]interface{})

	if failed {
		// Increment failed attempts
		if err := r.db.Model(&models.User{}).
			Where("id = ?", userID).
			Update("failed_attempts", gorm.Expr("failed_attempts + 1")).Error; err != nil {
			return err
		}

		// Check if should be locked
		var user models.User
		if err := r.db.First(&user, userID).Error; err != nil {
			return err
		}

		if user.FailedAttempts >= 5 {
			lockUntil := time.Now().Add(15 * time.Minute)
			return r.db.Model(&models.User{}).
				Where("id = ?", userID).
				Update("locked_until", &lockUntil).Error
		}
	} else {
		// Reset failed attempts on successful login
		now := time.Now()
		updates["failed_attempts"] = 0
		updates["locked_until"] = nil
		updates["last_login"] = &now

		if err := r.db.Model(&models.User{}).
			Where("id = ?", userID).
			Updates(updates).Error; err != nil {
			return err
		}
	}

	return nil
}

// UserFilter represents filter options for querying users
type UserFilter struct {
	Search string // Search by name, email, or username
	Role   string
	Limit  int
	Offset int
}

// GetAll retrieves users with optional filtering and pagination
func (r *UserRepository) GetAll(filter UserFilter) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := r.db.Model(&models.User{})

	// Apply search filter (name, email, or username)
	if filter.Search != "" {
		search := "%" + EscapeLike(filter.Search) + "%"
		query = query.Where(
			"username ILIKE ? OR email ILIKE ? OR first_name ILIKE ? OR last_name ILIKE ?",
			search, search, search, search,
		)
	}

	// Apply role filter
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}

	// Count total before pagination
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count users: %v", err)
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by created_at descending
	if err := query.Order("created_at DESC").Find(&users).Error; err != nil {
		logger.Error("Failed to list users: %v", err)
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	return users, total, nil
}

// SoftDelete soft deletes a user by ID
func (r *UserRepository) SoftDelete(id uint) error {
	result := r.db.Delete(&models.User{}, id)
	if result.Error != nil {
		logger.Error("Failed to delete user %d: %v", id, result.Error)
		return fmt.Errorf("failed to delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found: %d", id)
	}
	logger.Info("Soft deleted user: %d", id)
	return nil
}

// GetByUsernameOrEmail retrieves an active user with email alerts enabled,
// matching by username, email, or full name (first_name || ' ' || last_name).
// This is a targeted single-row query used by ticket email notifications to
// avoid loading all users into memory.
func (r *UserRepository) GetByUsernameOrEmail(identifier string) (*models.User, error) {
	var user models.User
	err := r.db.Where(
		"(username = ? OR email = ? OR CONCAT(first_name, ' ', last_name) = ?) AND is_active = true AND email_alerts = true",
		identifier, identifier, identifier,
	).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by identifier %q: %w", identifier, err)
	}
	return &user, nil
}

// UpdateFields updates specific fields of a user
func (r *UserRepository) UpdateFields(id uint, updates map[string]interface{}) error {
	result := r.db.Model(&models.User{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		logger.Error("Failed to update user %d: %v", id, result.Error)
		return fmt.Errorf("failed to update user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found: %d", id)
	}
	return nil
}

// SessionRepository handles session database operations
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new session
func (r *SessionRepository) Create(session *models.Session) error {
	if err := r.db.Create(session).Error; err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// InvalidateByToken invalidates a session by token
func (r *SessionRepository) InvalidateByToken(token string) error {
	result := r.db.Model(&models.Session{}).
		Where("token = ?", token).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to invalidate session: %w", result.Error)
	}
	return nil
}

// InvalidateAllForUser invalidates all sessions for a user
func (r *SessionRepository) InvalidateAllForUser(userID uint) error {
	result := r.db.Model(&models.Session{}).
		Where("user_id = ?", userID).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to invalidate sessions: %w", result.Error)
	}
	logger.Info("Invalidated all sessions for user %d", userID)
	return nil
}

// GetActiveByToken retrieves an active session by token
func (r *SessionRepository) GetActiveByToken(token string) (*models.Session, error) {
	var session models.Session
	if err := r.db.Where("token = ? AND is_active = ? AND expires_at > ?",
		token, true, time.Now()).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &session, nil
}

// CleanupExpired removes expired sessions
func (r *SessionRepository) CleanupExpired() error {
	result := r.db.Where("expires_at < ?", time.Now()).Delete(&models.Session{})
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup sessions: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		logger.Info("Cleaned up %d expired sessions", result.RowsAffected)
	}
	return nil
}
