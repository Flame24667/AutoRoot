import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'

ReactDOM.createRoot(document.getElementById('root')).render(
    <React.StrictMode>
        <App />
    </React.StrictMode>,
)

ipcMain.handle('go:invoke', async (event, { action, payload }) => {
  console.log('🔌 IPC Received:', action, payload);
  try {
    // Your Go bridge call here
    const result = await goBridge.invoke(action, payload);
    return result;
  } catch (err) {
    console.error('❌ Go invoke failed:', err);
    throw err;
  }
});