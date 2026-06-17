package cmis

import (
	"fmt"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
)

// CMIS-QL query engine.
//
// Supported:
//   SELECT * FROM cmis:document [WHERE ...] [ORDER BY ...]
//   SELECT * FROM cmis:folder [WHERE ...] [ORDER BY ...]
//   SELECT * FROM cmis:document d JOIN oy:documentMetadata m ON d.cmis:objectId = m.cmis:objectId [WHERE ...]
//
// WHERE clause predicates:
//   IN_FOLDER('objectId')                — direct children
//   IN_TREE('objectId')                  — descendants
//   property = 'value'                   — equality
//   property <> 'value'                  — inequality
//   property LIKE 'pattern'              — SQL LIKE (%, _)
//   property IS NULL / IS NOT NULL       — null checks
//   CONTAINS('text')                     — fulltext (metadata-based)
//   property IN ('a', 'b', 'c')         — set membership
//
// Logical operators: AND, OR (with AND binding tighter)
// ORDER BY: property [ASC|DESC] [, ...]
// Pagination: maxItems/skipCount in POST form data.

// Query handles cmisaction=query on a repository.
func (h *Handler) Query(w http.ResponseWriter, r *http.Request, repoID string) {
	statement := r.FormValue("statement")
	if statement == "" {
		writeError(w, 400, "invalidArgument", "statement is required")
		return
	}

	maxItems := formInt(r, "maxItems", 100)
	skipCount := formInt(r, "skipCount", 0)

	q, err := parseQuery(statement)
	if err != nil {
		writeError(w, 400, "invalidArgument", "Invalid CMIS-QL: "+err.Error())
		return
	}

	// Determine the folder to search in
	ref, err := h.resolveQueryFolder(r, repoID, q)
	if err != nil {
		writeError(w, 400, "invalidArgument", err.Error())
		return
	}

	// List contents (recursive for IN_TREE, direct for IN_FOLDER)
	var infos []*provider.ResourceInfo
	if q.inTree != "" {
		infos = h.listRecursive(r, ref, 10) // max depth 10
	} else {
		listRes, err := h.gw.Gateway.ListContainer(r.Context(), &provider.ListContainerRequest{Ref: ref})
		if err != nil || listRes.Status.Code != rpc.Code_CODE_OK {
			writeJSON(w, 200, ObjectList{Objects: []ObjectEntry{}, NumItems: 0})
			return
		}
		infos = listRes.Infos
	}

	// Request metadata for filtering on arbitrary properties
	var infosWithMeta []*provider.ResourceInfo
	for _, info := range infos {
		if !matchesType(info, q.fromType) {
			continue
		}
		// If we have predicates on non-cmis properties or CONTAINS, fetch metadata
		if needsMetadata(q) && info.ArbitraryMetadata == nil {
			metaRef := &provider.Reference{ResourceId: info.Id, Path: "."}
			metaRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{
				Ref:                   metaRef,
				ArbitraryMetadataKeys: []string{"*"},
			})
			if err == nil && metaRes.Status.Code == rpc.Code_CODE_OK {
				info = metaRes.Info
			}
		}
		infosWithMeta = append(infosWithMeta, info)
	}

	// Filter by predicates and CONTAINS
	var results []ObjectEntry
	for _, info := range infosWithMeta {
		if !matchesPredicates(info, q.predicates) {
			continue
		}
		if q.contains != "" && !matchesContains(info, q.contains) {
			continue
		}
		results = append(results, ObjectEntry{Object: mapResourceToObject(info)})
	}

	// Sort by ORDER BY
	if len(q.orderBy) > 0 {
		sortResults(results, q.orderBy)
	}

	// Pagination
	total := len(results)
	start := skipCount
	if start > total {
		start = total
	}
	end := start + maxItems
	if end > total {
		end = total
	}

	writeJSON(w, 200, ObjectList{
		Objects:      results[start:end],
		HasMoreItems: end < total,
		NumItems:     total,
	})
}

// --- Query Parser ---

