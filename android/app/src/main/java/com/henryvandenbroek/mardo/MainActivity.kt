package com.henryvandenbroek.mardo

import android.annotation.SuppressLint
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.webkit.JavascriptInterface
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import androidx.activity.OnBackPressedCallback
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.webkit.WebViewCompat
import androidx.webkit.WebViewFeature
import org.json.JSONArray
import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.io.File
import java.util.concurrent.Executors

/** Opens a document for reading and writing, and keeps the permission across restarts. */
private class OpenForEdit : ActivityResultContracts.OpenDocument() {
    override fun createIntent(context: Context, input: Array<String>): Intent =
        super.createIntent(context, input).addFlags(Intent.FLAG_GRANT_WRITE_URI_PERMISSION or Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION)
}

private class CreateForEdit : ActivityResultContracts.CreateDocument("text/markdown") {
    override fun createIntent(context: Context, input: String): Intent =
        super.createIntent(context, input).addFlags(Intent.FLAG_GRANT_WRITE_URI_PERMISSION or Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION)
}

/** The web view that shows the page, wired to the Go core. */
class MainActivity : AppCompatActivity() {
    private val origin = "https://appassets.androidplatform.net"
    private lateinit var web: WebView
    private lateinit var documents: Documents
    private lateinit var host: DeviceHost
    private lateinit var engine: mobile.Engine
    private val calls = Executors.newCachedThreadPool()

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        documents = Documents(this)
        host = DeviceHost(documents)
        val dir = File(filesDir, "mardo").apply { mkdirs() }
        engine = mobile.Mobile.new_(dir.path, Secrets(this), host)

        host.openPicker = registerForActivityResult(OpenForEdit()) { uri -> finished(uri) }
        host.savePicker = registerForActivityResult(CreateForEdit()) { uri -> finished(uri) }

        web = WebView(this)
        host.web = web
        val frame = FrameLayout(this)
        frame.addView(web, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT))
        setContentView(frame)
        val dark = (resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK) == android.content.res.Configuration.UI_MODE_NIGHT_YES
        WindowCompat.getInsetsController(window, frame).apply { isAppearanceLightStatusBars = !dark; isAppearanceLightNavigationBars = !dark } // readable clock and icons on the page's own colors
        // The system bars and the keyboard take their space; the page lays itself out in the rest.
        ViewCompat.setOnApplyWindowInsetsListener(frame) { v, insets ->
            val bars = insets.getInsets(WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.ime() or WindowInsetsCompat.Type.displayCutout())
            v.setPadding(bars.left, bars.top, bars.right, bars.bottom)
            WindowInsetsCompat.CONSUMED
        }
        web.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            allowFileAccess = false
            allowContentAccess = false
            mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
            setSupportZoom(false)
            builtInZoomControls = false
        }
        web.addJavascriptInterface(Bridge(), "MardoAndroid")
        if (WebViewFeature.isFeatureSupported(WebViewFeature.DOCUMENT_START_SCRIPT)) {
            // Where the pages the reader saves live; the page shows addresses there as Mardo/name.md.
            WebViewCompat.addDocumentStartJavaScript(web, "window.mardoDocuments = ${JSONObject.quote(documents.folder.path)};", setOf(origin))
        }
        web.webViewClient = object : WebViewClient() {
            override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? {
                val url = request.url
                if (url.scheme + "://" + url.host != origin) return null
                return serve(url)
            }

            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                if (request.url.host == Uri.parse(origin).host) return false
                runCatching { startActivity(Intent(Intent.ACTION_VIEW, request.url)) }
                return true
            }
        }
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                // The page's Back button first; at the start of its history, leave the app.
                web.evaluateJavascript("(() => { const b = document.getElementById('back'); if (b && !b.disabled) { b.click(); return true; } return false; })()") {
                    if (it != "true") finish()
                }
            }
        })
        web.loadUrl("$origin/index.html")
    }

    private fun finished(uri: Uri?) {
        if (uri != null) runCatching {
            contentResolver.takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION)
        }
        host.answer?.complete(uri)
    }

    /** The page's own files from the app, and anything else (web images that need the app's credentials, files on disk) from the core. */
    private fun serve(url: Uri): WebResourceResponse {
        var path = url.path.orEmpty().ifEmpty { "/" }
        if (path == "/") path = "/index.html"
        val clean = path.trimStart('/')
        if (!clean.contains("..")) {
            try {
                val stream = assets.open("dist/$clean")
                return WebResourceResponse(mime(clean), null, 200, "OK", mapOf("Cache-Control" to "no-cache"), stream)
            } catch (e: java.io.FileNotFoundException) {
                // not one of the page's files
            }
        }
        val withQuery = url.encodedQuery?.let { "$path?$it" } ?: path
        return try {
            val a = engine.asset(withQuery)
            WebResourceResponse(a.type.ifEmpty { "application/octet-stream" }, null, a.status.toInt(), "OK", mapOf("Cache-Control" to "no-cache"), ByteArrayInputStream(a.body ?: ByteArray(0)))
        } catch (e: Exception) {
            WebResourceResponse("text/plain", "utf-8", 500, "Error", emptyMap(), ByteArrayInputStream((e.message ?: "error").toByteArray()))
        }
    }

    private fun mime(name: String): String = when (name.substringAfterLast('.').lowercase()) {
        "html" -> "text/html"
        "js", "mjs" -> "text/javascript"
        "css" -> "text/css"
        "svg" -> "image/svg+xml"
        "png" -> "image/png"
        "jpg", "jpeg" -> "image/jpeg"
        "woff2" -> "font/woff2"
        "json" -> "application/json"
        else -> "application/octet-stream"
    }

    /** What the page calls: window.MardoAndroid.call(id, method, argsJSON). The answer comes back to window.mardoReply(id, json). */
    private inner class Bridge {
        @JavascriptInterface
        fun call(id: Int, method: String, args: String) {
            if (method == "Share") { // the share sheet is the app's own, not the core's
                share(args)
                reply(id, "{\"ok\":null}")
                return
            }
            if (method == "OpenURL") { // a page of the web, in the browser (also the sign-in page of a site)
                val url = runCatching { Uri.parse(JSONArray(args).getString(0)) }.getOrNull()
                if (url != null && (url.scheme == "https" || url.scheme == "http")) runOnUiThread { runCatching { startActivity(Intent(Intent.ACTION_VIEW, url)) } }
                reply(id, "{\"ok\":null}")
                return
            }
            // Off the main thread: a call can take as long as a page load or an AI answer, and the core may
            // ask for a file picker, which waits for the main thread.
            calls.execute {
                val answer = engine.call(method, args)
                documents.sync()
                reply(id, answer)
            }
        }

        private fun reply(id: Int, json: String) {
            runOnUiThread { web.evaluateJavascript("window.mardoReply($id, ${JSONObject.quote(json)})", null) }
        }

        private fun share(argsJSON: String) {
            val item = runCatching { JSONArray(argsJSON).getString(0) }.getOrNull() ?: return
            val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, item)
            runOnUiThread { startActivity(Intent.createChooser(send, null)) }
        }
    }
}
