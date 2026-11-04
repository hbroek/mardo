import Foundation
import UIKit
import UniformTypeIdentifiers
import WebKit
import Mardo

/// What the Go core asks of the device: file pickers, events for the page.
final class DeviceHost: NSObject, MobileHostProtocol, UIDocumentPickerDelegate {
    weak var web: WKWebView?

    private var picked: URL?
    private let pickerDone = DispatchSemaphore(value: 0)

    /// Pages the reader opens or saves live here, in the app's Documents folder (visible in the Files app).
    static var documents: URL {
        FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
    }

    // MARK: MobileHostProtocol

    /// Shows the document picker and waits for it. The core calls this off the main thread (see Bridge).
    func pickOpen(_ title: String?, filtersJSON: String?, error: NSErrorPointer) -> String {
        picked = nil
        DispatchQueue.main.async {
            let types: [UTType] = [.plainText, .text, UTType(filenameExtension: "md") ?? .plainText, UTType(filenameExtension: "markdown") ?? .plainText, .html, .data]
            let picker = UIDocumentPickerViewController(forOpeningContentTypes: types, asCopy: true) // a copy the app may change
            picker.delegate = self
            picker.allowsMultipleSelection = false
            Self.topController()?.present(picker, animated: true)
        }
        pickerDone.wait()
        guard let url = picked else { return "" }
        // Keep the copy in Documents so that it stays, and so that saving writes to a file the app owns.
        let target = Self.unique(Self.documents.appendingPathComponent(url.lastPathComponent))
        do {
            try FileManager.default.copyItem(at: url, to: target)
            return target.path
        } catch {
            return url.path
        }
    }

    /// There is no save panel to give a path on iOS: the reader names the file in an alert, and it goes to
    /// the Documents folder, which the Files app shows. A name that is taken gets a number. Canceling gives "".
    func pickSave(_ title: String?, dir: String?, name: String?, filtersJSON: String?, error: NSErrorPointer) -> String {
        let suggestion = (name?.isEmpty == false) ? name! : "untitled.md"
        var chosen: String?
        DispatchQueue.main.async {
            let alert = UIAlertController(title: title ?? "Save", message: "Name the file. It is saved in the Mardo folder in Files.", preferredStyle: .alert)
            alert.addTextField { field in
                field.text = suggestion
                field.autocapitalizationType = .none
                field.autocorrectionType = .no
                field.clearButtonMode = .whileEditing
            }
            alert.addAction(UIAlertAction(title: "Cancel", style: .cancel) { _ in chosen = nil; self.pickerDone.signal() })
            alert.addAction(UIAlertAction(title: "Save", style: .default) { _ in
                chosen = alert.textFields?.first?.text?.trimmingCharacters(in: .whitespacesAndNewlines)
                self.pickerDone.signal()
            })
            Self.topController()?.present(alert, animated: true) {
                // Select the name without its extension, so typing replaces just that.
                if let field = alert.textFields?.first, let dot = suggestion.lastIndex(of: "."), let start = field.position(from: field.beginningOfDocument, offset: 0), let end = field.position(from: field.beginningOfDocument, offset: suggestion.distance(from: suggestion.startIndex, to: dot)) {
                    field.selectedTextRange = field.textRange(from: start, to: end)
                }
            }
        }
        pickerDone.wait()
        guard let chosen, !chosen.isEmpty else { return "" }
        // Only a file name: no folders from the typed text.
        let file = (chosen as NSString).lastPathComponent
        return Self.unique(Self.documents.appendingPathComponent(file)).path
    }

    func emit(_ event: String?, dataJSON: String?) {
        guard let event, let dataJSON else { return }
        #if DEBUG
        if event == "net" { NSLog("[net] %@", String(dataJSON.prefix(400))) }
        #endif
        let js = "window.mardoEmit && window.mardoEmit(\(Self.literal(event)), \(Self.literal(dataJSON)))"
        DispatchQueue.main.async { self.web?.evaluateJavaScript(js) }
    }

    func setChecked(_ id_: String?, on: Bool) {} // a phone has no menu bar

    // MARK: UIDocumentPickerDelegate

    func documentPicker(_ controller: UIDocumentPickerViewController, didPickDocumentsAt urls: [URL]) {
        picked = urls.first
        pickerDone.signal()
    }

    func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
        picked = nil
        pickerDone.signal()
    }

    // MARK: helpers

    /// `name.md`, or `name 2.md`, `name 3.md` … when that exists already.
    static func unique(_ url: URL) -> URL {
        var candidate = url
        var n = 2
        while FileManager.default.fileExists(atPath: candidate.path) {
            let base = url.deletingPathExtension().lastPathComponent
            candidate = url.deletingLastPathComponent().appendingPathComponent("\(base) \(n)").appendingPathExtension(url.pathExtension)
            n += 1
        }
        return candidate
    }

    /// A string as a JavaScript literal.
    static func literal(_ s: String) -> String {
        let data = try? JSONSerialization.data(withJSONObject: [s], options: [])
        let json = data.flatMap { String(data: $0, encoding: .utf8) } ?? "[\"\"]"
        return String(json.dropFirst().dropLast())
    }

    static func topController() -> UIViewController? {
        let scene = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first
        var top = scene?.keyWindow?.rootViewController
        while let next = top?.presentedViewController { top = next }
        return top
    }
}
