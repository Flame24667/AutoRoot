const { app, BrowserWindow, ipcMain, dialog } = require('electron');
const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

let goProcess;
let mainWindow;
const pending = new Map();
let reqId = 0;
let destructiveJob = false;
const isDev = !app.isPackaged;
const staticUI = process.env.AUTOROOT_STATIC_UI === '1';

// The backend is the only thing that touches the phone. This whitelist is the
// trust boundary: an action that is not listed here cannot be invoked from the
// renderer, even if the UI is compromised.
//
// Flashing is exposed only as a guarded background workflow job; backend
// preflight, file/device-bound approval and final wipe confirmation remain mandatory.
const allowedActions = new Set([
    'ping',
    'databasePlan',
    'startAutomationJob',
    'automationStatus',
    'getDeviceInfo',
    'checkFirmware',
    'checkOdinAvailability',
    'verifyRootAfterFlash',
    'extractFirmwareToFolder',
    'handleDroppedFirmware',
    'listAvailableFirmware',
    // Hardened workflow
    'startSession',
    'beginNewRun',
    'preflight',
    'dryRun',
    'engineStatus',
    'diagnoseEngineLaunch',
    'flashPlan',
    'verifyRoot',
    'sessionState',
    'resetSession',
]);

function getGoPath() {
    const ext = process.platform === 'win32' ? '.exe' : '';
    const bin = `myapp-go${ext}`;
    if (isDev) {
        const automationBuild = path.join(__dirname, '..', 'bin', `myapp-go.automation${ext}`);
        if (fs.existsSync(automationBuild)) return automationBuild;
        const developmentBuild = path.join(__dirname, '..', 'bin', `myapp-go.new${ext}`);
        if (fs.existsSync(developmentBuild)) return developmentBuild;
        return path.join(__dirname, '..', 'bin', bin);
    }
    return path.join(process.resourcesPath, 'bin', bin);
}

function getFrontendPath() {
    if (isDev && !staticUI) return 'http://localhost:5173';
    if (isDev && staticUI) return path.join(__dirname, '..', 'work-cache', 'poc-ui', 'index.html');
    
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
            if (res.result?.operation === 'executeFlash') destructiveJob = res.result.status === 'running';
            clearTimeout(p.timer);
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
    mainWindow.on('close', event => {
        if (destructiveJob) {
            event.preventDefault();
            dialog.showErrorBox('Flashing in progress', 'Do not close AutoRoot or disconnect USB until flashing completes.');
        }
    });

    const target = getFrontendPath();
    if (isDev && !staticUI) {
        mainWindow.loadURL(target);
        mainWindow.webContents.openDevTools();
    } else {
        mainWindow.loadFile(target);
    }
}

// Selecting a firmware package is a deliberate act, so it goes through a
// native dialog filtered to .zip rather than a text field. The path is returned
// to the renderer but not trusted: the backend validates model, sales code,
// anti-rollback, ZIP integrity and every internal MD5 before it can be used.
ipcMain.handle('dialog:pickFirmware', async () => {
    const result = await dialog.showOpenDialog(mainWindow, {
        title: 'Select a Samsung firmware package',
        properties: ['openFile'],
        filters: [{ name: 'Firmware package', extensions: ['zip'] }],
    });
    if (result.canceled || result.filePaths.length === 0) return null;
    return result.filePaths[0];
});

ipcMain.handle('go:invoke', async (_e, action, payload) => {
    if (!allowedActions.has(action)) {
        throw new Error(`Backend action is not allowed: ${action}`);
    }
    if (!goProcess || !goProcess.stdin || goProcess.killed) {
        throw new Error('Backend is not running. Restart AutoRoot and try again.');
    }

    // Protect the window before dispatch, not only after the job-start reply.
    const startsFlash = action === 'startAutomationJob' && payload?.operation === 'executeFlash';
    if (startsFlash) destructiveJob = true;

    return new Promise((resolve, reject) => {
        const id = `req_${++reqId}`;
        const timeoutMs = action === 'verifyRootAfterFlash' ? 11 * 60 * 1000 : 5 * 60 * 1000;
        const timer = setTimeout(() => {
            pending.delete(id);
            reject(new Error(`Backend timed out while running ${action}.`));
        }, timeoutMs);

        pending.set(id, {
            resolve: value => { clearTimeout(timer); resolve(value); },
            reject: error => { clearTimeout(timer); if (startsFlash) destructiveJob = false; reject(error); },
            timer,
        });

        goProcess.stdin.write(JSON.stringify({ id, action, payload }) + '\n', err => {
            if (!err) return;
            clearTimeout(timer);
            pending.delete(id);
            if (startsFlash) destructiveJob = false;
            reject(err);
        });
    });
});

app.whenReady().then(() => { startGo(); createWindow(); });
app.on('window-all-closed', () => process.platform !== 'darwin' && app.quit());
app.on('before-quit', event => {
    if (destructiveJob) {
        event.preventDefault();
        dialog.showErrorBox('Flashing in progress', 'Keep AutoRoot open and USB connected. Wait until the flash job completes.');
        return;
    }
    goProcess?.kill();
});
