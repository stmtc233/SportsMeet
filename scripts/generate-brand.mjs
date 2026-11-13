// Vector geometry and outlined lettering are the inputs; exports never depend
// on an installed font. Requires the project's Playwright Chromium and Python 3
// (standard library only, for the delivery ZIP).
import { chromium } from "playwright";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile, copyFile, readdir } from "node:fs/promises";
import { resolve, relative, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { execFileSync } from "node:child_process";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const base = resolve(root, "resources/brand");
const identity = JSON.parse(await readFile(resolve(base, "source/identity.json"), "utf8"));
const wordmark = JSON.parse(await readFile(resolve(base, "source/wordmark.json"), "utf8"));
const C = identity.colors;
const n = value => Number(value.toFixed(4));
const save = async (path, data) => {
  const destination = resolve(base, path);
  await mkdir(dirname(destination), { recursive: true });
  await writeFile(destination, data);
};

// Each lane is a closed ribbon with two concentric semicircles. Opposite
// curvature reverses the offset at the middle straight. No raster tracing.
function mark(color, small = false) {
  const m = identity.mark;
  const [ux, uy] = m.upperCenter;
  const [lx, ly] = m.lowerCenter;
  const t = (small ? m.smallLaneWidth : m.laneWidth) / 2;
  return (small ? m.smallLaneOffsets : m.laneOffsets).map(d => {
    const upper = m.radius - d;
    const lower = m.radius + d;
    const path = `M${m.terminalX[0]} ${uy-upper-t}H${ux}` +
      `A${upper+t} ${upper+t} 0 0 0 ${ux} ${uy+upper+t}H${lx}` +
      `A${lower-t} ${lower-t} 0 0 1 ${lx} ${ly+lower-t}H${m.terminalX[1]}` +
      `V${ly+lower+t}H${lx}A${lower+t} ${lower+t} 0 0 0 ${lx} ${ly-lower-t}` +
      `H${ux}A${upper-t} ${upper-t} 0 0 1 ${ux} ${uy-upper+t}H${m.terminalX[0]}Z`;
    return `<path fill="${color}" d="${path}"/>`;
  }).join("");
}

function svg(width, height, content, title = "SportsMeet") {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-label="${title}"><title>${title}</title>${content}</svg>\n`;
}

function lettering(color, capHeight, x, y) {
  const scale = capHeight / wordmark.capHeight;
  return `<path fill="${color}" transform="translate(${n(x)} ${n(y)}) scale(${n(scale)})" d="${wordmark.path}"/>`;
}

function lockup(layout, variant) {
  const symbolColor = variant === "blue" ? C.blue : C[variant];
  const textColor = variant === "blue" ? C.ink : C[variant];
  if (layout === "mark") return svg(512, 512, mark(symbolColor));
  if (layout === "horizontal") {
    // Visible mark height 160; gap 40; cap height 88. Canvas includes clear space.
    const scale = 160 / 444;
    const cap = 88;
    const textWidth = wordmark.advance * cap / wordmark.capHeight;
    return svg(n(64+160+40+textWidth+64), 288,
      `<g transform="translate(${n(64-34*scale)} ${n(64-34*scale)}) scale(${n(scale)})">${mark(symbolColor)}</g>` +
      lettering(textColor, cap, 264, 97));
  }
  const cap = 64;
  const textWidth = wordmark.advance * cap / wordmark.capHeight;
  const width = 688;
  return svg(width, 720,
    `<g transform="translate(88 40)">${mark(symbolColor)}</g>` +
    lettering(textColor, cap, (width-textWidth)/2, 580));
}

// Sample the same superellipse exponent as corner-shape: superellipse(1.78),
// into SVG geometry, so icon silhouettes do not depend on CSS support.
function smoothSquare(x, y, size, radius) {
  const exponent = 2 / (2 ** identity.appIcon.cornerShape);
  const centers = [[x+size-radius,y+radius], [x+size-radius,y+size-radius],
    [x+radius,y+size-radius], [x+radius,y+radius]];
  const points = [];
  centers.forEach(([cx,cy], corner) => {
    for (let step = 0; step <= 32; step++) {
      const angle = (-90+corner*90+step*90/32)*Math.PI/180;
      const cos = Math.cos(angle), sin = Math.sin(angle);
      points.push(`${n(cx+radius*Math.sign(cos)*Math.abs(cos)**exponent)} ${n(cy+radius*Math.sign(sin)*Math.abs(sin)**exponent)}`);
    }
  });
  return `<path fill="${C.blue}" d="M${points.join("L")}Z"/>`;
}

function appIcon({ square = false, small = false } = {}) {
  const { canvas, inset, cornerRadius, markScale } = identity.appIcon;
  const scale = square ? 1.66 : markScale;
  const offset = (canvas-512*scale)/2;
  return svg(canvas, canvas,
    (square ? `<path fill="${C.blue}" d="M0 0H1024V1024H0Z"/>` : smoothSquare(inset, inset, canvas-2*inset, cornerRadius)) +
    `<g transform="translate(${n(offset)} ${n(offset)}) scale(${scale})">${mark(C.white, small)}</g>`);
}

function favicon() {
  return svg(512, 512, smoothSquare(0,0,512,112) +
    `<g transform="translate(35.84 35.84) scale(.86)">${mark(C.white, true)}</g>`);
}

const browser = await chromium.launch({
  ...(process.env.SPORTSMEET_BROWSER_PATH ? { executablePath: process.env.SPORTSMEET_BROWSER_PATH } : {}),
});
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 });

// Canvas exports sample the SVG at the target size (4x for tiny icons). PNG and
// WebP preserve alpha; JPEG gets an explicit white or dark background.
async function raster(source, width, type = "png", background = null) {
  const encoded = await page.evaluate(async ({ source, width, type, background }) => {
    const image = new Image();
    image.src = "data:image/svg+xml;base64," + btoa(unescape(encodeURIComponent(source)));
    await image.decode();
    const height = Math.round(width*image.height/image.width);
    const factor = width <= 64 ? 4 : 1;
    const large = document.createElement("canvas");
    large.width = width*factor; large.height = height*factor;
    const context = large.getContext("2d");
    if (background) { context.fillStyle = background; context.fillRect(0,0,large.width,large.height); }
    context.drawImage(image,0,0,large.width,large.height);
    const canvas = document.createElement("canvas");
    canvas.width=width; canvas.height=height;
    canvas.getContext("2d").drawImage(large,0,0,width,height);
    return canvas.toDataURL("image/"+type,1).split(",")[1];
  }, { source, width, type, background });
  return Buffer.from(encoded, "base64");
}

async function artworkPDF(source, path) {
  const width = Number(source.match(/width="([\d.]+)"/)[1]);
  const height = Number(source.match(/height="([\d.]+)"/)[1]);
  await page.setContent(`<html><head><style>@page{size:${width}px ${height}px;margin:0}html,body{margin:0}svg{display:block}</style></head><body>${source}</body></html>`);
  await save(path, await page.pdf({ preferCSSPageSize: true, printBackground: true, omitBackground: true }));
}

try {
  for (const layout of ["mark", "horizontal", "stacked"]) {
    for (const variant of ["blue", "black", "white"]) {
      const source = lockup(layout, variant);
      const name = `${layout}-${variant}`;
      await save(`svg/${name}.svg`,source);
      for (const width of identity.pngWidths) await save(`png/${name}-${width}.png`,await raster(source,width));
      await save(`web/${name}.webp`,await raster(source,1024,"webp"));
      await save(`web/${name}.jpg`,await raster(source,1024,"jpeg",variant==="white" ? C.dark : C.white));
      if (variant === "blue") await artworkPDF(source, `pdf/${name}.pdf`);
    }
  }
  for (const variant of ["blue","black","white"]) await save(`svg/mark-small-${variant}.svg`,svg(512,512,mark(C[variant],true)));
  await save("svg/app-icon.svg",appIcon());
  await save("web/app-icon-square.svg",appIcon({ square: true }));
  await save("web/favicon.svg",favicon());
  const iconBuffers = new Map();
  for (const size of identity.iconSizes) {
    const source = size <= identity.mark.smallMaxPixels ? favicon() : appIcon();
    const data = await raster(source,size);
    iconBuffers.set(size,data);
    await save(`icons/app-${size}.png`,data);
  }
  for (const size of [180,192,512]) await save(`web/${size===180?"apple-touch-icon":"app-square-"+size}.png`,await raster(appIcon({square:true}),size));

  const icoSizes = identity.iconSizes.filter(s=>s<=256);
  const header=Buffer.alloc(6+16*icoSizes.length);
  header.writeUInt16LE(1,2); header.writeUInt16LE(icoSizes.length,4);
  let offset=header.length;
  const frames=icoSizes.map((size,i)=>{
    const data=iconBuffers.get(size), entry=6+16*i;
    header[entry]=size===256?0:size; header[entry+1]=header[entry];
    header.writeUInt16LE(1,entry+4); header.writeUInt16LE(32,entry+6);
    header.writeUInt32LE(data.length,entry+8); header.writeUInt32LE(offset,entry+12);
    offset+=data.length; return data;
  });
  await save("icons/app-icon.ico",Buffer.concat([header,...frames]));
  await save("web/favicon.ico",Buffer.concat([header,...frames]));
  const icnsTypes={icp4:16,icp5:32,icp6:64,ic07:128,ic08:256,ic09:512,ic10:1024,ic11:32,ic12:64,ic13:256,ic14:512};
  const chunks=Object.entries(icnsTypes).map(([type,size])=>{
    const data=iconBuffers.get(size), chunk=Buffer.alloc(8);
    chunk.write(type,0); chunk.writeUInt32BE(data.length+8,4);
    return Buffer.concat([chunk,data]);
  });
  const icnsHeader=Buffer.alloc(8); icnsHeader.write("icns");
  icnsHeader.writeUInt32BE(8+chunks.reduce((sum,b)=>sum+b.length,0),4);
  await save("icons/app-icon.icns",Buffer.concat([icnsHeader,...chunks]));

  // Visual documentation shares the exact exported vectors.
  const h = lockup("horizontal","blue"), s = lockup("stacked","blue");
  const black = lockup("horizontal","black"), white = lockup("horizontal","white");
  const icon = appIcon();
  const style = `<style>*{box-sizing:border-box}body{margin:0;color:${C.ink};font:15px/1.65 -apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;font-variant-numeric:tabular-nums;background:${C.paper}}h1,h2,h3,p{margin:0}h1{font-size:36px;letter-spacing:-1px}h2{font-size:27px}h3{font-size:16px;margin-bottom:8px}p{color:#586171}small{font-size:12px;color:#667080}svg{display:block;max-width:100%;height:auto}.sheet{width:794px;height:1123px;padding:58px;background:white;position:relative;overflow:hidden;break-after:page}.eyebrow{font-size:11px;letter-spacing:3px;color:${C.blue};margin-bottom:12px}.heading{margin-bottom:28px}.footer{position:absolute;bottom:35px;left:58px;right:58px;display:flex;justify-content:space-between;font-size:11px;color:#667080}.hero{width:480px;margin:8px auto 20px}.row{display:flex;gap:20px;align-items:stretch}.row>*{flex:1;min-width:0}.card{padding:22px;border:1px solid #E1E5EB;border-radius:18px;corner-shape:superellipse(1.78)}.card svg{width:100%}.dark{background:${C.dark};color:white;border-color:${C.dark}}.dark p{color:#BBC6D4}.swatch{height:54px;border-radius:9px;margin-bottom:8px}.gap{margin-top:24px}.section{margin-top:32px}.table{width:100%;border-collapse:collapse;font-size:13px}.table td,.table th{padding:10px 0;text-align:left;border-bottom:1px solid #E5E8ED;vertical-align:top}.table th{color:#667080;font-weight:500}.geometry{display:flex;align-items:center;justify-content:center;height:334px;background-image:linear-gradient(#EAF0F7 1px,transparent 1px),linear-gradient(90deg,#EAF0F7 1px,transparent 1px);background-size:20px 20px;background-color:#FAFBFD;border-radius:18px}.geometry svg{height:280px}.sizes{display:flex;align-items:end;justify-content:space-between;min-height:130px;gap:14px}.sizes div{flex:none;text-align:center}.sizes img{display:block;margin:auto auto 8px}.tag{display:inline-block;background:#E8F1FF;color:#0B4FB3;padding:5px 12px;border-radius:8px;font-size:12px}.usage{margin:16px 0;padding-left:20px;color:#586171}.usage li{margin-bottom:8px}.code{font-family:ui-monospace,monospace;font-size:12px;background:${C.paper};padding:15px;border-radius:10px;white-space:pre-wrap}@page{size:A4;margin:0}@media print{body{background:white}.sheet{width:210mm;height:297mm}}</style>`;
  const footer = number => `<div class="footer"><span>SPORTSMEET / 标识与应用规范</span><span>${number} / 04</span></div>`;
  const sheets = [
    `<section class="sheet"><div class="eyebrow">SPORTSMEET IDENTITY</div><h1>标识与应用规范</h1><p>跑道 S · 田径与校园运动会软件</p><div class="hero">${s}</div><div class="row"><div class="card"><h3>有秩序的运动感</h3><p>三条平行跑道组成 S。相同的间隔与连续的弯道，把田径和赛事编排联系起来。</p></div><div class="card" style="max-width:168px;padding:8px">${icon}</div></div><p class="gap">适用于应用图标、官网、项目文档与印刷材料。</p>${footer("01")}</section>`,
    `<section class="sheet"><div class="eyebrow">01 / CONSTRUCTION</div><div class="heading"><h2>几何与留白</h2><p>以矢量母版为准，等比例使用。</p></div><div class="geometry">${svg(512,512,`<rect x="34" y="34" width="444" height="444" fill="none" stroke="#A8C9FA" stroke-dasharray="6 6"/>${mark(C.blue)}`)}</div><table class="table gap"><tr><th>标准图形</th><td>512 × 512 画布；可见边界 444 × 444</td></tr><tr><th>跑道</th><td>3 条；线宽 28；中心距 40；净间距 12</td></tr><tr><th>安全空间</th><td>可见边界四周至少 56 单位，约为图形高度的 1/8</td></tr><tr><th>最小尺寸</th><td>标准图形宽 48px；小尺寸版宽 16px；印刷宽 10mm</td></tr></table><div class="section"><h3>品牌色与中性色</h3><div class="row">${[["品牌蓝",C.blue],["文字墨色",C.ink],["深色底",C.dark],["纸面灰",C.paper]].map(([name,color])=>`<div><div class="swatch" style="background:${color};border:1px solid #E1E5EB"></div><b>${name}</b><br><small>${color}</small></div>`).join("")}</div></div><p class="gap">黑白印刷使用纯黑；深色底使用纯白。渐变、阴影和纹理不属于主标。</p>${footer("02")}</section>`,
    `<section class="sheet"><div class="eyebrow">02 / LOCKUPS</div><div class="heading"><h2>组合与背景</h2><p>横式用于导航和页眉，竖式用于封面和展示。</p></div><div class="card">${h}</div><div class="row gap"><div class="card"><h3>单色</h3>${black}</div><div class="card dark"><h3>反白</h3>${white}</div></div><div class="row gap"><div class="card" style="display:flex;align-items:center;justify-content:center">${s}</div><div class="card"><h3>使用规则</h3><ul class="usage"><li>保持三条跑道与两段弯道。</li><li>使用导出的字标路径，名称写作 SportsMeet。</li><li>横式图形高 160，图文间距 40，字标大写字高 88。</li><li>不拉伸、旋转、描边或改变跑道数量。</li><li>复杂图片上先提供纯色承托。</li></ul></div></div><p class="gap">横式组合宽至少 320px，竖式宽至少 160px。更窄的位置单独使用图形。</p>${footer("03")}</section>`,
    `<section class="sheet delivery"><div class="eyebrow">03 / DELIVERY</div><div class="heading"><h2>图标与文件</h2><p>所有导出来自同一组几何参数与字标路径。</p></div><div class="sizes">${[16,24,32,48,64,128].map(size=>`<div><img src="data:image/png;base64,${iconBuffers.get(size).toString("base64")}" width="${size}" height="${size}"/><small>${size}px</small></div>`).join("")}</div><p class="gap">16–32px 使用加宽间隔的小尺寸版。桌面图标自带透明边距；触屏图标为不透明满底，由系统裁切。</p><table class="table section"><tr><th>格式</th><th>用途</th></tr><tr><td>SVG</td><td>可编辑矢量、网页、任意尺寸缩放</td></tr><tr><td>PNG / WebP</td><td>透明底图片，文档、网页与演示</td></tr><tr><td>JPG</td><td>白底或深底展示；不支持透明</td></tr><tr><td>PDF</td><td>矢量印刷稿与本规范；RGB 色彩</td></tr><tr><td>ICO / ICNS</td><td>Windows 多尺寸图标 / macOS 多分辨率图标</td></tr></table><div class="section"><h3>资产位置</h3><div class="code">resources/brand/{svg,png,web,icons,pdf}/<br>docs/brand-guidelines.md</div></div><div class="section"><h3>重新导出</h3><div class="code">bun install<br>bunx playwright install chromium<br>bun run brand:generate</div><small>需要 Python 3 标准库生成 ZIP。无须安装字体或联系在线服务。</small></div><p class="gap">字标基于 Noto Sans Bold，已转为路径；字体来源与许可见 LICENSES/OFL-NotoSans.txt。印刷转换为 CMYK 时由输出方打样确认。</p>${footer("04")}</section>`,
  ];
  const deliveryStyle="<style>.delivery .section{margin-top:24px}.delivery .table td,.delivery .table th{padding:8px 0}.delivery .code{padding:12px;font-size:11px}.delivery .gap{margin-top:18px}.delivery .sizes{min-height:122px}</style>";
  const guide=`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>SportsMeet 标识与应用规范</title>${style}${deliveryStyle}<body>${sheets.join("")}</body></html>`;
  await save("guide.html",guide);
  await page.setContent(guide);
  await page.evaluate(()=>document.fonts.ready);
  const crowdedPages = await page.locator(".sheet").evaluateAll(sheets => sheets.flatMap((sheet,index) => {
    const footerTop = sheet.querySelector(".footer").getBoundingClientRect().top;
    const contentBottom = Math.max(...[...sheet.children].filter(child=>!child.classList.contains("footer"))
      .map(child=>child.getBoundingClientRect().bottom));
    return contentBottom > footerTop-20 ? [index+1] : [];
  }));
  if (crowdedPages.length) throw new Error(`Guide content overlaps footer clearance on pages: ${crowdedPages.join(", ")}`);
  await save("pdf/brand-guidelines.pdf",await page.pdf({preferCSSPageSize:true,printBackground:true,tagged:true}));

  const preview=`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>SportsMeet 品牌总览</title>${style}<style>body{width:1280px;padding:56px}.overview{background:white;border-radius:28px;padding:44px}.overview h1{font-size:24px}.preview-hero{display:flex;align-items:center;gap:50px;padding:38px 0}.preview-hero .lockup{flex:1}.preview-hero .app{width:228px;flex:none}.preview-row{display:flex;gap:24px}.preview-row>.card{flex:1}.preview-row svg{height:112px;width:100%}.swatches{display:flex;gap:22px;margin-top:28px}.swatches>div{flex:1}.notes{display:flex;justify-content:space-between;margin-top:24px;color:#667080;font-size:13px}</style><body><div class="overview"><div class="eyebrow">SPORTSMEET / BRAND IDENTITY</div><h1>跑道 S · 正式标识</h1><div class="preview-hero"><div class="lockup">${h}</div><div class="app">${icon}</div></div><div class="preview-row"><div class="card"><small>单色印刷</small>${black}</div><div class="card dark"><small>深色背景</small>${white}</div></div><div class="swatches">${[["品牌蓝",C.blue],["文字墨色",C.ink],["深色底",C.dark],["纸面灰",C.paper]].map(([name,color])=>`<div><div class="swatch" style="background:${color};border:1px solid #E1E5EB"></div><small>${name} / ${color}</small></div>`).join("")}</div><div class="notes"><span>三条跑道 · 双弯道 · 几何字标</span><span>SVG · PNG · WebP · JPG · PDF · ICO · ICNS</span></div></div></body></html>`;
  await save("preview.html",preview);
  await page.setViewportSize({width:1280,height:1000});
  await page.setContent(preview);
  await page.evaluate(()=>document.fonts.ready);
  await save("preview.png",await page.screenshot({fullPage:true}));

  await copyFile(resolve(base,"icons/app-1024.png"),resolve(root,"resources/icon.png"));
  await copyFile(resolve(base,"icons/app-256.png"),resolve(root,"website/assets/icon.png"));
  for (const name of ["favicon.svg","favicon.ico","apple-touch-icon.png"]) await copyFile(resolve(base,"web",name),resolve(root,"website/assets",name));

  const all=[];
  async function walk(directory) {
    for (const item of (await readdir(directory,{withFileTypes:true})).sort((a,b)=>a.name.localeCompare(b.name))) {
      const path=resolve(directory,item.name);
      if(item.isDirectory()) await walk(path);
      else if(!["manifest.json","sportsmeet-brand-assets.zip"].includes(item.name)) {
        const bytes=await readFile(path);
        all.push({path:relative(base,path).replaceAll("\\","/"),bytes:bytes.length,sha256:createHash("sha256").update(bytes).digest("hex")});
      }
    }
  }
  await walk(base);
  await save("manifest.json",JSON.stringify({name:identity.name,concept:identity.concept,files:all},null,2)+"\n");
  execFileSync("python3",["-c",`from pathlib import Path\nfrom zipfile import ZipFile, ZIP_DEFLATED\nroot=Path(${JSON.stringify(root)})\nbase=root/'resources/brand'\nwith ZipFile(base/'sportsmeet-brand-assets.zip','w',ZIP_DEFLATED) as z:\n for p in sorted(base.rglob('*')):\n  if p.is_file() and p.suffix!='.zip': z.write(p, 'sportsmeet-brand/'+str(p.relative_to(base)))\n z.write(root/'docs/brand-guidelines.md', 'sportsmeet-brand/brand-guidelines.md')\n`]);
  console.log(`Exported ${all.length} brand assets, manifest, ZIP; synchronized application and website icons.`);
} finally {
  await browser.close();
}
