package tools

import (
	"io"
	"log"
	"os"
)

// MultiWriter writes to all the provided writers.
type MultiWriter struct {
	Writers []io.Writer
}

func init() {
	logFile, err := os.OpenFile("slm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening log file: %v", err)
	}
	multi := io.MultiWriter(logFile, os.Stdout)
	log.SetOutput(multi)
}

// Write writes bytes to all the writers and returns the number of bytes written to the first writer and any error encountered.
func (t *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.Writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return
}