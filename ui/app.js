import {DsmScheduler, getSynoToken} from './scheduler.js?v=0.1.5-0006';

const $ = selector => document.querySelector(selector);
const uiVersion = '0.1.5-0006';
const demo = new URLSearchParams(location.search).get('demo') === '1';
const scheduler = new DsmScheduler();
const selected = new Set();
const pendingCommits = new Map();
const busyDevices = new Set();
const busyFavorites = new Set();
const weekdays = ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'];
let state = {devices: [], schedules: [], logs: [], diagnostics: []};
let dsmToken = '';
let toastTimer;
let refreshing = false;
let confirmAction;
let scanResults = [];
let lastDiscoveryNetwork = '';

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = String(text);
  return node;
}

function button(text, className, action) {
  const node = element('button', `button ${className}`, text);
  node.type = 'button';
  node.addEventListener('click', action);
  return node;
}

function toast(message, error = false) {
  const target = $('#toast');
  clearTimeout(toastTimer);
  target.textContent = message;
  target.className = `toast${error ? ' error' : ''}`;
  target.hidden = false;
  toastTimer = setTimeout(() => { target.hidden = true; }, error ? 14000 : 6000);
}

function formError(selector, error) {
  const node = $(selector);
  node.textContent = error?.message || String(error || '');
  node.hidden = !error;
}

function connected(ok, text) {
  $('#connection-dot').className = `connection-dot ${ok ? 'connected' : 'error'}`;
  $('#connection-text').textContent = text || (ok ? 'Mit DiskStation verbunden' : 'Verbindung unterbrochen');
}

async function api(action, payload) {
  if (demo) return demoApi(action, payload);
  const headers = {'X-SYNO-TOKEN': dsmToken};
  const options = {credentials: 'same-origin', cache: 'no-store', headers, signal: AbortSignal.timeout(action === 'discover' ? 120000 : 30000)};
  if (payload !== undefined) {
    options.method = 'POST';
    headers['Content-Type'] = 'application/json';
    headers['X-SynoWake-CSRF'] = state.csrf || '';
    options.body = JSON.stringify(payload);
  }
  let response;
  try { response = await fetch(`api.cgi?action=${encodeURIComponent(action)}`, options); }
  catch (_) { throw new Error(action === 'discover' ? 'Die Netzwerksuche konnte nicht beendet werden. Bitte ein kleineres Netz versuchen.' : 'Die DiskStation ist nicht erreichbar. Bitte Verbindung und Paketstatus prüfen.'); }
  let json;
  try { json = await response.json(); } catch (_) { throw new Error(`Die DiskStation hat keine gültige Antwort geliefert (HTTP ${response.status}).`); }
  if (!response.ok || json.success !== true) {
    const detail = typeof json.error === 'string' ? json.error : json.error?.message;
    throw new Error(detail || `Die Anfrage wurde abgelehnt (HTTP ${response.status}).`);
  }
  return json.data ?? {};
}

async function refreshState() {
  const data = await api('state');
  for (const device of data.devices || []) {
    const previous = state.devices.find(item => item.id === device.id);
    if (!device.status && previous?.status) device.status = previous.status;
    if (device.wakeState === 'waking' && Date.now() - new Date(device.lastWake).getTime() < 90000) device.status = 'waking';
  }
  state = {...state, ...data, devices: data.devices || [], schedules: data.schedules || [], logs: data.logs || [], diagnostics: data.diagnostics || []};
  for (const id of selected) if (!favoriteDevices().some(device => device.id === id)) selected.delete(id);
  render();
  connected(true, demo ? 'Lokale Vorschau · Beispieldaten' : undefined);
}

function statusBadge(status) {
  const names = {online: 'Online', offline: 'Offline', waking: 'Wird aufgeweckt', unknown: 'Unbekannt'};
  const value = names[status] ? status : 'unknown';
  const badge = element('span', `status ${value}`);
  badge.append(element('span', 'status-dot'), document.createTextNode(names[value]));
  return badge;
}

function deviceStatus(device) { return busyDevices.has(device.id) ? 'waking' : device.status || 'unknown'; }
function favoriteDevices() { return state.devices.filter(device => device.favorite === true); }

function render() {
  renderDiagnostics();
  renderSummary();
  renderTiles();
  renderDevices();
  renderSchedules();
  renderLogs();
  updateSelection();
  $('#user-label').textContent = state.user ? `Angemeldet als ${state.user}` : '';
  $('#version-label').textContent = state.version ? `· ${state.version}` : '';
  $('#schedule-add').disabled = !state.devices.length;
  if (document.activeElement !== $('#settings-form').elements.namedItem('logCenterPort')) setField($('#settings-form'), 'logCenterPort', state.settings?.logCenterPort || 0);
}

