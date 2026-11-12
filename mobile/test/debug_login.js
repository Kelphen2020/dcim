/* 调试登录请求 */
const puppeteer = require('puppeteer-core');
const fs = require('fs');
const BASE = 'http://127.0.0.1/m/';
const PASS = Buffer.from('RTJlVGVzdEAyMDI2I0F1dG8=', 'base64').toString();
const EXE = ['/snap/bin/chromium', '/usr/bin/chromium-browser'].find((p) => fs.existsSync(p));

(async () => {
  const browser = await puppeteer.launch({ executablePath: EXE, headless: 'new', args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  const page = await browser.newPage();
  await page.setViewport({ width: 390, height: 844, isMobile: true, hasTouch: true });
  page.on('request', (r) => { if (r.url().includes('/api/')) console.log('REQ', r.method(), r.url(), r.postData() || ''); });
  page.on('response', async (r) => { if (r.url().includes('/api/v1/login')) { try { console.log('LOGIN RESP', r.status(), await r.text()); } catch (e) {} } });
  page.on('pageerror', (e) => console.log('PAGEERR', String(e)));

  await page.goto(BASE, { waitUntil: 'networkidle2' });
  await new Promise((r) => setTimeout(r, 1500));

  const capQ = await page.evaluate(() => document.querySelector('.cap-q')?.innerText || '');
  console.log('CAPTCHA:', capQ);
  const nums = (capQ.match(/\d+/g) || []).map(Number);
  let ans = '';
  if (nums.length >= 2) ans = capQ.includes('+') ? String(nums[0]+nums[1]) : String(nums[0]-nums[1]);
  console.log('ANS:', ans);

  const inputs = await page.$$('.field input');
  console.log('inputs count:', inputs.length);
  await inputs[0].type('e2ebot');
  await inputs[1].type(PASS);
  await inputs[2].type(ans);
  await new Promise((r) => setTimeout(r, 300));
  const vals = await page.evaluate(() => [...document.querySelectorAll('.field input')].map(i => i.value));
  console.log('input values:', JSON.stringify(vals));

  await page.evaluate(() => {
    const b = [...document.querySelectorAll('button, .login-btn')].find(x => x.innerText.includes('登'));
    if (b) b.click(); else console.log('NO BUTTON');
  });
  await new Promise((r) => setTimeout(r, 2500));
  console.log('hash after:', await page.evaluate(() => location.hash));
  await browser.close();
})();
