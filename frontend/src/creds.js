// The site credentials dialog. Secrets only go to the backend, never back.
import {Credentials, SaveCredential, DeleteCredential, ImportCookiesTxt, ClearCookies, ConnectOffer, ConnectStart, ConnectFinish, ConnectCancel} from '../wailsjs/go/main/App';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';

const $ = (id) => document.getElementById(id);
const dialog = $('creds-dialog');
const status = $('cred-status');

let onSaved = () => {};

function say(text, isError = false) {
    status.textContent = text;
    status.classList.toggle('error-text', isError);
}

function showKind() {
    const kind = $('cred-kind').value;
    for (const group of dialog.querySelectorAll('[data-kind]')) group.hidden = group.dataset.kind !== kind;
}

let saved = []; // the credentials listed, for what the sign-in says about replacing one

async function refreshList() {
    const items = (await Credentials()) ?? [];
    saved = items;
    const list = $('cred-list');
    list.replaceChildren(...items.map((c) => {
        const li = document.createElement('li');
        const label = document.createElement('span');
        label.textContent = c.oauth
            ? `${c.origin} · signed in${c.scope ? ` (${c.scope.split(' ').join(', ')})` : ''}`
            : `${c.origin} · ${c.kind}${c.username ? ` (${c.username})` : ''}${c.allowHttp ? ' · plain http' : ''}`;
        label.title = label.textContent;
        const edit = document.createElement('button');
        edit.type = 'button';
        edit.textContent = c.oauth ? 'Sign in again' : 'Replace';
        edit.onclick = c.oauth
            ? () => signInTo(c.origin)
            : () => { $('cred-origin').value = c.origin; $('cred-kind').value = c.kind; $('cred-allowhttp').checked = !!c.allowHttp; showKind(); say(''); offerSoon(); };
        const del = document.createElement('button');
        del.type = 'button';
        del.textContent = c.oauth ? 'Sign out' : 'Remove';
        del.title = c.oauth ? 'Forget the sign-in here, and tell the site to end it' : '';
        del.onclick = async () => {
            try { await DeleteCredential(c.origin); await refreshList(); offerSoon(); } catch (e) { say(String(e), true); }
        };
        li.append(label, edit, del);
        return li;
    }));
}

// --- signing in to a site, instead of copying a token ---

let offerTimer = 0;
let offered = null; // what the site said, for the origin in the field

const onPhone = () => document.documentElement.classList.contains('phone'); // the way back to the app needs the phone's own handling

const hostOf = (origin) => { try { return new URL(origin).host; } catch { return origin; } };

/** Asks the site, shortly after the field stops changing, whether it offers a sign-in. */
function offerSoon() {
    clearTimeout(offerTimer);
    offerTimer = setTimeout(showOffer, 350);
}

async function showOffer() {
    const box = $('cred-connect');
    const typed = $('cred-origin').value.trim();
    offered = null;
    box.hidden = true;
    if (!typed || onPhone()) return;
    let offer;
    try { offer = await ConnectOffer(typed); } catch { return; }
    if ($('cred-origin').value.trim() !== typed || !offer?.available) return; // the field moved on, or the site has none
    offered = offer;
    const host = hostOf(offer.origin);
    $('cred-connect-title').textContent = offer.signedIn ? `Signed in to ${host}` : `${host} can sign you in`;
    $('cred-connect-note').textContent = offer.signedIn
        ? 'Sign in again to change what Mardo may do, or to use another account.'
        : 'No token to copy: you approve Mardo on the site\'s own page, in your browser.';
    $('cred-connect-btn').textContent = offer.signedIn ? `Sign in to ${host} again` : `Sign in with ${host}`;
    box.hidden = false;
}

const connect = $('connect-dialog');
let signingIn = ''; // the id of the sign-in that is waiting for the reader

function connectStatus(text, isError = false) {
    $('connect-status').textContent = text;
    $('connect-status').classList.toggle('error-text', isError);
}

