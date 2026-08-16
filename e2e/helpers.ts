import { test as base, type Page, type APIRequestContext, type BrowserContext } from '@playwright/test';
import { execSync, spawn, type ChildProcess } from 'child_process';
import path from 'path';
import fs from 'fs';
import http from 'http';

const ROOT = path.resolve(__dirname, '..');
const BINARY = '/tmp/redchef-e2e';

export interface ServerInstance {
  process: ChildProcess;
  url: string;
  stop: () => void;
}

/**
 * Build the Go binary if it doesn't exist yet.
 */
export function ensureBinary(): void {
  if (!fs.existsSync(BINARY)) {
    // Build only the main package (module root) — `./...` matches multiple
    // packages and would require -o to be a directory.
    execSync('go build -o ' + BINARY + ' .', { cwd: ROOT, stdio: 'pipe' });
  }
}

/**
 * Start the RedChef server on a random available port.
 * Returns the server instance with url and stop function.
 */
export async function startServer(): Promise<ServerInstance> {
  ensureBinary();

  const dbPath = '/tmp/redchef-e2e.db';
  const uploadDir = '/tmp/redchef-e2e-media';

  // Clean up any previous state
  for (const f of [dbPath, dbPath + '-wal', dbPath + '-shm']) {
    try { fs.unlinkSync(f); } catch { /* ok */ }
  }
  try { fs.rmSync(uploadDir, { recursive: true, force: true }); } catch { /* ok */ }
  try { fs.mkdirSync(uploadDir, { recursive: true }); } catch { /* ok */ }

  const port = 8080;

  const proc = spawn(BINARY, [], {
    cwd: ROOT,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: {
      ...process.env,
      PORT: String(port),
      DB_PATH: dbPath,
      UPLOAD_DIR: uploadDir,
    },
  });

  const url = `http://localhost:${port}`;

  // Wait for server to be ready
  await waitForServer(url, 30000);

  const stop = () => {
    proc.kill('SIGTERM');
    // Clean up files after a brief delay
    setTimeout(() => {
      for (const f of [dbPath, dbPath + '-wal', dbPath + '-shm']) {
        try { fs.unlinkSync(f); } catch { /* ok */ }
      }
      try { fs.rmSync(uploadDir, { recursive: true, force: true }); } catch { /* ok */ }
    }, 100);
  };

  return { process: proc, url, stop };
}

function waitForServer(url: string, timeoutMs: number): Promise<void> {
  const start = Date.now();
  return new Promise((resolve, reject) => {
    function poll() {
      const req = http.get(url + '/api/settings/analytics', (res) => {
        if (res.statusCode === 200) {
          resolve();
        } else if (Date.now() - start > timeoutMs) {
          reject(new Error(`Server not ready after ${timeoutMs}ms (status ${res.statusCode})`));
        } else {
          setTimeout(poll, 200);
        }
      });
      req.on('error', () => {
        if (Date.now() - start > timeoutMs) {
          reject(new Error(`Server not reachable after ${timeoutMs}ms`));
        } else {
          setTimeout(poll, 200);
        }
      });
      req.end();
    }
    poll();
  });
}

/**
 * Create a new user via the API and return the session cookie.
 * Also returns the full APIRequestContext with the cookie set.
 */
export async function registerAndLogin(
  request: APIRequestContext,
  baseURL: string,
  email: string,
  password: string,
): Promise<{ cookies: { name: string; value: string }[] }> {
  // Register
  const regRes = await request.post(`${baseURL}/api/auth/register`, {
    data: { email, password },
  });
  if (regRes.status() !== 201 && regRes.status() !== 409) {
    // 409 means already exists, that's ok for shared-state tests
    throw new Error(`Register failed: ${regRes.status()} ${await regRes.text()}`);
  }

  // Login (with retry for SQLite WAL write-read race)
  let loginRes = await request.post(`${baseURL}/api/auth/login`, {
    data: { email, password },
  });
  if (loginRes.status() !== 200) {
    // Short delay to let WAL checkpoint propagate
    await new Promise(r => setTimeout(r, 100));
    loginRes = await request.post(`${baseURL}/api/auth/login`, {
      data: { email, password },
    });
  }
  if (loginRes.status() !== 200) {
    throw new Error(`Login failed: ${loginRes.status()} ${await loginRes.text()}`);
  }

  const cookies = loginRes.headers()['set-cookie'];
  if (!cookies) {
    // Extract from headers array
    const allHeaders = loginRes.headersArray();
    const cookieHeaders = allHeaders.filter(h => h.name.toLowerCase() === 'set-cookie');
    const sessionCookies = cookieHeaders.map(h => {
      const parts = h.value.split(';')[0]; // session_token=xxx
      const [name, ...valParts] = parts.split('=');
      return { name, value: valParts.join('=') };
    });
    return { cookies: sessionCookies };
  }

  const parts = cookies.split(';')[0];
  const [name, ...valParts] = parts.split('=');
  return { cookies: [{ name, value: valParts.join('=') }] };
}
