//go:build linux

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/AshBuk/dabri/v2/hotkeys/utils"
	"github.com/AshBuk/dabri/v2/internal/logger"
	evdev "github.com/holoplot/go-evdev"
)

// Implements KeyboardEventProvider using the Linux evdev interface
type EvdevKeyboardProvider struct {
	devices       []*evdev.InputDevice
	callbacks     map[string]func() error
	stopListening chan bool
	isListening   bool
	modifierState map[string]bool // Tracks the state of modifier keys
	mutex         sync.RWMutex
	logger        logger.Logger
	wg            *sync.WaitGroup // Tracks device listener goroutines; pointer to allow safe replacement on restart
	stopping      int32           // atomic flag to soften expected errors during shutdown
}

// Create a new EvdevKeyboardProvider instance
func NewEvdevKeyboardProvider(logger logger.Logger) *EvdevKeyboardProvider {
	return &EvdevKeyboardProvider{
		callbacks:     make(map[string]func() error),
		stopListening: make(chan bool),
		isListening:   false,
		modifierState: make(map[string]bool),
		logger:        logger,
		wg:            &sync.WaitGroup{},
	}
}

// Check if the evdev provider is supported on the current system
func (p *EvdevKeyboardProvider) IsSupported() bool {
	// Attempt to find keyboard devices without keeping them open
	devices, err := p.findKeyboardDevices()
	if err != nil || len(devices) == 0 {
		return false
	}
	// Close the devices after the check
	for _, dev := range devices {
		if err := dev.Close(); err != nil {
			p.logger.Error("Failed to close evdev device: %v", err)
		}
	}
	return true
}

// Find all input devices that are identified as keyboards
func (p *EvdevKeyboardProvider) findKeyboardDevices() ([]*evdev.InputDevice, error) {
	devices := []*evdev.InputDevice{}

	devicePaths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, fmt.Errorf("failed to list input devices: %w", err)
	}
	// Filter for devices that are keyboards
	for _, path := range devicePaths {
		dev, err := evdev.Open(path)
		if err != nil {
			p.logger.Warning("Could not open input device %s: %v", path, err)
			continue
		}

		// Check if the device is a keyboard to avoid capturing mice, etc
		devName, _ := dev.Name()
		if strings.Contains(strings.ToLower(devName), "keyboard") ||
			isKeyboardDevice(dev) {
			devices = append(devices, dev)
		} else {
			if err := dev.Close(); err != nil {
				p.logger.Error("Failed to close evdev device: %v", err)
			}
		}
	}
	return devices, nil
}

// Check if a device exposes typical keyboard-related key codes
func isKeyboardDevice(dev *evdev.InputDevice) bool {
	// Check for EV_KEY capability
	types := dev.CapableTypes()
	hasKeyType := false
	for _, evType := range types {
		if evType == evdev.EV_KEY {
			hasKeyType = true
			break
		}
	}
	if !hasKeyType {
		return false
	}
	// The presence of common letter keys strongly indicates a keyboard
	events := dev.CapableEvents(evdev.EV_KEY)
	common := map[uint16]bool{16: true, 30: true, 44: true, 57: true} // q, a, z, space
	for _, code := range events {
		if common[uint16(code)] {
			return true
		}
	}
	return false
}

// Start listening for keyboard events from all found devices
func (p *EvdevKeyboardProvider) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isListening {
		return fmt.Errorf("evdev keyboard provider already started")
	}
	var err error
	// (Re)initialize stop channel on each start to avoid using a closed channel
	p.stopListening = make(chan bool)
	// clear stopping flag
	atomic.StoreInt32(&p.stopping, 0)
	p.devices, err = p.findKeyboardDevices()
	if err != nil {
		return fmt.Errorf("failed to find keyboard devices: %w", err)
	}

	if len(p.devices) == 0 {
		return fmt.Errorf("no keyboard devices found")
	}
	p.isListening = true
	// Fresh WaitGroup per Start() so previous Stop() timeout goroutine doesn't race with Add()
	p.wg = &sync.WaitGroup{}
	// Start a listener goroutine for each device
	wg := p.wg
	for i := range p.devices {
		idx := i
		wg.Add(1)
		go func(deviceIdx int) {
			defer wg.Done()
			p.listenDevice(deviceIdx)
		}(idx)
	}
	return nil
}

