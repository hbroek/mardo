import Foundation
import SafariServices
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
        if method == "OpenURL" { // a page of the web, in the app's own browser sheet (see openInBrowser)
            Self.openInBrowser(args)
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
            if method == "ConnectFinish" || method == "ConnectCancel" { Self.closeBrowser() } // the sign-in page has done its job
            replyHandler(answer, nil)
        }
    }

    /// Shows a web page in a Safari sheet inside the app. The app stays in front while the reader signs in to a site,
    /// so the address Mardo listens on for the answer (127.0.0.1) keeps working; switching to the Safari app would
    /// let iOS suspend Mardo and the answer would never arrive. `argsJSON` is `[url]`.
    private static func openInBrowser(_ argsJSON: String) {
        guard let data = argsJSON.data(using: .utf8), let list = try? JSONSerialization.jsonObject(with: data) as? [String],
              let item = list.first, let url = URL(string: item), url.scheme == "https" || url.scheme == "http" else { return }
        DispatchQueue.main.async {
            guard let top = DeviceHost.topController() else { return }
            top.present(SFSafariViewController(url: url), animated: true)
        }
    }

    /// Closes the Safari sheet, if one is showing.
    private static func closeBrowser() {
        DispatchQueue.main.async {
            if let sheet = DeviceHost.topController() as? SFSafariViewController { sheet.dismiss(animated: true) }
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
