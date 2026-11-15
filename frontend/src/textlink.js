// Text for copying: a link that marks a passage, and a passage as a quote.

/** The address of the page with the text marked, as `#:~:text=` links do: the whole text, or its first and last words. */
export function highlightLink(pageUrl, text) {
    const enc = (t) => encodeURIComponent(t).replace(/-/g, '%2D');
    const words = text.replace(/\s+/g, ' ').trim().split(' ');
    const fragment = words.length > 10 ? `${enc(words.slice(0, 4).join(' '))},${enc(words.slice(-4).join(' '))}` : enc(words.join(' '));
    return `${pageUrl.replace(/#.*$/, '')}#:~:text=${fragment}`;
}

/** The text as a markdown quote, followed by where it came from. */
export function quote(text, title, url) {
    const lines = text.split(/\r?\n/).map((l) => `> ${l}`.trimEnd());
    return `${lines.join('\n')}\n>\n> — [${title || url}](${url})`;
}

/** The passage a `#:~:text=` address points at (its start, when it gives a start and an end), or ''. */
export function fragmentText(url) {
    const at = (url ?? '').indexOf('#:~:text=');
    if (at < 0) return '';
    for (const part of url.slice(at + 4).split('&')) { // "text=...&text=..." has one directive for each passage
        if (!part.startsWith('text=')) continue;
        const terms = part.slice(5).split(',').filter((t) => !t.endsWith('-') && !t.startsWith('-')); // prefix- and -suffix only narrow the match
        try { if (terms[0]) return decodeURIComponent(terms[0]); } catch { /* not valid: no passage */ }
    }
    return '';
}
