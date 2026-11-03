import Foundation
import Security
import Mardo

/// The core's secrets (site logins, the AI key, the cookie key) in the iOS Keychain.
final class Keychain: NSObject, MobileSecretsProtocol {
    private let service = "mardo"

    private func query(_ name: String) -> [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service, kSecAttrAccount as String: name]
    }

    func get(_ name: String?) -> String {
        guard let name else { return "" }
        var q = query(name)
        q[kSecReturnData as String] = true
        q[kSecMatchLimit as String] = kSecMatchLimitOne
        var out: CFTypeRef?
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let data = out as? Data else { return "" }
        return String(data: data, encoding: .utf8) ?? ""
    }

    func set(_ name: String?, value: String?) throws {
        guard let name, let value else { return }
        let data = Data(value.utf8)
        let status = SecItemUpdate(query(name) as CFDictionary, [kSecValueData as String: data] as CFDictionary)
        if status == errSecItemNotFound {
            var q = query(name)
            q[kSecValueData as String] = data
            q[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
            let added = SecItemAdd(q as CFDictionary, nil)
            if added != errSecSuccess { throw NSError(domain: NSOSStatusErrorDomain, code: Int(added)) }
        } else if status != errSecSuccess {
            throw NSError(domain: NSOSStatusErrorDomain, code: Int(status))
        }
    }

    func delete(_ name: String?) {
        guard let name else { return }
        SecItemDelete(query(name) as CFDictionary)
    }
}
