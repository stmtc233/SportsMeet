import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

test("侧栏选中底板停在当前分类上", async ({ page }) => {
  await installRuntime(page);
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "项目", exact: true }).click();
  const nav = page.locator(".app__side .st-nav-group");
  const thumb = nav.locator(".st-nav-group__thumb");
  const aligned = (name: string) => async () => {
    const item = await nav.getByRole("button", { name, exact: true }).boundingBox();
    const box = await thumb.boundingBox();
    return item && box ? Math.abs(item.y - box.y) + Math.abs(item.height - box.height) : -1;
  };
  await expect.poll(aligned("项目")).toBeLessThan(1);
  await page.getByRole("button", { name: "运动员", exact: true }).click();
  await expect.poll(aligned("运动员")).toBeLessThan(1);
});
