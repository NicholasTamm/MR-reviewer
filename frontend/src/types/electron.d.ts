interface ElectronAPI {
  getBackendPort: () => Promise<number>;
  getAuthToken: () => Promise<string>;
}

interface Window {
  electronAPI?: ElectronAPI;
}
