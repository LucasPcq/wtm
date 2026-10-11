const puppeteer = require('puppeteer-core');
const { existsSync } = require('node:fs');

const [url, out, width = '1440', height = '900', target] = process.argv.slice(2);
if (!url || !out) {
  console.error('usage: shoot.sh <url> <out.png> [width] [height] [css-selector | full]');
  process.exit(2);
}
const chrome = [
  process.env.CHROME,
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/Applications/Chromium.app/Contents/MacOS/Chromium',
  '/usr/bin/google-chrome',
  '/usr/bin/chromium',
].find((p) => p && existsSync(p));
if (!chrome) {
  console.error('no Chrome found: set CHROME to its executable');
  process.exit(2);
}

(async () => {
  const browser = await puppeteer.launch({ executablePath: chrome, headless: true });
  const page = await browser.newPage();
  await page.setViewport({ width: Number(width), height: Number(height) });
  await page.emulateMediaFeatures([{ name: 'prefers-reduced-motion', value: 'reduce' }]);
  await page.goto(url, { waitUntil: 'networkidle0' });
  await page.evaluate(() => document.fonts.ready);
  const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
  if (target && target !== 'full') {
    const element = await page.$(target);
    if (!element) throw new Error(`no element matches ${target}`);
    await element.screenshot({ path: out });
  } else {
    await page.screenshot({ path: out, fullPage: target === 'full' });
  }
  await browser.close();
  console.log(`${out}  (viewport ${width}px, scrollWidth ${scrollWidth}px)`);
  if (scrollWidth > Number(width)) {
    console.error(`the page scrolls sideways: ${scrollWidth}px wide in a ${width}px viewport`);
    process.exit(1);
  }
})().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
