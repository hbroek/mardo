import SwiftUI

@main
struct MardoApp: App {
    var body: some Scene {
        WindowGroup {
            WebScreen().ignoresSafeArea(.container, edges: .bottom)
        }
    }
}
