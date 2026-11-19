// The Files view of the side panel: the folders and the markdown and text files in the home folder.
// Folders open on a click and are listed only when they are shown; a file opens in the page.
import {svgIcon} from './icons.js';
import {ListFolder, RenamePath} from '../wailsjs/go/main/App';
import {popupMenu} from './contextmenu.js';

const tree = document.getElementById('files-tree');
const OPEN_KEY = 'files.open';
const ROOT_KEY = 'files.root';
const rootBar = document.getElementById('files-root');

let root = '';             // the pinned folder; '' is the home folder
let home = '';             // the home folder, as the last listing reported it
const asked = new Set();   // folders the reader opened in this run
let open = new Set();      // paths of expanded folders
let currentKey = '';       // the row that is (or was last) focused
let onOpenUrl = () => {};
const actions = new WeakMap();

try { root = localStorage.getItem(ROOT_KEY) ?? ''; } catch { root = ''; }
try { open = new Set(JSON.parse(localStorage.getItem(OPEN_KEY) ?? '[]')); } catch { open = new Set(); }
const setRoot = (path, refresh = true) => {
    root = path;
    try { path ? localStorage.setItem(ROOT_KEY, path) : localStorage.removeItem(ROOT_KEY); } catch { /* not remembered */ }
    if (refresh) refreshFiles();
};
// macOS asks for permission the first time an app reads these. A folder that was left open in an earlier
// run is therefore not listed again until the reader opens it, so the question does not come out of the blue.
const guarded = (path) => home && ['Documents', 'Desktop', 'Downloads'].some((d) => path === `${home}/${d}`);
const shown = (path) => open.has(path) && (!guarded(path) || asked.has(path));
/** After a folder was renamed, what was remembered about it and what is inside it follows. */
function followRename(from, to) {
    const moved = (p) => (p === from ? to : p.startsWith(from + '/') ? to + p.slice(from.length) : p);
    open = new Set([...open].map(moved));
    saveOpen();
    if (root) setRoot(moved(root), false);
    currentKey = currentKey.slice(0, 2) + moved(currentKey.slice(2));
}

/** Replaces the name on `r` with a text box. Enter renames, Escape or leaving the box gives up. */
function startRename(r, e) {
    const label = r.querySelector('.tree-label');
    if (!label || r.querySelector('.rename')) return;
    const input = el('input', {type: 'text', className: 'rename', value: e.name, spellcheck: false, autocomplete: 'off', ariaLabel: `New name for ${e.name}`});
    let done = false;
    const stop = () => { done = true; r.querySelector('.rename-error')?.remove(); input.replaceWith(label); label.focus?.(); r.focus(); };
    const fail = (msg) => {
        r.querySelector('.rename-error')?.remove();
        r.append(el('div', {className: 'rename-error error-text', textContent: msg, role: 'alert'}));
    };
    const commit = async () => {
        const name = input.value.trim();
        if (!name || name === e.name) return stop();
        try {
            const to = await RenamePath(e.path, name);
            done = true;
            if (e.folder) followRename(e.path, to);
            else if (currentKey === `d:${e.path}`) currentKey = `d:${to}`;
            await refreshFiles();
            focusFiles();
        } catch (err) {
            fail(String(err).replace(/^Error: /, ''));
            input.focus();
        }
    };
    input.addEventListener('keydown', (ev) => {
        ev.stopPropagation();
        if (ev.key === 'Enter') { ev.preventDefault(); commit(); }
        else if (ev.key === 'Escape') { ev.preventDefault(); stop(); }
    });
    input.addEventListener('blur', () => { if (!done) setTimeout(() => { if (!done && document.activeElement !== input) stop(); }, 0); });
    input.addEventListener('click', (ev) => ev.stopPropagation());
    input.addEventListener('contextmenu', (ev) => ev.stopPropagation()); // the text box keeps the system menu
    label.replaceWith(input);
    input.focus();
    // Select the name without its extension, so typing replaces just that.
    const dot = e.folder ? -1 : e.name.lastIndexOf('.');
    input.setSelectionRange(0, dot > 0 ? dot : e.name.length);
}

const saveOpen = () => { try { localStorage.setItem(OPEN_KEY, JSON.stringify([...open])); } catch { /* not remembered */ } };

function el(tag, props = {}, ...children) {
    const node = Object.assign(document.createElement(tag), props);
    node.append(...children);
    return node;
}

function row(key, depth, ...children) {
    const r = el('div', {className: 'tree-row', tabIndex: -1}, ...children);
    r.dataset.key = key;
    r.setAttribute('role', 'treeitem');
    r.setAttribute('aria-level', String(depth + 1));
    r.style.paddingLeft = `${8 + depth * 16}px`;
    return r;
}

function folderRow(e, depth) {
    const isOpen = shown(e.path);
    const toggle = () => {
        if (isOpen) open.delete(e.path); else { open.add(e.path); asked.add(e.path); }
        saveOpen();
        refreshFiles();
    };
    const label = el('button', {type: 'button', className: 'tree-label', tabIndex: -1, title: e.path, onclick: toggle},
        el('span', {className: 'caret', textContent: isOpen ? '▾' : '▸'}),
        el('span', {className: 'folder-name'}, svgIcon('folder'), e.name));
    const r = row(`f:${e.path}`, depth, label);
    r.classList.add('folder');
    r.addEventListener('contextmenu', (ev) => { // right click: make this folder the root of the list
        ev.preventDefault();
        popupMenu(ev.clientX, ev.clientY, [['Pin as Root', () => setRoot(e.path)], ['Rename\u2026', () => startRename(r, e)]]);
    });
    r.setAttribute('aria-expanded', String(isOpen));
    r.setAttribute('aria-label', e.name);
    actions.set(r, {folder: true, open: isOpen, activate: toggle, toggle, rename: () => startRename(r, e)});
    return r;
}

