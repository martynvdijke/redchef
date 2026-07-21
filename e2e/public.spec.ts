import { test, expect } from '@playwright/test';
import { startServer, type ServerInstance } from './helpers';

let server: ServerInstance;

test.beforeAll(async () => {
  server = await startServer();
});

test.afterAll(async () => {
  server.stop();
});

test('list posts returns empty array initially', async ({ request }) => {
  const res = await request.get(`${server.url}/api/posts`);
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(Array.isArray(body)).toBe(true);
  expect(body.length).toBe(0);
});

test('invalid type filter returns 400', async ({ request }) => {
  const res = await request.get(`${server.url}/api/posts?type=gif`);
  expect(res.status()).toBe(400);
  const body = await res.json();
  expect(body.error).toBeTruthy();
});

test('get nonexistent post returns 404', async ({ request }) => {
  const res = await request.get(`${server.url}/api/posts/99999`);
  expect(res.status()).toBe(404);
});

test('get post with invalid id returns 400', async ({ request }) => {
  const res = await request.get(`${server.url}/api/posts/abc`);
  expect(res.status()).toBe(400);
});

test('analytics settings endpoint returns default', async ({ request }) => {
  const res = await request.get(`${server.url}/api/settings/analytics`);
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(body.tracking_enabled).toBe(false);
});
