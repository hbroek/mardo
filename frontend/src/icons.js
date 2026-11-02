// Small line icons. Emoji need a color emoji font, which many Linux systems lack (they show as empty boxes),
// so every icon is drawn here instead. They take the text color.

const PATHS = {
    folder: ['M3 6.5A1.5 1.5 0 0 1 4.5 5H9l2 2.5h8.5A1.5 1.5 0 0 1 21 9v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18z'],
    key: ['M12.5 11.5L20 4', 'M16 8l2.5 2.5', 'M14 10l2 2'],
    sparkle: ['M10 3l1.8 5.2L17 10l-5.2 1.8L10 17l-1.8-5.2L3 10l5.2-1.8z', 'M18 14l.8 2.2L21 17l-2.2.8L18 20l-.8-2.2L15 17l2.2-.8z'],
    page: ['M7 3h7l4 4v14H7z', 'M14 3v4h4'],
    save: ['M5 4h11l3 3v13H5z', 'M8 4v5h7V4', 'M8 20v-6h8v6'],
    clock: ['M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16z', 'M12 8v4.5l3 2'],
    trash: ['M5 7h14', 'M9 7V4h6v3', 'M7 7l1 13h8l1-13'],
};

/** An inline SVG icon by name, sized to the text around it. */
export function svgIcon(name) {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 24 24');
    svg.setAttribute('aria-hidden', 'true');
    svg.setAttribute('class', 'inline-icon');
    for (const d of PATHS[name] ?? []) {
        const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        path.setAttribute('d', d);
        svg.append(path);
    }
    if (name === 'key') { // the head of the key
        const c = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
        c.setAttribute('cx', '8');
        c.setAttribute('cy', '15');
        c.setAttribute('r', '4');
        svg.append(c);
    }
    return svg;
}
