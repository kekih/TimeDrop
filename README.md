# TimeDrop

**Native desktop GUI for Tailscale Taildrop**

Beautiful dark UI, animations, system window — not a browser tab.

## Features

- Native desktop window (Wails + system webview)
- Send files to devices on your tailnet
- Inbox: save / delete incoming Taildrop files
- Live device list and daemon status
- Single binary, no Electron

## Requirements

- Tailscale installed and running (`tailscaled`)
- Taildrop enabled in admin console

## Build (local)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
git clone https://github.com/kekih/TimeDrop.git
cd TimeDrop
go mod tidy
wails build
```

Binary appears in `build/bin/`.

### Linux deps

```bash
# Debian/Ubuntu
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
```

## Release (GitHub Actions)

Push a version tag — CI builds desktop binaries and publishes a Release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Or: **Actions → Release → Run workflow**.

## License

MIT
