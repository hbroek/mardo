// The phone layout: in the iPhone app, in a narrow window, the page is a reader with its tools in a bar
// at the bottom, in reach of a thumb, and the panels (bookmarks, ask, tabs, menus) open as sheets over the
// page. The look is all in style.css under `html.phone`; this keeps the class up to date and lets one
// sheet at a time be open.
const $ = (id) => document.getElementById(id);
const root = document.documentElement;
const narrow = matchMedia('(max-width: 640px)');

const inApp = () => !!globalThis.mardoDocuments; // set by the iPhone app's shell
const update = () => root.classList.toggle('phone', inApp() && narrow.matches);
update();
if (root.classList.contains('phone')) $('address').placeholder = 'Search or enter an address';
narrow.addEventListener('change', update);

/** The sheets: how to tell one is open, and how to close it. */
const sheets = [
    {open: () => !$('bookmarks-panel').hidden, close: () => $('bookmarks-close').click()},
    {open: () => !$('ask-panel').hidden, close: () => $('ask-close').click()},
    {open: () => document.body.classList.contains('tabs-open'), close: () => tabs(false)},
    {open: () => !$('settings-menu').hidden, close: () => $('menu').click()},
    {open: () => !$('reading-menu').hidden, close: () => $('reading').click()},
];

function tabs(on) {
    document.body.classList.toggle('tabs-open', on);
    $('tabs-toggle').setAttribute('aria-pressed', String(on));
}

/** Closes every sheet but the one `except` names. */
function only(except) {
    if (!root.classList.contains('phone')) return;
    sheets.forEach((s, i) => { if (i !== except && s.open()) s.close(); });
}

$('tabs-toggle').addEventListener('click', () => {
    const on = !document.body.classList.contains('tabs-open');
    if (on) only(2);
    tabs(on);
});

// Choosing a tab, or opening a new one, closes the list.
$('tabs').addEventListener('click', (e) => {
    if (e.target.closest('.tab') && !e.target.closest('.tab-close') || e.target.closest('#tab-new')) tabs(false);
});

// Opening one sheet closes the others.
sheets.forEach((s, i) => {
    const el = [$('bookmarks-panel'), $('ask-panel'), null, $('settings-menu'), $('reading-menu')][i];
    if (el) new MutationObserver(() => { if (s.open()) only(i); }).observe(el, {attributes: true, attributeFilter: ['hidden']});
});

// Touching the page puts the sheets away.
$('content').addEventListener('pointerdown', () => only(-1));
$('address').addEventListener('focus', () => only(-1));

// The tabs button shows how many tabs there are.
const count = () => { $('tabs-count').textContent = String($('tabs').querySelectorAll('.tab').length); };
new MutationObserver(count).observe($('tabs'), {childList: true});
count();