function renderDiagnostics() {
  const container = $('#diagnostics');
  container.replaceChildren();
  if (demo) container.append(element('div', 'notice info', 'DEMO · Diese Vorschau verwendet Beispieldaten. Aufwecken und Automatiken werden nur im Browser simuliert.'));
  if (state.version && state.version !== uiVersion) container.append(element('div', 'notice warning', `Oberfläche ${uiVersion}, Paketdienst ${state.version}: Bitte das Paket aktualisieren bzw. neu starten und DSM vollständig neu laden.`));
  if (state.active === false) container.append(element('div', 'notice warning', 'Das SynoWake-Paket ist angehalten. Bitte im DSM-Paketzentrum starten, bevor Geräte aufgeweckt oder Automatiken ausgeführt werden.'));
  for (const item of state.diagnostics.filter(value => typeof value === 'string' || value.level !== 'info').slice(0, 4)) {
    container.append(element('div', `notice ${item.level === 'error' ? 'error' : 'warning'}`, typeof item === 'string' ? item : item.message));
  }
  for (const pending of pendingCommits.values()) {
    const warning = element('div', 'notice warning');
    warning.append(element('span', '', `Die DSM-Aufgabe „${pending.name}“ wurde gespeichert, ihre Zuordnung in SynoWake fehlt noch.`));
    warning.append(button('Zuordnung speichern', 'secondary compact', async event => {
      const submit = event.currentTarget;
      submit.disabled = true;
      try { await commitPending(pending.id); await refreshState(); toast('Automatik erfolgreich zugeordnet.'); }
      catch (error) { toast(error.message, true); submit.disabled = false; }
    }));
    container.append(warning);
  }
  for (const candidate of (state.pendingSchedules || []).filter(schedule => !pendingCommits.has(schedule.id))) {
    const warning = element('div', 'notice warning');
    warning.append(element('span', '', `Die Vorbereitung für „${candidate.name}“ wurde unterbrochen. SynoWake prüft die vorhandene DSM-Aufgabe, bevor die Automatik abgeschlossen wird.`));
    warning.append(button('Vorbereitung abschließen', 'secondary compact', async event => {
      const submit = event.currentTarget; submit.disabled = true;
      try {
        const saved = demo ? {taskId: candidate.taskId || 123, taskOwner: state.user} : await scheduler.recover(candidate);
        if (saved) { pendingCommits.set(candidate.id, {id: candidate.id, name: candidate.name, ...saved}); await commitPending(candidate.id); }
        else await api('schedule-abort', {id: candidate.id});
        await refreshState(); toast(saved ? 'Vorbereitete Automatik erfolgreich im DSM gespeichert und zugeordnet.' : 'Keine zugehörige DSM-Aufgabe gefunden. Die Vorbereitung wurde zurückgesetzt; bitte die Automatik erneut speichern.');
      } catch (error) { toast(error.message, true); submit.disabled = false; }
    }));
    container.append(warning);
  }
}

function renderSummary() {
  const counts = {online: 0, offline: 0, waking: 0};
  for (const device of favoriteDevices()) if (deviceStatus(device) in counts) counts[deviceStatus(device)]++;
  const container = $('#summary');
  container.replaceChildren();
  for (const [status, label] of [['online', 'online'], ['offline', 'offline'], ['waking', 'wird aufgeweckt']]) {
    const chip = element('div', `summary-chip ${status}`);
    const dot = element('span', `status-dot ${status}`);
    dot.style.background = {online: '#239372', offline: '#a0adb7', waking: '#e0a63b'}[status];
    chip.append(dot, element('strong', '', counts[status]), document.createTextNode(label));
    container.append(chip);
  }
}

function empty(container, title, copy, actionText, action) {
  const node = element('div', 'empty-state');
  node.append(element('strong', '', title), element('div', '', copy));
  if (action) node.append(button(actionText, 'primary', action));
  container.append(node);
}

function renderTiles() {
  const container = $('#device-tiles');
  container.replaceChildren();
  if (!state.devices.length) { empty(container, 'Das erste Gerät wartet.', 'Hinterlege Name, IP und MAC oder suche im lokalen Netzwerk.', 'Gerät hinzufügen', () => openDevice()); return; }
  const favorites = favoriteDevices();
  if (!favorites.length) { empty(container, 'Welche Geräte möchtest du schnell aufwecken?', 'Markiere in der Geräteliste deine Favoriten. Nur diese erscheinen hier als Kacheln.', 'Favoriten auswählen', () => $('[data-tab=devices]').click()); return; }
  for (const device of favorites) {
    const card = element('article', `device-card${selected.has(device.id) ? ' selected' : ''}`);
    const top = element('div', 'card-top');
    top.append(element('span', 'device-icon', /server|nas/i.test(device.name) ? '▥' : '▣'));
    const checkbox = element('input'); checkbox.type = 'checkbox'; checkbox.checked = selected.has(device.id);
    checkbox.setAttribute('aria-label', `${device.name} auswählen`);
    checkbox.addEventListener('change', () => { checkbox.checked ? selected.add(device.id) : selected.delete(device.id); card.classList.toggle('selected', checkbox.checked); updateSelection(); });
    top.append(checkbox);
    const text = element('div', 'card-text');
    text.append(element('h3', '', device.name), element('div', 'address', device.ip), statusBadge(deviceStatus(device)));
    const bottom = element('div', 'card-footer');
    const wake = element('button', 'wake-button'); wake.type = 'button';
    wake.append(element('span', 'power-icon', '⏻'), document.createTextNode('Aufwecken'));
    wake.disabled = deviceStatus(device) === 'waking';
    wake.addEventListener('click', () => wakeDevices([device.id]));
    const edit = element('button', 'icon-button', '⋯'); edit.type = 'button'; edit.setAttribute('aria-label', `${device.name} bearbeiten`); edit.addEventListener('click', () => openDevice(device));
    bottom.append(wake, edit);
    card.append(top, text, bottom);
    if (device.error) card.title = device.error;
    container.append(card);
  }
}

