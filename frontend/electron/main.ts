import { app, BrowserWindow, dialog, ipcMain, shell } from "electron";
import { ChildProcess, spawn } from "child_process";
import { randomBytes } from "crypto";
import { config as loadEnv } from "dotenv";
import * as http from "http";
import * as net from "net";
import * as path from "path";
import { fileURLToPath } from "url";
import { readFile, writeFile } from "fs/promises";
import { createRequire } from "module";

let backendProcess: ChildProcess | null = null;
let backendPort = 0;
let mainWindow: BrowserWindow | null = null;
let isQuitting = false;
const authToken = randomBytes(32).toString("hex");
const moduleDir = path.dirname(fileURLToPath(import.meta.url));
const keytar = createRequire(import.meta.url)("keytar") as typeof import("keytar");
const keytarService = "mr-reviewer";
const credentialKeys = new Set(["GITLAB_TOKEN", "GITHUB_TOKEN", "ANTHROPIC_API_KEY", "GEMINI_API_KEY"]);
const settingKeys = new Set(["MR_REVIEWER_PROVIDER", "MR_REVIEWER_MODEL", "MR_REVIEWER_FOCUS", "MR_REVIEWER_PARALLEL", "MR_REVIEWER_PARALLEL_THRESHOLD", "MR_REVIEWER_MAX_COMMENTS", "OLLAMA_HOST", "MR_REVIEWER_GITLAB_URL"]);
let runtimeSettings: Record<string, string> = {};

if (!app.isPackaged) {
  loadEnv({ path: path.resolve(moduleDir, "../../.env") });
}

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

async function loadRuntimeSettings(): Promise<void> {
  try { runtimeSettings = JSON.parse(await readFile(path.join(app.getPath("userData"), "settings.json"), "utf8")); }
  catch { runtimeSettings = {}; }
}

async function getBackendEnv(): Promise<NodeJS.ProcessEnv> {
  const entries = await Promise.all([...credentialKeys].map(async (key) => [key, await keytar.getPassword(keytarService, key)] as const));
  return { ...process.env, ...runtimeSettings, ...Object.fromEntries(entries.filter(([, value]) => value)), MR_REVIEWER_TOKEN: authToken };
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
  const executable = app.isPackaged
    ? path.join(process.resourcesPath, "backend", process.platform === "win32" ? "mr-reviewer-server.exe" : "mr-reviewer-server")
    : "python";
  const args = app.isPackaged
    ? ["--serve", "--host", "127.0.0.1", "--port", String(backendPort)]
    : ["-m", "mr_reviewer", "--serve", "--host", "127.0.0.1", "--port", String(backendPort), "--verbose"];
  backendProcess = spawn(executable, args, {
      cwd: app.isPackaged ? process.resourcesPath : path.resolve(moduleDir, "../.."),
      env: await getBackendEnv(),
      stdio: "pipe",
    });
  backendProcess.stderr?.on("data", (data: Buffer) => process.stderr.write(`[backend] ${data}`));
  backendProcess.once("error", (error) => dialog.showErrorBox("Unable to start backend", error.message));
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
      sandbox: true,
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

app.whenReady().then(async () => { await loadRuntimeSettings(); await createWindow(); }).catch((error: Error) => {
  dialog.showErrorBox("Unable to start MR Reviewer", error.message);
  app.quit();
});

ipcMain.handle("get-backend-port", () => backendPort);
ipcMain.handle("backend-request", (_event, requestPath: string, method = "GET", body = "") => new Promise<{ status: number; body: string }>((resolve, reject) => {
  if (!requestPath.startsWith("/api/")) { reject(new Error("Invalid backend path")); return; }
  const request = http.request({ hostname: "127.0.0.1", port: backendPort, path: requestPath, method, headers: { Authorization: `Bearer ${authToken}`, "Content-Type": "application/json" } }, (response) => {
    let responseBody = "";
    response.on("data", (chunk) => { responseBody += chunk; });
    response.on("end", () => resolve({ status: response.statusCode ?? 500, body: responseBody }));
  });
  request.on("error", reject);
  if (body) request.write(body);
  request.end();
}));
ipcMain.handle("get-runtime-settings", async () => ({ settings: runtimeSettings, credentials: Object.fromEntries(await Promise.all([...credentialKeys].map(async (key) => [key, Boolean(await keytar.getPassword(keytarService, key))]))) }));
ipcMain.handle("save-runtime-settings", async (_event, settings: Record<string, string>) => {
  for (const [key, value] of Object.entries(settings)) if (!settingKeys.has(key) || typeof value !== "string" || value.length > 4096) throw new Error("Invalid runtime setting");
  runtimeSettings = { ...runtimeSettings, ...settings };
  await writeFile(path.join(app.getPath("userData"), "settings.json"), JSON.stringify(runtimeSettings), "utf8");
  backendProcess?.kill("SIGTERM");
  backendProcess = null;
  backendPort = 0;
  await ensureBackend();
  mainWindow?.webContents.reload();
});
ipcMain.handle("save-credential", async (_event, key: string, value: string) => {
  if (!credentialKeys.has(key) || !value.trim()) throw new Error("Invalid credential");
  await keytar.setPassword(keytarService, key, value.trim());
  backendProcess?.kill("SIGTERM");
  backendProcess = null;
  backendPort = 0;
  await ensureBackend();
  mainWindow?.webContents.reload();
});

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
