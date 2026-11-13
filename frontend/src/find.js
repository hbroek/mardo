// Find in the page: a small bar at the top right of the page (not a dialog: the page stays usable) that
// counts the matches of what you type, scrolls to the first one half a second after you stop typing, and steps
// through them with the arrows (Enter and Shift+Enter too). Matches are wrapped in <mark> while the bar is open.
const content = document.getElementById('content');
const root = document.documentElement;
const DELAY = 500;
const SKIP = 'script, style, button, textarea, svg, .copy-bar, .find-skip';

const bar = document.createElement('div');
bar.id = 'find-bar';
bar.setAttribute('role', 'search');
bar.hidden = true;
bar.innerHTML = `
    <div class="find-row">
    <button type="button" id="find-toggle" title="Replace" aria-label="Show replace" aria-expanded="false" hidden><svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M9 6l6 6-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></button>
    <input type="text" id="find-input" placeholder="Find in page" aria-label="Find in page" enterkeyhint="search" autocomplete="off" autocapitalize="off" autocorrect="off" spellcheck="false">
    <span id="find-count" role="status" aria-live="polite"></span>
    <button type="button" id="find-prev" title="Previous match (Shift+Enter)" aria-label="Previous match"><svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M6 15l6-6 6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></button>
    <button type="button" id="find-next" title="Next match (Enter)" aria-label="Next match"><svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M6 9l6 6 6-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></button>
    <button type="button" id="find-close" title="Close (Esc)" aria-label="Close find"><svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg></button>
    </div>
    <div class="find-row" id="replace-row" hidden>
    <input type="text" id="replace-input" placeholder="Replace with" aria-label="Replace with" autocomplete="off" autocapitalize="off" autocorrect="off" spellcheck="false">
    <button type="button" class="text" id="replace-one" title="Replace this match and go to the next (Enter)">Replace</button>
    <button type="button" class="text" id="replace-all" title="Replace every match">All</button>
    </div>`;
document.body.append(bar);
const input = bar.querySelector('#find-input');
const count = bar.querySelector('#find-count');
const toggle = bar.querySelector('#find-toggle');
const replaceRow = bar.querySelector('#replace-row');
const replaceInput = bar.querySelector('#replace-input');
let replaceOn = false; // the replace row is open (it only shows while the page is a text box)
let spans = []; // where the matches are in the text box
/** What main.js gives to keep the edit's undo steps: called before the text box changes. */
export const hooks = {checkpoint: () => {}};

let hits = [];
let current = -1;
let timer = 0;
let mirror = null; // while the page is a text box: a copy of its text behind which the matches are marked
let busy = false; // our own changes to the page are not the page changing

/** Takes the marks out again and puts the text back together. */
function clear() {
    busy = true;
    mirror?.remove();
    mirror = null;
    const parents = new Set();
    for (const m of content.querySelectorAll('mark.find-hit')) {
        parents.add(m.parentNode);
        m.replaceWith(...m.childNodes);
    }
    for (const p of parents) p.normalize();
    hits = [];
    spans = [];
    current = -1;
    watch();
}

function textNodes() {
    const nodes = [];
    const walker = document.createTreeWalker(content, NodeFilter.SHOW_TEXT, {
        acceptNode: (n) => (n.nodeValue.trim() && !n.parentElement.closest(SKIP) ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT),
    });
    while (walker.nextNode()) nodes.push(walker.currentNode);
    return nodes;
}

/** Marks every match of `query`, case-insensitively, and shows the count. Returns the marks. */
export function mark(query) {
    clear();
    if (!query) { show(); return hits; }
    busy = true;
    const needle = query.toLowerCase();
    const area = content.querySelector('textarea.source-edit');
    if (area) { markArea(area, needle); watch(); return hits; }
    for (const node of textNodes()) {
        const text = node.nodeValue.toLowerCase();
        const at = [];
        for (let i = text.indexOf(needle); i >= 0; i = text.indexOf(needle, i + needle.length)) at.push(i);
        for (const i of at.reverse()) { // from the end, so earlier offsets stay right
            const range = document.createRange();
            range.setStart(node, i);
            range.setEnd(node, i + needle.length);
            const m = document.createElement('mark');
            m.className = 'find-hit';
            range.surroundContents(m);
            hits.unshift(m);
        }
    }
    watch();
    return hits;
}

/**
 * A text box cannot hold marks, so a copy of its text goes over it, with the same font and wrapping, text invisible
 * and the matches colored; it follows the box's scrolling.
 */
function markArea(area, needle) {
    const text = area.value;
    const lower = text.toLowerCase();
    mirror = document.createElement('div');
    mirror.className = 'find-mirror';
    mirror.setAttribute('aria-hidden', 'true');
    mirror.style.left = `${area.offsetLeft}px`;
    mirror.style.top = `${area.offsetTop}px`;
    mirror.style.width = `${area.clientWidth + 2}px`; // inside the scrollbar, plus the border
    mirror.style.height = `${area.offsetHeight}px`;
    let from = 0;
    for (let i = lower.indexOf(needle); i >= 0; i = lower.indexOf(needle, from)) {
        mirror.append(text.slice(from, i));
        const m = document.createElement('mark');
        m.className = 'find-hit';
        m.textContent = text.slice(i, i + needle.length);
        mirror.append(m);
        hits.push(m);
        spans.push({start: i, end: i + needle.length});
        from = i + needle.length;
    }
    mirror.append(text.slice(from) + '\n');
    area.after(mirror);
    area.onscroll = () => { if (mirror) mirror.scrollTop = area.scrollTop; };
    mirror.scrollTop = area.scrollTop;
}

