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
  const page = await browser.newPage({locale: "de-DE", viewport: {width: 920, height: 660}});
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
  assert.equal(await page.locator('.discover-row').count(), 4);
  assert.equal(await page.locator('.discover-row input[type=checkbox]:checked').count(), 0, 'Search starts with no selected results');
  assert.equal(await page.locator('#discover-import').isDisabled(), true);
  assert.equal(await page.locator('.discover-row.discover-existing').count(), 2, 'Both saved devices remain recognizable');
  assert.equal(await page.locator('.discover-row.discover-existing .discover-present:visible').count(), 2);
  assert.equal(await page.locator('.discover-row.discover-existing input[type=checkbox]:disabled').count(), 2);
  assert.equal(await page.locator('.discover-row.discover-existing input[type=text]:disabled').count(), 2);
  assert.equal(await page.locator('.discover-row:not(.discover-existing) .discover-present:visible').count(), 0, 'New devices must not show the existing marker');
  await page.screenshot({path: path.join(screenshots, 'synowake-discovery-existing.png'), fullPage: true});
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
  await page.locator('#schedule-add').click();
  const automaticDevice = await page.locator('#schedule-form [name=deviceId] option:checked').textContent();
  await page.locator('#days-all').click();
  await page.locator('#schedule-form [name=time]').fill('06:45');
  await page.locator('#schedule-form button[type=submit]').click();
  await page.locator('#schedule-dialog').waitFor({state: 'hidden'});
  const automatic = page.locator('.schedule-card').filter({hasText: `${automaticDevice} · Täglich · 06:45`});
  await automatic.locator('button', {hasText: 'Bearbeiten'}).click();
  await page.locator('#schedule-form [name=name]').fill('');
  for (const input of await page.locator('#weekday-options input').all()) await input.uncheck();
  for (const day of [0, 3, 1]) await page.locator(`#weekday-options input[value="${day}"]`).check();
  await page.locator('#schedule-form button[type=submit]').click();
  await page.locator('#schedule-dialog').waitFor({state: 'hidden'});
  const specificDays = page.locator('.schedule-card').filter({hasText: `${automaticDevice} · Mo, Mi, So · 06:45`});
  await specificDays.locator('button', {hasText: 'Löschen'}).click();
  await page.locator('#confirm-submit').click();
  await page.locator('#confirm-dialog').waitFor({state: 'hidden'});
  await page.setViewportSize({width: 390, height: 844});
  await page.locator('[data-tab=dashboard]').click();
  await page.screenshot({path: path.join(screenshots, 'synowake-mobile.png'), fullPage: true});
  assert.equal(networkWrites.length, 0, 'Demo must never make backend or DSM requests');
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, 'Mobile viewport must not overflow');
  console.log('PASS: Demo dashboard, multi-wake, validation, safe name rendering, device edit/delete, discovery/import, schedules, log settings and mobile layout; no production requests.');

  const production = await browser.newPage({locale: "de-DE", viewport: {width: 920, height: 760}});
  production.on('pageerror', error => errors.push(error.message));
  await production.addInitScript(() => {
    if (window !== window.top) return;
    window.SYNO = {API: {Request(config) {
      const body = new URLSearchParams({api: config.api, method: config.method, version: String(config.version), SynoToken: 'dsm-token'});
      for (const [key, value] of Object.entries(config.params)) body.set(key, JSON.stringify(value));
      fetch('/webapi/entry.cgi/SYNO.Core.TaskScheduler', {method: 'POST', body,
        headers: {'X-SYNO-TOKEN': 'dsm-token', 'X-SYNO-HASH': 'test-native-session-hash'}})
        .then(response => response.json()).then(json => config.callback(json.success, json.success ? json.data : json.error))
        .catch(() => config.callback(false, {status: 0}));
    }}};
  });
  const deviceId = 'a'.repeat(32);
  const scheduleId = 'b'.repeat(32);
  const command = `'bin/synowake' '--run-schedule' '${scheduleId}' '--token' '${'c'.repeat(64)}'`;
  const data = {user: 'tester', version: '0.1.14-0015', csrf: 'csrf-test', active: true, devices: [{id: deviceId, name: 'Arbeitsrechner', ip: '192.168.1.20', mac: 'A0:B1:C2:D3:E4:01', broadcast: '192.168.1.255', port: 9, favorite: true}], schedules: [], pendingSchedules: [], logs: [], diagnostics: [], settings: {logCenterPort: 0}};
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
  await production.route('**/webapi/entry.cgi?*', route => route.fulfill({contentType: 'application/json', body: success({'SYNO.Core.TaskScheduler': {path: 'entry.cgi', minVersion: 1, maxVersion: 3, requestFormat: 'JSON'}})}));
  await production.route('**/webapi/entry.cgi/SYNO.Core.TaskScheduler', async route => {
    const body = new URLSearchParams(route.request().postData());
    const method = body.get('method');
    const parsed = Object.fromEntries([...body].filter(([key]) => key !== 'SynoToken').map(([key, value]) => [key, ['api', 'method', 'version'].includes(key) ? value : JSON.parse(value)]));
    assert.equal(route.request().headers()['x-syno-token'], 'dsm-token');
    assert.equal(route.request().headers()['x-syno-hash'], 'test-native-session-hash', 'All scheduler operations must use the DSM session transport');
    assert.equal(body.get('SynoToken'), 'dsm-token');
    calls.push(parsed);
    let result = {};
    // Match DSM 7.1: the catalog advertises 3, but list exists at version 2.
    if (method === 'list' && parsed.version !== '2') {
      await route.fulfill({contentType: 'application/json', body: JSON.stringify({success: false, error: {code: 103}})}); return;
    }
    if (['create', 'get', 'set'].includes(method) && parsed.version !== '2') {
      await route.fulfill({contentType: 'application/json', body: JSON.stringify({success: false, error: {code: 104}})}); return;
    }
    if (method === 'create') { assert.equal(parsed.version, '2'); assert.equal(parsed.owner, 'tester'); assert.equal(parsed.type, 'script'); assert.equal(parsed.extra.script, command); assert.equal(parsed.schedule.repeat_date, 1002); assert.equal(tasks.size, 0); tasks.set(42, {...parsed, id: 42}); result = {id: 42}; }
    if (method === 'get') result = tasks.get(Number(parsed.id));
    if (method === 'set') tasks.set(Number(parsed.id), {...parsed, type: 'script'});
    if (method === 'list') result = {tasks: [...tasks.values()].map(task => ({id: task.id, name: task.name, owner: task.owner, real_owner: task.real_owner})), total: tasks.size};
    if (method === 'delete') { assert.equal(parsed.version, '2'); tasks.delete(parsed.tasks[0].id); }
    await route.fulfill({contentType: 'application/json', body: success(result)});
  });
  let statusReply = {devices: [{id: deviceId, status: 'offline'}]};
  await production.route('**/api.cgi?action=*', async route => {
    const request = route.request();
    assert.equal(request.headers()['x-syno-token'], 'dsm-token');
    const action = new URL(request.url()).searchParams.get('action');
    const payload = request.postDataJSON() || {};
    if (request.method() === 'POST') assert.equal(request.headers()['x-synowake-csrf'], 'csrf-test');
    let result = {};
    if (action === 'state') result = data;
    if (action === 'status') result = statusReply;
    if (action === 'discover') {
      assert.ok(['192.167.178.0/24', '192.167.178.64/26'].includes(payload.cidr)); searches.push(payload.cidr); data.discoveryCidr = payload.cidr;
      const existing = data.devices.some(device => device.mac === '02:11:22:33:44:55');
      result = {devices: [{name: 'Entdeckt <strong>PC</strong>.fritz.box', ip: existing ? '192.167.178.71' : '192.167.178.70', mac: existing ? '02-11-22-33-44-55' : '02:11:22:33:44:55', type: 'LAN-Gerät', broadcast: '192.167.178.255'}], warnings: []};
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
  for (const asset of ['app.js', 'scheduler.js', 'i18n.js', 'translations.js', 'assets/synowake.css']) assert.ok(assetRequests.some(url => url.endsWith(`${asset}?v=0.1.14-0015`)), `Cache version missing for ${asset}`);
  await production.locator('.status.offline').first().waitFor();
  statusReply = {devices: [{id: deviceId, status: 'unknown', probeMethod: 'tcp', error: 'Keine TCP-Antwort. Status unbekannt.'}]};
  await production.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
  await production.locator('#device-tiles .status.unknown').waitFor();
  assert.match(await production.locator('.device-card').getAttribute('title'), /Keine TCP-Antwort/);
  statusReply = {devices: [{id: deviceId, status: 'online', probeMethod: 'tcp'}]};
  await production.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
  await production.locator('#device-tiles .status.online').waitFor();
  assert.equal(await production.locator('.device-card').getAttribute('title'), 'Erreichbarkeit per TCP geprüft.', 'A recovered status must clear the old error');
  statusReply = {devices: [{id: deviceId, status: 'offline'}]};
  await production.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
  await production.locator('#device-tiles .status.offline').waitFor();
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
  await production.locator('#discover-dialog').waitFor({state: 'visible'});
  await cidrField.fill('192.167.178.64/26'); await production.locator('#discover-form button').click();
  await production.locator('.discover-row').first().waitFor();
  assert.equal(await production.locator('.discover-row.discover-existing').count(), 1, 'Saved MAC recognized despite changed IP and MAC formatting');
  assert.equal(await production.locator('.discover-row .discover-present').isVisible(), true);
  assert.equal(await production.locator('.discover-row input[type=checkbox]').isDisabled(), true);
  assert.equal(await production.locator('.discover-row input[type=checkbox]').isChecked(), false);
  assert.equal(await production.locator('.discover-row input[type=text]').inputValue(), 'Büro-PC', 'Existing row uses the saved name');
  assert.equal(await production.locator('.discover-row input[type=text]').isDisabled(), true);
  assert.equal(imports.length, 1, 'Repeated search must not create a duplicate');
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
  await production.locator('#schedule-form [name=name]').fill('   ');
  await production.locator('#schedule-form button[type=submit]').click();
  await production.locator('#schedule-dialog').waitFor({state: 'hidden'});
  assert.equal(tasks.size, 1);
  assert.equal(data.schedules[0].taskId, 42);
  assert.equal(data.schedules[0].name, 'Arbeitsrechner · Mo–Fr · 08:00');
  assert.equal(tasks.get(42).name, 'SynoWake: Arbeitsrechner · Mo–Fr · 08:00', 'Blank names must reach DSM with device, selected days and time');
  assert.deepEqual(calls.filter(call => call.method === 'create').map(call => call.version), ['3', '2'], 'Unsupported create version is rejected before exactly one compatible creation');
  await production.locator('.schedule-card .toggle input').uncheck();
  await production.locator('.schedule-card').filter({hasText: 'Pausiert'}).waitFor();
  assert.equal(tasks.get(42).enable, false, 'Enable toggle must update DSM task');
  assert.deepEqual(calls.filter(call => call.method === 'set').map(call => call.version), ['3', '2']);
  tasks.get(42).extra.script = 'foreign command';
  await production.locator('.schedule-card button', {hasText: 'Bearbeiten'}).click();
  await production.locator('#schedule-form [name=time]').fill('08:30');
  await production.locator('#schedule-form button[type=submit]').click();
  await production.locator('#schedule-form-error').filter({hasText: 'stimmt nicht'}).waitFor();
  assert.deepEqual(calls.filter(call => call.method === 'set').map(call => call.version), ['3', '2'], 'Foreign script must never trigger another modification');
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
  assert.ok(calls.some(call => call.method === 'list'));
  assert.ok(calls.filter(call => call.method === 'list').every(call => call.version === '2'), 'Recovery and deletion use the native DSM 7.1 list version');
  assert.equal(calls.filter(call => call.method === 'delete').length, 1, 'The owned DSM task is deleted exactly once');
  assert.deepEqual(errors, []);
  await production.route('**/native-session-child', route => route.fulfill({contentType: 'text/html', body: '<!doctype html><title>Session child</title>'}));
  const childNavigation = production.waitForEvent('framenavigated', {predicate: frame => frame.url().endsWith('/native-session-child')});
  await production.evaluate(() => {
    const iframe = document.createElement('iframe'); iframe.src = '/native-session-child'; iframe.hidden = true; document.body.append(iframe);
  });
  await childNavigation;
  const child = production.frames().find(frame => frame.url().endsWith('/native-session-child'));
  const childResult = await child.evaluate(async baseURL => {
    if (window.SYNO?.API?.Request) throw new Error('The child must obtain the session API from its DSM parent');
    const {DsmScheduler} = await import(`${baseURL}/scheduler.js`);
    return new DsmScheduler().call('list', {offset: 0, limit: 100});
  }, base);
  assert.deepEqual(childResult, {tasks: [], total: 0}, 'An embedded iframe must use its parent DSM session API');
  console.log('PASS: Production CGI headers/CSRF, native DSM session transport for create/get/set/list/delete and embedded iframe, foreign task guard, commit recovery and persisted pending recovery.');
  const english = await browser.newPage({locale: 'en-US', viewport: {width: 920, height: 760}});
  english.on('pageerror', error => errors.push(error.message));
  await english.goto(`${base}/?demo=1`);
  await english.locator('.device-card').first().waitFor();
  assert.equal(await english.locator('html').getAttribute('lang'), 'en');
  assert.match(await english.locator('#connection-text').innerText(), /sample data/);
  assert.equal((await english.locator('.tab').first().innerText()).replace(/\s+/g, ' '), '▦ Overview');
  assert.match(await english.locator('.device-card').first().innerText(), /Work PC/);
  await english.locator('#dashboard-add').click();
  assert.equal(await english.locator('#device-dialog-title').innerText(), 'Add device');
  assert.equal(await english.locator('#device-form input[name=name]').getAttribute('placeholder'), 'For example, work PC');
  const unchangedName = 'Name enthält Steuerzeichen. {0} <PC> 100%';
  await english.locator('#device-form input[name=name]').fill(unchangedName);
  await english.locator('#device-form input[name=ip]').fill('192.168.1.99');
  await english.locator('#device-form input[name=mac]').fill('02:11:22:33:44:99');
  await english.locator('#device-form button[type=submit]').click();
  await english.locator('#device-dialog').waitFor({state: 'hidden'});
  await english.locator('[data-tab=devices]').click();
  assert.equal(await english.locator('.device-name').filter({hasText: unchangedName}).count(), 1, 'Language selection must not translate user names');
  assert.equal(await english.locator('#device-search').getAttribute('placeholder'), 'Search name, IP or MAC');
  await english.locator('#discover-open').click();
  assert.match(await english.locator('#discover-dialog').innerText(), /Detected NAS network/);
  await english.locator('#discover-form button[type=submit]').click();
  await english.locator('.discover-row').first().waitFor();
  assert.match(await english.locator('#discover-progress').innerText(), /devices found/);
  assert.equal(await english.locator('.discover-row input[type=checkbox]:checked').count(), 0);
  assert.match(await english.locator('.discover-existing').first().innerText(), /Already saved/);
  await english.locator('#discover-dialog .close-dialog').first().click();
  await english.locator('[data-tab=automation]').click();
  await english.locator('#schedule-add').click();
  assert.equal(await english.locator('#schedule-dialog-title').innerText(), 'Add wake schedule');
  assert.equal(await english.locator('#schedule-form input[name=name]').getAttribute('placeholder'), 'Leave empty: device · days · time');
  await english.locator('#schedule-form input[name=time]').fill('09:15');
  await english.locator('#schedule-form button[type=submit]').click();
  await english.locator('#schedule-dialog').waitFor({state: 'hidden'});
  assert.equal(await english.locator('.schedule-info h3').filter({hasText: 'Work PC · Mon–Fri · 09:15'}).count(), 1);
  await english.setViewportSize({width: 390, height: 760});
  assert.ok(await english.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'English layout overflows on mobile');
  await english.setViewportSize({width: 920, height: 760});
  await english.screenshot({path: path.join(screenshots, 'english-automation.png'), fullPage: true});
  await english.close();
  console.log('PASS: English overview, labels, discovery, already-saved markers, optional schedule names, weekdays, mobile layout and unchanged user names.');

  const css = await browser.newPage();
  const cssResponse = css.waitForResponse(response => response.url().endsWith('/__results'));
  await css.goto(`${base}/tests/css-isolation.html`);
  await css.waitForFunction(() => /^(PASS|FAIL)/.test(document.title));
  await cssResponse;
  assert.equal(cssResult?.success, true, cssResult?.error || await css.locator('#report').textContent());
  fs.writeFileSync(path.join(screenshots, 'css-isolation-result.json'), JSON.stringify(cssResult, null, 2));
  console.log(`PASS: CSS isolation: ${cssResult.rules} scoped rules, external styles and dimensions unchanged at ${cssResult.widths.join('/')} px.`);
  console.log('PASS: Favorites persist, failed saves revert, bulk selection excludes hidden devices; discovery opt-in and local DNS names; manual network remembered after reload; versioned assets loaded.');
  console.log('PASS: Existing discovery results remain visible, show saved names and a marker, and cannot be imported twice; MAC match survives changed IP and formatting.');
}
run().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => { if (browser) await browser.close(); server.close(); });
