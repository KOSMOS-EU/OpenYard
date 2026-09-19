package metadata

// defaultKnownKeys returns the hardcoded mapping for legacy DMS system fields.
// These are WinYard-internal field names that get translated to English oy.* keys.
// They are NOT client-facing IFR fields — those go through the namespace mapping.
func defaultKnownKeys() map[string]string {
	return map[string]string{
		// Document metadata
		"Betreff":                      "oy.subject",
		"Beschreibung":                 "oy.description",
		"Kategorie":                    "oy.category",
		"Aktz":                         "oy.fileReference",
		"Status":                       "oy.status",
		"Version":                      "oy.version",
		"LatestVersion":                "oy.latestVersion",

		// Timestamps
		"Created":                      "oy.created",
		"LastUpdated":                  "oy.lastUpdated",
		"FileCreated":                  "oy.fileCreated",
		"Date_Rom":                     "oy.dateRom",

		// People
		"Creator":                      "oy.creatorId",
		"CreatorFullName":              "oy.creatorName",
		"LastUpdated_User":             "oy.lastUpdatedById",
		"LastUpdated_User_Fullname":    "oy.lastUpdatedByName",

		// File info
		"DocName":                      "oy.docName",
		"FileExtension":                "oy.fileExtension",
		"FileSize":                     "oy.fileSize",

		// Paths & references
		"FullpathString":               "oy.fullPath",
		"ParentFolderId":               "oy.parentFolderId",
		"DocId":                        "oy.docId",

		// Document type
		"DocTypeName":                  "oy.type.name",
		"DocTypeId":                    "oy.type.id",

		// Migration
		"OldDocId":                     "oy.oldDocId",
		"OldFolderId":                  "oy.oldFolderId",

		// Checkout
		"Checked_Out":                  "oy.checkedOut",
		"Checked_Out_User_Id":          "oy.checkedOutById",
		"Checked_Out_User_FullName":    "oy.checkedOutByName",
		"Checked_Out_Date":             "oy.checkedOutDate",

		// Deletion
		"DelStatus":                    "oy.deleteStatus",
		"DelUserId":                    "oy.deletedById",
		"DelDateUser":                  "oy.deletedDate",

		// Notes
		"HasNotes":                     "oy.hasNotes",
		"Notice":                       "note",

		// Misc
		"BelongsTo":                    "oy.belongsTo",
		"PrimaryIndexValue":            "oy.primaryIndex",
	}
}

// defaultInfoKeys returns the hardcoded mapping for user-defined DocIndex fields.
func defaultInfoKeys() map[string]string {
	return map[string]string{
		"Ident":                        "info.ident",
		"Aktenzeichen":                 "info.fileReference",
		"Betreff":                      "info.subject",
		"Flurstücksdruckident":         "info.parcelIdent",
		"Bildungsvorschrift":           "info.formationRule",
		"Objektart":                    "info.objectType",
		"Register":                     "info.register",
		"Suchbegriff 1":                "info.searchTerm1",
		"Suchbegriff 2":                "info.searchTerm2",
		"Vertragsbeginn":               "info.contractStart",
		"Vertragsende":                 "info.contractEnd",
		"Absender":                     "info.sender",
		"Absender-E-Mailadresse":       "info.senderEmail",
		"Eingangs-/Versanddatum":       "info.sentDate",
		"Empfänger":                    "info.recipient",
		"Signatur":                     "info.signature",
		"Friedhofsbezeichnung":         "info.cemeteryName",
		"Grabfeldbezeichnung":          "info.graveSectionName",
		"Grabfeldnummer":               "info.graveSectionNumber",
		"Grabreihenbezeichnung":        "info.graveRowName",
		"Grabreihennummer":             "info.graveRowNumber",
		"Objektbezeichnung":            "info.objectName",
		"Objektnummer":                 "info.objectNumber",
		"Kassenzeichen":                "info.cashReference",
		"Bezeichnung":                  "info.designation",
		"Betrag in EUR":                "info.amountEur",
		"Bemerkung":                    "info.remark",
		"Baumname":                     "info.treeName",
		"KZ_DKS":                       "info.dksCode",
	}
}
