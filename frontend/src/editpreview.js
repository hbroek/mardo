// An optional preview beside the text box while editing a page. It follows what is typed, is split from the
// editor side by side or stacked, and keeps its place when the editor scrolls. The choices are remembered.
const pref = (key, fallback) => { try { return localStorage.getItem(`edit.${key}`) ?? fallback; } catch { return fallback; } };
const setPref = (key, value) => { try { localStorage.setItem(`edit.${key}`, value); } catch { /* not remembered */ } };

let cached = {text: null, html: ''}; // the last preview, so the editor being rebuilt does not blink

function el(tag, props = {}, ...children) {
    const node = Object.assign(document.createElement(tag), props);
    node.append(...children);
    return node;
}

/**
 * `area` is the text box; `render(text)` returns the sanitized HTML of a markdown text.
 * Returns {buttons, body, update}: the buttons for the edit bar, the element holding the editor and
 * the preview, and `update()` to call after the text changed.
 */
export function editSplit(area, render) {
    let on = pref('preview', '0') === '1';
    let layout = pref('layout', document.documentElement.classList.contains('phone') ? 'stack' : 'side') === 'stack' ? 'stack' : 'side'; // a phone is too narrow for two columns
    let share = Math.min(80, Math.max(20, Number(pref('split', '50')) || 50)); // percent of the room for the editor

    const preview = el('article', {className: 'markdown edit-preview', ariaLabel: 'Preview', tabIndex: -1});
    const divider = el('div', {className: 'edit-divider', role: 'separator', title: 'Drag to resize (double-click to reset)'});
    const body = el('div', {className: 'edit-split'}, area, divider, preview);

    const toggle = el('button', {type: 'button', textContent: 'Preview', title: 'Show how the page will look beside the editor'});
    const flip = el('button', {type: 'button', title: 'Put the preview below the editor or beside it'});

    function apply() {
        body.dataset.layout = layout;
        body.classList.toggle('previewing', on);
        body.style.setProperty('--editor-share', `${share}%`);
        toggle.setAttribute('aria-pressed', String(on));
        toggle.classList.toggle('on', on);
        flip.hidden = !on;
        flip.textContent = layout === 'side' ? 'Stacked' : 'Side by side';
        flip.setAttribute('aria-label', layout === 'side' ? 'Put the preview below the editor' : 'Put the preview beside the editor');
    }

    let timer = 0;
    let latest = 0;
    async function draw() {
        const mine = ++latest;
        const text = area.value;
        try {
            const html = await render(text);
            if (mine !== latest || !on) return;
            cached = {text, html};
            preview.classList.remove('preview-error');
            preview.innerHTML = html;
            follow();
        } catch (e) {
            if (mine !== latest) return;
            preview.classList.add('preview-error');
            preview.textContent = String(e).replace(/^Error: /, '');
        }
    }

    /** Called after each change of the text: the preview catches up once typing pauses. */
    function update(now = false) {
        if (!on) return;
        clearTimeout(timer);
        timer = setTimeout(draw, now ? 0 : 150);
    }

    /** The preview is scrolled to the same relative place as the editor. */
    function follow() {
        if (!on) return;
        const room = area.scrollHeight - area.clientHeight;
        const ratio = room > 0 ? area.scrollTop / room : 0;
        preview.scrollTop = ratio * (preview.scrollHeight - preview.clientHeight);
    }
    area.addEventListener('scroll', follow);

    toggle.addEventListener('click', () => {
        on = !on;
        setPref('preview', on ? '1' : '0');
        apply();
        if (on) update(true);
        area.focus();
    });
    flip.addEventListener('click', () => {
        layout = layout === 'side' ? 'stack' : 'side';
        setPref('layout', layout);
        apply();
        follow();
    });

    divider.addEventListener('dblclick', () => { share = 50; setPref('split', '50'); apply(); });
    divider.addEventListener('pointerdown', (e) => {
        e.preventDefault();
        divider.setPointerCapture(e.pointerId);
        body.classList.add('dragging');
        const move = (ev) => {
            const r = body.getBoundingClientRect();
            const fraction = layout === 'side' ? (ev.clientX - r.left) / r.width : (ev.clientY - r.top) / r.height;
            share = Math.min(80, Math.max(20, Math.round(fraction * 100)));
            apply();
        };
        const stop = () => {
            divider.removeEventListener('pointermove', move);
            divider.removeEventListener('pointerup', stop);
            divider.removeEventListener('pointercancel', stop);
            body.classList.remove('dragging');
            setPref('split', String(share));
            follow();
        };
        divider.addEventListener('pointermove', move);
        divider.addEventListener('pointerup', stop);
        divider.addEventListener('pointercancel', stop);
    });

    // Links in the preview are for looking at: they must not take the reader away from the editor.
    preview.addEventListener('click', (e) => {
        const a = e.target.closest?.('a');
        if (!a) return;
        e.preventDefault();
        e.stopPropagation();
        const id = a.getAttribute('href')?.startsWith('#') ? a.getAttribute('href').slice(1) : '';
        if (id) preview.querySelector(`[id="${CSS.escape(id)}"]`)?.scrollIntoView({block: 'start'});
    });

    apply();
    if (on) {
        if (cached.text === area.value) preview.innerHTML = cached.html;
        update(true);
    }
    return {buttons: [toggle, flip], body, update};
}
