/* 截屏：工作台（新液态玻璃 UI），带登录重试 */
const puppeteer = require('puppeteer-core');
const fs = require('fs');
const PASS = Buffer.from('RTJlVGVzdEAyMDI2I0F1dG8=', 'base64').toString();
const EXE = ['/snap/bin/chromium', '/usr/bin/chromium-browser'].find((p) => fs.existsSync(p));

(async () => {
  const browser = await puppeteer.launch({ executablePath: EXE, headless: 'new', args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  const page = await browser.newPage();
  await page.setViewport({ width: 390, height: 844, deviceScaleFactor: 2, isMobile: true, hasTouch: true });

  const tryLogin = async () => {
    const inputs = await page.$$('.field input');
    if (inputs.length < 3) return false;
    await inputs[0].type('e2ebot');
    await inputs[1].type(PASS);
    const q = await page.evaluate(() => document.querySelector('.cap-q')?.innerText || '');
    const n = (q.match(/\d+/g) || []).map(Number);
    if (n.length < 2) return false;
    const a = q.includes('+') ? n[0] + n[1] : q.includes('-') || q.includes('−') ? n[0] - n[1] : n[0] * n[1];
    await inputs[2].type(String(a));
    await new Promise((r) => setTimeout(r, 300));
    await page.evaluate(() => {
      const b = [...document.querySelectorAll('button, .login-btn, uni-button')].find(x => (x.innerText || '').includes('登'));
      if (b) b.click();
    });
    await new Promise((r) => setTimeout(r, 2800));
    const h = await page.evaluate(() => location.hash);
    return !h.includes('login');
  };

  await page.goto('http://127.0.0.1/m/', { waitUntil: 'networkidle2' });
  await new Promise((r) => setTimeout(r, 1500));
  let ok = await tryLogin();
  if (!ok) { await page.reload({ waitUntil: 'networkidle2' }); await new Promise(r=>setTimeout(r,1500)); ok = await tryLogin(); }
  console.log('login ok:', ok);

  await new Promise((r) => setTimeout(r, 1200));
  await page.screenshot({ path: '/tmp/m5_index_top.png' });
  await page.evaluate(() => window.scrollTo(0, 360));
  await new Promise((r) => setTimeout(r, 700));
  await page.screenshot({ path: '/tmp/m5_index_scroll.png' });
  await page.evaluate(() => { location.hash = '#/pages/alert/alert'; });
  await new Promise((r) => setTimeout(r, 1800));
  await page.screenshot({ path: '/tmp/m5_alert.png' });
  await page.evaluate(() => { location.hash = '#/pages/me/me'; });
  await new Promise((r) => setTimeout(r, 1500));
  await page.screenshot({ path: '/tmp/m5_me.png' });
  await browser.close();
  console.log('SHOTS DONE');
})();
