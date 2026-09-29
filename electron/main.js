const { app, BrowserWindow, ipcMain } = require('electron');
const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

let goProcess;
let mainWindow;
const pending = new Map();
let reqId = 0;
const isDev = !app.isPackaged;

// The backend is the only thing that touches the phone. This whitelist is the
// trust boundary: an action that is not listed here cannot be invoked from the
// renderer, even if the UI is compromised.
//
// Note what is deliberately absent: there is no "flash" action. Flashing
// requires building a plan, which the backend refuses without an explicit
// approval recorded in the persisted session. Keeping the destructive step out
// of the IPC surface means a stray click cannot start it.
const allowedActions = new Set([
    'ping',
    'getDeviceInfo',
    'checkFirmware',
    'rebootToDownloadMode',
    'checkOdinAvailability',
    'verifyRootAfterFlash',
    'extractFirmwareToFolder',
    'handleDroppedFirmware',
    'transferFileToDevice',
    'ensureMagiskInstalled',
    'keepDeviceAwake',
    'getLatestMagiskPatchedFile',
    'listAvailableFirmware',
    // Hardened workflow
    'startSession',
    'adoptFirmware',
    'fetchFirmware',
    'recordPatchedAP',
    'preflight',
    'dryRun',
    'engineStatus',
    'approveFlash',
    'flashPlan',
    'verifyRoot',
    'sessionState',
    'resetSession',
]);

function getGoPath() {
    const ext = process.platform === 'win32' ? '.exe' : '';
    const bin = `myapp-go${ext}`;
    if (isDev) {
        const developmentBuild = path.join(__dirname, '..', 'bin', `myapp-go.new${ext}`);
        if (fs.existsSync(developmentBuild)) return developmentBuild;
        return path.join(__dirname, '..', 'bin', bin);
    }
    return path.join(process.resourcesPath, 'bin', bin);
}

function getFrontendPath() {
    if (isDev) return 'http://localhost:5173';
    
    // In packaged app, files live inside resources/app.asar (or unpacked)
    const indexPath = path.join(app.getAppPath(), 'frontend', 'dist', 'index.html');
    console.log('[Main] 📂 Resolved path:', indexPath);
    console.log('[Main] ✅ Exists?', fs.existsSync(indexPath));
    return indexPath;
}

function startGo() {
    const goPath = getGoPath();
    if (!fs.existsSync(goPath)) {
        console.error('[Main] ❌ Go binary missing:', goPath);
        return;
    }
    
    console.log('[Main] 🚀 Starting Go:', goPath);
    goProcess = spawn(goPath, { 
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
        if (!line.trimStart().startsWith('{')) {
            console.log('[Go]', line);
            continue;
        }
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
    goProcess.on('close', code => {
        for (const { reject, timer } of pending.values()) {
            clearTimeout(timer);
            reject(new Error(`Backend stopped unexpectedly (exit code ${code}).`));
        }
        pending.clear();
        goProcess = null;
    });

    goProcess.on('error', err => {
        console.error('[Main] Failed to start Go backend:', err);
    });
}

function createWindow() {
    mainWindow = new BrowserWindow({
        width: 1200, height: 800,
        title: 'AutoRoot',
        webPreferences: {
        nodeIntegration: false,
        contextIsolation: true,
        preload: path.join(__dirname, 'preload.js'),
        webSecurity: true,
        allowRunningInsecureContent: false,
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

ipcMain.handle('go:invoke', async (_e, action, payload) => {
    if (!allowedActions.has(action)) {
        throw new Error(`Backend action is not allowed: ${action}`);
    }
    if (!goProcess || !goProcess.stdin || goProcess.killed) {
        throw new Error('Backend is not running. Restart AutoRoot and try again.');
    }

    return new Promise((resolve, reject) => {
        const id = `req_${++reqId}`;
        const timeoutMs = action === 'verifyRootAfterFlash' ? 11 * 60 * 1000 : 5 * 60 * 1000;
        const timer = setTimeout(() => {
            pending.delete(id);
            reject(new Error(`Backend timed out while running ${action}.`));
        }, timeoutMs);

        pending.set(id, {
            resolve: value => { clearTimeout(timer); resolve(value); },
            reject: error => { clearTimeout(timer); reject(error); },
            timer,
        });

        goProcess.stdin.write(JSON.stringify({ id, action, payload }) + '\n', err => {
            if (!err) return;
            clearTimeout(timer);
            pending.delete(id);
            reject(err);
        });
    });
});

app.whenReady().then(() => { startGo(); createWindow(); });
app.on('window-all-closed', () => process.platform !== 'darwin' && app.quit());
app.on('before-quit', () => goProcess?.kill());
