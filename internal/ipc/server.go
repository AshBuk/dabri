// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AshBuk/dabri/v2/internal/logger"
)

const (
	defaultIdleTimeout = 30 * time.Second
)

// Server provides a simple Unix domain socket server for local IPC control.
type Server struct {
	path     string
	listener net.Listener
	handlers map[string]Handler
	log      logger.Logger

	mu       sync.RWMutex
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup // Tracks acceptLoop and connection handlers
}

// NewServer constructs a new IPC server bound to the specified socket path.
func NewServer(path string, log logger.Logger) *Server {
	return &Server{
		path:     path,
		handlers: make(map[string]Handler),
		log:      log,
		stopCh:   make(chan struct{}),
	}
}

// Register associates a handler with a command name.
func (s *Server) Register(command string, handler Handler) {
	cmd := strings.TrimSpace(strings.ToLower(command))
	if cmd == "" || handler == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[cmd] = handler
}

// Start launches the server in a background goroutine.
func (s *Server) Start() error {
	if s.path == "" {
		return fmt.Errorf("ipc server requires a socket path")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("failed to create ipc directory: %w", err)
	}

	// Remove stale socket if present.
	if err := os.RemoveAll(s.path); err != nil {
		return fmt.Errorf("failed to remove stale socket: %w", err)
	}

	ln, err := net.Listen("unix", s.path)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket: %w", err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		_ = ln.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	s.listener = ln

	s.wg.Add(1)
	go s.acceptLoop()
	s.log.Info("IPC server listening on %s", s.path)
	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// Check if we are shutting down.
			select {
			case <-s.stopCh:
				return
			default:
			}
			if isTransientAcceptError(err) {
				s.log.Warning("Temporary IPC accept error: %v", err)
				time.Sleep(50 * time.Millisecond)
				continue
			}

			s.log.Error("IPC accept error: %v", err)
			return
		}
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		_ = conn.Close()
	}()

	if err := conn.SetDeadline(time.Now().Add(defaultIdleTimeout)); err != nil {
		s.log.Debug("IPC set deadline failed: %v", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		if !errors.Is(err, net.ErrClosed) {
			s.log.Debug("IPC read error: %v", err)
		}
		return
	}
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		s.writeResponse(conn, NewErrorResponse("invalid request payload"))
		return
	}

	handler := s.getHandler(req.Command)
	if handler == nil {
		s.writeResponse(conn, NewErrorResponse("unknown command"))
		return
	}

	resp, err := handler(req)
	if err != nil {
		s.writeResponse(conn, NewErrorResponse(err.Error()))
		return
	}
	s.writeResponse(conn, resp)
}

func isTransientAcceptError(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, syscall.EINTR)
}

func (s *Server) getHandler(command string) Handler {
	cmd := strings.TrimSpace(strings.ToLower(command))
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handlers[cmd]
}

func (s *Server) writeResponse(conn net.Conn, resp Response) {
	if err := conn.SetWriteDeadline(time.Now().Add(defaultIdleTimeout)); err != nil {
		s.log.Debug("IPC set write deadline failed: %v", err)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		s.log.Error("IPC marshal response failed: %v", err)
		return
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		s.log.Debug("IPC write error: %v", err)
	}
}

// Stop shuts down the server and removes the socket file.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		if s.listener != nil {
			_ = s.listener.Close()
		}

		// Wait for acceptLoop and connection handlers with timeout
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			s.log.Info("IPC server stopped cleanly")
		case <-time.After(2 * time.Second):
			s.log.Warning("IPC server stop timeout - handlers may still be running")
		}

		if s.path != "" {
			if err := os.RemoveAll(s.path); err != nil && !os.IsNotExist(err) {
				s.log.Debug("Failed to remove IPC socket: %v", err)
			}
		}
	})
}
