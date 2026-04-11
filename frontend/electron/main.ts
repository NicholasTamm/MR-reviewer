import { app, BrowserWindow, ipcMain } from 'electron';
import { spawn, ChildProcess } from 'child_process';
import * as path from 'path';
import * as net from 'net';
import * as http from 'http';
import { randomBytes } from 'crypto';
import keytar from 'keytar';

const authToken = randomBytes(32).toString('hex');
const KEYTAR_SERVICE = 'mr-reviewer';

let backendProcess: ChildProcess | null = null;
let backendPort = 8080;

function findFreePort(start: number): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.listen(start, '127.0.0.1', () => {
      const addr = server.address() as net.AddressInfo;
      server.close(() => resolve(addr.port));
    });
    server.on('error', () => {
      // Port in use — try next
      findFreePort(start + 1).then(resolve).catch(reject);
    });
  });
}

function waitForBackend(port: number, timeoutMs = 30000): Promise<void> {
  const start = Date.now();
  return new Promise((resolve, reject) => {
    const check = () => {
      const req = http.get(`http://127.0.0.1:${port}/api/health`, (res) => {
        if (res.statusCode === 200) {
          resolve();
        } else {
          retry();
        }
      });
      req.on('error', retry);
      req.end();
    };
    const retry = () => {
      if (Date.now() - start > timeoutMs) {
        reject(new Error(`Backend did not start within ${timeoutMs}ms`));
      } else {
        setTimeout(check, 500);
      }
    };
    check();
  });
}

function startBackend(port: number): ChildProcess {
  const env = { ...process.env, MR_REVIEWER_TOKEN: authToken };
  if (app.isPackaged) {
    const binaryName = process.platform === 'win32'
      ? 'mr-reviewer-server.exe'
      : 'mr-reviewer-server';
    const binaryPath = path.join(process.resourcesPath, 'backend', binaryName);
    return spawn(binaryPath, ['--serve', '--port', String(port)], { stdio: 'pipe', env });
  } else {
    // Dev mode: use system Python
    const repoRoot = path.join(__dirname, '../../');
    return spawn('python', ['-m', 'mr_reviewer', '--serve', '--port', String(port)], {
      stdio: 'pipe',
      cwd: repoRoot,
      env,
    });
  }
}

async function createWindow(): Promise<void> {
  backendPort = await findFreePort(8080);
  backendProcess = startBackend(backendPort);

  backendProcess.stdout?.on('data', (d: Buffer) => process.stdout.write(`[backend] ${d}`));
  backendProcess.stderr?.on('data', (d: Buffer) => process.stderr.write(`[backend] ${d}`));

  await waitForBackend(backendPort);

  const preloadPath = path.join(__dirname, 'preload.js');

  const win = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 900,
    minHeight: 600,
    webPreferences: {
      preload: preloadPath,
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  const devServerUrl = process.env['VITE_DEV_SERVER_URL'];
  if (!app.isPackaged && devServerUrl) {
    win.loadURL(devServerUrl);
  } else {
    win.loadFile(path.join(__dirname, '../dist/index.html'));
  }
}

ipcMain.handle('get-backend-port', () => backendPort);
ipcMain.handle('get-auth-token', () => authToken);

ipcMain.handle('keytar-set', async (_event, key: string, value: string) => {
  await keytar.setPassword(KEYTAR_SERVICE, key, value);
});

ipcMain.handle('keytar-get', async (_event, key: string) => {
  return keytar.getPassword(KEYTAR_SERVICE, key);
});

ipcMain.handle('keytar-delete', async (_event, key: string) => {
  return keytar.deletePassword(KEYTAR_SERVICE, key);
});

app.whenReady().then(createWindow);

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit();
});

let cleanupDone = false;
function cleanup(): void {
  if (cleanupDone) return;
  cleanupDone = true;
  if (backendProcess) {
    backendProcess.kill('SIGTERM');
    setTimeout(() => backendProcess?.kill('SIGKILL'), 3000).unref();
  }
}

app.on('before-quit', cleanup);
app.on('will-quit', cleanup);