/** Asks the reader to agree, naming the site and what is asked for, then opens the site's page and waits. */
async function signInTo(origin) {
    if (!saved.length) saved = (await Credentials().catch(() => [])) ?? []; // for what the sign-in says about replacing one
    let offer;
    try { offer = await ConnectOffer(origin); } catch (e) { say(String(e), true); return; }
    if (!offer?.available) { say(offer?.reason || 'This site does not offer a sign-in.', true); return; }
    const host = hostOf(offer.origin);
    const replaces = saved.find((c) => c.origin === offer.origin && !c.oauth);
    $('connect-title').textContent = `Sign in to ${host}`;
    $('connect-text').textContent = `Mardo will open the sign-in page of ${host} in your browser. There you sign in and approve Mardo. ${replaces ? `This replaces the ${replaces.kind} you saved for ${host}. ` : ''}Mardo keeps the sign-in in your system keychain and sends it only to ${host}.`;
    $('connect-scopes').replaceChildren(...(offer.scopes?.length ? [Object.assign(document.createElement('li'), {textContent: `The site is asked for: ${offer.scopes.join(', ')}. It shows you what that means.`})] : []));
    connectStatus('');
    $('connect-go').hidden = false;
    $('connect-go').disabled = false;
    $('connect-cancel').textContent = 'Cancel';
    $('connect-go').onclick = async () => {
        $('connect-go').disabled = true;
        connectStatus('Opening the sign-in page in your browser. Approve Mardo there, then come back.');
        try {
            const started = await ConnectStart(offer.origin, '');
            signingIn = started.id;
            BrowserOpenURL(started.url);
            const info = await ConnectFinish(started.id);
            signingIn = '';
            connect.close();
            say(`Signed in to ${hostOf(info.origin)}`);
            if (dialog.open) dialog.close();
            await refreshList().catch(() => {});
            onSaved(info.origin);
        } catch (e) {
            signingIn = '';
            if (!connect.open) return; // cancelled
            $('connect-go').disabled = false;
            $('connect-go').textContent = 'Try again';
            connectStatus(String(e).replace(/^Error: /, ''), true);
        }
    };
    $('connect-go').textContent = 'Open the sign-in page';
    if (!connect.open) connect.showModal();
}

/** Signs in to the site at origin from anywhere (a sign-in required page): it asks the reader to agree first. */
export function signInWith(origin) { return signInTo(origin); }

function endSignIn() {
    if (signingIn) { ConnectCancel(signingIn).catch(() => {}); signingIn = ''; }
    if (connect.open) connect.close();
}

function clearSecrets() {
    for (const id of ['cred-cookie', 'cred-token', 'cred-user', 'cred-pass', 'cred-cookiefile']) $(id).value = '';
}

/** Opens the dialog. `prefill` may carry {origin, kind}. */
export async function openCredentials(prefill = {}) {
    clearSecrets();
    say('');
    $('cred-origin').value = prefill.origin ?? '';
    $('cred-kind').value = prefill.kind ?? 'cookie';
    $('cred-allowhttp').checked = false;
    showKind();
    await refreshList().catch((e) => say(String(e), true));
    showOffer();
    if (!dialog.open) dialog.showModal();
    $('cred-origin').value ? $('cred-cookie').focus() : $('cred-origin').focus();
}

export function initCredentials(savedCallback) {
    onSaved = savedCallback;
    $('cred-kind').addEventListener('change', showKind);
    $('cred-origin').addEventListener('input', offerSoon);
    $('cred-connect-btn').addEventListener('click', () => signInTo(offered?.origin ?? $('cred-origin').value));
    $('connect-cancel').addEventListener('click', endSignIn);
    connect.addEventListener('close', () => { if (signingIn) { ConnectCancel(signingIn).catch(() => {}); signingIn = ''; } });
    $('cred-close').addEventListener('click', () => dialog.close());
    dialog.addEventListener('close', clearSecrets);

    $('cred-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const cred = {
            origin: $('cred-origin').value,
            kind: $('cred-kind').value,
            cookie: $('cred-cookie').value.trim(),
            token: $('cred-token').value.trim(),
            username: $('cred-user').value,
            password: $('cred-pass').value,
            allowHttp: $('cred-allowhttp').checked,
        };
        try {
            const origin = await SaveCredential(cred);
            clearSecrets();
            await refreshList();
            say(`Saved for ${origin}`);
            dialog.close();
            onSaved(origin);
        } catch (err) {
            say(String(err), true);
        }
    });

    $('cred-cookiefile').addEventListener('change', async (e) => {
        const file = e.target.files[0];
        if (!file) return;
        try {
            const n = await ImportCookiesTxt($('cred-origin').value, await file.text());
            const origin = $('cred-origin').value;
            clearSecrets();
            await refreshList();
            say(`Imported ${n} cookie${n === 1 ? '' : 's'}`);
            dialog.close();
            onSaved(origin);
        } catch (err) {
            say(String(err), true);
            e.target.value = '';
        }
    });

    $('cred-clear-cookies').addEventListener('click', async () => {
        await ClearCookies();
        say('Saved cookies cleared');
    });
}

/** Origins that have a saved credential, as URL.origin strings. */
export async function credentialOrigins() {
    try {
        return new Set(((await Credentials()) ?? []).map((c) => c.origin));
    } catch {
        return new Set();
    }
}
