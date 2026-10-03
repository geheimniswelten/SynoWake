import {t} from './i18n.js?v=0.1.14-0015';
// DSM's private TaskScheduler API is discovered at runtime. No crontab writes.
const API = 'SYNO.Core.TaskScheduler';
const TASK_PREFIX = 'SynoWake: ';
let cachedToken = '';

function invalidResponse(message) {
  const error = new Error(message);
  error.invalidResponse = true;
  return error;
}

function taskExtra(value) {
  try {
    const extra = typeof value === 'string' ? JSON.parse(value) : value;
    if (!extra || typeof extra !== 'object' || Array.isArray(extra)) throw new Error();
    return extra;
  } catch (_) {
    throw new Error(t('DSM liefert ungültige Aufgabendetails. Die Aufgabe wird nicht verändert. Bitte im DSM-Aufgabenplaner prüfen.'));
  }
}

export async function getSynoToken() {
  if (cachedToken) return cachedToken;
  const candidates = [new URLSearchParams(location.search).get('SynoToken'), window.SynoToken];
  try { candidates.push(window.parent.SYNO?.SDS?.Session?.SynoToken, window.parent.SynoToken); } catch (_) { /* A cross-origin parent is inaccessible. */ }
  cachedToken = candidates.find(value => typeof value === 'string' && value.length > 0) || '';
  if (cachedToken) return cachedToken;
  const response = await fetch('/webman/login.cgi', {credentials: 'same-origin', cache: 'no-store', signal: AbortSignal.timeout(15000)});
  if (!response.ok) throw new Error(t("Die DSM-Sitzung konnte nicht gelesen werden. Bitte im DSM erneut anmelden."));
  let json;
  try { json = await response.json(); } catch (_) { throw new Error(t("DSM hat keine gültige Sitzungsantwort geliefert. Bitte SynoWake im DSM öffnen.")); }
  cachedToken = json.SynoToken || json.data?.SynoToken || '';
  if (typeof cachedToken !== 'string' || !cachedToken) throw new Error(t("Kein DSM-Sitzungstoken verfügbar. Bitte SynoWake aus dem DSM-Hauptmenü öffnen."));
  return cachedToken;
}

export function schedulerTiming(schedule) {
  const [hour, minute] = schedule.time.split(':').map(Number);
  const days = [...new Set(schedule.days)].sort((a, b) => a - b);
  if (!Number.isInteger(hour) || hour < 0 || hour > 23 || !Number.isInteger(minute) || minute < 0 || minute > 59 || !days.length || days.some(day => !Number.isInteger(day) || day < 0 || day > 6)) throw new Error(t("Uhrzeit oder Wochentage sind ungültig."));
  return {date_type: 0, monthly_week: '[]', repeat_date: days.length === 7 ? 1001 : 1002, week_day: days.join(','), hour, minute, repeat_hour: 0, repeat_min: 0, last_work_hour: hour};
}

export class DsmScheduler {
  constructor() { this.info = null; this.methodVersions = new Map(); }

  async discover() {
    if (this.info) return this.info;
    const token = await getSynoToken();
    const query = new URLSearchParams({api: 'SYNO.API.Info', method: 'query', version: '1', query: API});
    let info;
    for (const endpoint of ['entry.cgi', 'query.cgi']) {
      const response = await fetch(`/webapi/${endpoint}?${query}`, {credentials: 'same-origin', cache: 'no-store', headers: {'X-SYNO-TOKEN': token}, signal: AbortSignal.timeout(15000)});
      if (response.status === 404 && endpoint === 'entry.cgi') continue;
      const data = await this.decode(response, t("API-Erkennung"));
      info = data[API];
      if (info?.path) break;
    }
    // The catalog and individual method versions can differ across DSM builds.
    if (!info?.path) throw new Error(t("DSM meldet keine TaskScheduler-API. Bitte Aufgabenplaner und DSM-Sitzung prüfen."));
    if (!/^[A-Za-z0-9_.\/-]+\.cgi$/.test(info.path) || info.path.includes('..')) throw new Error(t("DSM meldet einen unerwarteten API-Pfad."));
    this.info = info;
    return info;
  }