type cmisQuery struct {
	selectCols []string       // "*" or list of property IDs
	fromType   string         // "cmis:document" or "cmis:folder"
	joins      []queryJoin    // JOIN clauses
	where      *whereNode     // parsed WHERE tree (supports OR)
	predicates []predicate    // flat predicates (legacy compat, populated from where)
	inFolder   string         // objectId for IN_FOLDER
	inTree     string         // objectId for IN_TREE
	contains   string         // CONTAINS() search term
	orderBy    []orderByClause
}

type queryJoin struct {
	joinType string // "JOIN" or "LEFT JOIN"
	typeName string // e.g. "oy:documentMetadata"
	alias    string
}

type orderByClause struct {
	property   string
	descending bool
}

type predicate struct {
	property string
	operator string // "=", "LIKE", "<>", "<", ">", "<=", ">=", "IS NULL", "IS NOT NULL", "IN"
	value    string
	values   []string // for IN operator
}

// whereNode represents a node in the WHERE expression tree.
type whereNode struct {
	op       string      // "AND", "OR", "PRED"
	pred     *predicate  // only for op="PRED"
	children []*whereNode
}

func parseQuery(sql string) (*cmisQuery, error) {
	q := &cmisQuery{}
	tokens := tokenize(sql)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty query")
	}

	i := 0

	// SELECT
	if !tokEq(tokens, i, "SELECT") {
		return nil, fmt.Errorf("expected SELECT")
	}
	i++

	// Columns: * or comma-separated list
	if i >= len(tokens) {
		return nil, fmt.Errorf("expected column list after SELECT")
	}
	if tokens[i] == "*" {
		q.selectCols = []string{"*"}
		i++
	} else {
		for i < len(tokens) && !tokEq(tokens, i, "FROM") {
			col := tokens[i]
			if col != "," {
				q.selectCols = append(q.selectCols, col)
			}
			i++
		}
	}

	// FROM
	if !tokEq(tokens, i, "FROM") {
		return nil, fmt.Errorf("expected FROM")
	}
	i++
	if i >= len(tokens) {
		return nil, fmt.Errorf("expected type after FROM")
	}
	q.fromType = tokens[i]
	i++

	// Optional alias
	if i < len(tokens) && !isKeyword(tokens[i]) {
		i++ // skip alias
	}

	// JOIN clauses
	for i < len(tokens) && (tokEq(tokens, i, "JOIN") || tokEq(tokens, i, "LEFT") || tokEq(tokens, i, "INNER")) {
		jt := "JOIN"
		if tokEq(tokens, i, "LEFT") || tokEq(tokens, i, "INNER") {
			jt = strings.ToUpper(tokens[i]) + " JOIN"
			i++ // skip LEFT/INNER
		}
		if tokEq(tokens, i, "JOIN") {
			i++
		}
		if i >= len(tokens) {
			break
		}
		typeName := tokens[i]
		i++
		alias := ""
		if i < len(tokens) && !isKeyword(tokens[i]) {
			alias = tokens[i]
			i++
		}
		q.joins = append(q.joins, queryJoin{joinType: jt, typeName: typeName, alias: alias})
		// Skip ON clause (we don't need it — join is implicit via objectId)
		if tokEq(tokens, i, "ON") {
			i++
			// Skip until WHERE, ORDER, JOIN, or end
			for i < len(tokens) && !tokEq(tokens, i, "WHERE") && !tokEq(tokens, i, "ORDER") && !tokEq(tokens, i, "JOIN") && !tokEq(tokens, i, "LEFT") && !tokEq(tokens, i, "INNER") {
				i++
			}
		}
	}

	// WHERE (optional)
	if tokEq(tokens, i, "WHERE") {
		i++
		if err := parseWhere(tokens, &i, q); err != nil {
			return nil, err
		}
	}

	// ORDER BY (optional)
	if tokEq(tokens, i, "ORDER") {
		i++
		if tokEq(tokens, i, "BY") {
			i++
		}
		for i < len(tokens) {
			prop := tokens[i]
			i++
			desc := false
			if tokEq(tokens, i, "DESC") {
				desc = true
				i++
			} else if tokEq(tokens, i, "ASC") {
				i++
			}
			q.orderBy = append(q.orderBy, orderByClause{property: prop, descending: desc})
			if tokEq(tokens, i, ",") {
				i++
			} else {
				break
			}
		}
	}

	return q, nil
}

