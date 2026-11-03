import {test, beforeEach} from 'node:test';
import assert from 'node:assert/strict';
import {call, EventsOn, EventsEmit} from './bridge.js';

let sent;
beforeEach(() => {
    sent = [];
    globalThis.webkit = {messageHandlers: {mardo: {postMessage: async (msg) => {
        sent.push(msg);
        return msg.method === 'Boom' ? '{"error":"it broke"}' : '{"ok":{"echo":' + msg.args + '}}';
    }}}};
});

test('a call goes to the app as a method name and a JSON list of arguments', async () => {
    const r = await call('Navigate', [1, 'https://example.com/']);
    assert.deepEqual(sent, [{method: 'Navigate', args: '[1,"https://example.com/"]'}]);
    assert.deepEqual(r, {echo: [1, 'https://example.com/']});
});

test('an error answer rejects with its message', async () => {
    await assert.rejects(call('Boom', []), {message: 'it broke'});
});

test('shell-only calls never reach the app', async () => {
    assert.equal(await call('SetViewShown', ['ask', true]), null);
    assert.equal(sent.length, 0);
});

test('without the app a call fails clearly', async () => {
    delete globalThis.webkit;
    await assert.rejects(call('NewTab', []), /not connected/);
});

test('events from Swift reach their listeners until they stop listening', () => {
    const got = [];
    const off = EventsOn('net', (d) => got.push(d));
    globalThis.mardoEmit('net', '{"url":"x"}');
    off();
    globalThis.mardoEmit('net', '{"url":"y"}');
    assert.deepEqual(got, [{url: 'x'}]);
    EventsOn('menu', (...a) => got.push(a));
    EventsEmit('menu', 'save');
    assert.deepEqual(got.at(-1), ['save']);
});
