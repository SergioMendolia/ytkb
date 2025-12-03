# YouTrack Writer

A CLI application for syncing YouTrack knowledge base articles to local markdown files, edit them in your editor of choice and push them back online.

## Features

- **Download**: Download all pages from a YouTrack knowledge base, preserving hierarchy and order
- **Diff**: Compare local files with server versions to see what's changed
- **Push**: Push changes back to YouTrack (update only, no creation)
- **Safety**: Page deletion must be done manually in YouTrack - the app will warn you with links

## Installation

Build the project:

```bash
go build -o youtrack_writer
```

Or install:

```bash
go install
```

## Configuration

### Global Configuration

Create a configuration file at `~/.config/youtrack_writer/config.ini` or let the app guide you on first run

```ini
[config]
token=your_api_token_here
url=https://your-youtrack-instance.com
```

If the file doesn't exist, the app will prompt you to create it interactively.

### Project Configuration

Create a `.env` file in your working directory:

```
KB_KEY=your_knowledge_base_key
```

If the `.env` file doesn't exist, the app will prompt you to select a knowledge base.

## Usage

### Download

Download all articles from the knowledge base:

```bash
youtrack_writer download
```

This creates a nested directory structure matching the YouTrack hierarchy, with each article saved as a markdown file with YAML frontmatter.

### Diff

Compare local files with the server:

```bash
youtrack_writer diff
```

### Push

Push changes to YouTrack:

```bash
# Push a specific page
youtrack_writer push path/to/article.md

# Push all changes
youtrack_writer push
```

**Note**: The app will not delete pages. If a page is deleted locally, you'll see a warning with a link to delete it manually in YouTrack.

## File Format

Each markdown file includes YAML frontmatter:

```yaml
---
id: article-id
title: Article Title
order: 0
url: https://youtrack-instance.com/article-url
---
```

The article content follows the frontmatter. Parent relationships are inferred from the directory structure.

## License

MIT
