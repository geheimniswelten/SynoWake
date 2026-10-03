const fs = require('node:fs');
const assert = require('node:assert/strict');

async function run() {
  global.window = {SynoToken: 'test-token', parent: {}};
  global.location = {search: ''};
  const source = fs.readFileSync(require('node:path').join(__dirname, '../ui/scheduler.js'), 'utf8');
  const {DsmScheduler, schedulerTiming} = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  assert.equal(schedulerTiming({time: '07:30', days: [0,1,2,3,4,5,6]}).repeat_date, 1001);
  assert.equal(schedulerTiming({time: '07:30', days: [1,2,3,4,5]}).repeat_date, 1002);
  assert.throws(() => schedulerTiming({time: '24:00', days: [1]}));
  const candidate = {id: 'a'.repeat(32), name: 'Recovery', taskOwner: 'tester', command: 'matching fixed command', time: '07:30', days: [1,2,3,4,5], enabled: true};
  const tasks = Array.from({length: 150}, (_, index) => ({id: index + 1, name: `SynoWake: ${index}`, owner: 'tester', real_owner: 'tester', type: 'script', extra: {script: index === 125 ? candidate.command : `unrelated fixed command ${index}`}}));
  let mode = 'normal';
  let offsets = [];
  let mutations = 0;
  global.fetch = async (url, options = {}) => {
    if (url.startsWith('/webapi/entry.cgi?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': {path: 'entry.cgi', minVersion: 1, maxVersion: 3, requestFormat: 'JSON'}}});
    const body = new URLSearchParams(options.body);
    const method = body.get('method');
    const parameters = Object.fromEntries([...body].filter(([key]) => !['api', 'method', 'version', 'SynoToken'].includes(key)).map(([key,value]) => [key, JSON.parse(value)]));
    let data;
    if (method === 'list') {
      const offset = parameters.offset; offsets.push(offset);
      if (mode === 'normal') data = {tasks: tasks.slice(offset, offset + 50), total: 150};
      if (mode === 'gap') data = {tasks: offset ? [] : tasks.slice(0, 50), total: 150};
      if (mode === 'changed') data = {tasks: tasks.slice(offset, offset + 50), total: offset ? 149 : 150};
      if (mode === 'overlap') data = {tasks: tasks.slice(0, 50), total: 150};
      if (mode === 'missing-total') data = {tasks: []};
    } else if (method === 'get') data = tasks[parameters.id - 1];
    else if (method === 'set') { mutations++; data = {}; }
    else throw new Error(`Unexpected method ${method}`);
    return Response.json({success: true, data});
  };
  const recovered = await new DsmScheduler().recover(candidate);
  assert.equal(recovered.taskId, 126);
  assert.deepEqual(offsets, [0, 50, 100], 'Pagination must advance by actual page count even if DSM clamps the requested limit');
  assert.equal(mutations, 1);
  for (const failure of ['gap', 'changed', 'overlap', 'missing-total']) {
    mode = failure; offsets = [];
    await assert.rejects(new DsmScheduler().recover(candidate));
    assert.equal(mutations, 1, 'Incomplete inventories must never modify or conclude task absence');
  }
  console.log('PASS: Scheduler daily/weekday/time contract; complete paginated recovery with clamped limit; incomplete, changing, overlapping and missing-total inventories reject safely.');
  let deletionMode = 'absent';
  let inventoryTasks = [];
  let getCalls = 0;
  let deleteCalls = 0;
  const mapped = {...candidate, taskId: 126};
  global.fetch = async (url, options = {}) => {
    if (url.startsWith('/webapi/entry.cgi?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': {path: 'entry.cgi', minVersion: 1, maxVersion: 3, requestFormat: 'JSON'}}});
    const body = new URLSearchParams(options.body);
    const method = body.get('method');
    const offset = JSON.parse(body.get('offset') || '0');
    if (method === 'list') {
      const data = deletionMode === 'incomplete' ? {tasks: offset ? [] : inventoryTasks, total: 2} : {tasks: inventoryTasks.slice(offset, offset + 100), total: inventoryTasks.length};
      return Response.json({success: true, data});
    }
    if (method === 'get') { getCalls++; return Response.json({success: true, data: inventoryTasks[0]}); }
    if (method === 'delete') {
      deleteCalls++;
      if (deletionMode === 'unknown-removed') { inventoryTasks = []; throw new Error('Lost successful response'); }
      if (deletionMode === 'unknown-present') throw new Error('Lost unsuccessful response');
      inventoryTasks = []; return Response.json({success: true});
    }
    throw new Error(`Unexpected deletion method ${method}`);
  };
  await new DsmScheduler().remove(mapped);
  assert.equal(getCalls, 0, 'Confirmed absence permits local cleanup without a failing get');
  assert.equal(deleteCalls, 0);
  deletionMode = 'foreign'; inventoryTasks = [{...tasks[125], extra: {script: 'foreign command'}}];
  await assert.rejects(new DsmScheduler().remove(mapped), /stimmt nicht/);
  assert.equal(deleteCalls, 0, 'A present foreign task must never be deleted');
  deletionMode = 'unknown-removed'; inventoryTasks = [tasks[125]];
  await new DsmScheduler().remove(mapped);
  assert.equal(deleteCalls, 1, 'A lost successful delete response is reconciled from confirmed absence');
  deletionMode = 'unknown-present'; inventoryTasks = [tasks[125]];
  await assert.rejects(new DsmScheduler().remove(mapped), /Ausgang ist unklar/);
  assert.equal(deleteCalls, 2, 'A lost unsuccessful response must not permit local cleanup');
  deletionMode = 'incomplete'; inventoryTasks = [tasks[125]];
  await assert.rejects(new DsmScheduler().remove(mapped), /unvollständig/);
  assert.equal(deleteCalls, 2, 'A partial inventory must block deletion before any mutation');
  console.log('PASS: Deletion proves complete inventory absence, guards foreign scripts, reconciles a lost successful response, and preserves local state for unresolved deletion and partial inventory.');
  const info = {path: 'entry.cgi', minVersion: 1, maxVersion: 3, requestFormat: 'JSON'};
  for (const discoveryMode of ['modern', 'legacy-404', 'legacy-empty', 'no-api', 'unsafe-path']) {
    const infoCalls = []; const commands = [];
    global.fetch = async (url, options = {}) => {
      if (url.includes('?')) {
        infoCalls.push(url);
        if (url.startsWith('/webapi/entry.cgi?') && discoveryMode === 'legacy-404') return new Response('', {status: 404});
        const data = discoveryMode === 'no-api' || (url.startsWith('/webapi/entry.cgi?') && discoveryMode === 'legacy-empty') ? {} : {'SYNO.Core.TaskScheduler': discoveryMode === 'unsafe-path' ? {...info, path: '../foreign.cgi'} : info};
        return Response.json({success: true, data});
      }
      const body = new URLSearchParams(options.body); commands.push(body);
      assert.equal(body.get('SynoToken'), 'test-token');
      assert.equal(options.headers['X-SYNO-TOKEN'], 'test-token');
      assert.equal(body.get('version'), '3', 'Prefer the advertised version before trying newer method versions');
      return Response.json({success: true, data: {id: 99}});
    };
    const scheduler = new DsmScheduler();
    if (['no-api', 'unsafe-path'].includes(discoveryMode)) {
      await assert.rejects(scheduler.call('create', {name: 'SynoWake: API-Test'})); assert.equal(commands.length, 0);
    } else {
      assert.equal((await scheduler.call('create', {name: 'SynoWake: API-Test'})).id, 99);
      assert.equal(commands.length, 1, 'Create must not be retried speculatively');
    }
    assert.ok(infoCalls[0].startsWith('/webapi/entry.cgi?'));
    if (discoveryMode.startsWith('legacy')) assert.equal(infoCalls.length, 2);
  }
  for (const code of [104, 105, 106, 109]) {
    let requests = 0;
    global.fetch = async (url) => {
      if (url.includes('?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': info}});
      requests++; return Response.json({success: false, error: {code}});
    };
    await assert.rejects(new DsmScheduler().call('create', {}), error => {
      assert.equal(error.code, code);
      if (code === 104) { assert.match(error.message, /API-Version/); assert.doesNotMatch(error.message, /Keine Berechtigung/); }
      if (code === 105) { assert.match(error.message, /Sitzung/); assert.doesNotMatch(error.message, /mit einem DSM-Administrator anmelden/); }
      if (code === 109) assert.equal(error.uncertain, true);
      return true;
    });
    assert.equal(requests, code === 104 ? 3 : 1, 'Only an explicit unsupported-version rejection permits another create version');
  }
  console.log('PASS: Catalog version preference; modern/legacy discovery; missing/unsafe APIs block writes; token transport; accurate version/permission/session errors; only error 104 permits version fallback.');

  for (const supported of [2, 3, 4]) {
    const versions = []; let created = 0;
    global.fetch = async (url, options = {}) => {
      if (url.includes('?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': info}});
      const body = new URLSearchParams(options.body); const version = Number(body.get('version')); versions.push(version);
      if (version !== supported) return Response.json({success: false, error: {code: 104}});
      created++; return Response.json({success: true, data: {id: 99}});
    };
    const scheduler = new DsmScheduler();
    assert.equal((await scheduler.call('create', {})).id, 99);
    assert.equal(created, 1, 'Exactly one successful creation during version negotiation');
    const first = supported === 3 ? [3] : supported === 2 ? [3, 2] : [3, 2, 4];
    assert.deepEqual(versions, first);
    await scheduler.call('create', {});
    assert.deepEqual(versions, [...first, supported], 'Reuse the successful method version');
  }
  for (const supported of [2, 3]) {
    const attempts = []; const scheduler = new DsmScheduler();
    global.fetch = async (url, options = {}) => {
      if (url.includes('?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': {...info, maxVersion: 4}}});
      const body = new URLSearchParams(options.body); const version = Number(body.get('version')); attempts.push(version);
      return Response.json(version === supported ? {success: true, data: {id: 100}} : {success: false, error: {code: 104}});
    };
    assert.equal((await scheduler.call('create', {})).id, 100);
    assert.deepEqual(attempts, supported === 3 ? [4,3] : [4,3,2]);
  }
  let afterRejection = 0;
  global.fetch = async (url) => {
    if (url.includes('?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': info}});
    if (++afterRejection === 1) return Response.json({success: false, error: {code: 104}});
    throw new Error('Lost response during compatible attempt');
  };
  await assert.rejects(new DsmScheduler().call('create', {}), error => error.uncertain === true);
  assert.equal(afterRejection, 2, 'An ambiguous compatible attempt must stop negotiation immediately');
  const mixedCalls = [];
  global.fetch = async (url, options = {}) => {
    if (url.includes('?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': info}});
    const body = new URLSearchParams(options.body); const method = body.get('method'); const version = Number(body.get('version'));
    mixedCalls.push([method, version]);
    const supported = {create: 2, get: 3, set: 4, list: 3, delete: 2}[method];
    if (version !== supported) return Response.json({success: false, error: {code: 104}});
    return Response.json({success: true, data: {id: 99}});
  };
  const mixed = new DsmScheduler();
  for (const method of ['create', 'get', 'set']) await mixed.call(method, {});
  await mixed.call('list', {}, 3); await mixed.call('delete', {}, 2);
  assert.deepEqual(mixedCalls, [['create',3],['create',2],['get',3],['set',3],['set',2],['set',4],['list',3],['delete',2]]);
  for (const failure of ['network', 'http', 'invalid-json', 101, 103, 105, 106, 109, 110, 111, 114, 117, 118, 119]) {
    let writes = 0;
    global.fetch = async (url) => {
      if (url.includes('?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': info}});
      writes++;
      if (failure === 'network') throw new Error('Lost response');
      if (failure === 'http') return new Response('', {status: 502});
      if (failure === 'invalid-json') return new Response('not JSON');
      return Response.json({success: false, error: {code: failure}});
    };
    await assert.rejects(new DsmScheduler().call('create', {}));
    assert.equal(writes, 1, `Never negotiate on ${failure}`);
  }
  console.log('PASS: Create on versions 2/3/4 with exactly one successful creation; per-method caching and mixed versions; no retry on network/HTTP/malformed responses or other error codes.');

  let rawWrites = 0;
  global.fetch = async url => {
    if (url.includes('?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': info}});
    rawWrites++; throw new Error('Native DSM requests must never fall back to raw writes');
  };
  const nativeCalls = [];
  const nativeAPI = {Request(request) {
    assert.equal(this, nativeAPI);
    assert.equal(request.api, 'SYNO.Core.TaskScheduler');
    assert.equal(request.timeout, 20000);
    nativeCalls.push(request);
    if (request.version !== 2) request.callback(false, {code: 104});
    else request.callback(true, {id: 101});
  }};
  window.parent = {SYNO: {API: nativeAPI}};
  const native = new DsmScheduler();
  const nativeParams = {name: 'SynoWake: Sitzung', enable: false, schedule: {hour: 7}, extra: {script: 'fixed command'}};
  assert.equal((await native.call('create', nativeParams)).id, 101);
  assert.deepEqual(nativeCalls.map(request => request.version), [3, 2]);
  assert.deepEqual(nativeCalls[1].params, nativeParams, 'The DSM SDK receives typed values');
  nativeCalls[1].params.extra.script = 'modified by SDK';
  assert.equal(nativeParams.extra.script, 'fixed command', 'SDK mutations must not alter caller parameters');
  await native.call('create', nativeParams);
  assert.deepEqual(nativeCalls.map(request => request.version), [3, 2, 2]);
  for (const failure of [105, 109, 'network', 'malformed', 'throw']) {
    let attempts = 0;
    nativeAPI.Request = request => {
      attempts++;
      if (failure === 'throw') throw new Error('Interrupted native request');
      request.callback(false, failure === 'network' ? {status: 0, timedout: true} : failure === 'malformed' ? {code: ''} : {code: failure});
    };
    await assert.rejects(new DsmScheduler().call('create', nativeParams), error => {
      if (failure === 105) { assert.equal(error.code, 105); assert.notEqual(error.uncertain, true); assert.match(error.message, /Sitzung/); }
      else assert.equal(error.uncertain, true, 'Ambiguous native mutations retain recovery state');
      return true;
    });
    assert.equal(attempts, 1, 'Native failures must not repeat a write or switch transport');
  }
  const realSetTimeout = global.setTimeout, realClearTimeout = global.clearTimeout;
  let timeoutCallback, lateReply, cleared = 0;
  try {
    global.setTimeout = callback => { timeoutCallback = callback; return 42; };
    global.clearTimeout = id => { assert.equal(id, 42); cleared++; };
    nativeAPI.Request = request => { lateReply = request.callback; };
    const timedOut = native.callNative(nativeAPI, 'create', nativeParams, 3);
    timeoutCallback();
    await assert.rejects(timedOut, error => error.uncertain === true);
    lateReply(true, {id: 102});
    assert.equal(cleared, 1, 'Late native callbacks must not complete a timed out mutation');
  } finally { global.setTimeout = realSetTimeout; global.clearTimeout = realClearTimeout; }
  assert.equal(rawWrites, 0);
  Object.defineProperty(window, 'parent', {configurable: true, get() { throw new Error('Cross-origin parent'); }});
  let fallbackWrites = 0;
  global.fetch = async url => {
    if (url.includes('?')) return Response.json({success: true, data: {'SYNO.Core.TaskScheduler': info}});
    fallbackWrites++; return Response.json({success: true, data: {id: 103}});
  };
  assert.equal((await new DsmScheduler().call('create', nativeParams)).id, 103);
  assert.equal(fallbackWrites, 1);
  console.log('PASS: Native DSM session transport with typed parameters, per-method versions, permission/error handling, no raw retry, timeout recovery and late callback protection; inaccessible parent fallback.');
}
run().catch(error => { console.error(error); process.exitCode = 1; });
