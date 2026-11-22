import './style.css';
import './schemes.css';
import './pagestyles.css';
import {RenderPreview, Anchor, GoTo, TabHistory, Navigate, Reload, Back, Forward, ClearHistory, FullHistory, About, BookmarkFor, NewTab, CloseTab, SummarizePage, ReadPage, SavePage, SaveSource, SetViewShown, PickFile, SaveNewDocument, TakeOpenRequests, NavigateAsIs, ShareItem, PrintNative, MarkdownVersions, SetMarkdownVersions, Bonbon, SetBonbon, FollowLink, StyleSheets, StyleSheet, OpenStylesFolder, ConvertPage, Actions, RunAction, OpenActionsFolder, EditActions, Favicon, ConnectOffer, ConnectStatus} from '../wailsjs/go/main/App';
import {OnFileDrop, EventsOn, BrowserOpenURL} from '../wailsjs/runtime/runtime';
import {friendly, real, hasDocuments} from './address.js';
import * as view from './view.js';
import {svgIcon} from './icons.js';
import {initConsole, toggleConsole} from './console.js';
import {initBookmarks, openBookmark, openFolder as openBookmarkFolder} from './bookmarks.js';
import {initMarkBar, setMarkBar, markBarShown, render as renderMarkBar} from './markbar.js';
import {initPanel, panelShortcut, refreshPanel, togglePanel} from './bookmarkpanel.js';
import {initStorage} from './storage.js';
import {initTagging} from './tagging.js';
import {initOrganize} from './organize.js';
import {initMenu, showCredentialState} from './theme.js';
import {initAI, openAI, aiReady, newRequestId, cancelRequest} from './ai.js';
import {initAsk, refreshAsk, toggleAsk, askNow} from './ask.js';
import {initReading, selectedText} from './reading.js';
import {initContextMenu, popupMenu} from './contextmenu.js';
import {fragmentText} from './textlink.js';
import {initSplitters} from './splitter.js';
import {initCompare, openCompare} from './compare.js';
import './phone.js';
import './wide.js';
import './imagealign.js';
import * as finder from './find.js';
import {hasNoBody} from './emptybody.js';
import './linkstatus.js';
import {initCredentials, openCredentials, credentialOrigins, signInWith} from './creds.js';

const $ = (id) => document.getElementById(id);
const address = $('address');
const content = $('content');
const sourceBtn = $('source');
const sourceIcon = sourceBtn.innerHTML;
const pencilIcon = '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20l1-4L16.5 4.5a2 2 0 0 1 3 3L8 19z"/><path d="M14.5 6.5l3 3"/></svg>';

// Each tab is {id, page, pending, summary, chat, chatTried, showSource, seq, loading, scroll}. A restored tab
// that was not selected has only `pending`, its address, until it is first shown.
// The Go side keeps the tab's back/forward history under the same id; the last page shown is kept here
// so switching tabs does not fetch again.
const tabs = [];
let active = null;
let saved = null;        // the bookmark for the active tab's page, if any

/** Puts the cursor in the address field, except on a phone, where that would raise the keyboard over the page. */
const focusAddress = () => { if (!document.documentElement.classList.contains('phone')) address.focus(); };

const currentPage = () => active?.page ?? null;

/** The state of editing the active tab's source, for view.source. */
function editor() {
    const tab = active;
    return {
        draft: tab.draft ?? null,
        saving: !!tab.saving,
        error: tab.saveError ?? '',
        diff: !!tab.showDiff,
        aiDraft: !!tab.aiDraft,
        canUndo: !!tab.undo?.length,
        canRedo: !!tab.redo?.length,
        onUndo: () => undoDraft(tab, true),
        onRedo: () => undoDraft(tab, false),
        onDiff: () => { tab.showDiff = !tab.showDiff; render(); },
        onEdit: startEdit,
        preview: (text) => RenderPreview(text, tab.page?.url ?? ''),
        onInput: (text) => {
            const now = Date.now();
            if (now - (tab.lastEdit ?? 0) > UNDO_PAUSE) remember(tab); // typing in one go is one step; a pause starts the next
            tab.lastEdit = now;
            tab.draft = text;
            tab.discardArmed = false;
            const label = tabLabel(tab);
            if (label !== tab.shownLabel) { tab.shownLabel = label; renderTabs(); } // a front matter title names the tab as it is typed
        },
        onSave: () => saveEdit(),
        onCancel: cancelEdit,
    };
}

// Undo steps for the draft of a tab: typing in one go is one step, and so is each change the AI makes.
const UNDO_PAUSE = 800; // ms
const UNDO_LEVELS = 1000;

/** Keeps the draft as it is now, to go back to; anything undone before is no longer there to redo. */
function remember(tab, text = tab.draft) {
    (tab.undo ??= []).push(text);
    if (tab.undo.length > UNDO_LEVELS) tab.undo.shift();
    tab.redo = [];
}

/** One step back (or forward again) in the draft. */
function undoDraft(tab, back) {
    const from = back ? tab.undo : tab.redo;
    if (tab.draft == null || tab.saving || !from?.length) return;
    (back ? (tab.redo ??= []) : (tab.undo ??= [])).push(tab.draft);
    tab.draft = from.pop();
    tab.lastEdit = 0;
    tab.discardArmed = false;
    if (tab.draft === tab.page.source) tab.aiDraft = false; // all the way back: nothing of the AI's is left
    if (tab !== active) return;
    render();
    const box = $('content').querySelector('.source-edit');
    box?.focus();
    box?.setSelectionRange(box.value.length, box.value.length);
}

/** Starts editing from wherever the page is: the rendered view switches to the source, in a text box with the cursor in it. */
function startEdit() {
    const tab = active;
    if (!tab?.page?.editUrl || tab.draft != null) return;
    tab.editFromRendered = !tab.showSource; // cancelling or saving goes back to reading
    tab.showSource = true;
    tab.draft = tab.page.source;
    tab.undo = [];
    tab.redo = [];
    tab.lastEdit = 0;
    tab.saveError = '';
    tab.discardArmed = false;
    tab.showDiff = false;
    tab.aiDraft = false;
    render();
    refreshAsk(); // the panel now takes changes, not questions
    const box = $('content').querySelector('.source-edit');
    box?.focus();
    box?.setSelectionRange(0, 0);
}

/** Leaves editing. Changes need a second Esc or Cancel so a stray key does not lose them. */
function cancelEdit() {
    const tab = active;
    if (!tab || tab.saving) return;
    if (tab.draft !== tab.page.source && !tab.discardArmed) {
        tab.discardArmed = true;
        tab.saveError = 'You have unsaved changes. Press Esc or Cancel again to discard them.';
        render();
        $('content').querySelector('.source-edit')?.focus();
        return;
    }
    endEdit(tab);
}

/** Leaves editing, dropping the draft. */
function endEdit(tab) {
    if (tab.page?.url === NEW_DOC) return closeTab(tab); // never saved, so there is nothing to go back to
    tab.draft = null;
    tab.saveError = '';
    tab.discardArmed = false;
    tab.showDiff = false;
    tab.aiDraft = false;
    if (tab.editFromRendered) tab.showSource = false;
    if (tab === active) { render(); refreshAsk(); }
}

/** The pencil: starts editing, or stops it; changes not saved yet are asked about first. */
function toggleEdit() {
    const tab = active;
    if (!tab?.page?.editUrl || tab.saving) return;
    if (tab.draft == null) return startEdit();
    if (tab.draft === tab.page.source) return endEdit(tab);
    const dialog = $('unsaved-dialog');
    if (!dialog.open) dialog.showModal();
}
let closeAfter = null; // the tab whose page is to be closed once its unsaved changes are settled
for (const [id, act] of [
    ['unsaved-keep', () => { closeAfter = null; }],
    ['unsaved-discard', () => { const tab = active; const clear = closeAfter === tab; closeAfter = null; endEdit(tab); if (clear && tabs.includes(tab)) clearPage(tab); }],
    ['unsaved-save', async () => { const tab = active; const clear = closeAfter === tab; closeAfter = null; await saveEdit(); if (clear && tab.draft == null && tabs.includes(tab)) clearPage(tab); }],
]) {
    $(id).addEventListener('click', () => { $('unsaved-dialog').close(); act(); });
}

