import {test} from 'node:test';
import assert from 'node:assert/strict';
import {insertSyntax} from './cheatsheet.js';

// A stand-in for a textarea: the text, a selection, and the few methods insertSyntax uses.
function box(value, start = value.length, end = start) {
    const a = {value, selectionStart: start, selectionEnd: end, fired: 0,
        focus() {},
        setRangeText(t, from, to) { a.value = a.value.slice(0, from) + t + a.value.slice(to); a.selectionStart = a.selectionEnd = from + t.length; },
        setSelectionRange(s, e) { a.selectionStart = s; a.selectionEnd = e; },
        dispatchEvent() { a.fired++; return true; }};
    return a;
}
const sel = (a) => a.value.slice(a.selectionStart, a.selectionEnd);

test('wrap puts a hint between the marks and selects it', () => {
    const a = box('hi ');
    insertSyntax(a, {kind: 'wrap', before: '**', after: '**', hint: 'bold'});
    assert.equal(a.value, 'hi **bold**');
    assert.equal(sel(a), 'bold');
    assert.equal(a.fired, 1);
});

test('wrap goes around the selected text', () => {
    const a = box('make this bold now', 5, 9);
    insertSyntax(a, {kind: 'wrap', before: '**', after: '**', hint: 'bold'});
    assert.equal(a.value, 'make **this** bold now');
    assert.equal(sel(a), 'this');
});

test('line puts the mark at the start of every selected line', () => {
    const a = box('one\ntwo\nthree', 5, 7); // inside "two"
    insertSyntax(a, {kind: 'line', before: '- '});
    assert.equal(a.value, 'one\n- two\nthree');
    const b = box('one\ntwo\nthree', 1, 10);
    insertSyntax(b, {kind: 'line', before: '> '});
    assert.equal(b.value, '> one\n> two\n> three');
});

test('a block gets lines of its own', () => {
    const a = box('text');
    insertSyntax(a, {kind: 'block', text: '---'});
    assert.equal(a.value, 'text\n\n---\n');
    const b = box('');
    insertSyntax(b, {kind: 'block', text: '---'});
    assert.equal(b.value, '---\n');
});

test('a code block wraps the selection in fences', () => {
    const a = box('x = 1', 0, 5);
    insertSyntax(a, {kind: 'fence'});
    assert.equal(a.value, '```\nx = 1\n```\n');
    assert.equal(sel(a), 'x = 1');
});
