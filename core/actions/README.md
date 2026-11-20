# Send to actions

Each `.md` file in this folder is a button on Mardo's **Send to toolbar** (switch it on in the ☰ menu). The toolbar reads
this folder again each time you come back to Mardo, so a file you add or change shows at once. This README is not an action. Delete or change the two examples as you like:
they are only put here the first time.

An action file has front matter that says what it is, and then, for an API call or the clipboard, the text to send:

```
---
name: Weft it
icon: favicon
url: https://weft.example.com/compose?page={{url}}&ptitle={{title}}&pdesc={{description}}&pimg={{image}}
---
```

## The front matter

| Key | What it does |
| --- | --- |
| `name` | The text of the button. The file name if you leave it out. |
| `icon` | An emoji or a few letters; a picture file in this folder (`weft.png`); a web address of a picture; or `favicon` for the icon of the site in `url`. |
| `url` | An http or https address. Placeholders in it are percent-encoded. |
| `method` | `GET` (default), `POST`, `PUT`, `PATCH` or `DELETE`. |
| `mode` | `browser`: open `url` in your default browser, where you are signed in (the default for `GET`). `api`: Mardo calls `url` itself and shows only the result, using the sign-in saved for that site (menu, Site credentials) (the default for the other methods). `copy`: put the text on the clipboard; no `url`. |
| `contentType` | Of the body: `application/json`, `text/plain`, and so on. JSON when the body starts with `{` or `[`, else text. |
| `order` | A number: where the button goes in the toolbar. The lowest comes first; actions without one come after them, by name. Leave gaps (10, 20, 30) to slip another in later. |
| `requires` | `selection`: the action only runs with text selected in the page, and says so when there is none. |

The **✎** button at the end of the toolbar opens a page that lists these files, so that each can be opened and edited in Mardo, and a new one can be made from a template. That is the way to do it on a phone.

A line that starts with `#` in the front matter is a comment, and is ignored. Everything after the closing `---` is the body: what is sent by `POST`, `PUT` and `PATCH`, or copied by `copy`.

## Placeholders

Write them in the `url` and the body as `{{name}}`. The names are not case sensitive. One the page does not have gives
nothing, so `{{description}}` on a page without one is empty.

| Placeholder | Is |
| --- | --- |
| `{{url}}` | The address of the page. |
| `{{title}}` | Its title. |
| `{{description}}` | The `description` of its front matter, else its first paragraph (up to 200 characters). |
| `{{image}}` | The `image` or `rightMarginImage` of its front matter, else its first picture, as a full address. |
| `{{keywords}}` | The `keywords` or `tags` of its front matter. |
| `{{selection}}` | The text you had selected in the page. |
| `{{body}}` | The markdown of the page without its front matter. |
| `{{fullbody}}` | The markdown with its front matter. |
| `{{host}}` | The site, as in `www.example.com`. |
| `{{date}}`, `{{datetime}}` | Today as `2026-11-20`, and now as `2026-11-20T10:15:00-05:00` (ISO 8601). |
| `{{author}}`, `{{pubDate}}`, ... | Any key of the page's front matter, also as `{{page.author}}`. A nested one is `{{source.url}}`. |

In an address the values are percent-encoded. In a JSON body they are escaped to sit inside a string, so write
`"title": "{{title}}"` with the quotes yourself. Anywhere else they are put in as they are.

## Examples

Open a service's post page (this is what a bookmarklet does):

```
---
name: Weft it
icon: favicon
url: https://weft.example.com/compose?page={{url}}&ptitle={{title}}&pdesc={{description}}&pimg={{image}}&keywords={{keywords}}
---
```

Post to an API without showing anything, with the sign-in saved in Mardo for that site:

```
---
name: Save to my notes
icon: 📝
method: POST
url: https://notes.example.com/api/notes
---
{
  "link": "{{url}}",
  "title": "{{title}}",
  "quote": "{{selection}}",
  "added": "{{datetime}}"
}
```

Save the whole page's markdown to a server that accepts `PUT`:

```
---
name: Save a copy
method: PUT
url: https://files.example.com/pages/{{host}}.md
contentType: text/markdown
---
{{fullbody}}
```

Search for the text you selected (the same works for `https://claude.ai/new?q=` and `https://chatgpt.com/?q=`):

```
---
name: Search Google for the selection
icon: favicon
requires: selection
url: https://www.google.com/search?q={{selection}}
---
```

Put a quote on the clipboard:

```
---
name: Copy selection as a quote
mode: copy
---
> {{selection}}
>
> — [{{title}}]({{url}})
```

The first time an `api` action runs, Mardo says where it will send the page and asks you to agree; if you edit the
file it asks again. Every action also leaves a line in the network console (Ctrl/Cmd+J) with the address it used.
