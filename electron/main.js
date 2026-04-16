const { app, BrowserWindow, ipcMain } = require('electron');
const { spawn } = require('child_process');
const path = require('path');

let goProcess;
const pending = new Map();
let reqId = 0;

function startGo() {
    const ext = process.platform === 'win32' ? '.exe' : '';
    const goPath = path.join(__dirname, '..', 'bin', `myapp-go${ext}`);
    
    goProcess = spawn(goPath, { stdio: ['pipe', 'pipe', 'inherit'] });

    let buffer = '';
    goProcess.stdout.on('data', (chunk) => {
        buffer += chunk.toString();
        const lines = buffer.split('\n');
        buffer = lines.pop(); // keep incomplete line
        for (const line of lines) {
        if (!line.trim()) continue;
        const resp = JSON.parse(line);
        const p = pending.get(resp.id);
        if (p) {
            if (resp.error) p.reject(new Error(resp.error));
            else p.resolve(resp.result);
            pending.delete(resp.id);
        }
        }
    });

    goProcess.on('error', (err) => console.error('Go process failed:', err));
    goProcess.on('close', (code) => console.log('Go exited:', code));
}

ipcMain.handle('go:invoke', async (_evt, action, payload) => {
    return new Promise((resolve, reject) => {
        const id = `req_${++reqId}`;
        pending.set(id, { resolve, reject });
        const msg = JSON.stringify({ id, action, payload }) + '\n';
        goProcess.stdin.write(msg);
    });
});

app.whenReady().then(() => {
    startGo();
    const win = new BrowserWindow({ width: 1000, height: 700, webPreferences: { preload: path.join(__dirname, 'preload.js') } });
    win.loadURL('http://localhost:5173'); // dev, or load file in prod
});

app.on('before-quit', () => goProcess?.kill());