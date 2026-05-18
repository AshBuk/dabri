// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/gorilla/websocket"
)

// AudioController exposes recording actions to WebSocket handlers.
type AudioController interface {
	HandleStartRecording() error
	HandleStopRecordingSync(ctx context.Context) (string, error)
}

// WebSocket server configuration constants
const (
	// Buffer sizes for WebSocket connections
	readBufferSize  = 1024 // 1KB
	writeBufferSize = 1024 // 1KB

	// Message size limits
	maxMessageSize = 1024 * 1024 // 1MB

	// Timeout configurations
	readTimeout             = 60 * time.Second // Client read timeout
	writeTimeout            = 10 * time.Second // Client write timeout
	pingInterval            = 20 * time.Second // Health check interval
	serverReadTimeout       = 15 * time.Second // HTTP server read timeout
	serverWriteTimeout      = 15 * time.Second // HTTP server write timeout
	serverIdleTimeout       = 60 * time.Second // HTTP server idle timeout
	shutdownTimeout         = 5 * time.Second  // Graceful shutdown timeout
	transcriptionCtxTimeout = 30 * time.Second // Context timeout for transcription
)

// Enables real-time speech-to-text API for external clients
type WebSocketServer struct {
	config      *config.Config
	clients     map[*websocket.Conn]bool
	clientsLock sync.Mutex
	upgrader    websocket.Upgrader
	audio       AudioController
	audioMu     sync.RWMutex
	server      *http.Server
	started     atomic.Bool
	retryCount  map[*websocket.Conn]int // Track retry attempts
	logger      logger.Logger
	wg          sync.WaitGroup
}

