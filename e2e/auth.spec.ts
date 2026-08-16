import { test, expect } from '@playwright/test';
import { startServer, registerAndLogin, type ServerInstance } from './helpers';

let server: ServerInstance;

test.beforeAll(async () => {
  server = await startServer();
});

test.afterAll(async () => {
  server.stop();
});

test('register a new user', async ({ request }) => {
  const email = `register-${Date.now()}@test.com`;
  const res = await request.post(`${server.url}/api/auth/register`, {
    data: { email, password: 'test1234' },
  });
  expect(res.status()).toBe(201);
});

test('register with existing email returns 409', async ({ request }) => {
  const email = `dup-${Date.now()}@test.com`;
  await request.post(`${server.url}/api/auth/register`, {
    data: { email, password: 'test1234' },
  });
  const res = await request.post(`${server.url}/api/auth/register`, {
    data: { email, password: 'test1234' },
  });
  expect(res.status()).toBe(409);
});

test('register with invalid email returns 400', async ({ request }) => {
  const res = await request.post(`${server.url}/api/auth/register`, {
    data: { email: '', password: 'test1234' },
  });
  expect(res.status()).toBe(400);
});

test('login with correct credentials', async ({ request }) => {
  const email = `login-ok-${Date.now()}-${Math.random().toString(36).slice(2, 6)}@test.com`;
  const regRes = await request.post(`${server.url}/api/auth/register`, {
    data: { email, password: 'test1234' },
  });
  expect(regRes.status()).toBe(201);

  // Login (with retry for SQLite WAL write-read race)
  let res = await request.post(`${server.url}/api/auth/login`, {
    data: { email, password: 'test1234' },
  });
  if (res.status() !== 200) {
    await new Promise(r => setTimeout(r, 100));
    res = await request.post(`${server.url}/api/auth/login`, {
      data: { email, password: 'test1234' },
    });
  }
  expect(res.status()).toBe(200);
  // Should set a session cookie
  const cookies = res.headers()['set-cookie'];
  expect(cookies).toBeTruthy();
  expect(cookies).toContain('session_token');
});

test('login with wrong password returns 401', async ({ request }) => {
  const email = `login-wrong-${Date.now()}@test.com`;
  await request.post(`${server.url}/api/auth/register`, {
    data: { email, password: 'test1234' },
  });
  const res = await request.post(`${server.url}/api/auth/login`, {
    data: { email, password: 'wrongpass' },
  });
  expect(res.status()).toBe(401);
});

test('me endpoint returns authenticated=true when logged in', async ({ request }) => {
  const email = `me-auth-${Date.now()}@test.com`;
  const { cookies } = await registerAndLogin(request, server.url, email, 'test1234');

  const res = await request.get(`${server.url}/api/auth/me`, {
    headers: { Cookie: `${cookies[0].name}=${cookies[0].value}` },
  });
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(body.authenticated).toBe(true);
});

test('me endpoint returns authenticated=false when not logged in', async ({ request }) => {
  const res = await request.get(`${server.url}/api/auth/me`);
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(body.authenticated).toBe(false);
});

test('logout clears session', async ({ request }) => {
  const email = `logout-${Date.now()}@test.com`;
  const { cookies } = await registerAndLogin(request, server.url, email, 'test1234');

  // Logout
  const logoutRes = await request.post(`${server.url}/api/auth/logout`, {
    headers: { Cookie: `${cookies[0].name}=${cookies[0].value}` },
  });
  expect(logoutRes.status()).toBe(200);

  // Verify session is cleared
  const meRes = await request.get(`${server.url}/api/auth/me`, {
    headers: { Cookie: `${cookies[0].name}=${cookies[0].value}` },
  });
  const body = await meRes.json();
  expect(body.authenticated).toBe(false);
});

test('forgot password flow shows confirmation', async ({ page }) => {
  await page.goto(server.url + '/');
  await page.click('#btn-login');
  await page.click('#switch-to-forgot');

  // Forgot modal replaces the login modal
  await expect(page.locator('#forgot-modal')).toBeVisible();

  await page.fill('#forgot-email', `nobody-${Date.now()}@test.com`);
  await page.click('#forgot-form button[type="submit"]');

  // Always shows the generic success message (no account enumeration)
  await expect(page.locator('#forgot-form .auth-success')).toBeVisible();
});

test('reset password page rejects invalid token', async ({ page }) => {
  await page.goto(server.url + '/reset?token=bogus-token');

  // Reset modal opens automatically from the ?token= query param
  await expect(page.locator('#reset-modal')).toBeVisible();

  await page.fill('#reset-password', 'newpass123');
  await page.fill('#reset-confirm', 'newpass123');
  await page.click('#reset-form button[type="submit"]');

  await expect(page.locator('#reset-error')).toContainText('invalid or expired');
});
