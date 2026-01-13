package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"user-service/internal/telemetry"
)

type MockTelemetryPublisher struct {
	mock.Mock
}

func (m *MockTelemetryPublisher) Publish(ctx context.Context, routingKey string, event telemetry.Envelope) error {
	args := m.Called(ctx, routingKey, event)
	return args.Error(0)
}

func (m *MockTelemetryPublisher) Close() error {
	args := m.Called()
	return args.Error(0)
}
