package sensors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// Task 5.7
func TestMoonrakerSensor_ParseStatus(t *testing.T) {
	fixture, err := os.ReadFile("testdata/moonraker_status.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	sensor := NewMoonrakerSensor("printer1", srv.URL, 5*time.Second)
	data, err := sensor.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}

	// Check parsed fields
	if temp, ok := data["extruder_temp_c"].(float64); !ok || temp != 205.3 {
		t.Errorf("extruder_temp_c=%v, want 205.3", data["extruder_temp_c"])
	}
	if temp, ok := data["bed_temp_c"].(float64); !ok || temp != 60.1 {
		t.Errorf("bed_temp_c=%v, want 60.1", data["bed_temp_c"])
	}
	if progress, ok := data["print_progress"].(float64); !ok || progress != 0.42 {
		t.Errorf("print_progress=%v, want 0.42", data["print_progress"])
	}
	if state, ok := data["state"].(string); !ok || state != "printing" {
		t.Errorf("state=%v, want 'printing'", data["state"])
	}
	if filename, ok := data["filename"].(string); !ok || filename != "benchy.gcode" {
		t.Errorf("filename=%v, want 'benchy.gcode'", data["filename"])
	}
}

// Task 5.8
func TestMoonrakerSensor_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate slow server
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Very short timeout (S10)
	sensor := NewMoonrakerSensor("printer1", srv.URL, 50*time.Millisecond)
	data, err := sensor.Poll(context.Background())
	// Timeout should be treated as unreachable -> graceful nil, nil
	if err != nil {
		t.Errorf("expected graceful nil error, got: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data on timeout, got: %v", data)
	}
}

// Task 5.9
func TestMoonrakerSensor_ValidateRanges(t *testing.T) {
	t.Run("extruder temp too high", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"result": {
					"status": {
						"extruder": {"temperature": 350.0},
						"heater_bed": {"temperature": 60.0},
						"print_stats": {"state": "printing", "filename": "test.gcode"},
						"display_status": {"progress": 0.5}
					}
				}
			}`))
		}))
		defer srv.Close()

		sensor := NewMoonrakerSensor("printer1", srv.URL, 5*time.Second)
		_, err := sensor.Poll(context.Background())
		if err == nil {
			t.Error("expected error for extruder temp > 300")
		}
	})

	t.Run("print progress too high", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"result": {
					"status": {
						"extruder": {"temperature": 200.0},
						"heater_bed": {"temperature": 60.0},
						"print_stats": {"state": "printing", "filename": "test.gcode"},
						"display_status": {"progress": 1.5}
					}
				}
			}`))
		}))
		defer srv.Close()

		sensor := NewMoonrakerSensor("printer1", srv.URL, 5*time.Second)
		_, err := sensor.Poll(context.Background())
		if err == nil {
			t.Error("expected error for print progress > 1.0")
		}
	})

	t.Run("bed temp too high", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"result": {
					"status": {
						"extruder": {"temperature": 200.0},
						"heater_bed": {"temperature": 200.0},
						"print_stats": {"state": "printing", "filename": "test.gcode"},
						"display_status": {"progress": 0.5}
					}
				}
			}`))
		}))
		defer srv.Close()

		sensor := NewMoonrakerSensor("printer1", srv.URL, 5*time.Second)
		_, err := sensor.Poll(context.Background())
		if err == nil {
			t.Error("expected error for bed temp > 150")
		}
	})

	t.Run("negative extruder temp", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"result": {
					"status": {
						"extruder": {"temperature": -5.0},
						"heater_bed": {"temperature": 60.0},
						"print_stats": {"state": "printing", "filename": "test.gcode"},
						"display_status": {"progress": 0.5}
					}
				}
			}`))
		}))
		defer srv.Close()

		sensor := NewMoonrakerSensor("printer1", srv.URL, 5*time.Second)
		_, err := sensor.Poll(context.Background())
		if err == nil {
			t.Error("expected error for negative extruder temp")
		}
	})
}

// Task 5.10
func TestMoonrakerSensor_Unreachable(t *testing.T) {
	// Use a URL that will definitely refuse connections
	sensor := NewMoonrakerSensor("printer1", "http://127.0.0.1:1", 1*time.Second)
	data, err := sensor.Poll(context.Background())
	if err != nil {
		t.Errorf("expected graceful nil error for unreachable, got: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data for unreachable, got: %v", data)
	}
}

// Task 5.11
func TestPrinterState_InvalidEnum(t *testing.T) {
	ps := &PrinterState{
		ExtruderTempC: 200.0,
		BedTempC:      60.0,
		PrintProgress: 0.5,
		State:         PrinterStateEnum("standby"), // invalid
		Filename:      "test.gcode",
	}

	err := ps.Validate()
	if err == nil {
		t.Error("expected error for invalid printer state enum")
	}
}

func TestPrinterState_ValidStates(t *testing.T) {
	states := []PrinterStateEnum{PrinterIdle, PrinterPrinting, PrinterPaused, PrinterError}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			ps := &PrinterState{
				ExtruderTempC: 200.0,
				BedTempC:      60.0,
				PrintProgress: 0.5,
				State:         state,
				Filename:      "test.gcode",
			}
			if err := ps.Validate(); err != nil {
				t.Errorf("valid state %q rejected: %v", state, err)
			}
		})
	}
}

func TestPrinterState_ToMap(t *testing.T) {
	ps := &PrinterState{
		ExtruderTempC: 205.3,
		BedTempC:      60.1,
		PrintProgress: 0.42,
		State:         PrinterPrinting,
		Filename:      "benchy.gcode",
	}

	m := ps.ToMap()
	if m["extruder_temp_c"] != 205.3 {
		t.Errorf("extruder_temp_c=%v, want 205.3", m["extruder_temp_c"])
	}
	if m["state"] != "printing" {
		t.Errorf("state=%v, want 'printing'", m["state"])
	}
}

func TestMoonrakerSensor_Name(t *testing.T) {
	sensor := NewMoonrakerSensor("my-printer", "http://localhost:7125", 5*time.Second)
	if got := sensor.Name(); got != "my-printer" {
		t.Errorf("Name()=%q, want %q", got, "my-printer")
	}
}

func TestMoonrakerSensor_Close(t *testing.T) {
	sensor := NewMoonrakerSensor("printer1", "http://localhost:7125", 5*time.Second)
	if err := sensor.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestMoonrakerSensor_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	sensor := NewMoonrakerSensor("printer1", srv.URL, 5*time.Second)
	data, err := sensor.Poll(context.Background())
	if err != nil {
		t.Errorf("expected graceful nil for bad JSON, got: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data for bad JSON, got: %v", data)
	}
}

func TestMoonrakerSensor_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sensor := NewMoonrakerSensor("printer1", srv.URL, 5*time.Second)
	data, err := sensor.Poll(context.Background())
	if err != nil {
		t.Errorf("expected graceful nil for 500, got: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data for 500, got: %v", data)
	}
}
