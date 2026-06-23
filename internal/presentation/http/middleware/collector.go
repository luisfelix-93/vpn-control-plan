package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Rate & Errors: Conta o total de requisições agrupadas por método, rota e status
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total de requisições HTTP recebidas",
		},
		[]string{"method", "route", "status"},
	)

	// Duration: Mede o tempo de duração das requisições agrupadas por método e rota
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duração das requisições HTTP em segundos",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func MetricsMiddleware(routePattern string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Inicializa o espião assumindo 200 OK caso o handler não defina um status explicitamente
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Passa a bola para o próximo Handler (nosso ClusterHandler ou PeerHandler)
		next(recorder, r)

		// A requisição terminou! Agora calculamos a duração
		duration := time.Since(start).Seconds()
		statusStr := strconv.Itoa(recorder.statusCode)

		// Atualiza o Prometheus em memória
		httpRequestsTotal.WithLabelValues(r.Method, routePattern, statusStr).Inc()
		httpRequestDuration.WithLabelValues(r.Method, routePattern).Observe(duration)
	}
}
