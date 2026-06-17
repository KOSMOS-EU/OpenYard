#!/bin/bash
# Generiert eine hierarchische YAML aus dem Klassifikations-Skelett
# Input: aktenplan_skelett.csv (35-Spalten-CSV mit ^-Trenner)
# Output: brandis-aktenplan.yaml

set -euo pipefail

INPUT="${1:-aktenplan_skelett.csv}"

# Hauptgruppen-Bezeichnungen nach sächsischem Standard-Aktenplan
declare -A HAUPTGRUPPEN=(
  ["11"]="Innere Verwaltung"
  ["12"]="Sicherheit und Ordnung"
  ["21"]="Schule und Bildung"
  ["24"]="Erwachsenenbildung"
  ["27"]="Wissenschaft, Forschung"
  ["28"]="Kultur"
  ["31"]="Soziale Leistungen"
  ["33"]="Familien- und Sozialhilfe"
  ["35"]="Senioren"
  ["36"]="Kinder- und Jugendarbeit"
  ["42"]="Sportförderung"
  ["51"]="Bauleitplanung, Stadtentwicklung"
  ["52"]="Bauordnung, Wohnungswesen"
  ["53"]="Ver- und Entsorgung"
  ["54"]="Verkehrswesen"
  ["55"]="Öffentliches Grün, Umwelt"
  ["57"]="Wirtschaftsförderung"
  ["61"]="Finanzwirtschaft, Steuern"
  ["71"]="Brand- und Katastrophenschutz"
  ["81"]="Personalwirtschaft"
)

# Header
cat <<HEADER
# Brandis-Aktenplan (generiert aus WinYard-Strukturexport)
# Basis: Sächsischer Standard-Aktenplan
# Generiert: $(date +%Y-%m-%d)
#
# Struktur:
#   knoten:
#     - kennung: "XX"             # Hauptgruppe
#       name: "Bezeichnung"
#       kinder:
#         - kennung: "XX.YY"      # Gruppe
#           ...

aktenplan:
  version: "1.0"
  kommune: "Brandis"
  quelle: "WinYard-Export"
HEADER

# Knoten aus CSV extrahieren und sortieren
awk -F'^' '
  NF == 35 && $10 != "NULL" && $10 ~ /^[0-9]{2}\.[0-9]{2}\.[0-9]{2}\.[0-9]{2}$/ {
    name = $8
    gsub(/"+/, "", name)
    print $10 "\t" name
  }
' "$INPUT" | sort -t. -k1,1n -k2,2n -k3,3n -k4,4n -u > /tmp/aktenplan_sorted.tsv

# YAML hierarchisch aufbauen
echo "  knoten:"

current_hg=""
current_gr=""
current_ug=""

while IFS=$'\t' read -r kennung name; do
  IFS='.' read -r hg gr ug sg <<< "$kennung"
  
  # Hauptgruppe wechseln?
  if [[ "$hg" != "$current_hg" ]]; then
    hg_name="${HAUPTGRUPPEN[$hg]:-Hauptgruppe $hg}"
    echo "    - kennung: \"$hg\""
    echo "      name: \"$hg_name\""
    echo "      kinder:"
    current_hg="$hg"
    current_gr=""
    current_ug=""
  fi
  
  # Gruppe wechseln?
  if [[ "$gr" != "$current_gr" ]]; then
    echo "        - kennung: \"$hg.$gr\""
    echo "          kinder:"
    current_gr="$gr"
    current_ug=""
  fi
  
  # Untergruppe wechseln?
  if [[ "$ug" != "$current_ug" ]]; then
    echo "            - kennung: \"$hg.$gr.$ug\""
    echo "              kinder:"
    current_ug="$ug"
  fi
  
  # Sachgruppe als Blattknoten
  # Name escapen
  escaped=$(echo "$name" | sed 's/"/\\"/g')
  echo "                - kennung: \"$kennung\""
  echo "                  name: \"$escaped\""
done < /tmp/aktenplan_sorted.tsv

rm -f /tmp/aktenplan_sorted.tsv
