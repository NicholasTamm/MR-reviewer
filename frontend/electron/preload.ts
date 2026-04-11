import { contextBridge, ipcRenderer } from 'electron';

contextBridge.exposeInMainWorld('electronAPI', {
  getBackendPort: (): Promise<number> => ipcRenderer.invoke('get-backend-port'),
  getAuthToken: (): Promise<string> => ipcRenderer.invoke('get-auth-token'),
  setCredential: (key: string, value: string): Promise<void> =>
    ipcRenderer.invoke('keytar-set', key, value),
  getCredential: (key: string): Promise<string | null> =>
    ipcRenderer.invoke('keytar-get', key),
  deleteCredential: (key: string): Promise<boolean> =>
    ipcRenderer.invoke('keytar-delete', key),
});
