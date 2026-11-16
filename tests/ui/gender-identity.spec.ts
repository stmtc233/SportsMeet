import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [540, 1280]) {
  test(`性别认同与参赛类别独立，${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.emulateMedia({ colorScheme: width === 540 ? "dark" : "light" });
    await installRuntime(page);
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("navigation").getByRole("button", { name: "报名", exact: true }).click();
    if (width < 700) await page.getByRole("complementary", { name: "组别与单位" }).getByRole("button", { name: /^全部 / }).click();
    await page.getByRole("button", { name: "运动员1", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "编辑运动员", exact: true });
    const category = dialog.getByRole("radiogroup", { name: "参赛类别", exact: true });
    const entries = dialog.getByRole("group", { name: "报名项目", exact: true });
    await expect(category.getByRole("radio", { name: "男子", exact: true })).toBeChecked();
    await dialog.getByRole("button", { name: "性别认同", exact: true }).click();
    await dialog.getByRole("option", { name: "非二元", exact: true }).click();
    await expect(category.getByRole("radio", { name: "男子", exact: true })).toBeChecked();
    await expect(entries.getByRole("checkbox", { name: "女子100m", exact: true })).toHaveCount(0);
    await dialog.getByRole("checkbox", { name: "查看全部项目", exact: true }).check();
    await entries.getByRole("checkbox", { name: "女子100m", exact: true }).check();
    await dialog.getByRole("checkbox", { name: "查看全部项目", exact: true }).uncheck();
    await expect(entries.getByRole("checkbox", { name: "女子100m", exact: true })).toBeChecked();
    await category.getByRole("radio", { name: "女子", exact: true }).click();
    await expect(dialog.getByRole("button", { name: "性别认同", exact: true })).toHaveText("非二元");
    await expect(entries.getByRole("checkbox", { name: "男子100m", exact: true })).toBeChecked();
    await dialog.screenshot({ path: testInfo.outputPath("athlete-gender.png") });
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog).toBeHidden();
    const saved = await page.evaluate(() => (globalThis as any).__athleteSaves.at(-1)[2]);
    expect(saved).toMatchObject({ competitionGender: "女", genderIdentity: "非二元" });
    expect(saved).not.toHaveProperty("gender");
    expect(await page.evaluate(() => (globalThis as any).__athleteEntries[2])).toEqual(["event", "female"]);
  });
}

test("保存并继续不沿用性别认同", async ({ page }) => {
  await installRuntime(page);
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("navigation").getByRole("button", { name: "报名", exact: true }).click();
  await page.getByRole("button", { name: "添加运动员", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "添加运动员", exact: true });
  await dialog.getByRole("textbox", { name: "姓名", exact: true }).fill("甲");
  await dialog.getByRole("button", { name: "性别认同", exact: true }).click();
  await dialog.getByRole("option", { name: "不愿透露", exact: true }).click();
  const category = dialog.getByRole("radiogroup", { name: "参赛类别", exact: true });
  await category.getByRole("radio", { name: "女子", exact: true }).click();
  await dialog.getByRole("button", { name: "保存并继续", exact: true }).click();
  await expect(dialog.getByRole("textbox", { name: "姓名", exact: true })).toHaveValue("");
  await expect(dialog.getByRole("button", { name: "性别认同", exact: true })).toHaveText("未填");
  await expect(category.getByRole("radio", { name: "女子", exact: true })).toBeChecked();
});
