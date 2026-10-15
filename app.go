package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// App is the Wails backend bound to the frontend.
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

func (a *App) GetVersion() string {
	return version
}

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

func (a *App) GetDevices() ([]Device, error) {
	return listDevices(a.ctx)
}

func (a *App) GetInbox() ([]WaitingFile, error) {
	return listInbox(a.ctx)
}

func (a *App) SaveFile(name string) (string, error) {
	return saveFile(a.ctx, name, defaultSaveDir())
}

func (a *App) DeleteFile(name string) error {
	return deleteFile(a.ctx, name)
}

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
