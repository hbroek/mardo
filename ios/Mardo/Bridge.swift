import Foundation
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
}
