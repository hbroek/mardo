# Mardo on the App Store

Text and settings to paste into App Store Connect, and the steps to upload a build.

## One-time setup (developer.apple.com / appstoreconnect.apple.com)

1. Certificates, Identifiers & Profiles > Identifiers: register the App ID `com.henryvandenbroek.mardo`
   (explicit). Xcode automatic signing can create it too.
2. App Store Connect > Apps > + > New App: platform iOS, name Mardo, primary language English,
   bundle ID `com.henryvandenbroek.mardo`, SKU `mardo-ios`.
3. Agreements, Tax, and Banking: accept the Free Apps agreement.
4. `make ios-archive TEAM=8YUXRC9P9J BUILD=1` archives and opens the archive in Xcode's Organizer. There click
   Distribute App > App Store Connect > Upload. (Uploading from the command line hung every time.) Raise BUILD for
   each upload of one version. The version number is `MARKETING_VERSION` in `ios/project.yml`.
   If the upload sits at "Preparing to upload" for more than a minute or two, restart the Mac and try again.
5. In App Store Connect pick the processed build under the version, fill in the page, Submit for Review.

## Listing

- **Name:** Mardo
- **Subtitle (30):** Read the web as markdown
- **Category:** Productivity (secondary: Utilities)
- **Price:** Free
- **Support URL:** https://mardo.henryvandenbroek.com/
- **Marketing URL:** https://mardo.henryvandenbroek.com/
- **Privacy Policy URL:** https://mardo.henryvandenbroek.com/privacy.html
- **Copyright:** 2026 Henry van den Broek
- **Keywords (100):** reader,notes,editor,bookmarks,writer,web,text,md,translate,outline,wiki,textcasting,bigundo

**Promotional text (170):**
Read pages as clean markdown, keep bookmarks in folders, and use AI tools to summarize, outline or translate what you read. Private by design.

**Description:**
Mardo is a reader for the web that asks servers for markdown, so pages show up clean, fast and free of clutter.

READ
- Pages are requested as markdown and rendered cleanly. Pages that only come as HTML are converted with one tap, by fixed rules or with an AI model.
- Tabs, history, light and dark themes.

BOOKMARKS
- Folders, tags and search, one tap away. Optional sync with LinkaGoGo.

AI READING TOOLS
- Summarize, outline, translate or explain a page, or ask it a question.
- Uses Apple Intelligence on supported devices, so nothing leaves your device. Or bring your own key for Anthropic, OpenAI or any compatible server.

WRITE
- Write a page of your own, open and save markdown files in Files, and save changes back to servers that allow it.

PRIVATE
- No account, no ads, no tracking. Your bookmarks and settings stay on your device. Mardo is free and open source.

**What's new (0.11.0):** First release on iPhone and iPad.

## App Review information

- **Sign-in required:** No. No demo account needed.
- **Notes for the reviewer** (paste into the Notes field; under 4000 characters). Attach the screen recording as mardo-review.mp4 under App Review Information > Attachment:

```
SCREEN RECORDING (physical iPhone, latest iOS): attached to this submission as mardo-review.mp4.
It starts with launching the app and shows: opening a page, reading it as markdown, tabs, converting an HTML page, bookmarks and folders, the AI tools, writing and saving a page, and the licenses in About.

PURPOSE AND AUDIENCE
Mardo is a web browser for markdown. It asks websites for markdown, and shows a page as clean, fast, readable text without ads or clutter. Pages that only exist as HTML are converted to markdown with one tap. It also lets people write markdown pages and keep bookmarks in folders. It is for readers, writers, students and developers who read documentation, blogs and notes, and for people who like plain text. It is a general web browser: pages are shown with WebKit (WKWebView) and fetched over HTTPS.

HOW TO USE IT (no account, no login, no sample files needed)
1. Open the app and type an address in the address bar, for example:
   https://mardo.henryvandenbroek.com/  (a page with a markdown version)
   https://github.com/hbroek/mardo/blob/main/README.md
   https://blog.henryvandenbroek.com  (shows the blog's RSS feed)
2. Convert: on a page with no markdown version, tap "Convert to markdown".
3. Bookmarks: tap the bookmark button to save a page, and the folder button to add a folder.
4. Write: Menu (top right) > New page, type markdown, and save it to Files.
5. AI tools (optional): tap the sparkle button to summarize, outline, translate or ask a question. They use Apple Intelligence on supported devices. Otherwise the user adds their own API key under Menu > AI assistant. Without one, all other features work.
6. About: Menu > About Mardo shows the version and the open source licenses.

EXTERNAL SERVICES
- The websites the user chooses to open (Mardo is a browser).
- AI providers, only if the user turns them on and adds their own key: Anthropic, OpenAI, or any OpenAI-compatible server the user names. Apple Intelligence (on device) needs no key.
- LinkaGoGo (linkagogo.com), an optional bookmark storage service the user can choose. Bookmarks are stored only on the device by default.
- No analytics, advertising, authentication or payment services. The developer receives no data.

ACCOUNTS, PURCHASES AND USER CONTENT
Mardo has no accounts to create or delete, no in-app purchases and no subscriptions. It does not host or share user-generated content: pages the user writes are files on their own device, and bookmarks are private. So there is no reporting or blocking mechanism to show.

REGIONS
The app works the same in every region. Only Apple Intelligence depends on the device, language and region Apple supports; elsewhere the user can use their own AI key, or skip the AI tools.

REGULATED INDUSTRY / THIRD-PARTY MATERIAL
None. The app shows web pages the user chooses to visit. It includes open source libraries under MIT and BSD licenses, listed in the app under Menu > About Mardo > Open source licenses.
```

- **Contact:** your name, phone and email (required by App Store Connect)

## App Privacy questionnaire

- Data collection: **Data Not Collected.** The developer receives nothing; user-chosen requests go to the sites and AI providers the user picks.
- Tracking: No.
- Matches `ios/Mardo/PrivacyInfo.xcprivacy`.

## Other questionnaires

- **Export compliance:** Uses only standard HTTPS provided by the operating system. `ITSAppUsesNonExemptEncryption` is false in the app, so it is not asked on each upload.
- **Age rating:** Answer "Yes" to "Unrestricted Web Access" (the app is a web browser); that makes it 18+ on Apple's newer scale (the choices are 4+, 9+, 13+, 16+, 18+). Everything else: None.
- **Content rights:** the app shows third-party web content the user navigates to.
- **Government/Regulated:** none. **Encryption:** exempt as above.

## Screenshots (required)

- iPhone 6.9": 1320x2868 (iPhone 17 Pro Max / 16 Pro Max simulator).
- iPad 13": 2064x2752 (iPad Pro 13-inch M4/M5 simulator), since the app runs on iPad.
- 3 to 10 each. Suggested: reading a page, bookmarks sheet, AI tools sheet, Ask sheet, converting an HTML page, dark theme.
  Use public pages only (not your own bookmarks).

## Review risk notes

- Guideline 4.2 (minimum functionality): the app is a browser plus reader plus editor with AI tools; the reviewer notes explain this.
- Guideline 2.5.6 (web browsers must use WebKit): it does, via WKWebView for display.
- No subscription or payment text remains in the app.
