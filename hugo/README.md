# Hugo Integration Resources

This directory contains integration templates, sample data, and a reference blog for installing the Spiritriot comment system on a Hugo site.

## Directory Structure

* **`layouts/partials/`**: Partial templates to place in your Hugo layouts directory.
  * **`spiritriotcomment.html`**: The HTML container and script hooks for loading the dynamic JavaScript comment widget and submit forms.
  * **`spiritriotarchives.html`**: A static comment archive generator. It reads build-time comment data files from `data/comments/` and renders archived comments that fall outside the date range of the interactive "new comment" system.
* **`data/comments/`**: Target directory for build-time static comment files.
  * **`test.yaml`**: A sample YAML comment data structure showing how comments map to pages. YAML files like these are generated using the tools in `tools/comment-importer`.
* **`sample-blog/`**: A lightweight Hugo sample site configured with these templates for local testing and integration development.

---

## Hugo Integration Guide

### 1. Add layouts and styling links
Copy the files in `layouts/partials/` into your Hugo site's `layouts/partials/` folder. Add a reference to render the comments container in your single post template (e.g. `layouts/_default/single.html`):

```html
{{ partial "spiritriotcomment.html" . }}
```

Include a reference to the compiled stylesheet in your head layout template:

```html
<link rel="stylesheet" href="https://assets.yourdomain.com/css/comments.css">
```

### 2. Configure build parameters
In your site's `hugo.toml`, define the comment URLs and endpoints:

```toml
[params]
  commentUrl = "https://comments.yourdomain.com"
```

### 3. Build-Time Static Archives
When generating static pages, ensure your comment JSON/YAML backups are downloaded and saved under `data/comments/` so that `spiritriotarchives.html` can render them dynamically during Hugo build execution.
