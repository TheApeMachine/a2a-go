package file

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/theapemachine/a2a-go/pkg/errors"
)

/*
Handler manages file operations with streaming support.
*/
type Handler struct {
	mu      sync.RWMutex
	handles map[string]*os.File
	baseDir string
}

/*
NewHandler creates a new file handler.
*/
func NewHandler(baseDir string) (*Handler, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, errors.ErrInternal.WithMessagef("failed to create base directory: %v", err)
	}

	return &Handler{
		handles: make(map[string]*os.File),
		baseDir: baseDir,
	}, nil
}

/*
Open opens a file for reading or writing.
*/
func (h *Handler) Open(ctx context.Context, name string, mode int) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	path := filepath.Join(h.baseDir, name)
	file, err := os.OpenFile(path, mode, 0644)
	if err != nil {
		return "", errors.ErrInternal.WithMessagef("failed to open file: %v", err)
	}

	handle := name
	h.handles[handle] = file

	return handle, nil
}

/*
Read reads from a file handle.
*/
func (h *Handler) Read(ctx context.Context, handle string, p []byte) (int, error) {
	h.mu.RLock()
	file, ok := h.handles[handle]
	h.mu.RUnlock()

	if !ok {
		return 0, errors.ErrInvalidParams.WithMessagef("invalid handle: %s", handle)
	}

	return file.Read(p)
}

/*
Write writes to a file handle.
*/
func (h *Handler) Write(ctx context.Context, handle string, p []byte) (int, error) {
	h.mu.RLock()
	file, ok := h.handles[handle]
	h.mu.RUnlock()

	if !ok {
		return 0, errors.ErrInvalidParams.WithMessagef("invalid handle: %s", handle)
	}

	return file.Write(p)
}

/*
Close closes a file handle.
*/
func (h *Handler) Close(ctx context.Context, handle string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	file, ok := h.handles[handle]
	if !ok {
		return errors.ErrInvalidParams.WithMessagef("invalid handle: %s", handle)
	}

	delete(h.handles, handle)
	return file.Close()
}

/*
Upload handles file upload with base64 encoding.
*/
func (h *Handler) Upload(ctx context.Context, name string, content string) error {
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return errors.ErrInvalidParams.WithMessagef("invalid base64 content: %v", err)
	}

	path := filepath.Join(h.baseDir, name)
	return os.WriteFile(path, data, 0644)
}

/*
Download handles file download with base64 encoding.
*/
func (h *Handler) Download(ctx context.Context, name string) (string, error) {
	path := filepath.Join(h.baseDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.ErrInternal.WithMessagef("failed to read file: %v", err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

/*
ToBase64 converts a file to base64.
*/
func (h *Handler) ToBase64(ctx context.Context, handle string) (string, error) {
	h.mu.RLock()
	file, ok := h.handles[handle]
	h.mu.RUnlock()

	if !ok {
		return "", errors.ErrInvalidParams.WithMessagef("invalid handle: %s", handle)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", errors.ErrInternal.WithMessagef("failed to read file: %v", err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

/*
FromBase64 creates a file from base64 content.
*/
func (h *Handler) FromBase64(ctx context.Context, name string, content string) error {
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return errors.ErrInvalidParams.WithMessagef("invalid base64 content: %v", err)
	}

	path := filepath.Join(h.baseDir, name)
	return os.WriteFile(path, data, 0644)
}

/*
Seek sets the offset for the next Read or Write on file.
*/
func (h *Handler) Seek(ctx context.Context, handle string, offset int64, whence int) (int64, error) {
	h.mu.RLock()
	file, ok := h.handles[handle]
	h.mu.RUnlock()

	if !ok {
		return 0, errors.ErrInvalidParams.WithMessagef("invalid handle: %s", handle)
	}

	return file.Seek(offset, whence)
}