/** The replace row shows while the page is a text box and replace is on. */
function layout() {
    const editing = !!content.querySelector('textarea.source-edit');
    toggle.hidden = !editing;
    replaceRow.hidden = !(editing && replaceOn);
    toggle.setAttribute('aria-expanded', String(!replaceRow.hidden));
    toggle.classList.toggle('on', !replaceRow.hidden);
}

function show() {
    layout();
    count.textContent = notice || (!input.value ? '' : hits.length ? `${current + 1} of ${hits.length}` : 'No results');
    notice = '';
    bar.classList.toggle('none', !!input.value && !hits.length);
}

/** Makes match `i` the current one (wrapping around), and scrolls to it unless `scroll` is false. */
function select(i, scroll = true) {
    if (!hits.length) { current = -1; show(); return; }
    hits[current]?.classList.remove('find-current');
    current = (i + hits.length) % hits.length;
    hits[current].classList.add('find-current');
    if (scroll && mirror) {
        const area = mirror.previousElementSibling;
        area.scrollTop = hits[current].offsetTop - area.clientHeight / 2;
        mirror.scrollTop = area.scrollTop;
    } else if (scroll) hits[current].scrollIntoView({block: 'center', inline: 'nearest'});
    show();
}

function search(scroll = true, keep = false) {
    clearTimeout(timer);
    const was = current;
    mark(input.value);
    select(keep ? Math.min(was, hits.length - 1) : 0, scroll);
}

let notice = '';
/** Changes the text box to `text` as one step of its undo, as if it was typed. */
function setText(area, text, caret) {
    hooks.checkpoint();
    area.value = text;
    area.setSelectionRange(caret, caret);
    area.dispatchEvent(new Event('input', {bubbles: true}));
}

/** Replaces the current match and goes to the next one. */
export function replaceOne() {
    const area = content.querySelector('textarea.source-edit');
    if (!area || area.disabled) return;
    if (!hits.length) search();
    const span = spans[current];
    if (!span) return;
    const text = replaceInput.value;
    setText(area, area.value.slice(0, span.start) + text + area.value.slice(span.end), span.start + text.length);
    const after = span.start + text.length; // not the replacement itself, even if it contains the phrase
    mark(input.value);
    const next = spans.findIndex((x) => x.start >= after);
    select(next < 0 ? 0 : next);
}

/** Replaces every match; one undo step. */
export function replaceAll() {
    const area = content.querySelector('textarea.source-edit');
    if (!area || area.disabled) return;
    if (!hits.length) search(false);
    if (!spans.length) return;
    const text = replaceInput.value;
    let out = '';
    let from = 0;
    for (const x of spans) { out += area.value.slice(from, x.start) + text; from = x.end; }
    out += area.value.slice(from);
    const n = spans.length;
    setText(area, out, 0);
    mark(input.value);
    select(-1, false);
    notice = `${n} replaced`;
    count.textContent = notice;
}

/** Next (1) or previous (-1) match. */
export function step(dir) {
    if (bar.hidden) { open(); return; }
    clearTimeout(timer);
    if (input.value && !hits.length) search();
    else select(current + dir);
}

export function open({replace = false} = {}) {
    if (replace && content.querySelector('textarea.source-edit')) replaceOn = true;
    const selected = window.getSelection()?.toString().trim();
    if (selected && !selected.includes('\n') && selected.length < 100 && !bar.contains(document.activeElement)) input.value = selected;
    bar.hidden = false;
    place();
    layout();
    const target = replace && !replaceRow.hidden && input.value ? replaceInput : input;
    target.focus();
    target.select();
    if (input.value) search();
}

export function close() {
    if (bar.hidden) return;
    clearTimeout(timer);
    clear();
    bar.hidden = true;
    unwatch();
    input.blur();
}

/** Sits at the top right of the page, wherever the page is (the side panels move it). */
function place() {
    const r = content.getBoundingClientRect();
    bar.style.top = `${r.top + 8}px`;
    bar.style.right = `${Math.max(8, window.innerWidth - r.right + 18)}px`;
}

// When the page changes under the bar (another tab, another page), find again, without jumping around.
const observer = new MutationObserver(() => {
    if (busy) { busy = false; return; }
    clearTimeout(timer);
    timer = setTimeout(() => { if (!bar.hidden) search(false); }, 150);
});
function watch() { observer.takeRecords(); observer.observe(content, {childList: true, subtree: true, characterData: true}); busy = false; }
function unwatch() { observer.disconnect(); }

input.addEventListener('input', () => {
    clearTimeout(timer);
    timer = setTimeout(search, DELAY);
});
input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); step(e.shiftKey ? -1 : 1); }
    else if (e.key === 'Escape') { e.preventDefault(); close(); }
});
bar.querySelector('#find-prev').addEventListener('click', () => step(-1));
bar.querySelector('#find-next').addEventListener('click', () => step(1));
bar.querySelector('#find-close').addEventListener('click', close);
new ResizeObserver(() => { if (!bar.hidden) { place(); if (mirror) search(false, true); } }).observe(content);
// Typing in the text box changes what is there to find.
content.addEventListener('input', (e) => {
    if (bar.hidden || !e.target.matches?.('textarea.source-edit')) return;
    clearTimeout(timer);
    timer = setTimeout(() => search(false, true), 250);
});
window.addEventListener('resize', () => { if (!bar.hidden) place(); });

toggle.addEventListener('click', () => { replaceOn = !replaceOn; layout(); (replaceOn ? replaceInput : input).focus(); });
replaceInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); replaceOne(); }
    else if (e.key === 'Escape') { e.preventDefault(); close(); }
});
bar.querySelector('#replace-one').addEventListener('click', replaceOne);
bar.querySelector('#replace-all').addEventListener('click', replaceAll);
