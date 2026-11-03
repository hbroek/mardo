import Foundation
import WebKit

/// Debug builds forward what the page logs and its uncaught errors to the native log.
final class Log: NSObject, WKScriptMessageHandler, WKNavigationDelegate {
    static let script = """
    (function () {
      const send = (level, args) => { try { window.webkit.messageHandlers.mardoLog.postMessage(level + ': ' + args.map(a => a instanceof Error ? a.stack || a.message : typeof a === 'string' ? a : JSON.stringify(a)).join(' ')); } catch (e) {} };
      for (const level of ['log', 'warn', 'error']) { const old = console[level].bind(console); console[level] = (...a) => { send(level, a); old(...a); }; }
      window.addEventListener('error', (e) => send('uncaught', [e.message + ' at ' + e.filename + ':' + e.lineno]));
      window.addEventListener('unhandledrejection', (e) => send('unhandled rejection', [e.reason]));
    })();
    """

    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage) {
        NSLog("[page] %@", String(describing: message.body))
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { NSLog("[page] loaded %@", webView.url?.absoluteString ?? "") }
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) { NSLog("[page] failed: %@", error.localizedDescription) }
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) { NSLog("[page] failed to start: %@", error.localizedDescription) }
}
