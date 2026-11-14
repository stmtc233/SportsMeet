import { expect, test } from "@playwright/test";
import { installCombinedRuntime } from "./combined_runtime";

for (const width of [1280, 800]) {
  for (const colorScheme of ["light", "dark"] as const) {
    test(`全能创建、报名、分项和总成绩 ${width} ${colorScheme}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.emulateMedia({ colorScheme });
      await installCombinedRuntime(page);
      await page.goto("/");
      await page.getByText("秋季运动会", { exact: true }).click();
      await page.getByRole("button", { name: "项目", exact: true }).click();
      await page.getByRole("button", { name: "添加全能", exact: true }).click();
      const create = page.getByRole("dialog", { name: "添加全能", exact: true });
      await create.getByLabel("全能名称", { exact: true }).fill("校园两项全能");
      const sections = create.locator("section");
      await sections.nth(0).getByLabel("项目名", { exact: true }).fill("100m");
      await sections.nth(0).getByLabel("计分表", { exact: true }).fill("13\t100\n14\t80");
      await sections.nth(0).getByRole("button", { name: "预览计分表", exact: true }).click();
      await expect(sections.nth(0).getByText("成绩标准", { exact: true })).toBeVisible();
      await sections.nth(1).getByLabel("项目名", { exact: true }).fill("跳远");
      await sections.nth(1).getByLabel("类型", { exact: true }).click();
      await page.getByRole("option", { name: "远度", exact: true }).click();
      await sections.nth(1).getByLabel("计分表", { exact: true }).fill("5\t100\n4\t80");
      await page.screenshot({ path: `test-results/combined-create-${width}-${colorScheme}.png`, animations: "disabled" });
      await create.getByRole("button", { name: "保存", exact: true }).click();
      await expect(create).not.toBeVisible();
      await page.getByText("校园两项全能", { exact: true }).click();
      await page.getByRole("radio", { name: "报名", exact: true }).click();
      await page.getByRole("button", { name: "管理报名", exact: true }).click();
      const roster = page.getByRole("dialog", { name: "全能报名", exact: true });
      await roster.getByRole("checkbox").nth(1).check();
      await roster.getByRole("checkbox").nth(2).check();
      await roster.getByRole("button", { name: "保存报名", exact: true }).click();
      await expect(roster).not.toBeVisible();
      await expect(page.getByText("2 人报名", { exact: true })).toBeVisible();
      for (const i of [0, 1]) {
        await page.getByRole("radio", { name: "分项", exact: true }).click();
        await page.getByRole("button", { name: "编排", exact: true }).nth(i).click();
        await page.getByRole("button", { name: "自动编排", exact: true }).first().click();
        await page.getByRole("dialog", { name: "自动编排", exact: true }).getByRole("button", { name: "生成", exact: true }).click();
        await page.locator(".subbar").getByRole("button", { name: "保存", exact: true }).click();
        await expect(page.locator(".subbar").getByRole("button", { name: "保存", exact: true })).toBeDisabled();
        await page.getByRole("radio", { name: "成绩", exact: true }).click();
        const marks = page.getByRole("textbox", { name: i === 0 ? "成绩" : "试跳 1", exact: true });
        await marks.nth(0).fill(i === 0 ? "12.50" : "5.20");
        await marks.nth(1).fill(i === 0 ? "13.50" : "4.50");
        await page.getByRole("button", { name: "保存", exact: true }).click();
        await expect(page.getByRole("button", { name: "保存", exact: true })).toBeDisabled();
        await page.getByRole("button", { name: "返回全能", exact: true }).click();
      }
      await page.getByRole("radio", { name: "总成绩", exact: true }).click();
      await expect(page.getByText("正式排名", { exact: true })).toBeVisible();
      await expect(page.getByText("200", { exact: true })).toBeVisible();
      await expect(page.getByText("160", { exact: true })).toBeVisible();
      await page.screenshot({ path: `test-results/combined-results-${width}-${colorScheme}.png`, animations: "disabled" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.getByRole("button", { name: "计分规则", exact: true }).click();
      const rules = page.getByRole("dialog", { name: "计分规则", exact: true });
      await rules.getByLabel("计分表", { exact: true }).nth(0).fill("13\t120\n14\t80");
      await expect(rules.getByRole("button", { name: "保存规则", exact: true })).toBeDisabled();
      await rules.getByRole("button", { name: "预览重算", exact: true }).click();
      await expect(rules.getByText("220", { exact: true })).toBeVisible();
      await page.screenshot({ path: `test-results/combined-rules-${width}-${colorScheme}.png`, animations: "disabled" });
      await rules.getByRole("button", { name: "保存规则", exact: true }).click();
      await expect(rules).not.toBeVisible();
      await expect(page.getByText("220", { exact: true })).toBeVisible();
      const calls = await page.evaluate(() => (globalThis as any).__combinedCalls);
      expect(calls.find((c: any) => c.method === "Combined.Entries").args[1]).toEqual(["a0", "a1"]);
    });
  }
}
