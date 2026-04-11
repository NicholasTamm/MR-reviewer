interface ElectronAPI {
  getBackendPort: () => Promise<number>;
  getAuthToken: () => Promise<string>;
  setCredential: (key: string, value: string) => Promise<void>;
  getCredential: (key: string) => Promise<string | null>;
  deleteCredential: (key: string) => Promise<boolean>;
}

interface Window {
  electronAPI?: ElectronAPI;
}