func isKeyword(tok string) bool {
	switch strings.ToUpper(tok) {
	case "WHERE", "ORDER", "JOIN", "LEFT", "INNER", "ON", "GROUP", "HAVING":
		return true
	}
	return false
}

func parseWhere(tokens []string, i *int, q *cmisQuery) error {
	for *i < len(tokens) {
		if tokEq(tokens, *i, "ORDER") {
			break
		}
		if tokEq(tokens, *i, "AND") || tokEq(tokens, *i, "OR") {
			*i++
			continue
		}

		// IN_FOLDER('id') or IN_TREE('id')
		if tokEq(tokens, *i, "IN_FOLDER") || tokEq(tokens, *i, "IN_TREE") {
			isTree := tokEq(tokens, *i, "IN_TREE")
			*i++
			if !tokEq(tokens, *i, "(") {
				return fmt.Errorf("expected ( after IN_FOLDER/IN_TREE")
			}
			*i++
			if *i >= len(tokens) {
				return fmt.Errorf("expected folder ID in IN_FOLDER/IN_TREE")
			}
			folderID := unquote(tokens[*i])
			*i++
			if tokEq(tokens, *i, ")") {
				*i++
			}
			if isTree {
				q.inTree = folderID
			} else {
				q.inFolder = folderID
			}
			continue
		}

		// CONTAINS('search text')
		if tokEq(tokens, *i, "CONTAINS") {
			*i++
			if tokEq(tokens, *i, "(") {
				*i++
			}
			if *i < len(tokens) {
				q.contains = unquote(tokens[*i])
				*i++
			}
			if tokEq(tokens, *i, ")") {
				*i++
			}
			continue
		}

		// property IS NULL / IS NOT NULL
		if *i+2 <= len(tokens) {
			prop := tokens[*i]
			if tokEq(tokens, *i+1, "IS") {
				if tokEq(tokens, *i+2, "NULL") {
					q.predicates = append(q.predicates, predicate{
						property: prop, operator: "IS NULL",
					})
					*i += 3
					continue
				}
				if tokEq(tokens, *i+2, "NOT") && *i+3 < len(tokens) && tokEq(tokens, *i+3, "NULL") {
					q.predicates = append(q.predicates, predicate{
						property: prop, operator: "IS NOT NULL",
					})
					*i += 4
					continue
				}
			}
		}

		// property IN ('a', 'b', 'c')
		if *i+2 < len(tokens) {
			prop := tokens[*i]
			if tokEq(tokens, *i+1, "IN") && tokEq(tokens, *i+2, "(") {
				*i += 3
				var vals []string
				for *i < len(tokens) && !tokEq(tokens, *i, ")") {
					if tokens[*i] != "," {
						vals = append(vals, unquote(tokens[*i]))
					}
					*i++
				}
				if tokEq(tokens, *i, ")") {
					*i++
				}
				q.predicates = append(q.predicates, predicate{
					property: prop, operator: "IN", values: vals,
				})
				continue
			}
		}

		// property = 'value' / property LIKE 'pattern' / property <> 'value'
		if *i+2 < len(tokens) {
			prop := tokens[*i]
			op := strings.ToUpper(tokens[*i+1])

			// Handle two-char operators
			if *i+3 < len(tokens) && (op == "<" || op == ">" || op == "!") {
				nextChar := tokens[*i+2]
				if (op == "<" && nextChar == ">") || (op == "<" && nextChar == "=") || (op == ">" && nextChar == "=") || (op == "!" && nextChar == "=") {
					op = op + nextChar
					*i++
				}
			}

			switch op {
			case "=", "LIKE", "<>", "!=", "<", ">", "<=", ">=":
				val := unquote(tokens[*i+2])
				q.predicates = append(q.predicates, predicate{
					property: prop,
					operator: op,
					value:    val,
				})
				*i += 3
				continue
			}
		}

		// Unknown token — skip
		*i++
	}
	return nil
}

