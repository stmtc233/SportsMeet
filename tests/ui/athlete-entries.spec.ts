import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

test("运动员对话框勾选报名项目", async ({ page }) => {
  await installRuntime(page);
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("navigation").getByRole("button", { name: "报名", exact: true }).click();
  await page.getByRole("button", { name: "运动员1", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "编辑运动员", exact: true });
  const group = dialog.getByRole("group", { name: "报名项目" });
  // In a heat: the entry stays until moved out there.
  await expect(group.getByRole("checkbox", { name: "男子100m", exact: true })).toBeChecked();
  await expect(group.getByRole("checkbox", { name: "男子100m", exact: true })).toBeDisabled();
  await group.getByRole("checkbox", { name: "混合跳高", exact: true }).check();
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(await page.evaluate(() => (globalThis as any).__athleteEntries)).toEqual(["meet", "a0", ["event", "high"]]);

  // Saving without touching the events leaves the entries alone.
  await page.evaluate(() => { (globalThis as any).__athleteEntries = undefined; });
  await page.getByRole("button", { name: "运动员2", exact: true }).click();
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(await page.evaluate(() => (globalThis as any).__athleteEntries)).toBeUndefined();
});
