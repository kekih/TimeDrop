package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// App is the Wails backend.
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {}

// GetVersion returns app version.
func (a *App) GetVersion() string {
	return version
}

// GetStatus returns Tailscale daemon status.
func (a *App) GetStatus() map[string]any {
	st, err := getStatus(a.ctx)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	self := ""
	if st.Self != nil {
		self = st.Self.HostName
	}
	return map[string]any{
		"backendState": st.BackendState,
		"self":         self,
		"version":      version,
	}
}

// GetDevices returns tailnet devices for Taildrop.
func (a *App) GetDevices() ([]Device, error) {
	return listDevices(a.ctx)
}

// GetInbox returns waiting Taildrop files.
func (a *App) GetInbox() ([]WaitingFile, error) {
	return listInbox(a.ctx)
}

// SaveFile saves an inbox file to Downloads.
func (a *App) SaveFile(name string) (string, error) {
	return saveFile(a.ctx, name, defaultSaveDir())
}

// DeleteFile removes an inbox file.
func (a *App) DeleteFile(name string) error {
	return deleteFile(a.ctx, name)
}

// SendFiles sends local file paths to a device via tailscale CLI.
func (a *App) SendFiles(device string, paths []string) []map[string]string {
	results := make([]map[string]string, 0, len(paths))
	for _, p := range paths {
		name := filepath.Base(p)
		err := sendPathViaCLI(device, p)
		if err != nil {
			results = append(results, map[string]string{"name": name, "error": err.Error()})
		} else {
			results = append(results, map[string]string{"name": name, "status": "sent"})
		}
	}
	return results
}

func sendPathViaCLI(device, path string) error {
	cmd := exec.Command("tailscale", "file", "cp", path, device+":")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SelectFiles opens a native file dialog and returns selected paths.
// Wails runtime dialog is used from frontend; this is a helper for path send.
func (a *App) SelectAndSend(device string, paths []string) []map[string]string {
	return a.SendFiles(device, paths)
}

// unused keep imports happy if needed
var _ = io.Copy
var _ = os.CreateTemp
