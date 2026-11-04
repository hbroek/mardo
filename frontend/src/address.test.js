import {test} from 'node:test';
import assert from 'node:assert/strict';
import {friendly, real, hasDocuments} from './address.js';

test('addresses are unchanged without a Documents folder', () => {
    delete globalThis.mardoDocuments;
    assert.equal(hasDocuments(), false);
    assert.equal(friendly('file:///a/b.md'), 'file:///a/b.md');
    assert.equal(real('~/b.md'), '~/b.md');
});

test('a file in the Documents folder is shown as ~/name and opened from it', () => {
    globalThis.mardoDocuments = '/var/mobile/Containers/Data/Application/X/Documents';
    const url = 'file:///var/mobile/Containers/Data/Application/X/Documents/blue%20vs%20red.md';
    assert.equal(friendly(url), '~/blue vs red.md');
    assert.equal(real('~/blue vs red.md'), url);
    assert.equal(friendly('http://example.com/a.md'), 'http://example.com/a.md');
    assert.equal(real('http://example.com/a.md'), 'http://example.com/a.md');
    delete globalThis.mardoDocuments;
});
