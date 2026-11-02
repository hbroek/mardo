// A line diff for showing what an edit changes. No state lives here.

const MAX_CELLS = 4_000_000; // beyond this the table would be too big; show it as one removal and one addition

/** Lines of `a` and `b` as [{type: 'same' | 'del' | 'add', text}], in order. */
export function diffLines(a, b) {
    const x = a === '' ? [] : a.split('\n');
    const y = b === '' ? [] : b.split('\n');
    let start = 0;
    while (start < x.length && start < y.length && x[start] === y[start]) start++;
    let endX = x.length, endY = y.length;
    while (endX > start && endY > start && x[endX - 1] === y[endY - 1]) { endX--; endY--; }
    const head = x.slice(0, start).map((text) => ({type: 'same', text}));
    const tail = x.slice(endX).map((text) => ({type: 'same', text}));
    const mid = diffMiddle(x.slice(start, endX), y.slice(start, endY));
    return [...head, ...mid, ...tail];
}

function diffMiddle(x, y) {
    const n = x.length, m = y.length;
    if (!n || !m || n * m > MAX_CELLS) {
        return [...x.map((text) => ({type: 'del', text})), ...y.map((text) => ({type: 'add', text}))];
    }
    // lcs[i][j]: the longest common run of x[i:] and y[j:]
    const w = m + 1;
    const lcs = new Uint32Array((n + 1) * w);
    for (let i = n - 1; i >= 0; i--) {
        for (let j = m - 1; j >= 0; j--) {
            lcs[i * w + j] = x[i] === y[j] ? lcs[(i + 1) * w + j + 1] + 1 : Math.max(lcs[(i + 1) * w + j], lcs[i * w + j + 1]);
        }
    }
    const out = [];
    let i = 0, j = 0;
    while (i < n && j < m) {
        if (x[i] === y[j]) { out.push({type: 'same', text: x[i]}); i++; j++; }
        else if (lcs[(i + 1) * w + j] >= lcs[i * w + j + 1]) out.push({type: 'del', text: x[i++]});
        else out.push({type: 'add', text: y[j++]});
    }
    while (i < n) out.push({type: 'del', text: x[i++]});
    while (j < m) out.push({type: 'add', text: y[j++]});
    return out;
}

/** The diff cut down to the changes with `context` unchanged lines around them; gaps become {type: 'gap'}. */
export function withContext(lines, context = 3) {
    const keep = new Array(lines.length).fill(false);
    lines.forEach((l, i) => {
        if (l.type === 'same') return;
        for (let k = Math.max(0, i - context); k <= Math.min(lines.length - 1, i + context); k++) keep[k] = true;
    });
    const out = [];
    lines.forEach((l, i) => {
        if (keep[i]) out.push(l);
        else if (out.length && out[out.length - 1].type !== 'gap') out.push({type: 'gap', text: ''});
    });
    return out;
}