/** Empties the tab, which stays open at the start page. */
function clearPage(tab) {
    tab.page = null;
    tab.pending = '';
    tab.summary = null;
    tab.chat = null;
    tab.chatTried = '';
    tab.showSource = false;
    tab.draft = null;
    tab.showDiff = false;
    tab.aiDraft = false;
    tab.saveError = '';
    if (tab === active) { display(); focusAddress(); }
    renderTabs();
}

/** File > Close Page: closes the page in this tab and keeps the tab; unsaved changes are asked about first. */
function closePage() {
    const tab = active;
    if (!tab || tab.saving || (!tab.page && !tab.pending)) return;
    if (tab.draft != null && tab.draft !== tab.page?.source) {
        closeAfter = tab;
        const dialog = $('unsaved-dialog');
        if (!dialog.open) dialog.showModal();
        return;
    }
    if (tab.page?.url === NEW_DOC) closeTab(tab); // a new page that was never saved has nothing left to show
    else clearPage(tab);
}

// --- send to: the actions of the actions folder ---

/** The text the reader selected in the page, or "". */
function actionSelection() {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || !content.contains(sel.anchorNode)) return '';
    return sel.toString().trim().slice(0, 20000);
}

/** The actions an api call was agreed to, by the fingerprint of the file: changing the file asks again. */
const agreedKey = (a) => `action.agreed.${a.id}`;

/** Asks, once for each version of an action that calls an api, whether it may send the page there. */
function agreeToSend(a) {
    try { if (localStorage.getItem(agreedKey(a)) === a.fingerprint) return Promise.resolve(true); } catch { /* ask */ }
    return new Promise((resolve) => {
        const dialog = $('action-dialog');
        $('action-text').textContent = `"${a.name}" sends the title, address and any text from the page to ${a.host} (${a.method}), using the sign-in saved for that site. It does this each time you choose it, without showing anything first.`;
        const done = (ok) => {
            $('action-allow').onclick = $('action-cancel').onclick = null;
            dialog.close();
            if (ok) try { localStorage.setItem(agreedKey(a), a.fingerprint); } catch { /* asked next time */ }
            resolve(ok);
        };
        $('action-allow').onclick = () => done(true);
        $('action-cancel').onclick = () => done(false);
        dialog.onclose = () => resolve(false);
        dialog.showModal();
    });
}

/** Runs an action of the "Send to" menu for the page in front. */
async function runAction(a) {
    const p = currentPage();
    if (!p?.url || p.kind === 'error' || isSearch(p.url) || p.url === NEW_DOC) return toast('Open a page first.', true);
    if (a.mode === 'api' && !(await agreeToSend(a))) return;
    try {
        const r = await RunAction(a.id, {url: p.url, title: p.title ?? '', source: markdownSource(p), selection: actionSelection()});
        if (r.mode === 'open') { BrowserOpenURL(r.url); toast(`${a.name}: opened in your browser.`); }
        else if (r.mode === 'copy') { await writeClipboard(r.text); toast(`${a.name}: copied.`); }
        else toast(r.message || `${a.name}: sent.`);
    } catch (e) {
        toast(String(e).replace(/^Error: /, ''), true);
    }
}

const sendbar = $('sendbar');
let sendBarOn = (() => { try { return localStorage.getItem('sendbar') === '1'; } catch { return false; } })();
let sendList = [];

/** Shows or hides the Send to toolbar, from the menu. */
function setSendBar(on) {
    sendBarOn = on;
    try { localStorage.setItem('sendbar', on ? '1' : '0'); } catch { /* not kept */ }
    renderSendBar();
    if (on) refreshSendBar();
}

/** Reads the actions folder again, and the toolbar shows what is in it. */
function refreshSendBar() {
    if (!sendBarOn) return;
    Actions().then((list) => { sendList = list ?? []; renderSendBar(); }).catch(() => {});
}

/** Greys the buttons while there is no page to send; called whenever the page shown changes. */
function syncSendBar() {
    if (!sendBarOn) return;
    const p = currentPage();
    const ready = Boolean(p?.url) && p.kind !== 'error';
    for (const b of sendbar.querySelectorAll('button:not(.send-folder)')) b.disabled = !ready;
}

function renderSendBar() {
    sendbar.hidden = !sendBarOn;
    document.documentElement.classList.toggle('has-sendbar', sendBarOn);
    if (!sendBarOn) return;
    const ready = Boolean(currentPage()?.url) && currentPage().kind !== 'error';
    const buttons = sendList.map((a) => {
        const b = Object.assign(document.createElement('button'), {type: 'button', title: a.name, disabled: !ready});
        b.setAttribute('aria-label', a.name);
        if (a.icon.startsWith('data:')) b.append(Object.assign(document.createElement('img'), {src: a.icon, alt: '', width: 16, height: 16}));
        else b.append(Object.assign(document.createElement('span'), {className: 'send-icon', textContent: a.icon || '\u2192'}));
        b.append(Object.assign(document.createElement('span'), {className: 'send-name', textContent: a.name}));
        b.addEventListener('click', () => runAction(a));
        return b;
    });
    const rest = [];
    const edit = Object.assign(document.createElement('button'), {type: 'button', className: 'send-folder send-edit', title: 'Edit the actions: change one or make a new one', textContent: '\u270E'});
    edit.setAttribute('aria-label', 'Edit the actions');
    edit.addEventListener('click', () => goActive((id) => EditActions(id)));
    if (!sendList.length) rest.push(Object.assign(document.createElement('span'), {className: 'note', textContent: 'No actions yet. Put a markdown file in the actions folder.'}));
    if (!document.documentElement.classList.contains('phone')) {
        const folder = Object.assign(document.createElement('button'), {type: 'button', className: 'send-folder', title: 'Open the actions folder', textContent: '\u2026'});
        folder.setAttribute('aria-label', 'Open the actions folder');
        folder.addEventListener('click', () => OpenActionsFolder().catch((e) => toast(String(e), true)));
        rest.push(edit, folder);
    } else rest.push(edit);
    sendbar.replaceChildren(...buttons, ...rest);
}
window.addEventListener('focus', refreshSendBar); // a file was added or changed in the folder meanwhile
if (sendBarOn) { renderSendBar(); refreshSendBar(); }
EventsOn('actionicons', () => { refreshSendBar(); faviconFor = ''; syncFavicon(currentPage()); }); // a picture was got

// --- the icon of the site, in the address bar ---

let faviconFor = ''; // the page the icon in the bar is for, so that it is asked for once

let faviconData = ''; // the icon of the site shown, at the left of the address field
let signedInHost = ''; // the site shown, when the reader is signed in to it: a key at the right
const KEY = 'data:image/svg+xml;utf8,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#7c3aed" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="15" r="4"/><path d="M11 12l9-9M16 7l3 3M14 9l2 2"/></svg>');

/** Paints the icons in the address field: a background picture each, so the layout of the field is the same. */
function paintAddress() {
    const s = address.style;
    const layers = [];
    const positions = [];
    if (faviconData) { layers.push(`url("${faviconData}")`); positions.push('10px center'); }
    if (signedInHost) { layers.push(`url("${KEY}")`); positions.push('right 10px center'); }
    s.backgroundImage = layers.join(', ');
    s.backgroundRepeat = layers.length ? 'no-repeat' : '';
    s.backgroundPosition = positions.join(', ');
    s.backgroundSize = layers.length ? '16px 16px' : '';
    address.classList.toggle('has-icon', Boolean(faviconData));
    address.classList.toggle('has-key', Boolean(signedInHost));
    address.title = signedInHost ? `Signed in to ${signedInHost}` : '';
}

const setAddressIcon = (data) => { faviconData = data; paintAddress(); };

/** Asks for the icon of the page shown (the one its front matter names, else its site's); it comes from the cache, or
 *  later, with an event. Only web pages have one. */
