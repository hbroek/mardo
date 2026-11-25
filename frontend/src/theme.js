// The menu at the far right of the toolbar: site credentials, theme and color
// scheme. Theme is System, Light or Dark; the palettes of the color schemes live
// in schemes.css and `swatch` is only the preview shown here.
import {svgIcon} from './icons.js';
import {showPageStyle} from './userstyle.js';

const $ = (id) => document.getElementById(id);
const root = document.documentElement;

const themes = [
    {id: 'system', name: 'Follow system', icon: 'contrast'},
    {id: 'light', name: 'Light', icon: 'sun'},
    {id: 'dark', name: 'Dark', icon: 'moon'},
];
const schemes = [
    {id: '', name: 'Violet', swatch: ['#4f46e5', '#c026d3', '#f97316']},
    {id: 'ocean', name: 'Ocean', swatch: ['#0369a1', '#0891b2', '#f59e0b']},
    {id: 'forest', name: 'Forest', swatch: ['#15803d', '#0d9488', '#ca8a04']},
    {id: 'sunset', name: 'Sunset', swatch: ['#dc2626', '#ea580c', '#db2777']},
    {id: 'rose', name: 'Rose', swatch: ['#be185d', '#e11d48', '#7c3aed']},
    {id: 'graphite', name: 'Graphite', swatch: ['#475569', '#64748b', '#0ea5e9']},
];

function read(key) {
    try { return localStorage.getItem(key); } catch { return null; }
}

function write(key, value) {
    try {
        if (value) localStorage.setItem(key, value);
        else localStorage.removeItem(key);
    } catch { /* not persisted */ }
}

let theme = themes.some((t) => t.id === read('theme')) ? read('theme') : 'system';
let scheme = schemes.some((s) => s.id === read('scheme')) ? read('scheme') : '';
let pageStyle = read('pagestyle') || ''; // the file name of a page style (the stylesheets folder), or '' for Mardo's own look
let pageStyles = [];                       // what the stylesheets folder has, read each time the menu opens
let credentials = {saved: false, origin: '', signedIn: false, offered: false};

function applyTheme(id) {
    theme = id;
    if (id === 'system') delete root.dataset.theme;
    else root.dataset.theme = id;
    write('theme', id === 'system' ? '' : id);
}

function applyScheme(id) {
    scheme = id;
    if (id) root.dataset.scheme = id;
    else delete root.dataset.scheme;
    write('scheme', id);
}

/** Puts the page style `id` on the page, or Mardo's own look when it is '' or cannot be read. */
async function applyPageStyle(id, actions) {
    let css = null;
    if (id) {
        try { css = await actions.styleSheet(id); } catch { id = ''; }
    }
    pageStyle = id;
    if (id) {
        root.dataset.pagestyle = id;
        showPageStyle(css);
    } else {
        delete root.dataset.pagestyle;
        showPageStyle(null);
    }
    write('pagestyle', id);
}

/** The key lights up the menu button when the current site has a saved credential. */
export function showCredentialState(saved, origin, {signedIn = false, offered = false} = {}) {
    credentials = {saved, origin, signedIn, offered};
    $('menu').classList.toggle('active', saved);
}

const hostName = (origin) => { try { return new URL(origin).host; } catch { return origin; } };

/** The menu entry for site credentials says what is known about the current site, without asking anyone. */
function credentialsLabel() {
    if (credentials.signedIn) return `Signed in to ${hostName(credentials.origin)}`;
    if (credentials.saved) return 'Site credentials (saved for this site)';
    if (credentials.offered) return `Sign in with ${hostName(credentials.origin)}\u2026`;
    return 'Site credentials\u2026';
}

function item(role, label, checked, onClick, lead) {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'menu-row';
    b.setAttribute('role', role);
    if (checked !== null) b.setAttribute('aria-checked', String(checked));
    if (lead) b.append(lead);
    b.append(label);
    b.addEventListener('click', onClick);
    return b;
}

function heading(text) {
    const h = document.createElement('div');
    h.className = 'menu-heading';
    h.textContent = text;
    return h;
}

/** The icon in front of a menu row. */
const icon = (name) => { const s = span('menu-icon'); s.append(svgIcon(name)); return s; };
const span = (className, text = '') => Object.assign(document.createElement('span'), {className, textContent: text});

