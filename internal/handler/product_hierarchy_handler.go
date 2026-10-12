package handler

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/repository"
)

// shiftKeys: if the incoming level3Key is one of these, the row is shifted up one level.
// Same list as the old file transform (product_hierarchies.json) and the products feed.
var shiftKeys = map[int]bool{
	100: true, 200: true, 350: true, 400: true, 450: true, 500: true, 550: true, 600: true,
	650: true, 700: true, 740: true, 800: true, 820: true, 840: true, 860: true, 875: true,
}

// junkWords: rows containing any of these strings are ignored (old "filterLines" rule).
var junkWords = []string{"OLD-OLD", "NULL"}

// phpNumeric matches what PHP's is_numeric() accepts for a trimmed string ("500", "500.0", "5e2", ...).
var phpNumeric = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)

type ProductHierarchyHandler struct {
	repo *repository.ProductHierarchyRepository
}

func NewProductHierarchyHandler(repo *repository.ProductHierarchyRepository) *ProductHierarchyHandler {
	return &ProductHierarchyHandler{repo: repo}
}

func (h *ProductHierarchyHandler) Store(c *gin.Context) {
	var req dto.ProductHierarchyRequest

	// 1. NORMALIZE + VALIDATE
	// dto.FlexString trims every value and turns numbers into text while decoding;
	// missing / null fields become "". Then the binding tags check: level 3 required, max 255.
	if err := c.ShouldBindJSON(&req); err != nil {
		var verrs validator.ValidationErrors
		if errors.As(err, &verrs) {
			errs := validationMessages(verrs)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": firstMessage(verrs, errs), "errors": errs})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON body"})
		return
	}
	row := req.ToRow()

	// 2. JUNK CHECK
	// If any value contains OLD-OLD or NULL, nothing is saved.
	for _, value := range row.Values() {
		for _, word := range junkWords {
			if strings.Contains(value, word) {
				c.JSON(http.StatusOK, gin.H{
					"status": "skipped",
					"reason": fmt.Sprintf("Row contains '%s', nothing saved", word),
				})
				return
			}
		}
	}

	// 3. SHIFT RULE
	// level3Key in shiftKeys => L3→L2, L4→L3, L5→L4, L5="", L6="".
	// The original level 2 is dropped. Level 1 stays.
	shifted := false
	if isShiftKey(row.Level3Key) {
		row = dto.ProductHierarchyRow{
			Level1Key: row.Level1Key, Level1Name: row.Level1Name,
			Level2Key: row.Level3Key, Level2Name: row.Level3Name,
			Level3Key: row.Level4Key, Level3Name: row.Level4Name,
			Level4Key: row.Level5Key, Level4Name: row.Level5Name,
			Level5Key: "", Level5Name: "",
			Level6Key: "",
		}
		shifted = true

		// After shifting, the new level 3 (old level 4) must exist.
		if row.Level3Key == "" || row.Level3Name == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "level4Key and level4Name are required when level3Key is one of the shift keys.",
			})
			return
		}
	}

	// 4. SAVE EVERYTHING IN ONE TRANSACTION (departments, parents, category, store links)
	report, err := h.repo.Save(c.Request.Context(), row)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"error": "Conflicting row was saved at the same time. Please retry."})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save product hierarchy"})
		return
	}

	// 5. Return what happened
	c.JSON(http.StatusOK, dto.ProductHierarchyResult{
		Status:                 "processed",
		Shifted:                shifted,
		Row:                    row,
		ProductHierarchyReport: *report,
	})
}

// isShiftKey copies PHP: is_numeric($key) && in_array((int) $key, SHIFT_KEYS, true).
// (int) "500.9" is 500 in PHP, so the number is truncated toward zero.
func isShiftKey(key string) bool {
	if !phpNumeric.MatchString(key) {
		return false
	}
	n, err := strconv.ParseFloat(key, 64)
	if err != nil {
		return false
	}
	return shiftKeys[int(math.Trunc(n))]
}

// validationMessages builds Laravel-style errors: {"level3Key": ["The level3 key field is required."]}.
func validationMessages(verrs validator.ValidationErrors) map[string][]string {
	errs := map[string][]string{}
	for _, fe := range verrs {
		jsonName := strings.ToLower(fe.Field()[:1]) + fe.Field()[1:] // Level3Key -> level3Key
		label := fieldLabel(fe.Field())                              // Level3Key -> level3 key

		msg := fmt.Sprintf("The %s field is invalid.", label)
		switch fe.Tag() {
		case "required":
			msg = fmt.Sprintf("The %s field is required.", label)
		case "max":
			msg = fmt.Sprintf("The %s field must not be greater than %s characters.", label, fe.Param())
		}
		errs[jsonName] = append(errs[jsonName], msg)
	}
	return errs
}

// firstMessage returns the first error message (in field order) for the "error" key.
func firstMessage(verrs validator.ValidationErrors, errs map[string][]string) string {
	first := verrs[0].Field()
	return errs[strings.ToLower(first[:1])+first[1:]][0]
}

// fieldLabel turns "Level3Key" into "level3 key", the same as Laravel's attribute names.
func fieldLabel(field string) string {
	var b strings.Builder
	for i, r := range field {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteRune(' ')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}
