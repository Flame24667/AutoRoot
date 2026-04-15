# 🔓 AutoRoot Desktop

**Universal Offline-First Rooting Solution v1.0**

A desktop application for automated phone rooting that works 100% offline. Solves the problem of dead download links and long download times by storing all root packages in local storage.

## Features

- **100% Offline Operation**: No internet required after initial setup
- **Auto Device Detection**: Automatically detects phone model via USB VID/PID
- **Multi-Protocol Support**:
  - Samsung Odin/Loke protocol (.tar files)
  - Universal Fastboot (Xiaomi, Google Pixel, Realme, etc.)
  - MediaTek BROM/Bypass mode
  - Qualcomm EDL mode (9008/900E)
- **Local Warehouse**: Drag-and-drop firmware importer
- **Driver Health Check**: Ensures required drivers are installed
- **Real-time Terminal Log**: Transparent process monitoring
- **Portable Build**: Single .exe installer for Windows

## Tech Stack

- **Frontend**: React + Vite
- **Desktop Framework**: Electron
- **State Management**: Zustand
- **Backend**: Python/Go (via IPC)
- **Database**: SQLite (local model mapping)

## Project Structure

```
autoroot-desktop/
├── electron/           # Electron main & preload scripts
│   ├── main.js
│   └── preload.js
├── src/
│   ├── components/     # React UI components
│   │   ├── DeviceMonitor.jsx
│   │   ├── FlashControl.jsx
│   │   └── AssetImporter.jsx
│   ├── store/          # Zustand state management
│   │   └── index.js
│   ├── App.jsx
│   └── App.css
├── backend/            # Python/Go backend (to be implemented)
├── assets/             # Local firmware warehouse
├── package.json
└── README.md
```

## Installation & Development

### Prerequisites

- Node.js 18+ 
- npm or yarn
- Python 3.8+ (for backend)
- Android SDK Platform Tools (for Fastboot)

### Setup

```bash
# Install dependencies
cd autoroot-desktop
npm install

# Run in development mode
npm run electron:dev

# Build for production
npm run electron:build
```

### Backend Setup (Python)

```bash
cd backend
pip install pyusb sqlite3

# Run backend server
python main.py
```

## Usage

1. **Import Firmware**: Drag and drop .tar, .img, or .zip files to the Local Warehouse
2. **Connect Device**: Plug in your phone (in appropriate mode: Download, Fastboot, or EDL)
3. **Auto-Detection**: The app automatically identifies the device model and protocol
4. **Start Root**: Click "Start Root" and monitor progress in the terminal log

## Supported Devices

| Brand | Protocol | File Format | Mode |
|-------|----------|-------------|------|
| Samsung | Odin/Loke | .tar | Download Mode |
| Xiaomi | Fastboot | .img | Fastboot Mode |
| Google Pixel | Fastboot | .img | Bootloader |
| Realme | Fastboot | .img | Fastboot Mode |
| MediaTek | MTKClient | .img | BROM/Bypass |
| Qualcomm | EDL | .mbn/.elf | EDL (9008) |

## Safety Warnings

⚠️ **USE AT YOUR OWN RISK**

- Rooting may void your warranty
- Incorrect flashing can brick your device
- Always backup important data before proceeding
- This tool is for educational purposes only

## Troubleshooting

### Device Not Detected
1. Ensure proper USB drivers are installed
2. Try different USB port/cable
3. Reboot phone into correct mode

### Missing Drivers
- Samsung: Install Samsung USB Driver
- Xiaomi: Install Mi USB Driver
- Generic: Install Universal ADB Driver

### Flash Failed
- Verify file integrity (checksum)
- Ensure sufficient battery (>50%)
- Don't disconnect during flashing

## License

MIT License - For educational purposes only

## Contributing

Contributions welcome! Please read our contributing guidelines first.

---

**Version**: 1.0  
**Build**: Alpha  
**Last Updated**: 2024
