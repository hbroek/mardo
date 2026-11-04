// Builds the DOM for each kind of page. No state lives here.
import {svgIcon} from './icons.js';
import {diffLines, withContext} from './diff.js';

function el(tag, props = {}, ...children) {
    const node = Object.assign(document.createElement(tag), props);
    node.append(...children);
    return node;
}

function originOf(url) {
    try {
        const o = new URL(url).origin;
        return o === 'null' || !o.startsWith('http') ? '' : o;
    } catch {
        return '';
    }
}

export function welcome() {
    const shortcuts = [
        ['Ctrl/Cmd + L', 'Focus the address bar'],
        ['Ctrl/Cmd + N', 'New page: write markdown from scratch'],
        ['Ctrl/Cmd + O', 'Open a page from disk'],
        ['Ctrl/Cmd + E', 'Edit this page, when you are allowed to change it'],
        ['Ctrl/Cmd + S', 'Save the page (or your edit)'],
        ['Ctrl/Cmd + T / W', 'New tab / close tab'],
        ['Ctrl + Tab, Ctrl/Cmd + 1-9', 'Switch tabs'],
        ['Ctrl/Cmd + R', 'Reload'],
        ['Alt + ← / →', 'Back / forward (Cmd + [ / ] on a Mac)'],
        ['Ctrl/Cmd + U', 'Toggle markdown source'],
        ['Ctrl/Cmd + B / D', 'Bookmarks panel / bookmark this page'],
        ['Ctrl/Cmd + Shift + A', 'Interact with this page: ask, or describe a change'],
        ['Ctrl/Cmd + Shift + C', 'Copy the page\'s markdown (Copy All)'],
        ['Ctrl/Cmd + J', 'Show / hide the network console'],
    ];
    if (document.documentElement.classList.contains('phone')) return welcomePhone();
    return el('section', {className: 'welcome'},
        el('h1', {textContent: 'Mardo - Markdown Browser & Author'}),
        el('p', {textContent: 'Enter an http, https or file address above, or start a new markdown page with File > New Page. Pages are requested as markdown and rendered here. Anything else shows its title and links.'}),
        el('dl', {}, ...shortcuts.flatMap(([k, v]) => [el('dt', {textContent: k}), el('dd', {textContent: v})])),
    );
}

/** The welcome page on a phone: there are no keys, so it points at the buttons along the bottom. */
function welcomePhone() {
    const tools = [
        ['Bookmarks', 'Your saved pages, in folders.'],
        ['Bookmark', 'Save the page you are reading.'],
        ['AI tools', 'Summarize, outline, translate or explain the page.'],
        ['Ask', 'Ask a question about the page.'],
        ['Tabs', 'Switch between pages, or open a new one.'],
        ['Menu', 'History, sign-ins, themes, and writing a page of your own.'],
    ];
    return el('section', {className: 'welcome'},
        el('h1', {textContent: 'Mardo'}),
        el('p', {textContent: 'A reader for markdown pages. Type an address or a search above. Pages are requested as markdown and shown cleanly.'}),
        el('dl', {}, ...tools.flatMap(([k, v]) => [el('dt', {textContent: k}), el('dd', {textContent: v})])),
    );
}

export function markdown(p) {
    // p.html is sanitized by the backend.
    return el('article', {className: 'markdown', innerHTML: p.html});
}

/** A busy line with a spinner and the seconds waited, for models that take their time. */
export function waiting(text) {
    const p = el('p', {className: 'note busy', textContent: `${text}\u2026`});
    const started = Date.now();
    const timer = setInterval(() => {
        if (!p.isConnected) return clearInterval(timer); // the card was replaced or closed
        p.textContent = `${text}\u2026 ${Math.round((Date.now() - started) / 1000)}s`;
    }, 1000);
    return p;
}

/** 4.2s under ten seconds, then whole seconds, then minutes. */
export function seconds(ms) {
    const s = ms / 1000;
    if (s < 10) return `${s.toFixed(1)}s`;
    if (s < 100) return `${Math.round(s)}s`;
    return `${Math.floor(s / 60)}m ${Math.round(s % 60)}s`;
}

/** The AI summary shown above a page: loading, the summary, or what went wrong. */
export function summary(s, onClose, onCancel) {
    const body = s.state === 'loading' ? el('div', {}, waiting(s.busy ?? 'Reading the page and asking the model'),
        el('button', {type: 'button', className: 'summary-cancel', textContent: 'Cancel', onclick: onCancel}))
        : s.state === 'error' ? el('p', {className: 'error-text', textContent: s.error})
        : el('div', {className: 'markdown', innerHTML: s.html}); // sanitized by the backend
    return el('aside', {className: 'summary', ariaLabel: s.title ?? 'AI summary'},
        el('div', {className: 'summary-head'},
            el('strong', {}, svgIcon('sparkle'), ` ${s.title ?? 'Summary'}`),
            el('button', {type: 'button', className: 'summary-close', title: 'Close', textContent: '\u00D7', onclick: onClose})),
        body,
        s.state === 'done' ? el('p', {className: 'note', textContent: `Written by ${s.source} in ${seconds(s.millis)}; it can be wrong.`}) : '');
}

