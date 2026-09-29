const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');
const fs = require('fs');
const { downloadFile } = require('./setup.js'); // Reuse download logic

function createWindow() {
  const win = new BrowserWindow({
    width: 800,
    height: 600,
    title: 'AutoRoot Setup',
    webPreferences: {
      nodeIntegration: true,
      contextIsolation: false,
    },
  });
  
  win.loadFile(path.join(__dirname, 'setup.html'));
}

app.whenReady().then(createWindow);