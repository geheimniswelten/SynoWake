const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {uiModuleURL} = require('./load-ui.cjs');

async function run() {
  global.window = {parent: {SYNO: {SDS: {Session: {lang: 'ger'}}}}};
  global.location = {search: ''};
  Object.defineProperty(global, 'navigator', {value: {language: 'en-US', languages: ['en-US']}, configurable: true});
  const de = await import(uiModuleURL('i18n.js', 'de'));
  assert.equal(de.language, 'de', 'DSM language takes precedence over browser language');
  assert.equal(de.t('Wird aufgeweckt'), 'Wird aufgeweckt');
  assert.equal(de.t('Mo–Fr'), 'Mo–Fr');
  const name = 'Arbeitsrechner {0} <img> 100%';
  assert.equal(de.t('Magic Packets gesendet: {0}', name), `Magic Packets gesendet: ${name}`);
  window.parent.SYNO.SDS.Session.lang = 'enu';
  const en = await import(uiModuleURL('i18n.js', 'en'));
  assert.equal(en.language, 'en');
  assert.equal(en.t('Wird aufgeweckt'), 'Waking up');
  assert.equal(en.t('Magic Packets gesendet: {0}', name), `Magic Packets sent: ${name}`, 'User names remain literal');
  assert.equal(en.t('Mo–Fr'), 'Mon–Fri');
  assert.equal(en.normalizeLanguage('de-DE'), 'de');
  assert.equal(en.normalizeLanguage('fr'), 'en');
  window.parent = {};
  navigator.languages = ['de-DE'];
  assert.equal(en.detectLanguage(), 'de', 'Direct page follows browser language');
  Object.defineProperty(window, 'parent', {get() { throw new Error('cross-origin'); }, configurable: true});
  assert.equal(en.detectLanguage(), 'de', 'Inaccessible parent uses browser language');
  location.search = '?lang=en';
  assert.equal(en.detectLanguage(), 'en');

  const catalog = JSON.parse(fs.readFileSync(path.join(__dirname, '../internal/synowake/translations.json'), 'utf8'));
  for (const [source, translated] of Object.entries(catalog)) {
    assert.ok(translated, `Empty translation: ${source}`);
    assert.deepEqual(source.match(/\{\d+\}|%[sw]/g)?.sort() || [], translated.match(/\{\d+\}|%[sw]/g)?.sort() || [], source);
  }
  window = {SynoToken: 'test-token', parent: {}};
  const {DsmScheduler} = await import(uiModuleURL('scheduler.js', 'en-errors'));
  const scheduler = new DsmScheduler();
  assert.throws(() => scheduler.decodePayload({success: false, error: {code: 105}}, 'create'), /session/);
  scheduler.info = {path: 'entry.cgi', minVersion: 1, maxVersion: 3};
  let writes = 0;
  global.fetch = async () => { writes++; return new Response('<html>DSM error</html>', {status: 200}); };
  await assert.rejects(scheduler.call('create', {name: 'SynoWake: Test'}), error => error.uncertain === true && error.invalidResponse === true && /invalid response/.test(error.message));
  assert.equal(writes, 1, 'An English malformed response must not cause a repeated create');
  scheduler.call = async () => ({id: 99, name: 'SynoWake: Test', type: 'script', owner: 'tester', real_owner: 'tester', extra: 'invalid JSON'});
  await assert.rejects(scheduler.getOwnedTask({taskId: 99, taskOwner: 'tester', command: 'fixed command'}), /invalid task details/);
  console.log(`PASS: ${Object.keys(catalog).length} translation templates, parameter parity, DSM/browser language selection, literal user names, English session errors and language-independent mutation recovery.`);
}
run().catch(error => { console.error(error); process.exitCode = 1; });