function syncFavicon(p) {
    const key = p && /^https?:/.test(p.url ?? '') && p.kind !== 'error' ? `${p.url}\n${p.favicon ?? ''}` : '';
    if (key === faviconFor) return;
    faviconFor = key;
    setAddressIcon('');
    if (!key) return;
    Favicon(p.url, p.favicon ?? '').then((d) => { if (faviconFor === key) setAddressIcon(d); }).catch(() => {});
}

/** The dialog that shows the data address of an edited data page, to copy. */
function showDataAddress(uri) {
    const box = $('data-uri');
    box.value = uri;
    const dialog = $('data-dialog');
    if (!dialog.open) dialog.showModal();
    box.select();
}
$('data-close').addEventListener('click', () => $('data-dialog').close());
$('data-copy').addEventListener('click', async () => {
    const box = $('data-uri');
    try { await writeClipboard(box.value); toast('Address copied.'); } catch { box.select(); toast('Select the address and copy it.', true); }
});

/** Saves the edited source to the server; the tab then shows what is stored. */
async function saveEdit() {
    const tab = active;
    const p = tab?.page;
    if (!p?.editUrl || tab.draft == null || tab.saving) return;
    tab.saving = true;
    tab.saveError = '';
    render();
    try {
        let fresh;
        if (p.url === NEW_DOC) { // the first save asks where to put the file, and the tab then shows that file
            const r = await SaveNewDocument(tab.id, tab.draft);
            if (r.canceled) { // no file chosen: still a draft
                tab.saving = false;
                if (tab === active) { render(); $('content').querySelector('.source-edit')?.focus(); }
                return;
            }
            fresh = r.page;
        } else {
            fresh = await SaveSource(tab.id, p.url, p.editUrl, p.etag, tab.draft);
        }
        if (fresh.kind === 'error') throw new Error(fresh.error);
        tab.saving = false;
        show(tab, fresh);
        refreshSendBar(); // the page may be an action
        if (fresh.url.startsWith('data:')) showDataAddress(fresh.url); // a data page has nowhere to be saved: its new address is the result
        else if (fresh.submitted) { // the server queued the change: the page is the old text until someone approves it
            const note = fresh.submittedNote ? ` (${fresh.submittedNote.slice(0, 160)})` : '';
            toast(`Submitted for review${note}. The page shows your change once it is approved.`, false, 9000);
        } else toast(p.url === NEW_DOC ? 'Saved.' : p.url.startsWith('file:') ? 'Saved the file.' : 'Saved to the server.');
    } catch (e) {
        tab.saving = false;
        tab.saveError = String(e).replace(/^Error: /, '');
        if (tab === active) { render(); $('content').querySelector('.source-edit')?.focus(); }
    }
}

function render() {
    const p = currentPage();
    syncSendBar();
    syncFavicon(p);
    const noBody = p?.kind === 'markdown' && hasNoBody(p.source); // only front matter: nothing to read, so show its source
    const showSource = (active?.showSource ?? false) || noBody;
    sourceBtn.disabled = p?.kind !== 'markdown';
    // On a page the server lets you change, the source button becomes the pencil that starts editing.
    const editable = p?.kind === 'markdown' && !!p.editUrl;
    if (sourceBtn.classList.contains('pencil') !== editable) {
        sourceBtn.classList.toggle('pencil', editable);
        sourceBtn.innerHTML = editable ? pencilIcon : sourceIcon;
    }
    const editing = editable && active?.draft != null;
    sourceBtn.title = !editable ? 'Show source' : editing ? 'Stop editing (Ctrl/Cmd+E)' : 'Edit this page (Ctrl/Cmd+E)';
    sourceBtn.setAttribute('aria-label', !editable ? 'Show source' : editing ? 'Stop editing' : 'Edit this page');
    if (editable) sourceBtn.setAttribute('aria-pressed', String(editing)); // the pencil stays lit while editing
    $('reading').disabled = !p?.url || p.kind === 'error' || isSearch(p.url) || p.url === NEW_DOC;
    if (!editable) sourceBtn.setAttribute('aria-pressed', String(showSource && !sourceBtn.disabled));
    const body = (
        !p ? view.welcome()
            : p.kind === 'markdown' ? (showSource ? view.source(p, editor()) : view.markdown(p))
            : p.kind === 'fallback' ? view.fallback(p, (origin) => openCredentials({origin}), {rules: () => convertPage('rules'), ai: () => convertPage('ai'), aiAvailable: aiReady(), open: /^https?:\/\//i.test(p.url) ? () => BrowserOpenURL(p.url) : null})
            : view.error(p, () => goActive((id) => Navigate(id, p.url)), (page) => openCredentials({origin: page.origin, kind: page.authScheme === 'basic' ? 'basic' : page.authScheme === 'bearer' ? 'bearer' : 'cookie'}), {offer: (o) => ConnectOffer(o), start: (o) => signInWith(o)}));
    const extras = [active?.draft != null ? null : copyBar(p), variantBar(p), ownAddressBar(p), searchSwitch(p), // no copy button over the text box while editing
        active?.summary ? view.summary(active.summary, closeSummary, cancelSummary) : null].filter(Boolean);
    content.replaceChildren(...extras, body);
    content.scrollTop = 0;
    const passage = p?.kind === 'markdown' && active?.draft == null ? fragmentText(p.url) : '';
    if (passage && active.passageOf !== p) { // an address that points at some text: show where
        active.passageOf = p;
        setTimeout(() => { if (currentPage() === p) finder.reveal(passage); }, 0);
    }
}

// A message from another part of the page (the AI settings), shown the way a toast is.
document.addEventListener('mardo-notice', (e) => toast(e.detail.text, false, e.detail.ms));

// --- save as markdown ---

let toastTimer = 0;

/** A short message at the bottom of the window. */
function toast(text, isError = false, ms = isError ? 6000 : 3000) {
    const box = $('toast');
    box.textContent = text;
    box.classList.toggle('error', isError);
    box.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { box.hidden = true; }, ms);
}

/** A new, empty markdown document in a tab of its own, already in edit mode. It is only a file once saved. */
async function newDocument() {
    const tab = await openTab();
    tab.page = {kind: 'markdown', url: NEW_DOC, title: 'Untitled', html: '', source: '', editUrl: NEW_DOC, etag: '', canBack: false, canForward: false};
    tab.showSource = true;
    tab.draft = '';
    tab.editFromRendered = false;
    tab.saveError = '';
    display();
    renderTabs();
    $('content').querySelector('.source-edit')?.focus();
}

/** Opens a local page (a file) chosen in the system's dialog, in the current tab. */
async function openFile() {
    try {
        const path = await PickFile();
        if (path) goActive((id) => Navigate(id, path));
    } catch (e) {
        toast(String(e), true);
    }
}

/** Saves the page as a markdown file: its own markdown, or the title and links of any other page. */
async function savePage() {
    if (active?.draft != null) return saveEdit(); // while editing the source, Save means save the edit
    const p = currentPage();
    if (!p?.url || p.kind === 'error' || isSearch(p.url)) return toast('Open a page first, then save it.', true);
    try {
        const path = await SavePage(p);
        if (path) toast(`Saved ${path}`);
    } catch (e) {
        toast(String(e), true);
    }
}

// --- copy markdown ---

/** The markdown text of the page, when it has any: not a feed, whose source is XML. */
function markdownSource(p) {
    if (p?.kind !== 'markdown' || !p.source || /^\s*<(\?xml|rss|feed|rdf:RDF)/i.test(p.source.slice(0, 200))) return '';
    return p.source;
}

async function writeClipboard(text) {
    try {
        await navigator.clipboard.writeText(text);
    } catch { // no clipboard permission in this webview: the old way, through a selection
        const box = Object.assign(document.createElement('textarea'), {value: text});
        box.style.cssText = 'position:fixed;opacity:0';
        document.body.append(box);
        box.select();
        const ok = document.execCommand('copy');
        box.remove();
        if (!ok) throw new Error('the clipboard is not available');
    }
}

let copiedTimer = 0;

/** Copies the page's markdown and says so on the button. */
async function copyMarkdown() {
    const text = markdownSource(currentPage());
    if (!text) return;
    const btn = document.querySelector('.copy-md');
    try {
        await writeClipboard(text);
        if (btn) { btn.textContent = '\u2713 Copied'; btn.classList.add('done'); }
    } catch (e) {
        if (btn) btn.textContent = 'Copy failed';
        console.error(e);
    }
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => { if (btn?.isConnected) { btn.textContent = '\u29C9 Copy markdown'; btn.classList.remove('done'); } }, 1800);
}

