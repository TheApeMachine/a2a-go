package spinner

import (
	"container/ring"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/theapemachine/a2a-go/pkg/kubrick/types"
	"github.com/theapemachine/a2a-go/pkg/logging"
)

var (
	defaultFrames = []rune{
		'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏',
	}

	successFrame = '✓'
	failureFrame = '✗'
)

type Spinner struct {
	wg       *sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	frames   *ring.Ring
	label    string
	artifact *types.Buffer
	state    types.State
	err      error
	// Store calculated max length for the display string (frame + space + label)
	// This helps in creating an appropriately sized buffer.
	currentDisplayWidth int
}

type SpinnerOption func(*Spinner)

func NewSpinner(options ...SpinnerOption) *Spinner {
	logging.Log("Spinner.NewSpinner: Called")
	frames := ring.New(len(defaultFrames))

	for _, frame := range defaultFrames {
		frames.Value = frame
		frames = frames.Next()
	}

	spinner := &Spinner{
		wg:     &sync.WaitGroup{},
		frames: frames,
		// artifact is initialized after options, esp. label, are set.
		state: types.StateCreated,
	}

	logging.Log("Spinner.NewSpinner: Applying options (%d)", len(options))
	for i, option := range options {
		logging.Log("Spinner.NewSpinner: Applying option %d", i)
		option(spinner)
	}

	// Calculate initial display width and create the artifact buffer
	spinner.updateArtifactSize()

	spinner.wg.Add(1)

	logging.Log("Spinner.NewSpinner: Starting render")
	if err := spinner.render(); err != nil {
		spinner.err = err
		logging.Log("Spinner.NewSpinner: Error from render: %v", err)
	}

	logging.Log("Spinner.NewSpinner: Initialization complete, label: '%s', artifact width: %d", spinner.label, spinner.currentDisplayWidth)
	return spinner
}

// updateArtifactSize calculates the required width for the spinner display
// (frame + optional space + label) and creates/resizes the artifact buffer.
func (spinner *Spinner) updateArtifactSize() {
	newWidth := 1 // For the spinner frame itself
	if spinner.label != "" {
		newWidth += 1 + len([]rune(spinner.label)) // +1 for space, then label length
	}

	if spinner.artifact == nil || spinner.currentDisplayWidth != newWidth {
		spinner.currentDisplayWidth = newWidth
		// Assuming types.NewBuffer(width, height)
		// If artifact exists, we might need a Resize method or Close and New.
		// For simplicity, let's assume NewBuffer is fine to call if it's being (re)created.
		// Proper handling might involve closing old artifact if it exists.
		if spinner.artifact != nil {
			spinner.artifact.Close() // Close old one if any
		}
		spinner.artifact = types.NewBuffer(spinner.currentDisplayWidth, 1) // width, height = 1
		logging.Log("Spinner.updateArtifactSize: Artifact (re)created with width %d for label '%s'", spinner.currentDisplayWidth, spinner.label)
	}
}

// Read implements io.Reader - streams the rendered view
func (spinner *Spinner) Read(p []byte) (n int, err error) {
	if spinner.artifact == nil {
		logging.Log("Spinner.Read: Artifact is nil, returning EOF")
		return 0, io.EOF
	}
	n, err = spinner.artifact.Read(p)
	return n, err
}

// Write implements io.Writer - updates spinner state based on commands
func (spinner *Spinner) Write(p []byte) (n int, err error) {
	// TODO: Implement command handling based on `p`.
	// For example, a command "LABEL:new label" could update spinner.label
	// and then call spinner.updateArtifactSize().
	// A command "STATE:SUCCESS" could change spinner.state.
	commandStr := string(p)
	logging.Log("Spinner.Write: Called with command: %s", commandStr)

	if strings.HasPrefix(commandStr, "LABEL:") {
		newLabel := strings.TrimSpace(strings.TrimPrefix(commandStr, "LABEL:"))
		if spinner.label != newLabel {
			spinner.label = newLabel
			spinner.updateArtifactSize() // Resize artifact for new label
			logging.Log("Spinner.Write: Label updated to '%s', artifact resized to width %d", spinner.label, spinner.currentDisplayWidth)
		}
	} else if strings.HasPrefix(commandStr, "STATE:") {
		newStateStr := strings.TrimSpace(strings.TrimPrefix(commandStr, "STATE:"))
		switch newStateStr {
		case "SUCCESS":
			spinner.state = types.StateSuccess
			logging.Log("Spinner.Write: State set to Success")
		case "FAILURE":
			spinner.state = types.StateFailure
			logging.Log("Spinner.Write: State set to Failure")
		case "RUNNING":
			spinner.state = types.StateRunning
			logging.Log("Spinner.Write: State set to Running")
		default:
			logging.Log("Spinner.Write: Unknown state command '%s'", newStateStr)
			return len(p), fmt.Errorf("unknown state command: %s", newStateStr)
		}
	} else {
		logging.Log("Spinner.Write: Received unparseable/unknown command: %s", commandStr)
		// Optionally, return an error for unknown commands
		// return len(p), fmt.Errorf("unknown command: %s", commandStr)
	}

	return len(p), nil
}

