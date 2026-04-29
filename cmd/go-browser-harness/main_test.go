package main

import (
	"errors"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errWriteFailed
}

var errWriteFailed = errors.New("write failed")

func TestWriteLineReturnsWriterError(t *testing.T) {
	err := writeLine(failingWriter{}, "version")
	if !errors.Is(err, errWriteFailed) {
		t.Fatalf("writeLine error = %v, want %v", err, errWriteFailed)
	}
}
