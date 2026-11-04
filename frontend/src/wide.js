// A button at the top left of the page to read at the full width of the window instead of in the narrow
// column. It only shows when the window is wider than the column, and the choice is remembered.
const root = document.documentElement;
const content = document.getElementById('content');
const COLUMN = 52 * 16 + 48; // the column (52rem) and the page's padding

const read = () => { try { return localStorage.getItem('wide') === '1'; } catch { return false; } };
const write = (on) => { try { localStorage.setItem('wide', on ? '1' : ''); } catch { /* not remembered */ } };

const button = Object.assign(document.createElement('button'), {type: 'button', id: 'wide-toggle'});
button.innerHTML = '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M3 12h18M7 8l-4 4 4 4M17 8l4 4-4 4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
document.body.append(button);

function show() {
    const on = root.classList.contains('wide');
    button.title = on ? 'Use a narrow column' : 'Use the full width';
    button.setAttribute('aria-label', button.title);
    button.setAttribute('aria-pressed', String(on));
}

/** Sits in the corner of the page, wherever the page is (the side panels move it). */
function place() {
    const r = content.getBoundingClientRect();
    button.hidden = root.classList.contains('phone') || r.width < COLUMN + 80;
    button.style.left = `${r.left + 8}px`;
    button.style.top = `${r.top + 8}px`;
}

button.addEventListener('click', () => {
    root.classList.toggle('wide');
    write(root.classList.contains('wide'));
    show();
});

root.classList.toggle('wide', read());
show();
new ResizeObserver(place).observe(content);
place();
