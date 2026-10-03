"""Check every static product text and source translation key without dependencies."""
from pathlib import Path
from html.parser import HTMLParser
import json, re

root = Path(__file__).resolve().parents[1]
catalog = json.loads((root/'internal/synowake/translations.json').read_text(encoding='utf-8'))
invariants = {'SynoWake', 'DEMO', 'WAKE-ON-LAN', '×', '⏻', '⌕', '▦', '▤', '◷'}
class Markup(HTMLParser):
    def __init__(self): super().__init__(); self.texts = set()
    def handle_data(self, text):
        if text.strip(): self.texts.add(text.strip())
    def handle_starttag(self, tag, attrs):
        for key, value in attrs:
            if key in ('title', 'placeholder', 'aria-label') and value: self.texts.add(value)

markup = Markup()
markup.feed((root/'ui/index.html').read_text(encoding='utf-8'))
missing = sorted(text for text in markup.texts if text not in catalog and text not in invariants
                 and not re.fullmatch(r'[\d.A-F:]+', text))
assert not missing, f'Untranslated static app text: {missing}'
for name in ('app.js', 'scheduler.js'):
    source = (root/'ui'/name).read_text(encoding='utf-8')
    for key in re.findall(r'\bt\(("(?:\\.|[^"\\])*")', source):
        assert json.loads(key) in catalog, f'Missing source key: {name}: {key}'
    for key in re.findall(r"\bt\('([^'\\]*(?:\\.[^'\\]*)*)'", source):
        assert key in catalog, f'Missing source key: {name}: {key}'

# Every package-defined error template needs an English counterpart. Native
# OS error details and command syntax are intentionally language-invariant.
error_invariants = {'%s: %w', 'no origin scheme', '--serve-demo 127.0.0.1:PORT UI_DIRECTORY'}
for path in (root/'internal/synowake').glob('*.go'):
    if path.name.endswith('_test.go'): continue
    source = path.read_text(encoding='utf-8')
    for literal in re.findall(r'(?:errors\.New|fmt\.Errorf)\(\s*("(?:\\.|[^"\\])*")', source):
        key = json.loads(literal)
        assert key in catalog or key in error_invariants, f'Missing backend error translation: {path.name}: {key}'

for language in ('ger', 'enu'):
    text = (root/'package/ui/texts'/language/'strings').read_text(encoding='utf-8')
    for key in ('title', 'message', 'wake_sent', 'wake_failed'):
        assert re.search(r'^'+key+r'=".+"$', text, re.M), f'Missing notification: {language}/{key}'
print(f'PASS: all {len(markup.texts)} static app texts/attributes, source keys and both notification catalogs')
