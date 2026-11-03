import Foundation
import UniformTypeIdentifiers
import WebKit
import Mardo

/// Serves mardo://app/… : the page's own files straight from the app, and anything else (images of web
/// pages that need the app's credentials, files on disk) from the Go core.
final class AssetHandler: NSObject, WKURLSchemeHandler {
    var engine: MobileEngine!
    private let root = Bundle.main.bundleURL.appendingPathComponent("dist-ios", isDirectory: true)

    func webView(_ webView: WKWebView, start task: WKURLSchemeTask) {
        guard let url = task.request.url else { return fail(task, "no address") }
        var path = url.path.isEmpty ? "/" : url.path
        if path == "/" { path = "/index.html" }
        #if DEBUG
        NSLog("[page] asset %@", path)
        #endif

        if let file = bundled(path) {
            do {
                let data = try Data(contentsOf: file)
                respond(task, url: url, status: 200, type: mime(for: file), body: data)
            } catch {
                fail(task, error.localizedDescription)
            }
            return
        }
        // Not one of the page's files: the core knows (it answers 404 for what it does not).
        let withQuery = url.query.map { "\(path)?\($0)" } ?? path
        DispatchQueue.global(qos: .userInitiated).async {
            do {
                let asset = try self.engine.asset(withQuery)
                self.respond(task, url: url, status: asset.status, type: asset.type, body: asset.body ?? Data())
            } catch {
                self.fail(task, error.localizedDescription)
            }
        }
    }

    func webView(_ webView: WKWebView, stop task: WKURLSchemeTask) {}

    /// The file under the page's folder for `path`, or nil: it must stay inside the folder.
    private func bundled(_ path: String) -> URL? {
        let file = root.appendingPathComponent(path).standardizedFileURL
        guard file.path.hasPrefix(root.standardizedFileURL.path + "/") else { return nil }
        var isDir: ObjCBool = false
        return FileManager.default.fileExists(atPath: file.path, isDirectory: &isDir) && !isDir.boolValue ? file : nil
    }

    private func mime(for file: URL) -> String {
        UTType(filenameExtension: file.pathExtension)?.preferredMIMEType ?? "application/octet-stream"
    }

    private func respond(_ task: WKURLSchemeTask, url: URL, status: Int, type: String, body: Data) {
        let headers = ["Content-Type": type.isEmpty ? "application/octet-stream" : type, "Content-Length": "\(body.count)", "Cache-Control": "no-cache"]
        guard let response = HTTPURLResponse(url: url, statusCode: status, httpVersion: "HTTP/1.1", headerFields: headers) else { return fail(task, "bad response") }
        DispatchQueue.main.async {
            task.didReceive(response)
            task.didReceive(body)
            task.didFinish()
        }
    }

    private func fail(_ task: WKURLSchemeTask, _ message: String) {
        DispatchQueue.main.async { task.didFailWithError(NSError(domain: "mardo", code: 1, userInfo: [NSLocalizedDescriptionKey: message])) }
    }
}
