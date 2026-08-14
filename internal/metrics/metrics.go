package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Requests         *prometheus.CounterVec
	SearchDuration   prometheus.Histogram
	CompileDuration  prometheus.Histogram
	Executions       prometheus.Counter
	ExecutionSuccess prometheus.Counter
	DBErrors         prometheus.Counter
	MCPRequests      *prometheus.CounterVec
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{Requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "skillbox_requests_total", Help: "HTTP requests."}, []string{"method", "route", "status"}), SearchDuration: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "skillbox_search_duration_seconds", Help: "Structured search duration."}), CompileDuration: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "skillbox_compile_duration_seconds", Help: "Compilation duration."}), Executions: prometheus.NewCounter(prometheus.CounterOpts{Name: "skillbox_skill_executions_total", Help: "Reported executions."}), ExecutionSuccess: prometheus.NewCounter(prometheus.CounterOpts{Name: "skillbox_skill_execution_success_total", Help: "Successful executions."}), DBErrors: prometheus.NewCounter(prometheus.CounterOpts{Name: "skillbox_db_errors_total", Help: "Database errors."}), MCPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "skillbox_mcp_requests_total", Help: "MCP requests."}, []string{"profile", "method", "status"})}
	reg.MustRegister(m.Requests, m.SearchDuration, m.CompileDuration, m.Executions, m.ExecutionSuccess, m.DBErrors, m.MCPRequests)
	return m
}
