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
	WebSocketEvents       *prometheus.CounterVec // 按事件类型统计已接收的 WebSocket 事件数
	WebSocketMissedEvents prometheus.Counter     // 检测到的 WebSocket 序列空洞（疑似丢事件）总数
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

	// loadtest_websocket_events_received_total{event="posted"|...}
	// 用于核对消息场景下各 agent 实际收到的事件量（业务层吞吐核对）。
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

	// loadtest_websocket_missed_events_total
	// 序列号跳变即计数一次，是"服务端过载丢推送"的关键探针指标。
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
