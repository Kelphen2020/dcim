/* M6 截图：工作台设备概览 + 资产速查增强卡 */
const puppeteer = require('puppeteer-core');
const fs = require('fs');
const PASS = Buffer.from('RTJlVGVzdEAyMDI2I0F1dG8=', 'base64').toString();
const EXE = ['/snap/bin/chromium', '/usr/bin/chromium-browser'].find((p) => fs.existsSync(p));

(async () => {
  const browser = await puppeteer.launch({ executablePath: EXE, headless: 'new', args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  const page = await browser.newPage();
  await page.setViewport({ width: 390, height: 844, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
  await page.goto('http://127.0.0.1/m/', { waitUntil: 'networkidle2' });
  await new Promise((r) => setTimeout(r, 1500));
  const tryLogin = async () => {
    const inputs = await page.$$('.field input');
    if (inputs.length < 3) return false;
    await inputs[0].type('e2ebot');
    await inputs[1].type(PASS);
    const q = await page.evaluate(() => document.querySelector('.cap-q')?.innerText || '');
    const n = (q.match(/\d+/g) || []).map(Number);
    if (n.length < 2) return false;
    const a = q.includes('+') ? String(n[0] + n[1]) : q.includes('-') || q.includes('−') ? String(n[0] - n[1]) : String(n[0] * n[1]);
    await inputs[2].type(a);
    await new Promise((r) => setTimeout(r, 400));
    await page.evaluate(() => { const b = [...document.querySelectorAll('button, .login-btn')].find((x) => x.innerText.includes('登')); if (b) b.click(); });
    await new Promise((r) => setTimeout(r, 2600));
    const h = await page.evaluate(() => location.hash);
    return !h.includes('login');
  };
  let ok = await tryLogin();
  if (!ok) { await page.reload({ waitUntil: 'networkidle2' }); await new Promise((r) => setTimeout(r, 1500)); ok = await tryLogin(); }
  if (!ok) { await page.reload({ waitUntil: 'networkidle2' }); await new Promise((r) => setTimeout(r, 1500)); ok = await tryLogin(); }
  console.log('login ok:', ok);
  // 工作台滚到设备概览
  await page.evaluate(() => window.scrollTo(0, 560));
  await new Promise((r) => setTimeout(r, 800));
  await page.screenshot({ path: '/tmp/m6_dev_overview.png' });
  // 资产速查（深链 status=1）
  await page.evaluate(() => { location.hash = '#/pages/asset/assets?status=1'; });
  await new Promise((r) => setTimeout(r, 2200));
  await page.screenshot({ path: '/tmp/m6_assets.png' });
  // 底部 TabBar 可见性
  const tab = await page.evaluate(() => ({ visible: !!document.querySelector('.glass-tabbar'), items: document.querySelectorAll('.tab-item').length }));
  console.log('资产页 TabBar:', JSON.stringify(tab));
  await browser.close();
  console.log('SHOTS DONE');
})();
