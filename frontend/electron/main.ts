import { app, BrowserWindow, dialog, ipcMain, shell } from "electron";
import { ChildProcess, spawn } from "child_process";
import { randomBytes } from "crypto";
import * as http from "http";
import * as net from "net";
import * as path from "path";
import { fileURLToPath } from "url";

let backendProcess: ChildProcess | null = null;
let backendPort = 0;
let mainWindow: BrowserWindow | null = null;
let isQuitting = false;
const authToken = randomBytes(32).toString("hex");
const moduleDir = path.dirname(fileURLToPath(import.meta.url));

function findFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.unref();
    server.on("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      server.close(() => {
        if (typeof address === "object" && address) resolve(address.port);
        else reject(new Error("Unable to determine backend port"));
      });
    });
  });
}

function waitForBackend(port: number, timeoutMs = 30_000): Promise<void> {
  const startedAt = Date.now();
  return new Promise((resolve, reject) => {
    const check = () => {
      const request = http.get(`http://127.0.0.1:${port}/api/health`, (response) => {
        response.resume();
        if (response.statusCode === 200) {
          resolve();
        } else {
          retry();
        }
      });
      request.on("error", retry);
      request.setTimeout(1_000, () => request.destroy());
    };
    const retry = () => {
      if (Date.now() - startedAt >= timeoutMs) {
        reject(new Error("The local review backend did not start within 30 seconds."));
      } else {
        setTimeout(check, 500);
      }
    };
    check();
  });
}

async function ensureBackend(): Promise<void> {
  if (backendProcess && backendPort) return;

  backendPort = await findFreePort();
  backendProcess = spawn(
    "python",
    ["-m", "mr_reviewer", "--serve", "--host", "127.0.0.1", "--port", String(backendPort)],
    {
      cwd: path.resolve(moduleDir, "../.."),
      env: { ...process.env, MR_REVIEWER_TOKEN: authToken },
      stdio: "pipe",
    },
  );
  backendProcess.stderr?.on("data", (data: Buffer) => process.stderr.write(`[backend] ${data}`));
  backendProcess.on("exit", (code) => {
    backendProcess = null;
    backendPort = 0;
    if (!isQuitting) {
      dialog.showErrorBox("Review backend stopped", `The local review backend exited (code ${code ?? "unknown"}).`);
    }
  });

  await waitForBackend(backendPort);
}

async function createWindow(): Promise<void> {
  await ensureBackend();

  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 900,
    minHeight: 600,
    webPreferences: {
      preload: path.join(moduleDir, "preload.mjs"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });
  mainWindow.on("closed", () => {
    mainWindow = null;
  });
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (url.startsWith("https://")) void shell.openExternal(url);
    return { action: "deny" };
  });
  mainWindow.webContents.on("will-navigate", (event, url) => {
    if (url.startsWith("https://")) void shell.openExternal(url);
    event.preventDefault();
  });

  const devServerUrl = process.env.VITE_DEV_SERVER_URL;
  if (devServerUrl) {
    await mainWindow.loadURL(devServerUrl);
  } else {
    await mainWindow.loadFile(path.join(moduleDir, "../dist/index.html"));
  }
}

process.on("uncaughtException", (error) => {
  dialog.showErrorBox("MR Reviewer error", error.message);
});

app.whenReady().then(createWindow).catch((error: Error) => {
  dialog.showErrorBox("Unable to start MR Reviewer", error.message);
  app.quit();
});

ipcMain.handle("get-backend-port", () => backendPort);
ipcMain.handle("get-auth-token", () => authToken);

app.on("activate", () => {
  if (!mainWindow) void createWindow();
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});

app.on("before-quit", () => {
  isQuitting = true;
  backendProcess?.kill("SIGTERM");
});

export function getBackendPort(): number {
  return backendPort;
}

export function getAuthToken(): string {
  return authToken;
}
