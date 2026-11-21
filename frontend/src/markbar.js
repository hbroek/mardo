// The bookmarks toolbar: the bookmarks of one folder, as buttons under the Send to toolbar.
// A folder inside it opens a list of what it holds. The ⌄ at the end chooses which folder the bar shows.
import {BookmarkFolders, BookmarksIn} from '../wailsjs/go/main/App';

const $ = (id) => document.getElementById(id);
const bar = $('markbar');
const keep = (k, v) => { try { v == null ? localStorage.removeItem(k) : localStorage.setItem(k, v); } catch { /* not kept */ } };
const read = (k) => { try { return localStorage.getItem(k); } catch { return null; } };

let on = read('markbar') === '1';
let folder = read('markbarFolder'); // null: none chosen yet. "": the top level. Otherwise a folder id.
let open = (url, newTab) => {};
let edit = {bookmark: () => {}, folder: () => {}};
let popup = null;

export const markBarShown = () => on;

/** Shows or hides the bar, from a menu. */
export function setMarkBar(show) {
    on = show;
    keep('markbar', on ? '1' : '0');
    render();
}

export function initMarkBar(openUrl, editors) {
    open = openUrl;
    edit = editors;
    document.addEventListener('click', (e) => { if (popup && !popup.contains(e.target) && !bar.contains(e.target)) closePopup(); });
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape') closePopup(); });
    render();
}

const el = (tag, props = {}, ...kids) => { const n = Object.assign(document.createElement(tag), props); n.append(...kids); return n; };
const nameOf = (b) => b.title || b.url.replace(/^[a-z]+:\/\//i, '').replace(/\/$/, '');

function closePopup() { popup?.remove(); popup = null; }

/** A list under a button: `rows` are {text, indent, onPick}. */
function showPopup(anchor, rows, empty) {
    closePopup();
    const list = el('div', {className: 'menu markbar-menu', role: 'menu'});
    if (!rows.length) list.append(el('div', {className: 'menu-empty', textContent: empty}));
    for (const r of rows) {
        const b = el('button', {type: 'button', className: 'menu-item', role: 'menuitem'}, el('span', {className: 'menu-title', textContent: r.text}));
        if (r.indent) b.style.paddingLeft = `${12 + r.indent * 14}px`;
        b.addEventListener('click', (e) => { closePopup(); r.onPick(e); });
        if (r.onEdit) b.addEventListener('contextmenu', (e) => { e.preventDefault(); closePopup(); r.onEdit(); });
        list.append(b);
    }
    document.body.append(list);
    const rect = anchor.getBoundingClientRect();
    Object.assign(list.style, {position: 'fixed', top: `${rect.bottom + 4}px`, width: 'min(22rem, 90vw)', maxHeight: '60vh'});
    const left = Math.min(rect.left, innerWidth - list.offsetWidth - 8);
    list.style.left = `${Math.max(8, left)}px`;
    popup = list;
    list.querySelector('button')?.focus();
}

async function pickFolder(anchor) {
    let all = [];
    try { all = (await BookmarkFolders()) ?? []; } catch { /* no backend */ }
    const kids = new Map();
    for (const f of all) kids.set(f.parentId, [...(kids.get(f.parentId) ?? []), f]);
    const rows = [{text: 'All bookmarks (the top level)', indent: 0, onPick: () => choose('')}];
    const walk = (parent, depth, seen) => {
        for (const f of (kids.get(parent) ?? []).sort((a, b) => a.name.localeCompare(b.name))) {
            if (seen.has(f.id)) continue;
            rows.push({text: f.name, indent: depth, onPick: () => choose(f.id)});
            walk(f.id, depth + 1, new Set([...seen, f.id]));
        }
    };
    walk('', 1, new Set());
    showPopup(anchor, rows, 'No folders yet. Make one in the bookmarks panel.');
}

function choose(id) {
    folder = id;
    keep('markbarFolder', id);
    render();
}

async function openFolder(anchor, f) {
    let marks = [];
    try { marks = (await BookmarksIn(f.id)) ?? []; } catch { /* shown as empty */ }
    showPopup(anchor, marks.map((m) => ({text: nameOf(m), onPick: (e) => open(m.url, e.metaKey || e.ctrlKey), onEdit: () => edit.bookmark(m)})), 'This folder is empty.');
}

let seq = 0;
/** Paints the bar again from the bookmarks: when it is shown, when the folder changes, and when a bookmark changes. */
export async function render() {
    bar.hidden = !on;
    document.documentElement.classList.toggle('has-markbar', on);
    if (!on) { closePopup(); return; }
    const mine = ++seq;
    let marks = [], subs = [], failed = '';
    if (folder != null) {
        try {
            marks = (await BookmarksIn(folder)) ?? [];
            subs = ((await BookmarkFolders()) ?? []).filter((f) => f.parentId === folder);
        } catch (e) { failed = String(e).replace(/^Error: /, ''); }
    }
    if (mine !== seq) return; // a newer paint is on its way
    const items = [
        ...subs.sort((a, b) => a.name.localeCompare(b.name)).map((f) => {
            const b = el('button', {type: 'button', className: 'mark-folder', title: f.name}, el('span', {className: 'mark-glyph', textContent: '\u{1F4C1}'}), el('span', {textContent: f.name}));
            b.addEventListener('click', () => openFolder(b, f));
            b.addEventListener('contextmenu', (e) => { e.preventDefault(); closePopup(); edit.folder(f); }); // right click: the folder's editor
            return b;
        }),
        ...marks.map((m) => {
            const b = el('button', {type: 'button', title: m.url}, el('span', {textContent: nameOf(m)}));
            b.addEventListener('click', (e) => open(m.url, e.metaKey || e.ctrlKey));
            b.addEventListener('contextmenu', (e) => { e.preventDefault(); closePopup(); edit.bookmark(m); }); // right click: the bookmark's editor
            b.addEventListener('auxclick', (e) => { if (e.button === 1) { e.preventDefault(); open(m.url, true); } });
            return b;
        }),
    ];
    const pick = el('button', {type: 'button', className: 'mark-pick', title: 'Choose the folder shown here'});
    pick.innerHTML = '<svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M5 9l7 7 7-7"/></svg>';
    pick.setAttribute('aria-label', 'Choose the folder shown here');
    pick.addEventListener('click', () => pickFolder(pick));
    const note = folder == null ? 'Choose a folder to show here.' : failed ? failed : !items.length ? 'This folder is empty.' : '';
    bar.replaceChildren(...items, ...(note ? [el('span', {className: 'note', textContent: note})] : []), pick);
}