// listenDevice listens to one device events and exits on stop signal or critical read error
func (p *EvdevKeyboardProvider) listenDevice(idx int) {
	// Capture device and stop channel references under lock to avoid races with Stop()
	p.mutex.RLock()
	if p.devices == nil || idx >= len(p.devices) {
		p.mutex.RUnlock()
		return
	}
	dev := p.devices[idx]
	stopCh := p.stopListening
	// Set non-blocking mode under lock so it completes before Stop() can Close() the device
	_ = dev.NonBlock()
	p.mutex.RUnlock()

	for {
		select {
		case <-stopCh:
			return
		default:
		}
		// Check if stopping flag is set
		if atomic.LoadInt32(&p.stopping) == 1 {
			return
		}
		event, err := dev.ReadOne()
		if err != nil {
			// EAGAIN/EWOULDBLOCK means no data available in non-blocking mode
			if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			// Exit on device read error to avoid infinite loops
			// "file already closed" is expected when device is released for rebind/reload
			if strings.Contains(err.Error(), "file already closed") || atomic.LoadInt32(&p.stopping) == 1 {
				p.logger.Warning("Device read ended: %v", err)
			} else {
				p.logger.Error("Device read error: %v", err)
			}
			return
		}
		if event.Type == evdev.EV_KEY {
			p.handleKeyEvent(idx, event)
		}
	}
}

// Process a key event from a device
func (p *EvdevKeyboardProvider) handleKeyEvent(_ int, event *evdev.InputEvent) {
	keyCode := int(event.Code)
	keyName := utils.GetKeyName(keyCode)
	if keyName == "" {
		keyName = fmt.Sprintf("KEY_%d", keyCode)
	}
	// Track the state of modifier keys
	if utils.IsModifierKey(keyName) {
		// Value 1 = key down, 0 = key up
		p.mutex.Lock()
		p.modifierState[strings.ToLower(keyName)] = (event.Value == 1)
		p.mutex.Unlock()
	}
	// Only process key-down events for hotkey triggers
	if event.Value != 1 {
		return
	}
	// Create a thread-safe copy of callbacks and modifier state
	p.mutex.RLock()
	callbacksCopy := make(map[string]func() error, len(p.callbacks))
	for k, v := range p.callbacks {
		callbacksCopy[k] = v
	}
	modState := make(map[string]bool, len(p.modifierState))
	for k, v := range p.modifierState {
		modState[k] = v
	}
	p.mutex.RUnlock()
	// Check for registered hotkeys
	for hotkeyStr, callback := range callbacksCopy {
		hotkey := utils.ParseHotkey(hotkeyStr)
		// Check if the pressed key matches the hotkey's main key
		if strings.EqualFold(hotkey.Key, keyName) {
			// Check if all required modifiers are also pressed
			allModifiersPressed := true
			for _, mod := range hotkey.Modifiers {
				if !utils.IsModifierPressed(mod, modState) {
					allModifiersPressed = false
					break
				}
			}
			if allModifiersPressed {
				go func(cb func() error) {
					if err := cb(); err != nil {
						p.logger.Error("Hotkey callback error: %v", err)
					}
				}(callback)
			}
		}
	}
}

// Register a callback for a hotkey
func (p *EvdevKeyboardProvider) RegisterHotkey(hotkey string, callback func() error) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.callbacks[hotkey] = callback
	return nil
}

// Stop listening for keyboard events and close all devices
func (p *EvdevKeyboardProvider) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.isListening {
		return
	}
	// Mark as stopping first to suppress error logs from ReadOne
	atomic.StoreInt32(&p.stopping, 1)
	// Close all device handles FIRST to unblock any pending ReadOne() calls
	for _, dev := range p.devices {
		if err := dev.Close(); err != nil {
			// If already closed, this is expected during shutdown
			p.logger.Warning("Evdev device close (ignored): %v", err)
		}
	}
	// Signal all listener goroutines to stop after devices are closed
	if p.stopListening != nil {
		// make Stop idempotent: close only once and nil the channel
		close(p.stopListening)
		p.stopListening = nil
	}

	p.isListening = false
	// With non-blocking mode, goroutines exit immediately after Close()
	// Short timeout as safety net
	// Capture current wg pointer so the goroutine waits on the correct instance
	// even if a new Start() creates a replacement before this Wait() completes
	currentWg := p.wg
	done := make(chan struct{})
	go func() {
		currentWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All listener goroutines exited cleanly
		p.logger.Info("Evdev listeners stopped cleanly")
		p.devices = nil
		atomic.StoreInt32(&p.stopping, 0)
	case <-time.After(50 * time.Millisecond):
		// Timeout - unexpected, but proceed to allow restart
		p.logger.Warning("Evdev stop timeout (50ms) - proceeding with restart")
		oldDevices := p.devices
		p.devices = nil
		atomic.StoreInt32(&p.stopping, 0)

		// Cleanup old devices in background after they unblock
		go func() {
			<-done
			for _, dev := range oldDevices {
				_ = dev.Close()
			}
		}()
	}
}

