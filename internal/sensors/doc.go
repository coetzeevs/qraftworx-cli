// Package sensors provides sensor polling for 3D printer telemetry and other
// hardware state. It supports MQTT subscription and Moonraker HTTP polling,
// with concurrent aggregation and graceful degradation.
//
// # Key Types
//
//   - SensorProvider: interface for sensor data sources (Name, Poll, Close).
//     Poll returns nil, nil for unreachable sensors (graceful degradation).
//   - Poller: aggregates multiple SensorProviders and polls them concurrently
//     within a configurable timeout.
//   - MQTTSensor: subscribes to an MQTT topic and caches the last valid message.
//     Validates incoming data against an optional ValueSchema.
//   - MQTTConfig: configuration for MQTT connections with security validation.
//   - MoonrakerSensor: polls the Moonraker HTTP API for Klipper printer status.
//     Parses response into typed PrinterState with range validation.
//   - PrinterState: typed, validated subset of Moonraker data (extruder temp,
//     bed temp, print progress, state enum, filename).
//   - ValueSchema: defines expected types and ranges for MQTT message fields.
//   - FieldSpec: describes a field's type, numeric range, and allowed enum values.
//
// # Architecture Role
//
// The sensors package feeds live hardware state into the Hydrator. The Poller
// implements the hydrator.SensorPoller interface, allowing the Hydrator to
// call PollAll() during context assembly. Sensor data enriches the prompt
// sent to Gemini, giving it awareness of current printer status.
//
// Graceful degradation is a core design principle: if sensors are unreachable,
// the system proceeds without sensor data rather than failing the interaction.
//
// # Security Considerations
//
//   - S5 (MQTT Transport): MQTTConfig.Validate() rejects plaintext broker URLs
//     (tcp://, mqtt://, ws://) unless AllowInsecure is explicitly set to true.
//     Secure protocols (ssl://, tls://, mqtts://, wss://) are accepted by default.
//   - S3 (Data Injection): MQTTSensor validates incoming messages against a
//     ValueSchema before caching. Invalid JSON, out-of-range values, wrong types,
//     and invalid enums are silently rejected. Only validated data reaches the
//     hydration pipeline.
//   - S3 (Typed Data): MoonrakerSensor parses responses into a typed PrinterState
//     with explicit range validation (extruder: 0-300C, bed: 0-150C, progress:
//     0-1.0, state: enum). Raw API strings are never injected into the prompt.
//   - S10 (Timeouts): MoonrakerSensor uses an http.Client with explicit Timeout.
//     Poller enforces a global timeout via context.WithTimeout.
//
// # Testing
//
// Tests use mock SensorProvider implementations with configurable data, errors,
// and delays. MQTT tests use a mock MQTTClient that implements Connect/Subscribe/
// Disconnect and provides simulateMessage() for pushing test data. Moonraker
// tests use httptest.Server with fixture JSON responses.
package sensors
