interface ElectronAPI {
  getBackendPort: () => Promise<number>;
  requestBackend: (path: string, method?: string, body?: string) => Promise<{ status: number; body: string }>;
  getRuntimeSettings: () => Promise<{ settings: Record<string, string>; credentials: Record<string, boolean> }>;
  saveRuntimeSettings: (settings: Record<string, string>) => Promise<void>;
  saveCredential: (key: string, value: string) => Promise<void>;
}

interface Window {
  electronAPI?: ElectronAPI;
}
