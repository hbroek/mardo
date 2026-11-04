// While the pointer is over a link in the page, its address shows in the bottom left corner, as in other browsers.
import {friendly} from './address.js';

const bar = Object.assign(document.createElement('div'), {id: 'link-status', hidden: true});
bar.setAttribute('aria-hidden', 'true'); // the link itself is what a screen reader reads
document.body.append(bar);

const link = (e) => e.target.closest?.('#content a[href], #ask-log a[href]');

document.addEventListener('mouseover', (e) => {
    const a = link(e);
    if (!a || a.getAttribute('href').startsWith('#')) return; // a fragment stays on the page
    bar.textContent = friendly(a.href);
    bar.hidden = false;
});

document.addEventListener('mouseout', (e) => {
    if (link(e)) bar.hidden = true;
});

// A page that changes under the pointer (a new tab, a reload) must not leave the address behind.
document.addEventListener('scroll', () => { bar.hidden = true; }, true);
