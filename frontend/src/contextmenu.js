// The right-click menu. In the page it offers what Mardo can do with a link, an image, the selected
// text or the page itself; in text fields and on the iPhone and iPad the system's own menu stays.
import {hasDocuments} from './address.js';
import {highlightLink, quote} from './textlink.js';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';
import {DownloadAs, ImageData} from '../wailsjs/go/main/App';

/**
 * A small menu at the pointer; it closes on any click elsewhere or Escape.
 * Entries are `[label, run, {disabled}]`, or null for a separator line.
 */
export function popupMenu(x, y, entries) {
    document.getElementById('folder-menu')?.remove();
    const menu = Object.assign(document.createElement('div'), {id: 'folder-menu', className: 'menu settings-menu'});
    menu.setAttribute('role', 'menu');
    const close = () => {
        menu.remove();
        document.removeEventListener('mousedown', away, true);
        document.removeEventListener('keydown', key, true);
        window.removeEventListener('blur', close);
    };
    const away = (e) => { if (!menu.contains(e.target)) close(); };
    const key = (e) => {
        if (e.key === 'Escape') { close(); return; }
        if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
        e.preventDefault();
        const rows = [...menu.querySelectorAll('button:not([disabled])')];
        const i = menu.contains(document.activeElement) ? rows.indexOf(document.activeElement) : (e.key === 'ArrowDown' ? -1 : 0);
        rows[(i + (e.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length]?.focus();
    };
    for (const entry of entries) {
        if (!entry) { menu.append(Object.assign(document.createElement('div'), {className: 'menu-sep'})); continue; }
        const [label, run, {disabled = false} = {}] = entry;
        const b = Object.assign(document.createElement('button'), {type: 'button', className: 'menu-row', textContent: label, disabled});
        b.setAttribute('role', 'menuitem');
        b.addEventListener('mousedown', (e) => e.preventDefault()); // pressing a row must not clear the selection
        b.addEventListener('click', () => { close(); run(); });
        menu.append(b);
    }
    document.body.append(menu);
    menu.style.position = 'fixed';
    menu.style.right = 'auto';
    menu.style.left = `${Math.max(4, Math.min(x, window.innerWidth - menu.offsetWidth - 8))}px`;
    menu.style.top = `${Math.max(4, Math.min(y, window.innerHeight - menu.offsetHeight - 8))}px`;
    document.addEventListener('mousedown', away, true);
    document.addEventListener('keydown', key, true);
    window.addEventListener('blur', close);
}

const clip = (text, n = 24) => { const t = text.replace(/\s+/g, ' ').trim(); return [...t].length > n ? `${[...t].slice(0, n).join('')}…` : t; };
const isMac = navigator.platform.startsWith('Mac');
const isWeb = (url) => /^https?:/i.test(url);

/** An image as a PNG blob, for the clipboard, which takes few formats. */
async function pngOf(url) {
    const blob = await (await fetch(await ImageData(url))).blob();
    if (blob.type === 'image/png') return blob;
    const bitmap = await createImageBitmap(blob);
    const canvas = Object.assign(document.createElement('canvas'), {width: bitmap.width, height: bitmap.height});
    canvas.getContext('2d').drawImage(bitmap, 0, 0);
    return new Promise((done, fail) => canvas.toBlob((b) => (b ? done(b) : fail(new Error('could not convert the image'))), 'image/png'));
}

/**
 * `host` gives the menu what it acts on:
 * `page()` the active page; `open(url, newTab)`; `openAsWeb(url)`; `bookmark(url, title)`; `copy(text)`; `selected()` the selected text
 * in the page; `run(id)` a menu command such as 'reload' or 'find'; `ask(question)`; `translate()` the selection; `searchBookmarks(text)`;
 * `share(text)`; `toast(text, isError)`; `canGo(way)`; `canEdit()`; `hasMarkdown()`.
 */
export function initContextMenu(host) {
    if (hasDocuments()) return; // iPhone and iPad: a long press has the system's menu
    const content = document.getElementById('content');

    // Pressing the right button on selected text must leave the selection alone.
    content.addEventListener('mousedown', (e) => {
        if (e.button !== 2 && !(e.button === 0 && e.ctrlKey)) return;
        const sel = window.getSelection();
        if (!sel || sel.isCollapsed || !sel.rangeCount) return;
        const r = sel.getRangeAt(0);
        if ([...r.getClientRects()].some((b) => e.clientX >= b.left && e.clientX <= b.right && e.clientY >= b.top && e.clientY <= b.bottom)) e.preventDefault();
    }, true);

    const failed = (what) => (e) => host.toast(`${what}: ${e?.message ?? e}`, true);
    const saveFile = (url) => DownloadAs(url).then((path) => path && host.toast(`Saved ${path}`)).catch(failed('Could not save the file'));
    const copyImage = (url) => {
        navigator.clipboard.write([new ClipboardItem({'image/png': pngOf(url)})]).then(() => host.toast('Image copied')).catch(failed('Could not copy the image'));
    };

    content.addEventListener('contextmenu', (e) => {
        if (e.target.closest('textarea, input, select, .copy-bar')) return; // fields keep Cut, Copy and Paste
        const p = host.page();
        const link = e.target.closest('a[href]');
        const href = link && !link.getAttribute('href').startsWith('#') ? link.href : '';
        const image = e.target.closest('img')?.src ?? '';
        const text = host.selected();
        const go = (id) => () => host.run(id);
        const sections = [];

        if (href) {
            sections.push([
                ['Open Link', () => host.open(href, false)],
                ['Open Link in New Tab', () => host.open(href, true)],
                ['Open Link as Web Page', () => host.openAsWeb(href), {disabled: !isWeb(href)}],
                ['Open Link in Browser', () => BrowserOpenURL(href), {disabled: !isWeb(href)}],
            ], [
                ['Download Linked File As…', () => saveFile(href), {disabled: !isWeb(href)}],
                ['Add Link to Bookmarks…', () => host.bookmark(href, link.textContent.trim())],
            ], [
                ['Copy Link', () => host.copy(href)],
                ['Copy as Markdown Link', () => host.copy(`[${link.textContent.trim() || href}](${href})`)],
                ...(isMac ? [['Share…', () => host.share(href)]] : []),
            ]);
        }
        if (image) {
            sections.push([
                ['Open Image in New Tab', () => host.open(image, true), {disabled: !isWeb(image)}],
                ['Save Image As…', () => saveFile(image), {disabled: !isWeb(image)}],
                ['Copy Image', () => copyImage(image), {disabled: !isWeb(image)}],
                ['Copy Image Address', () => host.copy(image)],
            ]);
        }
        if (text) {
            const shown = `“${clip(text)}”`;
            const canLink = Boolean(p?.url) && !p.url.startsWith('bookmarks:') && !p.url.startsWith('new:');
            sections.push([
                ['Copy', () => host.copy(text)],
                ['Copy as Quote', () => host.copy(quote(text, p?.title, p?.url ?? ''))],
                ['Copy Link with Highlight', () => host.copy(highlightLink(p.url, text)), {disabled: !canLink}],
            ], [
                ['Explain', () => host.ask(`Explain this passage from the page, in plain language and with the context of the page:\n\n"${text}"`)],
                ['Simplify', () => host.ask(`Rewrite this passage from the page in simpler words, keeping its meaning:\n\n"${text}"`)],
                ['Translate', () => host.translate()],
            ], [
                [`Find ${shown} in Page`, go('find')],
                [`Search Bookmarks for ${shown}`, () => host.searchBookmarks(text)],
                [`Search the Web for ${shown}`, () => host.open(`https://html.duckduckgo.com/html/?q=${encodeURIComponent(text)}`, true)],
                ...(isMac ? [['Share…', () => host.share(text)]] : []),
            ]);
        }
        if (!sections.length) {
            sections.push([
                ['Back', go('back'), {disabled: !host.canGo('back')}],
                ['Forward', go('forward'), {disabled: !host.canGo('forward')}],
                ['Reload', go('reload'), {disabled: !p?.url}],
            ], [
                ['Copy Markdown', go('copy-markdown'), {disabled: !host.hasMarkdown()}],
                ['Save Page As…', go('save-page'), {disabled: !p?.url}],
                ['Print Page…', go('print'), {disabled: !p?.url}],
                ['Add Bookmark…', go('bookmark-page'), {disabled: !p?.url}],
                ...(isMac ? [['Share Page…', () => host.share(p.url), {disabled: !isWeb(p?.url ?? '')}]] : []),
            ], [
                ['Find…', go('find')],
                ['Select All', go('select-all')],
            ], [
                ['Show Source', go('source')],
                ['Edit Page', go('edit-page'), {disabled: !host.canEdit()}],
            ], [
                ['Summarize Page', go('summary'), {disabled: !p?.url}],
                ['Outline of Page', go('ai-outline'), {disabled: !p?.url}],
            ]);
        }
        const entries = [];
        for (const group of sections) { if (entries.length) entries.push(null); entries.push(...group); }
        e.preventDefault();
        popupMenu(e.clientX, e.clientY, entries);
    });
}
