import './App.css'
import { DeviceMonitor, DeviceStatus, DriverHealth } from './components/DeviceMonitor'
import { FlashControl } from './components/FlashControl'
import { AssetImporter, AssetList } from './components/AssetImporter'

function App() {
  return (
    <div className="app-container">
      <header className="app-header">
        <h1>🔓 AutoRoot Desktop</h1>
        <p className="subtitle">Universal Offline-First Rooting Solution v1.0</p>
      </header>

      {/* Hidden device monitor for real-time USB detection */}
      <DeviceMonitor />

      <main className="app-main">
        <section className="sidebar">
          <DeviceStatus />
          <DriverHealth />
          
          <div className="quick-info">
            <h3>Quick Info</h3>
            <ul>
              <li>✓ 100% Offline Operation</li>
              <li>✓ Auto-Detect Device</li>
              <li>✓ Multi-Protocol Support</li>
              <li>✓ Samsung Odin/Loke</li>
              <li>✓ Fastboot (Xiaomi, Pixel)</li>
              <li>✓ MTK/EDL Mode</li>
            </ul>
          </div>
        </section>

        <section className="main-content">
          <FlashControl />
          
          <div className="warehouse-section">
            <AssetImporter />
            <AssetList />
          </div>
        </section>
      </main>

      <footer className="app-footer">
        <p>AutoRoot Desktop - For educational purposes only. Use at your own risk.</p>
      </footer>
    </div>
  )
}

export default App