  async decode(response, method) {
    if (!response.ok) throw new Error(t("DSM-Aufgabenplaner: HTTP {0} bei {1}.", response.status, method));
    let json;
    try { json = await response.json(); } catch (_) { throw invalidResponse(t("DSM-Aufgabenplaner: ungültige Antwort bei {0}.", method)); }
    return this.decodePayload(json, method);
  }

  decodePayload(json, method) {
    if (!json || typeof json !== 'object') throw invalidResponse(t("DSM-Aufgabenplaner: ungültige Antwort bei {0}.", method));
    if (json.success !== true) {
      const code = json.error?.code ?? t("unbekannt");
      const explanation = {104: t("Die angeforderte API-Version unterstützt diese Methode nicht."), 105: t("DSM verweigert dieser Sitzung den Aufgabenplaner-Aufruf. Die Sitzungsanbindung oder DSM-Zugriffsrechte müssen geprüft werden."), 106: t("Die DSM-Sitzung ist abgelaufen. Bitte erneut anmelden."), 107: t("Die DSM-Sitzung wurde unterbrochen. Bitte erneut anmelden."), 119: t("Die DSM-Sitzung ist ungültig. Bitte erneut anmelden."), 101: t("Ein erforderlicher API-Parameter fehlt."), 102: t("Die TaskScheduler-API ist nicht verfügbar."), 103: t("Die API-Methode wird nicht unterstützt."), 100: t("DSM meldet einen allgemeinen Fehler."), 114: t("Ein erforderlicher Aufgabenparameter fehlt.")}[code];
      const error = new Error(t("DSM-Aufgabenplaner ({0}, Fehler {1}): {2}", method, code, explanation || t("Die Anfrage wurde abgelehnt. Bitte DSM-Aufgabenplaner und Benutzerrechte prüfen.")));
      error.code = Number(code); throw error;
    }
    return json.data ?? {};
  }

  async call(method, parameters, version) {
    const info = await this.discover();
    const minimum = Number(info.minVersion || 1);
    const maximum = Number(info.maxVersion || 4);
    // DSM 7.1's own task grid uses list v2 despite a catalog maximum of 3.
    // Newer implementations use list v3. Keep write-method selection separate.
    const known = version !== undefined ? [version] : method === 'list' ? [2, 3] : ['create', 'get', 'set'].includes(method) ? [4, 3, 2] : [4];
    // Prefer advertised versions, then allow known newer method versions when
    // the catalog is outdated. Cache successful versions separately per method.
    const candidates = [...new Set([this.methodVersions.get(method), ...known.filter(value => value <= maximum), ...known.filter(value => value > maximum)])].filter(value => known.includes(value) && value >= minimum);
    if (!candidates.length) throw new Error(t("DSM meldet keine unterstützte API-Version für {0}.", method));
    for (const [index, candidate] of candidates.entries()) {
      try {
        const result = await this.callVersion(method, parameters, candidate, info);
        this.methodVersions.set(method, candidate);
        return result;
      } catch (error) {
        // 104 rejects this version before execution. Read-only list can also
        // try its other known version after method rejection (103). Writes
        // still never retry on 103 or an ambiguous outcome/transport failure.
        if ((error.code !== 104 && !(method === 'list' && error.code === 103)) || error.uncertain) throw error;
        this.methodVersions.delete(method);
        if (index === candidates.length - 1) {
          error.message += t(" Geprüfte Versionen: {0}.", candidates.join(', '));
          throw error;
        }
      }
    }
  }

