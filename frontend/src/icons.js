// Small line icons. Emoji need a color emoji font, which many Linux systems lack (they show as empty boxes),
// so every icon is drawn here instead. They take the text color.

const PATHS = {
    folder: ['M3 6.5A1.5 1.5 0 0 1 4.5 5H9l2 2.5h8.5A1.5 1.5 0 0 1 21 9v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18z'],
    key: ['M12.5 11.5L20 4', 'M16 8l2.5 2.5', 'M14 10l2 2'],
    sparkle: ['M10 3l1.8 5.2L17 10l-5.2 1.8L10 17l-1.8-5.2L3 10l5.2-1.8z', 'M18 14l.8 2.2L21 17l-2.2.8L18 20l-.8-2.2L15 17l2.2-.8z'],
    page: ['M7 3h7l4 4v14H7z', 'M14 3v4h4'],
    save: ['M5 4h11l3 3v13H5z', 'M8 4v5h7V4', 'M8 20v-6h8v6'],
    info: ['M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16z', 'M12 11v5', 'M12 8h.01'],
    clock: ['M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16z', 'M12 8v4.5l3 2'],
    trash: ['M5 7h14', 'M9 7V4h6v3', 'M7 7l1 13h8l1-13'],
    gear: ['M10 3h4l.6 2.4 1.7.7 2.1-1.3 2.8 2.8-1.3 2.1.7 1.7 2.4.6v4l-2.4.6-.7 1.7 1.3 2.1-2.8 2.8-2.1-1.3-1.7.7L14 21h-4l-.6-2.4-1.7-.7-2.1 1.3-2.8-2.8 1.3-2.1-.7-1.7L2.6 14v-4l2.4-.6.7-1.7-1.3-2.1 2.8-2.8 2.1 1.3 1.7-.7z'],
    sun: ['M12 2.5v2.5M12 19v2.5M2.5 12H5M19 12h2.5M5.3 5.3l1.8 1.8M16.9 16.9l1.8 1.8M18.7 5.3l-1.8 1.8M7.1 16.9l-1.8 1.8'],
    moon: ['M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z'],
    contrast: ['M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16z', 'M12 4a8 8 0 0 1 0 16z'],
    forward: ['M5 12h14', 'M13 6l6 6-6 6'],
    reload: ['M20 12a8 8 0 1 1-2.3-5.7', 'M20 4v4h-4'],
    share: ['M12 15V3', 'M8 7l4-4 4 4', 'M5 12v8h14v-8'],
    pencil: ['M4 20l1-4L16.5 4.5a2 2 0 0 1 3 3L8 19z', 'M14.5 6.5l3 3'],
    code: ['M9 7l-5 5 5 5', 'M15 7l5 5-5 5'],
};

const CIRCLES = {key: [8, 15, 4], gear: [12, 12, 3], sun: [12, 12, 4]};

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
    const circle = CIRCLES[name]; // the round part of some icons: the head of the key, the middle of the gear
    if (circle) {
        const c = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
        c.setAttribute('cx', String(circle[0]));
        c.setAttribute('cy', String(circle[1]));
        c.setAttribute('r', String(circle[2]));
        svg.append(c);
    }
    return svg;
}
