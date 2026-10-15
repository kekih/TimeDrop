package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	st, err := ts.Status(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"backendState": st.BackendState,
		"self":         st.Self.HostName,
		"version":      version,
	})
}

func handleDevices(w http.ResponseWriter, r *http.Request) {
	devs, err := listDevices(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, devs)
}

func handleInbox(w http.ResponseWriter, r *http.Request) {
	files, err := listInbox(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, files)
}

func handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var req struct {
		Name string `json:"name"`
		Dir  string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	if req.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "name required"})
		return
	}
	if req.Dir == "" {
		req.Dir = defaultSaveDir()
	}
	path, err := saveFile(r.Context(), req.Name, req.Dir)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"path": path})
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "POST/DELETE only", 405)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	if err := deleteFile(r.Context(), req.Name); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}

	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	device := r.FormValue("device")
	if device == "" {
		writeJSON(w, 400, map[string]string{"error": "device required"})
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeJSON(w, 400, map[string]string{"error": "no files"})
		return
	}

	results := make([]map[string]string, 0, len(files))
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			results = append(results, map[string]string{"name": fh.Filename, "error": err.Error()})
			continue
		}

		err = sendViaCLI(device, fh.Filename, f)
		f.Close()
		if err != nil {
			results = append(results, map[string]string{"name": fh.Filename, "error": err.Error()})
		} else {
			results = append(results, map[string]string{"name": fh.Filename, "status": "sent"})
		}
	}
	writeJSON(w, 200, results)
}

func sendViaCLI(device, name string, r io.Reader) error {
	tmp, err := os.CreateTemp("", "timedrop-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	final := filepath.Join(filepath.Dir(tmp.Name()), name)
	if err := os.Rename(tmp.Name(), final); err != nil {
		return err
	}
	defer os.Remove(final)

	cmd := exec.Command("tailscale", "file", "cp", final, device+":")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
