# New API Electron Desktop App

This directory contains the Electron wrapper for New API, providing a native desktop application with system tray support for Windows, macOS, and Linux.

## Prerequisites

- Node.js 22.12.0 or newer
- Bun 1.3.14

### 1. Go Binary (Required)
The Electron app requires the compiled Go binary to function. You have two options:

**Option A: Use existing binary (without Go installed)**
```bash
# If you have a pre-built binary (e.g., new-api-macos)
cp ../new-api-macos ../new-api
```

**Option B: Build from source (requires Go)**
Run from the repository root:

```bash
cd web
bun install --frozen-lockfile
cd default
DISABLE_ESLINT_PLUGIN=true VITE_REACT_APP_VERSION="$(tr -d '\r\n' < ../../VERSION)" bun run build
cd ../classic
VITE_REACT_APP_VERSION="$(tr -d '\r\n' < ../../VERSION)" bun run build
cd ../..

VERSION="$(tr -d '\r\n' < VERSION)"
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$VERSION'" \
  -o new-api
```

On Windows, build the backend as `new-api.exe` instead. The Electron package configuration selects the platform-specific filename automatically.

### 2. Electron Dependencies
```bash
cd electron
npm ci
npm test
```

## Development

Run the app in development mode:
```bash
npm run start-app
```

This will:
- Start the Go backend on port 3000
- Open an Electron window with DevTools enabled
- Create a system tray icon (menu bar on macOS)
- Store database in `../data/new-api.db`

## Building for Production

### Quick Build
```bash
# Run from the electron directory. This builds both frontends, the Go binary,
# the Electron tests, and the package for the current platform.
bash ./build.sh
```

### Package an Existing Build
If the frontend and platform Go binary are already built:

```bash
npm ci
npm test

npm run build:mac    # Creates .dmg and .zip
npm run build:win    # Creates .exe installer
npm run build:linux  # Creates .AppImage and .deb
```

### Build Output
- Built applications are in `electron/dist/`
- macOS: `.dmg` (installer) and `.zip` (portable)
- Windows: `.exe` (installer) and portable exe
- Linux: `.AppImage` and `.deb`

## Configuration

### Port
Default port is 3000. To change, edit `main.js`:
```javascript
const PORT = 3000; // Change to desired port
```

### Database Location
- **Development**: `../data/new-api.db` (project directory)
- **Production**:
  - macOS: `~/Library/Application Support/New API/data/`
  - Windows: `%APPDATA%/New API/data/`
  - Linux: `~/.config/New API/data/`
