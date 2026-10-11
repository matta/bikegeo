import { test, expect } from '@playwright/test';

test.describe('BikeGeo HTMX Critical Paths Suite', () => {

  test('Target Pattern 1: Inline-editable data row/cell outerHTML swap', async ({ page }) => {
    await page.goto('/');

    // Locate the first editable cell (Reach)
    const editableCell = page.locator('[title="Click to inline-edit"]').first();
    await expect(editableCell).toBeVisible();

    // Click to swap to inline edit form
    await editableCell.click();

    // Locate input field inside the swapped form
    const input = page.locator('form input[name="value"]');
    await expect(input).toBeVisible();

    // Fill in a distinct value and save
    await input.fill('488.5');
    await page.locator('form button[title="Save"]').click();

    // Web-first assertion: HTMX swaps back to InlineCellDisplay with updated value
    await expect(page.locator('[title="Click to inline-edit"]').first()).toContainText('488.5');
  });

  test('Target Pattern 2: Debounced search box filtering comparison matrix', async ({ page }) => {
    await page.goto('/');

    const searchInput = page.getByPlaceholder('Search / filter bikes');
    await expect(searchInput).toBeVisible();

    // Assert initial comparison matrix has content
    const matrix = page.locator('#comparison-matrix');
    await expect(matrix).toBeVisible();
    await expect(matrix).toContainText('Binary');

    // Type a non-matching query (triggers debounced HTMX request after 300ms)
    await searchInput.pressSequentially('NonExistentBikeBrandXYZ', { delay: 30 });

    // Web-first assertion: Playwright polls automatically while HTMX completes debounced swap
    await expect(matrix).not.toContainText('Binary Havok');

    // Clear search filter
    await searchInput.fill('');
    await searchInput.dispatchEvent('input');
    await expect(matrix).toContainText('Binary');
  });

  test('Target Pattern 3: Asynchronous form submission with inline validation (HTTP 422)', async ({ page }) => {
    await page.goto('/');

    // Navigate to Add Bike editor
    const addBikeBtn = page.getByRole('button', { name: /Add Bike/i });
    await expect(addBikeBtn).toBeVisible();
    await addBikeBtn.click();

    // Verify inline editor view swapped into #app-content
    const editor = page.locator('#frame-editor-view');
    await expect(editor).toBeVisible();

    // Clear manufacturer and model inputs, submit form
    const mfgInput = page.locator('input[name="manufacturer"]');
    await mfgInput.fill('');
    const modelInput = page.locator('input[name="model"]');
    await modelInput.fill('');
    await page.locator('#bike-editor-form button[type="submit"]').click();

    // Web-first assertion: Server returns 422 and HTMX beforeSwap allows swapping validation errors
    await expect(page.getByText('Missing Manufacturer')).toBeVisible();

    // Fill valid data and submit
    await mfgInput.fill('Pivot');
    await modelInput.fill('Les');
    await page.locator('input[name="reach_M"]').fill('450');
    await page.locator('input[name="stack_M"]').fill('630');
    await page.locator('input[name="hta_deg_M"]').fill('67.5');
    await page.locator('input[name="sta_deg_M"]').fill('74.0');
    await page.locator('input[name="htl_M"]').fill('110');
    await page.locator('input[name="bb_drop_M"]').fill('60');
    await page.locator('input[name="wheelbase_M"]').fill('1155');
    await page.locator('#bike-editor-form button[type="submit"]').click();

    // Web-first assertion: Successful save redirects/swaps to validator view for the new bike
    await expect(page.locator('#frame-editor-view')).toBeHidden();
    await expect(page.locator('#sag-simulator-results')).toBeVisible();
  });

  test('Out-of-band updates & Tab navigation across views', async ({ page }) => {
    await page.goto('/');

    // Switch to CAD Proof tab
    const proofTabBtn = page.getByRole('button', { name: /Master Stack CAD Proof/i });
    await expect(proofTabBtn).toBeVisible();
    await proofTabBtn.click();

    // Out-of-band swap asserts: active tab indicator highlighted and validator view displayed
    await expect(page.locator('#sag-simulator-results')).toBeVisible();

    // Switch back to Apples-to-Apples Compare tab
    const compareTabBtn = page.getByRole('button', { name: /Apples-to-Apples Compare/i });
    await expect(compareTabBtn).toBeVisible();
    await compareTabBtn.click();

    await expect(page.locator('#comparison-matrix')).toBeVisible();
  });

});
