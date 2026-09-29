const { contextBridge, ipcRenderer, webUtils } = require('electron');

try {
    contextBridge.exposeInMainWorld('goAPI', {
        call: (action, payload) => ipcRenderer.invoke('go:invoke', action, payload),

        // Used for drag-and-drop of an already-opened file.
        getFilePath: (file) => webUtils.getPathForFile(file),

        // Opens a native picker and resolves to the chosen path, or null when
        // the operator cancels. The dialog is limited to .zip so a package is
        // always selected deliberately rather than typed by hand.
        pickFirmwareFile: () => ipcRenderer.invoke('dialog:pickFirmware'),
    });
    console.log('[preload] goAPI injected');
} catch (err) {
    console.error('[preload] Failed to inject goAPI:', err);
}
