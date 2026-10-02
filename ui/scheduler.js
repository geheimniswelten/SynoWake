// DSM's private TaskScheduler API is discovered at runtime. No crontab writes.
const API = 'SYNO.Core.TaskScheduler';
const TASK_PREFIX = 'SynoWake: ';
let cachedToken = '';

export async function getSynoToken() {
  if (cachedToken) return cachedToken;
  const candidates = [new URLSearchParams(location.search).get('SynoToken'), window.SynoToken];
  try { candidates.push(window.parent.SYNO?.SDS?.Session?.SynoToken, window.parent.SynoToken); } catch (_) { /* A cross-origin parent is inaccessible. */ }
  cachedToken = candidates.find(value => typeof value === 'string' && value.length > 0) || '';
  if (cachedToken) return cachedToken;
  const response = await fetch('/webman/login.cgi', {credentials: 'same-origin', cache: 'no-store', signal: AbortSignal.timeout(15000)});
  if (!response.ok) throw new Error('Die DSM-Sitzung konnte nicht gelesen werden. Bitte im DSM erneut anmelden.');
  let json;
  try { json = await response.json(); } catch (_) { throw new Error('DSM hat keine gültige Sitzungsantwort geliefert. Bitte SynoWake im DSM öffnen.'); }
  cachedToken = json.SynoToken || json.data?.SynoToken || '';
  if (typeof cachedToken !== 'string' || !cachedToken) throw new Error('Kein DSM-Sitzungstoken verfügbar. Bitte SynoWake aus dem DSM-Hauptmenü öffnen.');
  return cachedToken;
}

export function schedulerTiming(schedule) {
  const [hour, minute] = schedule.time.split(':').map(Number);
  const days = [...new Set(schedule.days)].sort((a, b) => a - b);
  if (!Number.isInteger(hour) || hour < 0 || hour > 23 || !Number.isInteger(minute) || minute < 0 || minute > 59 || !days.length || days.some(day => !Number.isInteger(day) || day < 0 || day > 6)) throw new Error('Uhrzeit oder Wochentage sind ungültig.');
  return {date_type: 0, monthly_week: '[]', repeat_date: days.length === 7 ? 1001 : 1002, week_day: days.join(','), hour, minute, repeat_hour: 0, repeat_min: 0, last_work_hour: hour};
}

export class DsmScheduler {
  constructor() { this.info = null; }

  async discover() {
    if (this.info) return this.info;
    const token = await getSynoToken();
    const query = new URLSearchParams({api: 'SYNO.API.Info', method: 'query', version: '1', query: API});
    const response = await fetch(`/webapi/query.cgi?${query}`, {credentials: 'same-origin', cache: 'no-store', headers: {'X-SYNO-TOKEN': token}, signal: AbortSignal.timeout(15000)});
    const data = await this.decode(response, 'API-Erkennung');
    const info = data[API];
    if (!info || !info.path || Number(info.maxVersion) < 4 || Number(info.minVersion || 1) > 4) throw new Error('DSM stellt keine unterstützte TaskScheduler-API (Version 4) bereit.');
    if (!/^[A-Za-z0-9_.\/-]+\.cgi$/.test(info.path) || info.path.includes('..')) throw new Error('DSM meldet einen unerwarteten API-Pfad.');
    this.info = info;
    return info;
  }

  async decode(response, method) {
    if (!response.ok) throw new Error(`DSM-Aufgabenplaner: HTTP ${response.status} bei ${method}.`);
    let json;
    try { json = await response.json(); } catch (_) { throw new Error(`DSM-Aufgabenplaner: ungültige Antwort bei ${method}.`); }
    if (json.success !== true) {
      const code = json.error?.code ?? 'unbekannt';
      const explanation = {104: 'Keine Berechtigung. Ein DSM-Administrator muss SynoWake öffnen.', 105: 'Keine Berechtigung für den Aufgabenplaner.', 106: 'Die DSM-Sitzung ist abgelaufen. Bitte erneut anmelden.', 107: 'Die DSM-Sitzung wurde unterbrochen. Bitte erneut anmelden.', 119: 'Die DSM-Sitzung ist ungültig. Bitte erneut anmelden.', 101: 'Diese DSM-Version unterstützt die Anfrage nicht.', 102: 'Die TaskScheduler-API ist nicht verfügbar.', 103: 'Die API-Methode wird nicht unterstützt.', 100: 'DSM hat die Parameter abgelehnt.'}[code];
      throw new Error(`DSM-Aufgabenplaner (${method}, Fehler ${code}): ${explanation || 'Die Anfrage wurde abgelehnt. Bitte DSM-Aufgabenplaner und Benutzerrechte prüfen.'}`);
    }
    return json.data ?? {};
  }

