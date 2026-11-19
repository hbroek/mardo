package com.henryvandenbroek.mardo

import android.content.Intent
import android.net.Uri
import android.webkit.WebView
import androidx.activity.result.ActivityResultLauncher
import org.json.JSONObject
import java.util.concurrent.CompletableFuture

/** What the Go core asks of the device: file pickers, events for the page. The core calls these off the main thread. */
class DeviceHost(private val documents: Documents) : mobile.Host {
    var web: WebView? = null
    var openPicker: ActivityResultLauncher<Array<String>>? = null
    var savePicker: ActivityResultLauncher<String>? = null

    /** The answer of the picker that is showing; the result callbacks in MainActivity complete it. */
    @Volatile var answer: CompletableFuture<Uri?>? = null

    private fun ask(show: () -> Unit): Uri? {
        val f = CompletableFuture<Uri?>()
        answer = f
        web?.post(show)
        return try { f.get() } finally { answer = null }
    }

    override fun pickOpen(title: String, filtersJSON: String): String {
        val uri = ask { openPicker?.launch(arrayOf("text/*", "application/octet-stream", "*/*")) } ?: return ""
        return documents.open(uri)
    }

    override fun pickSave(title: String, dir: String, name: String, filtersJSON: String): String {
        val uri = ask { savePicker?.launch(name.ifEmpty { "untitled.md" }) } ?: return ""
        return documents.create(uri)
    }

    override fun emit(event: String, dataJSON: String) {
        val js = "window.mardoEmit && window.mardoEmit(${JSONObject.quote(event)}, ${JSONObject.quote(dataJSON)})"
        web?.post { web?.evaluateJavascript(js, null) }
    }

    override fun setChecked(id: String, on: Boolean) {} // a phone has no menu bar

    // Apple Intelligence has no counterpart here: the core offers AI only with the reader's own key.
    override fun aiStatus(): String = "The device's own model is not available on Android."

    override fun aiChat(messagesJSON: String): String = throw UnsupportedOperationException("no on-device model")
}
