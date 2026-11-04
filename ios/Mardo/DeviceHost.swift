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

    /// Where pages go when no other folder is chosen, and what the page shows as ~/ : the app's Documents folder (Files > On My iPad > Mardo).
    static var documents: URL {
        FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
    }

    // MARK: MobileHostProtocol

    /// Shows the document picker and waits for it. The core calls this off the main thread (see Bridge).
    /// The file is opened where it is, so saving writes back to it, and access to it is kept across restarts.
    func pickOpen(_ title: String?, filtersJSON: String?, error: NSErrorPointer) -> String {
        let types: [UTType] = [.plainText, .text, UTType(filenameExtension: "md") ?? .plainText, UTType(filenameExtension: "markdown") ?? .plainText, .html, .data]
        guard let url = pick(types, asCopy: false) else { return "" }
        Scoped.shared.retain(url)
        return url.path
    }

    /// There is no save panel to give a path on iOS. The reader names the file in an alert; it goes to the folder
    /// chosen the first time (or with "Choose Folder…"), anywhere in Files, such as iCloud Drive: outside the
    /// app, so it survives deleting and reinstalling it. A name that is taken gets a number. Canceling gives "".
    func pickSave(_ title: String?, dir: String?, name: String?, filtersJSON: String?, error: NSErrorPointer) -> String {
        let suggestion = (name?.isEmpty == false) ? name! : "untitled.md"
        let current = Scoped.shared.saveFolder
        var chosen: String?
        var changeFolder = current == nil
        DispatchQueue.main.async {
            let where_ = current.map { "in \"\($0.lastPathComponent)\"" } ?? "in a folder you choose next"
            let alert = UIAlertController(title: title ?? "Save", message: "Name the file. It is saved \(where_), in Files.", preferredStyle: .alert)
            alert.addTextField { field in
                field.text = suggestion
                field.autocapitalizationType = .none
                field.autocorrectionType = .no
                field.clearButtonMode = .whileEditing
            }
            let finish = { (folder: Bool) in
                chosen = alert.textFields?.first?.text?.trimmingCharacters(in: .whitespacesAndNewlines)
                changeFolder = changeFolder || folder
                self.pickerDone.signal()
            }
            alert.addAction(UIAlertAction(title: "Cancel", style: .cancel) { _ in chosen = nil; self.pickerDone.signal() })
            if current != nil { alert.addAction(UIAlertAction(title: "Choose Folder…", style: .default) { _ in finish(true) }) }
            alert.addAction(UIAlertAction(title: "Save", style: .default) { _ in finish(false) })
            Self.topController()?.present(alert, animated: true) {
                // Select the name without its extension, so typing replaces just that.
                if let field = alert.textFields?.first, let dot = suggestion.lastIndex(of: "."), let start = field.position(from: field.beginningOfDocument, offset: 0), let end = field.position(from: field.beginningOfDocument, offset: suggestion.distance(from: suggestion.startIndex, to: dot)) {
                    field.selectedTextRange = field.textRange(from: start, to: end)
                }
            }
        }
        pickerDone.wait()
        guard let chosen, !chosen.isEmpty else { return "" }
        var folder = current
        if changeFolder {
            guard let picked = pick([.folder], asCopy: false) else { return "" }
            Scoped.shared.saveFolder = picked
            folder = picked
        }
        guard let folder else { return "" }
        // Only a file name: no folders from the typed text.
        let file = (chosen as NSString).lastPathComponent
        return Self.unique(folder.appendingPathComponent(file)).path
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

    /// Shows the system picker and waits for the answer (called off the main thread).
    private func pick(_ types: [UTType], asCopy: Bool) -> URL? {
        picked = nil
        DispatchQueue.main.async {
            let picker = UIDocumentPickerViewController(forOpeningContentTypes: types, asCopy: asCopy)
            picker.delegate = self
            picker.allowsMultipleSelection = false
            Self.topController()?.present(picker, animated: true)
        }
        pickerDone.wait()
        return picked
    }

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
