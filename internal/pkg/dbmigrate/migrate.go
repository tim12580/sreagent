// Package dbmigrate 提供基于 golang-migrate 的嵌入式数据库版本化迁移。
//
// 迁移文件存放在 migrations/ 子目录，以版本号命名：
//
//	000001_initial_schema.up.sql   / 000001_initial_schema.down.sql
//	000002_add_xxx.up.sql          / 000002_add_xxx.down.sql
//	...
//
// 每次应用启动时调用 RunMigrations，它会自动把还未执行过的迁移文件
// 按版本顺序应用到数据库，并在 schema_migrations 表中记录版本号。
package dbmigrate

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations 将所有未执行的 SQL 迁移文件应用到数据库。
// db 必须是已连接的 *sql.DB（从 gorm.DB.DB() 获取）。
// 如果数据库已是最新版本，则静默跳过，不返回错误。
//
// 自愈逻辑：如果检测到 schema_migrations 处于 dirty 状态（某次迁移失败），
// 会自动 Force 回退到上一个干净版本后重新 Up。这避免了每次手动到 DB 里改
// schema_migrations 表的痛苦，对开发 / 滚动更新尤其重要。
func RunMigrations(db *sql.DB, dbName string, logger *zap.Logger) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("dbmigrate: create iofs source: %w", err)
	}

	driver, err := mysql.WithInstance(db, &mysql.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		return fmt.Errorf("dbmigrate: create mysql driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "mysql", driver)
	if err != nil {
		return fmt.Errorf("dbmigrate: create migrator: %w", err)
	}

	// Dirty state auto-recovery.
	if version, dirty, verErr := m.Version(); verErr == nil && dirty {
		target := int(version) - 1
		if target < 0 {
			target = 0
		}
		logger.Warn("database migrations in dirty state, auto-forcing to previous clean version",
			zap.Uint("dirty_version", version),
			zap.Int("forced_to", target),
		)
		if ferr := m.Force(target); ferr != nil {
			return fmt.Errorf("dbmigrate: force clean version: %w", ferr)
		}
	}

	logger.Info("running database migrations...")

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("database schema is already up to date")
			return nil
		}
		return fmt.Errorf("dbmigrate: apply migrations: %w", err)
	}

	version, dirty, verErr := m.Version()
	if verErr == nil {
		logger.Info("database migrations applied successfully",
			zap.Uint("schema_version", version),
			zap.Bool("dirty", dirty),
		)
	}

	return nil
}
