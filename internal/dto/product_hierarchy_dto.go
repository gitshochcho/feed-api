package dto

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// =============================================================================
// INPUT
// =============================================================================

// FlexString does "STEP 1 — NORMALIZE" while the JSON is being decoded.
// It copies PHP's `is_scalar($v) ? trim((string) $v) : ”`:
//
//	"  DELI "     -> "DELI"   (strings are trimmed)
//	300           -> "300"    (numbers become text, so 300 and "300" are the same)
//	3.0 / 1e3     -> "3" / "1000"
//	true          -> "1"
//	false / null  -> ""
//	[...] / {...} -> ""       (arrays/objects are not scalar)
//
// A field that is missing from the JSON is never decoded, so it stays "".
type FlexString string

var jsonIntLiteral = regexp.MustCompile(`^-?\d+$`)

func (f *FlexString) UnmarshalJSON(data []byte) error {
	raw := bytes.TrimSpace(data)
	if len(raw) == 0 {
		*f = ""
		return nil
	}

	switch raw[0] {
	case '"': // string
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		*f = FlexString(strings.TrimSpace(s))
	case 't': // true
		*f = "1"
	case 'f', 'n', '[', '{': // false, null, array, object
		*f = ""
	default: // number
		text := string(raw)
		if !jsonIntLiteral.MatchString(text) {
			// Float: write it the short way PHP does (3.0 -> "3", 1e3 -> "1000").
			n, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return err
			}
			text = strconv.FormatFloat(n, 'f', -1, 64)
		}
		*f = FlexString(text)
	}
	return nil
}

// ProductHierarchyRequest is one feed row (one line of the old text file).
// The binding tags are "STEP 2 — VALIDATE": only level 3 is required.
// max=255 counts characters (runes), the same as Laravel's max:255 for strings.
type ProductHierarchyRequest struct {
	Level1Key  FlexString `json:"level1Key" binding:"max=255"`
	Level1Name FlexString `json:"level1Name" binding:"max=255"`
	Level2Key  FlexString `json:"level2Key" binding:"max=255"`
	Level2Name FlexString `json:"level2Name" binding:"max=255"`
	Level3Key  FlexString `json:"level3Key" binding:"required,max=255"`
	Level3Name FlexString `json:"level3Name" binding:"required,max=255"`
	Level4Key  FlexString `json:"level4Key" binding:"max=255"`
	Level4Name FlexString `json:"level4Name" binding:"max=255"`
	Level5Key  FlexString `json:"level5Key" binding:"max=255"`
	Level5Name FlexString `json:"level5Name" binding:"max=255"`
	Level6Key  FlexString `json:"level6Key" binding:"max=255"`
}

// ProductHierarchyRow is the clean row after normalize/validate (and after the shift, if any).
// It is sent back as "row" in the response, so the field order matches the PHP output.
type ProductHierarchyRow struct {
	Level1Key  string `json:"level1Key"`
	Level1Name string `json:"level1Name"`
	Level2Key  string `json:"level2Key"`
	Level2Name string `json:"level2Name"`
	Level3Key  string `json:"level3Key"`
	Level3Name string `json:"level3Name"`
	Level4Key  string `json:"level4Key"`
	Level4Name string `json:"level4Name"`
	Level5Key  string `json:"level5Key"`
	Level5Name string `json:"level5Name"`
	Level6Key  string `json:"level6Key"`
}

// ToRow converts the decoded request into a plain-string row.
func (r ProductHierarchyRequest) ToRow() ProductHierarchyRow {
	return ProductHierarchyRow{
		Level1Key: string(r.Level1Key), Level1Name: string(r.Level1Name),
		Level2Key: string(r.Level2Key), Level2Name: string(r.Level2Name),
		Level3Key: string(r.Level3Key), Level3Name: string(r.Level3Name),
		Level4Key: string(r.Level4Key), Level4Name: string(r.Level4Name),
		Level5Key: string(r.Level5Key), Level5Name: string(r.Level5Name),
		Level6Key: string(r.Level6Key),
	}
}

// Values returns all 11 values in field order (used by the junk check).
func (r ProductHierarchyRow) Values() []string {
	return []string{
		r.Level1Key, r.Level1Name, r.Level2Key, r.Level2Name, r.Level3Key, r.Level3Name,
		r.Level4Key, r.Level4Name, r.Level5Key, r.Level5Name, r.Level6Key,
	}
}

// Level returns the key and name of level 1..5 (level 6 has no name, so it is never saved).
func (r ProductHierarchyRow) Level(n int) (key, name string) {
	switch n {
	case 1:
		return r.Level1Key, r.Level1Name
	case 2:
		return r.Level2Key, r.Level2Name
	case 3:
		return r.Level3Key, r.Level3Name
	case 4:
		return r.Level4Key, r.Level4Name
	case 5:
		return r.Level5Key, r.Level5Name
	}
	return "", ""
}

// =============================================================================
// OUTPUT
// =============================================================================

// ProductHierarchyReport is what the DB transaction did (steps 5..8).
type ProductHierarchyReport struct {
	Departments     []DepartmentReport `json:"departments"`
	Parents         []ParentReport     `json:"parents"`
	Category        *CategoryReport    `json:"category"`
	StoreLinksAdded int64              `json:"storeLinksAdded"`
}

// ProductHierarchyResult is the full success response.
// The embedded report's fields appear at the top level, like PHP's `[...] + $result`.
type ProductHierarchyResult struct {
	Status  string              `json:"status"`
	Shifted bool                `json:"shifted"`
	Row     ProductHierarchyRow `json:"row"`
	ProductHierarchyReport
}

// DepartmentReport is one department touched in STEP 5.
// Action is "inserted", "updated" or "unchanged". Before is set only when updated.
type DepartmentReport struct {
	ID         uint              `json:"id"`
	ExternalID string            `json:"external_id"`
	Name       string            `json:"name"`
	Level      int               `json:"level"`
	Action     string            `json:"action"`
	Before     *DepartmentBefore `json:"before"`
}

type DepartmentBefore struct {
	Name  string `json:"name"`
	Level *int   `json:"level"`
}

// ParentReport is one parent_id change made in STEP 6.
type ParentReport struct {
	DepartmentID uint  `json:"department_id"`
	From         *uint `json:"from"`
	To           *uint `json:"to"`
}

// CategoryReport is the result of STEP 7. Changes is only present when action = "updated".
type CategoryReport struct {
	ID      uint                   `json:"id"`
	Action  string                 `json:"action"`
	Changes map[string]FieldChange `json:"changes,omitempty"`
}

type FieldChange struct {
	From any `json:"from"`
	To   any `json:"to"`
}
