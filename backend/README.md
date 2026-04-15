# Backend Implementation Guide

This directory contains the backend implementation for AutoRoot Desktop.

## Supported Backends

### Python Backend (Recommended)
- Uses `pyusb` for USB device detection
- Integrates with existing flashing tools (Odin, Fastboot, MTKClient)
- Communicates with Electron via IPC

### Go Backend (Alternative)
- Better performance for concurrent operations
- Native binary compilation
- Uses `gousb` for USB handling

## Required Tools (to be installed on user's PC)

1. **Samsung Devices**: Odin CLI or Loke
2. **Fastboot Devices**: Android SDK Platform Tools
3. **MediaTek Devices**: MTKClient or SP Flash Tool
4. **Qualcomm EDL**: QFIL or edl binary

## File Structure

```
backend/
├── main.py          # Python backend entry point
├── usb_detector.py  # USB VID/PID detection
├── flasher.py       # Flashing protocol handlers
├── asset_index.py   # Local warehouse indexing
└── database.py      # SQLite model mapping
```

## Protocol Detection Logic

Device detection is based on USB VID:PID pairs:

- **Samsung**: 04E8 (Download Mode), 04E8 (Odin Mode)
- **Xiaomi**: 2717 (Fastboot), 2717 (EDL)
- **MediaTek**: 0E8D (BROM/Bypass mode)
- **Qualcomm**: 05C6 (EDL mode: 9008, 900E)

## Next Steps for Full Implementation

1. Install required Python packages: `pip install pyusb sqlite3`
2. Download platform-tools binary for your OS
3. Implement actual USB polling in electron/main.js
4. Create native bindings for flashing protocols
