const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');

let mainWindow;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.js')
    },
    icon: path.join(__dirname, '../public/favicon.svg')
  });

  // Load from Vite dev server or built files
  const isDev = process.env.NODE_ENV === 'development' || !app.isPackaged;
  
  if (isDev) {
    mainWindow.loadURL('http://localhost:5173');
    mainWindow.webContents.openDevTools();
  } else {
    mainWindow.loadFile(path.join(__dirname, '../dist/index.html'));
  }

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

app.whenReady().then(() => {
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

// IPC Handlers for USB Detection and Flashing
ipcMain.handle('get-device-info', async () => {
  // This will be implemented with actual USB detection logic
  return { connected: false, device: null };
});

ipcMain.handle('start-flash', async (event, config) => {
  // This will trigger the backend flashing process
  return { success: false, message: 'Flash not implemented yet' };
});

ipcMain.handle('check-drivers', async () => {
  // Check if required drivers are installed
  return { installed: true, missing: [] };
});

ipcMain.handle('get-assets', async () => {
  // Return list of available assets from local warehouse
  return [];
});
