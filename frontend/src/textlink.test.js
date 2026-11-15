import test from 'node:test';
import assert from 'node:assert/strict';

import {highlightLink, quote, fragmentText} from './textlink.js';

test('a short selection becomes a text fragment link', () => {
    assert.equal(highlightLink('https://a.test/p#old', 'Hello, world-wide'), 'https://a.test/p#:~:text=Hello%2C%20world%2Dwide');
});

test('a long selection is marked by its first and last words', () => {
    const text = 'one two three four five six seven eight nine ten eleven twelve';
    assert.equal(highlightLink('https://a.test/p', text), 'https://a.test/p#:~:text=one%20two%20three%20four,nine%20ten%20eleven%20twelve');
});

test('a quote says where it is from', () => {
    assert.equal(quote('a\nb', 'Title', 'https://a.test/'), '> a\n> b\n>\n> — [Title](https://a.test/)');
});

test('the passage is read back from a text fragment', () => {
    assert.equal(fragmentText('https://a.test/p#:~:text=Hello%2C%20world%2Dwide'), 'Hello, world-wide');
    assert.equal(fragmentText('https://a.test/p#:~:text=one%20two,nine%20ten'), 'one two');
    assert.equal(fragmentText('https://a.test/p#:~:text=intro-,the%20thing,-after'), 'the thing');
    assert.equal(fragmentText('https://a.test/p#section'), '');
    assert.equal(fragmentText(undefined), '');
});
