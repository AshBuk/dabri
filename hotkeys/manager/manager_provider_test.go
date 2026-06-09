// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package manager

import (
	"errors"
	"sync"
	"testing"

	"github.com/AshBuk/dabri/v2/hotkeys/interfaces"
	"github.com/AshBuk/dabri/v2/hotkeys/mocks"
)

// MockFactory for testing dependency injection
type MockFactory struct {
	mu        sync.RWMutex
	providers map[string]interfaces.KeyboardEventProvider
	createErr error
}

// NewMockFactory creates a new mock factory
func NewMockFactory() *MockFactory {
	return &MockFactory{
		providers: make(map[string]interfaces.KeyboardEventProvider),
	}
}

// RegisterProvider registers a provider in the factory
func (f *MockFactory) RegisterProvider(name string, provider interfaces.KeyboardEventProvider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[name] = provider
}

// CreateProvider creates a provider by name
func (f *MockFactory) CreateProvider(name string) (interfaces.KeyboardEventProvider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.createErr != nil {
		return nil, f.createErr
	}

	provider, exists := f.providers[name]
	if !exists {
		return nil, errors.New("provider not found: " + name)
	}

	return provider, nil
}

// SetCreateError sets an error for CreateProvider
func (f *MockFactory) SetCreateError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createErr = err
}

func TestHotkeyFactory_DependencyInjection(t *testing.T) {
	factory := NewMockFactory()

	t.Run("RegisterAndCreateProvider", func(t *testing.T) {
		mockProvider := mocks.NewMockHotkeyProvider()
		factory.RegisterProvider("mock", mockProvider)
		provider, err := factory.CreateProvider("mock")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if provider != mockProvider {
			t.Error("expected same provider instance")
		}
	})

	t.Run("CreateNonExistentProvider", func(t *testing.T) {
		provider, err := factory.CreateProvider("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent provider")
		}
		if provider != nil {
			t.Error("expected nil provider for non-existent provider")
		}
	})

	t.Run("MultipleProviders", func(t *testing.T) {
		mock1 := mocks.NewMockHotkeyProvider()
		mock2 := mocks.NewMockHotkeyProvider()

		factory.RegisterProvider("provider1", mock1)
		factory.RegisterProvider("provider2", mock2)
		provider1, err := factory.CreateProvider("provider1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		provider2, err := factory.CreateProvider("provider2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if provider1 == provider2 {
			t.Error("expected different provider instances")
		}
	})

	t.Run("OverrideProvider", func(t *testing.T) {
		mock1 := mocks.NewMockHotkeyProvider()
		mock2 := mocks.NewMockHotkeyProvider()

		factory.RegisterProvider("override", mock1)
		factory.RegisterProvider("override", mock2)
		provider, err := factory.CreateProvider("override")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if provider != mock2 {
			t.Error("expected overridden provider")
		}
	})
}

func TestHotkeyFactory_ErrorHandling(t *testing.T) {
	factory := NewMockFactory()

	t.Run("FactoryError", func(t *testing.T) {
		expectedError := errors.New("factory error")
		factory.SetCreateError(expectedError)
		provider, err := factory.CreateProvider("any")
		if err == nil {
			t.Error("expected error but got none")
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("expected error %q, got %q", expectedError.Error(), err.Error())
		}
		if provider != nil {
			t.Error("expected nil provider on error")
		}
	})

	t.Run("NilProvider", func(t *testing.T) {
		factory.SetCreateError(nil) // Reset error from previous test
		factory.RegisterProvider("nil", nil)

		provider, err := factory.CreateProvider("nil")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if provider != nil {
			t.Error("expected nil provider")
		}
	})
}

func TestHotkeyFactory_InterfaceCompliance(t *testing.T) {
	factory := NewMockFactory()

	t.Run("MockProviderCompliance", func(t *testing.T) {
		mockProvider := mocks.NewMockHotkeyProvider()
		factory.RegisterProvider("mock", mockProvider)

		provider, err := factory.CreateProvider("mock")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify interface compliance
		var _ interfaces.KeyboardEventProvider = provider

		// Test interface methods
		if !provider.IsSupported() {
			t.Error("expected provider to be supported")
		}

		err = provider.Start()
		if err != nil {
			t.Errorf("unexpected error starting provider: %v", err)
		}

		err = provider.RegisterHotkey("test", "test", func() error { return nil })
		if err != nil {
			t.Errorf("unexpected error registering hotkey: %v", err)
		}

		provider.Stop()
	})

	t.Run("ErrorProviderCompliance", func(t *testing.T) {
		errorProvider := mocks.NewMockHotkeyProviderWithErrors()
		factory.RegisterProvider("error", errorProvider)

		provider, err := factory.CreateProvider("error")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify interface compliance
		var _ interfaces.KeyboardEventProvider = provider

		// Test error scenarios
		errorProvider.SimulateUnsupportedEnvironment()
		if provider.IsSupported() {
			t.Error("expected provider to be unsupported")
		}

		err = provider.Start()
		if err == nil {
			t.Error("expected error starting unsupported provider")
		}
	})
}

