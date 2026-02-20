package nexmosphere

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Service manages Nexmosphere controllers and dispatches events to handlers
type Service struct {
	controllers  map[string]*Controller
	handlers     []EventHandler
	logger       *zap.SugaredLogger
	scanTicker   *time.Ticker
	scanInterval time.Duration
	stopChan     chan struct{}
	mu           sync.RWMutex
	running      bool
}

// Option configures a Service
type Option func(*Service)

// WithLogger sets a custom logger
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithScanInterval sets the controller scan interval (default: 2s)
func WithScanInterval(interval time.Duration) Option {
	return func(s *Service) {
		s.scanInterval = interval
	}
}

// NewService creates a new Nexmosphere service
func NewService(opts ...Option) *Service {
	// Default logger
	logger, _ := zap.NewDevelopment()

	s := &Service{
		controllers:  make(map[string]*Controller),
		handlers:     make([]EventHandler, 0),
		logger:       logger.Sugar(),
		scanInterval: 2 * time.Second,
		stopChan:     make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start begins scanning for controllers and dispatching events
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("service already running")
	}

	s.running = true
	s.logger.Info("Nexmosphere service starting")

	// Initial scan
	go s.scanForControllers()

	// Start periodic scanning
	s.scanTicker = time.NewTicker(s.scanInterval)
	go func() {
		for {
			select {
			case <-s.scanTicker.C:
				s.scanForControllers()
			case <-s.stopChan:
				return
			}
		}
	}()

	return nil
}

// Stop stops the service and closes all controllers
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Nexmosphere service stopping")

	close(s.stopChan)

	if s.scanTicker != nil {
		s.scanTicker.Stop()
	}

	// Close all controllers
	for _, c := range s.controllers {
		c.close()
	}

	s.running = false
	return nil
}

// AddHandler registers an event handler
func (s *Service) AddHandler(h EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, h)
}

// dispatch sends an event to all registered handlers
func (s *Service) dispatch(event Event) {
	s.mu.RLock()
	handlers := s.handlers
	s.mu.RUnlock()

	event.Timestamp = time.Now()

	s.logger.Debugf("Event: type=%s action=%s address=%d controller=%s",
		event.Type, event.Action, event.Address, event.Controller)

	for _, h := range handlers {
		go h.HandleEvent(event) // Non-blocking dispatch
	}
}

// GetControllers returns information about connected controllers
func (s *Service) GetControllers() []ControllerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := make([]ControllerInfo, 0, len(s.controllers))
	for _, c := range s.controllers {
		info = append(info, c.getInfo())
	}
	return info
}

// SendCommand sends a command to a specific controller
func (s *Service) SendCommand(controllerName string, cmd string) error {
	s.mu.RLock()
	c, ok := s.controllers[controllerName]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("controller %s not found", controllerName)
	}

	return c.write(cmd)
}

// SetDeviceHoldInterval configures the hold tick interval for a specific device
// When set, "hold" events will be emitted periodically while a button is held
// Set to 0 to disable hold events
func (s *Service) SetDeviceHoldInterval(controllerName string, address int, interval time.Duration) error {
	s.mu.RLock()
	c, ok := s.controllers[controllerName]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("controller %s not found", controllerName)
	}

	if address < 1 || address >= 1000 {
		return fmt.Errorf("invalid device address %d (must be 1-999)", address)
	}

	d := c.getDevice(address)
	d.HoldTickInterval = interval

	s.logger.Debugf("Set hold interval for %s device %d to %s", controllerName, address, interval)
	return nil
}

// ControllerInfo provides information about a connected controller
type ControllerInfo struct {
	Name        string
	IsUSB       bool
	VID         string
	PID         string
	DeviceCount int
}
