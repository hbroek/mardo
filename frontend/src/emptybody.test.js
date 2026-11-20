import test from 'node:test';
import assert from 'node:assert/strict';

import {hasNoBody} from './emptybody.js';

test('front matter alone has no body', () => {
    assert.equal(hasNoBody('---\nname: Weft it\nurl: https://x.test/\n---\n'), true);
    assert.equal(hasNoBody('---\r\nname: A\r\n---'), true);
    assert.equal(hasNoBody('---\n# a comment\nname: A\n---\n\n\n  \n'), true);
});

test('an empty file has no body', () => {
    assert.equal(hasNoBody(''), true);
    assert.equal(hasNoBody('  \n'), true);
});

test('text after the front matter is a body', () => {
    assert.equal(hasNoBody('---\nname: A\n---\n\nHello\n'), false);
    assert.equal(hasNoBody('# Title\n'), false);
});

test('a horizontal rule is not front matter', () => {
    assert.equal(hasNoBody('---\n\nAfter the rule\n'), false);
});

test('not text', () => {
    assert.equal(hasNoBody(undefined), false);
});
