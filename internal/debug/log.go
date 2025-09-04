package debug

import (
	"fmt"
	"os"
	"time"
)

var logFile *os.File

// Init sets up the logging file.
func Init() {
	var err error
	// Use O_TRUNC to clear the log file on each new run
	logFile, err = os.OpenFile("wifitui-debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		// If we can't open the log file, we can't log the error either.
		// The Log function will just do nothing.
		return
	}
	Log("--- wifitui debug log ---")
}

// Log writes a formatted string to the log file.
func Log(format string, a ...interface{}) {
	if logFile == nil {
		return
	}
	timestamp := time.Now().Format("15:04:05.000")
	logFile.WriteString(fmt.Sprintf("%s: %s\n", timestamp, fmt.Sprintf(format, a...)))
}

// Close flushes and closes the log file.
func Close() {
	if logFile != nil {
		Log("--- session ended ---")
		logFile.Close()
	}
}
