import { test, expect } from '@playwright/test';
import { loginAs } from './helpers/auth';

test.describe('Floor Attendance & Real-Time Check-In Suite', () => {
  test.beforeEach(async ({ page }) => {
    // Floor check-in requires Coach, Admin, or Operation Manager
    await loginAs(page, 'coach');
  });

  test('should display live session header and floor check-in components', async ({ page }) => {
    const sessionID = 'c1111111-1111-1111-1111-111111111111';
    await page.goto(`/sessions/${sessionID}/live`);

    await expect(page.locator('h2', { hasText: 'Floor Attendance Search' })).toBeVisible();
    await expect(page.locator('#student-search-input')).toBeVisible();
    await expect(page.locator('#attendance-roster')).toBeVisible();
  });

  test('should search student via HTMX and admit to roster dynamically', async ({ page }) => {
    const sessionID = 'c1111111-1111-1111-1111-111111111111';
    await page.goto(`/sessions/${sessionID}/live`);

    // Search for student Chloe Ramirez
    const searchInput = page.locator('#student-search-input');
    await searchInput.click();
    await searchInput.pressSequentially('Chloe', { delay: 50 });

    // Locate Chloe Ramirez's search card specifically
    const chloeCard = page.locator('#search-results > div > div').filter({ hasText: 'Chloe Ramirez' });
    await expect(chloeCard).toBeVisible({ timeout: 5000 });

    // Admit Chloe Ramirez
    const admitButton = chloeCard.locator('button', { hasText: 'Admit Student' });
    await expect(admitButton).toBeVisible();
    await admitButton.click();

    // Verify student is dynamically swapped into attendance roster without page reload
    const rosterItem = page.locator('#attendance-roster').locator('text=Chloe Ramirez');
    await expect(rosterItem).toBeVisible();
  });

  test('should admit non-student guest attendee dynamically', async ({ page }) => {
    const sessionID = 'c1111111-1111-1111-1111-111111111111';
    await page.goto(`/sessions/${sessionID}/live`);

    const guestName = 'Grandmaster Park (VIP Guest)';
    await page.fill('input[name="attendee_name"]', guestName);
    await page.locator('form[hx-post*="checkin-attendee"] button[type="submit"]').click();

    // Verify guest appears in attendance roster
    const rosterItem = page.locator('#attendance-roster').locator(`text=${guestName}`);
    await expect(rosterItem).toBeVisible();
  });
});