  async call(method, parameters, version = 4) {
    const info = await this.discover();
    if (Number(info.minVersion || 1) > version) throw new Error(`DSM unterstützt die benötigte API-Version ${version} für ${method} nicht.`);
    const body = new URLSearchParams({api: API, method, version: String(version)});
    const jsonFormat = info.requestFormat === 'JSON';
    for (const [key, value] of Object.entries(parameters)) body.set(key, jsonFormat || typeof value === 'object' ? JSON.stringify(value) : String(value));
    const token = await getSynoToken();
    const path = `/webapi/${info.path.replace(/^\//, '')}${jsonFormat ? `/${API}` : ''}`;
    let response;
    try { response = await fetch(path, {method: 'POST', credentials: 'same-origin', cache: 'no-store', headers: {'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8', 'X-SYNO-TOKEN': token}, body, signal: AbortSignal.timeout(20000)}); }
    catch (_) { const recovery = method === 'delete' ? 'Bitte Löschen erneut ausführen, damit SynoWake den aktuellen Aufgabenbestand prüft.' : 'Bitte „Vorbereitung abschließen“ verwenden, bevor du erneut speicherst.'; const error = new Error(`DSM-Aufgabenplaner: Verbindung bei ${method} unterbrochen. Der Ausgang ist unklar. ${recovery}`); error.uncertain = ['create', 'set', 'delete'].includes(method); throw error; }
    try { return await this.decode(response, method); }
    catch (error) { if ((!response.ok || error.message.includes('ungültige Antwort')) && ['create', 'set', 'delete'].includes(method)) error.uncertain = true; throw error; }
  }

  async getOwnedTask(schedule) {
    const id = Number(schedule.taskId);
    if (!Number.isInteger(id) || id <= 0 || !schedule.taskOwner || !schedule.command) throw new Error('Die Zuordnung zur DSM-Aufgabe fehlt. Bitte die Automatik im DSM prüfen.');
    const result = await this.call('get', {id, real_owner: schedule.taskOwner});
    const task = result.task || result;
    const script = typeof task.extra === 'string' ? JSON.parse(task.extra).script : task.extra?.script;
    if (Number(task.id) !== id || !String(task.name || '').startsWith(TASK_PREFIX) || task.type !== 'script' || String(task.real_owner || task.owner) !== String(schedule.taskOwner) || String(task.owner) !== String(schedule.taskOwner) || script !== schedule.command) throw new Error('Die DSM-Aufgabe stimmt nicht mit dieser SynoWake-Automatik überein. Sie wird zur Sicherheit nicht verändert.');
    return task;
  }

  async upsert(prepared, previous) {
    if (!prepared.owner || prepared.owner === 'root') throw new Error('Automatiken benötigen einen angemeldeten DSM-Benutzer. Root-Aufgaben werden nicht angelegt.');
    const schedule = prepared.schedule;
    const parameters = {name: `${TASK_PREFIX}${schedule.name}`, real_owner: prepared.owner, owner: prepared.owner, enable: Boolean(schedule.enabled), schedule: schedulerTiming(schedule), extra: {notify_enable: false, notify_mail: '', notify_if_error: false, script: prepared.command}};
    if (previous?.taskId) {
      await this.getOwnedTask(previous);
      if (String(previous.taskOwner) !== String(prepared.owner)) throw new Error('Diese Automatik gehört einem anderen DSM-Benutzer. Bitte mit diesem Benutzer bearbeiten.');
      parameters.id = Number(previous.taskId);
      await this.call('set', parameters);
      return {taskId: parameters.id, taskOwner: prepared.owner};
    }
    const result = await this.call('create', {...parameters, type: 'script'});
    const id = Number(result.id || result.task?.id);
    if (!Number.isInteger(id) || id <= 0) { const error = new Error('DSM meldet Erfolg, liefert aber keine Aufgaben-ID. Bitte „Vorbereitung abschließen“ verwenden; die Automatik wurde noch nicht zugeordnet.'); error.uncertain = true; throw error; }
    return {taskId: id, taskOwner: prepared.owner};
  }

