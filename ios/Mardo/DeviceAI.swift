import Foundation
#if canImport(FoundationModels)
import FoundationModels
#endif

/// Apple Intelligence's language model, which runs on the device (iOS 26 and later, on supported devices). The Go core
/// offers it as the default AI provider where it is available; the text never leaves the device.
enum DeviceAI {
    /// "" when the model can be used, else why not, in words for the reader.
    static func status() -> String {
        #if canImport(FoundationModels)
        if #available(iOS 26.0, *) {
            switch SystemLanguageModel.default.availability {
            case .available:
                return ""
            case .unavailable(.deviceNotEligible):
                return "This device does not support Apple Intelligence."
            case .unavailable(.appleIntelligenceNotEnabled):
                return "Apple Intelligence is turned off. Turn it on in Settings > Apple Intelligence & Siri."
            case .unavailable(.modelNotReady):
                return "Apple Intelligence is still getting its model ready. Try again in a while."
            case .unavailable:
                return "Apple Intelligence is not available right now."
            }
        }
        #endif
        return "Apple Intelligence needs iOS 26 or later."
    }

    /// One answer to a conversation: the system message is the instructions, the earlier turns go in as a transcript
    /// and the last message is the question.
    static func chat(_ messages: [(role: String, content: String)]) async throws -> String {
        #if canImport(FoundationModels)
        if #available(iOS 26.0, *) {
            let instructions = messages.filter { $0.role == "system" }.map(\.content).joined(separator: "\n\n")
            let turns = messages.filter { $0.role != "system" }
            guard let last = turns.last else { return "" }
            var prompt = ""
            if turns.count > 1 {
                prompt += "The conversation so far:\n"
                for turn in turns.dropLast() { prompt += (turn.role == "assistant" ? "Assistant: " : "User: ") + turn.content + "\n" }
                prompt += "\nNow answer this:\n"
            }
            prompt += last.content
            let session = LanguageModelSession(instructions: instructions)
            do {
                return try await session.respond(to: prompt).content
            } catch let error as LanguageModelSession.GenerationError {
                switch error {
                case .exceededContextWindowSize:
                    throw DeviceAIError("The text is too long for the model on this device. Choose another provider (menu > AI assistant) for long pages.")
                case .guardrailViolation:
                    throw DeviceAIError("Apple Intelligence declined to answer this, because of its safety rules.")
                case .unsupportedLanguageOrLocale:
                    throw DeviceAIError("Apple Intelligence does not support the language of this text.")
                case .assetsUnavailable:
                    throw DeviceAIError("The Apple Intelligence model is not downloaded on this device yet. Check Settings > Apple Intelligence & Siri, or choose another provider.")
                case .rateLimited:
                    throw DeviceAIError("Apple Intelligence is busy. Try again in a moment.")
                case .concurrentRequests:
                    throw DeviceAIError("Apple Intelligence is answering something else. Try again in a moment.")
                default:
                    throw DeviceAIError("Apple Intelligence could not answer (\(String(describing: error))). Choose another provider under menu > AI assistant if this keeps happening.")
                }
            } catch let error as DeviceAIError {
                throw error
            } catch {
                let e = error as NSError
                NSLog("[ai] Apple Intelligence failed: %@", String(describing: e))
                throw DeviceAIError("Apple Intelligence could not answer here (\(e.domain.split(separator: ".").last ?? "error") \(e.code)). The model may not be installed on this device or in the simulator. Choose another provider under menu > AI assistant.")
            }
        }
        #endif
        throw DeviceAIError("Apple Intelligence needs iOS 26 or later.")
    }
}

struct DeviceAIError: LocalizedError {
    let message: String
    init(_ message: String) { self.message = message }
    var errorDescription: String? { message }
}
