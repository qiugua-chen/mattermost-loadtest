// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package userentity

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

func (ue *UserEntity) incWebSocketConnections() {
	if ue.metrics != nil {
		ue.metrics.WebSocketConnections.Inc()
	}
}

func (ue *UserEntity) decWebSocketConnections() {
	if ue.metrics != nil {
		ue.metrics.WebSocketConnections.Dec()
	}
}

// incWebSocketEvent 按事件类型累加 WebSocket 事件计数（对应 metrics.go 中的
// WebSocketEvents 指标，由 ltagent 的 /metrics 端点暴露给 Prometheus）。
func (ue *UserEntity) incWebSocketEvent(event string) {
	if ue.metrics != nil {
		ue.metrics.WebSocketEvents.
			WithLabelValues(event).
			Inc()
	}
}

// incWebSocketMissedEvent 累加"丢事件"计数：当收到的事件序列号与预期不一致时调用，
// 用于量化压测期间 WebSocket 推送丢失（序列空洞）。
func (ue *UserEntity) incWebSocketMissedEvent() {
	if ue.metrics != nil {
		ue.metrics.WebSocketMissedEvents.Inc()
	}
}

func (ue *UserEntity) incHTTPErrors(path, method string, status int) {
	if ue.metrics != nil {
		ue.metrics.HTTPErrors.With(prometheus.Labels{
			"path":        path,
			"method":      method,
			"status_code": strconv.Itoa(status),
		}).Inc()
	}
}

func (ue *UserEntity) observeHTTPRequestTimes(elapsed float64) {
	if ue.metrics != nil {
		ue.metrics.HTTPRequestTimes.Observe(elapsed)
	}
}

func (ue *UserEntity) incHTTPTimeouts(path, method string) {
	if ue.metrics != nil {
		ue.metrics.HTTPTimeouts.With(prometheus.Labels{
			"path":   path,
			"method": method,
		}).Inc()
	}
}
