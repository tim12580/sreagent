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
//
// 可靠性要求：
//   - 部分失败（dirty）的迁移会在下次启动自动回退到上一个干净版本重试
//   - "效果已达成"的错误（重复列、重复键、表已存在、删除不存在的列/键）
//     被视作幂等成功，自动跳过该版本继续前进
//   - 这两条合起来让启动路径在面对人工干预过的 DB / 升级回退场景时依旧
//     能够把 schema 推进到目标状态，不再要求运维登进 DB 手动改
//     schema_migrations 表。
package dbmigrate

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	mysqlmig "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// idempotentMySQLErrors are error codes that mean "the change you were trying
// to apply is already applied" — safe to skip and mark the version as done.
//
// References: https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html
//
//	1050 ER_TABLE_EXISTS_ERROR            — CREATE TABLE X (already exists)
//	1051 ER_BAD_TABLE_ERROR               — DROP TABLE X (doesn't exist)
//	1060 ER_DUP_FIELDNAME                 — ADD COLUMN X (already exists)
//	1061 ER_DUP_KEYNAME                   — ADD INDEX X  (already exists)
//	1091 ER_CANT_DROP_FIELD_OR_KEY        — DROP COLUMN/KEY X (doesn't exist)
//	1146 ER_NO_SUCH_TABLE                 — any op on missing table (for down)
var idempotentMySQLErrors = map[uint16]struct{}{
	1050: {}, 1051: {}, 1060: {}, 1061: {}, 1091: {}, 1146: {},
}

func isIdempotentMySQLError(err error) bool {
	if err == nil {
		return false
	}
	// go-sql-driver's *mysql.MySQLError is often wrapped by golang-migrate
	// in a struct whose Unwrap() chain eventually reaches it. Use errors.As.
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		_, ok := idempotentMySQLErrors[me.Number]
		return ok
	}
	// Fallback: some wrappers stringify the underlying error. Match on the
	// well-known prefix "Error <code> (<sqlstate>)" which go-sql-driver
	// emits regardless of the wrapping layer.
	s := err.Error()
	for code := range idempotentMySQLErrors {
		if strings.Contains(s, fmt.Sprintf("Error %d ", code)) ||
			strings.Contains(s, fmt.Sprintf("Error %d:", code)) ||
			strings.Contains(s, fmt.Sprintf("(errno %d)", code)) {
			return true
		}
	}
	return false
}

// RunMigrations 将所有未执行的 SQL 迁移文件应用到数据库。
// db 必须是已连接的 *sql.DB（从 gorm.DB.DB() 获取）。
// 如果数据库已是最新版本，则静默跳过，不返回错误。
func RunMigrations(db *sql.DB, dbName string, logger *zap.Logger) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("dbmigrate: create iofs source: %w", err)
	}

	driver, err := mysqlmig.WithInstance(db, &mysqlmig.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		return fmt.Errorf("dbmigrate: create mysql driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "mysql", driver)
	if err != nil {
		return fmt.Errorf("dbmigrate: create migrator: %w", err)
	}

	// Dirty state auto-recovery: roll back one version then retry Up. The
	// typical dirty cause is a single failed ALTER; once fixed by an upgrade
	// of the app image, this lets the service come back up without manual
	// DB surgery.
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

	// Idempotent step-through: we walk one version at a time so that
	// "already-applied" errors (duplicate column etc.) can be detected and
	// skipped per-migration without aborting the whole chain. `Steps(1)` is
	// exactly what the regular `Up()` does internally but gives us per-step
	// control.
	for {
		stepErr := m.Steps(1)
		if stepErr == nil {
			continue
		}
		// Past the last migration file: iofs returns fs.ErrNotExist (or
		// os.ErrNotExist, which is the same sentinel). ErrNoChange surfaces
		// when Up() has nothing to do at the top of the chain.
		if errors.Is(stepErr, migrate.ErrNoChange) ||
			errors.Is(stepErr, fs.ErrNotExist) ||
			errors.Is(stepErr, os.ErrNotExist) {
			break
		}
		// Belt-and-suspenders: some wrapping layers stringify without
		// preserving the sentinel. Match on the message as a fallback.
		if strings.Contains(stepErr.Error(), "file does not exist") ||
			strings.Contains(stepErr.Error(), "no migration") {
			break
		}

		if isIdempotentMySQLError(stepErr) {
			ver, dirty, _ := m.Version()
			logger.Warn("migration step hit an idempotent MySQL error — schema already in target state, skipping",
				zap.Uint("version", ver),
				zap.Bool("dirty", dirty),
				zap.Error(stepErr),
			)
			if dirty {
				// Mark this version clean and move on. Force to the same
				// version in non-dirty state.
				if ferr := m.Force(int(ver)); ferr != nil {
					return fmt.Errorf("dbmigrate: force-clean version %d after idempotent error: %w", ver, ferr)
				}
			}
			continue
		}

		return fmt.Errorf("dbmigrate: apply migrations: %w", stepErr)
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

