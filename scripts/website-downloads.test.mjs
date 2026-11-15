import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

const script = join(dirname(fileURLToPath(import.meta.url)), "website-downloads.mjs");
const amd64Files = [
  "sportsmeet-0.1.0.dmg",
  "sportsmeet-0.1.0-darwin-universal-portable.zip",
  "sportsmeet-setup-0.1.0-amd64.exe",
  "sportsmeet-0.1.0-windows-amd64-portable.zip",
  "sportsmeet_0.1.0_amd64.deb",
  "sportsmeet-0.1.0-1.x86_64.rpm",
  "sportsmeet-0.1.0-linux-amd64.tar.gz",
];
const arm64Files = [
  "sportsmeet_0.1.0_arm64.deb",
  "sportsmeet-0.1.0-1.aarch64.rpm",
  "sportsmeet-0.1.0-linux-arm64.tar.gz",
];

function build(files) {
  const dir = mkdtempSync(join(tmpdir(), "sportsmeet-downloads-"));
  try {
    const releases = join(dir, "releases.json");
    const out = join(dir, "site");
    writeFileSync(releases, JSON.stringify([{
      tag_name: "v0.1.0", draft: false, prerelease: false,
      html_url: "https://example.com/releases/v0.1.0",
      assets: files.map((name) => ({ name, browser_download_url: `https://example.com/${name}` })),
    }]));
    const result = spawnSync(process.execPath, [script, releases, out], { encoding: "utf8" });
    return { ...result, html: result.status === 0 ? readFileSync(join(out, "index.html"), "utf8") : "" };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

test("Linux links select their architecture even when ARM64 assets come first", () => {
  const { status, stderr, html } = build([...arm64Files, ...amd64Files]);
  assert.equal(status, 0, stderr);
  for (const [arch, files] of [["x86-64", amd64Files.slice(-3)], ["ARM64", arm64Files]]) {
    for (const [i, label] of ["下载 .deb", ".rpm", "便携版"].entries()) {
      assert.ok(html.includes(`href="https://example.com/${files[i]}" aria-label="Linux ${arch} ${label}"`));
    }
  }
});

test("releases without ARM64 assets still publish x86-64 downloads", () => {
  const { status, stderr, html } = build(amd64Files);
  assert.equal(status, 0, stderr);
  assert.ok(html.includes('aria-label="Linux x86-64 下载 .deb"'));
  assert.ok(!html.includes("ARM64"));
});

test("a partial ARM64 release fails instead of linking to another architecture", () => {
  for (const missing of arm64Files) {
    const { status, stderr } = build([...amd64Files, ...arm64Files.filter((name) => name !== missing)]);
    assert.notEqual(status, 0, missing);
    assert.match(stderr, /has no file matching/);
  }
});
