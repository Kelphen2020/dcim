/* PC 端 UI 审查截图：多页面 SvgIcon 渲染验证 */
const puppeteer = require('puppeteer-core');
const fs = require('fs');
const EXE = ['/snap/bin/chromium', '/usr/bin/chromium-browser'].find((p) => fs.existsSync(p));
const BASE = 'http://127.0.0.1:3000';

(async () => {
  const browser = await puppeteer.launch({ executablePath: EXE, headless: 'new', args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  const page = await browser.newPage();
  await page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 1 });
  const errors = [];
  page.on('pageerror', (e) => errors.push(String(e).slice(0, 150)));

  // API 登录拿 token，直接注入 localStorage（复用回归脚本模式）
  const cap = await (await fetch(BASE + '/api/v1/captcha')).json();
  const q = cap.data.question;
  const n = (q.match(/\d+/g) || []).map(Number);
  const ans = q.includes('+') ? n[0] + n[1] : q.includes('-') ? n[0] - n[1] : n[0] * n[1];
  const lr = await (await fetch(BASE + '/api/v1/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: 'e2ebot', password: 'E2eTest@2026#Auto', captcha: String(ans), captcha_id: cap.data.id })
  })).json();
  if (!lr.data || !lr.data.token) { console.log('LOGIN FAIL', JSON.stringify(lr).slice(0, 200)); await browser.close(); process.exit(1); }
  const token = lr.data.token;

  await page.goto(BASE + '/index.html#/login', { waitUntil: 'domcontentloaded' });
  await page.evaluate((t, u, sids) => {
    localStorage.setItem('dcim_token', t);
    localStorage.setItem('dcim_user', JSON.stringify(u));
    localStorage.setItem('dcim_site_ids', JSON.stringify(sids || []));
  }, token, lr.data.user, lr.data.site_ids);
  await page.reload({ waitUntil: 'domcontentloaded' });
  await new Promise((r) => setTimeout(r, 800));

  const shots = [
    ['#/monitor', 'pc_monitor'],
    ['#/alert-wall', 'pc_alertwall'],
    ['#/assets', 'pc_assets'],
    ['#/room-3d', 'pc_room3d'],
    ['#/duty-roster', 'pc_duty'],
  ];
  for (const [hash, name] of shots) {
    await page.goto(BASE + '/index.html' + hash, { waitUntil: 'domcontentloaded' });
    await new Promise((r) => setTimeout(r, 3500));
    const n = await page.evaluate(() => document.querySelectorAll('svg.svg-icon').length);
    const hash2 = await page.evaluate(() => location.hash);
    console.log(name, hash2, 'svg-icons:', n);
    await page.screenshot({ path: `/tmp/${name}.png` });
  }
  console.log('JS errors:', errors.length ? errors.slice(0, 5) : 'NONE');
  await browser.close();
})();
