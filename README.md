# TimeDrop

**Beautiful, fast & animated GUI for Tailscale Taildrop**

Send and receive files between your devices over the encrypted Tailscale mesh — with a modern, smooth interface.

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go) ![License](https://img.shields.io/badge/license-MIT-blue) ![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)

## Features

- 🌈 **Beautiful dark UI** with glassmorphism and smooth CSS animations
- ⚡ **Extremely fast** — single static binary, zero heavy frameworks
- 📤 **Send** files via drag-and-drop or file picker to any online device
- 📥 **Inbox** — see incoming files instantly, save or delete with one click
- 📊 Live progress bars and device status (online / offline / reason)
- 🔒 Uses official Tailscale LocalAPI (same as `tailscale file`)
- 📁 Auto-saves to Downloads (configurable)

## Requirements

- Running `tailscaled` (Tailscale must be installed and logged in)
- Taildrop enabled in your tailnet admin console

## Install

### From releases (recommended)

Download the latest binary for your OS from the [Releases](https://github.com/kekih/TimeDrop/releases) page.

```bash
# Linux / macOS example
chmod +x timedrop
./timedrop
```

Then open http://127.0.0.1:8742 in your browser.

### Build from source

```bash
git clone https://github.com/kekih/TimeDrop.git
cd TimeDrop
go build -ldflags="-s -w" -o timedrop .
./timedrop
```

## Usage

1. Make sure Tailscale is running and Taildrop is enabled.
2. Launch TimeDrop.
3. Open the local web UI (it opens automatically if possible).
4. Drag files onto the window or use the Send tab to choose a device.
5. Incoming files appear in the Inbox tab with one-click Save / Delete.

## Architecture

```
TimeDrop (Go)
├─ LocalAPI client  → talks to tailscaled (same interface as CLI)
├─ Lightweight HTTP server on 127.0.0.1:8742
└─ Embedded modern web UI (HTML + CSS + vanilla JS)
```

No Electron, no Wails runtime, no Node — just one fast binary.

## License

MIT
