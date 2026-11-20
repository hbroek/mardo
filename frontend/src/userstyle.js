// Page styles are CSS files in the format of Bonbon's stylesheets: tokens on :root, the page on body and .page, and
// the elements of the rendered markdown (h1, p, a, img, ...). Such a file styles the whole document, but Mardo's
// page shares its document with the toolbar and the panels, so each rule is limited to the page while a style
// is on:
//   :root, :root[data-theme="dark"] ...   -> the tokens are set on the reading area (#content), not the app
//   body, html                            -> the reading area
//   .page, .divPage                       -> the text column (.markdown)
//   anything else (h1, p, a, ...)         -> inside the text column
// Every rule starts with :root[data-pagestyle], which is only set while a style is chosen.

const ON = ':root[data-pagestyle]';

/** Splits a selector list at the commas that are not inside (), [] or a string. */
export function splitSelectors(list) {
    const out = [];
    let depth = 0, quote = '', start = 0;
    for (let i = 0; i < list.length; i++) {
        const c = list[i];
        if (quote) { if (c === '\\') i++; else if (c === quote) quote = ''; continue; }
        if (c === '"' || c === "'") quote = c;
        else if (c === '(' || c === '[') depth++;
        else if (c === ')' || c === ']') depth--;
        else if (c === ',' && depth === 0) { out.push(list.slice(start, i)); start = i + 1; }
    }
    out.push(list.slice(start));
    return out.map((s) => s.trim()).filter(Boolean);
}

/** One selector of a Bonbon stylesheet, limited to the page. */
export function scopeSelector(sel) {
    const root = /^:root(?:\[[^\]]*\]|:[\w-]+(?:\([^)]*\))?)*/.exec(sel);
    if (root) return `${ON}${root[0].slice(5)} #content${sel.slice(root[0].length)}`;
    if (/^(body|html)(?![\w-])/.test(sel)) return `${ON} #content${sel.replace(/^(body|html)/, '')}`;
    if (/^\.(page|divPage)(?![\w-])/.test(sel)) return `${ON} .markdown${sel.replace(/^\.(page|divPage)/, '')}`;
    return `${ON} .markdown ${sel}`;
}

export const scopeSelectors = (list) => splitSelectors(list).map(scopeSelector).join(', ');

function scopeRule(rule) {
    if (typeof CSSStyleRule !== 'undefined' && rule instanceof CSSStyleRule) return `${scopeSelectors(rule.selectorText)} { ${rule.style.cssText} }`;
    if (typeof CSSMediaRule !== 'undefined' && rule instanceof CSSMediaRule) return `@media ${rule.media.mediaText} { ${[...rule.cssRules].map(scopeRule).join('\n')} }`;
    if (typeof CSSSupportsRule !== 'undefined' && rule instanceof CSSSupportsRule) return `@supports ${rule.conditionText} { ${[...rule.cssRules].map(scopeRule).join('\n')} }`;
    if (typeof CSSImportRule !== 'undefined' && rule instanceof CSSImportRule) return ''; // the core has put what it imports in place
    return rule.cssText; // @font-face, @keyframes
}

/** The CSS of a page style with every rule limited to the page. It is parsed in a document of its own, so
 *  nothing of it applies while it is read. */
export function scopeCss(css) {
    const doc = document.implementation.createHTMLDocument('');
    const style = doc.createElement('style');
    style.textContent = css;
    doc.head.append(style);
    return [...style.sheet.cssRules].map(scopeRule).join('\n');
}

/** Puts a page style on the page (css is the file's text), or takes the style off (null). */
export function showPageStyle(css) {
    let el = document.getElementById('page-style');
    if (css == null) { el?.remove(); return; }
    if (!el) {
        el = Object.assign(document.createElement('style'), {id: 'page-style'});
        document.head.append(el); // after the app's own CSS, so on equal specificity the style wins
    }
    el.textContent = scopeCss(css);
}
