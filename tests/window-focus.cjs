const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const definitions = {};
let inheritedRender = 0;
let inheritedDestroy = 0;
const base = {
  afterRender() { inheritedRender++; },
  beforeDestroy() { inheritedDestroy++; }
};
const context = {
  Ext: {
    namespace() {},
    define(name, definition) { definitions[name] = definition; }
  },
  SYNO: {SDS: {h5uSynoWake: {MainWindow: {superclass: base}}}}
};
vm.runInNewContext(fs.readFileSync(path.resolve(__dirname, '../package/ui/SynoWake.js'), 'utf8'), context);
const windowDefinition = definitions['SYNO.SDS.h5uSynoWake.MainWindow'];

class Document extends EventTarget {
  constructor() { super(); this.listeners = new Set(); }
  addEventListener(type, listener, capture) {
    assert.equal(type, 'pointerdown');
    assert.equal(capture, true);
    this.listeners.add(listener);
    super.addEventListener(type, listener, capture);
  }
  removeEventListener(type, listener, capture) {
    assert.equal(capture, true);
    this.listeners.delete(listener);
    // Node's EventTarget needs the capture flag in an options object here.
    super.removeEventListener(type, listener, {capture});
  }
}
const pointer = button => {
  const event = new Event('pointerdown', {bubbles: true, cancelable: true});
  event.button = button;
  return event;
};

function fixture() {
  const frame = new EventTarget();
  const parentDocument = {};
  frame.ownerDocument = parentDocument;
  frame.contentDocument = new Document();
  const calls = [];
  const instance = {
    body: {dom: {querySelector(selector) { assert.equal(selector, '.synowake-frame'); return frame; }}},
    toFront(event) {
      // A click target from inside the iframe cannot activate DSM's window.
      assert.equal(typeof event.getTarget, 'function');
      assert.equal(event.getTarget(), frame);
      assert.equal(event.getTarget().ownerDocument, parentDocument);
      calls.push(event);
    }
  };
  return {frame, instance, calls};
}

const first = fixture();
const second = fixture();
windowDefinition.afterRender.call(first.instance);
windowDefinition.afterRender.call(second.instance);
assert.equal(inheritedRender, 2);
const click = pointer(0);
first.frame.contentDocument.dispatchEvent(click);
assert.equal(first.calls.length, 1);
assert.equal(second.calls.length, 0, 'Only the clicked window is activated');
assert.equal(click.defaultPrevented, false, 'Inputs and buttons must keep their default behavior');
first.frame.contentDocument.dispatchEvent(pointer(1));
first.frame.contentDocument.dispatchEvent(pointer(2));
assert.equal(first.calls.length, 1, 'Middle and right clicks must not activate the window');

const previous = first.frame.contentDocument;
first.frame.contentDocument = new Document();
first.frame.dispatchEvent(new Event('load'));
assert.equal(previous.listeners.size, 0, 'Navigation must release the previous document');
previous.dispatchEvent(pointer(0));
assert.equal(first.calls.length, 1);
first.frame.contentDocument.dispatchEvent(pointer(0));
assert.equal(first.calls.length, 2);
first.frame.dispatchEvent(new Event('load'));
assert.equal(first.frame.contentDocument.listeners.size, 1, 'Repeated load events must not duplicate listeners');

const accessible = first.frame.contentDocument;
Object.defineProperty(first.frame, 'contentDocument', {configurable: true, get() { throw new Error('Cross-origin navigation'); }});
assert.doesNotThrow(() => first.frame.dispatchEvent(new Event('load')));
assert.equal(accessible.listeners.size, 0);
Object.defineProperty(first.frame, 'contentDocument', {value: new Document(), configurable: true});
first.frame.dispatchEvent(new Event('load'));
const current = first.frame.contentDocument;
windowDefinition.beforeDestroy.call(first.instance);
assert.equal(inheritedDestroy, 1);
assert.equal(current.listeners.size, 0);
current.dispatchEvent(pointer(0));
first.frame.dispatchEvent(new Event('load'));
assert.equal(current.listeners.size, 0, 'Closing must also remove the iframe load listener');
assert.equal(first.calls.length, 2);
second.frame.contentDocument.dispatchEvent(pointer(0));
assert.equal(second.calls.length, 1, 'Closing one window must leave other windows working');
windowDefinition.beforeDestroy.call(second.instance);

const missing = {body: {dom: {querySelector() { return null; }}}};
assert.doesNotThrow(() => windowDefinition.afterRender.call(missing));
assert.doesNotThrow(() => windowDefinition.beforeDestroy.call(missing));
console.log('PASS: DSM iframe activation target, left clicks, independent windows, navigation, inaccessible frames and listener cleanup');
