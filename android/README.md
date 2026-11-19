# Mardo for Android

A Kotlin shell around a `WebView`. The page is the same one the iPhone app shows (`npm run build:ios`), and the core is the same Go `mobile` package, bound with gomobile.

## Build

You need the Android SDK with an NDK and the command line tools, `gomobile` (`go install golang.org/x/mobile/cmd/gomobile@latest; gomobile init`), Node, and Android Studio (its bundled JDK is used).

```
make android-apk        # a debug build: adb install -r android/app/build/outputs/apk/debug/app-debug.apk
make android-bundle CODE=2   # the signed bundle for Google Play
```

The bundle is signed with `android/keystore.properties` (storeFile, storePassword, keyAlias, keyPassword), which is not in the repository, nor is the key. See `PLAYSTORE.md` for the listing and the release steps.

## How it works

- `MainActivity.kt` serves the page from the app's assets at `https://appassets.androidplatform.net`, and anything else (images, the Go core's assets) through `Engine.Asset`.
- The page calls the core through `window.MardoAndroid.call(id, method, args)`; the answer comes back through `window.mardoReply`. Events from the core arrive through `window.mardoEmit`.
- Files are `content://` URIs on Android while the core works with paths, so a file you open or save is copied into the app's `Documents` folder and written back to the original after each call (`Documents.kt`).
- Logins are kept encrypted with the Android Keystore (`Secrets.kt`).
- There is no on-device AI on Android: the AI features use your own key.
