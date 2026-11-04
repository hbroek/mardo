import Foundation

/// Files and folders the reader picked outside the app (iCloud Drive, On My iPad, another provider).
/// iOS lets the app touch them only while it holds their security-scoped access, and only after a restart
/// if it kept a bookmark, so both are done here. Pages saved there survive deleting and reinstalling the app.
final class Scoped {
    static let shared = Scoped()

    private let bookmarksKey = "scopedBookmarks"   // [path: bookmark data]
    private let folderKey = "saveFolderPath"        // where new pages go
    private var held: [String: URL] = [:]           // access is kept for as long as the app runs

    /// Takes up everything remembered from earlier runs. Call it before the page asks for a file.
    func restore() {
        let saved = UserDefaults.standard.dictionary(forKey: bookmarksKey) as? [String: Data] ?? [:]
        var kept = saved
        for (path, data) in saved {
            var stale = false
            guard let url = try? URL(resolvingBookmarkData: data, options: [], relativeTo: nil, bookmarkDataIsStale: &stale) else {
                kept[path] = nil // the folder is gone
                continue
            }
            _ = url.startAccessingSecurityScopedResource()
            held[url.path] = url
            if stale, let fresh = try? url.bookmarkData() { kept[path] = fresh }
        }
        UserDefaults.standard.set(kept, forKey: bookmarksKey)
    }

    /// Keeps access to a picked file or folder, now and after a restart.
    func retain(_ url: URL) {
        guard held[url.path] == nil else { return }
        _ = url.startAccessingSecurityScopedResource()
        held[url.path] = url
        guard let data = try? url.bookmarkData() else { return }
        var saved = UserDefaults.standard.dictionary(forKey: bookmarksKey) as? [String: Data] ?? [:]
        saved[url.path] = data
        UserDefaults.standard.set(saved, forKey: bookmarksKey)
    }

    /// The folder new pages are saved in, while it is still there.
    var saveFolder: URL? {
        get {
            guard let path = UserDefaults.standard.string(forKey: folderKey), let url = held[path],
                  FileManager.default.fileExists(atPath: url.path) else { return nil }
            return url
        }
        set {
            if let url = newValue { retain(url) }
            UserDefaults.standard.set(newValue?.path, forKey: folderKey)
        }
    }
}