  async callVersion(method, parameters, version, info) {
    if (Number(info.minVersion || 1) > version) throw new Error(t("DSM unterstützt die benötigte API-Version {0} für {1} nicht.", version, method));
    let nativeAPI;
    try { if (typeof window.parent.SYNO?.API?.Request === 'function') nativeAPI = window.parent.SYNO.API; } catch (_) { /* Cross-origin parents are inaccessible. */ }
    if (nativeAPI) return this.callNative(nativeAPI, method, parameters, version);
    const body = new URLSearchParams({api: API, method, version: String(version)});
    const jsonFormat = info.requestFormat === 'JSON';
    for (const [key, value] of Object.entries(parameters)) body.set(key, jsonFormat || typeof value === 'object' ? JSON.stringify(value) : String(value));
    const token = await getSynoToken();
    body.set('SynoToken', token);
    const path = `/webapi/${info.path.replace(/^\//, '')}${jsonFormat ? `/${API}` : ''}`;
    let response;
    try { response = await fetch(path, {method: 'POST', credentials: 'same-origin', cache: 'no-store', headers: {'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8', 'X-SYNO-TOKEN': token}, body, signal: AbortSignal.timeout(20000)}); }
    catch (_) { const recovery = method === 'delete' ? t("Bitte Löschen erneut ausführen, damit SynoWake den aktuellen Aufgabenbestand prüft.") : t("Bitte „Vorbereitung abschließen“ verwenden, bevor du erneut speicherst."); const error = new Error(t("DSM-Aufgabenplaner: Verbindung bei {0} unterbrochen. Der Ausgang ist unklar. {1}", method, recovery)); error.uncertain = ['create', 'set', 'delete'].includes(method); throw error; }
    try { return await this.decode(response, method); }
    catch (error) { if ((!response.ok || error.invalidResponse || [109, 110, 111, 117, 118].includes(error.code)) && ['create', 'set', 'delete'].includes(method)) error.uncertain = true; throw error; }
  }

  callNative(nativeAPI, method, parameters, version) {
    // DSM's request layer supplies the live session token and handshake hash.
    // Forward typed parameters; DSM performs the JSON encoding itself.
    return new Promise((resolve, reject) => {
      const mutation = ['create', 'set', 'delete'].includes(method);
      const recovery = method === 'delete' ? t("Bitte Löschen erneut ausführen, damit SynoWake den aktuellen Aufgabenbestand prüft.") : t("Bitte „Vorbereitung abschließen“ verwenden, bevor du erneut speicherst.");
      let settled = false;
      const finish = (error, result) => {
        if (settled) return;
        settled = true; clearTimeout(timer);
        if (error) {
          if (mutation && (!Number.isFinite(error.code) || [109, 110, 111, 117, 118].includes(error.code))) error.uncertain = true;
          reject(error);
        } else resolve(result);
      };
      const timer = setTimeout(() => finish(new Error(t("DSM-Aufgabenplaner: Keine Antwort bei {0}. Der Ausgang ist unklar. {1}", method, recovery))), 22000);
      try {
        nativeAPI.Request.call(nativeAPI, {
          api: API, method, version, params: structuredClone(parameters), timeout: 20000,
          callback: (success, data) => {
            try {
              const code = data?.code ?? data?.error?.code;
              if (success !== true && (!data || typeof data !== 'object' || !Number.isInteger(Number(code)) || Number(code) <= 0)) throw invalidResponse(t("DSM-Aufgabenplaner: ungültige Antwort bei {0}. Der Ausgang ist unklar. {1}", method, recovery));
              finish(null, this.decodePayload(success === true ? {success: true, data} : {success: false, error: data.error || data}, method));
            } catch (error) { finish(error); }
          }
        });
      } catch (_) {
        finish(new Error(t("DSM-Aufgabenplaner: DSM-Anfrage bei {0} unterbrochen. Der Ausgang ist unklar. {1}", method, recovery)));
      }
    });
  }

  async getOwnedTask(schedule) {
    const id = Number(schedule.taskId);
    if (!Number.isInteger(id) || id <= 0 || !schedule.taskOwner || !schedule.command) throw new Error(t("Die Zuordnung zur DSM-Aufgabe fehlt. Bitte die Automatik im DSM prüfen."));
    const result = await this.call('get', {id, real_owner: schedule.taskOwner});
    const task = result.task || result;
    const script = taskExtra(task.extra).script;
    if (Number(task.id) !== id || !String(task.name || '').startsWith(TASK_PREFIX) || task.type !== 'script' || String(task.real_owner || task.owner) !== String(schedule.taskOwner) || String(task.owner) !== String(schedule.taskOwner) || script !== schedule.command) throw new Error(t("Die DSM-Aufgabe stimmt nicht mit dieser SynoWake-Automatik überein. Sie wird zur Sicherheit nicht verändert."));
    return task;
  }