/** `actions` has the functions behind the menu's first entries: credentials, ai, savePage, history, clearHistory and about. */
export function initMenu(actions) {
    const btn = $('menu');
    const menu = $('settings-menu');

    const close = () => {
        menu.hidden = true;
        btn.setAttribute('aria-expanded', 'false');
    };

    function build() {
        const creds = item('menuitem', credentialsLabel(), null,
            () => { close(); if (credentials.offered && !credentials.signedIn && !credentials.saved && actions.signIn) actions.signIn(credentials.origin); else actions.credentials(); }, icon('key'));
        const markdownEntry = item('menuitemcheckbox', 'Look for markdown versions of pages', markdownVersions, async () => { markdownVersions = !markdownVersions; try { await actions.setMarkdownVersions(markdownVersions); } catch { markdownVersions = !markdownVersions; } build(); });
        const bonbonEntry = item('menuitemcheckbox', 'Bonbon mode: markdown only', bonbon, async () => { bonbon = !bonbon; try { await actions.setBonbon(bonbon); } catch { bonbon = !bonbon; } build(); });
        const sendEntry = item('menuitemcheckbox', 'Send to toolbar', sendBar, () => { sendBar = !sendBar; actions.setSendBar(sendBar); build(); });
        const markEntry = item('menuitemcheckbox', 'Bookmarks toolbar', markBar, () => { markBar = !markBar; actions.setMarkBar(markBar); build(); });
        const aiEntry = item('menuitem', 'AI assistant\u2026', null, () => { close(); actions.ai(); }, icon('sparkle'));
        const newEntry = item('menuitem', 'New page', null, () => { close(); actions.newDocument(); }, icon('page'));
        const openEntry = item('menuitem', 'Open page\u2026', null, () => { close(); actions.openFile(); }, icon('folder'));
        const saveEntry = item('menuitem', 'Save page\u2026', null, () => { close(); actions.savePage(); }, icon('save'));
        const historyEntry = item('menuitem', 'History', null, () => { close(); actions.history(); }, icon('clock'));
        const findEntry = item('menuitem', 'Find in page', null, () => { close(); actions.find(); }, icon('search'));
        const aboutEntry = item('menuitem', 'About Mardo', null, () => { close(); actions.about(); }, icon('info'));
        const clearEntry = item('menuitem', 'Clear history', null, async () => { close(); await actions.clearHistory(); }, icon('trash'));
        // On a phone the toolbar leaves out what is rarely needed there; it is here instead.
        const phone = root.classList.contains('phone');
        const press = (id) => () => { close(); $(id).click(); };
        const phoneRows = !phone ? [] : [
            ...(!$('share').hidden ? [item('menuitem', 'Share\u2026', null, press('share'), icon('share'))] : []),
        ];
        // Reading comes first on a phone; writing is a section of its own at the end.
        const postEntries = [
            ...actions.addKinds().map((k, i) => item('menuitem', `New ${k.name} here\u2026`, null, () => { close(); actions.addPage(i); }, icon('page'))),
            ...(actions.canDelete() ? [item('menuitem', 'Delete this page\u2026', null, () => { close(); actions.deletePage(); }, icon('trash'))] : []),
        ];
        const writing = [newEntry, ...postEntries, openEntry, saveEntry, ...(phone ? [item('menuitem', 'View source or edit', null, press('source'), icon('code'))] : [])];
        const themeRows = [
            heading('Theme'),
            ...themes.map((t) => item('menuitemradio', t.name, t.id === theme, () => { applyTheme(t.id); build(); }, icon(t.icon))),
            heading('Color scheme'),
            ...schemes.map((s) => {
                const sw = span('swatch');
                sw.style.background = `linear-gradient(90deg, ${s.swatch.join(', ')})`;
                return item('menuitemradio', s.name, s.id === scheme, () => { applyScheme(s.id); build(); }, sw);
            }),
            heading('Page style'),
            item('menuitemradio', 'Mardo', pageStyle === '', async () => { await applyPageStyle('', actions); build(); }),
            ...pageStyles.map((p) => item('menuitemradio', p.name, p.id === pageStyle, async () => { await applyPageStyle(p.id, actions); build(); })),
            ...(phone ? [] : [item('menuitem', 'Open stylesheets folder\u2026', null, () => { close(); actions.openStylesFolder(); }, icon('folder'))]),
        ];
        menu.replaceChildren(...(phone
            ? [...phoneRows, findEntry, historyEntry, creds, aiEntry, markdownEntry, bonbonEntry, sendEntry, markEntry, clearEntry, heading('Write'), ...writing, ...themeRows, heading('About'), aboutEntry]
            : [findEntry, creds, aiEntry, ...writing, historyEntry, clearEntry, markdownEntry, bonbonEntry, sendEntry, markEntry, ...themeRows, aboutEntry]));
    }

    let sendBar = false; // the Send to toolbar
    let markBar = false; // the bookmarks toolbar
    let bonbon = false; // Bonbon mode, which the menu bar can also change
    let markdownVersions = true; // whether .md versions of web pages are looked for; the menu bar can change it too

    function open() {
        build();
        actions.styleSheets().then((list) => { pageStyles = list ?? []; if (!menu.hidden) build(); }).catch(() => {});
        sendBar = Boolean(actions.sendBar());
        markBar = Boolean(actions.markBar());
        actions.bonbon().then((on) => { if (on !== bonbon) { bonbon = on; if (!menu.hidden) build(); } }).catch(() => {});
        actions.markdownVersions().then((on) => { if (on !== markdownVersions) { markdownVersions = on; if (!menu.hidden) build(); } }).catch(() => {});
        menu.hidden = false;
        btn.setAttribute('aria-expanded', 'true');
        menu.querySelector('button')?.focus();
    }

    applyTheme(theme);
    applyScheme(scheme);
    if (pageStyle) applyPageStyle(pageStyle, actions);
    btn.addEventListener('click', () => (menu.hidden ? open() : close()));
    document.addEventListener('mousedown', (e) => {
        if (!menu.hidden && !e.target.closest('#settings-menu') && !e.target.closest('#menu')) close();
    });
    document.addEventListener('keydown', (e) => {
        if (menu.hidden) return;
        if (e.key === 'Escape') { close(); btn.focus(); return; }
        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            e.preventDefault();
            const rows = [...menu.querySelectorAll('button')];
            const i = rows.indexOf(document.activeElement);
            rows[(i + (e.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length].focus();
        }
    });
}
