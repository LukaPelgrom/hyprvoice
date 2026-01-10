package injection

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Injector interface {
	Inject(ctx context.Context, text string) error
}

type Config struct {
	Backends         []string      // Ordered list: "ydotool", "wtype", "clipboard"
	YdotoolTimeout   time.Duration // Timeout for ydotool commands
	WtypeTimeout     time.Duration // Timeout for wtype commands
	ClipboardTimeout time.Duration // Timeout for clipboard operations
}

type injector struct {
	config   Config
	backends []Backend
}

func NewInjector(config Config) Injector {
	// Build backend chain from config
	backends := make([]Backend, 0, len(config.Backends))
	for _, name := range config.Backends {
		switch name {
		case "ydotool":
			backends = append(backends, NewYdotoolBackend())
		case "wtype":
			backends = append(backends, NewWtypeBackend())
		case "clipboard":
			backends = append(backends, NewClipboardBackend())
		default:
			log.Printf("Injection: unknown backend %q, skipping", name)
		}
	}

	// Default to clipboard if no valid backends
	if len(backends) == 0 {
		log.Printf("Injection: no valid backends configured, defaulting to clipboard")
		backends = append(backends, NewClipboardBackend())
	}

	return &injector{
		config:   config,
		backends: backends,
	}
}

func (i *injector) Inject(ctx context.Context, text string) error {
	if text == "" {
		return fmt.Errorf("cannot inject empty text")
	}

	// Always copy to clipboard first (best effort, don't fail if clipboard fails)
	clipboardBackend := NewClipboardBackend()
	clipboardErr := clipboardBackend.Inject(ctx, text, i.config.ClipboardTimeout)
	if clipboardErr != nil {
		log.Printf("Injection: clipboard copy failed (will continue with other backends): %v", clipboardErr)
	} else {
		log.Printf("Injection: text copied to clipboard")
	}

	// Try each backend in order for typing
	var lastErr error
	for _, backend := range i.backends {
		// Skip clipboard backend since we already did it
		if backend.Name() == "clipboard" {
			continue
		}
		timeout := i.getTimeout(backend.Name())
		err := backend.Inject(ctx, text, timeout)
		if err == nil {
			log.Printf("Injection: success via %s", backend.Name())
			return nil
		}
		log.Printf("Injection: %s failed: %v, trying next backend", backend.Name(), err)
		lastErr = err
	}

	// If clipboard succeeded but all typing backends failed, that's still partial success
	if clipboardErr == nil && lastErr != nil {
		log.Printf("Injection: typing failed but text is in clipboard")
		return nil
	}

	if lastErr != nil {
		return fmt.Errorf("all injection backends failed, last error: %w", lastErr)
	}

	return nil
}

func (i *injector) getTimeout(backendName string) time.Duration {
	switch backendName {
	case "ydotool":
		return i.config.YdotoolTimeout
	case "wtype":
		return i.config.WtypeTimeout
	case "clipboard":
		return i.config.ClipboardTimeout
	default:
		return 5 * time.Second
	}
}
