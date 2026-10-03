const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const ui = path.resolve(__dirname, '../ui');
const repository = path.resolve(__dirname, '..');
const screenshots = path.resolve(repository, 'work');
fs.mkdirSync(screenshots, {recursive: true});
let cssResult;
const server = http.createServer((req, res) => {
  const url = new URL(req.url, 'http://localhost');
  if (url.pathname === '/__results' && req.method === 'POST') {
    let content = ''; req.on('data', data => { content += data; });
    req.on('end', () => { cssResult = JSON.parse(content); res.end('OK'); }); return;
  }
  const filename = url.pathname === '/' ? 'index.html' : decodeURIComponent(url.pathname).slice(1);
  const file = url.pathname === '/legacy.css' ? path.join(repository, 'tests/fixtures/legacy-style.css') : path.resolve(filename.startsWith('ui/') || filename.startsWith('tests/') ? repository : ui, filename);
  if (!file.startsWith(repository + path.sep)) { res.writeHead(403); res.end(); return; }
  if (!fs.existsSync(file)) { res.writeHead(404); res.end(); return; }
  res.setHeader('Content-Type', filename.endsWith('.js') ? 'text/javascript' : filename.endsWith('.css') ? 'text/css' : 'text/html');
  res.end(fs.readFileSync(file));
});
const errors = [];
let browser;

function success(data = {}) { return JSON.stringify({success: true, data}); }
const name = 'Testrechner <img src=x onerror=alert(1)>';

