package model

// DataSourceType defines the type of monitoring data source.
type DataSourceType string

const (
	DSTypePrometheus      DataSourceType = "prometheus"
	DSTypeVictoriaMetrics DataSourceType = "victoriametrics"
	DSTypeZabbix          DataSourceType = "zabbix"
	DSTypeVictoriaLogs    DataSourceType = "victorialogs"
)

// DataSourceStatus defines the health status of a data source.
type DataSourceStatus string

const (
	DSStatusHealthy   DataSourceStatus = "healthy"
	DSStatusUnhealthy DataSourceStatus = "unhealthy"
	DSStatusUnknown   DataSourceStatus = "unknown"
)

// DataSource represents an external monitoring data source.
type DataSource struct {
	BaseModel
	Name        string           `json:"name" gorm:"uniqueIndex;size:128;not null"`
	Type        DataSourceType   `json:"type" gorm:"size:32;not null;index"`
	Endpoint    string           `json:"endpoint" gorm:"size:512;not null"`
	Description string           `json:"description" gorm:"size:512"`
	Labels      JSONLabels       `json:"labels" gorm:"type:json"`
	Status      DataSourceStatus `json:"status" gorm:"size:32;default:unknown"`
	// Auth config (stored encrypted in production)
	AuthType   string `json:"auth_type" gorm:"size:32"` // none, basic, bearer, api_key
	AuthConfig string `json:"-" gorm:"type:text"`       // JSON: {"username":"x","password":"y"} or {"token":"x"}
	// Health check
	HealthCheckInterval int    `json:"health_check_interval" gorm:"default:60"` // seconds
	IsEnabled           bool   `json:"is_enabled" gorm:"default:true"`
	Version             string `json:"version" gorm:"size:128"` // populated by health check
}

func (DataSource) TableName() string {
	return "datasources"
}
