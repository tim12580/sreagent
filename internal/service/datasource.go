package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/pkg/datasource"
	apperr "github.com/sreagent/sreagent/internal/pkg/errors"
	"github.com/sreagent/sreagent/internal/repository"
)

type DataSourceService struct {
	repo   *repository.DataSourceRepository
	logger *zap.Logger
}

func NewDataSourceService(repo *repository.DataSourceRepository, logger *zap.Logger) *DataSourceService {
	return &DataSourceService{repo: repo, logger: logger}
}

func (s *DataSourceService) Create(ctx context.Context, ds *model.DataSource) error {
	// Check if name already exists
	existing, _ := s.repo.GetByName(ctx, ds.Name)
	if existing != nil {
		return apperr.WithMessage(apperr.ErrDuplicateName, fmt.Sprintf("datasource '%s' already exists", ds.Name))
	}

	if err := s.repo.Create(ctx, ds); err != nil {
		s.logger.Error("failed to create datasource", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	return nil
}

func (s *DataSourceService) GetByID(ctx context.Context, id uint) (*model.DataSource, error) {
	ds, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.ErrDSNotFound
	}
	return ds, nil
}

func (s *DataSourceService) List(ctx context.Context, dsType string, page, pageSize int) ([]model.DataSource, int64, error) {
	return s.repo.List(ctx, dsType, page, pageSize)
}

func (s *DataSourceService) Update(ctx context.Context, ds *model.DataSource) error {
	existing, err := s.repo.GetByID(ctx, ds.ID)
	if err != nil {
		return apperr.ErrDSNotFound
	}

	// Update fields
	existing.Name = ds.Name
	existing.Type = ds.Type
	existing.Endpoint = ds.Endpoint
	existing.Description = ds.Description
	existing.Labels = ds.Labels
	existing.AuthType = ds.AuthType
	if ds.AuthConfig != "" {
		existing.AuthConfig = ds.AuthConfig
	}
	existing.HealthCheckInterval = ds.HealthCheckInterval

	if err := s.repo.Update(ctx, existing); err != nil {
		s.logger.Error("failed to update datasource", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	return nil
}

func (s *DataSourceService) Delete(ctx context.Context, id uint) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return apperr.ErrDSNotFound
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete datasource", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	return nil
}

// HealthCheckResult is the richer result returned to API callers.
type HealthCheckResult struct {
	Status    model.DataSourceStatus `json:"status"`
	Message   string                 `json:"message"`
	LatencyMs int64                  `json:"latency_ms"`
	Version   string                 `json:"version,omitempty"`
}

// HealthCheck performs a multi-phase health probe against the datasource.
// It updates the datasource status in the DB and returns the full result.
func (s *DataSourceService) HealthCheck(ctx context.Context, id uint) (*HealthCheckResult, error) {
	ds, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.ErrDSNotFound
	}

	checker, err := datasource.NewChecker(string(ds.Type))
	if err != nil {
		s.logger.Warn("unsupported datasource type for health check",
			zap.String("type", string(ds.Type)),
		)
		return &HealthCheckResult{Status: model.DSStatusUnknown, Message: "unsupported datasource type"}, nil
	}

	hr := checker.CheckHealth(ctx, ds.Endpoint, ds.AuthType, ds.AuthConfig)

	status := model.DSStatusHealthy
	if !hr.Healthy {
		status = model.DSStatusUnhealthy
		s.logger.Warn("datasource health check failed",
			zap.String("datasource", ds.Name),
			zap.String("message", hr.Message),
			zap.Int64("latency_ms", hr.LatencyMs),
		)
	} else {
		s.logger.Info("datasource health check passed",
			zap.String("datasource", ds.Name),
			zap.String("version", hr.Version),
			zap.Int64("latency_ms", hr.LatencyMs),
		)
	}

	ds.Status = status
	if hr.Healthy && hr.Version != "" {
		ds.Version = hr.Version
	}
	if err := s.repo.Update(ctx, ds); err != nil {
		s.logger.Error("failed to persist datasource health status",
			zap.String("datasource", ds.Name),
			zap.Error(err),
		)
	}

	return &HealthCheckResult{
		Status:    status,
		Message:   hr.Message,
		LatencyMs: hr.LatencyMs,
		Version:   hr.Version,
	}, nil
}

// QueryResponse holds the result of a datasource query test.
type QueryResponse struct {
	ResultType string            `json:"result_type"`
	Series     []QuerySeriesItem `json:"series"`
	RawCount   int               `json:"raw_count"`
}

// QuerySeriesItem represents a single series in the query response.
type QuerySeriesItem struct {
	Labels map[string]string `json:"labels"`
	Values []QueryDataPoint  `json:"values"`
}

// QueryDataPoint represents a single data point in a series.
type QueryDataPoint struct {
	Timestamp int64   `json:"ts"`
	Value     float64 `json:"value"`
}

// QueryDatasource executes an expression against the given datasource for testing.
func (s *DataSourceService) QueryDatasource(ctx context.Context, dsID uint, expression string, queryTime time.Time) (*QueryResponse, error) {
	ds, err := s.repo.GetByID(ctx, dsID)
	if err != nil {
		return nil, apperr.ErrDSNotFound
	}

	qc := datasource.NewQueryClient()
	resp := &QueryResponse{}

	switch ds.Type {
	case model.DSTypePrometheus, model.DSTypeVictoriaMetrics:
		results, err := qc.InstantQuery(ctx, ds.Endpoint, ds.AuthType, ds.AuthConfig, expression, queryTime)
		if err != nil {
			return nil, apperr.WithMessage(apperr.ErrExternalAPI, err.Error())
		}
		resp.ResultType = "vector"
		for _, r := range results {
			item := QuerySeriesItem{Labels: r.Labels}
			for _, v := range r.Values {
				item.Values = append(item.Values, QueryDataPoint{Timestamp: v.Timestamp.UnixMilli(), Value: v.Value})
			}
			resp.Series = append(resp.Series, item)
		}
	case model.DSTypeVictoriaLogs:
		results, err := datasource.VictoriaLogsInstantQuery(ctx, ds.Endpoint, ds.AuthType, ds.AuthConfig, expression)
		if err != nil {
			return nil, apperr.WithMessage(apperr.ErrExternalAPI, err.Error())
		}
		resp.ResultType = "logs"
		if len(results) > 0 && len(results[0].Values) > 0 {
			resp.RawCount = int(results[0].Values[0].Value)
		}
	default:
		return nil, apperr.WithMessage(apperr.ErrInvalidParam, "expression testing not supported for "+string(ds.Type))
	}

	// Limit series count
	if len(resp.Series) > 100 {
		resp.Series = resp.Series[:100]
	}
	return resp, nil
}
