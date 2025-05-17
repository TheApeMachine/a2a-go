package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	GlobalLogger *log.Logger
	logFile      *os.File
)

// Init initializes the global logger to write to a file.
func Init(logFilePath string) error {
	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logFilePath, err)
	}

	GlobalLogger = log.New(logFile, "", 0) // No prefix, no flags (timestamp will be added manually)
	GlobalLogger.SetOutput(logFile)
	Log("Logging initialized to file: %s", logFilePath)
	return nil
}

// Log formats and writes a log message to the global logger.
// It includes a timestamp and caller info.
func Log(format string, v ...interface{}) {
	if GlobalLogger == nil {
		// Fallback to stdout if logger not initialized (shouldn't happen if Init is called first)
		fmt.Printf("[NO_LOGGER_INIT] "+format+"\n", v...)
		return
	}

	// Get caller info
	_, file, line, ok := runtime.Caller(1)
	callerInfo := ""
	if ok {
		callerInfo = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	msg := fmt.Sprintf(format, v...)
	timestamp := time.Now().Format("2006-01-02 15:04:05.000000")
	GlobalLogger.Printf("%s [%s] %s", timestamp, callerInfo, msg)
}

// Close closes the log file.
func Close() {
	if logFile != nil {
		Log("Closing log file.")
		logFile.Close()
	}
}
