---
title: Cookbook
description: Task-focused recipes for common goway scenarios.
---

The recipes in this section walk through solutions to common migration tasks. Pick the one that matches your use case and adapt it to your needs.

- **[Embedding Migrations](/goway/cookbook/embedding-migrations/)** — Package SQL scripts into your binary using `go:embed` and the `FS` method.
- **[Programmatic Setup](/goway/cookbook/programmatic-setup/)** — Initialize and run migrations from Go code with custom callbacks and configurations.
- **[Non-Transactional Migrations](/goway/cookbook/non-transactional-migrations/)** — Run operations like `CREATE INDEX CONCURRENTLY` outside a transaction.
- **[Baselining an Existing Database](/goway/cookbook/baseline-existing-database/)** — Bring an existing schema under version control by establishing a baseline version.
