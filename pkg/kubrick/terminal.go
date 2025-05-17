package kubrick

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/theapemachine/a2a-go/pkg/kubrick/layouts"
	"github.com/theapemachine/a2a-go/pkg/kubrick/types"
	"github.com/theapemachine/a2a-go/pkg/logging"
)

// Define control characters
const (
	ctrlQ = 17 // ASCII value for Ctrl-Q
)

// Terminal handles the terminal I/O and raw mode
type Terminal struct {
	*types.Contextualizer

	wg         *sync.WaitGroup
	transport  Transport
	sigChan    chan os.Signal
	shouldQuit bool
	root       layouts.Layout
	err        error
}

type TerminalOption func(*Terminal)

// NewTerminal creates a new terminal handler
func NewTerminal(opts ...TerminalOption) *Terminal {
	logging.Log("Terminal.NewTerminal: Called")
	terminal := &Terminal{
		Contextualizer: types.NewContextualizer(),
		transport:      NewLocalTransport(),
		sigChan:        make(chan os.Signal, 1),
		wg:             &sync.WaitGroup{},
	}

	for i, opt := range opts {
		logging.Log("Terminal.NewTerminal: Applying option %d", i)
		opt(terminal)
	}

	logging.Log("Terminal.NewTerminal: Transport is %T", terminal.transport)

	terminal.wg.Add(1)

	logging.Log("Terminal.NewTerminal: Starting render (main loop)")
	if err := terminal.render(); err != nil {
		logging.Log("Terminal.NewTerminal: Error from terminal.render: %v", err)
		return nil
	}

	logging.Log("Terminal.NewTerminal: Initialization complete")
	return terminal
}

