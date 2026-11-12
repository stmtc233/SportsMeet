import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

test("数据刷新保留未保存的成绩输入", async ({ page }) => {
  await installRuntime(page);
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByText("男子100m 初中组", { exact: true }).click();
  await page.getByRole("radio", { name: "成绩", exact: true }).click();
  const marks = page.getByRole("textbox", { name: "成绩", exact: true });
  await marks.nth(0).fill("12.34");
  await marks.nth(1).fill("12.50");
  await page.evaluate(() => (globalThis as any).__scheduleExternalChange());
  await page.waitForTimeout(700); // past the window that merges reloads
  await expect(marks.nth(0)).toHaveValue("12.34");
  await expect(marks.nth(1)).toHaveValue("12.50");
  const save = page.getByRole("button", { name: "保存", exact: true });
  await expect(save).toBeEnabled();
  await save.click();
  await expect(save).toBeDisabled();
  await expect(marks.nth(0)).toHaveValue("12.34");
  await expect(page.getByRole("button", { name: "放弃", exact: true })).toBeDisabled();
});
