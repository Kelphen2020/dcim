/* PC 端图标渲染程序化核验（替代 vision） */
const puppeteer = require('puppeteer-core');
const fs = require('fs');
const EXE = ['/snap/bin/chromium', '/usr/bin/chromium-browser'].find((p) => fs.existsSync(p));
const BASE = 'http://127.0.0.1:3000';
const emojiRe = /[\u{1F000}-\u{1FAFF}\u{2600}-\u{27BF}\u{2B00}-\u{2BFF}\u{FE0F}]/u;

(async () => {
  const browser = await puppeteer.launch({ executablePath: EXE, headless: 'new', args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  const page = await browser.newPage();
  await page.setViewport({ width: 1440, height: 900 });
  const cap = await (await fetch(BASE + '/api/v1/captcha')).json();
  const q = cap.data.question, n = (q.match(/\d+/g) || []).map(Number);
  const ans = q.includes('+') ? n[0] + n[1] : q.includes('-') ? n[0] - n[1] : n[0] * n[1];
  const lr = await (await fetch(BASE + '/api/v1/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: 'e2ebot', password: 'E2eTest@2026#Auto', captcha: String(ans), captcha_id: cap.data.id }) })).json();
  await page.goto(BASE + '/index.html#/login', { waitUntil: 'domcontentloaded' });
  await page.evaluate((t, u, s) => { localStorage.setItem('dcim_token', t); localStorage.setItem('dcim_user', JSON.stringify(u)); localStorage.setItem('dcim_site_ids', JSON.stringify(s || [])); }, lr.data.token, lr.data.user, lr.data.site_ids);
  await page.reload({ waitUntil: 'domcontentloaded' });
  await new Promise((r) => setTimeout(r, 800));

  let totalIcons = 0, totalEmpty = 0, totalEmoji = [];
  const shots = ['#/monitor', '#/alert-wall', '#/assets', '#/room-3d', '#/duty-roster', '#/gateways', '#/settings', '#/big-screen', '#/network-topology', '#/power-chain'];
  for (const h of shots) {
    await page.goto(BASE + '/index.html' + h, { waitUntil: 'domcontentloaded' });
    await new Promise((r) => setTimeout(r, 2200));
    const res = await page.evaluate(() => {
      const re = /[\u{1F000}-\u{1FAFF}\u{2600}-\u{27BF}\u{2B00}-\u{2BFF}\u{FE0F}]/u;
      const icons = document.querySelectorAll('svg.svg-icon');
      let empty = 0;
      icons.forEach((s) => {
        const p = s.querySelector('path');
        if (!p || !p.getAttribute('d')) empty++;
      });
      const bodyText = document.body.innerText;
      const emojiFound = [...bodyText].filter((c) => re.test(c));
      return { n: icons.length, empty, emojiFound: [...new Set(emojiFound)] };
    });
    totalIcons += res.n; totalEmpty += res.empty;
    if (res.emojiFound.length) totalEmoji.push(h + ':' + res.emojiFound.join(''));
    console.log(`${h.padEnd(22)} icons=${String(res.n).padStart(3)} empty=${res.empty} emoji=${res.emojiFound.length ? res.emojiFound.join('') : 'NONE'}`);
  }
  console.log(`\n== 总计 SVG图标 ${totalIcons} 个, 空路径 ${totalEmpty} 个, emoji残留页 ${totalEmoji.length} ==`);
  await browser.close();
})();