func TestHotkeyFactory_LifecycleManagement(t *testing.T) {
	factory := NewMockFactory()

	t.Run("ProviderLifecycle", func(t *testing.T) {
		mockProvider := mocks.NewMockHotkeyProvider()
		factory.RegisterProvider("lifecycle", mockProvider)

		provider, err := factory.CreateProvider("lifecycle")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Test lifecycle methods
		err = provider.Start()
		if err != nil {
			t.Errorf("unexpected error starting provider: %v", err)
		}

		if !mockProvider.IsStarted() {
			t.Error("expected provider to be started")
		}

		provider.Stop()

		if !mockProvider.WasStopCalled() {
			t.Error("expected stop to be called")
		}
	})

	t.Run("MultipleProviderLifecycles", func(t *testing.T) {
		providers := make([]interfaces.KeyboardEventProvider, 3)
		for i := range providers {
			mock := mocks.NewMockHotkeyProvider()
			providers[i] = mock
			factory.RegisterProvider(string(rune('a'+i)), mock)
		}

		// Start all providers
		for i := range providers {
			provider, err := factory.CreateProvider(string(rune('a' + i)))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			err = provider.Start()
			if err != nil {
				t.Errorf("unexpected error starting provider %d: %v", i, err)
			}
		}

		// Verify all are started
		for i, provider := range providers {
			if mock, ok := provider.(*mocks.MockHotkeyProvider); ok {
				if !mock.IsStarted() {
					t.Errorf("expected provider %d to be started", i)
				}
			}
		}

		// Stop all providers
		for i := range providers {
			provider, _ := factory.CreateProvider(string(rune('a' + i)))
			provider.Stop()
		}

		// Verify all are stopped
		for i, provider := range providers {
			if mock, ok := provider.(*mocks.MockHotkeyProvider); ok {
				if !mock.WasStopCalled() {
					t.Errorf("expected provider %d to be stopped", i)
				}
			}
		}
	})
}

func TestHotkeyFactory_ConcurrentAccess(t *testing.T) {
	factory := NewMockFactory()

	t.Run("ConcurrentRegistration", func(t *testing.T) {
		done := make(chan bool)

		// Register providers concurrently
		for i := 0; i < 10; i++ {
			go func(index int) {
				defer func() { done <- true }()

				mock := mocks.NewMockHotkeyProvider()
				factory.RegisterProvider(string(rune('a'+index)), mock)
			}(i)
		}

		// Wait for all registrations
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify all providers are registered
		for i := 0; i < 10; i++ {
			provider, err := factory.CreateProvider(string(rune('a' + i)))
			if err != nil {
				t.Errorf("provider %d not registered: %v", i, err)
			}
			if provider == nil {
				t.Errorf("provider %d is nil", i)
			}
		}
	})

	t.Run("ConcurrentCreation", func(t *testing.T) {
		mock := mocks.NewMockHotkeyProvider()
		factory.RegisterProvider("concurrent", mock)

		done := make(chan bool)
		errorsChan := make(chan error, 10)

		// Create providers concurrently
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()

				provider, err := factory.CreateProvider("concurrent")
				if err != nil {
					errorsChan <- err
					return
				}
				if provider == nil {
					errorsChan <- errors.New("provider is nil")
					return
				}
			}()
		}

		// Wait for all creations
		for i := 0; i < 10; i++ {
			<-done
		}

		// Check for errors
		close(errorsChan)
		for err := range errorsChan {
			t.Errorf("concurrent creation error: %v", err)
		}
	})
}

func TestHotkeyFactory_EdgeCases(t *testing.T) {
	factory := NewMockFactory()

	t.Run("EmptyProviderName", func(t *testing.T) {
		mock := mocks.NewMockHotkeyProvider()
		factory.RegisterProvider("", mock)

		provider, err := factory.CreateProvider("")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if provider != mock {
			t.Error("expected registered provider")
		}
	})

	t.Run("SpecialCharacterNames", func(t *testing.T) {
		names := []string{"test-provider", "test_provider", "test.provider", "test@provider"}

		for _, name := range names {
			mock := mocks.NewMockHotkeyProvider()
			factory.RegisterProvider(name, mock)

			provider, err := factory.CreateProvider(name)
			if err != nil {
				t.Errorf("unexpected error for name %q: %v", name, err)
			}
			if provider != mock {
				t.Errorf("expected registered provider for name %q", name)
			}
		}
	})

	t.Run("UnicodeProviderNames", func(t *testing.T) {
		names := []string{"тест", "测试", "テスト", "🎹"}

		for _, name := range names {
			mock := mocks.NewMockHotkeyProvider()
			factory.RegisterProvider(name, mock)

			provider, err := factory.CreateProvider(name)
			if err != nil {
				t.Errorf("unexpected error for unicode name %q: %v", name, err)
			}
			if provider != mock {
				t.Errorf("expected registered provider for unicode name %q", name)
			}
		}
	})

	t.Run("LongProviderNames", func(t *testing.T) {
		longName := ""
		for i := 0; i < 1000; i++ {
			longName += "a"
		}

		mock := mocks.NewMockHotkeyProvider()
		factory.RegisterProvider(longName, mock)

		provider, err := factory.CreateProvider(longName)
		if err != nil {
			t.Errorf("unexpected error for long name: %v", err)
		}
		if provider != mock {
			t.Error("expected registered provider for long name")
		}
	})
}
