import { test, expect } from '@playwright/test';

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
