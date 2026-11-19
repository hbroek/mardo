// An optional markdown cheatsheet for the editor: a panel over the editor's right edge. Choosing an entry
// puts that syntax in the text, around the selected text if there is some.
const pref = () => { try { return localStorage.getItem('edit.cheatsheet') === '1'; } catch { return false; } };
const setPref = (on) => { try { localStorage.setItem('edit.cheatsheet', on ? '1' : '0'); } catch { /* not remembered */ } };

// kind 'wrap': text goes between before and after. kind 'line': before goes at the start of each selected line.
// kind 'block': the text is put on lines of its own.
const ENTRIES = [
    ['Basics', [
        ['Bold', '**bold**', {kind: 'wrap', before: '**', after: '**', hint: 'bold'}],
        ['Italic', '*italic*', {kind: 'wrap', before: '*', after: '*', hint: 'italic'}],
        ['Strikethrough', '~~gone~~', {kind: 'wrap', before: '~~', after: '~~', hint: 'gone'}],
        ['Inline code', '`code`', {kind: 'wrap', before: '`', after: '`', hint: 'code'}],
        ['Link', '[text](https://…)', {kind: 'wrap', before: '[', after: '](https://)', hint: 'text'}],
        ['Image', '![alt](image.png)', {kind: 'wrap', before: '![', after: '](image.png)', hint: 'alt text'}],
    ]],
    ['Headings and quotes', [
        ['Heading 1', '# Title', {kind: 'line', before: '# '}],
        ['Heading 2', '## Section', {kind: 'line', before: '## '}],
        ['Heading 3', '### Part', {kind: 'line', before: '### '}],
        ['Quote', '> quoted', {kind: 'line', before: '> '}],
        ['Divider', '---', {kind: 'block', text: '---'}],
    ]],
    ['Lists', [
        ['Bullet list', '- item', {kind: 'line', before: '- '}],
        ['Numbered list', '1. item', {kind: 'line', before: '1. '}],
        ['Task list', '- [ ] to do', {kind: 'line', before: '- [ ] '}],
    ]],
    ['Blocks', [
        ['Code block', '```\ncode\n```', {kind: 'fence'}],
        ['Table', '| a | b |\n|---|---|\n| 1 | 2 |', {kind: 'block', text: '| Column | Column |\n|--------|--------|\n|        |        |'}],
        ['Footnote', 'text[^1]\n\n[^1]: note', {kind: 'block', text: 'text[^1]\n\n[^1]: The note.'}],
    ]],
];

/** Puts a snippet into the text box the way an entry says, and tells the editor the text changed. */
export function insertSyntax(area, how) {
    const text = area.value;
    const start = area.selectionStart;
    const end = area.selectionEnd;
    const chosen = text.slice(start, end);
    let from = start;
    let to = end;
    let put;
    let select = null; // [from, to] to select afterward, relative to the start of the inserted text
    const atLineStart = start === 0 || text[start - 1] === '\n';
    if (how.kind === 'wrap') {
        const inner = chosen || how.hint;
        put = how.before + inner + how.after;
        select = [how.before.length, how.before.length + inner.length];
    } else if (how.kind === 'line') {
        // Whole lines: from the start of the first selected line to the end of the last one.
        from = text.lastIndexOf('\n', start - 1) + 1;
        const stop = text.indexOf('\n', Math.max(end, start));
        to = stop === -1 ? text.length : stop;
        const lines = text.slice(from, to).split('\n');
        put = lines.map((l) => how.before + l).join('\n');
    } else {
        const body = how.kind === 'fence' ? '```\n' + (chosen || 'code') + '\n```' : how.text;
        if (how.kind === 'fence' && chosen) { to = end; }
        const lead = atLineStart ? '' : '\n\n';
        const tail = end >= text.length || text[end] === '\n' ? '\n' : '\n\n';
        put = lead + body + tail;
        select = how.kind === 'fence' ? [lead.length + 4, lead.length + 4 + (chosen || 'code').length] : null;
    }
    area.focus();
    area.setRangeText(put, from, to, 'end');
    if (select) area.setSelectionRange(from + select[0], from + select[1]);
    area.dispatchEvent(new Event('input', {bubbles: true}));
}

/** Returns {button, panel}: the edit bar's button and the panel to put in the editor's wrapper. */
export function cheatsheet(area) {
    let on = pref();
    const button = Object.assign(document.createElement('button'), {type: 'button', textContent: 'Cheatsheet', title: 'Markdown syntax to look up or put in the text'});
    const panel = document.createElement('aside');
    panel.className = 'cheatsheet';
    panel.setAttribute('aria-label', 'Markdown cheatsheet');
    const head = document.createElement('div');
    head.className = 'cheat-head';
    const title = Object.assign(document.createElement('strong'), {textContent: 'Markdown'});
    const close = Object.assign(document.createElement('button'), {type: 'button', textContent: '×', title: 'Close the cheatsheet', ariaLabel: 'Close the cheatsheet'});
    head.append(title, close);
    panel.append(head);
    for (const [group, items] of ENTRIES) {
        const h = Object.assign(document.createElement('h4'), {textContent: group});
        const list = document.createElement('div');
        list.className = 'cheat-list';
        for (const [name, syntax, how] of items) {
            const row = Object.assign(document.createElement('button'), {type: 'button', className: 'cheat-row', title: `Put "${name}" in the text`});
            row.append(Object.assign(document.createElement('span'), {textContent: name}), Object.assign(document.createElement('code'), {textContent: syntax}));
            row.addEventListener('mousedown', (e) => e.preventDefault()); // the text keeps its selection and focus
            row.addEventListener('click', () => insertSyntax(area, how));
            list.append(row);
        }
        panel.append(h, list);
    }
    const apply = () => {
        panel.hidden = !on;
        button.setAttribute('aria-pressed', String(on));
        button.classList.toggle('on', on);
    };
    const flip = () => { on = !on; setPref(on); apply(); if (!on) area.focus(); };
    button.addEventListener('click', flip);
    close.addEventListener('click', flip);
    apply();
    return {button, panel};
}
