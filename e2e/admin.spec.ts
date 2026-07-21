import { test, expect } from '@playwright/test';
import { startServer, registerAndLogin, type ServerInstance } from './helpers';

let server: ServerInstance;

test.beforeAll(async () => {
  server = await startServer();
});

test.afterAll(async () => {
  server.stop();
});

test('unauthenticated user gets 401 for admin endpoints', async ({ request }) => {
  const res = await request.get(`${server.url}/api/admin/posts`);
  expect(res.status()).toBe(401);
});

test('non-admin user gets 401 for admin endpoints', async ({ request }) => {
  // First registered user is admin; register a second user who is "normal"
  const adminEmail = `admin-${Date.now()}@test.com`;
  await registerAndLogin(request, server.url, adminEmail, 'test1234');

  const normalEmail = `normal-${Date.now()}@test.com`;
  const { cookies } = await registerAndLogin(request, server.url, normalEmail, 'test1234');

  const res = await request.get(`${server.url}/api/admin/posts`, {
    headers: { Cookie: `${cookies[0].name}=${cookies[0].value}` },
  });
  // AdminAuth returns 401 for both unauthenticated and non-admin users
  expect(res.status()).toBe(401);
});

test('setup status endpoint returns needs_setup', async ({ request }) => {
  const res = await request.get(`${server.url}/api/setup/status`);
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(typeof body.needs_setup).toBe('boolean');
});
