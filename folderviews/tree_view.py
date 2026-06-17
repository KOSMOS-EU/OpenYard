#!/usr/bin/env python3
"""Zeigt eine YAML-Ordnerstruktur als Baum an.

Usage:
    python3 tree_view.py trees/Sicherheit_und_Ordnung.yaml
    python3 tree_view.py trees/*.yaml
    python3 tree_view.py trees/*.yaml --summary
"""
import sys
import yaml


def print_tree(node, prefix="", is_last=True):
    connector = "└── " if is_last else "├── "
    docs = f" ({node.get('docs', 0)} docs)" if node.get('docs', 0) > 0 else ""
    print(f"{prefix}{connector}{node['name']}{docs}")
    children = node.get("folders", [])
    for i, child in enumerate(children):
        extension = "    " if is_last else "│   "
        print_tree(child, prefix + extension, i == len(children) - 1)


def main():
    summary = "--summary" in sys.argv
    files = [f for f in sys.argv[1:] if not f.startswith("--")]

    if not files:
        print("Usage: tree_view.py <yaml-file> [--summary]")
        sys.exit(1)

    for path in sorted(files):
        with open(path) as f:
            data = yaml.safe_load(f)
        vol = data.get("volume", "?")
        folders = data.get("total_folders", 0)
        docs = data.get("total_docs", 0)

        if summary:
            print(f"  {folders:>5} Ordner  {docs:>6} Docs  {vol}")
        else:
            print(f"\n{vol} — {folders} Ordner, {docs} Dokumente\n")
            print_tree(data["tree"])


if __name__ == "__main__":
    main()
