const { app, BrowserWindow, ipcMain, dialog } = require('electron');
const path = require('path');
const fs = require('fs');

// --- GLOBALS ---
let goProcess;
let mainWindow;
const pending = new Map();
let reqId = 0;
const isDev = !app.isPackaged;

// --- HELPER FUNCTIONS ---
function getPreloadPath() {
    return path.join(__dirname, 'preload.js');
}

function getFrontendPath() {
    if (isDev) return 'http://localhost:5173';
    return path.join(__dirname, '..', 'frontend', 'dist', 'index.html');
}

function startGo() {
    const ext = process.platform === 'win32' ? '.exe' : '';
    const goPath = isDev 
        ? path.join(__dirname, '..', 'bin', `myapp-go${ext}`)
        : path.join(process.resourcesPath, 'bin', `myapp-go${ext}`);
        
    if (!fs.existsSync(goPath)) {
        console.error('[Main] ❌ Go binary missing:', goPath);
        return;
    }
    
    console.log('[Main] 🚀 Starting Go:', goPath);
    goProcess = require('child_process').spawn(goPath, { 
        stdio: ['pipe', 'pipe', 'pipe'],
        windowsHide: true,
        cwd: isDev ? process.cwd() : process.resourcesPath
    });

    let buffer = '';
    goProcess.stdout.on('data', chunk => {
        buffer += chunk.toString();
        const lines = buffer.split('\n');
        buffer = lines.pop();
        for (const line of lines) {
        if (!line.trim()) continue;
        try {
            const res = JSON.parse(line);
            const p = pending.get(res.id);
            if (p) {
            if (res.error) p.reject(new Error(res.error));
            else p.resolve(res.result);
            pending.delete(res.id);
            }
        } catch (e) { console.error('[Main] JSON parse error:', e); }
        }
    });

    goProcess.on('close', code => console.log(`[Main] Go exited: ${code}`));
}

function createWindow() {
    mainWindow = new BrowserWindow({
        width: 1200, 
        height: 800,
        title: 'AutoRoot',
        webPreferences: {
        nodeIntegration: false,
        contextIsolation: true,
            preload: getPreloadPath(),
            webSecurity: false,
        allowRunningInsecureContent: true,
        },
    });

    const target = getFrontendPath();
    if (isDev) {
        mainWindow.loadURL(target);
        mainWindow.webContents.openDevTools();
    } else {
        mainWindow.loadFile(target);
    }
}

// --- IPC HANDLERS ---
ipcMain.handle('go:invoke', async (_e, { action, payload }) => {
    return new Promise((resolve, reject) => {
        const id = `req_${++reqId}`;
        pending.set(id, { resolve, reject });
        goProcess?.stdin?.write(JSON.stringify({ id, action, payload }) + '\n');
    });
});

ipcMain.handle('select-firmware-file', async () => {
    const result = await dialog.showOpenDialog(mainWindow, {
        properties: ['openFile'],
        filters: [{ name: 'Firmware ZIP', extensions: ['zip'] }]
    });
    return result.canceled ? null : result.filePaths[0];
});

// --- STARTUP ---
app.whenReady().then(() => {
    startGo();
    createWindow();
});

app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') app.quit();
});

app.on('before-quit', () => {
    if (goProcess) goProcess.kill();
});