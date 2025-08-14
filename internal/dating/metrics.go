package dating

import (
    "context"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    dateRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "dating_date_requests_total",
            Help: "Total number of date requests",
        },
        []string{"status"},
    )
    
    matchesTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "dating_matches_total",
            Help: "Total number of matches created",
        },
    )
    
    hotpicksGenerated = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "dating_hotpicks_generated_total",
            Help: "Total number of hotpicks generated",
        },
    )
    
    compatibilityScores = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "dating_compatibility_scores",
            Help:    "Distribution of compatibility scores",
            Buckets: prometheus.LinearBuckets(0, 0.1, 11),
        },
    )
    
    responseTime = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "dating_response_time_seconds",
            Help: "Response time for date requests",
        },
        []string{"action"},
    )
)

type Metrics struct {
    DateRequestsSent       int64
    DateRequestsAccepted   int64
    DateRequestsDeclined   int64
    MatchesCreated         int64
    MatchesUnmatched       int64
    HotpicksGenerated      int64
    HotpicksActedOn        int64
    AverageCompatScore     float64
    AverageResponseTime    time.Duration
    ActiveUsers            int64
    DailyActiveUsers       int64
}

type MetricsService struct {
    repo Repository
}

func NewMetricsService(repo Repository) *MetricsService {
    return &MetricsService{repo: repo}
}

func (m *MetricsService) CollectMetrics(ctx context.Context) (*Metrics, error) {
    metrics := &Metrics{}
    
    // Collect from database
    query := `
        SELECT 
            COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending,
            COUNT(CASE WHEN status = 'accepted' THEN 1 END) as accepted,
            COUNT(CASE WHEN status = 'declined' THEN 1 END) as declined,
            AVG(EXTRACT(EPOCH FROM (responded_at - created_at))) as avg_response_time
        FROM date_requests
        WHERE created_at > NOW() - INTERVAL '30 days'
    `
    
    err := m.repo.db.QueryRowContext(ctx, query).Scan(
        &metrics.DateRequestsSent,
        &metrics.DateRequestsAccepted,
        &metrics.DateRequestsDeclined,
        &metrics.AverageResponseTime,
    )
    if err != nil {
        return nil, err
    }
    
    // Update Prometheus metrics
    dateRequestsTotal.WithLabelValues("accepted").Add(float64(metrics.DateRequestsAccepted))
    dateRequestsTotal.WithLabelValues("declined").Add(float64(metrics.DateRequestsDeclined))
    
    return metrics, nil
}

func RecordDateRequest(status string) {
    dateRequestsTotal.WithLabelValues(status).Inc()
}

func RecordMatch() {
    matchesTotal.Inc()
}

func RecordCompatibilityScore(score float64) {
    compatibilityScores.Observe(score)
}

func RecordResponseTime(action string, duration time.Duration) {
    responseTime.WithLabelValues(action).Observe(duration.Seconds())
}