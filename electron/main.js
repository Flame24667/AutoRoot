const { app, BrowserWindow, ipcMain } = require('electron');
const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

let goProcess;
let mainWindow;
const pendingRequests = new Map();
let requestId = 0;

// ✅ Correct dev/prod detection
const isDev = !app.isPackaged || process.env.NODE_ENV === 'development';

function getGoBinaryPath() {
    const ext = process.platform === 'win32' ? '.exe' : '';
    const binaryName = `myapp-go${ext}`;
    
    if (isDev) {
        // Dev: binary is in project root /bin
        return path.join(__dirname, '..', 'bin', binaryName);
    } else {
        // Prod: binary is in resources/bin (bundled by electron-builder)
        return path.join(process.resourcesPath, 'bin', binaryName);
    }
}

function getFrontendPath() {
    if (isDev) {
        return 'http://localhost:5173';
    } else {
        // Prod: Load built React files from packaged resources
        // electron-builder puts files in resources/app/
        return path.join(process.resourcesPath, 'app', 'frontend', 'dist', 'index.html');
    }
}

function startGoBackend() {
    const goPath = getGoBinaryPath();
    
    // Windows-specific: hide the console window
    const spawnOptions = {
        stdio: ['pipe', 'pipe', 'pipe'], // Hide stderr too, or use 'inherit' to see errors
        cwd: isDev ? process.cwd() : process.resourcesPath,
        windowsHide: true, // ← THIS HIDES THE COMMAND WINDOW ON WINDOWS
    };
    
    // Verify binary exists
    if (!fs.existsSync(goPath)) {
        console.error('[main] ❌ Go binary not found at:', goPath);
        return;
    }
    
    console.log('[main] 🚀 Starting Go backend:', goPath);
    
    goProcess = spawn(goPath, spawnOptions);

    let buffer = '';
    goProcess.stdout.on('data', (chunk) => {
        buffer += chunk.toString();
        const lines = buffer.split('\n');
        buffer = lines.pop();
        
        for (const line of lines) {
        if (!line.trim()) continue;
        try {
            const response = JSON.parse(line);
            const pending = pendingRequests.get(response.id);
            if (pending) {
            if (response.error) {
                pending.reject(new Error(response.error));
            } else {
                pending.resolve(response.result);
            }
            pendingRequests.delete(response.id);
            }
        } catch (err) {
            console.error('[main] Failed to parse Go response:', err);
        }
        }
    });

    goProcess.on('error', (err) => {
        console.error('[main] Go process error:', err);
    });
    
    goProcess.on('close', (code) => {
        console.log(`[main] Go process exited with code ${code}`);
    });
}

function createWindow() {
    mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    title: 'AutoRoot',
    webPreferences: {
        nodeIntegration: false,
        contextIsolation: true,
        preload: path.join(__dirname, 'preload.js'),
        webSecurity: false, // ← DISABLES CSP (works but less secure)
        allowRunningInsecureContent: true,
        },
    });

    const target = getFrontendPath();
    console.log('[main] Loading:', target);
    
    if (isDev) {
        mainWindow.loadURL(target);
        mainWindow.webContents.openDevTools();
    } else {
        mainWindow.loadFile(target);
    }

    mainWindow.on('closed', () => {
        mainWindow = null;
    });
}

// IPC bridge: React → Go
ipcMain.handle('go:invoke', async (_event, action, payload) => {
    return new Promise((resolve, reject) => {
        const id = `req_${++requestId}`;
        pendingRequests.set(id, { resolve, reject });
        
        const message = JSON.stringify({ id, action, payload }) + '\n';
        if (goProcess?.stdin?.writable) {
        goProcess.stdin.write(message);
        } else {
        pendingRequests.delete(id);
        reject(new Error('Go backend not ready'));
        }
    });
});

app.whenReady().then(() => {
    startGoBackend();
    createWindow();
    
    app.on('activate', () => {
        if (BrowserWindow.getAllWindows().length === 0) {
        createWindow();
        }
    });
});

app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') {
        app.quit();
    }
});

// Graceful shutdown
app.on('before-quit', () => {
    if (goProcess) {
        console.log('[main] Shutting down Go backend...');
        goProcess.kill();
    }
});