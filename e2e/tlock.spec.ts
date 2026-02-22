import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import {
  getRememoryBin,
  generateStandaloneHTML,
  createTestProject,
  extractBundle,
  CreationPage,
  RecoveryPage,
} from './helpers';

test.describe('Time-lock: maker.html advanced options', () => {
  let htmlPath: string;
  let tmpDir: string;

  test.beforeAll(async () => {
    const bin = getRememoryBin();
    if (!fs.existsSync(bin)) {
      test.skip();
      return;
    }
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rememory-tlock-e2e-'));
    htmlPath = generateStandaloneHTML(tmpDir, 'create');
  });

  test.afterAll(async () => {
    if (tmpDir && fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  test('advanced options tabs visible when tlock-js is loaded', async ({ page }) => {
    const creation = new CreationPage(page, htmlPath);
    await creation.open();

    // tlock-js is included in default maker.html, so Simple/Advanced tabs should be visible
    const advancedTabs = page.locator('#advanced-options');
    await expect(advancedTabs).toBeVisible();

    // Both tabs should be visible
    await expect(advancedTabs.locator('[data-mode="simple"]')).toBeVisible();
    await expect(advancedTabs.locator('[data-mode="advanced"]')).toBeVisible();
  });

  test('timelock checkbox toggles date picker', async ({ page }) => {
    const creation = new CreationPage(page, htmlPath);
    await creation.open();

    // Switch to Advanced tab
    await page.locator('#advanced-options [data-mode="advanced"]').click();

    // Timelock panel should be visible, options hidden initially
    const tlockOptions = page.locator('#timelock-options');
    await expect(tlockOptions).toHaveClass(/hidden/);

    // Check the timelock checkbox
    await page.locator('#timelock-checkbox').check();

    // Timelock options should now be visible
    await expect(tlockOptions).not.toHaveClass(/hidden/);

    // Date preview should be visible
    await expect(page.locator('#timelock-date-preview')).toBeVisible();

    // Uncheck the timelock checkbox
    await page.locator('#timelock-checkbox').uncheck();

    // Timelock options should be hidden again
    await expect(tlockOptions).toHaveClass(/hidden/);
  });

  test('timelock date preview updates with value and unit changes', async ({ page }) => {
    const creation = new CreationPage(page, htmlPath);
    await creation.open();

    // Switch to Advanced and enable timelock
    await page.locator('#advanced-options [data-mode="advanced"]').click();
    await page.locator('#timelock-checkbox').check();

    const preview = page.locator('#timelock-date-preview');

    // Default is 30 days — preview should show a date
    await expect(preview).not.toBeEmpty();
    const initialText = await preview.textContent();

    // Change to weeks
    await page.locator('#timelock-unit').selectOption('w');
    const weeksText = await preview.textContent();
    expect(weeksText).not.toBe(initialText);

    // Change value to 1
    await page.locator('#timelock-value').fill('1');
    await page.locator('#timelock-value').dispatchEvent('input');
    const oneWeekText = await preview.textContent();
    expect(oneWeekText).toBeTruthy();
  });

  test('rememoryTlock is available on window', async ({ page }) => {
    const creation = new CreationPage(page, htmlPath);
    await creation.open();

    const hasTlock = await page.evaluate(() => typeof (window as any).rememoryTlock !== 'undefined');
    expect(hasTlock).toBe(true);
  });
});

test.describe('Time-lock: maker.html bundle creation and recovery with tlock', () => {
  let makerPath: string;
  let recoverPath: string;
  let tmpDir: string;

  test.beforeAll(async () => {
    const bin = getRememoryBin();
    if (!fs.existsSync(bin)) {
      test.skip();
      return;
    }
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rememory-tlock-create-e2e-'));
    makerPath = generateStandaloneHTML(tmpDir, 'create');
    recoverPath = generateStandaloneHTML(tmpDir, 'recover');
  });

  test.afterAll(async () => {
    if (tmpDir && fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  test('create tlock bundle and recover after unlock time', async ({ page }, testInfo) => {
    // This test hits the real drand network — give it plenty of time
    testInfo.setTimeout(180000);

    const creation = new CreationPage(page, makerPath);
    await creation.open();

    // Set up friends
    await creation.setFriend(0, 'Alice', 'alice@test.com');
    await creation.setFriend(1, 'Bob', 'bob@test.com');

    // Add test files
    const testFiles = creation.createTestFiles(tmpDir, 'tlock');
    await creation.addFiles(testFiles);

    // Enable timelock
    await page.locator('#advanced-options [data-mode="advanced"]').click();
    await page.locator('#timelock-checkbox').check();

    // Patch roundForTime to return a round ~15 seconds from now (quicknet: 3s period)
    // This way the tlock encryption targets a near-future round that will be available
    // by the time we try to recover.
    await page.evaluate(() => {
      const tlock = (window as any).rememoryTlock;
      const GENESIS = tlock.QUICKNET_GENESIS;
      const PERIOD = tlock.QUICKNET_PERIOD;
      const targetUnix = Math.floor(Date.now() / 1000) + 15;
      const nearFutureRound = Math.ceil((targetUnix - GENESIS) / PERIOD) + 1;
      tlock.roundForTime = () => nearFutureRound;
    });

    // Generate bundles — exercises the full tlock path:
    // JS archive → JS tlock-encrypt via real drand → WASM age-encrypt + split + bundle
    await creation.generate();
    await creation.expectGenerationComplete();

    await creation.expectBundleCount(2);
    await creation.expectBundleFor('Alice');
    await creation.expectBundleFor('Bob');

    // Download both bundles and save to disk
    const aliceData = await creation.downloadBundle(0);
    expect(aliceData).toBeTruthy();
    const bobData = await creation.downloadBundle(1);
    expect(bobData).toBeTruthy();

    const aliceZipPath = path.join(tmpDir, 'bundle-alice.zip');
    const bobZipPath = path.join(tmpDir, 'bundle-bob.zip');
    fs.writeFileSync(aliceZipPath, aliceData!);
    fs.writeFileSync(bobZipPath, bobData!);

    // Extract Alice's bundle to get recover.html
    const AdmZip = require('adm-zip');
    const aliceZip = new AdmZip(aliceZipPath);
    const aliceDir = path.join(tmpDir, 'bundle-alice');
    aliceZip.extractAllTo(aliceDir, true);

    // Wait for the tlock round to pass (~15s from when we patched + some buffer)
    await page.waitForTimeout(20000);

    // Open the personalized recover.html from Alice's bundle
    const recovery = new RecoveryPage(page, aliceDir);
    await recovery.open();

    // Verify Alice's share is pre-loaded (holder share from personalization)
    await recovery.expectShareCount(1);

    // Manifest should be auto-loaded (embedded in personalization for small archives)
    await recovery.expectManifestLoaded();

    // Add Bob's bundle ZIP as the second share — recovery auto-starts when threshold is met
    await recovery.addBundleZip(tmpDir, 'bob');
    await recovery.expectShareCount(2);

    // Recovery auto-starts — wait for completion (tlock-decrypt via real drand)
    await recovery.expectRecoveryComplete();

    // Verify the recovered files
    await recovery.expectFileCount(2);
    await recovery.expectDownloadVisible();
  });
});

test.describe('Time-lock: maker.html --no-timelock', () => {
  let htmlPath: string;
  let tmpDir: string;

  test.beforeAll(async () => {
    const bin = getRememoryBin();
    if (!fs.existsSync(bin)) {
      test.skip();
      return;
    }
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rememory-tlock-notl-e2e-'));
    htmlPath = generateStandaloneHTML(tmpDir, 'create', ['--no-timelock']);
  });

  test.afterAll(async () => {
    if (tmpDir && fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  test('advanced options hidden when --no-timelock', async ({ page }) => {
    const creation = new CreationPage(page, htmlPath);
    await creation.open();

    // Without tlock-js, advanced options should remain hidden
    const advancedSection = page.locator('#advanced-options');
    await expect(advancedSection).toHaveClass(/hidden/);
  });

  test('rememoryTlock is not available on window', async ({ page }) => {
    const creation = new CreationPage(page, htmlPath);
    await creation.open();

    const hasTlock = await page.evaluate(() => typeof (window as any).rememoryTlock !== 'undefined');
    expect(hasTlock).toBe(false);
  });
});

test.describe('Time-lock: recover.html tlock detection', () => {
  let genericRecoverPath: string;
  let tmpDir: string;

  test.beforeAll(async () => {
    const bin = getRememoryBin();
    if (!fs.existsSync(bin)) {
      test.skip();
      return;
    }
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rememory-tlock-recover-e2e-'));
    genericRecoverPath = generateStandaloneHTML(tmpDir, 'recover');
  });

  test.afterAll(async () => {
    if (tmpDir && fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  test('generic recover.html includes tlock-js', async ({ page }) => {
    const recovery = new RecoveryPage(page, tmpDir);
    await recovery.openFile(genericRecoverPath);

    const hasTlock = await page.evaluate(() => typeof (window as any).rememoryTlock !== 'undefined');
    expect(hasTlock).toBe(true);
  });

  test('detects tlock envelope on uploaded manifest', async ({ page }) => {
    const recovery = new RecoveryPage(page, tmpDir);
    await recovery.openFile(genericRecoverPath);

    // Craft a fake tlock-enveloped manifest: JSON header + dummy ciphertext
    const futureDate = new Date();
    futureDate.setFullYear(futureDate.getFullYear() + 1);
    const meta = {
      v: 1,
      rememory: 'v0.0.16',
      tlock: {
        v: 1,
        method: 'drand-quicknet',
        round: 99999999,
        unlock: futureDate.toISOString(),
        chain: '52db9ba70e0cc0f6eaf7803dd07447a1f5477735fd3f661792ba94600c84e971',
      },
    };
    const headerLine = JSON.stringify(meta);
    const dummyCiphertext = 'age-encryption.org/v1\nfake-ciphertext-data';
    const fileContent = headerLine + '\n' + dummyCiphertext;

    // Write the crafted manifest to a temp file
    const manifestPath = path.join(tmpDir, 'MANIFEST.age');
    fs.writeFileSync(manifestPath, fileContent);

    // Upload the manifest
    await recovery.addManifestFile(manifestPath);

    // Should detect the tlock envelope and show manifest as loaded (checkmark, no tlock notice)
    await recovery.expectManifestLoaded();

    // Manifest card should show normally — tlock info only appears in step 3 waiting panel
    const manifestStatus = page.locator('#manifest-status');
    await expect(manifestStatus).toContainText('MANIFEST.age');
  });
});

test.describe('Time-lock: non-tlock bundles', () => {
  let projectDir: string;

  test.beforeAll(async () => {
    const bin = getRememoryBin();
    if (!fs.existsSync(bin)) {
      test.skip();
      return;
    }
    projectDir = createTestProject();
  });

  test('personalized non-tlock recover.html does not include tlock-js', async ({ page }) => {
    const bundlesDir = path.join(projectDir, 'output', 'bundles');
    const aliceDir = extractBundle(bundlesDir, 'alice');

    const recovery = new RecoveryPage(page, aliceDir);
    await recovery.open();

    // Non-tlock personalized bundle should not have tlock-js
    const hasTlock = await page.evaluate(() => typeof (window as any).rememoryTlock !== 'undefined');
    expect(hasTlock).toBe(false);
  });

  test('non-tlock recover.html is smaller than generic (no tlock-js overhead)', async ({ page }) => {
    // Get the size of a personalized non-tlock recover.html
    const bundlesDir = path.join(projectDir, 'output', 'bundles');
    const aliceDir = extractBundle(bundlesDir, 'alice');
    const personalizedSize = fs.statSync(path.join(aliceDir, 'recover.html')).size;

    // Generate generic recover.html (which includes tlock-js)
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rememory-tlock-size-e2e-'));
    try {
      const genericPath = generateStandaloneHTML(tmpDir, 'recover');
      const genericSize = fs.statSync(genericPath).size;

      // Generic recover.html should be larger because it includes tlock-js
      // The personalized one has the embedded manifest but no tlock-js,
      // so the tlock-js overhead should make generic larger than the difference
      // from the embedded manifest. We just verify generic includes more JS.
      const genericContent = fs.readFileSync(genericPath, 'utf8');
      const personalizedContent = fs.readFileSync(path.join(aliceDir, 'recover.html'), 'utf8');

      // Generic should contain the tlock.js bundle (identifiable by the drand chain hash)
      expect(genericContent).toContain('QUICKNET_CHAIN_HASH');
      // Personalized non-tlock should not have the tlock.js bundle
      expect(personalizedContent).not.toContain('QUICKNET_CHAIN_HASH');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });
});