/** A button at the top right of the page, shown while the pointer is over the page. */
function copyBar(p) {
    if (!markdownSource(p)) return null;
    const btn = Object.assign(document.createElement('button'), {
        type: 'button', className: 'copy-md', textContent: '\u29C9 Copy markdown',
        title: `Copy the markdown of this page (${isMac ? 'Cmd' : 'Ctrl'}+Shift+C)`,
    });
    btn.addEventListener('click', copyMarkdown);
    return Object.assign(document.createElement('div'), {className: 'copy-bar'}, ).appendChild(btn).parentNode;
}

// Bookmark searches live at "bookmarks:search?q=..."; show them as typed: "?query".
// Searches by meaning ("@ ...") live at "bookmarks:meaning?q=..." and are shown as "@query".
const SEARCH_PREFIX = 'bookmarks:search?q=';
const MEANING_PREFIX = 'bookmarks:meaning?q=';
/** The window title, in full wherever it appears. */
const APP_TITLE = 'Mardo - Markdown Browser & Author';

/** The address of a new document that is not on disk yet; it is only in this window until it is saved. */
const NEW_DOC = 'new:document';
const isSearch = (url) => url.startsWith(SEARCH_PREFIX) || url.startsWith(MEANING_PREFIX);
function searchQueryOf(url) {
    const prefix = url.startsWith(MEANING_PREFIX) ? MEANING_PREFIX : SEARCH_PREFIX;
    const words = url.slice(prefix.length).split('&site=')[0]; // a results page may also carry the site search it asked
    try { return decodeURIComponent(words.replace(/\+/g, ' ')); } catch { return ''; }
}
function displayAddress(url) {
    if (url === NEW_DOC) return '';
    if (!isSearch(url)) return friendly(url);
    return (url.startsWith(MEANING_PREFIX) ? '@' : '?') + searchQueryOf(url);
}

/** On an HTML page that was converted to markdown: say how, and offer the page as it was. */
function convertedBar(p) {
    const c = p.converted;
    const bar = document.createElement('p');
    bar.className = 'variant-bar note';
    const back = document.createElement('a');
    back.href = '#';
    back.textContent = 'Show the original page';
    back.addEventListener('click', (e) => { e.preventDefault(); active.page = c.original; display(); });
    if (c.how === 'ai') {
        const secs = Math.max(1, Math.round(c.millis / 1000));
        bar.append(`Converted from HTML by ${c.model} in ${secs}s. A model can leave out or change things, so check it against the original before relying on it. `
            + (c.truncated ? 'The page was too long to send whole; the end is missing. ' : ''), back);
    } else {
        bar.append('Converted from HTML with fixed rules, keeping the main content. Complicated layouts can come out untidy. ', back);
    }
    return bar;
}

/** Converts the HTML page of the active tab to markdown: how is "rules" or "ai" (the configured model). */
async function convertPage(how) {
    const tab = active;
    const original = tab?.page;
    if (original?.kind !== 'fallback') return;
    if (how === 'ai' && !(await aiReady())) return openAI();
    const requestId = newRequestId();
    const title = 'Conversion to markdown';
    const mine = {state: 'loading', url: original.url, requestId, title, busy: how === 'ai' ? 'Asking the model to rewrite the page as markdown' : 'Converting the page to markdown'};
    tab.summary = mine;
    if (tab === active) render();
    let r;
    try {
        r = await ConvertPage(requestId, original.url, how);
    } catch (e) {
        if (tab.summary === mine) { tab.summary = {state: 'error', url: original.url, title, error: String(e)}; if (tab === active) render(); }
        return;
    }
    if (tab.summary !== mine || tab.page !== original) return; // cancelled, or the tab moved on
    tab.summary = null;
    tab.showSource = false;
    tab.page = {...original, kind: 'markdown', title: r.title || original.title, html: r.html, source: r.source, links: [], editUrl: '', etag: '', markdownOf: '',
        converted: {how: r.how, model: r.model, millis: r.millis, truncated: r.truncated, original}};
    renderTabs();
    if (tab === active) display();
}

/** On the markdown version of a web page: say so, and offer the page as it is. */
function variantBar(p) {
    if (p?.converted) return convertedBar(p);
    if (p?.negotiated) { // one address, asked for markdown: the web page is at the same address
        const bar = document.createElement('p');
        bar.className = 'variant-bar note';
        const view = document.createElement('a');
        view.href = '#';
        view.textContent = 'View the web page';
        view.addEventListener('click', (e) => { e.preventDefault(); goActive((id) => NavigateAsIs(id, p.url)); });
        bar.append('This site sent markdown for this address. ', view);
        return bar;
    }
    if (!p?.markdownOf) return null;
    const bar = document.createElement('p');
    bar.className = 'variant-bar note';
    const view = document.createElement('a');
    view.href = '#';
    view.textContent = 'View the original page';
    view.addEventListener('click', (e) => { e.preventDefault(); goActive((id) => NavigateAsIs(id, p.markdownOf)); });
    const site = p.url.endsWith('/llms.txt'); // the site's overview for language models, in markdown
    bar.append(site ? 'This site has no markdown home page; this is its llms.txt. ' : 'This is the markdown version of this page. ', view);
    return bar;
}

/** On a page that the site asks to be read under its own name (bonbon.json, cname): offer that address. */
function ownAddressBar(p) {
    if (!p?.ownAddress) return null;
    const bar = document.createElement('p');
    bar.className = 'variant-bar note';
    const go = document.createElement('a');
    go.href = '#';
    go.textContent = 'Open it there';
    let host = p.ownAddress;
    try { host = new URL(p.ownAddress).host; } catch { /* shown whole */ }
    go.addEventListener('click', (e) => { e.preventDefault(); goActive((id) => Navigate(id, p.ownAddress)); });
    bar.append(`This site has its own address, ${host}. `, go);
    return bar;
}

/** On a search results page: a button to run the same search the other way. */
function searchSwitch(p) {
    if (!p?.url || !isSearch(p.url) || p.kind === 'error') return null;
    const query = searchQueryOf(p.url);
    const byMeaning = p.url.startsWith(MEANING_PREFIX);
    if (!byMeaning && !query.trim()) return null;
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'search-switch';
    if (byMeaning) button.textContent = 'Search by keywords instead';
    else button.append(svgIcon('sparkle'), ' Search by meaning with AI');
    button.title = byMeaning ? 'Every word must match' : 'Sends the titles, addresses, keywords and descriptions of your bookmarks and history to your AI provider';
    button.addEventListener('click', () => goActive((id) => Navigate(id, (byMeaning ? '?' : '@') + query)));
    return button;
}

/** Puts the active tab's page on screen: address bar, buttons, content. */
function display() {
    const p = currentPage();
    const url = p?.url ?? active?.pending ?? '';
    address.value = url ? displayAddress(url) : '';
    document.title = p?.title ? `${p.title} - ${APP_TITLE}` : APP_TITLE;
    $('back').disabled = !p?.canBack;
    $('forward').disabled = !p?.canForward;
    $('reload').disabled = !p?.url || p.url === NEW_DOC;
    shareBtn.disabled = active?.draft == null && (!p?.url || p.url === NEW_DOC || p.kind === 'error');
    document.body.classList.toggle('loading', Boolean(active?.loading));
    render();
    refreshAsk();
    updateCredentialIndicator();
    updateBookmarkIndicator();
}

