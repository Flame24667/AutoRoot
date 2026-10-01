const { contextBridge, ipcRenderer } = require('electron');

try {
    contextBridge.exposeInMainWorld('goAPI', {
        call: (action, payload = {}) => ipcRenderer.invoke('go:invoke', { action, payload })
    });

    contextBridge.exposeInMainWorld('electronAPI', {
        selectFirmware: () => ipcRenderer.invoke('select-firmware-file'),
        checkBridge: () => ({ goAPI: !!window.goAPI, electronAPI: !!window.electronAPI })
    });

    console.log('[preload] bridges injected');
} catch (err) {
    console.error('[preload] Failed to inject bridges:', err);
}