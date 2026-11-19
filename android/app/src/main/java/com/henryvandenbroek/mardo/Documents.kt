package com.henryvandenbroek.mardo

import android.content.Context
import android.net.Uri
import android.provider.OpenableColumns
import org.json.JSONObject
import java.io.File

/**
 * Android hands out files as content:// addresses, and the core reads and writes paths. So a file the reader
 * opens or saves is kept as a copy in the app's Documents folder, which is what the core works on, and each
 * change to the copy is written back to the original (see [sync]). The links between copies and originals are
 * remembered across restarts.
 */
class Documents(private val context: Context) {
    val folder = File(context.filesDir, "Documents").apply { mkdirs() }
    private val prefs = context.getSharedPreferences("documents", Context.MODE_PRIVATE)
    private val synced = HashMap<String, Long>() // copy's path -> its modification time when last written back

    /** Copy path -> original address. */
    private fun links(): JSONObject = JSONObject(prefs.getString("links", "{}") ?: "{}")

    private fun remember(path: String, uri: Uri) {
        prefs.edit().putString("links", links().put(path, uri.toString()).toString()).apply()
    }

    private fun existing(uri: Uri): String? {
        val l = links()
        return l.keys().asSequence().firstOrNull { l.getString(it) == uri.toString() && File(it).exists() }
    }

    private fun displayName(uri: Uri): String {
        context.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use {
            if (it.moveToFirst()) it.getString(0)?.let { n -> return n.substringAfterLast('/') }
        }
        return uri.lastPathSegment?.substringAfterLast('/') ?: "untitled.md"
    }

    private fun freshFile(name: String): File {
        var f = File(folder, name)
        var n = 2
        val base = name.substringBeforeLast('.', name)
        val ext = name.substringAfterLast('.', "").let { if (it.isEmpty()) "" else ".$it" }
        while (f.exists()) f = File(folder, "$base $n$ext").also { n++ }
        return f
    }

    /** A file the reader chose to open: its copy's path. The copy is made again each time, so it is current. */
    fun open(uri: Uri): String {
        val file = existing(uri)?.let { File(it) } ?: freshFile(displayName(uri))
        context.contentResolver.openInputStream(uri)?.use { input -> file.outputStream().use { input.copyTo(it) } }
        remember(file.path, uri)
        synced[file.path] = file.lastModified()
        return file.path
    }

    /** A file the reader chose to save to (the system made it, empty): the path of its copy, which the core writes. */
    fun create(uri: Uri): String {
        val file = existing(uri)?.let { File(it) } ?: freshFile(displayName(uri))
        file.writeText("")
        remember(file.path, uri)
        synced[file.path] = 0
        return file.path
    }

    /** Writes each copy that changed since the last time back to its original. Called after every call into the core. */
    @Synchronized
    fun sync() {
        val l = links()
        for (path in l.keys().asSequence().toList()) {
            val f = File(path)
            if (!f.exists() || f.lastModified() == (synced[path] ?: f.lastModified().also { synced[path] = it })) continue
            try {
                context.contentResolver.openOutputStream(Uri.parse(l.getString(path)), "wt")?.use { out -> f.inputStream().use { it.copyTo(out) } }
                synced[path] = f.lastModified()
            } catch (e: Exception) {
                // the original is gone or no longer allowed: the copy is still there
            }
        }
    }
}
