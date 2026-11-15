#!/usr/bin/env node
// Builds the website for publishing: copies website/ to the output directory
// and replaces the downloads block of index.html (between the
// "downloads:start" and "downloads:end" comments) with links to the files of
// a release.
//
//   node scripts/website-downloads.mjs <releases.json> <outDir>
//
// releases.json is the answer of GitHub's "list releases" API. The newest
// release that is not a prerelease is linked; while there is none, the
// newest prerelease. Without any release the block stays as written.
import { cpSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const [releasesPath, outDir] = process.argv.slice(2);
if (!releasesPath || !outDir) {
  console.error("usage: website-downloads.mjs <releases.json> <outDir>");
  process.exit(2);
}
const root = join(dirname(fileURLToPath(import.meta.url)), "..");

rmSync(outDir, { recursive: true, force: true });
cpSync(join(root, "website"), outDir, { recursive: true });

const releases = JSON.parse(readFileSync(releasesPath, "utf8")).filter((r) => !r.draft);
const release = releases.find((r) => !r.prerelease) ?? releases[0];
if (!release) {
  console.log("no release yet: the downloads block stays as written");
  process.exit(0);
}

const escape = (s) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

/** The download URL of the asset whose name matches; every listed file must be there. */
function asset(pattern) {
  const found = release.assets.find((a) => pattern.test(a.name));
  if (!found) throw new Error(`release ${release.tag_name} has no file matching ${pattern}`);
  return escape(found.browser_download_url);
}

// What mygo build and .github/workflows/release.yml name the files.
const platforms = [
  { name: "macOS", builds: [
    { needs: "macOS 12 及以上", main: ["下载安装包", /\.dmg$/], alt: [["便携版", /-darwin-universal-portable\.zip$/]] },
  ] },
  { name: "Windows", builds: [
    { needs: "Windows 10 / 11", main: ["下载安装包", /^sportsmeet-setup-.*\.exe$/], alt: [["便携版", /-windows-amd64-portable\.zip$/]] },
  ] },
  { name: "Linux", builds: [
    { needs: "x86-64", main: ["下载 .deb", /_amd64\.deb$/], alt: [[".rpm", /\.x86_64\.rpm$/], ["便携版", /-linux-amd64\.tar\.gz$/]] },
    { needs: "ARM64", optional: true, main: ["下载 .deb", /_arm64\.deb$/], alt: [[".rpm", /\.aarch64\.rpm$/], ["便携版", /-linux-arm64\.tar\.gz$/]] },
  ] },
];

// Releases without ARM64 files remain publishable; a partial set is an error.
function buildLinks(platform, build) {
  const links = [build.main, ...build.alt];
  if (build.optional && !links.some(([, pattern]) => release.assets.some((a) => pattern.test(a.name)))) return "";
  const link = ([label, pattern], className = "") => `<a${className ? ` class="${className}"` : ""} href="${asset(pattern)}" aria-label="${escape(`${platform} ${build.needs} ${label}`)}">${label}</a>`;
  return `      <p>${build.needs}</p>
      ${link(build.main, "tag")}
      <div class="alt">${build.alt.map((a) => link(a)).join("")}</div>`;
}

const version = escape(release.tag_name.replace(/^v/, ""));
const cards = platforms.map((p) => `    <div class="card platform">
      <h3>${p.name}</h3>
${p.builds.map((b) => buildLinks(p.name, b)).filter(Boolean).join("\n")}
    </div>`).join("\n");

const block = `<p class="lead">${release.prerelease ? "测试版" : "最新版本"} ${version}</p>
  <div class="grid platforms">
${cards}
  </div>
  <p class="note"><a href="${escape(release.html_url)}" rel="noopener">更新说明与全部文件</a></p>`;

const indexPath = join(outDir, "index.html");
const html = readFileSync(indexPath, "utf8");
const marked = /<!-- downloads:start[^>]*-->[\s\S]*?<!-- downloads:end -->/;
if (!marked.test(html)) throw new Error("website/index.html has no downloads block");
writeFileSync(indexPath, html.replace(marked, () => block));
console.log(`linked ${release.tag_name}`);
