import { test, expect } from '@playwright/test';
import { startServer, registerAndLogin, type ServerInstance } from './helpers';

let server: ServerInstance;

test.beforeAll(async () => {
  server = await startServer();
});

test.afterAll(async () => {
  server.stop();
});

test('list comments for nonexistent post returns empty array', async ({ request }) => {
  const res = await request.get(`${server.url}/api/posts/99999/comments`);
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(Array.isArray(body)).toBe(true);
});

test('create comment requires authentication', async ({ request }) => {
  const res = await request.post(`${server.url}/api/posts/1/comments`, {
    data: { body: 'Nice post!' },
  });
  expect(res.status()).toBe(401);
});

test('create comment on nonexistent post returns 404', async ({ request }) => {
  const email = `comment-${Date.now()}@test.com`;
  const { cookies } = await registerAndLogin(request, server.url, email, 'test1234');

  const res = await request.post(`${server.url}/api/posts/99999/comments`, {
    data: { body: 'Nice post!' },
    headers: { Cookie: `${cookies[0].name}=${cookies[0].value}` },
  });
  expect(res.status()).toBe(404);
});

test('invalid post id returns 400', async ({ request }) => {
  const res = await request.get(`${server.url}/api/posts/abc/comments`);
  expect(res.status()).toBe(400);
});
