package sensors

import (
	"context"
	"log"
	"sync"
	"time"
)

// Poller aggregates multiple sensor providers and polls them with timeout.
type Poller struct {
	providers []SensorProvider
	timeout   time.Duration
}

// NewPoller creates a Poller with the given timeout.
func NewPoller(timeout time.Duration, providers ...SensorProvider) *Poller {
	return &Poller{
		providers: providers,
		timeout:   timeout,
	}
}

// sensorResult holds the outcome of polling a single sensor.
type sensorResult struct {
	name string
	data map[string]any
}

// PollAll queries all sensors within the timeout.
// Returns available data; unavailable sensors are logged and skipped.
func (p *Poller) PollAll(ctx context.Context) map[string]any {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	results := make(chan sensorResult, len(p.providers))
	var wg sync.WaitGroup

	for _, provider := range p.providers {
		wg.Add(1)
		go func(prov SensorProvider) {
			defer wg.Done()

			data, err := prov.Poll(ctx)
			if err != nil {
				log.Printf("sensor %q poll error: %v", prov.Name(), err)
				return
			}
			if data == nil {
				log.Printf("sensor %q unavailable, skipping", prov.Name())
				return
			}

			results <- sensorResult{name: prov.Name(), data: data}
		}(provider)
	}

	// Close results channel once all goroutines finish
	go func() {
		wg.Wait()
		close(results)
	}()

	merged := make(map[string]any)
	for r := range results {
		merged[r.name] = r.data
	}

	return merged
}
