import { test, expect } from '@playwright/test';
import { startServer, type ServerInstance } from './helpers';

let server: ServerInstance;

test.beforeAll(async () => {
  server = await startServer();
});

test.afterAll(async () => {
  server.stop();
});

test('server responds to public API', async ({ request }) => {
  const res = await request.get(`${server.url}/api/posts`);
  expect(res.status()).toBe(200);
});

test('server serves static files', async ({ request }) => {
  const res = await request.get(`${server.url}/`);
  expect(res.status()).toBe(200);
});