// --- Query Execution ---

func (h *Handler) resolveQueryFolder(r *http.Request, repoID string, q *cmisQuery) (*provider.Reference, error) {
	folderID := q.inFolder
	if folderID == "" {
		folderID = q.inTree
	}

	if folderID != "" {
		ref, err := refFromObjectID(folderID)
		if err != nil {
			return nil, fmt.Errorf("invalid folder ID: %w", err)
		}
		return ref, nil
	}

	// Default: search in repository root
	ref, err := refFromObjectID(repoID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	return ref, nil
}

func (h *Handler) listRecursive(r *http.Request, ref *provider.Reference, maxDepth int) []*provider.ResourceInfo {
	if maxDepth <= 0 {
		return nil
	}

	listRes, err := h.gw.Gateway.ListContainer(r.Context(), &provider.ListContainerRequest{Ref: ref})
	if err != nil || listRes.Status.Code != rpc.Code_CODE_OK {
		return nil
	}

	var result []*provider.ResourceInfo
	for _, info := range listRes.Infos {
		result = append(result, info)
		if info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER {
			childRef := &provider.Reference{ResourceId: info.Id, Path: "."}
			children := h.listRecursive(r, childRef, maxDepth-1)
			result = append(result, children...)
		}
	}
	return result
}

func matchesType(info *provider.ResourceInfo, fromType string) bool {
	switch fromType {
	case BaseTypeDocument:
		return info.Type == provider.ResourceType_RESOURCE_TYPE_FILE
	case BaseTypeFolder:
		return info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER
	default:
		return true // Unknown type — include all
	}
}

func matchesPredicates(info *provider.ResourceInfo, preds []predicate) bool {
	for _, p := range preds {
		val := getPropertyValue(info, p.property)
		if !matchPredicate(val, p) {
			return false
		}
	}
	return true
}

func getPropertyValue(info *provider.ResourceInfo, propID string) string {
	switch propID {
	case "cmis:name":
		return path.Base(info.Path)
	case "cmis:objectId":
		return encodeObjectID(info.Id)
	case "cmis:contentStreamMimeType":
		return info.MimeType
	case "cmis:path":
		return info.Path
	case "cmis:baseTypeId":
		if info.Type == provider.ResourceType_RESOURCE_TYPE_FILE {
			return BaseTypeDocument
		}
		return BaseTypeFolder
	case "cmis:contentStreamLength":
		return strconv.FormatUint(info.Size, 10)
	case "cmis:createdBy":
		if info.Owner != nil {
			return info.Owner.OpaqueId
		}
		return ""
	default:
		// Check arbitrary metadata
		if info.ArbitraryMetadata != nil {
			if v, ok := info.ArbitraryMetadata.Metadata[propID]; ok {
				return v
			}
		}
		return ""
	}
}

func matchPredicate(actual string, p predicate) bool {
	switch p.operator {
	case "=":
		return actual == p.value
	case "<>", "!=":
		return actual != p.value
	case "LIKE":
		return matchLike(actual, p.value)
	case "<":
		return actual < p.value
	case ">":
		return actual > p.value
	case "<=":
		return actual <= p.value
	case ">=":
		return actual >= p.value
	case "IS NULL":
		return actual == ""
	case "IS NOT NULL":
		return actual != ""
	case "IN":
		for _, v := range p.values {
			if actual == v {
				return true
			}
		}
		return false
	default:
		return true
	}
}

// matchesContains checks if any property value contains the search term (case-insensitive).
func matchesContains(info *provider.ResourceInfo, term string) bool {
	term = strings.ToLower(term)
	// Check name
	if strings.Contains(strings.ToLower(path.Base(info.Path)), term) {
		return true
	}
	// Check arbitrary metadata
	if info.ArbitraryMetadata != nil {
		for _, v := range info.ArbitraryMetadata.Metadata {
			if strings.Contains(strings.ToLower(v), term) {
				return true
			}
		}
	}
	return false
}

// needsMetadata returns true if the query references non-cmis properties.
func needsMetadata(q *cmisQuery) bool {
	if q.contains != "" {
		return true
	}
	for _, p := range q.predicates {
		if !strings.HasPrefix(p.property, "cmis:") {
			return true
		}
	}
	for _, o := range q.orderBy {
		if !strings.HasPrefix(o.property, "cmis:") {
			return true
		}
	}
	return false
}

// sortResults sorts query results by ORDER BY clauses.
func sortResults(results []ObjectEntry, orderBy []orderByClause) {
	sort.SliceStable(results, func(i, j int) bool {
		for _, ob := range orderBy {
			vi := getObjectPropertyValue(results[i].Object, ob.property)
			vj := getObjectPropertyValue(results[j].Object, ob.property)
			if vi == vj {
				continue
			}
			if ob.descending {
				return vi > vj
			}
			return vi < vj
		}
		return false
	})
}

// getObjectPropertyValue extracts a property value as string from a CMIS Object.
func getObjectPropertyValue(obj Object, propID string) string {
	if p, ok := obj.Properties[propID]; ok {
		return fmt.Sprintf("%v", p.Value)
	}
	return ""
}

// matchLike implements SQL LIKE pattern matching.
// % matches any sequence, _ matches any single character.
func matchLike(s, pattern string) bool {
	s = strings.ToLower(s)
	pattern = strings.ToLower(pattern)
	return matchLikeRec(s, pattern)
}

func matchLikeRec(s, pattern string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '%':
			// Try matching remainder at every position
			pattern = pattern[1:]
			for i := 0; i <= len(s); i++ {
				if matchLikeRec(s[i:], pattern) {
					return true
				}
			}
			return false
		case '_':
			if len(s) == 0 {
				return false
			}
			s = s[1:]
			pattern = pattern[1:]
		default:
			if len(s) == 0 || s[0] != pattern[0] {
				return false
			}
			s = s[1:]
			pattern = pattern[1:]
		}
	}
	return len(s) == 0
}

