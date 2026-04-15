import { create } from 'zustand';

// USB Device Connection State
export const useDeviceStore = create((set) => ({
  connected: false,
  device: null,
  vendorId: null,
  productId: null,
  model: null,
  protocol: null, // 'odin', 'fastboot', 'mtk', 'edl'
  
  setDevice: (deviceInfo) => set({
    connected: true,
    device: deviceInfo.device,
    vendorId: deviceInfo.vendorId,
    productId: deviceInfo.productId,
    model: deviceInfo.model,
    protocol: deviceInfo.protocol
  }),
  
  clearDevice: () => set({
    connected: false,
    device: null,
    vendorId: null,
    productId: null,
    model: null,
    protocol: null
  })
}));

// Flashing Process State
export const useFlashStore = create((set) => ({
  isFlashing: false,
  progress: 0,
  status: 'idle', // 'idle', 'preparing', 'flashing', 'verifying', 'completed', 'failed'
  logs: [],
  error: null,
  
  startFlash: () => set({ 
    isFlashing: true, 
    progress: 0, 
    status: 'preparing', 
    logs: [], 
    error: null 
  }),
  
  updateProgress: (progress, status, logMessage) => set((state) => ({
    progress,
    status,
    logs: [...state.logs, `[${new Date().toLocaleTimeString()}] ${logMessage}`]
  })),
  
  completeFlash: (success, errorMessage) => set({
    isFlashing: false,
    status: success ? 'completed' : 'failed',
    error: errorMessage
  }),
  
  addLog: (message) => set((state) => ({
    logs: [...state.logs, `[${new Date().toLocaleTimeString()}] ${message}`]
  })),
  
  clearLogs: () => set({ logs: [] })
}));

// Asset Warehouse State
export const useAssetStore = create((set) => ({
  assets: [],
  indexed: false,
  lastIndexed: null,
  
  setAssets: (assets) => set({
    assets,
    indexed: true,
    lastIndexed: new Date()
  }),
  
  addAsset: (asset) => set((state) => ({
    assets: [...state.assets, asset]
  })),
  
  findAssetForModel: (model) => {
    const state = useAssetStore.getState();
    return state.assets.find(a => 
      a.compatibleModels?.includes(model) || 
      a.model === model
    );
  }
}));

// Driver Health State
export const useDriverStore = create((set) => ({
  driversInstalled: false,
  missingDrivers: [],
  checked: false,
  
  setDriverStatus: (installed, missing) => set({
    driversInstalled: installed,
    missingDrivers: missing,
    checked: true
  })
}));