  async inventory() {
    const tasks = [];
    const seen = new Set();
    let total;
    do {
      const result = await this.call('list', {sort_by: 'name', sort_direction: 'ASC', offset: tasks.length, limit: 100}, 3);
      const count = Number(result.total);
      if (result.total === undefined || result.total === null || result.total === '' || !Number.isInteger(count) || count < 0 || !Array.isArray(result.tasks)) throw new Error('DSM liefert keine vollständige Aufgabenliste. Die SynoWake-Zuordnung bleibt für eine sichere Prüfung erhalten.');
      if (count > 1000) throw new Error('Zu viele DSM-Aufgaben für eine sichere automatische Zuordnung. Bitte im DSM-Aufgabenplaner prüfen.');
      if (total !== undefined && count !== total) throw new Error('Die DSM-Aufgabenliste wurde während der Prüfung verändert. Bitte erneut versuchen.');
      total = count;
      if (!result.tasks.length && tasks.length < total) throw new Error('Die DSM-Aufgabenliste ist unvollständig. Die Vorbereitung bleibt erhalten.');
      for (const task of result.tasks) {
        if (!Number.isInteger(Number(task.id)) || Number(task.id) <= 0) throw new Error('DSM liefert eine ungültige Aufgaben-ID. Die SynoWake-Zuordnung bleibt erhalten.');
        if (seen.has(String(task.id))) throw new Error('DSM liefert überlappende Listenseiten. Die Vorbereitung bleibt erhalten.');
        seen.add(String(task.id)); tasks.push(task);
      }
      if (tasks.length > total) throw new Error('DSM liefert eine widersprüchliche Aufgabenliste. Die Vorbereitung bleibt erhalten.');
    } while (tasks.length < total);
    return tasks;
  }

  async recover(candidate) {
    const tasks = await this.inventory();
    const matching = [];
    for (const summary of tasks.filter(task => String(task.name || '').startsWith(TASK_PREFIX) && String(task.real_owner || task.owner) === candidate.taskOwner)) {
      const task = await this.call('get', {id: Number(summary.id), real_owner: candidate.taskOwner});
      const extra = typeof task.extra === 'string' ? JSON.parse(task.extra) : task.extra;
      if (extra?.script === candidate.command && task.type === 'script' && String(task.owner) === candidate.taskOwner) matching.push(task);
    }
    if (matching.length > 1) throw new Error('Mehrere DSM-Aufgaben verwenden diese Automatik. Bitte die Duplikate im DSM-Aufgabenplaner prüfen.');
    if (!matching.length) return null;
    const matched = matching[0];
    return this.upsert({schedule: candidate, command: candidate.command, owner: candidate.taskOwner}, {...candidate, taskId: Number(matched.id)});
  }

  async remove(schedule) {
    const id = Number(schedule.taskId);
    if (!Number.isInteger(id) || id <= 0) throw new Error('Die Zuordnung zur DSM-Aufgabe ist ungültig. Bitte im DSM prüfen.');
    const tasks = await this.inventory();
    if (!tasks.some(task => Number(task.id) === id)) return;
    await this.getOwnedTask(schedule);
    try { await this.call('delete', {tasks: [{id, real_owner: schedule.taskOwner}]}, 2); }
    catch (error) {
      if (!error.uncertain) throw error;
      const current = await this.inventory();
      if (current.some(task => Number(task.id) === id)) throw error;
      // A lost delete response is safe to reconcile only after a complete inventory proves absence.
    }
  }
}
