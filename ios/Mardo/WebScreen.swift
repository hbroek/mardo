import SwiftUI
import WebKit

/// The web view that shows the page, wired to the Go core.
struct WebScreen: UIViewRepresentable {
    func makeCoordinator() -> Core { Core() }

    func makeUIView(context: Context) -> WKWebView {
        let core = context.coordinator
        let config = WKWebViewConfiguration()
        config.setURLSchemeHandler(core.assets, forURLScheme: Core.scheme)
        config.userContentController.addScriptMessageHandler(core.bridge, contentWorld: .page, name: "mardo")
        #if DEBUG
        // The page's errors and console output, in Xcode's console (and `simctl spawn booted log stream`).
        config.userContentController.add(core.log, name: "mardoLog")
        config.userContentController.addUserScript(WKUserScript(source: Log.script, injectionTime: .atDocumentStart, forMainFrameOnly: true))
        #endif
        let web = WKWebView(frame: .zero, configuration: config)
        web.isInspectable = true
        #if DEBUG
        web.navigationDelegate = core.log
        #endif
        web.scrollView.contentInsetAdjustmentBehavior = .never // the page handles the safe area itself
        core.attach(web)
        web.load(URLRequest(url: URL(string: "\(Core.scheme)://app/index.html")!))
        return web
    }

    func updateUIView(_ web: WKWebView, context: Context) {}
}
