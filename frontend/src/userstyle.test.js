import {test} from 'node:test';
import assert from 'node:assert/strict';
import {splitSelectors, scopeSelector, scopeSelectors} from './userstyle.js';

test('selectors are split at top-level commas only', () => {
    assert.deepEqual(splitSelectors('h2 a, h3 a:not(.x, .y), a[title="a, b"]'), ['h2 a', 'h3 a:not(.x, .y)', 'a[title="a, b"]']);
});

test('tokens and the page go to the reading area, elements to the text column', () => {
    const on = ':root[data-pagestyle]';
    assert.equal(scopeSelector(':root'), `${on} #content`);
    assert.equal(scopeSelector(':root[data-theme="dark"]'), `${on}[data-theme="dark"] #content`);
    assert.equal(scopeSelector(':root:not([data-theme="light"])'), `${on}:not([data-theme="light"]) #content`);
    assert.equal(scopeSelector('body'), `${on} #content`);
    assert.equal(scopeSelector('.page'), `${on} .markdown`);
    assert.equal(scopeSelector('.divPage > p'), `${on} .markdown > p`);
    assert.equal(scopeSelector('h2 a'), `${on} .markdown h2 a`);
    assert.equal(scopeSelector('p > img:first-child'), `${on} .markdown p > img:first-child`);
    assert.equal(scopeSelector('a[title="Direct link to this item."]:hover'), `${on} .markdown a[title="Direct link to this item."]:hover`);
    assert.equal(scopeSelector('bodyguard'), `${on} .markdown bodyguard`); // not body
    assert.equal(scopeSelectors('h2 a, h3 a'), `${on} .markdown h2 a, ${on} .markdown h3 a`);
});
