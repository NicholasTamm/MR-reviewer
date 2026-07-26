import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("electronAPI", {
  getBackendPort: (): Promise<number> => ipcRenderer.invoke("get-backend-port"),
  getAuthToken: (): Promise<string> => ipcRenderer.invoke("get-auth-token"),
});