function updateSelection() {
  const count = selected.size;
  const total = favoriteDevices().length;
  $('#selection-count').textContent = count ? `${count} ausgewählt` : '';
  $('#wake-selected').disabled = !count || [...selected].every(id => busyDevices.has(id) || state.devices.find(device => device.id === id)?.status === 'waking');
  $('#select-all').checked = count > 0 && count === total;
  $('#select-all').indeterminate = count > 0 && count < total;
  $('#select-all').disabled = !total;
}

function renderDevices() {
  const query = $('#device-search').value.trim().toLocaleLowerCase('de');
  const devices = state.devices.filter(device => [device.name, device.ip, device.mac].some(value => String(value).toLocaleLowerCase('de').includes(query)));
  $('#device-count').textContent = `${devices.length} ${devices.length === 1 ? 'Gerät' : 'Geräte'}`;
  const tbody = $('#device-rows'); tbody.replaceChildren();
  if (!devices.length) {
    const row = element('tr'); const cell = element('td', 'muted', query ? 'Keine passenden Geräte gefunden.' : 'Noch keine Geräte hinterlegt.'); cell.colSpan = 6; row.append(cell); tbody.append(row); return;
  }
  for (const device of devices) {
    const row = element('tr');
    const favorite = element('input'); favorite.type = 'checkbox'; favorite.checked = device.favorite === true;
    favorite.disabled = busyFavorites.has(device.id);
    favorite.title = 'Als Kachel in Übersicht anzeigen';
    favorite.setAttribute('aria-label', `${device.name}: Als Kachel in Übersicht anzeigen`);
    favorite.addEventListener('change', async () => {
      const value = favorite.checked;
      busyFavorites.add(device.id); favorite.disabled = true;
      try {
        await api('device-favorite', {id: device.id, favorite: value});
        await refreshState();
      } catch (error) { favorite.checked = device.favorite === true; toast(error.message, true); }
      finally { busyFavorites.delete(device.id); renderDevices(); }
    });
    const favoriteCell = element('td'); favoriteCell.append(favorite); row.append(favoriteCell);
    row.append(element('td', 'device-name', device.name), element('td', 'monospace', device.ip), element('td', 'monospace', device.mac));
    const status = element('td'); status.append(statusBadge(deviceStatus(device))); row.append(status);
    const actions = element('td'); const group = element('div', 'row-actions');
    const wake = button('Aufwecken', 'quiet', () => wakeDevices([device.id])); wake.disabled = deviceStatus(device) === 'waking';
    group.append(wake, button('Bearbeiten', 'quiet', () => openDevice(device)), button('Löschen', 'quiet', () => deleteDevice(device)));
    actions.append(group); row.append(actions); tbody.append(row);
  }
}

function dayDescription(days) {
  if (days?.length === 7) return 'Täglich';
  if (JSON.stringify([...(days || [])].sort()) === '[1,2,3,4,5]') return 'Mo–Fr';
  return [...(days || [])].sort((a, b) => (a || 7) - (b || 7)).map(day => weekdays[day]).join(', ');
}

function renderSchedules() {
  const list = $('#schedule-list'); list.replaceChildren();
  if (!state.schedules.length) { empty(list, 'Deine Geräte dürfen ausschlafen.', 'Lege eine Weckzeit für jeden Tag oder ausgewählte Wochentage an.', state.devices.length ? 'Automatik anlegen' : 'Zuerst Gerät hinzufügen', () => state.devices.length ? openSchedule() : openDevice()); return; }
  for (const schedule of state.schedules) {
    const card = element('article', `schedule-card${schedule.enabled ? '' : ' schedule-disabled'}`);
    card.append(element('div', 'schedule-time', schedule.time));
    const info = element('div', 'schedule-info'); info.append(element('h3', '', schedule.name));
    const meta = element('div', 'schedule-meta');
    meta.append(element('span', '', state.devices.find(device => device.id === schedule.deviceId)?.name || 'Gerät fehlt'), element('span', '', dayDescription(schedule.days)));
    if (schedule.notify) meta.append(element('span', '', '♧ DSM-Benachrichtigung'));
    info.append(meta); card.append(info);
    const label = element('label', 'toggle'); const toggle = element('input'); toggle.type = 'checkbox'; toggle.checked = Boolean(schedule.enabled); toggle.setAttribute('aria-label', `${schedule.name} aktivieren`);
    toggle.addEventListener('change', async () => {
      toggle.disabled = true;
      try { await saveSchedule({...schedule, enabled: toggle.checked}); await refreshState(); toast(toggle.checked ? 'Automatik aktiviert.' : 'Automatik pausiert.'); }
      catch (error) { toggle.checked = Boolean(schedule.enabled); toast(error.message, true); }
      finally { toggle.disabled = false; }
    });
    label.append(toggle, element('span', 'toggle-track'), element('span', '', schedule.enabled ? 'Aktiv' : 'Pausiert')); card.append(label);
    const actions = element('div', 'row-actions'); actions.append(button('Bearbeiten', 'quiet', () => openSchedule(schedule)), button('Löschen', 'quiet', () => deleteSchedule(schedule))); card.append(actions); list.append(card);
  }
}