func (terminal *Terminal) render() (err error) {
	logging.Log("Terminal.render: Called (main loop started)")

	if lt, ok := terminal.transport.(*LocalTransport); ok {
		logging.Log("Terminal.render: Using LocalTransport, setting up signal handling and raw mode.")
		signal.Notify(
			terminal.sigChan,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGWINCH,
		)
		defer signal.Stop(terminal.sigChan)

		// Enable raw mode
		logging.Log("Terminal.render: Setting raw mode")
		if err = lt.SetRawMode(); err != nil {
			terminal.err = fmt.Errorf("failed to set raw mode: %w", err)
			logging.Log("Terminal.render: Error setting raw mode: %v", terminal.err)
			return terminal.err
		}
		defer func() {
			logging.Log("Terminal.render: Restoring mode (defer)")
			lt.RestoreMode()
		}()
	} else {
		logging.Log("Terminal.render: Transport is not LocalTransport (%T), not setting raw mode or signals.", terminal.transport)
	}

	// Initial setup - these write to terminal.transport
	logging.Log("Terminal.render: Writing hideCursor to transport")
	terminal.transport.Write([]byte(hideCursor))
	logging.Log("Terminal.render: Writing clearScreenHome to transport")
	terminal.transport.Write([]byte(clearScreenHome))

	// Create input channel
	inputCh := make(chan rune, 10)

	// Input reader goroutine
	logging.Log("Terminal.render: Starting input reader goroutine")
	go func() {
		defer logging.Log("Terminal.render: Input reader goroutine finished")
		defer close(inputCh)

		buf := make([]byte, 1)
		for {
			select {
			case <-terminal.Done():
				logging.Log("Terminal.render: Input reader context done, closing terminal")
				terminal.Close()
				return
			default:
				n, readErr := terminal.transport.Read(buf)
				if readErr != nil || n == 0 {
					if !terminal.shouldQuit {
						logging.Log("Terminal.render: Error reading input from transport: %v (n=%d). Quitting input loop.", readErr, n)
						terminal.shouldQuit = true
					}
					return
				}
				r := rune(buf[0])
				if r == ctrlQ {
					logging.Log("Terminal.render: Ctrl-Q received, quitting input loop.")
					terminal.shouldQuit = true
					return
				}
				select {
				case inputCh <- r:
				default:
					logging.Log("Terminal.render: Input channel full, dropping rune: %c (%d)", r, r)
				}
			}
		}
	}()

	// Main loop
	displayTicker := time.NewTicker(time.Millisecond * 16)
	defer displayTicker.Stop()

	logging.Log("Terminal.render: Starting display/event loop goroutine")
	go func() {
		defer logging.Log("Terminal.render: Display/event loop goroutine finished")
		for !terminal.shouldQuit {
			select {
			case <-terminal.Done():
				logging.Log("Terminal.render: Display/event loop context done. Setting shouldQuit.")
				terminal.shouldQuit = true

			case r, ok := <-inputCh:
				if !ok {
					logging.Log("Terminal.render: Input channel closed. Setting shouldQuit.")
					terminal.shouldQuit = true
					continue
				}
				logging.Log("Terminal.render: Received rune from inputCh: %c (%d)", r, r)
				if inputHandler, ok := terminal.root.(interface{ HandleInput(rune) }); ok {
					logging.Log("Terminal.render: Passing input to root layout")
					inputHandler.HandleInput(r)
				}

			case sig := <-terminal.sigChan:
				logging.Log("Terminal.render: Received signal: %v", sig)
				switch sig {
				case syscall.SIGINT, syscall.SIGTERM:
					logging.Log("Terminal.render: SIGINT/SIGTERM received. Setting shouldQuit.")
					terminal.shouldQuit = true
				case syscall.SIGWINCH:
					logging.Log("Terminal.render: SIGWINCH received. Resizing.")
					if terminal.root != nil && terminal.transport != nil {
						width, height, errGetSize := terminal.transport.GetSize()
						if errGetSize != nil {
							logging.Log("Terminal.render: Error getting transport size for SIGWINCH: %v", errGetSize)
							continue
						}
						logging.Log("Terminal.render: Resizing root to width=%d, height=%d", width, height)
						terminal.root.SetRect(layouts.Rect{
							Pos:  layouts.Position{Row: 0, Col: 0},
							Size: layouts.Size{Width: width, Height: height},
						})
					} else {
						logging.Log("Terminal.render: SIGWINCH: root or transport is nil")
					}
				}

			case <-displayTicker.C:
				if terminal.root == nil {
					continue
				}
				if terminal.transport == nil {
					continue
				}
				_, errCopy := io.Copy(terminal.transport, terminal.root)
				if errCopy != nil {
					logging.Log("Terminal.render: Error copying root to transport in display tick: %v", errCopy)
				}
			}
			if terminal.shouldQuit {
				logging.Log("Terminal.render: shouldQuit is true, breaking display/event loop.")
				break
			}
		}
		terminal.wg.Done()
		logging.Log("Terminal.render: Display/event loop goroutine ending naturally due to shouldQuit.")
	}()

	logging.Log("Terminal.render: Waiting for terminal.wg (display/event loop to finish due to quit signal)")
	terminal.wg.Wait()

	logging.Log("Terminal.render: Main loop finished. Performing cleanup.")
	return terminal.err
}

func (terminal *Terminal) Read(p []byte) (n int, err error) {
	return terminal.transport.Read(p)
}

func (terminal *Terminal) Write(p []byte) (n int, err error) {
	return terminal.transport.Write(p)
}

func (terminal *Terminal) Close() error {
	logging.Log("Terminal.Close: Called. Setting shouldQuit and cancelling context.")
	terminal.shouldQuit = true
	terminal.Cancel()

	if terminal.transport != nil {
		logging.Log("Terminal.Close: Writing showCursor and clearScreenHome to transport")
		terminal.transport.Write([]byte(showCursor))
		terminal.transport.Write([]byte(clearScreenHome))
		logging.Log("Terminal.Close: Closing transport")
		return terminal.transport.Close()
	}
	logging.Log("Terminal.Close: Transport was nil")
	return nil
}

func WithRoot(root layouts.Layout) TerminalOption {
	return func(t *Terminal) {
		logging.Log("Terminal.WithRoot: Setting root to %T", root)
		t.root = root
	}
}

func WithTransport(transport Transport) TerminalOption {
	return func(t *Terminal) {
		logging.Log("Terminal.WithTransport: Setting transport to %T", transport)
		t.transport = transport
	}
}