function show(tab, p) {
    tab.page = p;
    tab.pending = '';
    tab.summary = null;
    if (tab.chat && tab.chat.url !== p.url) tab.chat = null; // a conversation is about one page; it is saved and comes back with the page
    tab.showSource = false;
    tab.draft = null;
    tab.showDiff = false;
    tab.aiDraft = false;
    tab.saveError = '';
    if (tab === active) display();
    renderTabs();
}

/** Runs a navigation for `tab`; a response that was superseded in the meantime is dropped. */
async function go(tab, action, restoreScroll = false) {
    const mine = ++tab.seq;
    // Where the reader was on this page, for coming back to it with Back or Forward.
    if (tab.page?.url) (tab.scrolls ??= new Map()).set(tab.page.url, tab === active ? content.scrollTop : (tab.scroll ?? 0));
    tab.loading = true;
    if (tab === active) document.body.classList.add('loading');
    renderTabs();
    try {
        const p = await action();
        if (p?.kind === 'anchor' && mine === tab.seq && tabs.includes(tab)) { // another place on the same page
            moveWithin(tab, p, restoreScroll);
        } else if (p && mine === tab.seq && tabs.includes(tab)) { // null: nothing to show, the tab stays as it is
            show(tab, p);
            if (restoreScroll) restoreScrollOf(tab, p);
        }
    } catch (e) {
        if (mine === tab.seq && tabs.includes(tab)) show(tab, {kind: 'error', error: String(e), url: tab.page?.url ?? ''});
    } finally {
        if (mine === tab.seq) {
            tab.loading = false;
            if (tab === active) document.body.classList.remove('loading');
            renderTabs();
        }
    }
}

/** Runs `fn(tabId)` in the tab that is active now. */
function goActive(fn, restoreScroll = false) {
    const tab = active;
    return go(tab, () => fn(tab.id), restoreScroll);
}

/** The element a #fragment names: by id, or an anchor with that name. */
function fragmentTarget(fragment) {
    let f = fragment;
    try { f = decodeURIComponent(fragment); } catch { /* used as it is */ }
    return document.getElementById(f) ?? [...content.querySelectorAll('a[name]')].find((x) => x.name === f) ?? null;
}

/**
 * A move to another place on the page that is shown (a #link, or Back and Forward between two of them): the page
 * stays, and the address, the buttons and the scroll position follow. With `restore`, the reader goes back to where
 * they were when they left that entry; otherwise to the place the address names.
 */
function moveWithin(tab, p, restore) {
    tab.page = {...tab.page, url: p.url, canBack: p.canBack, canForward: p.canForward};
    if (tab !== active) { saveTabs(); return; }
    address.value = displayAddress(p.url);
    $('back').disabled = !p.canBack;
    $('forward').disabled = !p.canForward;
    const hash = p.url.includes('#') ? p.url.slice(p.url.indexOf('#') + 1) : '';
    const saved = tab.scrolls?.get(p.url);
    if (restore && saved != null) content.scrollTop = saved;
    else if (hash && !hash.startsWith(':~:')) (fragmentTarget(hash) ?? content).scrollIntoView?.();
    else content.scrollTop = 0;
    tab.scroll = content.scrollTop;
    renderTabs();
    updateBookmarkIndicator();
}

/** After Back or Forward: the page is scrolled to where it was when the reader left it. */
function restoreScrollOf(tab, p) {
    tab.scroll = tab.scrolls?.get(p.url) ?? 0;
    if (tab !== active) return;
    content.scrollTop = tab.scroll;
    requestAnimationFrame(() => { if (tab === active && tab.page === p) content.scrollTop = tab.scroll; }); // once images have their size
}

// --- tabs ---

// The open tabs (their current addresses and which is selected) survive a restart.
// Each one's back/forward history starts afresh.
const TABS_KEY = 'tabs';

/** The page title of a tab, or the one saved for it while it has not been loaded. */
function tabTitle(t) {
    if (t.page) return t.page.kind !== 'error' && t.page.title ? t.page.title : '';
    return t.pendingTitle ?? '';
}

function saveTabs() {
    const kept = tabs.filter((t) => (t.page?.url && t.page.url !== NEW_DOC) || t.pending); // a new document has no file to come back to
    try {
        localStorage.setItem(TABS_KEY, JSON.stringify({
            urls: kept.map((t) => t.page?.url ?? t.pending),
            titles: kept.map((t) => tabTitle(t)), // so tabs that are not loaded yet keep their names
            active: Math.max(0, kept.indexOf(active)),
        }));
    } catch { /* not persisted */ }
}

function savedTabs() {
    try {
        const s = JSON.parse(localStorage.getItem(TABS_KEY));
        const urls = Array.isArray(s?.urls) ? s.urls : [];
        const titles = Array.isArray(s?.titles) ? s.titles : []; // absent in what older versions saved
        const kept = urls.map((u, i) => ({url: u, title: titles[i]})).filter((t) => typeof t.url === 'string' && t.url);
        return {
            urls: kept.map((t) => t.url),
            titles: kept.map((t) => (typeof t.title === 'string' ? t.title : '')),
            active: Number.isInteger(s?.active) ? s.active : 0,
        };
    } catch { return {urls: [], titles: [], active: 0}; }
}

async function restoreTabs() {
    const {urls, titles, active: which} = savedTabs();
    if (!urls.length) return openTab();
    for (const [i, url] of urls.entries()) { // only the selected tab is fetched now; the rest load when shown
        const tab = await newTab();
        tab.pending = url;
        tab.pendingTitle = titles[i] ?? '';
        tabs.push(tab);
    }
    activate(tabs[Math.min(which, tabs.length - 1)]);
}

const tabStrip = $('tabs');
const newTabButton = $('tab-new'); // the last item of the strip, right after the last tab

