import { test, expect } from '@playwright/test';
import { startServer, registerAndLogin, type ServerInstance } from './helpers';

let server: ServerInstance;

test.beforeAll(async () => {
  server = await startServer();
});

test.afterAll(async () => {
  server.stop();
});

test('unauthenticated pay unlock returns 401', async ({ request }) => {
  const res = await request.post(`${server.url}/api/pay/unlock`, {
    data: { bank: 'ING' },
  });
  expect(res.status()).toBe(401);
});

test('unauthenticated pay item returns 401', async ({ request }) => {
  const res = await request.post(`${server.url}/api/pay/item`, {
    data: { post_id: 1, bank: 'ING' },
  });
  expect(res.status()).toBe(401);
});

test('pay unlock works for authenticated non-paid user', async ({ request }) => {
  const email = `pay-${Date.now()}@test.com`;
  const { cookies } = await registerAndLogin(request, server.url, email, 'test1234');

  const res = await request.post(`${server.url}/api/pay/unlock`, {
    data: { bank: 'ING' },
    headers: { Cookie: `${cookies[0].name}=${cookies[0].value}` },
  });
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(body.ok).toBe(true);
  expect(body.paid).toBe(true);
});

test('pay item with missing post_id returns 400', async ({ request }) => {
  const email = `payitem-${Date.now()}@test.com`;
  const { cookies } = await registerAndLogin(request, server.url, email, 'test1234');

  const res = await request.post(`${server.url}/api/pay/item`, {
    data: { bank: 'ING' },
    headers: { Cookie: `${cookies[0].name}=${cookies[0].value}` },
  });
  expect(res.status()).toBe(400);
});
