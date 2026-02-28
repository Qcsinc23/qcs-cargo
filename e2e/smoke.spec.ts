import { test, expect } from '@playwright/test';

test.describe.configure({ mode: 'serial' });

test('home page loads', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/QCS/);
});

test('health API', async ({ request }) => {
  const r = await request.get('/api/v1/health');
  expect(r.ok()).toBeTruthy();
});

test('login page', async ({ page }) => {
  await page.goto('/login');
  await expect(page.locator('h1').first()).toBeVisible();
});

test('auth + public API contracts', async ({ request }) => {
  const nonce = Date.now();
  const email = `e2e-user-${nonce}@example.com`;

  const register = await request.post('/api/v1/auth/register', {
    data: {
      name: 'E2E User',
      email,
      phone: '+15551234567',
      password: 'StrongPass1!',
    },
  });
  expect(register.status()).toBe(201);
  const registerBody = await register.json();
  const registerMessage = registerBody?.data?.message ?? '';
  const registeredEmail = registerBody?.data?.user?.email ?? registerBody?.data?.email ?? '';
  expect(registerMessage.includes('Account created') || registeredEmail === email).toBeTruthy();

  const resend = await request.post('/api/v1/auth/resend-verification', {
    data: { email },
  });
  expect(resend.status()).toBe(200);

  const verifyInvalid = await request.post('/api/v1/auth/verify-email', {
    data: { token: 'invalid-token' },
  });
  expect(verifyInvalid.status()).toBe(400);

  const magicRequest = await request.post('/api/v1/auth/magic-link/request', {
    data: { email },
  });
  expect(magicRequest.status()).toBe(200);

  const magicVerifyInvalid = await request.post('/api/v1/auth/magic-link/verify', {
    data: { token: 'invalid-token' },
  });
  expect(magicVerifyInvalid.status()).toBe(401);

  const calculator = await request.get('/api/v1/calculator?dest=guyana&weight=5');
  expect(calculator.status()).toBe(200);

  const trackUnknown = await request.get('/api/v1/track/NOT-FOUND-E2E');
  expect(trackUnknown.status()).toBe(404);
});

test('logout returns 204 no content', async ({ request }) => {
  const logout = await request.post('/api/v1/auth/logout');
  expect(logout.status()).toBe(204);
  const body = await logout.text();
  expect(body).toBe('');
});

test('booking + ship request routes require authentication', async ({ request }) => {
  const bookingList = await request.get('/api/v1/bookings');
  expect(bookingList.status()).toBe(401);

  const shipRequests = await request.get('/api/v1/ship-requests');
  expect(shipRequests.status()).toBe(401);
});