/**
 * The markdown source. When the server allows saving it (p.editUrl), an Edit button turns it into a text box;
 * `ed` is {draft, saving, error, onEdit, onInput, onSave, onCancel}, with draft null while not editing.
 */
export function source(p, ed) {
    if (!p.editUrl || !ed) return el('pre', {className: 'source'}, el('code', {textContent: p.source}));
    if (ed.draft === null) {
        return el('div', {className: 'source-wrap'},
            el('div', {className: 'edit-bar'},
                el('span', {className: 'note', textContent: 'The server lets you change this page.'}),
                el('button', {type: 'button', textContent: 'Edit', onclick: ed.onEdit})),
            el('pre', {className: 'source'}, el('code', {textContent: p.source})));
    }
    const area = el('textarea', {className: 'source-edit', value: ed.draft, spellcheck: false, autocapitalize: 'off', autocorrect: 'off', disabled: ed.saving, ariaLabel: 'Markdown source'});
    area.addEventListener('input', () => ed.onInput(area.value));
    area.addEventListener('keydown', (e) => {
        if ((e.metaKey || e.ctrlKey) && (e.key === 'Enter' || e.key.toLowerCase() === 's')) { e.preventDefault(); ed.onSave(); }
        else if ((e.metaKey || e.ctrlKey) && !e.altKey && e.key.toLowerCase() === 'z') { e.preventDefault(); (e.shiftKey ? ed.onRedo : ed.onUndo)(); }
        else if (e.ctrlKey && e.key.toLowerCase() === 'y') { e.preventDefault(); ed.onRedo(); }
        else if (e.key === 'Escape') { e.preventDefault(); ed.onCancel(); }
    });
    const changed = ed.draft !== p.source;
    return el('div', {className: 'source-wrap editing'},
        el('div', {className: 'edit-bar'},
            el('button', {type: 'button', className: 'primary', textContent: ed.saving ? 'Saving\u2026' : 'Save', disabled: ed.saving, onclick: ed.onSave}),
            el('button', {type: 'button', textContent: 'Cancel', disabled: ed.saving, onclick: ed.onCancel}),
            el('button', {type: 'button', textContent: 'Undo', title: 'Undo (Ctrl/Cmd+Z)', disabled: ed.saving || !ed.canUndo, onclick: ed.onUndo}),
            el('button', {type: 'button', textContent: 'Redo', title: 'Redo (Shift+Ctrl/Cmd+Z)', disabled: ed.saving || !ed.canRedo, onclick: ed.onRedo}),
            el('button', {type: 'button', textContent: ed.diff ? 'Back to editing' : 'Show changes', disabled: ed.saving || (!changed && !ed.diff), onclick: ed.onDiff}),
            el('span', {className: 'note', textContent: ed.aiDraft ? 'Drafted by AI. Review the changes, then Save; Cancel discards them.' : 'Ctrl/Cmd+S saves, Esc cancels.'})),
        ed.error ? el('p', {className: 'error-text', textContent: ed.error}) : '',
        ed.diff ? changes(p.source, ed.draft) : area);
}

/** What a draft changes: removed lines and added lines, with a few unchanged lines around them. */
function changes(before, after) {
    const lines = withContext(diffLines(before, after));
    if (!lines.some((l) => l.type !== 'gap' && l.type !== 'same')) return el('p', {className: 'note', textContent: 'No changes.'});
    const mark = {same: '  ', del: '- ', add: '+ ', gap: ''};
    return el('pre', {className: 'source diff'}, ...lines.map((l) => l.type === 'gap'
        ? el('span', {className: 'diff-gap', textContent: '\u2026\n'})
        : el('span', {className: `diff-${l.type}`, textContent: `${mark[l.type]}${l.text}\n`})));
}

export function fallback(p, onCredentials) {
    const links = p.links ?? [];
    const origin = originOf(p.url);
    return el('section', {className: 'fallback'},
        el('h1', {textContent: p.title}),
        el('p', {className: 'note', textContent: `No markdown available (${p.contentType || 'unknown type'}). ${links.length} link${links.length === 1 ? '' : 's'} found.`}),
        origin ? el('p', {className: 'note'}, 'Expected a signed-in page? ',
            el('a', {href: '#', textContent: `Add credentials for ${origin}`, onclick: (e) => { e.preventDefault(); onCredentials(origin); }})) : '',
        el('ul', {}, ...links.map((l) => el('li', {},
            el('a', {href: l.href, textContent: l.text}),
            l.text === l.href ? '' : el('span', {className: 'href', textContent: l.href}),
        ))),
    );
}

export function error(p, onRetry, onCredentials) {
    const needsLogin = (p.status === 401 || p.status === 403) && p.origin;
    return el('section', {className: 'error'},
        el('h1', {textContent: needsLogin ? 'Sign-in required' : 'Cannot display this page'}),
        el('p', {textContent: p.error}),
        needsLogin ? el('p', {className: 'note', textContent: 'This site did not accept the request as anonymous. Add your session cookies or a token for it.'}) : '',
        p.url ? el('p', {className: 'href', textContent: p.url}) : '',
        needsLogin ? el('button', {className: 'primary', textContent: `Add credentials for ${p.origin}`, onclick: () => onCredentials(p)}) : '',
        p.url ? el('button', {textContent: 'Retry', onclick: onRetry}) : '',
    );
}
