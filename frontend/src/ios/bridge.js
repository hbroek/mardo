// The iPhone and Android builds talk to the Go core through the app instead of Wails. Swift answers each message
// from the web view by calling Engine.Call(method, argsJSON) of the mobile package (see mobile/mobile.go)
// and replies with its JSON text: {"ok": result} or {"error": "message"}.

// Calls the shell handles itself: the desktop's menu bar has no counterpart on a phone.
const local = {
    SetViewShown: () => null,
};

// Android: the app's JavaScript interface answers later, through window.mardoReply(id, json).
const pending = new Map();
let nextId = 1;
globalThis.mardoReply = (id, json) => {
    const done = pending.get(id);
    if (!done) return;
    pending.delete(id);
    done(json);
};

function transport() {
    const t = globalThis.webkit?.messageHandlers?.mardo;
    if (t) return t;
    const android = globalThis.MardoAndroid;
    if (android) {
        return {
            postMessage: ({method, args}) => new Promise((resolve) => {
                const id = nextId++;
                pending.set(id, resolve);
                android.call(id, method, args);
            }),
        };
    }
    throw new Error('the app is not connected to its Go core');
}

/** Runs the core's method `method` with the list `args`; the promise gives its result or fails with its error. */
export async function call(method, args) {
    if (method in local) return local[method](...args);
    const reply = await transport().postMessage({method, args: JSON.stringify(args)});
    const answer = JSON.parse(reply);
    if (answer.error !== undefined) throw new Error(answer.error);
    return answer.ok;
}

const listeners = new Map(); // event name -> Set of callbacks

/** Swift calls this (through evaluateJavaScript) when the core has something to tell the page. */
globalThis.mardoEmit = (event, dataJSON) => {
    let data;
    try { data = JSON.parse(dataJSON); } catch { data = dataJSON; }
    for (const cb of [...(listeners.get(event) ?? [])]) cb(data);
};

/** Wails' EventsOn: calls `callback` for each event; the result stops listening. */
export function EventsOn(event, callback) {
    if (!listeners.has(event)) listeners.set(event, new Set());
    listeners.get(event).add(callback);
    return () => listeners.get(event)?.delete(callback);
}

export function EventsOff(...events) {
    for (const e of events) listeners.delete(e);
}

export function EventsEmit(event, ...data) {
    for (const cb of [...(listeners.get(event) ?? [])]) cb(...data);
}

/** There is no dropping files on a phone. */
export function OnFileDrop() {}
export function OnFileDropOff() {}

/** The right-click menu is not used on a phone; this keeps its import of Wails' runtime resolvable. */
export function BrowserOpenURL(url) {
    // A web view cannot open a new window by itself: the app opens the page (iPhone: a Safari sheet; Android: the browser).
    call('OpenURL', [url]).catch(() => {});
}