// Protocol structure for bidirectional client communication
type Message struct {
	Type       string      `json:"type"`
	Payload    interface{} `json:"payload,omitempty"`
	APIVersion string      `json:"api_version,omitempty"`
	RequestID  string      `json:"request_id,omitempty"`
	Timestamp  int64       `json:"timestamp,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// checkOriginFunc creates a CORS origin validation function
func checkOriginFunc(cfg *config.Config) func(*http.Request) bool {
	return func(r *http.Request) bool {
		// Allow all origins if configured with "*"
		if cfg.WebServer.CORSOrigins == "*" {
			return true
		}

		// Get the origin from the request
		origin := r.Header.Get("Origin")
		if origin == "" {
			// No origin header - might be same-origin request
			return true
		}

		// Check if origin matches configured CORS origins
		// For simplicity, exact match. Could be extended to support wildcards
		return origin == cfg.WebServer.CORSOrigins
	}
}

// Initialize server with security and resource constraints.
func NewWebSocketServer(config *config.Config, logger logger.Logger) *WebSocketServer {
	return &WebSocketServer{
		config:  config,
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  readBufferSize,
			WriteBufferSize: writeBufferSize,
			CheckOrigin:     checkOriginFunc(config),
		},
		retryCount: make(map[*websocket.Conn]int),
		logger:     logger,
	}
}

// SetAudioController wires recording actions.
func (s *WebSocketServer) SetAudioController(audio AudioController) {
	s.audioMu.Lock()
	s.audio = audio
	s.audioMu.Unlock()
}

// audioController returns the current AudioController (thread-safe).
func (s *WebSocketServer) audioController() AudioController {
	s.audioMu.RLock()
	defer s.audioMu.RUnlock()
	return s.audio
}

// Begin accepting client connections with health monitoring
func (s *WebSocketServer) Start() error {
	if !s.config.WebServer.Enabled {
		return nil
	}
	// Handler for WebSocket connection setup
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)

	// API versioning - support multiple API versions
	apiVersion := s.config.WebServer.APIVersion
	if apiVersion != "" {
		mux.HandleFunc(fmt.Sprintf("/api/%s/ws", apiVersion), s.handleWebSocket)
	}

	// Add a health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			s.logger.Debug("health write error: %v", err)
		}
	})
	// Create HTTP server with timeouts
	addr := fmt.Sprintf("%s:%d", s.config.WebServer.Host, s.config.WebServer.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}
	// Start HTTP server in background goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("Starting WebSocket server on %s", addr)
		s.started.Store(true)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("WebSocket server error: %v", err)
		}
	}()

	return nil
}

// Ensure clean client disconnection before termination
func (s *WebSocketServer) Stop() {
	if s.server != nil && s.started.Load() {
		s.logger.Info("Stopping WebSocket server...")
		// Create a context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		// Close all client connections
		s.clientsLock.Lock()
		for client := range s.clients {
			_ = client.Close()
		}
		s.clients = make(map[*websocket.Conn]bool)
		s.clientsLock.Unlock()
		// Shutdown the server
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Error("Error shutting down WebSocket server: %v", err)
		} else {
			s.logger.Info("WebSocket server stopped")
		}
		// Wait for server goroutine to finish
		s.wg.Wait()
		s.started.Store(false)
	}
}

// Authenticate and establish secure client session
func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check authentication first
	if !s.authenticate(r) {
		s.logger.Warning("Unauthorized WebSocket connection attempt from %s", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Check if we're at max clients limit
	s.clientsLock.Lock()
	clientCount := len(s.clients)
	s.clientsLock.Unlock()

	if s.config.WebServer.MaxClients > 0 && clientCount >= s.config.WebServer.MaxClients {
		s.logger.Warning("Max clients limit reached, rejecting connection from %s", r.RemoteAddr)
		http.Error(w, "Too many connections", http.StatusServiceUnavailable)
		return
	}
	// Establish connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Error upgrading to WebSocket: %v", err)
		return
	}
	// Configure connection
	conn.SetReadLimit(maxMessageSize)
	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		s.logger.Debug("SetReadDeadline error: %v", err)
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(readTimeout))
	})

	// Register new client
	s.clientsLock.Lock()
	s.clients[conn] = true
	s.clientsLock.Unlock()

	defer func() {
		if err := conn.Close(); err != nil {
			s.logger.Debug("conn close error: %v", err)
		}
		s.clientsLock.Lock()
		delete(s.clients, conn)
		delete(s.retryCount, conn)
		s.clientsLock.Unlock()
	}()

	// Send welcome message with version info
	s.sendMessage(conn, "connected", map[string]string{
		"server":      "Dabri",
		"api_version": s.config.WebServer.APIVersion,
	})
	// Start ping/pong goroutine.
	// Fire-and-forget is safe here: Stop() closes all connections, causing WriteControl
	// in pingClient to return an error and exit. Tracking via WaitGroup creates race
	// conditions (wg.Add after wg.Wait) without adding value since conn closure
	// guarantees goroutine termination.
	go s.pingClient(conn)
	// Process messages from client
	s.processMessages(conn)
}

// Maintain connection health to prevent proxy timeouts
func (s *WebSocketServer) pingClient(conn *websocket.Conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeTimeout)); err != nil {
			s.logger.Debug("Ping error: %v", err)
			return
		}
	}
}

// Deliver structured response with timeout protection
func (s *WebSocketServer) sendMessage(conn *websocket.Conn, messageType string, payload interface{}, requestID ...string) {
	msg := Message{
		Type:       messageType,
		Payload:    payload,
		APIVersion: s.config.WebServer.APIVersion,
		Timestamp:  time.Now().Unix(),
	}
	// Set request ID if provided
	if len(requestID) > 0 && requestID[0] != "" {
		msg.RequestID = requestID[0]
	}
	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error("Error marshaling message: %v", err)
		return
	}
	// Log if enabled
	if s.config.WebServer.LogRequests {
		s.logger.Debug("Sending WebSocket message: %s", string(data))
	}
	// Send message
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		s.logger.Error("SetWriteDeadline error: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		s.logger.Error("Error sending message: %v", err)
	}
}

// Notify all active clients of server-wide events
func (s *WebSocketServer) BroadcastMessage(messageType string, payload interface{}) {
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()

	for conn := range s.clients {
		s.sendMessage(conn, messageType, payload)
	}
}
