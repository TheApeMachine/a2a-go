package utils

// import (
// 	"fmt"
// 	"io"
// 	"os"
// 	"os/exec"
// 	"runtime"

// 	"github.com/charmbracelet/log"
// 	"github.com/zeozeozeo/gaudio"
// )

// // /*
// // playPCM plays audio data using gaudio which doesn't require CGO.
// // */
// // func PlayPCM(r io.Reader) error {
// // 	segment, err := gaudio.LoadAudio(r, gaudio.FormatRAW)

// // 	if err != nil {
// // 		return fmt.Errorf("failed to load audio data: %w", err)
// // 	}

// // 	tmpFile, err := os.CreateTemp("", "tts-*.wav")

// // 	if err != nil {
// // 		return fmt.Errorf("failed to create temporary file: %w", err)
// // 	}

// // 	defer func() {
// // 		tmpFile.Close()
// // 		os.Remove(tmpFile.Name())
// // 	}()

// // 	if err := segment.Export(tmpFile, gaudio.FormatWAVE); err != nil {
// // 		return fmt.Errorf("failed to export audio to WAV: %w", err)
// // 	}

// // 	var cmd string
// // 	var args []string
// // 	switch runtime.GOOS {
// // 	case "darwin":
// // 		cmd = "afplay"
// // 		args = []string{tmpFile.Name()}
// // 	case "linux":
// // 		cmd = "aplay"
// // 		args = []string{tmpFile.Name()}
// // 	case "windows":
// // 		cmd = "powershell"
// // 		args = []string{"-c", "(New-Object Media.SoundPlayer '" + tmpFile.Name() + "').PlaySync()"}
// // 	default:
// // 		log.Error("unsupported operating system", "os", runtime.GOOS)
// // 		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
// // 	}

// // 	if err := exec.Command(cmd, args...).Run(); err != nil {
// // 		log.Error(err)
// // 		return err
// // 	}

// // 	return nil
// // }
