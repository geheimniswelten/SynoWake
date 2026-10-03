const fs = require('node:fs');
const path = require('node:path');

// Load the real browser ES modules in Node without adding a runtime framework
// to the application. Resolve their local imports into nested data URLs.
function uiModuleURL(name, variant = '') {
  const file = path.resolve(__dirname, '../ui', name);
  const source = fs.readFileSync(file, 'utf8').replace(/(from\s+['"])(\.\/[^'"]+)(['"])/g, (_, before, reference, after) => {
    const target = reference.split('?')[0].slice(2);
    return before + uiModuleURL(target, variant) + after;
  });
  return `data:text/javascript;base64,${Buffer.from(source + '\n//' + variant).toString('base64')}`;
}

module.exports = {uiModuleURL};
