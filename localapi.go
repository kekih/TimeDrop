package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
)

var ts = &local.Client{}

// Device represents a peer that can (or cannot) receive files.
type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	OS       string `json:"os"`
	Online   bool   `json:"online"`
	Taildrop string `json:"taildrop"` // "available" or human reason
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

func listDevices(ctx context.Context) ([]Device, error) {
	fts, err := ts.FileTargets(ctx)
	if err != nil {
		return nil, err
	}
	st, err := ts.Status(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[tailcfg.StableNodeID]Device)

	for _, ft := range fts {
		n := ft.Node
		d := Device{
			ID:       string(n.StableID),
			Name:     n.ComputedName,
			Taildrop: "available",
		}
		if ps, ok := st.Peer[n.Key]; ok {
			d.OS = ps.OS
			d.Online = ps.Online
			if !ps.LastSeen.IsZero() {
				d.LastSeen = ps.LastSeen.Format(time.RFC3339)
			}
		}
		byID[n.StableID] = d
	}

	// Add peers that cannot receive files (with reason)
	for _, ps := range st.Peer {
		if _, ok := byID[ps.ID]; ok {
			continue
		}
		d := Device{
			ID:     string(ps.ID),
			Name:   ps.HostName,
			OS:     ps.OS,
			Online: ps.Online,
		}
		if !ps.LastSeen.IsZero() {
			d.LastSeen = ps.LastSeen.Format(time.RFC3339)
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

func taildropReason(t ipnstate.TaildropTargetStatus) string {
	switch t {
	case ipnstate.TaildropTargetAvailable:
		return "available"
	case ipnstate.TaildropTargetOffline:
		return "offline"
	case ipnstate.TaildropTargetOwnedByOtherUser:
		return "owned by another user"
	case ipnstate.TaildropTargetUnsupportedOS:
		return "OS does not support Taildrop"
	case ipnstate.TaildropTargetNoPeerAPI:
		return "no peer API"
	default:
		return "unavailable"
	}
}

func listInbox(ctx context.Context) ([]WaitingFile, error) {
	wfs, err := ts.WaitingFiles(ctx)
	if err != nil {
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
	rc, size, err := ts.GetWaitingFile(ctx, name)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	target := conflictFree(dir, name)
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	n, err := io.Copy(f, rc)
	if err != nil {
		return "", err
	}
	if n != size {
		return "", fmt.Errorf("short write: got %d of %d", n, size)
	}
	if err := ts.DeleteWaitingFile(ctx, name); err != nil {
		return "", err
	}
	return target, nil
}

func deleteFile(ctx context.Context, name string) error {
	return ts.DeleteWaitingFile(ctx, name)
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

// defaultSaveDir returns the user's Downloads folder.
func defaultSaveDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Downloads")
}