// captureSession holds state for a single capture operation
type captureSession struct {
	devices       []*evdev.InputDevice
	modState      map[string]bool
	modStateMutex sync.Mutex
	resultCh      chan string
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// processKeyEvent processes a single key event during capture
func (cs *captureSession) processKeyEvent(ev *evdev.InputEvent) (shouldStop bool, result string) {
	if ev.Type != evdev.EV_KEY {
		return false, ""
	}

	keyCode := int(ev.Code)
	keyName := utils.GetKeyName(keyCode)
	if keyName == "" || strings.HasPrefix(keyName, "BTN_") {
		return false, ""
	}
	// Track modifier state
	if utils.IsModifierKey(keyName) {
		cs.modStateMutex.Lock()
		cs.modState[strings.ToLower(keyName)] = (ev.Value == 1)
		cs.modStateMutex.Unlock()
		return false, ""
	}
	// Only process key-down events
	if ev.Value != 1 {
		return false, ""
	}
	// Check for cancellation
	cs.modStateMutex.Lock()
	isCancelled := utils.CheckCancelCondition(keyName, cs.modState)
	cs.modStateMutex.Unlock()

	if isCancelled {
		return true, ""
	}
	// Build hotkey string
	cs.modStateMutex.Lock()
	mods := utils.BuildModifierState(cs.modState)
	cs.modStateMutex.Unlock()

	combo := utils.BuildHotkeyString(mods, keyName)
	if err := utils.ValidateHotkey(combo); err != nil {
		return false, ""
	}

	return true, combo
}

// listenDevice listens for events on a single device during capture (non-blocking mode)
func (cs *captureSession) listenDevice(idx int) {
	defer cs.wg.Done()

	dev := cs.devices[idx]
	// Set non-blocking mode for fast cleanup on stop
	_ = dev.NonBlock()

	for {
		select {
		case <-cs.stopCh:
			return
		default:
		}

		ev, err := dev.ReadOne()
		if err != nil {
			// EAGAIN/EWOULDBLOCK means no data available in non-blocking mode
			if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
				// Short sleep to avoid busy-loop, then retry
				time.Sleep(5 * time.Millisecond)
				continue
			}
			// Any other error (including closed device) - exit
			return
		}

		shouldStop, result := cs.processKeyEvent(ev)
		if !shouldStop {
			continue
		}
		select {
		case cs.resultCh <- result:
		default:
		}
		return
	}
}

// cleanup closes devices and waits for goroutines to finish
func (cs *captureSession) cleanup() {
	for _, d := range cs.devices {
		_ = d.Close()
	}
	captureDone := make(chan struct{})
	go func() {
		cs.wg.Wait()
		close(captureDone)
	}()

	select {
	case <-captureDone:
	case <-time.After(50 * time.Millisecond):
	}
}

// SupportsCaptureOnce returns true as evdev supports one-shot hotkey capture
func (p *EvdevKeyboardProvider) SupportsCaptureOnce() bool { return true }

// Start a short-lived capture session to get a single hotkey combination
// Note: This creates a fresh isolated session, not tied to the regular provider lifecycle
func (p *EvdevKeyboardProvider) CaptureOnce(timeout time.Duration) (string, error) {
	devices, err := p.findKeyboardDevices()
	if err != nil {
		p.logger.Error("CaptureOnce: Failed to find keyboard devices: %v", err)
		return "", fmt.Errorf("failed to find keyboard devices: %w", err)
	}
	if len(devices) == 0 {
		p.logger.Error("CaptureOnce: No keyboard devices found")
		return "", fmt.Errorf("no keyboard devices found")
	}

	session := &captureSession{
		devices:  devices,
		modState: make(map[string]bool),
		resultCh: make(chan string, 1),
		stopCh:   make(chan struct{}),
	}
	// Start listeners for all devices
	for i := range devices {
		session.wg.Add(1)
		go session.listenDevice(i)
	}
	// Wait for result or timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var result string
	select {
	case result = <-session.resultCh:
		close(session.stopCh)
	case <-timer.C:
		close(session.stopCh)
		result = ""
	}
	session.cleanup()
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("capture cancelled")
	}
	return result, nil
}
