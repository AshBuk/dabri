// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package mocks

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MockHotkeyProvider implements KeyboardEventProvider interface for testing
type MockHotkeyProvider struct {
	mu                sync.RWMutex
	isStarted         bool
	isSupported       bool
	registeredHotkeys map[string]func() error
	startError        error
	registerError     error
	simulateEvents    bool
	eventChannel      chan string
	callHistory       []string
	stopCalled        bool
}

// NewMockHotkeyProvider creates a new mock hotkey provider
func NewMockHotkeyProvider() *MockHotkeyProvider {
	return &MockHotkeyProvider{
		isSupported:       true,
		registeredHotkeys: make(map[string]func() error),
		eventChannel:      make(chan string, 100),
		callHistory:       make([]string, 0),
	}
}

// Start simulates starting the hotkey provider
func (m *MockHotkeyProvider) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Always record the call attempt, regardless of outcome
	m.callHistory = append(m.callHistory, "Start")
	if m.startError != nil {
		return m.startError
	}

	if m.isStarted {
		return errors.New("hotkey provider already started")
	}

	m.isStarted = true
	// Create new event channel if it was closed
	if m.eventChannel == nil {
		m.eventChannel = make(chan string, 10)
	}
	// Start event simulation if enabled
	if m.simulateEvents {
		go m.simulateHotkeyEvents()
	}

	return nil
}

// Stop simulates stopping the hotkey provider
func (m *MockHotkeyProvider) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isStarted {
		// Already stopped, don't close channel again
		m.callHistory = append(m.callHistory, "Stop")
		return
	}

	m.isStarted = false
	m.stopCalled = true
	m.callHistory = append(m.callHistory, "Stop")
	// Close event channel only if it's still open
	if m.eventChannel != nil {
		close(m.eventChannel)
		m.eventChannel = nil
	}
}

// RegisterHotkey simulates registering an action, keyed by its stable id.
func (m *MockHotkeyProvider) RegisterHotkey(id, hotkey string, callback func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Always record the call attempt, regardless of outcome
	m.callHistory = append(m.callHistory, "RegisterHotkey")
	if m.registerError != nil {
		return m.registerError
	}

	if !m.isSupported {
		return errors.New("hotkey registration not supported")
	}

	if callback == nil {
		return errors.New("callback cannot be nil")
	}

	m.registeredHotkeys[id] = callback
	return nil
}

// IsSupported returns whether the provider is supported
func (m *MockHotkeyProvider) IsSupported() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isSupported
}

// CaptureOnce returns a fixed combo for tests
func (m *MockHotkeyProvider) CaptureOnce(_ time.Duration) (string, error) {
	return "alt+r", nil
}

// SupportsCaptureOnce returns true for mock provider
func (m *MockHotkeyProvider) SupportsCaptureOnce() bool { return true }

// Test helper methods

// SetSupported configures whether the provider is supported
func (m *MockHotkeyProvider) SetSupported(supported bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isSupported = supported
}

// SetStartError configures the mock to return an error on Start
func (m *MockHotkeyProvider) SetStartError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startError = err
}

// SetRegisterError configures the mock to return an error on RegisterHotkey
func (m *MockHotkeyProvider) SetRegisterError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registerError = err
}

// IsStarted returns whether the provider is started
func (m *MockHotkeyProvider) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isStarted
}

// WasStopCalled returns whether Stop was called
func (m *MockHotkeyProvider) WasStopCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stopCalled
}

// GetRegisteredHotkeys returns all registered hotkeys
func (m *MockHotkeyProvider) GetRegisteredHotkeys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hotkeys := make([]string, 0, len(m.registeredHotkeys))
	for hotkey := range m.registeredHotkeys {
		hotkeys = append(hotkeys, hotkey)
	}
	return hotkeys
}

// IsHotkeyRegistered returns whether a specific hotkey is registered
func (m *MockHotkeyProvider) IsHotkeyRegistered(hotkey string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.registeredHotkeys[hotkey]
	return exists
}

// SimulateHotkeyPress simulates pressing a registered hotkey
func (m *MockHotkeyProvider) SimulateHotkeyPress(hotkey string) error {
	m.mu.RLock()
	callback, exists := m.registeredHotkeys[hotkey]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("hotkey %s not registered", hotkey)
	}
	return callback()
}

// EnableEventSimulation enables automatic hotkey event simulation
func (m *MockHotkeyProvider) EnableEventSimulation() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.simulateEvents = true
}

// DisableEventSimulation disables automatic hotkey event simulation
func (m *MockHotkeyProvider) DisableEventSimulation() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.simulateEvents = false
}

// GetCallHistory returns the history of method calls
func (m *MockHotkeyProvider) GetCallHistory() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	history := make([]string, len(m.callHistory))
	copy(history, m.callHistory)
	return history
}

// Reset clears all mock state
func (m *MockHotkeyProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Close existing channel if it's open
	if m.eventChannel != nil && m.isStarted {
		close(m.eventChannel)
	}

	m.isStarted = false
	m.isSupported = true
	m.registeredHotkeys = make(map[string]func() error)
	m.startError = nil
	m.registerError = nil
	m.simulateEvents = false
	m.callHistory = make([]string, 0)
	m.stopCalled = false
	// Create new event channel
	m.eventChannel = make(chan string, 100)
}

// WasMethodCalled checks if a method was called
func (m *MockHotkeyProvider) WasMethodCalled(method string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, call := range m.callHistory {
		if strings.Contains(call, method) {
			return true
		}
	}
	return false
}

// GetMethodCallCount returns the number of times a method was called
func (m *MockHotkeyProvider) GetMethodCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, call := range m.callHistory {
		if strings.Contains(call, method) {
			count++
		}
	}
	return count
}

// simulateHotkeyEvents simulates hotkey events for testing
func (m *MockHotkeyProvider) simulateHotkeyEvents() {
	// This would simulate hotkey events in a real scenario
	// For testing purposes, we can trigger events manually
}

// MockHotkeyProviderWithErrors provides pre-configured error scenarios
type MockHotkeyProviderWithErrors struct {
	*MockHotkeyProvider
}

// NewMockHotkeyProviderWithErrors creates a mock provider with common error scenarios
func NewMockHotkeyProviderWithErrors() *MockHotkeyProviderWithErrors {
	return &MockHotkeyProviderWithErrors{
		MockHotkeyProvider: NewMockHotkeyProvider(),
	}
}

// SimulateUnsupportedEnvironment simulates an unsupported environment
func (m *MockHotkeyProviderWithErrors) SimulateUnsupportedEnvironment() {
	m.SetSupported(false)
	m.SetStartError(errors.New("hotkey provider not supported in this environment"))
}

// SimulatePermissionDenied simulates permission denied error
func (m *MockHotkeyProviderWithErrors) SimulatePermissionDenied() {
	m.SetStartError(errors.New("permission denied: cannot access input devices"))
}

// SimulateInvalidHotkey simulates invalid hotkey registration
func (m *MockHotkeyProviderWithErrors) SimulateInvalidHotkey() {
	m.SetRegisterError(errors.New("invalid hotkey combination"))
}

// SimulateSystemBusy simulates system busy error
func (m *MockHotkeyProviderWithErrors) SimulateSystemBusy() {
	m.SetStartError(errors.New("system busy: cannot initialize hotkey provider"))
}

// SimulateHotkeyConflict simulates hotkey conflict error
func (m *MockHotkeyProviderWithErrors) SimulateHotkeyConflict() {
	m.SetRegisterError(errors.New("hotkey conflict: key combination already in use"))
}
