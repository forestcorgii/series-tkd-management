import { test, expect } from '@playwright/test';
import { loginAs } from './helpers/auth';

test.describe('Student Directory & Belt Progression Suite', () => {
  test.beforeEach(async ({ page }) => {
    // Operation Manager has full access to student management
    await loginAs(page, 'manager');
  });

  test('should display student directory and search students via HTMX', async ({ page }) => {
    await page.goto('/students');

    await expect(page.locator('#student-rows')).toBeVisible();
    await expect(page.locator('#student-rows').getByRole('link', { name: 'Alex Vance' })).toBeVisible();
    await expect(page.locator('#student-rows').getByRole('link', { name: 'Chloe Ramirez' })).toBeVisible();

    // Search query via HTMX
    const searchInput = page.locator('input[name="query"]');
    await searchInput.click();
    await searchInput.pressSequentially('Alex', { delay: 50 });

    // Wait for filtered results
    await expect(page.locator('#student-rows')).toContainText('Alex Vance');
  });

  test('should navigate to student profile and display belt progression details', async ({ page }) => {
    const studentID = 'b1111111-1111-1111-1111-111111111111';
    await page.goto(`/students/${studentID}`);

    // Verify student header and details
    await expect(page.locator('h1')).toContainText('Alex Vance');
    await expect(page.locator('text=Belt: White')).toBeVisible();

    // Verify readiness promotion badge is rendered
    const readinessBadge = page.locator('.badge-ready, .badge-pretest, .badge-developing');
    await expect(readinessBadge).toBeVisible();

    // Verify medical notes in profile banner
    await expect(page.getByText('🩺 Mild asthma, uses inhaler')).toBeVisible();
  });

  test('should allow student user to view their own profile but restrict administrative actions', async ({ page }) => {
    // Switch to Student login
    await loginAs(page, 'student');
    const studentID = 'b1111111-1111-1111-1111-111111111111';
    await page.goto(`/students/${studentID}`);

    await expect(page.locator('h1')).toContainText('Alex Vance');

    // Edit and Delete administrative buttons should not be present for students
    await expect(page.locator('button:has-text("Delete")')).not.toBeVisible();
    await expect(page.locator('button:has-text("Edit")')).not.toBeVisible();
  });
});