function fileRow(e, depth) {
    const go = (ev) => onOpenUrl(e.url, Boolean(ev?.metaKey || ev?.ctrlKey));
    const label = el('button', {type: 'button', className: 'tree-label plain', tabIndex: -1, title: e.path, textContent: e.name, onclick: go});
    const r = row(`d:${e.path}`, depth + 1, label); // one level in, like a bookmark under its folder
    r.setAttribute('aria-label', e.name);
    r.addEventListener('contextmenu', (ev) => { // right click: open in a new tab
        ev.preventDefault();
        popupMenu(ev.clientX, ev.clientY, [['Open in New Tab', () => go({metaKey: true})], ['Rename\u2026', () => startRename(r, e)]]);
    });
    actions.set(r, {folder: false, activate: go, rename: () => startRename(r, e)});
    return r;
}

const rowsOf = () => [...tree.querySelectorAll('.tree-row')];
const levelOf = (r) => Number(r.getAttribute('aria-level'));

function setCurrent(r) {
    for (const x of tree.querySelectorAll('.tree-row[tabindex="0"]')) x.tabIndex = -1;
    r.tabIndex = 0;
    currentKey = r.dataset.key;
}

tree.addEventListener('focusin', (e) => {
    const r = e.target.closest('.tree-row');
    if (r) setCurrent(r);
});

tree.addEventListener('keydown', (e) => {
    const r = e.target;
    if (!r.classList?.contains('tree-row') || e.metaKey || e.ctrlKey || e.altKey) return;
    const rows = rowsOf();
    const i = rows.indexOf(r);
    const act = actions.get(r);
    let handled = true;
    switch (e.key) {
        case 'ArrowDown': rows[i + 1]?.focus(); break;
        case 'ArrowUp': rows[i - 1]?.focus(); break;
        case 'Home': rows[0]?.focus(); break;
        case 'End': rows[rows.length - 1]?.focus(); break;
        case 'ArrowRight':
            if (!act.folder) break;
            if (!act.open) act.toggle();
            else if (rows[i + 1] && levelOf(rows[i + 1]) > levelOf(r)) rows[i + 1].focus();
            break;
        case 'ArrowLeft':
            if (act.folder && act.open) { act.toggle(); break; }
            rows.slice(0, i).reverse().find((x) => levelOf(x) < levelOf(r))?.focus();
            break;
        case 'Enter':
        case ' ':
            act.activate();
            break;
        case 'F2':
            act.rename();
            break;
        case 'Escape':
            document.getElementById('address').focus();
            break;
        default:
            handled = false;
    }
    if (handled) e.preventDefault();
});

/** Lists `dir` ('' is the home folder) and, below it, every folder that is shown open. */
async function level(dir, depth) {
    const list = await ListFolder(dir);
    home = list.home || home;
    const nodes = [];
    for (const e of list.entries ?? []) {
        if (e.folder) {
            nodes.push(folderRow(e, depth));
            if (shown(e.path)) {
                let inner;
                try { inner = await level(e.path, depth + 1); } catch { inner = [el('div', {className: 'tree-empty', textContent: 'Cannot be read', style: `padding-left:${8 + (depth + 2) * 16}px`})]; }
                if (!inner.length) inner = [el('div', {className: 'tree-empty', textContent: 'No markdown or text files', style: `padding-left:${8 + (depth + 2) * 16}px`})];
                nodes.push(...inner);
            }
        } else {
            nodes.push(fileRow(e, depth));
        }
    }
    return nodes;
}

/** While a folder is pinned, a bar above the list names it and unpins it. */
function showRoot() {
    rootBar.hidden = !root;
    if (!root) return;
    const name = root.split(/[\\/]/).filter(Boolean).pop() ?? root;
    rootBar.replaceChildren(
        el('span', {className: 'root-name', title: root}, svgIcon('folder'), name),
        el('button', {type: 'button', className: 'unpin', title: 'Unpin this folder and show the home folder', ariaLabel: 'Unpin', textContent: '\u00d7', onclick: () => setRoot('')}),
    );
}

/** Rebuilds the list from the disk. */
export async function refreshFiles() {
    if (tree.hidden) return;
    showRoot();
    try {
        const nodes = await level(root, 0);
        const hadFocus = tree.contains(document.activeElement);
        tree.replaceChildren(...(nodes.length ? nodes : [el('p', {className: 'note tree-note', textContent: 'No markdown or text files in your home folder.'})]));
        const rows = rowsOf();
        const current = rows.find((r) => r.dataset.key === currentKey) ?? rows[0];
        if (current) {
            setCurrent(current);
            if (hadFocus) current.focus();
        }
    } catch (e) {
        tree.replaceChildren(el('p', {className: 'error-text tree-note', textContent: String(e)}));
    }
}

/** Moves into the list: the row that was last used, or the first. */
export function focusFiles() {
    const rows = rowsOf();
    (rows.find((r) => r.dataset.key === currentKey) ?? rows[0])?.focus();
}

/** `openUrl(url, newTab)` is called when a file is chosen. */
export function initFiles(openUrl) {
    onOpenUrl = openUrl;
    document.getElementById('files-refresh').addEventListener('click', refreshFiles);
}
