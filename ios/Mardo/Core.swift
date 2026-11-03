import Foundation
import WebKit
import Mardo // the Go core, from gomobile (Mobile* names)

/// Owns the Go engine and the pieces that connect it to the web view.
final class Core {
    static let scheme = "mardo"

    private(set) var engine: MobileEngine!
    let host = DeviceHost()
    let bridge = Bridge()
    let assets = AssetHandler()
    let log = Log()

    init() {
        let dir = (try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
            .appendingPathComponent("mardo", isDirectory: true)) ?? URL(fileURLWithPath: NSTemporaryDirectory())
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        engine = MobileNew(dir.path, Keychain(), host)
        bridge.engine = engine
        assets.engine = engine
    }

    func attach(_ web: WKWebView) {
        host.web = web
    }
}