function formatDate(value) {
  if (!value) return '';
  const date = new Date(typeof value === 'number' ? value * (value < 100000000000 ? 1000 : 1) : value);
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat('de-DE', {day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit'}).format(date);
}

function renderLogs() {
  const list = $('#execution-log'); list.replaceChildren();
  if (!state.logs.length) { list.append(element('div', 'log-row muted', 'Noch keine Ausführungen protokolliert.')); return; }
  for (const log of state.logs.slice(0, 16)) {
    const row = element('div', 'log-row');
    const failed = log.success === false || log.level === 'error' || log.status === 'error';
    row.append(element('span', `log-icon${failed ? ' failed' : ''}`, failed ? '!' : '✓'));
    const content = element('div', 'log-content');
    content.append(element('div', 'log-title', log.message || `${log.deviceName || 'Gerät'} ${failed ? 'konnte nicht aufgeweckt werden' : 'aufgeweckt'}`));
    const detail = [log.source === 'scheduled' || log.source === 'schedule' ? 'Automatik' : log.source === 'manual' ? 'Manuell' : log.source, log.scheduleName, log.error].filter(Boolean).join(' · ');
    if (detail) content.append(element('div', 'log-detail', detail));
    if (log.centerPending) content.append(element('div', 'log-detail center-pending', 'Protokoll-Center-Übertragung ausstehend'));
    row.append(content, element('time', 'log-date', formatDate(log.time || log.timestamp || log.createdAt))); list.append(row);
  }
  const pendingCount = state.logs.filter(log => log.centerPending).length;
  $('#log-pending').textContent = pendingCount ? `${pendingCount} Übertragung(en) ausstehend` : '';
}

async function wakeDevices(ids) {
  ids = ids.filter(id => !busyDevices.has(id) && state.devices.find(device => device.id === id)?.status !== 'waking');
  if (!ids.length) return;
  ids.forEach(id => busyDevices.add(id)); renderTiles(); renderDevices(); renderSummary(); updateSelection();
  try {
    const results = [];
    for (let offset = 0; offset < ids.length; offset += 32) {
      const result = await api('wake', {ids: ids.slice(offset, offset + 32), notify: $('#manual-notify').checked});
      results.push(...(result.results || result.devices || []));
    }
    const failures = results.filter(item => item.success === false || item.error);
    ids.forEach(id => selected.delete(id));
    toast(failures.length ? `${failures.length} Gerät(e) konnten nicht aufgeweckt werden. ${failures[0].error || ''}` : `${ids.length === 1 ? 'Magic Packet gesendet.' : `Magic Packets an ${ids.length} Geräte gesendet.`} Der Online-Status wird geprüft.`, Boolean(failures.length));
    await refreshState();
    await pollStatus();
  } catch (error) { toast(error.message, true); }
  finally { ids.forEach(id => busyDevices.delete(id)); renderTiles(); renderDevices(); renderSummary(); updateSelection(); }
}

function setField(form, name, value) { form.elements.namedItem(name).value = value ?? ''; }

function openDevice(device = {}) {
  const form = $('#device-form'); form.reset(); formError('#device-form-error');
  $('#device-dialog-title').textContent = device.id ? 'Gerät bearbeiten' : 'Gerät hinzufügen';
  for (const name of ['id', 'name', 'ip', 'mac', 'broadcast']) setField(form, name, device[name]);
  setField(form, 'port', device.port || 9);
  form.elements.namedItem('favorite').checked = device.favorite === true;
  $('#device-dialog').showModal();
}

function validIp(ip) { return /^\d{1,3}(\.\d{1,3}){3}$/.test(ip) && ip.split('.').every(part => Number(part) >= 0 && Number(part) <= 255); }

function devicePayload(form) {
  const data = Object.fromEntries(new FormData(form));
  data.favorite = form.elements.namedItem('favorite').checked;
  for (const field of ['name', 'ip', 'mac', 'broadcast']) data[field] = String(data[field] || '').trim();
  data.mac = data.mac.replace(/-/g, ':').toUpperCase(); data.port = Number(data.port);
  if (!data.name || !validIp(data.ip)) throw new Error('Bitte einen Namen und eine gültige IPv4-Adresse eingeben.');
  if (!/^([0-9A-F]{2}:){5}[0-9A-F]{2}$/.test(data.mac)) throw new Error('Bitte die MAC-Adresse als AA:BB:CC:DD:EE:FF eingeben.');
  if (data.broadcast && !validIp(data.broadcast)) throw new Error('Die Broadcast-Adresse muss eine gültige IPv4-Adresse sein.');
  if (!Number.isInteger(data.port) || data.port < 1 || data.port > 65535) throw new Error('Der UDP-Port muss zwischen 1 und 65535 liegen.');
  return data;
}

function askDelete(title, copy, action) {
  $('#confirm-title').textContent = title; $('#confirm-copy').textContent = copy;
  formError('#confirm-error'); confirmAction = action;
  $('#confirm-dialog').showModal();
}

function deleteDevice(device) {
  const related = state.schedules.filter(schedule => schedule.deviceId === device.id);
  if (related.length) { toast(`„${device.name}“ wird von ${related.length} Automatik(en) verwendet. Bitte diese zuerst löschen.`, true); return; }
  askDelete('Gerät löschen', `„${device.name}“ aus SynoWake entfernen?`, async () => { await api('device-delete', {id: device.id}); selected.delete(device.id); await refreshState(); toast('Gerät gelöscht.'); });
}

function selectDays(days) { for (const input of $('#weekday-options').querySelectorAll('input')) input.checked = days.includes(Number(input.value)); }

function openSchedule(schedule = {}) {
  if (!state.devices.length) { toast('Bitte zuerst ein Gerät hinzufügen.'); return; }
  const form = $('#schedule-form'); form.reset(); formError('#schedule-form-error');
  $('#schedule-dialog-title').textContent = schedule.id ? 'Weckzeit bearbeiten' : 'Weckzeit anlegen';
  for (const name of ['id', 'name']) setField(form, name, schedule[name]);
  const select = form.elements.namedItem('deviceId'); select.replaceChildren();
  for (const device of state.devices) { const option = element('option', '', device.name); option.value = device.id; select.append(option); }
  if (schedule.deviceId) select.value = schedule.deviceId;
  setField(form, 'time', schedule.time || '08:00');
  form.elements.namedItem('enabled').checked = schedule.enabled !== false;
  form.elements.namedItem('notify').checked = Boolean(schedule.notify);
  selectDays(schedule.days || [1, 2, 3, 4, 5]);
  $('#schedule-dialog').showModal();
}

async function commitPending(id) {
  const pending = pendingCommits.get(id);
  if (!pending) return;
  await api('schedule-commit', {id, taskId: pending.taskId, taskOwner: pending.taskOwner});
  pendingCommits.delete(id); renderDiagnostics();
}

async function saveSchedule(payload) {
  if (payload.id && pendingCommits.has(payload.id)) { await commitPending(payload.id); return; }
  const previous = state.schedules.find(schedule => schedule.id === payload.id);
  if (previous?.pending || state.pendingSchedules?.some(schedule => schedule.id === payload.id)) throw new Error('Für diese Automatik ist noch eine Vorbereitung offen. Bitte zuerst „Vorbereitung abschließen“ verwenden.');
  const clean = Object.fromEntries(['id', 'name', 'deviceId', 'time', 'days', 'enabled', 'notify'].map(key => [key, payload[key]]));
  const prepared = await api('schedule-prepare', clean);
  let saved;
  try { saved = demo ? {taskId: previous?.taskId || Math.floor(Date.now() / 1000), taskOwner: state.user} : await scheduler.upsert(prepared, previous); }
  catch (error) {
    if (!error.uncertain) { try { await api('schedule-abort', {id: prepared.schedule.id}); } catch (_) { /* Keep the original error visible. */ } }
    else await refreshState().catch(() => {});
    throw error;
  }
  const id = prepared.schedule.id;
  pendingCommits.set(id, {id, name: prepared.schedule.name, ...saved});
  if (!payload.id) setField($('#schedule-form'), 'id', id);
  try { await commitPending(id); }
  catch (error) { renderDiagnostics(); throw new Error(`Die DSM-Aufgabe wurde gespeichert, ihre Zuordnung konnte nicht gespeichert werden. Bitte „Zuordnung speichern“ verwenden. ${error.message}`); }
}

function deleteSchedule(schedule) {
  askDelete('Automatik löschen', `„${schedule.name}“ aus SynoWake und dem DSM-Aufgabenplaner entfernen?`, async () => {
    if (!demo && schedule.taskId && !schedule._taskRemoved) { await scheduler.remove(schedule); schedule._taskRemoved = true; }
    try { await api('schedule-delete', {id: schedule.id}); }
    catch (error) { throw new Error(`Die DSM-Aufgabe wurde entfernt. Die Liste konnte noch nicht aktualisiert werden; Löschen erneut ausführen. ${error.message}`); }
    await refreshState(); toast('Automatik gelöscht.');
  });
}

async function openDiscover() {
  try { await refreshState(); }
  catch (error) { toast(error.message, true); return; }
  const networks = (state.networks || []).filter(network => network.searchCidr);
  const select = $('#discover-network'); select.replaceChildren();
  for (const network of networks) {
    const option = element('option', '', `${network.interface} · ${network.ip} · ${network.cidr}`);
    option.value = network.searchCidr; select.append(option);
  }
  $('#discover-network-field').hidden = !networks.length;
  if (state.discoveryCidr && !networks.some(network => network.searchCidr === state.discoveryCidr)) {
    const option = element('option', '', `Letzter Suchbereich · ${state.discoveryCidr}`);
    option.value = state.discoveryCidr; select.append(option);
  }
  select.value = [...select.options].some(option => option.value === lastDiscoveryNetwork) ? lastDiscoveryNetwork : state.discoveryCidr || networks[0]?.searchCidr || '';
  $('#discover-form').elements.namedItem('cidr').value = select.value;
  clearDiscovery();
  if (state.networkError) $('#discover-warnings').append(element('div', 'notice error', state.networkError));
  else if (!networks.length) $('#discover-warnings').append(element('div', 'notice warning', 'Kein durchsuchbares NAS-Netz erkannt. Einen angeschlossenen IPv4-Suchbereich von /24 bis /30 eingeben.'));
  $('#discover-dialog').showModal();
}

function clearDiscovery() {
  scanResults = []; renderDiscovery();
  $('#discover-warnings').replaceChildren(); $('#discover-progress').textContent = '';
}

function renderDiscovery() {
  const target = $('#discover-results'); target.replaceChildren();
  for (const [index, device] of scanResults.entries()) {
    const row = element('div', 'discover-row');
    const check = element('input'); check.type = 'checkbox'; check.dataset.index = index; check.checked = false; check.disabled = !device.mac;
    check.setAttribute('aria-label', `${device.ip} übernehmen`); check.addEventListener('change', updateImportButton);
    const label = element('label', 'field', 'Name');
    const name = element('input'); name.type = 'text'; name.value = discoveryName(device.name) || device.type || device.ip; name.maxLength = 80; name.dataset.index = index;
    label.append(name);
    const address = element('div', 'discover-address'); address.append(element('span', '', device.ip), element('span', 'monospace', device.mac || 'Keine MAC ermittelt'));
    if (device.type) address.append(element('span', 'discover-type', device.type));
    row.append(check, label, address); target.append(row);
  }
  updateImportButton();
}

function discoveryName(name = '') {
  return String(name).trim().replace(/\.$/, '').replace(/\.(fritz\.box|local|lan)$/i, '');
}

function updateImportButton() {
  const count = $('#discover-results').querySelectorAll('input[type=checkbox]:checked:not(:disabled)').length;
  $('#discover-import').disabled = !count;
  $('#discover-import').textContent = count ? `${count} ${count === 1 ? 'Gerät' : 'Geräte'} übernehmen` : 'Auswahl übernehmen';
}

async function pollStatus() {
  if (refreshing || document.hidden || !state.csrf) return;
  refreshing = true;
  try {
    const data = await api('status');
    for (const update of data.devices || []) { const device = state.devices.find(item => item.id === update.id); if (device) Object.assign(device, update); }
    renderTiles(); renderDevices(); renderSummary(); updateSelection(); connected(true, demo ? 'Lokale Vorschau · Beispieldaten' : undefined);
  } catch (error) { connected(false); }
  finally { refreshing = false; }
}

for (const tab of document.querySelectorAll('.tab')) tab.addEventListener('click', () => {
  for (const button of document.querySelectorAll('.tab')) { button.classList.toggle('active', button === tab); button.toggleAttribute('aria-current', button === tab); if (button === tab) button.setAttribute('aria-current', 'page'); }
  for (const page of document.querySelectorAll('.page')) page.hidden = page.id !== tab.dataset.tab;
  if (tab.dataset.tab === 'automation') refreshState().catch(error => toast(error.message, true));
});
for (const close of document.querySelectorAll('.close-dialog')) close.addEventListener('click', () => { if (!close.closest('dialog').dataset.busy) close.closest('dialog').close(); });
for (const dialog of document.querySelectorAll('dialog')) dialog.addEventListener('cancel', event => { if (dialog.dataset.busy) event.preventDefault(); });
$('#dashboard-add').addEventListener('click', () => openDevice());
$('#device-add').addEventListener('click', () => openDevice());
$('#discover-open').addEventListener('click', openDiscover);
$('#discover-network').addEventListener('change', event => {
  lastDiscoveryNetwork = event.target.value;
  $('#discover-form').elements.namedItem('cidr').value = event.target.value;
  clearDiscovery();
});
$('#discover-form').elements.namedItem('cidr').addEventListener('input', clearDiscovery);
$('#schedule-add').addEventListener('click', () => openSchedule());
$('#device-search').addEventListener('input', renderDevices);
$('#select-all').addEventListener('change', event => { selected.clear(); if (event.target.checked) favoriteDevices().forEach(device => selected.add(device.id)); renderTiles(); updateSelection(); });
$('#wake-selected').addEventListener('click', () => wakeDevices([...selected]));
$('#refresh-logs').addEventListener('click', async event => { const submit = event.currentTarget; submit.disabled = true; try { await refreshState(); } catch (error) { toast(error.message, true); } finally { submit.disabled = false; } });
$('#settings-form').addEventListener('submit', async event => {
  event.preventDefault(); const submit = event.currentTarget.querySelector('button'); submit.disabled = true; formError('#settings-error');
  try { await api('settings-save', {logCenterPort: Number(event.currentTarget.elements.namedItem('logCenterPort').value)}); await refreshState(); toast('Protokoll-Center-Einstellung gespeichert.'); }
  catch (error) { formError('#settings-error', error); }
  finally { submit.disabled = false; }
});
$('#log-retry').addEventListener('click', async event => {
  const submit = event.currentTarget; submit.disabled = true; formError('#settings-error');
  try { const result = await api('log-retry', {}); await refreshState(); toast(result.message || (result.remaining ? `${result.attempted} Einträge geprüft; ${result.remaining} Übertragung(en) noch ausstehend. Bitte Empfänger und Port prüfen.` : `${result.attempted || 0} Einträge geprüft. Keine Übertragungen mehr ausstehend.`), Boolean(result.remaining)); }
  catch (error) { formError('#settings-error', error); }
  finally { submit.disabled = false; }
});
$('#days-all').addEventListener('click', () => selectDays([0, 1, 2, 3, 4, 5, 6]));
$('#days-work').addEventListener('click', () => selectDays([1, 2, 3, 4, 5]));
for (const day of [1, 2, 3, 4, 5, 6, 0]) { const label = element('label', 'weekday'); const input = element('input'); input.type = 'checkbox'; input.name = 'day'; input.value = day; label.append(input, element('span', '', weekdays[day])); $('#weekday-options').append(label); }

async function submitBusy(form, action, errorSelector) {
  const submit = form.querySelector('button[type=submit]');
  const dialog = form.closest('dialog'); submit.disabled = true; dialog.dataset.busy = 'true'; formError(errorSelector);
  try { await action(); } catch (error) { formError(errorSelector, error); }
  finally { submit.disabled = false; delete dialog.dataset.busy; }
}

$('#device-form').addEventListener('submit', event => { event.preventDefault(); submitBusy(event.currentTarget, async () => { await api('device-save', devicePayload(event.currentTarget)); await refreshState(); $('#device-dialog').close(); toast('Gerät gespeichert.'); }, '#device-form-error'); });
$('#schedule-form').addEventListener('submit', event => {
  event.preventDefault();
  submitBusy(event.currentTarget, async () => {
    const form = event.currentTarget; const payload = Object.fromEntries(new FormData(form));
    payload.days = [...form.querySelectorAll('input[name=day]:checked')].map(input => Number(input.value)); delete payload.day;
    payload.enabled = form.elements.namedItem('enabled').checked; payload.notify = form.elements.namedItem('notify').checked; payload.name = payload.name.trim();
    if (!payload.days.length) throw new Error('Bitte mindestens einen Wochentag auswählen.');
    await saveSchedule(payload); await refreshState(); $('#schedule-dialog').close(); toast('Automatik im DSM-Aufgabenplaner gespeichert.');
  }, '#schedule-form-error');
});
$('#confirm-form').addEventListener('submit', event => { event.preventDefault(); submitBusy(event.currentTarget, async () => { await confirmAction(); $('#confirm-dialog').close(); }, '#confirm-error'); });
$('#discover-form').addEventListener('submit', async event => {
  event.preventDefault(); const form = event.currentTarget; const submit = form.querySelector('button'); submit.disabled = true; $('#discover-dialog').dataset.busy = 'true';
  $('#discover-progress').textContent = 'Netzwerk wird geprüft … Das kann etwa eine Minute dauern.'; $('#discover-warnings').replaceChildren(); scanResults = []; renderDiscovery();
  try {
    const cidr = form.elements.namedItem('cidr').value.trim();
    if (!/^\d{1,3}(\.\d{1,3}){3}\/(\d|[12]\d|3[0-2])$/.test(cidr)) throw new Error('Bitte ein gültiges IPv4-Netz mit Präfix eingeben, zum Beispiel 192.168.1.0/24.');
    const data = await api('discover', {cidr}); lastDiscoveryNetwork = cidr; scanResults = data.devices || []; renderDiscovery();
    for (const warning of data.warnings || []) $('#discover-warnings').append(element('div', 'notice warning', warning.message || warning));
    $('#discover-progress').textContent = scanResults.length ? `${scanResults.length} ${scanResults.length === 1 ? 'Gerät gefunden' : 'Geräte gefunden'}. Namen vor der Übernahme anpassen.` : 'Keine Geräte gefunden. Netzbereich und Erreichbarkeit prüfen.';
  } catch (error) { $('#discover-warnings').append(element('div', 'notice error', error.message)); $('#discover-progress').textContent = ''; }
  finally { submit.disabled = false; delete $('#discover-dialog').dataset.busy; }
});
$('#discover-import').addEventListener('click', async event => {
  const submit = event.currentTarget; submit.disabled = true; $('#discover-dialog').dataset.busy = 'true';
  let count = 0;
  try {
    for (const check of $('#discover-results').querySelectorAll('input[type=checkbox]:checked:not(:disabled)')) {
      const index = Number(check.dataset.index); const device = scanResults[index];
      const name = $('#discover-results').querySelector(`input[type=text][data-index="${index}"]`).value.trim() || discoveryName(device.name) || device.type || device.ip;
      await api('device-save', {name, ip: device.ip, mac: device.mac, broadcast: device.broadcast || '', port: 9, favorite: false}); count++; check.checked = false;
    }
    await refreshState(); $('#discover-dialog').close(); toast(`${count} ${count === 1 ? 'Gerät übernommen' : 'Geräte übernommen'}.`);
  } catch (error) { $('#discover-warnings').append(element('div', 'notice error', `${count ? `${count} Gerät(e) bereits übernommen. ` : ''}${error.message}`)); await refreshState().catch(() => {}); }
  finally { delete $('#discover-dialog').dataset.busy; updateImportButton(); }
});

const demoState = {
  user: 'DSM-Demo', csrf: 'demo-only', version: '0.1.5-0006', diagnostics: [], settings: {logCenterPort: 0}, active: true,
  networks: [{interface: 'Demo-LAN', ip: '192.168.1.2', cidr: '192.168.1.0/24', searchCidr: '192.168.1.0/24', broadcast: '192.168.1.255'}],
  devices: [
    {id: 'demo1', name: 'Arbeitsrechner', ip: '192.168.1.20', mac: 'A0:B1:C2:D3:E4:01', broadcast: '192.168.1.255', port: 9, favorite: true, status: 'offline'},
    {id: 'demo2', name: 'Medien-PC', ip: '192.168.1.32', mac: 'A0:B1:C2:D3:E4:02', broadcast: '192.168.1.255', port: 9, favorite: true, status: 'online'},
    {id: 'demo3', name: 'Backup-Server', ip: '192.168.1.45', mac: 'A0:B1:C2:D3:E4:03', broadcast: '192.168.1.255', port: 9, status: 'offline'},
    {id: 'demo4', name: 'Studio', ip: '192.168.1.60', mac: 'A0:B1:C2:D3:E4:04', broadcast: '192.168.1.255', port: 9, status: 'offline'}
  ],
  schedules: [
    {id: 'demo-schedule1', name: 'Arbeitsbeginn', deviceId: 'demo1', time: '07:45', days: [1, 2, 3, 4, 5], enabled: true, notify: true, taskId: 1, taskOwner: 'DSM-Demo'},
    {id: 'demo-schedule2', name: 'Nächtliches Backup', deviceId: 'demo3', time: '02:00', days: [0, 1, 2, 3, 4, 5, 6], enabled: false, notify: false, taskId: 2, taskOwner: 'DSM-Demo'}
  ],
  logs: [
    {timestamp: new Date(Date.now() - 900000).toISOString(), message: 'Magic Packet an Arbeitsrechner gesendet', source: 'scheduled', scheduleName: 'Arbeitsbeginn', success: true},
    {timestamp: new Date(Date.now() - 4200000).toISOString(), message: 'Magic Packet an Medien-PC gesendet', source: 'manual', success: true}
  ]
};
const demoPrepared = new Map();
async function demoApi(action, payload = {}) {
  await new Promise(resolve => setTimeout(resolve, action === 'discover' ? 1000 : 120));
  switch (action) {
    case 'state': return structuredClone(demoState);
    case 'status': return {devices: structuredClone(demoState.devices)};
    case 'settings-save': demoState.settings = {...demoState.settings, ...payload}; return {};
    case 'log-retry': return {message: 'Demo: Protokoll-Center-Übertragung simuliert.'};
    case 'device-save': {
      const device = {...payload, id: payload.id || `demo-${Date.now()}-${Math.random().toString(16).slice(2, 6)}`, status: demoState.devices.find(device => device.id === payload.id)?.status || 'offline'};
      const index = demoState.devices.findIndex(item => item.id === device.id); index < 0 ? demoState.devices.push(device) : demoState.devices.splice(index, 1, device); return {device};
    }
    case 'device-favorite': {
      const device = demoState.devices.find(device => device.id === payload.id);
      if (!device) throw new Error('Gerät nicht gefunden.');
      device.favorite = payload.favorite; return device;
    }
    case 'device-delete': demoState.devices = demoState.devices.filter(device => device.id !== payload.id); return {};
    case 'wake':
      for (const id of payload.ids) {
        const device = demoState.devices.find(item => item.id === id); if (!device) continue;
        device.status = 'waking'; device.lastWake = new Date().toISOString();
        demoState.logs.unshift({timestamp: new Date().toISOString(), message: `Magic Packet an ${device.name} gesendet`, source: 'manual', success: true});
        setTimeout(() => { device.status = 'online'; pollStatus(); }, 5500);
      }
      return {};
    case 'discover': demoState.discoveryCidr = payload.cidr; return {devices: [{name: 'Wohnzimmer-PC.fritz.box', ip: '192.168.1.70', mac: 'A0:B1:C2:D3:E4:05', type: 'Computer'}, {name: '', ip: '192.168.1.80', mac: 'A0:B1:C2:D3:E4:06', type: 'Netzwerkgerät'}], warnings: []};
    case 'schedule-prepare': {
      const schedule = {...payload, id: payload.id || `demo-schedule-${Date.now()}`}; demoPrepared.set(schedule.id, schedule); return {schedule, command: 'DEMO', owner: demoState.user};
    }
    case 'schedule-commit': {
      const prepared = demoPrepared.get(payload.id); if (!prepared) throw new Error('Die vorbereitete Demo-Automatik fehlt.');
      const schedule = {...prepared, taskId: payload.taskId, taskOwner: payload.taskOwner};
      const index = demoState.schedules.findIndex(item => item.id === payload.id); index < 0 ? demoState.schedules.push(schedule) : demoState.schedules.splice(index, 1, schedule); demoPrepared.delete(payload.id); return {schedule};
    }
    case 'schedule-abort': demoPrepared.delete(payload.id); return {};
    case 'schedule-delete': demoState.schedules = demoState.schedules.filter(schedule => schedule.id !== payload.id); return {};
    default: throw new Error('Unbekannte Demo-Aktion.');
  }
}

async function boot() {
  $('#demo-badge').hidden = !demo;
  $('#boot-error').hidden = true;
  try { if (!demo) dsmToken = await getSynoToken(); await refreshState(); await pollStatus(); }
  catch (error) {
    connected(false, 'DSM-Verbindung fehlt');
    const target = $('#boot-error'); target.replaceChildren(element('span', '', error.message), button('Erneut versuchen', 'secondary compact', boot)); target.hidden = false;
    render();
  }
}
boot();
setInterval(pollStatus, 10000);
document.addEventListener('visibilitychange', () => { if (!document.hidden) pollStatus(); });
