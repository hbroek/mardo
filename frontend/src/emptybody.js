// A markdown file that is only front matter (or nothing at all) renders as an empty page. Such a file, an action of the
// Send to toolbar for one, is more useful as its source, which can be changed.

const FRONT = /^﻿?---[ \t]*\r?\n[\s\S]*?\r?\n(?:---|\.\.\.)[ \t]*(?:\r?\n|$)/;

/** True when the markdown has nothing to show: no text after its front matter, if it has any. */
export function hasNoBody(source) {
    if (typeof source !== 'string') return false;
    const m = FRONT.exec(source);
    return (m ? source.slice(m[0].length) : source).trim() === '';
}
