// Open the printed URL in Firefox. This serves only local regression fixtures.
import { createServer } from 'node:http';
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const legacy = readFileSync(resolve(root, 'tests/fixtures/legacy-style.css'));
const resultPath = process.argv[2];
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' };
const server = createServer((request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname;
  response.setHeader('Cache-Control', 'no-store');
  if (pathname === '/__results' && request.method === 'POST') {
    let content = '';
    request.on('data', data => { content += data; });
    request.on('end', () => {
      const result = JSON.parse(content);
      if (resultPath) writeFileSync(resultPath, JSON.stringify(result, null, 2));
      console.log(JSON.stringify(result)); response.end('OK');
    });
    return;
  }
  if (pathname === '/legacy.css') { response.setHeader('Content-Type', 'text/css'); response.end(legacy); return; }
  const file = resolve(root, '.' + decodeURIComponent(pathname));
  if (!file.startsWith(root.endsWith(sep) ? root : root + sep)) { response.writeHead(403); response.end(); return; }
  try {
    response.setHeader('Content-Type', types[file.slice(file.lastIndexOf('.'))] || 'application/octet-stream');
    response.end(readFileSync(file));
  } catch { response.writeHead(404); response.end(); }
});
server.listen(0, '127.0.0.1', () => console.log(`http://127.0.0.1:${server.address().port}/tests/css-isolation.html`));
