// A button next to the full-width one that sets where the images of a page sit: as the page has them (the
// default), floated left or right with the text flowing around them, or centered on their own line.
// Each click goes to the next choice; the choice is remembered.
const root = document.documentElement;
const content = document.getElementById('content');
const wide = document.getElementById('wide-toggle');

const MODES = [
    {id: '', title: 'Images: as the page has them', icon: '<rect x="4" y="5" width="16" height="6" rx="1"/><path d="M4 15h16M4 19h16"/>'},
    {id: 'left', title: 'Images: left, text flows around', icon: '<rect x="4" y="5" width="8" height="7" rx="1"/><path d="M15 6h5M15 10h5M4 16h16M4 20h16"/>'},
    {id: 'right', title: 'Images: right, text flows around', icon: '<rect x="12" y="5" width="8" height="7" rx="1"/><path d="M4 6h5M4 10h5M4 16h16M4 20h16"/>'},
    {id: 'center', title: 'Images: centered', icon: '<path d="M4 5h16"/><rect x="8" y="9" width="8" height="6" rx="1"/><path d="M4 19h16"/>'},
];

const read = () => { try { return localStorage.getItem('image-align') || ''; } catch { return ''; } };
const write = (id) => { try { localStorage.setItem('image-align', id); } catch { /* not remembered */ } };

const button = Object.assign(document.createElement('button'), {type: 'button', id: 'image-align'});
document.body.append(button);

let index = Math.max(0, MODES.findIndex((m) => m.id === read()));

function apply() {
    const mode = MODES[index];
    for (const m of MODES) if (m.id) root.classList.toggle(`img-${m.id}`, m.id === mode.id);
    button.innerHTML = `<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${mode.icon}</svg>`;
    button.title = mode.title;
    button.setAttribute('aria-label', mode.title);
    button.setAttribute('aria-pressed', String(mode.id !== ''));
}

/** Next to the full-width button (or in the corner when that one is hidden). */
function place() {
    const r = content.getBoundingClientRect();
    button.hidden = root.classList.contains('phone');
    button.style.left = `${r.left + (wide && !wide.hidden ? 44 : 8)}px`;
    button.style.top = `${r.top + 8}px`;
}

button.addEventListener('click', () => {
    index = (index + 1) % MODES.length;
    write(MODES[index].id);
    apply();
});

apply();
new ResizeObserver(place).observe(content);
if (wide) new MutationObserver(place).observe(wide, {attributes: true, attributeFilter: ['hidden']});
place();
