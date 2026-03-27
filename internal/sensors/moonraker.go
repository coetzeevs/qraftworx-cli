package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PrinterStateEnum represents valid Klipper printer states.
type PrinterStateEnum string

const (
	// PrinterIdle means the printer is idle.
	PrinterIdle PrinterStateEnum = "idle"
	// PrinterPrinting means the printer is actively printing.
	PrinterPrinting PrinterStateEnum = "printing"
	// PrinterPaused means the print is paused.
	PrinterPaused PrinterStateEnum = "paused"
	// PrinterError means the printer is in an error state.
	PrinterError PrinterStateEnum = "error"
)

// validPrinterStates is the canonical set of allowed states.
var validPrinterStates = map[PrinterStateEnum]bool{
	PrinterIdle:     true,
	PrinterPrinting: true,
	PrinterPaused:   true,
	PrinterError:    true,
}

// PrinterState is the validated, typed subset of Moonraker data (S3).
// Only typed fields are injected into the hydrator, never raw JSON.
type PrinterState struct {
	ExtruderTempC float64          `json:"extruder_temp_c"`
	BedTempC      float64          `json:"bed_temp_c"`
	PrintProgress float64          `json:"print_progress"`
	State         PrinterStateEnum `json:"state"`
	Filename      string           `json:"filename"`
}

// Validate checks PrinterState for schema compliance (S3).
func (ps *PrinterState) Validate() error {
	if !validPrinterStates[ps.State] {
		return fmt.Errorf("printer state: invalid state %q (S3)", ps.State)
	}
	if ps.ExtruderTempC < 0 || ps.ExtruderTempC > 300 {
		return fmt.Errorf("printer state: extruder temp %.1f out of range [0, 300] (S3)", ps.ExtruderTempC)
	}
	if ps.BedTempC < 0 || ps.BedTempC > 150 {
		return fmt.Errorf("printer state: bed temp %.1f out of range [0, 150] (S3)", ps.BedTempC)
	}
	if ps.PrintProgress < 0 || ps.PrintProgress > 1.0 {
		return fmt.Errorf("printer state: print progress %.2f out of range [0, 1.0] (S3)", ps.PrintProgress)
	}
	return nil
}

// ToMap converts PrinterState to a map for hydration context.
func (ps *PrinterState) ToMap() map[string]any {
	return map[string]any{
		"extruder_temp_c": ps.ExtruderTempC,
		"bed_temp_c":      ps.BedTempC,
		"print_progress":  ps.PrintProgress,
		"state":           string(ps.State),
		"filename":        ps.Filename,
	}
}

// moonrakerStatusResponse models the Moonraker /printer/objects/query response.
type moonrakerStatusResponse struct {
	Result struct {
		Status struct {
			Extruder struct {
				Temperature float64 `json:"temperature"`
			} `json:"extruder"`
			HeaterBed struct {
				Temperature float64 `json:"temperature"`
			} `json:"heater_bed"`
			PrintStats struct {
				State    string `json:"state"`
				Filename string `json:"filename"`
			} `json:"print_stats"`
			DisplayStatus struct {
				Progress float64 `json:"progress"`
			} `json:"display_status"`
		} `json:"status"`
	} `json:"result"`
}

// MoonrakerSensor polls the Moonraker HTTP API for printer status.
type MoonrakerSensor struct {
	name    string
	baseURL string
	client  *http.Client // S10: with explicit timeout
}

// NewMoonrakerSensor creates a MoonrakerSensor with an explicit HTTP timeout (S10).
func NewMoonrakerSensor(name, baseURL string, timeout time.Duration) *MoonrakerSensor {
	return &MoonrakerSensor{
		name:    name,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the sensor name.
func (m *MoonrakerSensor) Name() string {
	return m.name
}

// Poll fetches the current printer state from Moonraker.
// Returns nil, nil if the server is unreachable (graceful degradation).
func (m *MoonrakerSensor) Poll(ctx context.Context) (map[string]any, error) {
	url := m.baseURL + "/printer/objects/query?extruder&heater_bed&print_stats&display_status"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("moonraker: create request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		// Unreachable is graceful: return nil, nil (not an application error)
		return nil, nil //nolint:nilerr // graceful degradation for unreachable sensor
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, nil // non-200 treated as unavailable
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil //nolint:nilerr // read failure treated as unavailable
	}

	var status moonrakerStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, nil //nolint:nilerr // malformed JSON treated as unavailable
	}

	ps := &PrinterState{
		ExtruderTempC: status.Result.Status.Extruder.Temperature,
		BedTempC:      status.Result.Status.HeaterBed.Temperature,
		PrintProgress: status.Result.Status.DisplayStatus.Progress,
		State:         PrinterStateEnum(status.Result.Status.PrintStats.State),
		Filename:      status.Result.Status.PrintStats.Filename,
	}

	if err := ps.Validate(); err != nil {
		return nil, err
	}

	return ps.ToMap(), nil
}

// Close releases resources. No persistent connections for HTTP polling.
func (m *MoonrakerSensor) Close() error {
	return nil
}