/** The title in the front matter at the top of the text, or ''. */
export function frontTitle(text) {
    const m = /^---\r?\n([\s\S]*?)\r?\n(?:---|\.\.\.)[ \t]*(?:\r?\n|$)/.exec(text ?? '');
    const t = m && /^title:[ \t]*(.*?)[ \t]*$/mi.exec(m[1]);
    return t ? t[1].replace(/^(["'])(.*)\1$/, '$2') : '';
}

function tabLabel(t) {
    const p = t.page;
    const typed = t.draft != null ? frontTitle(t.draft) : '';
    if (typed) return typed;
    if (!p) return t.pending ? (t.pendingTitle || displayAddress(t.pending)) : 'New tab';
    if (p.title && p.kind !== 'error') return p.title;
    return p.url ? displayAddress(p.url) : 'New tab';
}

function renderTabs() {
    saveTabs();
    tabStrip.replaceChildren(
        ...tabs.map((t) => {
            const label = (t.loading ? '\u2026 ' : '') + tabLabel(t);
            const close = document.createElement('button');
            close.type = 'button';
            close.className = 'tab-close';
            close.textContent = '\u00D7';
            close.title = 'Close tab';
            close.tabIndex = -1;
            close.addEventListener('click', (e) => { e.stopPropagation(); closeTab(t); });
            const el = document.createElement('div');
            el.className = 'tab';
            el.setAttribute('role', 'tab');
            el.setAttribute('aria-selected', String(t === active));
            el.tabIndex = t === active ? 0 : -1;
            el.title = label;
            const text = document.createElement('span');
            text.className = 'tab-title';
            text.textContent = label;
            el.append(text, close);
            el.addEventListener('click', () => activate(t));
            el.addEventListener('contextmenu', (e) => tabMenu(e, t));
            el.addEventListener('auxclick', (e) => { if (e.button === 1) { e.preventDefault(); closeTab(t); } });
            el.addEventListener('keydown', (e) => {
                if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); activate(t); }
            });
            return el;
        }),
        newTabButton,
    );
    tabStrip.querySelector('[aria-selected="true"]')?.scrollIntoView({block: 'nearest', inline: 'nearest'});
}

function activate(tab) {
    if (tab === active) return;
    if (active) active.scroll = content.scrollTop;
    active = tab;
    if (tab.pending && !tab.loading) { // a restored tab shows its page for the first time
        const url = tab.pending;
        go(tab, () => Navigate(tab.id, url));
    }
    display();
    content.scrollTop = tab.scroll ?? 0;
    renderTabs();
    if (!tab.page && !tab.pending) focusAddress();
}

/** Opens a tab, loading `url` in it when given. A background tab does not take over the window. */
const newTab = async () => ({id: await NewTab(), page: null, pending: '', summary: null, chat: null, chatTried: '', showSource: false, seq: 0, loading: false, scroll: 0});

async function openTab(url = '', {background = false, atEnd = false} = {}) {
    const tab = await newTab();
    const at = active && !atEnd ? tabs.indexOf(active) + 1 : tabs.length; // next to the current one, except for a new empty tab: that goes last
    tabs.splice(at, 0, tab);
    if (!background || !active) activate(tab);
    renderTabs();
    if (url) go(tab, () => Navigate(tab.id, url));
    return tab;
}

function closeTab(tab) {
    const i = tabs.indexOf(tab);
    if (i < 0) return;
    CloseTab(tab.id);
    tabs.splice(i, 1);
    if (!tabs.length) {
        active = null;
        openTab(); // the window always has a tab
        return;
    }
    if (tab === active) {
        active = null; // so activate() does not store scroll for the closed tab
        activate(tabs[Math.min(i, tabs.length - 1)]);
    } else {
        renderTabs();
    }
}

/** Closes several tabs at once; `keep` is the tab to show afterwards when the one shown is among them. */
function closeTabs(list, keep) {
    for (const t of list) { CloseTab(t.id); tabs.splice(tabs.indexOf(t), 1); }
    if (list.includes(active)) { active = null; activate(keep); } else renderTabs();
}

/** The right-click menu of a tab. */
function tabMenu(e, t) {
    e.preventDefault();
    const i = tabs.indexOf(t);
    const others = tabs.filter((x) => x !== t);
    const right = tabs.slice(i + 1);
    popupMenu(e.clientX, e.clientY, [
        ['Close Tab', () => closeTab(t)],
        ['Close Other Tabs', () => closeTabs(others, t), {disabled: !others.length}],
        ['Close Tabs to the Right', () => closeTabs(right, t), {disabled: !right.length}],
    ]);
}

function stepTab(by) {
    if (tabs.length < 2) return;
    activate(tabs[(tabs.indexOf(active) + by + tabs.length) % tabs.length]);
}

$('tab-new').addEventListener('click', () => openTab('', {atEnd: true}));

// --- AI summary ---

function closeSummary() {
    active.summary = null;
    render();
}

function cancelSummary() {
    if (active.summary?.requestId) cancelRequest(active.summary.requestId);
    closeSummary();
}

/**
 * Runs an AI tool on the active page and shows the result in the card above it.
 * `tool` is "summary", "outline", "translate" or "digest".
 */
export async function readPage(tool, option = '') {
    const tab = active;
    const url = tab?.page?.url;
    if (!url || $('reading').disabled) return;
    if (!(await aiReady())) return openAI(); // first use: choose a provider
    const requestId = newRequestId();
    const label = {
        summary: ['Summary', 'Reading the page and asking the model'],
        outline: ['Outline', 'Reading the page and outlining it'],
        translate: [`Translation into ${option}`, `Translating the page into ${option}`],
        digest: ['Digest', 'Reading the feed and writing a digest'],
    }[tool];
    const mine = {state: 'loading', url, requestId, title: label[0], busy: label[1]};
    tab.summary = mine;
    if (tab === active) render();
    let result;
    try {
        const r = tool === 'summary' ? await SummarizePage(requestId, url) : await ReadPage(requestId, url, tool, option);
        result = {state: 'done', url, title: label[0], html: r.html, source: r.source, millis: r.millis};
    } catch (e) {
        result = {state: 'error', url, title: label[0], error: String(e)};
    }
    if (tab.summary !== mine || tab.page?.url !== url) return; // cancelled or closed, or the tab moved on
    tab.summary = result;
    if (tab === active) render();
}
const summarize = () => readPage('summary');

function toggleSource() {
    if (sourceBtn.disabled) return;
    active.showSource = !active.showSource;
    render();
}

address.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { const url = real(address.value); goActive((id) => Navigate(id, url)); }
});
address.addEventListener('focus', () => address.select());
$('back').addEventListener('click', () => goActive(Back, true));

/** A long press (or a right click) on Back or Forward lists the nearest fifteen places behind it; choosing one goes there. */
function historyMenu(button, direction) {
    let timer = 0;
    let fired = false;
    const show = async () => {
        fired = true;
        if (!active || button.disabled) return;
        const h = await TabHistory(active.id).catch(() => null);
        if (!h) return;
        const rows = h.items.map((item, i) => ({item, i})).filter(({i}) => (direction < 0 ? i < h.current : i > h.current));
        if (direction < 0) rows.reverse();
        rows.splice(15); // the nearest fifteen, as browsers show
        const at = button.getBoundingClientRect();
        popupMenu(at.left, at.bottom + 4, rows.map(({item, i}) => {
            const text = item.title || displayAddress(item.url);
            return [text.length > 70 ? `${text.slice(0, 69)}\u2026` : text, () => goActive((id) => GoTo(id, i), true)];
        }));
    };
    button.addEventListener('pointerdown', (e) => {
        if (e.button !== 0 || button.disabled) return;
        fired = false;
        timer = setTimeout(show, 500);
    });
    for (const ev of ['pointerup', 'pointerleave', 'pointercancel']) button.addEventListener(ev, () => clearTimeout(timer));
    button.addEventListener('click', (e) => { // the press that opened the list is not also a click on the button
        if (fired) { e.preventDefault(); e.stopImmediatePropagation(); fired = false; }
    }, true);
    button.addEventListener('contextmenu', (e) => { e.preventDefault(); show(); });
}
historyMenu($('back'), -1);
historyMenu($('forward'), 1);
$('forward').addEventListener('click', () => goActive(Forward, true));
$('reload').addEventListener('click', () => goActive(Reload, true));
sourceBtn.addEventListener('click', () => (sourceBtn.classList.contains('pencil') ? toggleEdit() : toggleSource()));

// The iOS share sheet (the app answers the message itself, not the Go core): the page's address, the file it
// came from, or the draft while it is being edited.
const shareBtn = $('share');
shareBtn.hidden = !hasDocuments();
shareBtn.addEventListener('click', () => {
    const item = active?.draft ?? currentPage()?.url;
    if (item && item !== NEW_DOC) (globalThis.webkit?.messageHandlers?.mardo ?? {postMessage: (m) => globalThis.MardoAndroid?.call(0, m.method, m.args)}).postMessage({method: 'Share', args: JSON.stringify([item])});
});

// Links in rendered content navigate inside the browser; fragments scroll in place.
content.addEventListener('click', (e) => {
    const a = e.target.closest('a[href]');
    if (!a) {
        if (e.target.matches('.markdown img')) e.target.classList.toggle('zoomed');
        return;
    }
    e.preventDefault();
    const href = a.getAttribute('href');
    const here = currentPage()?.url ?? '';
    const sameFragment = href.startsWith('#') ? href : (here && a.href.includes('#') && a.href.split('#')[0] === here.split('#')[0]) ? a.href.slice(a.href.indexOf('#')) : '';
    if (sameFragment && here && !e.metaKey && !e.ctrlKey) { // a place on this page: a history entry, as in any browser
        const target = here.split('#')[0] + sameFragment;
        goActive(async (id) => {
            const p = await Anchor(id, target);
            if (p.kind === 'anchor') return p;
            fragmentTarget(sameFragment.slice(1))?.scrollIntoView(); // the history does not know this page: just scroll
            return null;
        });
    } else if (href.startsWith('#')) {
        fragmentTarget(href.slice(1))?.scrollIntoView();
    } else if (e.metaKey || e.ctrlKey) {
        openTab(a.href, {background: true}); // Cmd/Ctrl+click: a tab in the background
    } else {
        const url = a.href;
        // In Bonbon mode a link to a web page opens in the default browser and this tab stays as it is.
        goActive(async (id) => { const p = await FollowLink(id, url); if (p.kind !== 'external') return p; BrowserOpenURL(p.url); return null; });
    }
});
content.addEventListener('auxclick', (e) => { // middle click
    const a = e.button === 1 && e.target.closest('a[href]');
    if (!a || a.getAttribute('href').startsWith('#')) return;
    e.preventDefault();
    openTab(a.href, {background: true});
});

