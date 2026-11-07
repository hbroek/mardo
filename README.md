# Mardo

**Website and downloads: [mardo.henryvandenbroek.com](https://mardo.henryvandenbroek.com/)** · [iPhone and iPad: App Store](https://apps.apple.com/app/id6830193468)

A small desktop browser and author for markdown: read pages, and write them back where the server allows. It asks servers for markdown, renders it, and falls back to a
title plus link list when a page is not markdown. Go backend, HTML/CSS/JS frontend, built with
[Wails](https://wails.io) so it runs on macOS, Windows and Linux with the system webview.

## Features

- Address bar, back, forward, reload, and a history (saved between runs; **History** is in the ☰ menu)
- Requests `Accept: text/markdown`; renders markdown (GFM tables, task lists, footnotes) as sanitized HTML
- **Source** button shows the raw markdown. When the server allows saving (`Allow: PUT`, optionally `Link: <…>; rel="edit"` on the same server), an
  **Edit** button turns it into an editor; saving sends `PUT` with the saved credentials and `If-Match` from the page's `ETag`,
  and says so if the document changed in the meantime
- A native menu bar (File, Edit, View, Bookmarks, Tools; plus the app and Window menus on macOS) has the same actions as the toolbar and
  shortcuts; Edit > Select All selects the field, bookmarks, chat or page you are in. On Windows and Linux it lists the shortcuts and the
  page handles the keys; the ☰ menu stays everywhere
- **File > New Page** (Cmd/Ctrl+N) opens an empty markdown page in edit mode; **File > Open Page…** (Cmd/Ctrl+O) opens a file from disk. A new page is only in the window until you press Save, which asks where to put the file; Cancel, or the pencil, closes it without leaving anything on disk. The AI panel can draft its first text
- Not markdown? Shows the page title and the links found on it. Before that, a web page (HTML) is checked for a markdown version by the
  convention that `/about` has one at `/about.md` (`/docs/` at `/docs/index.md`, `/guide.html` at `/guide.md`; a site's root also tries `/llms.txt`); if the server has it, you get that, with a
  note and a link to the original page. Addresses with a query are not checked. Turn this off under ☰ or Tools > Look for Markdown Versions
- An HTML page with no markdown version has a **Convert to markdown** button: its headings, paragraphs, emphasis, links, images, lists, quotes, code and tables are mapped to markdown syntax, keeping the main content and leaving out navigation, scripts and forms. With an AI provider set up there is also **Convert with AI**, which has the model rewrite the page; it says so on the result, since a model can leave out or change things. The converted page can be saved or copied like any markdown page, and the original is one click away
- RSS and Atom feeds render as a list of posts; an item's `<source:markdown>` is used when present
- `http`, `https` and `file://`; drop a `.md` file on the window to open it. Relative links and images work,
  including local images for pages opened from disk
- **History** in the ☰ menu generates a markdown page of everything you visited
- **Save page** (☰ menu, Cmd/Ctrl+S) writes a markdown page as it is, a feed as a document of its entries, and any other page as its title, address and links. The **Copy markdown** button at a page's top right (Cmd/Ctrl+Shift+C) puts the markdown on the clipboard.
- Tabs, each with its own back/forward history; open tabs are restored on the next start (only the selected one is loaded right away). Cmd/Ctrl+click or middle-click a link to open it in a background tab; click a bookmark with Cmd/Ctrl, or right-click it, to open it in a new tab.
- A network console (hidden by default; `>_` button or Cmd/Ctrl+J) lists every request with its status code;
  click the bar to see request and response headers, timing and size. Cookie and token values are hidden
- Bookmarks with folders, descriptions and keywords: see below
- The ☰ menu at the far right holds Site credentials, the theme (follow the OS by default, or Light or Dark) and the color scheme: Violet (default), Ocean, Forest, Sunset, Rose or Graphite
- Sites that need a login: see below

| Shortcut | Action |
|---|---|
| Cmd/Ctrl+L | Focus the address bar |
| Cmd/Ctrl+R | Reload |
| Alt+← / Alt+→ (Cmd+[ / Cmd+] on Mac) | Back / forward |
| Cmd/Ctrl+U | Toggle markdown source |
| Cmd/Ctrl+N, Cmd/Ctrl+O | New page (an empty markdown page in edit mode), open a page from disk |
| Cmd/Ctrl+E | Edit this page, when it can be changed; again to stop |
| Cmd/Ctrl+S | Save the page as a markdown file, or save your edit while editing (also in the ☰ menu) |
| Cmd/Ctrl+Shift+C | Copy the page's markdown to the clipboard (Edit > Copy All; the button at the page's top right does the same) |
| Cmd/Ctrl+D | Add or edit the bookmark for this page |
| Cmd/Ctrl+B | Open the bookmarks panel and move into it (again to close it) |
| Cmd/Ctrl+J | Show / hide the network console |
| Cmd/Ctrl+Shift+A | Show / hide the Interact with this page panel |
| Cmd/Ctrl+T, Cmd/Ctrl+W | New tab, close tab |
| Ctrl+Tab / Ctrl+Shift+Tab, Cmd/Ctrl+Shift+[ / ], Cmd/Ctrl+1–9 | Switch tabs (9 is the last) |

## Bookmarks

- The ribbon next to the address bar saves the current page (filled when it is already saved). The dialog takes a
  title, address, description, keywords and a folder. The panel button at the far left (Cmd/Ctrl+B) shows folders
  and bookmarks as a tree; the pencil on a row edits or deletes it, and Bookmarks > New Folder… adds folders. Drag a bookmark or folder onto a folder to move it, or onto empty space for the top level (Esc cancels).
  In the panel, use ↑/↓ to move, → / ← to open and close folders, Enter to open a bookmark, **e** to edit and
  Esc to go back to the address bar.
- **Search** by typing in the address bar: several words (`rust async`), a single bare word (`rust`), or anything
  starting with `?` (`?go.dev`; a bare `?` lists everything). Every word must match the title, address,
  description or a keyword. Pages from your browsing history that match (title or address) are merged in
  after the bookmarks, without repeating a bookmarked page. Anything with a dot, colon or slash, like `example.com`, is an address instead.
- **Search by meaning** starts with `@` (`@that article about async rust I read last month`) and needs the AI assistant (see below).
- **Import / Export** (bottom of the panel) read and write browser-compatible HTML bookmark files (Chrome, Firefox,
  Safari and others) and markdown lists. Importing merges folders by name and skips a page that is already in the same folder (the same page in another folder is imported).
  Markdown exports look like this, and hand-written files in the same shape import too (headings also work as
  folders):

  ```markdown
  - **Dev**
    - [Go](<https://go.dev/>) — The Go site
      Keywords: lang, go
  ```
- **Where bookmarks are kept** (the ⚙ in the panel header): by default a local SQLite file, `bookmarks.db` in the
  app's config folder, or a folder of your [LinkaGoGo](https://www.linkagogo.com) account. For LinkaGoGo, create
  an API key under Account Settings → API Keys, paste it in the dialog, load your folders and
  pick the one to use as this app's top level, or let it create one. Only that folder and what is inside it is ever
  shown, searched or changed. The key goes to your system keychain (never to a plain file; with no keyring on Linux, to an encrypted file in the config folder). Switching does not move
  bookmarks: export, switch, then import. The choice is saved in `settings.json` next to the database
  (`{"bookmarks": {"backend": "linkagogo", "config": {"root": "<folder id>"}}}`), and further backends plug in
  through `internal/bookmarks`.

## AI assistant

The sparkle button (✨) in the toolbar opens the AI tools; **Summarize this page** shows a summary of the current page in a card above it, and **Suggest from the page**
in the bookmark dialog fills in an empty title, description and keywords for you to edit. Choose a provider under
☰ → **AI assistant**; the first click on either button opens that dialog if nothing is set up.

Any server that speaks the OpenAI chat-completions API works. Presets are filled in for Claude (Anthropic's
compatible endpoint), OpenAI, Google Gemini, Ollama, and Apple's on-device model through `fm serve`
(`http://127.0.0.1:1976/v1`; run `fm serve` first). Or enter your own address and model.

The speech-bubble button (💬, Cmd/Ctrl+Shift+A) opens **Interact with this page**: a conversation about the page in the current tab, in a
panel on the right. Each tab has its own conversation, about one page. It is saved after every answer and comes back when you open
that page again, also after a restart (**Clear** deletes it; ☰ → AI assistant → *Delete saved conversations* deletes them all). The text
of your questions and the answers is kept in `conversations.json` in the config folder, readable only by you, for up to 50 pages. Answers are rendered as markdown, with links that open in the browser, and show the model and how long it took.
On a page you can edit, the panel follows the page: while you only read it, your messages are questions; once you click the pencil to edit, they are
changes to make. The AI writes the whole revised document, which opens as an unsaved draft with **Show changes** (what was removed and added);
nothing is saved until you press Save, and Cancel discards it. Each instruction builds on the current draft. Documents over 60,000 characters are too long for this.
Enter sends and Shift+Enter starts a new line. While a model is working, the **Cancel** button (or Esc) stops it, and so do the Cancel buttons on the summary card and the bookmark suggestion; the question goes back into the box.

**Tag with AI** (a button in the bookmarks panel footer, or right-click a folder to start with just that folder) asks the AI for a short
title, a description and keywords for many bookmarks at once. You choose the folder and whether to cover only bookmarks that are
missing a title, description or keywords, or every bookmark (new descriptions and keywords replace the old ones). Existing titles are
kept unless you tick *Replace existing titles with short AI titles*. The dialog first says how many pages will be read and the most text that could be
sent. Pages are read two at a time, **Stop** ends the run early, and a rate-limit error from the provider stops it too. Then
a table shows every suggestion with an editable title, description and keywords: nothing is saved until you untick what you do not want and
press **Apply**. Pages that could not be read are listed at the end with the reason.

**Search by meaning** is typed in the address bar with a leading `@`: `@that article about async rust I read last month`. Plain
searches (`rust async`, `?rust`) still need every word to match and never use the AI. The `@` search offers the model your bookmarks and recent
browsing history (up to 400 entries, the most recent ones, and those sharing words with your query first when you have more) and shows the ones it picks, best first, each with the
model's reason. Unlike the other features it sends not the text of a page but the **title, address, folder, keywords and description of each
bookmark and history entry**, so it needs the same permission for servers outside your computer. The results page says what was sent and where. A
keyword results page has a button to rerun the search by meaning, and the other way round. Searches by meaning are not recorded in history.

**Reading tools** (in the same sparkle menu) work on the page in the active tab. **Outline** lists the page's sections with a line on each;
**Translate into** a language you pick keeps the headings, lists, links and code and translates the text (up to the first 30,000 characters);
on a feed, **Digest** groups the entries by topic with links. They appear in the card above the page, like the summary, with a cancel button. Select
some text first and **Explain the selection** or **Simplify the selection** asks the Interact panel about that passage, so you can follow up and the
conversation is saved with the page. The page text goes to your AI provider, as for the summary.

**Sort unfiled** (a button in the bookmarks panel footer) asks the AI where the bookmarks that sit in no folder belong. It prefers your
existing folders and proposes a new one only when nothing fits. It sends the title, address, keywords and description of each unfiled
bookmark and the names of your folders (50 bookmarks per request), not the pages. You review a table, change any folder (typing a new
path creates it), untick what you do not want, and only then are bookmarks moved.

**Tidy into subfolders** (right-click a folder, next to **Tag with AI**) does the same for the bookmarks directly in one overgrown folder: the AI
sorts them into subfolders of it, preferring the ones that exist, and proposes new ones where several bookmarks share a topic. It never moves a
bookmark outside that folder, and you review the table before anything moves.

**Compare open tabs** (in the sparkle menu) opens a dialog listing your open tabs. Tick two to six, optionally say what to compare, and the
AI writes what the pages have in common and where they differ, with a table when the pages describe comparable things. The tab bar itself has no
checkboxes; the choosing happens in the dialog. The pages share one budget of text, so long pages are cut short.

- The API key is kept in the system keychain, never in `settings.json`.
- A summary, suggestion or question sends the page's text (cut to about 30,000 characters) to the provider. For a server that is not on this
  computer you must tick *Allow sending page text to …* once; servers on this computer need no permission.
- Pages are re-fetched for this, with your saved credentials, so pages behind a login work. Model output is shown
  as sanitized markdown, and the prompts tell the model to ignore instructions inside the page.
- The model can be wrong; the summary card says so.

## Sites that need a login

Open **Site credentials** in the ☰ menu (it is highlighted when the current site has one saved), or use the button on a 401/403 page, and add a credential for a site:

- **Cookies**: paste the `Cookie` request header from your browser's dev tools, or import a `cookies.txt`
  export (any "cookies.txt" browser extension) for that site. Only the cookies for the site you enter are kept.
- **Bearer token** or **username & password** for sites with API tokens or basic auth.

How credentials are handled:

- They are stored in the OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service; where none is running, an encrypted `secrets.bin` in the config folder) and
  never shown again or sent back to the UI.
- They are sent only to the exact origin (scheme, host and port) they were saved for, and are dropped on
  redirects to another origin.
- They are refused for plain `http` except for `localhost` and loopback addresses. To allow a host you trust on your own network, list it in the `MARDO_ALLOW_HTTP` environment variable (comma separated, exact host names, e.g. `MARDO_ALLOW_HTTP=wako.example.com,192.168.1.5`).
- Cookies a site sets are kept between runs in `cookies.bin`, encrypted with a key held in the keychain.
  "Clear saved cookies" removes them.
- The app does **not** read cookies out of other browsers.

Files live in the user config directory (`~/Library/Application Support/mardo` on macOS,
`%AppData%\mardo` on Windows, `~/.config/mardo` on Linux): `history.json`, `history.md`,
`credentials.json` (site names only, no secrets) and `cookies.bin`.

## Build

Requirements: Go 1.25+, Node (for the frontend build), the [Wails CLI](https://wails.io/docs/gettingstarted/installation)
(`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

```sh
make dev       # live-reload development window
make test      # go vet + go test
make build     # app for the current OS, in build/bin
make windows   # cross-compile mardo.exe (works from macOS/Linux)
```

Linux needs `libgtk-3-dev` and `libwebkit2gtk-4.1-dev` (a Secret Service such as gnome-keyring is used for
credentials when there is one); build with `make linux`.

The Linux release is a folder with the program, `install.sh`, a launcher entry and the icon. Run `./install.sh` inside it to put them under
`~/.local` (program, application-menu entry and icon; delete those three files to uninstall). Without it the program runs but
Wayland desktops have no launcher to take the taskbar icon from; X11 desktops show the window icon either way. A GitHub Actions workflow builds all three platforms.

## iPhone and iPad

The iPhone app runs the same Go core inside the app and shows the same page in a web view. The page calls the core
through a message bridge (`frontend/src/ios/bridge.js`), the web view loads it from a `mardo://app` scheme served from the
app bundle, and Swift supplies the Keychain, file picker and image requests (`ios/Mardo/`). Needs Xcode and
[XcodeGen](https://github.com/yonaskolb/XcodeGen) (`brew install xcodegen`):

```sh
make ios-framework ios-web ios-app   # the core, the page, then the app for the simulator
open ios/Mardo.xcodeproj             # run it from Xcode
```

Not done yet: a layout for the phone screen (the page is still the desktop one), opening `.md` files and links from
other apps, and a Share extension. A new page is saved in a folder you choose in Files, such as iCloud Drive, so it survives deleting and reinstalling the app; the first save asks for the folder and later saves reuse it (Choose Folder… changes it). Open Page opens a file where it is, so saving writes back to it. Mardo keeps access to those places across restarts. Addresses in the app's own Documents folder show as `~/name.md`.

## Releases

Push a version tag and GitHub Actions builds macOS (arm64), Windows and Linux, then publishes them on the
repository's Releases page:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The macOS build is signed and notarized. The Windows and Linux builds are not signed, so Windows SmartScreen warns about an unknown publisher
(choose More info, then Run anyway). Signing costs money and this is a free project, so you are encouraged to build it yourself from source: see [Build](#build).

### Signing releases (macOS)

Locally, with a "Developer ID Application" certificate in your keychain:

```sh
xcrun notarytool store-credentials mardo --apple-id <id> --team-id <team>   # once
make notarize
```

For the release workflow, add these repository secrets (Settings > Secrets and variables > Actions):

| Secret | Value |
| --- | --- |
| `MACOS_CERT_P12` | the Developer ID Application certificate exported as `.p12`, base64-encoded (`base64 -i cert.p12 \| pbcopy`) |
| `MACOS_CERT_PASSWORD` | the password you set when exporting the `.p12` |
| `APPLE_ID` | your Apple ID email |
| `APPLE_APP_PASSWORD` | an app-specific password from appleid.apple.com |
| `APPLE_TEAM_ID` | your 10-character team ID |

## Roadmap

**Editing.** Remote pages can be edited when the server allows `PUT` (see **Source** above). Still to do: local files (the source view as
an editor with a live preview, saving writes the file), `PATCH`, and other write protocols (WebDAV, a Git host's API), one at a time.

**Social media.** Not defined yet. Ideas to choose from: share a page, a selection or an AI summary as a post (copy as formatted text, or
post to Mastodon or Bluesky with an app password kept in the keychain), and save links from a feed or a timeline as bookmarks.

**AI features**, all through the same provider (see **AI assistant** above), in this order:

1. **Native Claude backend.** The Messages API for Claude-only features such as prompt caching, behind the same provider interface.

## Layout

```
main.go, app.go, menu.go  the desktop shell: Wails bootstrap, native menus and file dialogs
ios/                   the iPhone app (SwiftUI + WKWebView around the core; see below)
frontend/src/ios       the page's bridge to the core on iPhone (`make ios-web` builds frontend/dist-ios)
mobile/                the core packed for an iPhone app (`make ios-framework`): one `Call(method, json)` entry point
core/                  the methods the UI calls (tabs, bookmarks, AI, saving); a Host interface stands in for the window
internal/fetch         HTTP/file loading, content negotiation, size and redirect limits
internal/chats         saved page conversations
internal/ai            OpenAI-compatible chat client; summary and bookmark-suggestion prompts
internal/render        markdown -> sanitized HTML, feeds, link extraction
internal/browser       ties fetch + render + history together
internal/nav           back/forward stack
internal/history       visit log and the generated history page
internal/bookmarks     bookmark model, Store interface, backends (sqlite/, linkagogo/, memory), conformance tests
internal/settings      settings.json (bookmark backend choice)
internal/auth          per-site credentials in the keychain
internal/cookies       encrypted persistent cookie jar
internal/localfile     serves local images to pages opened from disk
tools/lggcheck         read-only probe of the LinkaGoGo API with your key (prints shapes, not data)
frontend/              vanilla HTML/CSS/JS (no framework)
```

## License

[MIT](LICENSE). The open source libraries Mardo includes, and their licenses, are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