// Close implements io.Closer
func (spinner *Spinner) Close() error {
	logging.Log("Spinner.Close: Called, current state: %v", spinner.state)
	switch spinner.state {
	case types.StateCanceled:
		logging.Log("Spinner.Close: StateCanceled")
		return spinner.artifact.Close()
	case types.StateRunning:
		logging.Log("Spinner.Close: StateRunning, calling cancel()")
		if spinner.cancel != nil {
			spinner.cancel()
		} else {
			logging.Log("Spinner.Close: Cancel func was nil for StateRunning")
		}
		spinner.state = types.StateCanceled
		return spinner.artifact.Close()
	case types.StateErrored:
		logging.Log("Spinner.Close: StateErrored")
		return spinner.artifact.Close()
	case types.StateClosed:
		logging.Log("Spinner.Close: Already StateClosed, returning existing error: %v", spinner.err)
		return spinner.err
	case types.StateSuccess, types.StateFailure:
		logging.Log("Spinner.Close: StateSuccess or StateFailure")
		return spinner.artifact.Close()
	default:
		logging.Log("Spinner.Close: Unknown state %v, attempting to close artifact", spinner.state)
		if spinner.artifact != nil {
			return spinner.artifact.Close()
		}
	}
	logging.Log("Spinner.Close: Fallthrough, returning existing error: %v", spinner.err)
	return spinner.err
}

func (spinner *Spinner) render() (err error) {
	spinner.state = types.StateRunning
	logging.Log("Spinner.render: Goroutine starting, state=Running")

	go func() {
		defer logging.Log("Spinner.render: Goroutine finished")
		logging.Log("Spinner.render: Goroutine waiting for wg.Wait() (signal from WithContext)")
		spinner.wg.Wait()
		logging.Log("Spinner.render: Goroutine wg.Wait() done, context should be set.")

		if spinner.ctx == nil {
			logging.Log("Spinner.render: CRITICAL - Context is nil after wg.Wait(). Spinner will not run correctly.")
			spinner.err = fmt.Errorf("spinner context was not set")
			spinner.state = types.StateErrored
			return
		}

		for {
			select {
			case <-spinner.ctx.Done():
				logging.Log("Spinner.render: Context done (reason: %v). Current state: %v", spinner.ctx.Err(), spinner.state)
				if spinner.state != types.StateCanceled && spinner.state != types.StateClosed {
					spinner.state = types.StateCanceled
					if spinner.artifact != nil {
						errClose := spinner.artifact.Close()
						if errClose != nil {
							logging.Log("Spinner.render: Error closing artifact in ctx.Done: %v", errClose)
						}
					}
				}
				return
			case <-time.After(100 * time.Millisecond):
				currentFrameChar := ' ' // Default blank
				var displayString string

				switch spinner.state {
				case types.StateRunning:
					if spinner.frames == nil {
						logging.Log("Spinner.render: StateRunning but frames is nil. Skipping write.")
						continue
					}
					frameVal, ok := spinner.frames.Value.(rune)
					if !ok {
						logging.Log("Spinner.render: Spinner frame value is not a rune (%T). Skipping write.", spinner.frames.Value)
						spinner.frames = spinner.frames.Next()
						continue
					}
					currentFrameChar = frameVal
					spinner.frames = spinner.frames.Next()
				case types.StateSuccess:
					currentFrameChar = successFrame
				case types.StateFailure:
					currentFrameChar = failureFrame
				case types.StateUpdated:
					// Potentially re-use last frame or a specific 'updated' frame
					if spinner.frames != nil && spinner.frames.Value != nil {
						frameVal, ok := spinner.frames.Value.(rune)
						if ok {
							currentFrameChar = frameVal
						}
					}
					logging.Log("Spinner.render: StateUpdated (showing current/last frame)")
				case types.StateCanceled, types.StateClosed, types.StateErrored:
					logging.Log("Spinner.render: State is %v, exiting render loop.", spinner.state)
					return // Exit loop for terminal states
				default:
					logging.Log("Spinner.render: Unknown spinner state %v", spinner.state)
					continue
				}

				if spinner.artifact == nil {
					logging.Log("Spinner.render: Artifact is nil. Skipping write.")
					continue
				}

				// Construct the display string
				if spinner.label != "" {
					displayString = fmt.Sprintf("%c %s", currentFrameChar, spinner.label)
				} else {
					displayString = string(currentFrameChar)
				}

				// Ensure displayString does not exceed artifact width. This is a simplified clear.
				// A proper Buffer.ClearRow or similar would be better.
				// For now, we assume WriteString overwrites or fills from the start.
				// If displayString is shorter than artifact width, it might leave old chars.
				// Let's pad with spaces to the artifact width to clear previous longer labels/frames.
				if len([]rune(displayString)) < spinner.currentDisplayWidth {
					padding := strings.Repeat(" ", spinner.currentDisplayWidth-len([]rune(displayString)))
					displayString += padding
				} else if len([]rune(displayString)) > spinner.currentDisplayWidth {
					// Truncate if too long (should ideally not happen if updateArtifactSize is correct)
					displayString = string([]rune(displayString)[:spinner.currentDisplayWidth])
				}

				spinner.artifact.WriteString(0, 0, displayString)
			}
		}
	}()

	return nil
}

func (spinner *Spinner) WithContext(ctx context.Context) {
	logging.Log("Spinner.WithContext: Called. Current spinner label: '%s'", spinner.label)
	if spinner.wg == nil {
		logging.Log("Spinner.WithContext: CRITICAL - Spinner WaitGroup is nil!")
		spinner.wg = &sync.WaitGroup{}
		spinner.wg.Add(1)
	}
	if ctx == nil {
		logging.Log("Spinner.WithContext: CRITICAL - Provided context is nil!")
		ctx = context.Background()
	}
	spinner.ctx, spinner.cancel = context.WithCancel(ctx)
	logging.Log("Spinner.WithContext: Context and cancel function set. Calling wg.Done() for spinner: '%s'", spinner.label)
	spinner.wg.Done()
}

func WithLabel(label string) SpinnerOption {
	return func(s *Spinner) {
		logging.Log("Spinner.WithLabel: Setting label to '%s'", label)
		s.label = label
		// s.updateArtifactSize() // This should be called after all options in NewSpinner, or if label changes dynamically via Write
	}
}
