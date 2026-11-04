import Foundation
import UIKit
import WebKit
import Mardo

/// Receives the page's calls ({method, args}) and answers with what the Go core says.
final class Bridge: NSObject, WKScriptMessageHandlerWithReply {
    var engine: MobileEngine!

    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage,
                               replyHandler: @escaping (Any?, String?) -> Void) {
        guard let body = message.body as? [String: Any], let method = body["method"] as? String else {
            replyHandler(nil, "the message has no method")
            return
        }
        let args = body["args"] as? String ?? "[]"
        if method == "Share" { // the share sheet is the app's own, not the core's
            Self.share(args)
            replyHandler("{\"ok\":null}", nil)
            return
        }
        // Off the main thread: a call can take as long as a page load or an AI answer, and the core may
        // ask for a file picker, which waits for the main thread.
        DispatchQueue.global(qos: .userInitiated).async {
            #if DEBUG
            NSLog("[page] call %@ (%d chars) %@", method, args.count, String(args.prefix(120)))
            #endif
            let answer = self.engine.call(method, argsJSON: args)
            #if DEBUG
            NSLog("[page] answered %@: %@", method, String(answer.prefix(method == "SaveSource" ? 600 : 120)))
            #endif
            replyHandler(answer, nil)
        }
    }

    /// Opens the share sheet for the page's address, its file, or its text (`argsJSON` is `[item]`).
    private static func share(_ argsJSON: String) {
        guard let data = argsJSON.data(using: .utf8), let list = try? JSONSerialization.jsonObject(with: data) as? [String], let item = list.first else { return }
        let shared: Any
        if item.hasPrefix("file://"), let url = URL(string: item) { shared = url }
        else if item.hasPrefix("http://") || item.hasPrefix("https://"), let url = URL(string: item) { shared = url }
        else { shared = item }
        DispatchQueue.main.async {
            guard let top = DeviceHost.topController() else { return }
            let sheet = UIActivityViewController(activityItems: [shared], applicationActivities: nil)
            // On an iPad the sheet is a popover, which needs somewhere to point to: the toolbar's right side.
            sheet.popoverPresentationController?.sourceView = top.view
            sheet.popoverPresentationController?.sourceRect = CGRect(x: top.view.bounds.maxX - 90, y: 90, width: 1, height: 1)
            top.present(sheet, animated: true)
        }
    }
}
