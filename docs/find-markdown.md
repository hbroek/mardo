# Where to Find Markdown on the Web

A guide to feeds, sites and directories that serve **Markdown directly**, plus search tricks for finding more.

*Compiled November 5, 2026. Everything here was found through web search; the links were not fetched one by one, so test before you rely on them.*

---

## Contents

1. [The four ways the web serves Markdown](#1-the-four-ways-the-web-serves-markdown)
2. [RSS feeds with `<source:markdown>`](#2-rss-feeds-with-sourcemarkdown)
3. [Feed readers and tools that understand Markdown in RSS](#3-feed-readers-and-tools-that-understand-markdown-in-rss)
4. [Sites with `.md` versions of their pages](#4-sites-with-md-versions-of-their-pages)
5. [Sites that answer `Accept: text/markdown`](#5-sites-that-answer-accept-textmarkdown)
6. [Platforms that make every site Markdown-ready](#6-platforms-that-make-every-site-markdown-ready)
7. [`llms.txt` directories](#7-llmstxt-directories)
8. [How to search for more](#8-how-to-search-for-more)
9. [Checkers and test commands](#9-checkers-and-test-commands)

---

## 1. The four ways the web serves Markdown

| Method | What it looks like | Best for |
|--------|--------------------|----------|
| **`<source:markdown>` in RSS** | Each feed item carries the original Markdown next to the usual HTML `<description>` | Blogs, microblogs, feed readers |
| **`.md` URL suffix** | `example.com/docs/page` → `example.com/docs/page.md` | Documentation sites, some blogs |
| **Content negotiation** | Same URL; send the header `Accept: text/markdown` and get Markdown back instead of HTML | Agents, scripts, CDN-level support |
| **`llms.txt` / `llms-full.txt`** | A Markdown index at the site root that links to Markdown versions of the pages | Finding a site's Markdown pages |

---

## 2. RSS feeds with `<source:markdown>`

`source:markdown` is an element from Dave Winer's **Source namespace** for RSS. A reader that can render Markdown can use it in place of the item's `<description>`. The namespace is declared at the top of the feed:

```xml
<rss version="2.0" xmlns:source="https://source.scripting.com/">
  ...
  <item>
    <description>&lt;p&gt;Watching &lt;a href="..."&gt;Edge of Tomorrow&lt;/a&gt;&lt;/p&gt;</description>
    <source:markdown>Watching [Edge of Tomorrow](https://www.themoviedb.org/movie/137113)</source:markdown>
  </item>
```

| Feed / source | Feed URL | Notes |
|---------------|----------|-------|
| **Scripting News** (Dave Winer) | `http://scripting.com/rss.xml` | Supports `source:markdown`, per Winer's GitHub issue #336 |
| **daveverse** (Dave Winer, written in WordLand) | `https://wordland.social/scripting/246529703/rss.xml` | WordLand "shadow feed": every item includes `source:markdown` |
| **Manton Reece** (Micro.blog) | `https://www.manton.org/feed.xml` | Micro.blog adds `source:markdown` to every blog by default |
| **Any Micro.blog blog** | `https://<blog>/feed.xml` | Same default for all Micro.blog-hosted blogs |
| **Any WordLand user** | WordLand shadow feed for each WordPress site | Only includes posts written in WordLand |
| **lqdev.me** (Luis Quintanilla) | See the site's feeds | Added Markdown to its RSS feeds after the Micro.blog announcement |
| **Source namespace spec** | `https://source.scripting.com/markdown.opml` | Winer's page on Markdown in RSS, published as an OPML outline |

**Related namespaces**

- **`now:markdown`** – from Colin Walker's *Now* namespace, for sharing /now-page updates in Markdown. Works the same way as `source:markdown`. [Spec](https://github.com/colin-walker/Now-Namespace)
- **`derived:code`** – from Rubenerd's *derived* namespace. Links to a post's original Markdown source file instead of embedding it: `<derived:code type="text/markdown" url="..."/>`. [Spec](https://rubenerd.com/derived)

---

## 3. Feed readers and tools that understand Markdown in RSS

| Tool | Type | What it does with Markdown |
|------|------|---------------------------|
| [NetNewsWire 6.2+](https://netnewswire.blog/2025/11/05/netnewswire-for-mac-and-ios.html) | Mac/iOS feed reader | Saves `source:markdown` and renders it in the article view |
| FeedLand | Web feed reader (Dave Winer) | Reads and passes along Markdown from feeds |
| WordLand (`wordland.social`) | Writing app for WordPress | Stores posts in Markdown and publishes feeds that include `source:markdown` |
| [Micro.blog](https://micro.blog) | Blog host | Puts the original Markdown in every feed |
| [Markdown RSS (a8cteam51)](https://github.com/a8cteam51/team51-markdown-rss) | WordPress plugin | Adds `<source:markdown>` to WordPress RSS feeds |
| [Post Content to Markdown (Roots)](https://github.com/roots/post-content-to-markdown) | WordPress plugin | Full Markdown feed at `/feed/markdown/`, `.md` URLs, `Accept: text/markdown`, and an `<atom:link type="text/markdown">` in the RSS feed so readers can find it |
| [feedToMasto](https://github.com/scripting/feedToMasto) | Node app | Posts feeds to Mastodon, Bluesky and WordPress; can send `source:markdown` instead of HTML |
| FreshRSS | Self-hosted reader | Support requested in [issue #8191](https://github.com/FreshRSS/FreshRSS/issues/8191), not confirmed as shipped |

---

## 4. Sites with `.md` versions of their pages

Add `.md` to a page URL (or follow the "Markdown" link on the page).

| Site | Example | Notes |
|------|---------|-------|
| **Vercel** blog & changelog | `vercel.com/blog/<post>.md` | Also has Markdown sitemaps: `vercel.com/blog/sitemap.md`, `vercel.com/changelog/sitemap.md` |
| **Stripe** docs | `docs.stripe.com/<page>.md` | Example: `docs.stripe.com/stripe-apps/pkce-oauth-flow.md` |
| **Claude Code** docs | `code.claude.com/docs/en/memory.md` | Index at `code.claude.com/docs/llms.txt` |
| **Cloudflare** developer docs | `developers.cloudflare.com/<path>/index.md` | Also answers `Accept: text/markdown` |
| **GitBook** docs | `gitbook.com/docs/<page>.md` | All GitBook-hosted docs offer this |
| **Google Search Central** docs | Download menu on each page | Files are `.md.txt`; Google says they aren't used for Search |
| **GeekNews** (Korean tech news) | `news.hada.io/topic/21957.md` | Every story is available as Markdown |
| **Flavio Copes** | `flaviocopes.com/npm.md` | Every post has a `.md` version |
| **HackMD** | `hackmd.io/@user/note.md` | Notes are written and stored in Markdown |
| **Any Discourse forum** | `<forum>/t/<topic>.md` | Topics and topic lists, on by default for hosted forums ([announcement](https://blog.discourse.org/2026/10/markdown-endpoints-your-community-in-a-format-that-ai-agents-can-read/)) |
| **Any WordPress site** with the Roots plugin | `/<post-slug>.md` | See section 3 |

---

## 5. Sites that answer `Accept: text/markdown`

Same URL as the web page; ask for Markdown in the request header:

```bash
curl -H "Accept: text/markdown" https://blog.cloudflare.com/markdown-for-agents/
```

| Site | What's covered |
|------|----------------|
| **Cloudflare Blog** – `blog.cloudflare.com` | All posts |
| **Cloudflare Developer Docs** – `developers.cloudflare.com` | All docs; the response includes an `x-markdown-tokens` header with an estimated token count |
| **Vercel** – `vercel.com/docs`, `/blog`, `/changelog` | Docs, blog and changelog; Markdown responses end with a `sitemap.md` link |
| **HackMD** – `hackmd.io` | Notes |
| **Mintlify-hosted docs** | Every page (also `Accept: text/plain`) |
| **Discourse forums** | Topics and topic lists |
| **roots.io** | Reference implementation used by acceptmarkdown.com |

---

## 6. Platforms that make every site Markdown-ready

If a site is built on one of these platforms, its pages are probably available as Markdown even if the site doesn't advertise it.

| Platform | Markdown support |
|----------|------------------|
| **Cloudflare – Markdown for Agents** | Converts HTML to Markdown at the edge for `Accept: text/markdown` requests. Beta, free on Pro, Business and Enterprise plans. The site owner has to turn it on |
| **Mintlify** (docs host) | `.md` on every page, `Accept: text/markdown`, and automatic Markdown for known AI agents |
| **GitBook** (docs host) | `.md` on every page, plus `llms.txt` |
| **Discourse** (forums) | `.md` and `Accept: text/markdown` on every topic |
| **Micro.blog** (blogs) | `source:markdown` in every RSS feed |
| **WordLand** (WordPress front-end) | `source:markdown` in its shadow feeds |
| **Hugo** (static sites) | Can output a Markdown version of every page through output formats |

---

## 7. `llms.txt` directories

An `llms.txt` file is itself Markdown: an H1 with the site name, a blockquote summary, then H2 sections of links. Good `llms.txt` files link straight to `.md` pages. These directories list sites that have one:

| Directory | Notes |
|-----------|-------|
| [directory.llmstxt.cloud](https://directory.llmstxt.cloud/) | Widely cited directory with thousands of entries |
| [llms.txt hub](https://llms-txt-hub.vercel.app/) (llmstxthub.com) | About 1,240 sites, grouped by category. It's open to anyone, so expect some spam listings |
| [llmstxt.site](https://github.com/krish-adi/llmstxt-site) | Community directory with token counts for each file |
| llmtxt.app | Says it lists 1,300+ sites |
| [BuiltWith: LLMS Text trends](https://trends.builtwith.com/robots/LLMS-Text) | Usage statistics |
| [llmstxt.org](https://llmstxt.org) | The proposal itself |

---

## 8. How to search for more

### Google

Google's own documentation says `filetype:` matches **either the HTTP `Content-Type` header or the file extension**, so in principle it can find files served as `text/markdown`. In practice `filetype:` has been unreliable since 2024 and works best combined with `site:` or `inurl:`. These queries are worth trying (not yet tested):

```text
filetype:md site:github.com <topic>
filetype:md -site:github.com <topic>        # the "-site:" trick re-enables filetype:
inurl:llms.txt <topic>
inurl:"llms-full.txt"
inurl:sitemap.md
"source:markdown" inurl:rss
"source:markdown" inurl:feed
"xmlns:source" "source.scripting.com"
```

### GitHub code search (needs a GitHub login)

```text
"source:markdown" path:*.xml              # feeds committed to repos
"xmlns:source" path:*.xml
"rel=\"alternate\" type=\"text/markdown\""  # pages advertising a Markdown twin
filename:llms.txt
```

### Discovery tags to look for in a page's HTML

```html
<link rel="alternate" type="text/markdown" href="/page.md">
<atom:link href="/feed/markdown/" rel="alternate" type="text/markdown"/>
```

---

## 9. Checkers and test commands

| Tool | What it does |
|------|--------------|
| [acceptmarkdown.com](https://acceptmarkdown.com) | Tests whether any URL serves Markdown (`Accept`, `Vary`, `406`) and tracks which AI agents send the header |
| [Fern Agent Score](https://buildwithfern.com/agent-score) | Scores documentation sites on `llms.txt`, `.md` URLs and content negotiation |
| Mintlify score | Similar scorecard; results are published as `.md` pages |

**Check whether a URL serves Markdown:**

```bash
# Content negotiation
curl -sI -H "Accept: text/markdown" https://vercel.com/docs | grep -i content-type

# .md suffix
curl -sI https://docs.stripe.com/stripe-apps/pkce-oauth-flow.md | grep -i content-type

# Does a feed include source:markdown?
curl -s https://www.manton.org/feed.xml | grep -c "<source:markdown>"
```

---

## Sources

- [Manton Reece – Markdown in RSS](https://www.manton.org/2025/11/11/markdown-in-rss.html)
- [Scripting News – Dev notes for Markdown in RSS](http://scripting.com/2022/07/19/152235.html?title=devNotesForMarkdownInRss)
- [Scripting News issue #336 – source:markdown in the Scripting News feed](https://github.com/scripting/Scripting-News/issues/336)
- [WordLand shadow feeds](https://github.com/scripting/wordlandDev/issues/3) · [daveverse feed sample](https://gist.github.com/scripting/deca6d866b76d566b05673c94e8610de)
- [NetNewsWire 6.2 release notes](https://netnewswire.blog/2025/11/05/netnewswire-for-mac-and-ios.html)
- [Cloudflare – Markdown for Agents docs](https://developers.cloudflare.com/fundamentals/reference/markdown-for-agents)
- [Vercel – Making agent-friendly pages with content negotiation](https://vercel.com/blog/making-agent-friendly-pages-with-content-negotiation) · [Vercel – Markdown access](https://vercel.com/docs/agent-resources/markdown-access)
- [Mintlify – Markdown export](https://mintlify.com/docs/ai/markdown-export)
- [Discourse – Markdown endpoints](https://blog.discourse.org/2026/10/markdown-endpoints-your-community-in-a-format-that-ai-agents-can-read/)
- [HackMD – Markdown-native content negotiation](https://hackmd.io/blog/2026/03/22/agentic-workflow-2026)
- [Roots – Serve your WordPress posts as Markdown](https://roots.io/serve-your-wordpress-posts-as-markdown.md)
- [Website Spec – Markdown source endpoints](https://specification.website/spec/agent-readiness/markdown-source-endpoints/)
- [Google Search Central – Search operators](https://developers.google.com/search/docs/monitor-debug/search-operators)
- [Search Engine Journal – filetype: operator issue](https://searchenginejournal.com/filetype-search-operator-missing/509656) · [Boolean Strings – filetype: workaround](https://booleanstrings.com/2024/02/27/googles-filetype-decline/)
- [Search Engine Roundtable – Google adds Markdown files to help docs](https://www.seroundtable.com/google-adds-markdown-files-to-help-docs-41342.html)