  async upsert(prepared, previous) {
    if (!prepared.owner || prepared.owner === 'root') throw new Error(t("Automatiken benötigen einen angemeldeten DSM-Benutzer. Root-Aufgaben werden nicht angelegt."));
    const schedule = prepared.schedule;
    const parameters = {name: `${TASK_PREFIX}${schedule.name}`, real_owner: prepared.owner, owner: prepared.owner, enable: Boolean(schedule.enabled), schedule: schedulerTiming(schedule), extra: {notify_enable: false, notify_mail: '', notify_if_error: false, script: prepared.command}};
    if (previous?.taskId) {
      await this.getOwnedTask(previous);
      if (String(previous.taskOwner) !== String(prepared.owner)) throw new Error(t("Diese Automatik gehört einem anderen DSM-Benutzer. Bitte mit diesem Benutzer bearbeiten."));
      parameters.id = Number(previous.taskId);
      await this.call('set', parameters);
      return {taskId: parameters.id, taskOwner: prepared.owner};
    }
    const result = await this.call('create', {...parameters, type: 'script'});
    const id = Number(result.id || result.task?.id);
    if (!Number.isInteger(id) || id <= 0) { const error = new Error(t("DSM meldet Erfolg, liefert aber keine Aufgaben-ID. Bitte „Vorbereitung abschließen“ verwenden; die Automatik wurde noch nicht zugeordnet.")); error.uncertain = true; throw error; }
    return {taskId: id, taskOwner: prepared.owner};
  }

  async inventory() {
    const tasks = [];
    const seen = new Set();
    let total;
    do {
      const result = await this.call('list', {sort_by: 'name', sort_direction: 'asc', offset: tasks.length, limit: 100});
      const count = Number(result.total);
      if (result.total === undefined || result.total === null || result.total === '' || !Number.isInteger(count) || count < 0 || !Array.isArray(result.tasks)) throw new Error(t("DSM liefert keine vollständige Aufgabenliste. Die SynoWake-Zuordnung bleibt für eine sichere Prüfung erhalten."));
      if (count > 1000) throw new Error(t("Zu viele DSM-Aufgaben für eine sichere automatische Zuordnung. Bitte im DSM-Aufgabenplaner prüfen."));
      if (total !== undefined && count !== total) throw new Error(t("Die DSM-Aufgabenliste wurde während der Prüfung verändert. Bitte erneut versuchen."));
      total = count;
      if (!result.tasks.length && tasks.length < total) throw new Error(t("Die DSM-Aufgabenliste ist unvollständig. Die Vorbereitung bleibt erhalten."));
      for (const task of result.tasks) {
        if (!Number.isInteger(Number(task.id)) || Number(task.id) <= 0) throw new Error(t("DSM liefert eine ungültige Aufgaben-ID. Die SynoWake-Zuordnung bleibt erhalten."));
        if (seen.has(String(task.id))) throw new Error(t("DSM liefert überlappende Listenseiten. Die Vorbereitung bleibt erhalten."));
        seen.add(String(task.id)); tasks.push(task);
      }
      if (tasks.length > total) throw new Error(t("DSM liefert eine widersprüchliche Aufgabenliste. Die Vorbereitung bleibt erhalten."));
    } while (tasks.length < total);
    return tasks;
  }

  async recover(candidate) {
    const tasks = await this.inventory();
    const matching = [];
    for (const summary of tasks.filter(task => String(task.name || '').startsWith(TASK_PREFIX) && String(task.real_owner || task.owner) === candidate.taskOwner)) {
      const task = await this.call('get', {id: Number(summary.id), real_owner: candidate.taskOwner});
      const extra = taskExtra(task.extra);
      if (extra?.script === candidate.command && task.type === 'script' && String(task.owner) === candidate.taskOwner) matching.push(task);
    }
    if (matching.length > 1) throw new Error(t("Mehrere DSM-Aufgaben verwenden diese Automatik. Bitte die Duplikate im DSM-Aufgabenplaner prüfen."));
    if (!matching.length) return null;
    const matched = matching[0];
    return this.upsert({schedule: candidate, command: candidate.command, owner: candidate.taskOwner}, {...candidate, taskId: Number(matched.id)});
  }

  async remove(schedule) {
    const id = Number(schedule.taskId);
    if (!Number.isInteger(id) || id <= 0) throw new Error(t("Die Zuordnung zur DSM-Aufgabe ist ungültig. Bitte im DSM prüfen."));
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