async function run() {
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const base = `http://127.0.0.1:${server.address().port}`;
  browser = await chromium.launch({headless: true, ...(process.env.BROWSER_EXECUTABLE ? {executablePath: process.env.BROWSER_EXECUTABLE} : {})});
  const page = await browser.newPage({viewport: {width: 920, height: 660}});
  page.on('pageerror', error => errors.push(error.message));
  const networkWrites = [];
  page.on('request', request => { if (/api.cgi|webapi|login.cgi/.test(request.url())) networkWrites.push(request.url()); });
  await page.goto(`${base}/?demo=1`);
  await page.locator('.device-card').first().waitFor();
  assert.equal(await page.locator('.device-card').count(), 2, 'Only favorites are dashboard tiles');
  assert.equal(await page.locator('.device-grid').evaluate(node => getComputedStyle(node).display), 'grid', 'Application stylesheet must be loaded');
  await page.screenshot({path: path.join(screenshots, 'synowake-dashboard.png'), fullPage: true});
  await page.locator('#select-all').check();
  await page.locator('#wake-selected').click();
  await page.waitForFunction(() => document.querySelectorAll('.device-card .status.waking').length === 2);
  await page.locator('[data-tab=devices]').click();
  await page.locator('#device-add').click();
  assert.equal(await page.locator('#device-form [name=favorite]').isChecked(), false, 'Manual devices default to no favorite');
  await page.locator('#device-form [name=name]').fill(name);
  await page.locator('#device-form [name=ip]').fill('192.168.1.90');
  await page.locator('#device-form [name=mac]').fill('bad');
  await page.locator('#device-form button[type=submit]').click();
  await page.locator('#device-form-error').waitFor({state: 'visible'});
  await page.locator('#device-form [name=mac]').fill('A0:B1:C2:D3:E4:09');
  await page.locator('#device-form button[type=submit]').click();
  await page.locator('#device-dialog').waitFor({state: 'hidden'});
  await page.locator('#device-search').fill('Testrechner');
  assert.equal(await page.locator('#device-rows tr').count(), 1);
  assert.equal(await page.locator('#device-rows img').count(), 0);
  assert.equal(await page.locator('#device-rows .device-name').innerText(), name);
  assert.equal(await page.locator('#device-rows input[type=checkbox]').isChecked(), false);
  await page.locator('#device-rows button', {hasText: 'Bearbeiten'}).click();
  await page.locator('#device-form [name=name]').fill('Testrechner');
  await page.locator('#device-form button[type=submit]').click();
  await page.locator('#device-dialog').waitFor({state: 'hidden'});
  await page.locator('#device-rows button', {hasText: 'Löschen'}).click();
  await page.locator('#confirm-submit').click();
  await page.locator('#confirm-dialog').waitFor({state: 'hidden'});
  await page.locator('#device-search').fill('');
  await page.locator('#discover-open').click();
  await page.locator('#discover-form button').click();
  await page.locator('.discover-row').first().waitFor();
  assert.equal(await page.locator('.discover-row').count(), 2);
  assert.equal(await page.locator('.discover-row input[type=checkbox]:checked').count(), 0, 'Search starts with no selected results');
  assert.equal(await page.locator('#discover-import').isDisabled(), true);
  assert.equal(await page.locator('.discover-row label').first().textContent(), 'Name');
  assert.equal(await page.locator('.discover-row input[type=text]').first().inputValue(), 'Wohnzimmer-PC');
  await page.locator('.discover-row input[type=text]').first().fill('Wohnzimmer');
  await page.locator('.discover-row input[type=checkbox]').first().check();
  await page.locator('#discover-import').click();
  await page.locator('#discover-dialog').waitFor({state: 'hidden'});
  assert.equal(await page.locator('#device-rows tr').count(), 5, 'Only the chosen result is imported');
  assert.equal(await page.locator('#device-rows tr').filter({hasText: 'Wohnzimmer'}).locator('input[type=checkbox]').isChecked(), false, 'Imported devices do not become favorites automatically');
  await page.locator('#device-rows tr').filter({hasText: 'Wohnzimmer'}).locator('input[type=checkbox]').check();
  await page.waitForFunction(() => document.querySelectorAll('.device-card').length === 3);
  await page.locator('[data-tab=dashboard]').click();
  await page.locator('#select-all').check();
  assert.equal(await page.locator('.device-card input:checked').count(), 3);
  await page.locator('[data-tab=devices]').click();
  await page.locator('#device-rows tr').filter({hasText: 'Wohnzimmer'}).locator('input[type=checkbox]').uncheck();
  await page.waitForFunction(() => document.querySelectorAll('.device-card').length === 2 && document.querySelector('#selection-count').textContent === '2 ausgewählt');
  await page.locator('[data-tab=automation]').click();
  await page.locator('#schedule-add').click();
  await page.locator('#schedule-form [name=name]').fill('Wochenende');
  await page.locator('#schedule-form [name=time]').fill('09:15');
  await page.locator('#days-all').click();
  await page.locator('#schedule-form [name=notify]').check();
  await page.locator('#schedule-form button[type=submit]').click();
  await page.locator('#schedule-dialog').waitFor({state: 'hidden'});
  assert.equal(await page.locator('.schedule-card').count(), 3);
  const created = page.locator('.schedule-card').filter({hasText: 'Wochenende'});
  await created.locator('.toggle input').uncheck();
  await page.waitForFunction(() => [...document.querySelectorAll('.schedule-card')].some(card => card.textContent.includes('Wochenende') && card.textContent.includes('Pausiert')));
  await page.locator('.log-settings summary').click();
  await page.locator('#settings-form [name=logCenterPort]').fill('514');
  await page.locator('#settings-form button').click();
  await page.locator('#toast').filter({hasText: 'Einstellung gespeichert'}).waitFor();
  await page.locator('#log-retry').click();
  await page.locator('#toast').filter({hasText: 'Übertragung simuliert'}).waitFor();
  await page.screenshot({path: path.join(screenshots, 'synowake-automation.png'), fullPage: true});
  await created.locator('button', {hasText: 'Löschen'}).click();
  await page.locator('#confirm-submit').click();
  await page.locator('#confirm-dialog').waitFor({state: 'hidden'});
  await page.setViewportSize({width: 390, height: 844});
  await page.locator('[data-tab=dashboard]').click();
  await page.screenshot({path: path.join(screenshots, 'synowake-mobile.png'), fullPage: true});
  assert.equal(networkWrites.length, 0, 'Demo must never make backend or DSM requests');
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, 'Mobile viewport must not overflow');
  console.log('PASS: Demo dashboard, multi-wake, validation, safe name rendering, device edit/delete, discovery/import, schedules, log settings and mobile layout; no production requests.');

  const production = await browser.newPage({viewport: {width: 920, height: 760}});
  production.on('pageerror', error => errors.push(error.message));
  const deviceId = 'a'.repeat(32);
  const scheduleId = 'b'.repeat(32);
  const command = `'bin/synowake' '--run-schedule' '${scheduleId}' '--token' '${'c'.repeat(64)}'`;
  const data = {user: 'tester', version: '0.1.5-0006', csrf: 'csrf-test', active: true, devices: [{id: deviceId, name: 'Arbeitsrechner', ip: '192.168.1.20', mac: 'A0:B1:C2:D3:E4:01', broadcast: '192.168.1.255', port: 9, favorite: true}], schedules: [], pendingSchedules: [], logs: [], diagnostics: [], settings: {logCenterPort: 0}};
  const networks = [
    {interface: 'eth0', ip: '192.167.178.21', cidr: '192.167.178.0/24', searchCidr: '192.167.178.0/24', broadcast: '192.167.178.255'},
    {interface: 'eth1', ip: '10.42.0.130', cidr: '10.42.0.128/25', searchCidr: '10.42.0.128/25', broadcast: '10.42.0.255'}
  ];
  data.networks = networks;
  const searches = [];
  const imports = [];
  const tasks = new Map();
  const calls = [];
  let prepared;
  let failCommit = false;
  let failFavorite = false;
  await production.route('**/webman/login.cgi', route => route.fulfill({contentType: 'application/json', body: JSON.stringify({SynoToken: 'dsm-token'})}));
  await production.route('**/webapi/query.cgi?*', route => route.fulfill({contentType: 'application/json', body: success({'SYNO.Core.TaskScheduler': {path: 'entry.cgi', minVersion: 1, maxVersion: 4, requestFormat: 'JSON'}})}));
  await production.route('**/webapi/entry.cgi/SYNO.Core.TaskScheduler', async route => {
    const body = new URLSearchParams(route.request().postData());
    const method = body.get('method');
    const parsed = Object.fromEntries([...body].map(([key, value]) => [key, ['api', 'method', 'version'].includes(key) ? value : JSON.parse(value)]));
    assert.equal(route.request().headers()['x-syno-token'], 'dsm-token');
    calls.push(parsed);
    let result = {};
    if (method === 'create') { assert.equal(parsed.owner, 'tester'); assert.equal(parsed.type, 'script'); assert.equal(parsed.extra.script, command); assert.equal(parsed.schedule.repeat_date, 1002); tasks.set(42, {...parsed, id: 42}); result = {id: 42}; }
    if (method === 'get') result = tasks.get(Number(parsed.id));
    if (method === 'set') tasks.set(Number(parsed.id), {...parsed, type: 'script'});
    if (method === 'list') result = {tasks: [...tasks.values()].map(task => ({id: task.id, name: task.name, owner: task.owner, real_owner: task.real_owner})), total: tasks.size};
    if (method === 'delete') { assert.equal(parsed.version, '2'); tasks.delete(parsed.tasks[0].id); }
    await route.fulfill({contentType: 'application/json', body: success(result)});
  });
  await production.route('**/api.cgi?action=*', async route => {
    const request = route.request();
    assert.equal(request.headers()['x-syno-token'], 'dsm-token');
    const action = new URL(request.url()).searchParams.get('action');
    const payload = request.postDataJSON() || {};
    if (request.method() === 'POST') assert.equal(request.headers()['x-synowake-csrf'], 'csrf-test');
    let result = {};
    if (action === 'state') result = data;
    if (action === 'status') result = {devices: [{id: deviceId, status: 'offline'}]};
    if (action === 'discover') {
      assert.ok(['192.167.178.0/24', '192.167.178.64/26'].includes(payload.cidr)); searches.push(payload.cidr); data.discoveryCidr = payload.cidr;
      result = {devices: [{name: 'Entdeckt <strong>PC</strong>.fritz.box', ip: '192.167.178.70', mac: '02:11:22:33:44:55', type: 'LAN-Gerät', broadcast: '192.167.178.255'}], warnings: []};
    }
    if (action === 'device-save') {
      assert.equal(payload.ip, '192.167.178.70'); assert.equal(payload.broadcast, '192.167.178.255');
      assert.equal(payload.favorite, false);
      imports.push(payload); result = {...payload, id: 'd'.repeat(32)}; data.devices.push(result);
    }
    if (action === 'device-favorite') {
      assert.deepEqual(Object.keys(payload).sort(), ['favorite', 'id']);
      if (failFavorite) { failFavorite = false; await route.fulfill({status: 500, contentType: 'application/json', body: JSON.stringify({success: false, error: 'Favorit konnte nicht gespeichert werden'})}); return; }
      const device = data.devices.find(device => device.id === payload.id); device.favorite = payload.favorite; result = device;
    }
    if (action === 'schedule-prepare') {
      assert.deepEqual(Object.keys(payload).sort(), ['days', 'deviceId', 'enabled', 'id', 'name', 'notify', 'time'].sort());
      prepared = {...payload, id: payload.id || scheduleId, command, taskOwner: 'tester'};
      data.pendingSchedules = [prepared]; result = {schedule: prepared, command, owner: 'tester'};
    }
    if (action === 'schedule-abort') data.pendingSchedules = [];
    if (action === 'schedule-commit') {
      if (failCommit) { failCommit = false; await route.fulfill({status: 500, contentType: 'application/json', body: JSON.stringify({success: false, error: 'Simulierter Speicherfehler'})}); return; }
      data.schedules = [{...prepared, taskId: payload.taskId, taskOwner: payload.taskOwner}]; data.pendingSchedules = [];
    }
    if (action === 'schedule-delete') data.schedules = [];
    await route.fulfill({contentType: 'application/json', body: success(result)});
  });
  await production.goto(base);
  const assetRequests = await production.evaluate(() => performance.getEntriesByType('resource').map(entry => entry.name));
  for (const asset of ['app.js', 'scheduler.js', 'assets/synowake.css']) assert.ok(assetRequests.some(url => url.endsWith(`${asset}?v=0.1.5-0006`)), `Cache version missing for ${asset}`);
  await production.locator('.status.offline').first().waitFor();
  await production.locator('[data-tab=devices]').click();
  await production.locator('#discover-open').click();
  await production.locator('#discover-dialog').waitFor({state: 'visible'});
  const cidrField = production.locator('#discover-form [name=cidr]');
  assert.equal(await cidrField.inputValue(), '192.167.178.0/24', 'NAS network must override the old first-device subnet');
  assert.equal(await production.locator('#discover-network option').count(), 2);
  await production.locator('#discover-network').selectOption('10.42.0.128/25');
  assert.equal(await cidrField.inputValue(), '10.42.0.128/25', 'Actual /25 mask must be kept');
  await production.locator('#discover-network').selectOption('192.167.178.0/24');
  await production.locator('#discover-form button').click();
  await production.locator('.discover-row').first().waitFor();
  assert.equal(await production.locator('#discover-results strong').count(), 0, 'Discovered names must remain text');
  assert.equal(await production.locator('.discover-row input[type=text]').inputValue(), 'Entdeckt <strong>PC</strong>');
  assert.equal(await production.locator('.discover-row input[type=checkbox]').isChecked(), false);
  await production.locator('.discover-row input[type=text]').fill('Büro-PC');
  await production.setViewportSize({width: 375, height: 812});
  assert.equal(await production.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);
  await production.screenshot({path: path.join(screenshots, 'synowake-discovery-mobile.png'), fullPage: true});
  await production.setViewportSize({width: 920, height: 760});
  await production.screenshot({path: path.join(screenshots, 'synowake-discovery.png'), fullPage: true});
  await production.locator('#discover-network').selectOption('10.42.0.128/25');
  assert.equal(await production.locator('.discover-row').count(), 0, 'Network switch must clear old results');
  assert.equal(await production.locator('#discover-import').isDisabled(), true);
  await production.locator('#discover-network').selectOption('192.167.178.0/24');
  await production.locator('#discover-form button').click();
  await production.locator('.discover-row').first().waitFor();
  await production.locator('.discover-row input[type=text]').fill('Büro-PC');
  await production.locator('.discover-row input[type=checkbox]').check();
  await production.locator('#discover-import').click();
  await production.locator('#discover-dialog').waitFor({state: 'hidden'});
  assert.equal(imports.length, 1); assert.equal(imports[0].name, 'Büro-PC'); assert.equal(searches.length, 2);
  await production.locator('#discover-open').click();
  await cidrField.fill('192.167.178.64/26'); await production.locator('#discover-form button').click();
  await production.locator('.discover-row').first().waitFor();
  assert.equal(await production.locator('#discover-import').isDisabled(), true, 'Repeated searches reset result selection');
  await production.reload(); await production.locator('.status.offline').first().waitFor();
  await production.locator('[data-tab=devices]').click(); await production.locator('#discover-open').click();
  await production.locator('#discover-dialog').waitFor({state: 'visible'});
  assert.equal(await cidrField.inputValue(), '192.167.178.64/26', 'Successful manual range survives page reload');
  await production.locator('#discover-dialog .close-dialog').first().click();
  const favorite = production.locator('#device-rows tr').filter({hasText: 'Büro-PC'}).locator('input[type=checkbox]');
  failFavorite = true; await favorite.check();
  await production.locator('#toast').filter({hasText: 'Favorit konnte nicht gespeichert'}).waitFor();
  assert.equal(await favorite.isChecked(), false, 'Failed favorite persistence must restore the checkbox');
  await favorite.check(); await production.waitForFunction(() => document.querySelectorAll('.device-card').length === 2);
  await production.reload(); await production.waitForFunction(() => document.querySelectorAll('.device-card').length === 2);
  await production.locator('#select-all').check();
  await production.locator('[data-tab=devices]').click(); await favorite.uncheck();
  await production.waitForFunction(() => document.querySelectorAll('.device-card').length === 1 && document.querySelector('#selection-count').textContent === '1 ausgewählt');
  await production.locator('#device-rows tr').filter({hasText: 'Arbeitsrechner'}).locator('input[type=checkbox]').uncheck();
  await production.waitForFunction(() => document.querySelectorAll('.device-card').length === 0 && document.querySelector('#select-all').disabled);
  assert.equal(await production.locator('#device-rows tr').count(), 2, 'Unfavoriting does not delete devices');
  await production.locator('#device-rows tr').filter({hasText: 'Arbeitsrechner'}).locator('input[type=checkbox]').check();
  await production.waitForFunction(() => document.querySelectorAll('.device-card').length === 1);
  data.networks = [];
  data.discoveryCidr = '';
  await production.reload(); await production.locator('.status.offline').first().waitFor();
  await production.locator('[data-tab=devices]').click(); await production.locator('#discover-open').click();
  await production.locator('#discover-dialog').waitFor({state: 'visible'});
  assert.equal(await cidrField.inputValue(), '', 'No hardcoded or first-device network fallback allowed');
  assert.equal(await production.locator('#discover-network-field').isVisible(), false);
  await production.locator('#discover-dialog .close-dialog').first().click();
  data.networks = networks;
  console.log('PASS: NAS network autofill without device heuristic, /25 selection, connected nonprivate discovery/import with broadcast, stale-result clearing and empty-inventory fallback; mobile layout.');
  await production.locator('[data-tab=automation]').click();
  await production.locator('#schedule-add').click();
  await production.locator('#schedule-form [name=name]').fill('API-Test');
  await production.locator('#schedule-form button[type=submit]').click();
  await production.locator('#schedule-dialog').waitFor({state: 'hidden'});
  assert.equal(tasks.size, 1);
  assert.equal(data.schedules[0].taskId, 42);
  await production.locator('.schedule-card .toggle input').uncheck();
  await production.locator('.schedule-card').filter({hasText: 'Pausiert'}).waitFor();
  assert.equal(tasks.get(42).enable, false, 'Enable toggle must update DSM task');
  assert.equal(calls.filter(call => call.method === 'set').length, 1);
  tasks.get(42).extra.script = 'foreign command';
  await production.locator('.schedule-card button', {hasText: 'Bearbeiten'}).click();
  await production.locator('#schedule-form [name=time]').fill('08:30');
  await production.locator('#schedule-form button[type=submit]').click();
  await production.locator('#schedule-form-error').filter({hasText: 'stimmt nicht'}).waitFor();
  assert.equal(calls.filter(call => call.method === 'set').length, 1, 'Foreign script must never be modified');
  tasks.get(42).extra.script = command;
  failCommit = true;
  await production.locator('#schedule-form button[type=submit]').click();
  await production.locator('#schedule-form-error').filter({hasText: 'Zuordnung'}).waitFor();
  await production.locator('#schedule-dialog .close-dialog').first().click();
  await production.locator('#diagnostics button', {hasText: 'Zuordnung speichern'}).click();
  await production.locator('#toast').filter({hasText: 'zugeordnet'}).waitFor();
  assert.equal(data.schedules[0].time, '08:30');
  assert.equal(tasks.get(42).schedule.minute, 30);
  // Persisted pending state after reload is recovered by checking real task ownership and script.
  prepared = {...data.schedules[0], time: '09:10'}; data.pendingSchedules = [prepared]; data.schedules[0].pending = true;
  await production.reload();
  await production.locator('#diagnostics button', {hasText: 'Vorbereitung abschließen'}).click();
  await production.locator('#toast').filter({hasText: 'Vorbereitete Automatik'}).waitFor();
  assert.equal(tasks.get(42).schedule.hour, 9);
  assert.equal(tasks.get(42).schedule.minute, 10);
  await production.locator('[data-tab=automation]').click();
  await production.locator('.schedule-card button', {hasText: 'Löschen'}).click();
  await production.locator('#confirm-submit').click();
  await production.locator('#confirm-dialog').waitFor({state: 'hidden'});
  assert.equal(tasks.size, 0);
  assert.equal(data.schedules.length, 0);
  assert.deepEqual(errors, []);
  console.log('PASS: Production CGI headers/CSRF, TaskScheduler JSON discovery/create/get/set/delete, sanitized toggle payload, foreign task guard, commit recovery and persisted pending recovery.');
  const css = await browser.newPage();
  const cssResponse = css.waitForResponse(response => response.url().endsWith('/__results'));
  await css.goto(`${base}/tests/css-isolation.html`);
  await css.waitForFunction(() => /^(PASS|FAIL)/.test(document.title));
  await cssResponse;
  assert.equal(cssResult?.success, true, cssResult?.error || await css.locator('#report').textContent());
  fs.writeFileSync(path.join(screenshots, 'css-isolation-result.json'), JSON.stringify(cssResult, null, 2));
  console.log(`PASS: CSS isolation: ${cssResult.rules} scoped rules, external styles and dimensions unchanged at ${cssResult.widths.join('/')} px.`);
  console.log('PASS: Favorites persist, failed saves revert, bulk selection excludes hidden devices; discovery opt-in and local DNS names; manual network remembered after reload; versioned assets loaded.');
}
run().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => { if (browser) await browser.close(); server.close(); });
