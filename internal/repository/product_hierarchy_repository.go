package repository

import (
	"context"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"feed-api/internal/dto"
	"feed-api/internal/model"
)

type ProductHierarchyRepository struct {
	db *gorm.DB
}

func NewProductHierarchyRepository(db *gorm.DB) *ProductHierarchyRepository {
	return &ProductHierarchyRepository{db: db}
}

// forUpdate adds "FOR UPDATE" to a SELECT (Laravel's lockForUpdate()).
// The row stays locked until the transaction ends, so two requests can't edit it at once.
var forUpdate = clause.Locking{Strength: "UPDATE"}

// Save runs STEP 5..8 for one (already validated and shifted) row inside ONE transaction.
// If any step returns an error, GORM rolls everything back, so nothing is saved.
func (r *ProductHierarchyRepository) Save(ctx context.Context, row dto.ProductHierarchyRow) (*dto.ProductHierarchyReport, error) {
	report := &dto.ProductHierarchyReport{
		Departments: []dto.DepartmentReport{}, // empty slices print as [] instead of null
		Parents:     []dto.ParentReport{},
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		deptIDs := map[int]uint{} // level number => departments.id

		if err := saveDepartments(tx, row, deptIDs, report); err != nil {
			return err
		}
		if err := saveParents(tx, deptIDs, report); err != nil {
			return err
		}
		if err := saveCategory(tx, row, deptIDs, report); err != nil {
			return err
		}
		return saveStoreLinks(tx, deptIDs, report)
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

// -----------------------------------------------------------------------------
// STEP 5 — DEPARTMENTS (one per level 1..5)
// Skip a level if its key OR name is empty.
// Find by external_id (unique):
//
//	not found          => INSERT
//	name/level differs => UPDATE name, level
//	same               => nothing
//
// Level 6 is never saved (it has no name).
// -----------------------------------------------------------------------------
func saveDepartments(tx *gorm.DB, row dto.ProductHierarchyRow, deptIDs map[int]uint, report *dto.ProductHierarchyReport) error {
	for level := 1; level <= 5; level++ {
		key, name := row.Level(level)
		if key == "" || name == "" {
			continue
		}

		entry := dto.DepartmentReport{ExternalID: key, Name: name, Level: level}

		// Find (not Take) so a missing row is not logged as an error; RowsAffected = 0 means "not found".
		var existing model.Department
		res := tx.Clauses(forUpdate).Where("external_id = ?", key).Limit(1).Find(&existing)

		switch {
		case res.Error != nil:
			return res.Error

		case res.RowsAffected == 0:
			// Not found => INSERT (only these 4 columns, the same as the Laravel insert).
			lvl := level
			dept := model.Department{ExternalID: key, Name: name, Level: &lvl, CreatedAt: time.Now()}
			if err := tx.Select("ExternalID", "Name", "Level", "CreatedAt").Create(&dept).Error; err != nil {
				return err
			}
			entry.ID, entry.Action = dept.ID, "inserted"

		case existing.Name != name || existing.Level == nil || *existing.Level != level:
			// Found but name or level differs => UPDATE, and remember the old values.
			if err := tx.Model(&model.Department{}).Where("id = ?", existing.ID).
				Updates(map[string]any{"name": name, "level": level}).Error; err != nil {
				return err
			}
			entry.ID, entry.Action = existing.ID, "updated"
			entry.Before = &dto.DepartmentBefore{Name: existing.Name, Level: existing.Level}

		default:
			// Found and identical => nothing to do.
			entry.ID, entry.Action = existing.ID, "unchanged"
		}

		deptIDs[level] = entry.ID
		report.Departments = append(report.Departments, entry)
	}
	return nil
}

// -----------------------------------------------------------------------------
// STEP 6 — PARENT LINKS (departments.parent_id)
//
//	level 1          => parent = NULL
//	level N (2..5)   => parent = department of level N-1 in THIS row
//	level N-1 empty  => keep the existing parent (do nothing)
//
// -----------------------------------------------------------------------------
func saveParents(tx *gorm.DB, deptIDs map[int]uint, report *dto.ProductHierarchyReport) error {
	for level := 1; level <= 5; level++ {
		id, ok := deptIDs[level]
		if !ok {
			continue // this level was not saved in STEP 5
		}

		var newParent *uint
		if level > 1 {
			prevID, ok := deptIDs[level-1]
			if !ok {
				continue // previous level not in this row => leave parent as it is
			}
			newParent = &prevID
		}

		if newParent != nil && *newParent == id {
			continue // never make a department its own parent
		}

		// Read the current parent_id fresh (NULL scans into a nil pointer).
		var oldParent *uint
		if err := tx.Model(&model.Department{}).Select("parent_id").Where("id = ?", id).
			Row().Scan(&oldParent); err != nil {
			return err
		}

		if !sameID(oldParent, newParent) {
			if err := tx.Model(&model.Department{}).Where("id = ?", id).
				Update("parent_id", newParent).Error; err != nil {
				return err
			}
			report.Parents = append(report.Parents, dto.ParentReport{DepartmentID: id, From: oldParent, To: newParent})
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// STEP 7 — CATEGORY
// Identity = all 5 keys together (unique in DB).
//
//	not found      => INSERT keys + names + department_id
//	found, differs => UPDATE only the changed names / department_id
//	found, same    => nothing
//
// department_id is ALWAYS the level-3 department.
// A changed KEY means a NEW category; the old one is never deleted.
// -----------------------------------------------------------------------------
func saveCategory(tx *gorm.DB, row dto.ProductHierarchyRow, deptIDs map[int]uint, report *dto.ProductHierarchyReport) error {
	var departmentID *uint
	if id, ok := deptIDs[3]; ok {
		departmentID = &id
	}

	keys := map[string]any{
		"level_1_key": row.Level1Key,
		"level_2_key": row.Level2Key,
		"level_3_key": row.Level3Key,
		"level_4_key": row.Level4Key,
		"level_5_key": row.Level5Key,
	}

	var category model.Category
	res := tx.Clauses(forUpdate).Where(keys).Limit(1).Find(&category)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		// Not found => INSERT a new category.
		newCategory := model.Category{
			Level1Key: row.Level1Key, Level1Name: &row.Level1Name,
			Level2Key: row.Level2Key, Level2Name: &row.Level2Name,
			Level3Key: row.Level3Key, Level3Name: &row.Level3Name,
			Level4Key: row.Level4Key, Level4Name: &row.Level4Name,
			Level5Key: row.Level5Key, Level5Name: &row.Level5Name,
			DepartmentID: departmentID,
			CreatedAt:    time.Now(),
		}
		if err := tx.Create(&newCategory).Error; err != nil {
			return err
		}
		report.Category = &dto.CategoryReport{ID: newCategory.ID, Action: "inserted"}
		return nil
	}

	// Found => compare every value as text, the same as PHP's (string) cast (NULL => "").
	compare := []struct {
		column   string
		old, new any
	}{
		{"level_1_name", strPtrValue(category.Level1Name), row.Level1Name},
		{"level_2_name", strPtrValue(category.Level2Name), row.Level2Name},
		{"level_3_name", strPtrValue(category.Level3Name), row.Level3Name},
		{"level_4_name", strPtrValue(category.Level4Name), row.Level4Name},
		{"level_5_name", strPtrValue(category.Level5Name), row.Level5Name},
		{"department_id", uintPtrValue(category.DepartmentID), uintPtrValue(departmentID)},
	}

	changes := map[string]dto.FieldChange{}
	updates := map[string]any{}
	for _, c := range compare {
		if asText(c.old) != asText(c.new) {
			changes[c.column] = dto.FieldChange{From: c.old, To: c.new}
			updates[c.column] = c.new
		}
	}

	if len(updates) == 0 {
		report.Category = &dto.CategoryReport{ID: category.ID, Action: "unchanged"}
		return nil
	}

	// Found and different => UPDATE only the changed columns.
	if err := tx.Table("categories").Where("id = ?", category.ID).Updates(updates).Error; err != nil {
		return err
	}
	report.Category = &dto.CategoryReport{ID: category.ID, Action: "updated", Changes: changes}
	return nil
}

// -----------------------------------------------------------------------------
// STEP 8 — STORE LINKS (store_departments)
// Every department of this row is linked to every store.
// Existing links are untouched (ON CONFLICT DO NOTHING).
// sync_up_mode (false) and status ('A') come from DB defaults.
// -----------------------------------------------------------------------------
func saveStoreLinks(tx *gorm.DB, deptIDs map[int]uint, report *dto.ProductHierarchyReport) error {
	ids := make([]uint, 0, len(deptIDs))
	for level := 1; level <= 5; level++ {
		if id, ok := deptIDs[level]; ok {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	// GORM expands "IN ?" with a slice into (id1, id2, ...).
	result := tx.Exec(`
		INSERT INTO store_departments (store_id, department_id, created_at)
		SELECT s.id, d.id, NOW()
		FROM stores s
		CROSS JOIN departments d
		WHERE d.id IN ?
		ON CONFLICT (store_id, department_id) DO NOTHING`, ids)
	if result.Error != nil {
		return result.Error
	}
	report.StoreLinksAdded = result.RowsAffected
	return nil
}

// --- small helpers -----------------------------------------------------------

// sameID compares two nullable ids (nil == nil, nil != 5, 5 == 5).
func sameID(a, b *uint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// strPtrValue returns nil for NULL, otherwise the string (so the report shows null or "text").
func strPtrValue(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// uintPtrValue returns nil for NULL, otherwise the number.
func uintPtrValue(p *uint) any {
	if p == nil {
		return nil
	}
	return *p
}

// asText is PHP's (string) cast for the values we compare: nil => "", 27 => "27".
func asText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	}
	return ""
}
