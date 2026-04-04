package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	OIDC     OIDCConfig     `mapstructure:"oidc"`
	Log      LogConfig      `mapstructure:"log"`
	Engine   EngineConfig   `mapstructure:"engine"`
}

// EngineConfig holds configuration for the native alert evaluator.
type EngineConfig struct {
	Enabled      bool `mapstructure:"enabled"`       // default true
	SyncInterval int  `mapstructure:"sync_interval"` // how often to sync rules from DB (seconds, default 30)
}

type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Mode         string `mapstructure:"mode"`
	ExternalBase string `mapstructure:"external_base"` // external base URL for links in notifications
}

func (s *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// ExternalURL returns the external base URL for the platform.
// Falls back to http://host:port if not explicitly configured.
func (s *ServerConfig) ExternalURL() string {
	if s.ExternalBase != "" {
		return s.ExternalBase
	}
	return fmt.Sprintf("http://%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	Driver       string `mapstructure:"driver"`
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	Charset      string `mapstructure:"charset"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxLifetime  int    `mapstructure:"max_lifetime"`
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		d.Username, d.Password, d.Host, d.Port, d.Database, d.Charset)
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

func (r *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
	Expire int    `mapstructure:"expire"`
	Issuer string `mapstructure:"issuer"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
	File   string `mapstructure:"file"`
}

// OIDCConfig holds configuration for OIDC/Keycloak integration.
// When Enabled is true, the platform supports "Login with SSO" alongside local auth.
type OIDCConfig struct {
	Enabled       bool              `mapstructure:"enabled"`        // master switch
	IssuerURL     string            `mapstructure:"issuer_url"`     // e.g. https://keycloak.example.com/realms/sreagent
	ClientID      string            `mapstructure:"client_id"`      // OIDC client ID
	ClientSecret  string            `mapstructure:"client_secret"`  // OIDC client secret
	RedirectURL   string            `mapstructure:"redirect_url"`   // e.g. https://sreagent.example.com/api/v1/auth/oidc/callback
	Scopes        []string          `mapstructure:"scopes"`         // default: ["openid","profile","email"]
	RoleClaim     string            `mapstructure:"role_claim"`     // JWT claim path for roles, default "realm_access.roles"
	RoleMapping   map[string]string `mapstructure:"role_mapping"`   // Keycloak role → SREAgent role, e.g. {"sre-admin":"admin","sre-member":"member"}
	DefaultRole   string            `mapstructure:"default_role"`   // role when no mapping matches, default "viewer"
	AutoProvision bool              `mapstructure:"auto_provision"` // create user on first OIDC login, default true
}

// Load reads config from file and environment variables.
// Config file path can be specified, defaults to configs/config.yaml.
func Load(cfgFile string) (*Config, error) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath(".")
	}

	// Allow environment variable overrides
	// e.g. SREAGENT_DATABASE_HOST=xxx
	viper.SetEnvPrefix("SREAGENT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Explicitly bind env vars for sensitive fields that may be absent from
	// the config file. Viper's AutomaticEnv only works for keys already
	// registered in the config file; BindEnv ensures these are always read
	// from the environment regardless.
	_ = viper.BindEnv("database.password", "SREAGENT_DATABASE_PASSWORD")
	_ = viper.BindEnv("database.host", "SREAGENT_DATABASE_HOST")
	_ = viper.BindEnv("database.port", "SREAGENT_DATABASE_PORT")
	_ = viper.BindEnv("database.username", "SREAGENT_DATABASE_USERNAME")
	_ = viper.BindEnv("redis.password", "SREAGENT_REDIS_PASSWORD")
	_ = viper.BindEnv("redis.host", "SREAGENT_REDIS_HOST")
	_ = viper.BindEnv("redis.port", "SREAGENT_REDIS_PORT")
	_ = viper.BindEnv("jwt.secret", "SREAGENT_JWT_SECRET")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
