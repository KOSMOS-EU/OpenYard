#!/bin/bash
set -euo pipefail

# skeleton_sample.sh — Erstellt eine Aktenplan-Teststruktur in einem OpenCloud Space
#
# Usage: ./skeleton_sample.sh <space-root-path>
# Example: ./skeleton_sample.sh /data/data/storage/users/projects/f9f939a9-.../

SPACE="${1:?Usage: $0 <space-root-path>}"
OWNER="adminuser:adminuser"

[ -d "$SPACE/.space" ] || { echo "ERROR: $SPACE is not a valid space root"; exit 1; }

echo "=== Creating Aktenplan skeleton in $SPACE ==="

# --- Space-Root Typ-Marker ---
touch "$SPACE/.type_aktenplan"

# --- Schema-Dateien ---
mkdir -p "$SPACE/.space/views"

cat > "$SPACE/.space/views/aktenplan.json" << 'EOF'
{
  "label": "Aktenplan",
  "icon": "archive",
  "children": {
    "protected": ["aktenplan"],
    "shielded": ["akte"],
    "default": ["aktenplan", "akte"]
  },
  "columns": ["name", "oy.fileReference", "typ", "anzahl"],
  "fileReferencePattern": "{parentRef}.{seq:2}",
  "metadata": {
    "oy.fileReference": { "label": "Aktenzeichen", "type": "string", "auto": true }
  }
}
EOF

cat > "$SPACE/.space/views/akte.json" << 'EOF'
{
  "label": "Akte",
  "icon": "folder-open",
  "children": ["vorgang"],
  "columns": ["name", "oy.fileReference", "oy.status", "abgelegt-von", "abgelegt-am"],
  "fileReferencePattern": "{parentRef}-{seq:2}",
  "metadata": {
    "oy.fileReference": { "label": "Aktenzeichen", "type": "string", "auto": true },
    "oy.status": { "label": "Status", "type": "enum", "values": ["offen", "gespeichert", "geschlossen"], "default": "offen" }
  }
}
EOF

cat > "$SPACE/.space/views/vorgang.json" << 'EOF'
{
  "label": "Vorgang",
  "icon": "file-list",
  "children": ["register"],
  "columns": ["name", "oy.fileReference", "oy.version", "abgelegt-von", "abgelegt-am"],
  "fileReferencePattern": "{parentRef}/{seq}",
  "metadata": {
    "oy.fileReference": { "label": "Aktenzeichen", "type": "string", "auto": true },
    "oy.version": { "label": "Version", "type": "string" }
  }
}
EOF

cat > "$SPACE/.space/views/register.json" << 'EOF'
{
  "label": "Register",
  "icon": "bookmark",
  "children": [],
  "columns": ["name", "oy.fileReference", "abgelegt-von", "abgelegt-am"],
  "fileReferencePattern": "{parentRef}#{seq}",
  "metadata": {
    "oy.fileReference": { "label": "Aktenzeichen", "type": "string", "auto": true }
  }
}
EOF

echo "[ok] Schema-Dateien angelegt"

# --- Ordnerstruktur ---

mk() {
  local path="$1" type="$2" ref="$3" immutable="${4:-}"
  mkdir -p "$path"
  touch "$path/.type_$type"
  setfattr -n user.oc.md.oy.fileReference -v "$ref" "$path"
  [ "$immutable" = "protected" ] && setfattr -n user.oc.immutable -v 1 "$path"
  chown -R "$OWNER" "$path"
  chmod 700 "$path"
  echo "  $type  $ref  ${immutable:+[$immutable]}  $path"
}

echo ""
echo "[creating] Sachgruppen (Aktenplan, protected)..."
mk "$SPACE/Innere Verwaltung"                                                             aktenplan "11"          protected
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung"                                          aktenplan "11.12"       protected
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Organisationsangelegenheiten"              aktenplan "11.12.01"    protected

echo ""
echo "[creating] Aktenschrank (letztes Blatt, shielded durch Parent)..."
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Organisationsangelegenheiten/Satzungen"    aktenplan "11.12.01.03"

echo ""
echo "[creating] Akten..."
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Organisationsangelegenheiten/Satzungen/Entschädigungssatzung"  akte "11.12.01.03-01"
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Organisationsangelegenheiten/Satzungen/Feuerwehrsatzung"       akte "11.12.01.03-02"

echo ""
echo "[creating] Vorgänge..."
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Organisationsangelegenheiten/Satzungen/Entschädigungssatzung/Fassung 2016"  vorgang "11.12.01.03-01/1"
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Organisationsangelegenheiten/Satzungen/Entschädigungssatzung/Fassung 2026"  vorgang "11.12.01.03-01/2"

echo ""
echo "[creating] Register..."
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Organisationsangelegenheiten/Satzungen/Entschädigungssatzung/Fassung 2016/Vorlagen und Vergleiche"  register "11.12.01.03-01/1#1"
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Organisationsangelegenheiten/Satzungen/Entschädigungssatzung/Fassung 2016/Beanstandungen"           register "11.12.01.03-01/1#2"

echo ""
echo "[creating] Weitere Sachgruppen..."
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Organisationsangelegenheiten/Landratsamt"  aktenplan "11.12.01.05"
mk "$SPACE/Innere Verwaltung/Kommunalverwaltung/Personalangelegenheiten"                   aktenplan "11.12.02"    protected
mk "$SPACE/Innere Verwaltung/Finanzverwaltung"                                             aktenplan "11.13"       protected
mk "$SPACE/Sicherheit und Ordnung"                                                         aktenplan "12"          protected
mk "$SPACE/Schulträgeraufgaben"                                                            aktenplan "21"          protected

# Space-Root Marker Rechte
chown "$OWNER" "$SPACE/.type_aktenplan" "$SPACE/.space/views/"*
chmod 700 "$SPACE/.space/views"

echo ""
echo "=== Aktenplan skeleton created ==="
echo ""
echo "Structure:"
echo "  77 Aktenplan/                           .type_aktenplan"
echo "  ├── Innere Verwaltung/                  protected  11"
echo "  │   └── Kommunalverwaltung/             protected  11.12"
echo "  │       └── Organisationsangel./        protected  11.12.01"
echo "  │           ├── Satzungen/              shielded   11.12.01.03  (Aktenschrank)"
echo "  │           │   ├── Entschädigungssat./ .type_akte 11.12.01.03-01"
echo "  │           │   │   ├── Fassung 2016/   .type_vorgang  11.12.01.03-01/1"
echo "  │           │   │   │   ├── Vorlagen/   .type_register 11.12.01.03-01/1#1"
echo "  │           │   │   │   └── Beanstand./ .type_register 11.12.01.03-01/1#2"
echo "  │           │   │   └── Fassung 2026/   .type_vorgang  11.12.01.03-01/2"
echo "  │           │   └── Feuerwehrsatzung/   .type_akte 11.12.01.03-02"
echo "  │           └── Landratsamt/            shielded   11.12.01.05"
echo "  ├── Sicherheit und Ordnung/             protected  12"
echo "  └── Schulträgeraufgaben/                protected  21"
