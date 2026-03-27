package sensors

import "context"

// SensorProvider fetches current state from a sensor source.
type SensorProvider interface {
	// Name returns the sensor name (for logging and hydration context).
	Name() string

	// Poll fetches current state with the given timeout.
	// Returns nil, nil if the sensor is unreachable (graceful degradation).
	Poll(ctx context.Context) (map[string]any, error)

	// Close releases resources.
	Close() error
}
