import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("electronAPI", {
  getBackendPort: (): Promise<number> => ipcRenderer.invoke("get-backend-port"),
  requestBackend: (path: string, method?: string, body?: string) => ipcRenderer.invoke("backend-request", path, method, body),
  getRuntimeSettings: () => ipcRenderer.invoke("get-runtime-settings"),
  saveRuntimeSettings: (settings: Record<string, string>) => ipcRenderer.invoke("save-runtime-settings", settings),
  saveCredential: (key: string, value: string) => ipcRenderer.invoke("save-credential", key, value),
});