const menuItems = {
    credentials: () => openCredentials({origin: originOf(currentPage()?.url ?? '').replace(/^null$/, '')}),
    ai: openAI,
    savePage,
    openFile,
    newDocument,
    markdownVersions: () => MarkdownVersions(),
    setMarkdownVersions: (on) => SetMarkdownVersions(on),
    bonbon: () => Bonbon(),
    styleSheets: () => StyleSheets(),
    styleSheet: (id) => StyleSheet(id),
    openStylesFolder: () => OpenStylesFolder().catch((e) => toast(String(e), true)),
    sendBar: () => sendBarOn,
    setSendBar,
    markBar: markBarShown,
    setMarkBar,
    setBonbon: (on) => SetBonbon(on),
    history: () => goActive(FullHistory),
    about: () => goActive(About),
    find: () => finder.open(),
    clearHistory: () => ClearHistory(),
};

/** Runs a bookmark tool from the menu by pressing its button, with the panel open so its messages are seen. */
async function bookmarkTool(button) {
    await togglePanel(true);
    $(button).click();
}

/** Find and replace: on a page that can be edited, starts editing first (replacing happens in the text box). */
function openReplace() {
    if (active?.draft == null && currentPage()?.editUrl) startEdit();
    finder.open({replace: true});
}
// A replace is one undo step of the edit, like a change the AI makes.
finder.hooks.checkpoint = () => { if (active?.draft != null) { remember(active); active.lastEdit = Date.now(); } };

// What the macOS menu bar does: its items have the shortcuts, and send their id here.
const menuActions = {
    'new-tab': () => openTab('', {atEnd: true}),
    'close-page': closePage,
    'close-tab': () => closeTab(active),
    'new-document': newDocument,
    'open-file': openFile,
    'save-page': savePage,
    'copy-markdown': copyMarkdown,
    'bookmark-page': () => { if (currentPage()?.url && !isSearch(currentPage().url)) openBookmarkDialog(); },
    reload: () => { if (currentPage()?.url) goActive(Reload, true); },
    back: () => { if (!$('back').disabled) goActive(Back, true); },
    forward: () => { if (!$('forward').disabled) goActive(Forward, true); },
    address: () => address.focus(),
    // The menu item says Show or Hide, so it does just that (the Ctrl+B key moves into the panel first).
    bookmarks: () => ($('bookmarks-panel').hidden ? panelShortcut() : togglePanel(false)),
    sendbar: () => setSendBar(!sendBarOn),
    markbar: () => setMarkBar(!markBarShown()),
    'bookmarks-new-folder': () => bookmarkTool('folder-new'),
    'bookmarks-organize': () => bookmarkTool('bookmarks-organize'),
    'bookmarks-tag': () => bookmarkTool('bookmarks-tag'),
    'bookmarks-import': () => bookmarkTool('bookmarks-import'),
    'bookmarks-export': () => bookmarkTool('bookmarks-export'),
    'bookmarks-export-md': () => bookmarkTool('bookmarks-export-md'),
    'bookmarks-storage': () => bookmarkTool('storage-open'),
    console: toggleConsole,
    source: toggleSource,
    'edit-page': toggleEdit,
    ask: toggleAsk,
    summary: summarize,
    'ai-outline': () => aiTools.outline(),
    'ai-translate': () => aiTools.translate(),
    'ai-compare': () => aiTools.compare(),
    'ai-digest': () => aiTools.digest(),
    'ai-explain': () => aiTools.explain(),
    'ai-simplify': () => aiTools.simplify(),
    ai: menuItems.ai,
    credentials: menuItems.credentials,
    history: menuItems.history,
    about: menuItems.about,
    find: () => finder.open(),
    replace: () => openReplace(),
    'find-next': () => finder.step(1),
    'find-prev': () => finder.step(-1),
    'clear-history': menuItems.clearHistory,
    print: () => (isMac ? PrintNative() : window.print()),
};
// Select All depends on where you are: a field selects its text, the bookmarks panel its
// folders and bookmarks, the chat its whole conversation, otherwise the page.
let lastPane = null;
document.addEventListener('pointerdown', (e) => { lastPane = e.target.closest('#bookmarks-panel, #ask-panel, #content'); }, true);
function selectAll() {
    const focus = document.activeElement;
    if (focus?.matches('input:not([type=checkbox]):not([type=radio]), textarea')) { focus.select(); return; }
    const pane = focus?.closest?.('#bookmarks-panel, #ask-panel, #content') ?? lastPane;
    const target = pane?.id === 'bookmarks-panel' ? $('bookmarks-tree')
        : pane?.id === 'ask-panel' ? $('ask-log')
        : content;
    const range = document.createRange();
    range.selectNodeContents(target);
    const selection = window.getSelection();
    selection.removeAllRanges();
    selection.addRange(range);
}
menuActions['select-all'] = selectAll;
EventsOn('menu', (id) => menuActions[id]?.());

// Bonbon mode: markdown only. The page is loaded again so it shows in the new mode.
const showBonbon = (on) => document.documentElement.classList.toggle('bonbon-mode', !!on);
Bonbon().then(showBonbon).catch(() => {});
// The site says the page of a tab changed (Bonbon live updates): load it again where the reader is, unless the
// reader is editing it, and say nothing if that fails.
EventsOn('pageupdated', ({tab: id, url}) => {
    const tab = tabs.find((t) => t.id === id);
    if (!tab || tab.draft != null || tab.loading || tab.page?.url?.split('#')[0] !== url) return;
    go(tab, async () => { const p = await Reload(tab.id); return p.kind === 'error' ? null : p; }, true);
});
EventsOn('bonbon', (on) => { showBonbon(on); if (currentPage()?.url) goActive(Reload, true); });

// The macOS View menu says Show or Hide for these four. Watch them rather than every place that changes them.
const viewShown = {};
function syncViewMenu() {
    const now = {
        bookmarks: !$('bookmarks-panel').hidden,
        console: !$('console').hidden,
        source: sourceBtn.getAttribute('aria-pressed') === 'true',
        ask: !$('ask-panel').hidden,
        sendbar: !sendbar.hidden,
        markbar: !$('markbar').hidden,
    };
    for (const [id, shown] of Object.entries(now)) {
        if (viewShown[id] !== shown) { viewShown[id] = shown; if (navigator.platform.startsWith('Mac')) SetViewShown(id, shown); }
    }
}
const viewObserver = new MutationObserver(syncViewMenu);
for (const [id, attr] of [['bookmarks-panel', 'hidden'], ['console', 'hidden'], ['ask-panel', 'hidden'], ['sendbar', 'hidden'], ['markbar', 'hidden']]) viewObserver.observe($(id), {attributes: true, attributeFilter: [attr]});
viewObserver.observe(sourceBtn, {attributes: true, attributeFilter: ['aria-pressed']});
syncViewMenu();

