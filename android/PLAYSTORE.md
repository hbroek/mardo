# Mardo on Google Play

Notes for the Play Console listing and each release. The app is `com.henryvandenbroek.mardo`.

## Build a release

```
make android-bundle CODE=2        # CODE is the versionCode: it must go up with every upload
```

The signed bundle is `android/app/build/outputs/bundle/release/app-release.aab`. The version name is
`0.19.1` unless you pass `-PversionName=...` to Gradle. It is signed with the upload key from
`android/keystore.properties` and `android/mardo-upload.jks`. Neither is in the repository: keep a copy of
both somewhere safe (a password manager). Google re-signs the app with its own key (Play App Signing),
so a lost upload key can be reset through Play support, but it is a hassle.

## Release notes

**What's new (0.19.1)** (500 max):
```
- Sign in with a site that offers OAuth, and "Sign in to edit" on a page you cannot change yet.
- Add and delete pages on a server that allows it, with a template to start from.
- A magnifier in the address bar shows when the site's own search is on.
- Back and Forward remember anchors; a long press lists the pages behind.
- Signing in works on the phone, and a site's own search works in either mode.
- Fixes and polish.
```

## Account steps (one time)

1. Register a Play developer account ($25, one time), as an individual or organization.
2. Create the app: name **Mardo**, default language English, App, Free.
3. A new personal account must run a **closed test with at least 12 testers for 14 days** before it can
   apply for production access. Upload the bundle to Testing > Closed testing, add testers by email or a Google
   group, and ask them to opt in and keep the app installed.

## Store listing

**App name** (30): Mardo: Markdown Browser

**Short description** (80): Read the web as markdown, write it back. Bookmarks, tabs, preview and AI.

**Full description** (4000):

Mardo is a small browser and author for markdown. It asks servers for markdown, shows it cleanly, and saves your changes back where the server allows.

- Asks for markdown. Sends Accept: text/markdown, and looks for a .md version of ordinary pages and a site's llms.txt. Tables, task lists and footnotes render properly.
- Convert any page. No markdown version? One tap maps an HTML page to markdown, or has your AI model rewrite it.
- Write it back. Where a server allows it, Mardo turns into an editor and saves with PUT, using your saved login and warning you if the page changed meanwhile.
- Bookmarks that you own. Folders, descriptions, keywords and search by typing in the address bar.
- Tabs and history. Tabs with their own history, restored on the next start. Back, Forward and Reload keep your place.
- Open and save .md files from your phone's storage.
- Preview as you write, with a cheatsheet that puts markdown syntax in the text with one tap.
- Find and replace, on pages and while you edit.
- Private by default. Logins are kept in the Android Keystore and sent only to the site they belong to. Nothing is sent to an AI service until you allow it.

AI that reads with you: bring your own key for Claude, OpenAI or Gemini, or point it at a compatible server. Summarize, outline, translate, ask about the page, compare tabs, edit with words.

Free and open source (MIT): https://github.com/hbroek/mardo

**Category:** Tools (or Productivity). **Tags:** browser, markdown, notes.
**Contact:** the email you want users to see, website https://mardo.henryvandenbroek.com
**Privacy policy:** https://mardo.henryvandenbroek.com/privacy.html

## Graphics

- App icon 512x512 PNG: made from `build/appicon.png`.
- Feature graphic 1024x500 (required).
- Phone screenshots (2 to 8, 16:9 or 9:16, 320 to 3840 px). Tablet screenshots are optional but recommended
  (7 and 10 inch).
- Take them on an emulator with `adb exec-out screencap -p`.

## Data safety

- Does the app collect or share user data? **Yes, only as below. Nothing is collected by the developer.**
- Data is processed on the device. The developer runs no server and receives nothing.
- The app fetches the pages the user asks for (their own request, to the sites they name).
- Optional AI features send the page text and chat to the AI provider the user picks, with the user's own key,
  after the user allows it. Declare this as **shared**: *App activity / Other user-generated content* and
  *Web browsing history* (the page address), purpose **App functionality**, **optional**, user chooses.
- Data encrypted in transit: **Yes** (https). Users can request deletion: bookmarks, history and logins are
  local and are deleted by clearing app data or uninstalling.
- No ads, no analytics, no tracking SDKs.

## Content rating and other forms

- Content rating questionnaire: a browser with user-generated and web content. Answer honestly for a
  web browser (users can reach any web content).
- Target audience: 18 and over (a general browser).
- Ads: none. News app: no. Government app: no. Health: no. Financial features: no.
- App access: no login needed.
