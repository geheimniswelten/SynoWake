const report = document.getElementById('report');
const frame = document.getElementById('app');
const results = [];
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
const settle = () => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
function check(condition, message) { if (!condition) throw new Error(message); results.push(message); report.textContent = results.join('\n'); }
async function until(predicate) { for (let n = 0; n < 250; n++) { if (predicate()) return; await delay(20); } throw new Error('Zeitüberschreitung'); }
const properties = ['fontFamily', 'fontSize', 'lineHeight', 'color', 'backgroundColor', 'boxSizing', 'margin', 'padding', 'display', 'borderWidth', 'borderRadius', 'width', 'height', 'whiteSpace'];
function outsideSnapshot() {
  return JSON.stringify([document.documentElement, document.body, ...document.querySelectorAll('.fixture, .fixture *')].map(node => {
    const style = getComputedStyle(node), rect = node.getBoundingClientRect();
    return { tag: node.tagName, values: properties.map(property => style[property]), rect: [rect.x, rect.y, rect.width, rect.height] };
  }));
}
async function loadStyle(href) {
  const link = document.createElement('link'); link.rel = 'stylesheet'; link.href = href;
  await new Promise((resolve, reject) => { link.onload = resolve; link.onerror = reject; document.head.append(link); });
  await settle(); return link;
}
function rulesIn(rules, verifyScope = false) {
  let count = 0;
  for (const rule of rules) {
    if (rule.selectorText) {
      count++;
      if (verifyScope && !rule.selectorText.split(',').every(selector => selector.trim().startsWith('#synowake-app'))) throw new Error(`Ungeschützter Selektor: ${rule.selectorText}`);
    } else if (rule.name && rule.cssRules) {
      if (verifyScope && !rule.name.startsWith('synowake-')) throw new Error('Globaler Animationsname');
    } else if (rule.cssRules) count += rulesIn(rule.cssRules, verifyScope);
  }
  return count;
}
async function run() {
  await until(() => frame.contentDocument?.querySelectorAll('.device-card').length === 2);
  const baseline = outsideSnapshot();
  const legacy = await loadStyle('/legacy.css');
  const legacyRules = rulesIn(legacy.sheet.cssRules);
  check(outsideSnapshot() !== baseline, 'Alte CSS reproduziert Änderungen am äußeren Dokument.');
  legacy.remove(); await settle();
  check(outsideSnapshot() === baseline, 'Entfernen der alten CSS stellt die Ausgangswerte wieder her.');
  const fixed = await loadStyle('/ui/assets/synowake.css');
  check(outsideSnapshot() === baseline, 'Neue CSS verändert außerhalb von SynoWake weder Stile noch Abmessungen.');
  const fixedRules = rulesIn(fixed.sheet.cssRules, true);
  check(fixedRules >= legacyRules && fixedRules > 150, `${fixedRules} gültige CSS-Regeln, alle auf SynoWake begrenzt.`);
  const app = frame.contentDocument;
  check(app.body.id === 'synowake-app' && !document.getElementById('synowake-app'), 'Der CSS-Bereich existiert ausschließlich im iframe.');
  check(app.defaultView.getComputedStyle(app.querySelector('.device-grid')).display === 'grid', 'Die SynoWake-Oberfläche bleibt vollständig gestaltet.');
  app.querySelector('[data-tab=devices]').click();
  check(app.querySelector('#devices').hidden === false && app.querySelectorAll('#device-rows tr').length === 4, 'Gerätetab und Tabelle funktionieren.');
  app.querySelector('#device-add').click();
  const dialog = app.querySelector('#device-dialog');
  check(dialog.open && app.defaultView.getComputedStyle(dialog).borderRadius === '15px', 'Dialog außerhalb der app-shell erhält weiterhin die Anwendungsstile.');
  dialog.close();
  const summary = [];
  for (const width of [920, 760, 510, 340]) {
    frame.style.width = `${width}px`; await settle();
    const grid = app.querySelector('.device-grid');
    check(app.defaultView.getComputedStyle(grid).display === 'grid', `Responsive CSS bei ${width} px aktiv.`);
    summary.push(width);
  }
  frame.style.width = '920px'; await settle();
  check(outsideSnapshot() === baseline, 'Auch nach Tabwechsel, Dialog und allen Media-Queries bleibt das äußere Dokument unverändert.');
  app.querySelector('[data-tab=dashboard]').click();
  results.push('ERGEBNIS: ALLE PRÜFUNGEN BESTANDEN');
  report.textContent = results.join('\n'); document.title = 'PASS · SynoWake CSS-Isolation';
  await fetch('/__results', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({success: true, rules: fixedRules, widths: summary, results, userAgent: navigator.userAgent})});
}
run().catch(async error => {
  report.textContent = results.concat(`FEHLER: ${error.message}`).join('\n'); document.title = 'FAIL · SynoWake CSS-Isolation';
  await fetch('/__results', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({success: false, error: error.message, results})});
});