// Keyboard shortcuts. "mod" is Cmd on macOS and Ctrl elsewhere.
const isMac = navigator.platform.startsWith('Mac');
/** True for the keys the macOS menu bar has as shortcuts. */
function menuOwnsKey(e, mod, key) {
    if (mod && e.altKey && e.code === 'KeyF') return true; // Find and Replace
    if (!mod || e.altKey) return false;
    if (e.shiftKey) return key === 'a' || key === 'c' || key === 'g';
    return 'lrujbdtwseonfg[]'.includes(key) && key.length === 1;
}
document.addEventListener('keydown', (e) => {
    const mod = isMac ? e.metaKey : e.ctrlKey;
    const key = e.key.toLowerCase();
    if (isMac && menuOwnsKey(e, mod, key)) return; // the menu bar runs it and sends a "menu" event
    let handled = true;
    if ((mod || (!isMac && e.metaKey)) && !e.altKey && key === 'f' && !e.shiftKey) finder.open();
    else if ((mod || (!isMac && e.metaKey)) && !e.altKey && key === 'g') finder.step(e.shiftKey ? -1 : 1);
    else if (!isMac && mod && !e.shiftKey && key === 'h') openReplace();
    else if (mod && key === 'l') address.focus();
    else if (mod && key === 'r') { if (currentPage()?.url) goActive(Reload, true); }
    else if (mod && key === 'u') toggleSource();
    else if (mod && !e.shiftKey && key === 'e') toggleEdit();
    else if (mod && e.shiftKey && key === 'a') toggleAsk();
    else if (mod && e.shiftKey && key === 'c') copyMarkdown();
    else if (mod && !e.shiftKey && key === 's') savePage();
    else if (mod && !e.shiftKey && key === 'o') openFile();
    else if (mod && !e.shiftKey && key === 'n') newDocument();
    else if (mod && key === 't') openTab('', {atEnd: true});
    else if (mod && key === 'w') closeTab(active);
    else if (e.ctrlKey && e.key === 'Tab') stepTab(e.shiftKey ? -1 : 1);
    else if (mod && e.shiftKey && (e.key === '{' || e.key === '[')) stepTab(-1);
    else if (mod && e.shiftKey && (e.key === '}' || e.key === ']')) stepTab(1);
    else if (mod && !e.shiftKey && !e.altKey && /^[1-9]$/.test(e.key)) activate(tabs[e.key === '9' ? tabs.length - 1 : Number(e.key) - 1] ?? active);
    else if (!isMac && mod && !e.shiftKey && key === 'a') selectAll();
    else if (mod && key === 'j') toggleConsole();
    else if (mod && key === 'b') panelShortcut();
    else if (mod && key === 'd') { if (currentPage()?.url && !isSearch(currentPage().url)) openBookmarkDialog(); }
    else if (e.altKey && e.key === 'ArrowLeft' || mod && e.key === '[') { if (!$('back').disabled) goActive(Back, true); }
    else if (e.altKey && e.key === 'ArrowRight' || mod && e.key === ']') { if (!$('forward').disabled) goActive(Forward, true); }
    else handled = false;
    if (handled) e.preventDefault();
});

// Drop a file anywhere on the window to open it.
OnFileDrop((x, y, paths) => {
    if (paths?.length) goActive((id) => Navigate(id, paths[0]));
}, true);

// Files the system asks the app to open (double click, Open With, a second launch): a new tab each, except in an empty one.
async function openRequested() {
    for (const path of (await TakeOpenRequests()) ?? []) {
        if (active && !active.page && !active.pending && !active.loading) goActive((id) => Navigate(id, path));
        else openTab(path);
    }
}
EventsOn('open-requests', openRequested);

// Site credentials: after saving for the page's own site, load it again.
function originOf(url) {
    try { return new URL(url).origin; } catch { return ''; }
}
// The key button lights up when the current site has a saved credential.
async function updateCredentialIndicator() {
    const origin = originOf(currentPage()?.url ?? '');
    const saved = origin && (await credentialOrigins()).has(origin);
    let status = {};
    if (origin) { try { status = await ConnectStatus(origin); } catch { /* the label is the plain one */ } } // from what is kept: no request
    showCredentialState(Boolean(saved), origin, {signedIn: Boolean(status.signedIn), offered: Boolean(status.known && status.available)});
    signedInHost = status.signedIn ? new URL(origin).host : '';
    paintAddress();
}

initCredentials((origin) => {
    updateCredentialIndicator();
    renderMarkBar();
    refreshPanel(); // the bookmarks may be kept at this site: they load, or stop saying they are not connected
    if (currentPage()?.url && originOf(currentPage().url) === originOf(origin)) goActive(Reload, true);
});

// The ribbon is an outline until the current page is bookmarked, then filled.
const bookmarkBtn = $('bookmark-add');
async function updateBookmarkIndicator() {
    const url = currentPage()?.url ?? '';
    bookmarkBtn.disabled = !url || isSearch(url) || url === NEW_DOC; // a results page, or a document with no file yet, is not a page to save
    let found = null;
    if (url && !isSearch(url) && url !== NEW_DOC) {
        try { found = await BookmarkFor(url); } catch { /* no backend: stays unsaved */ }
    }
    if ((currentPage()?.url ?? '') !== url) return; // navigated away meanwhile
    saved = found;
    bookmarkBtn.classList.toggle('active', Boolean(saved));
    bookmarkBtn.title = saved ? 'Edit bookmark (Ctrl/Cmd+D)' : 'Add bookmark (Ctrl/Cmd+D)';
}
function openBookmarkDialog() {
    const p = currentPage();
    openBookmark({bookmark: saved, url: p.url, title: p.title === p.url ? friendly(p.url) : p.title});
}
bookmarkBtn.addEventListener('click', openBookmarkDialog);
initMarkBar((url, newTab) => newTab ? openTab(url, {background: true}) : goActive((id) => Navigate(id, url)), {bookmark: (b) => openBookmark({bookmark: b}), folder: (f) => openBookmarkFolder({folder: f})});
initBookmarks(() => { updateBookmarkIndicator(); refreshPanel(); renderMarkBar(); });
initPanel((url, newTab) => newTab ? openTab(url) : goActive((id) => Navigate(id, url)), () => { updateBookmarkIndicator(); renderMarkBar(); });
initStorage(() => { updateBookmarkIndicator(); refreshPanel(); renderMarkBar(); });
initTagging(() => { updateBookmarkIndicator(); refreshPanel(); renderMarkBar(); });
initOrganize(() => { updateBookmarkIndicator(); refreshPanel(); renderMarkBar(); });

initMenu(menuItems);
initAI();
initAsk({
    tab: () => active,
    open: (url, newTab) => (newTab ? openTab(url, {background: true}) : goActive((id) => Navigate(id, url))),
    /** What an AI edit changes: the text the document would have, shown as an unsaved draft in the page. */
    draft: (tab, text) => {
        if (!tab.page?.editUrl || tab.saving) return;
        if (tab.draft == null) { tab.editFromRendered = !tab.showSource; tab.undo = []; } // cancelling goes back to reading
        remember(tab, tab.draft ?? tab.page.source);
        tab.lastEdit = 0;
        tab.showSource = true;
        tab.draft = text;
        tab.aiDraft = true;
        tab.showDiff = true; // the first thing to see is what changed
        tab.saveError = '';
        tab.discardArmed = false;
        if (tab === active) { render(); refreshAsk(); }
    },
});
initContextMenu({
    page: currentPage,
    selected: selectedText,
    run: (id) => menuActions[id]?.(),
    open: (url, newTab) => (newTab ? openTab(url, {background: true}) : goActive((id) => Navigate(id, url))),
    bookmark: (url, title) => openBookmark({url, title: title || friendly(url)}),
    copy: (text) => writeClipboard(text).catch(console.error),
    ask: askNow,
    translate: () => aiTools.translateSelection(),
    openAsWeb: (url) => goActive((id) => NavigateAsIs(id, url)),
    share: (text) => ShareItem(text),
    toast,
    searchBookmarks: (text) => goActive((id) => Navigate(id, `?${text}`)),
    canGo: (way) => !$(way).disabled,
    canEdit: () => Boolean(currentPage()?.editUrl),
    hasMarkdown: () => Boolean(markdownSource(currentPage())),
});
const aiTools = initReading({page: currentPage, run: readPage, compare: openCompare, toast});
initSplitters();
initCompare({
    // A restored tab that has not been shown yet has only its address (tab.pending); the Go side reads it from there.
    tabs: () => tabs.filter((t) => t.page?.kind !== 'error').map((t) => ({title: tabTitle(t), url: t.page?.url || t.pending || ''})),
    open: (url, newTab) => (newTab ? openTab(url, {background: true}) : openTab(url)),
});
initConsole();
Promise.resolve(restoreTabs()).then(openRequested);
focusAddress();
