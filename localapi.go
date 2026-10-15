package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Device represents a peer that can (or cannot) receive files.
type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	OS       string `json:"os"`
	Online   bool   `json:"online"`
	Taildrop string `json:"taildrop"`
	LastSeen string `json:"lastSeen,omitempty"`
}

// WaitingFile is a file sitting in the local Taildrop inbox.
type WaitingFile struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Arrived time.Time `json:"arrived"`
}

var (
	arrivedMu sync.Mutex
	arrivedAt = map[string]time.Time{}
)

func noteArrival(name string) time.Time {
	arrivedMu.Lock()
	defer arrivedMu.Unlock()
	if t, ok := arrivedAt[name]; ok {
		return t
	}
	t := time.Now()
	arrivedAt[name] = t
	return t
}

// localHTTP returns an HTTP client that talks to tailscaled LocalAPI.
func localHTTP() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialLocalAPI(ctx)
			},
		},
		Timeout: 30 * time.Second,
	}
}

func dialLocalAPI(ctx context.Context) (net.Conn, error) {
	switch runtime.GOOS {
	case "windows":
		// Named pipe on Windows
		return dialWindowsPipe(ctx)
	default:
		paths := []string{
			os.Getenv("TS_SOCKET"),
			"/var/run/tailscale/tailscaled.sock",
			"/var/run/tailscaled.sock",
			filepath.Join(os.Getenv("HOME"), ".tailscale", "tailscaled.sock"),
		}
		var lastErr error
		for _, p := range paths {
			if p == "" {
				continue
			}
			d := net.Dialer{Timeout: 2 * time.Second}
			c, err := d.DialContext(ctx, "unix", p)
			if err == nil {
				return c, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("no tailscaled socket found")
		}
		return nil, lastErr
	}
}

func dialWindowsPipe(ctx context.Context) (net.Conn, error) {
	// On Windows tailscaled uses a named pipe. We try the common path.
	// Full support would need golang.org/x/sys/windows, but for now
	// we fall back to the CLI for Windows users if this fails.
	return nil, fmt.Errorf("windows named pipe: use tailscale CLI fallback")
}

func localGET(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://local-tailscaled.sock"+path, nil)
	if err != nil {
		return nil, err
	}
	return localHTTP().Do(req)
}

func localDELETE(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "DELETE", "http://local-tailscaled.sock"+path, nil)
	if err != nil {
		return nil, err
	}
	return localHTTP().Do(req)
}

// ---- Status ----

type statusResp struct {
	BackendState string `json:"BackendState"`
	Self         *struct {
		HostName string `json:"HostName"`
		ID       string `json:"ID"`
	} `json:"Self"`
	Peer map[string]*peerStatus `json:"Peer"`
}

type peerStatus struct {
	ID                  string `json:"ID"`
	HostName            string `json:"HostName"`
	DNSName             string `json:"DNSName"`
	OS                  string `json:"OS"`
	Online              bool   `json:"Online"`
	LastSeen            string `json:"LastSeen"`
	TaildropTarget      int    `json:"TaildropTarget"`
	NoFileSharingReason string `json:"NoFileSharingReason"`
}

func getStatus(ctx context.Context) (*statusResp, error) {
	res, err := localGET(ctx, "/localapi/v0/status")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("status %d: %s", res.StatusCode, b)
	}
	var st statusResp
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		return nil, err
	}
	return &st, nil
}

// ---- File targets ----

type fileTarget struct {
	Node *struct {
		StableID     string `json:"StableID"`
		Name         string `json:"Name"`
		ComputedName string `json:"ComputedName"`
		Hostinfo     *struct {
			OS string `json:"OS"`
		} `json:"Hostinfo"`
	} `json:"Node"`
}

