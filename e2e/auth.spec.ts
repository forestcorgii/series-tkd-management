import { test, expect } from '@playwright/test';
import { loginAs } from './helpers/auth';

test.describe('Authentication & RBAC Suite', () => {
  test('should render the login page with form inputs', async ({ page }) => {
    await page.goto('/login');
    await expect(page).toHaveTitle(/Sign In \/ Register/);
    await expect(page.locator('#identifier')).toBeVisible();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toBeVisible();
  });

  test('should show an error alert when credentials are invalid', async ({ page }) => {
    await page.goto('/login');
    await page.fill('#identifier', 'manager@seriestkd.com');
    await page.fill('#password', 'wrongpassword');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL(/\/login/);
    const alert = page.locator('text=Invalid email or password');
    await expect(alert).toBeVisible();
  });

  test('should authenticate Operation Manager and access dashboard', async ({ page }) => {
    await loginAs(page, 'manager');
    await page.goto('/dashboard');
    await expect(page).toHaveURL(/\/dashboard/);
    await expect(page).toHaveTitle(/Dashboard/i);
    await expect(page.locator('text=Promotion Readiness Pipeline')).toBeVisible();
  });

  test('should authenticate Coach and access coach portal', async ({ page }) => {
    await loginAs(page, 'coach');
    await page.goto('/portal/coach');
    await expect(page).toHaveURL(/\/portal\/coach/);
  });

  test('should authenticate Student and land on student portal', async ({ page }) => {
    await loginAs(page, 'student');
    await expect(page).toHaveURL(/\/portal\/student/);
  });

  test('should reject unauthenticated access to protected routes', async ({ page }) => {
    await page.goto('/dashboard');
    await expect(page).toHaveURL(/\/login/);
  });

  test('should log out successfully and redirect to login page', async ({ page }) => {
    await loginAs(page, 'manager');
    await page.goto('/logout');
    await expect(page).toHaveURL(/\/login/);
    await expect(page.locator('text=signed out successfully')).toBeVisible();

    // Verify session is invalidated
    await page.goto('/dashboard');
    await expect(page).toHaveURL(/\/login/);
  });
});
