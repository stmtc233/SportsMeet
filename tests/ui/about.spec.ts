import { expect, test, type Page } from "@playwright/test";
import { installRuntime } from "./runtime";

async function openAbout(page: Page) {
  await page.goto("/");
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await page.getByRole("navigation", { name: "设置分类" }).getByRole("button", { name: "关于", exact: true }).click();
  await expect(page.getByText("0.1.0-beta.2", { exact: true })).toBeVisible();
}

for (const width of [1280, 800]) {
  for (const colorScheme of ["light", "dark"] as const) {
    test(`关于页更新渠道与仓库 ${width} ${colorScheme}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 800 });
      await page.emulateMedia({ colorScheme });
      await installRuntime(page, { slowUpdatePreferences: true });
      await openAbout(page);
      const repository = page.getByRole("link", { name: "打开仓库" });
      await expect(repository).toHaveAttribute("href", "https://github.com/stmtc233/SportsMeet");
      await repository.focus();
      await repository.press("Enter");
      await expect.poll(() => page.evaluate(() => (globalThis as any).__repositoryOpened)).toBe(true);
      await expect(page.getByRole("heading", { name: "关于", exact: true })).toBeVisible();

      const prereleases = page.getByRole("switch", { name: "接收测试版" });
      await expect(prereleases).toHaveAttribute("aria-checked", "false");
      await prereleases.click();
      await expect(prereleases).toHaveAttribute("aria-checked", "true");
      await page.getByRole("button", { name: "检查更新", exact: true }).click();
      await expect.poll(() => page.evaluate(() => (globalThis as any).__updateChecks)).toEqual([{ prereleases: true, selfUpdate: true }]);
      await page.screenshot({ path: testInfo.outputPath("about.png") });

      if (width === 800) await page.getByRole("button", { name: "设置分类", exact: true }).click();
      await page.getByRole("navigation", { name: "设置分类" }).getByRole("button", { name: "通用", exact: true }).click();
      await page.getByRole("radio", { name: "深色", exact: true }).click();
      if (width === 800) await page.getByRole("button", { name: "设置分类", exact: true }).click();
      await page.getByRole("navigation", { name: "设置分类" }).getByRole("button", { name: "关于", exact: true }).click();
      await expect(prereleases).toHaveAttribute("aria-checked", "true");
      await prereleases.click();
      await page.getByRole("button", { name: "检查更新", exact: true }).click();
      await expect.poll(() => page.evaluate(() => (globalThis as any).__updateChecks?.at(-1))).toEqual({ prereleases: false, selfUpdate: true });
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    });
  }
}

test("开发运行提供发布页入口", async ({ page }) => {
  await installRuntime(page, { selfUpdate: false });
  await openAbout(page);
  await expect(page.getByText("开发运行，请从发布页下载安装", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "检查更新", exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "查看发布", exact: true }).click();
  await expect.poll(() => page.evaluate(() => (globalThis as any).__updateChecks)).toEqual([{ prereleases: false, selfUpdate: false }]);
});

test("测试版选项保存失败后恢复", async ({ page }) => {
  await installRuntime(page, { rejectUpdatePreferences: true });
  await openAbout(page);
  const prereleases = page.getByRole("switch", { name: "接收测试版" });
  await prereleases.click();
  await expect(page.getByText("设置无法保存，请重试", { exact: true })).toBeVisible();
  await expect(prereleases).toHaveAttribute("aria-checked", "false");
});