func listDevices(ctx context.Context) ([]Device, error) {
	// Prefer file-targets for available peers
	res, err := localGET(ctx, "/localapi/v0/file-targets")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var targets []fileTarget
	if res.StatusCode == 200 {
		_ = json.NewDecoder(res.Body).Decode(&targets)
	}

	st, err := getStatus(ctx)
	if err != nil {
		return nil, err
	}

	byID := map[string]Device{}

	for _, ft := range targets {
		if ft.Node == nil {
			continue
		}
		n := ft.Node
		name := n.ComputedName
		if name == "" {
			name = n.Name
		}
		d := Device{
			ID:       n.StableID,
			Name:     name,
			Taildrop: "available",
			Online:   true,
		}
		if n.Hostinfo != nil {
			d.OS = n.Hostinfo.OS
		}
		byID[n.StableID] = d
	}

	for _, ps := range st.Peer {
		if ps == nil {
			continue
		}
		if _, ok := byID[ps.ID]; ok {
			// enrich
			d := byID[ps.ID]
			d.OS = ps.OS
			d.Online = ps.Online
			d.LastSeen = ps.LastSeen
			byID[ps.ID] = d
			continue
		}
		name := ps.HostName
		if name == "" {
			name = ps.DNSName
		}
		d := Device{
			ID:       ps.ID,
			Name:     strings.TrimSuffix(name, "."),
			OS:       ps.OS,
			Online:   ps.Online,
			LastSeen: ps.LastSeen,
		}
		if ps.NoFileSharingReason != "" {
			d.Taildrop = ps.NoFileSharingReason
		} else {
			d.Taildrop = taildropReason(ps.TaildropTarget)
		}
		byID[ps.ID] = d
	}

	out := make([]Device, 0, len(byID))
	for _, d := range byID {
		if strings.Contains(d.Name, "funnel-ingress") {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

func taildropReason(code int) string {
	// Values from ipnstate.TaildropTargetStatus
	switch code {
	case 1:
		return "available"
	case 5:
		return "offline"
	case 8:
		return "owned by another user"
	case 6:
		return "OS does not support Taildrop"
	case 7:
		return "no peer API"
	default:
		return "unavailable"
	}
}

// ---- Inbox ----

type waitingFileAPI struct {
	Name string `json:"Name"`
	Size int64  `json:"Size"`
}

func listInbox(ctx context.Context) ([]WaitingFile, error) {
	res, err := localGET(ctx, "/localapi/v0/files/")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("files %d: %s", res.StatusCode, b)
	}
	var wfs []waitingFileAPI
	if err := json.NewDecoder(res.Body).Decode(&wfs); err != nil {
		return nil, err
	}
	out := make([]WaitingFile, 0, len(wfs))
	for _, wf := range wfs {
		out = append(out, WaitingFile{
			Name:    wf.Name,
			Size:    wf.Size,
			Arrived: noteArrival(wf.Name),
		})
	}
	return out, nil
}

func saveFile(ctx context.Context, name, dir string) (string, error) {
	res, err := localGET(ctx, "/localapi/v0/files/"+name)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("get file %d: %s", res.StatusCode, b)
	}
	size := res.ContentLength

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := conflictFree(dir, name)
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	n, err := io.Copy(f, res.Body)
	if err != nil {
		return "", err
	}
	if size > 0 && n != size {
		return "", fmt.Errorf("short write: got %d of %d", n, size)
	}
	if err := deleteFile(ctx, name); err != nil {
		return "", err
	}
	return target, nil
}

func deleteFile(ctx context.Context, name string) error {
	res, err := localDELETE(ctx, "/localapi/v0/files/"+name)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 204 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("delete %d: %s", res.StatusCode, b)
	}
	return nil
}

func conflictFree(dir, name string) string {
	target := filepath.Join(dir, filepath.Base(name))
	ext := filepath.Ext(target)
	base := strings.TrimSuffix(target, ext)
	for i := 1; ; i++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return target
		}
		target = fmt.Sprintf("%s (%d)%s", base, i, ext)
	}
}

func defaultSaveDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Downloads")
}
