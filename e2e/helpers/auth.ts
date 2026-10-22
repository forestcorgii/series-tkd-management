import { Page, expect } from '@playwright/test';

export const TEST_USERS = {
  manager: {
    identifier: 'manager@seriestkd.com',
    password: 'manager123',
  },
  admin: {
    identifier: 'admin@seriestkd.com',
    password: 'admin123',
  },
  coach: {
    identifier: 'jiwoo.park@seriestkd.com',
    password: 'coach123',
  },
  student: {
    identifier: 'alex.vance@seriestkd.com',
    password: 'student123',
  },
};

export async function loginAs(page: Page, role: keyof typeof TEST_USERS) {
  const user = TEST_USERS[role];
  // Clear any existing session so /login form always renders
  await page.context().clearCookies();
  await page.goto('/login');
  await page.fill('#identifier', user.identifier);
  await page.fill('#password', user.password);
  await page.click('button[type="submit"]');
  // Wait for authentication redirect away from /login
  await page.waitForURL((url) => !url.pathname.includes('/login'));
  await expect(page).not.toHaveURL(/\/login/);
}
