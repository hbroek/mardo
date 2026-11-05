# Mardo on the App Store

Text and settings to paste into App Store Connect, and the steps to upload a build.

## One-time setup (developer.apple.com / appstoreconnect.apple.com)

1. Certificates, Identifiers & Profiles > Identifiers: register the App ID `com.henryvandenbroek.mardo`
   (explicit). Xcode automatic signing can create it too.
2. App Store Connect > Apps > + > New App: platform iOS, name Mardo, primary language English,
   bundle ID `com.henryvandenbroek.mardo`, SKU `mardo-ios`.
3. Agreements, Tax, and Banking: accept the Free Apps agreement.
4. `make ios-archive TEAM=8YUXRC9P9J BUILD=1` archives and uploads. Raise BUILD for each upload of one version.
   The version number is `MARKETING_VERSION` in `ios/project.yml`.
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
- **Keywords (100):** markdown,browser,reader,notes,editor,bookmarks,AI,summarize,writer,web,text

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
- **Notes for the reviewer:**
  Mardo is a browser for markdown. Type any address, for example https://mardo.henryvandenbroek.com/ or https://github.com/hbroek/mardo/blob/main/README.md.
  The AI tools are optional: they use Apple Intelligence where available, or an API key the user adds under Menu > AI assistant. Without a provider, everything else works.
  The app is free, has no in-app purchases and no accounts.
- **Contact:** your name, phone and email (required by App Store Connect)

## App Privacy questionnaire

- Data collection: **Data Not Collected.** The developer receives nothing; user-chosen requests go to the sites and AI providers the user picks.
- Tracking: No.
- Matches `ios/Mardo/PrivacyInfo.xcprivacy`.

## Other questionnaires

- **Export compliance:** Uses only standard HTTPS provided by the operating system. `ITSAppUsesNonExemptEncryption` is false in the app, so it is not asked on each upload.
- **Age rating:** Answer "Infrequent/Mild" for "Unrestricted Web Access" (the app is a web browser); expect 17+ or 12+ depending on the answer. Everything else: None.
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
