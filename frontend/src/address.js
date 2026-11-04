// On iOS the pages the reader saves or opens live in the app's Documents folder (Files > On My iPad > Mardo).
// Its real path is long and changes with each install, so the page shows it as ~/name.md: in the address bar,
// on tabs and in the bookmark dialog. Swift says where the folder is (window.mardoDocuments); elsewhere
// there is no such folder and addresses stay as they are.

const docs = () => globalThis.mardoDocuments ?? '';

/** True in the iOS app, which has that folder. */
export const hasDocuments = () => docs() !== '';

/** The address as shown: a file under the Documents folder becomes ~/name. */
export function friendly(url) {
    const prefix = `file://${docs()}/`;
    if (!docs() || !url?.startsWith(prefix)) return url;
    try { return '~/' + decodeURIComponent(url.slice(prefix.length)); } catch { return url; }
}

/** What the reader typed (or a bookmark kept), as an address the core opens: ~/name becomes the file's URL. */
export function real(text) {
    if (!docs() || !text?.startsWith('~/')) return text;
    return `file://${docs()}/` + encodeURI(text.slice(2));
}