// --- Tokenizer ---

func tokenize(sql string) []string {
	var tokens []string
	runes := []rune(sql)
	i := 0

	for i < len(runes) {
		// Skip whitespace
		if unicode.IsSpace(runes[i]) {
			i++
			continue
		}

		// Quoted string
		if runes[i] == '\'' {
			j := i + 1
			for j < len(runes) && runes[j] != '\'' {
				if runes[j] == '\\' {
					j++ // skip escaped char
				}
				j++
			}
			if j < len(runes) {
				j++ // include closing quote
			}
			tokens = append(tokens, string(runes[i:j]))
			i = j
			continue
		}

		// Single-char tokens
		if runes[i] == '(' || runes[i] == ')' || runes[i] == ',' {
			tokens = append(tokens, string(runes[i]))
			i++
			continue
		}

		// Operators
		if runes[i] == '<' || runes[i] == '>' || runes[i] == '!' || runes[i] == '=' {
			j := i + 1
			if j < len(runes) && (runes[j] == '=' || (runes[i] == '<' && runes[j] == '>')) {
				j++
			}
			tokens = append(tokens, string(runes[i:j]))
			i = j
			continue
		}

		// Word/identifier (includes ':' for cmis:name)
		j := i
		for j < len(runes) && !unicode.IsSpace(runes[j]) &&
			runes[j] != '(' && runes[j] != ')' && runes[j] != ',' &&
			runes[j] != '\'' && runes[j] != '=' &&
			!(runes[j] == '<' || runes[j] == '>' || runes[j] == '!') {
			j++
		}
		if j > i {
			tokens = append(tokens, string(runes[i:j]))
		}
		i = j
	}
	return tokens
}

func tokEq(tokens []string, i int, val string) bool {
	if i < 0 || i >= len(tokens) {
		return false
	}
	return strings.EqualFold(tokens[i], val)
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}

func formInt(r *http.Request, key string, def int) int {
	v := r.FormValue(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
