import {english} from './translations.js?v=0.1.13-0014';

export function normalizeLanguage(value) {
  return /^(de(?:[-_].*)?|ger|german)$/i.test(String(value || '')) ? 'de' : 'en';
}

export function detectLanguage() {
  const requested = new URLSearchParams(globalThis.location?.search || '').get('lang');
  if (/^(de|en|ger|enu)$/i.test(requested || '')) return normalizeLanguage(requested);
  try {
    const parent = globalThis.window?.parent;
    const dsm = parent?.SYNO?.SDS?.Session?.lang;
    if (dsm && dsm !== 'def') return normalizeLanguage(dsm);
    if (parent && parent !== globalThis.window && parent.document?.documentElement?.lang) return normalizeLanguage(parent.document.documentElement.lang);
  } catch (_) { /* A cross-origin parent cannot provide DSM language settings. */ }
  return normalizeLanguage(globalThis.navigator?.languages?.[0] || globalThis.navigator?.language || 'en');
}

export const language = detectLanguage();

// German source strings are gettext-style keys. Substitute once so device
// names containing {0}, markup or percent signs remain literal user data.
export function t(key, ...args) {
  const pattern = language === 'de' ? key : english[key] ?? key;
  return pattern.replace(/\{(\d+)\}/g, (match, index) => index < args.length ? String(args[index]) : match);
}

// Translate the initial, static app markup exactly once, before user data is
// rendered. No observer or translation pass ever rewrites stored device names.
export function translateDocument(root) {
  root.ownerDocument.documentElement.lang = language;
  const walker = root.ownerDocument.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    const node = walker.currentNode;
    if (['SCRIPT', 'STYLE'].includes(node.parentElement?.tagName)) continue;
    const key = node.nodeValue.trim();
    if (Object.hasOwn(english, key)) node.nodeValue = node.nodeValue.replace(key, t(key));
  }
  for (const element of root.querySelectorAll('[title], [aria-label], [placeholder]')) {
    for (const attribute of ['title', 'aria-label', 'placeholder']) {
      const key = element.getAttribute(attribute);
      if (key && Object.hasOwn(english, key)) element.setAttribute(attribute, t(key));
    }
  }
}
