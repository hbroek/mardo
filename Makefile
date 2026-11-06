WAILS ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)

# The day of the build, shown on the About page.
BUILT := $(shell date -u +%Y-%m-%d)

# Keep the macOS deployment target in line with Info.plist (silences linker warnings).
export CGO_CFLAGS  += -mmacosx-version-min=13.0
export CGO_LDFLAGS += -mmacosx-version-min=13.0

.PHONY: site dev build test windows linux sign notarize ios-framework ios-web ios-app ios-device ios-archive

# The website page written in markdown (docs/find-markdown.md) becomes its HTML twin. Run it after editing the .md.
site:
	pandoc -f gfm -t html5 --no-highlight --template docs/_page.html -V name=find-markdown \
		-V pagetitle="Where can I find markdown on the web?" \
		-V description="Feeds, sites and directories that serve Markdown directly, and how to search for more." \
		-o docs/find-markdown.html docs/find-markdown.md

dev:
	$(WAILS) dev

build:
	$(WAILS) build -ldflags "-X mardo/internal/about.Date=$(BUILT)"

test:
	go vet ./...
	go test ./...

# Cross-compiling works from any OS for Windows; Linux needs a Linux host
# with libgtk-3-dev and libwebkit2gtk-4.1-dev (see README).
windows:
	$(WAILS) build -platform windows/amd64

linux:
	$(WAILS) build -tags webkit2_41

# macOS release signing. IDENTITY is the "Developer ID Application" certificate;
# NOTARY_PROFILE is a keychain profile created once with:
#   xcrun notarytool store-credentials mardo --apple-id <id> --team-id <team>
IDENTITY ?= Developer ID Application
NOTARY_PROFILE ?= mardo
APP := build/bin/mardo.app

# Sign with the hardened runtime (required for notarization).
sign: build
	codesign --force --deep --options runtime --timestamp --sign "$(IDENTITY)" $(APP)
	codesign --verify --deep --strict --verbose=2 $(APP)

# Submit to Apple, wait, then staple the ticket so it works offline.
notarize: sign
	ditto -c -k --keepParent $(APP) build/bin/notarize.zip
	xcrun notarytool submit build/bin/notarize.zip --keychain-profile "$(NOTARY_PROFILE)" --wait
	xcrun stapler staple $(APP)
	spctl --assess --type execute --verbose $(APP)
	rm build/bin/notarize.zip

# The Go core as a framework for an iPhone app (the Swift shell is not in this repository yet).
# Needs Xcode and both tools: go install golang.org/x/mobile/cmd/gomobile@latest golang.org/x/mobile/cmd/gobind@latest
ios-framework:
	gomobile bind -ldflags "-X mardo/internal/about.Date=$(BUILT)" -target ios,iossimulator -o build/Mardo.xcframework ./mobile

# The page for the iPhone app, calling the Go core through the app's bridge (frontend/src/ios/bridge.js).
ios-web:
	cd frontend && npm run build:ios

# Builds the iPhone app for the simulator (after ios-framework and ios-web); open ios/Mardo.xcodeproj to run it.
ios-app:
	cd ios && xcodegen generate && xcodebuild -project Mardo.xcodeproj -scheme Mardo -destination 'generic/platform=iOS Simulator' -derivedDataPath build CODE_SIGN_IDENTITY=- build

# Builds for a connected device, signed by your Apple Development team: make ios-device TEAM=XXXXXXXXXX
ios-device:
	cd ios && xcodegen generate && xcodebuild -project Mardo.xcodeproj -scheme Mardo -destination 'generic/platform=iOS' -derivedDataPath build-device -allowProvisioningUpdates DEVELOPMENT_TEAM=$(TEAM) CODE_SIGN_STYLE=Automatic build

# Archives for the App Store and opens it in Xcode's Organizer: make ios-archive TEAM=XXXXXXXXXX BUILD=2
# Then Distribute App > App Store Connect > Upload. (xcodebuild -exportArchive with destination upload hung on this
# Mac every time, while Organizer's upload worked.) Raise BUILD for every upload of the same version. Needs the app
# record for com.henryvandenbroek.mardo to exist.
ios-archive: ios-framework ios-web
	cd ios && xcodegen generate && xcodebuild -project Mardo.xcodeproj -scheme Mardo -destination 'generic/platform=iOS' -archivePath build-archive/Mardo.xcarchive -allowProvisioningUpdates DEVELOPMENT_TEAM=$(TEAM) CODE_SIGN_STYLE=Automatic CURRENT_PROJECT_VERSION=$(BUILD) archive
	open ios/build-archive/Mardo.xcarchive
