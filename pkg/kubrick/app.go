package kubrick

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/theapemachine/a2a-go/pkg/kubrick/layouts"
	"github.com/theapemachine/a2a-go/pkg/kubrick/types"
	"github.com/theapemachine/a2a-go/pkg/logging"
)

/*
App is a container for a Kubrick application that manages screens and their rendering.
*/
type App struct {
	*types.Contextualizer

	wg           *sync.WaitGroup
	screens      []layouts.Layout
	activeScreen int
	status       types.State
	err          error

	// Rendering infrastructure
	framebuffer *Framebuffer
	transport   Transport
	artifact    *types.Buffer
	terminal    *Terminal

	// Synchronization
	mu sync.RWMutex

	// View state
	width  int
	height int
}

type AppOption func(*App)

/*
NewApp creates a new Kubrick application with the specified options.
*/
func NewApp(options ...AppOption) (*App, error) {
	logging.Log("App.NewApp: Called")
	app := &App{
		Contextualizer: types.NewContextualizer(),
		wg:             &sync.WaitGroup{},
		screens:        make([]layouts.Layout, 0),
		activeScreen:   0,
		status:         types.StateInitialized,
		framebuffer:    NewFramebuffer(),
		artifact:       types.NewBuffer(1024, 1024),
	}

	logging.Log("App.NewApp: Initializing StreamTransport")
	app.transport = NewStreamTransport(app.artifact, 1024, 1024)

	var err error
	logging.Log("App.NewApp: Getting transport size")
	if app.width, app.height, err = app.transport.GetSize(); err != nil {
		logging.Log("App.NewApp: Error getting transport size: %v", err)
		return nil, err
	}
	logging.Log("App.NewApp: Transport size: width=%d, height=%d", app.width, app.height)

	app.Contextualizer.WithContext(context.Background())
	logging.Log("App.NewApp: Applying options (%d)", len(options))
	for i, option := range options {
		logging.Log("App.NewApp: Applying option %d", i)
		option(app)
	}

	if len(app.screens) > 0 {
		logging.Log("App.NewApp: Creating Terminal")
		app.terminal = NewTerminal(
			WithRoot(app.screens[app.activeScreen]),
			WithTransport(app.transport),
		)
		if app.terminal == nil {
			logging.Log("App.NewApp: NewTerminal returned nil!")
			// Potentially return an error here
		} else {
			app.terminal.WithContext(app.Context())
		}
	} else {
		logging.Log("App.NewApp: No screens, not creating Terminal")
	}

	app.wg.Add(1) // This wg seems unused for waiting, consider removing or using it properly.
	logging.Log("App.NewApp: Starting render loop")
	if err := app.startRenderLoop(); err != nil {
		logging.Log("App.NewApp: Error starting render loop: %v", err)
		return nil, err
	}

	app.status = types.StateRunning
	logging.Log("App.NewApp: Initialization complete, status=Running")
	return app, nil
}

func (app *App) startRenderLoop() error {
	logging.Log("App.startRenderLoop: Goroutine starting")
	go func() {
		defer logging.Log("App.startRenderLoop: Goroutine finished")
		for {
			select {
			case <-app.Done():
				logging.Log("App.startRenderLoop: Context done, closing app")
				app.Close()
				return
			case <-time.Tick(time.Millisecond * 16):
				// logging.Log("App.startRenderLoop: Tick") // This might be too verbose
				if len(app.screens) > 0 {
					// logging.Log("App.startRenderLoop: Copying screen to transport (app.artifact)")
					if _, app.err = io.Copy(app.transport, app.screens[app.activeScreen]); app.err != nil {
						logging.Log("App.startRenderLoop: Error copying to transport: %v", app.err)
						app.status = types.StateErrored
						return
					}
				} else {
					// logging.Log("App.startRenderLoop: Tick - No screens to render")
				}
			}
		}
	}()

	return nil
}

func (app *App) Error() string {
	return app.err.Error()
}

func (app *App) UpdateComponent(name string, update interface{}) {
	if len(app.screens) == 0 {
		return
	}

	if grid, ok := app.screens[app.activeScreen].(*layouts.GridLayout); ok {
		for _, comp := range grid.Components {
			// Add logic to identify the component by name and send the update
			_ = comp // Temporary use of comp to satisfy linter
		}
	}
}

func (app *App) Read(p []byte) (n int, err error) {
	// logging.Log("App.Read: Called with len(p)=%d", len(p)) // Can be verbose
	if app.artifact == nil {
		logging.Log("App.Read: Artifact is nil, returning EOF")
		return 0, io.EOF
	}
	n, err = app.artifact.Read(p)
	// logging.Log("App.Read: Read %d bytes from artifact, err: %v", n, err) // Can be verbose
	return n, err
}

// Write implements io.Writer
func (app *App) Write(p []byte) (n int, err error) {
	logging.Log("App.Write: Called with %d bytes: %s", len(p), string(p))
	if len(app.screens) == 0 {
		logging.Log("App.Write: No screens, returning EOF")
		return 0, io.EOF
	}

	logging.Log("App.Write: Writing to active screen (%T)", app.screens[app.activeScreen])
	if n, app.err = app.screens[app.activeScreen].Write(p); app.err != nil {
		logging.Log("App.Write: Error writing to screen: %v", app.err)
		app.status = types.StateErrored
		return n, app.err
	}
	logging.Log("App.Write: Wrote %d bytes to screen", n)
	return n, nil
}

// Close implements io.Closer
func (app *App) Close() error {
	logging.Log("App.Close: Called")
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.status == types.StateCanceled || app.status == types.StateClosed {
		logging.Log("App.Close: Already canceled or closed (status: %v)", app.status)
		return app.err // Return existing error if any
	}

	logging.Log("App.Close: Cancelling context")
	app.Cancel()
	app.status = types.StateCanceled

	logging.Log("App.Close: Closing screens (%d)", len(app.screens))
	for i, screen := range app.screens {
		logging.Log("App.Close: Closing screen %d (%T)", i, screen)
		if err := screen.Close(); err != nil {
			logging.Log("App.Close: Error closing screen %d: %v", i, err)
			app.err = err // Keep first error
			// Don't return early, try to close other resources
		}
	}

	logging.Log("App.Close: Cleaning up resources (framebuffer, transport)")
	if app.framebuffer != nil {
		app.framebuffer.Clear() // This is safe to call multiple times
	}
	if app.transport != nil {
		errTransportClose := app.transport.Close()
		if errTransportClose != nil {
			logging.Log("App.Close: Error closing transport: %v", errTransportClose)
			if app.err == nil { // Only set if no prior error from screens
				app.err = errTransportClose
			}
		}
	}
	app.status = types.StateClosed
	logging.Log("App.Close: Finished, status=Closed, error: %v", app.err)
	return app.err
}

func WithScreen(screen layouts.Layout) AppOption {
	return func(app *App) {
		logging.Log("App.WithScreen: Adding screen (%T)", screen)
		app.screens = append(app.screens, screen)

		if app.Context() == nil {
			logging.Log("App.WithScreen: App context is nil before screen.WithContext!")
			// This indicates an issue, app.Contextualizer.WithContext(context.Background()) should have run
		}
		screen.WithContext(app.Context())

		logging.Log("App.WithScreen: Setting rect for screen: width=%d, height=%d", app.width, app.height)
		screen.SetRect(layouts.Rect{
			Pos: layouts.Position{Row: 0, Col: 0},
			Size: layouts.Size{
				Width:  app.width,
				Height: app.height,
			},
		})
	}
}
