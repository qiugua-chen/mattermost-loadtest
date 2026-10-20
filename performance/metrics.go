// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package performance

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsNamespace     = "loadtest"
	metricsSubSystemHTTP = "http"
	metricsSubSystemWS   = "websocket"
)

type UserEntityMetrics struct {
	HTTPRequestTimes      prometheus.Histogram
	HTTPErrors            *prometheus.CounterVec
	HTTPTimeouts          *prometheus.CounterVec
	WebSocketConnections  prometheus.Gauge
	WebSocketEvents       *prometheus.CounterVec
	WebSocketMissedEvents prometheus.Counter
}

type Metrics struct {
	registry  *prometheus.Registry
	ueMetrics UserEntityMetrics
}

func NewMetrics() *Metrics {
	var m Metrics
	m.registry = prometheus.NewRegistry()

	m.ueMetrics.HTTPRequestTimes = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubSystemHTTP,
		Name:      "request_time",
		Help:      "The time taken to execute client requests.",
	})
	m.registry.MustRegister(m.ueMetrics.HTTPRequestTimes)

	m.ueMetrics.HTTPErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubSystemHTTP,
		Name:      "errors_total",
		Help:      "The total number of HTTP client errors.",
	},
		[]string{"path", "method", "status_code"})
	m.registry.MustRegister(m.ueMetrics.HTTPErrors)

	m.ueMetrics.HTTPTimeouts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubSystemHTTP,
		Name:      "timeouts_total",
		Help:      "The total number of HTTP client timeouts.",
	},
		[]string{"path", "method"})
	m.registry.MustRegister(m.ueMetrics.HTTPTimeouts)

	m.ueMetrics.WebSocketConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubSystemWS,
		Name:      "connections_total",
		Help:      "The total number of active WebSocket connections.",
	})
	m.registry.MustRegister(m.ueMetrics.WebSocketConnections)

	m.ueMetrics.WebSocketEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubSystemWS,
			Name:      "events_received_total",
			Help:      "The total number of received WebSocket events.",
		},
		[]string{"event"},
	)
	m.registry.MustRegister(m.ueMetrics.WebSocketEvents)

	m.ueMetrics.WebSocketMissedEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubSystemWS,
			Name:      "missed_events_total",
			Help:      "The total number of detected WebSocket sequence gaps.",
		},
	)
	m.registry.MustRegister(m.ueMetrics.WebSocketMissedEvents)

	return &m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) UserEntityMetrics() *UserEntityMetrics {
	return &m.ueMetrics
}
