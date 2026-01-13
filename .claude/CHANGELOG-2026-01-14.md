# Changelog - January 14, 2026

## Summary

- **CRITICAL FIX**: Enforced non-clickable image validation in verification prompts
- All images (screenshots, diagrams, architecture images) must now be clickable
- Added new validation check: Non-Clickable Image Detection (ERROR severity)

---

## Key Changes

### 1. Non-Clickable Image Validation Enforcement

**Problem**: Verification prompts were not checking for missing `link=self,window=blank` on regular images/screenshots. While the prompts checked for external link caret enforcement, they incorrectly allowed non-clickable images with this example:
```
- Regular image: `image::screenshot.png[Screenshot]` ✅ (no link, no caret needed)
```

This was **WRONG** - all images should be clickable per team requirements from Nate Stephany.

**Solution**: Added comprehensive validation for ALL images requiring clickable links.

**Files Modified**:
- `.claude/prompts/enhanced_verification_workshop.txt`
- `.claude/prompts/enhanced_verification_demo.txt`

**Changes Made**:

1. **Updated Image Link Format Requirements** (lines 419-435):
   ```
   **CRITICAL: Clickable Image Link Format Requirements:**
   - ✅ **ALL images MUST be clickable** (screenshots use `link=self,window=blank` - NO caret)
   - ✅ **External image links MUST use `^` caret** to open in new tab
   - ✅ **Format for Screenshots/Self-links**: `image::screenshot.png[Alt Text,link=self,window=blank]` (NO caret)
   - ✅ **Format for External links**: `image::diagram.png[Alt Text,link=https://example.com^]` (YES caret)
   - ❌ **INVALID**: `image::screenshot.png[Alt Text,link=self^]` (DO NOT use caret with link=self)
   - ❌ **INVALID**: `image::diagram.png[Alt Text,link=https://example.com]` (external link missing caret)
   - ❌ **INVALID**: `image::screenshot.png[Screenshot]` (not clickable - missing link=self,window=blank)
   ```

2. **Added New Validation Check 3**: Non-Clickable Image Detection
   ```
   ### Check 3: Non-Clickable Image Detection (ALL IMAGES MUST BE CLICKABLE)
   **What to scan**: ALL `image::` macros in content files
   **Pattern to detect violations**:
   - `image::[^[]+\[[^\]]*\]` that does NOT contain `link=` attribute
   - Specifically: images without `link=self,window=blank` or `link=https://...^`
   **Error condition**: Image without any `link=` attribute
   **Severity**: ERROR
   **Why this matters**: Non-clickable images frustrate learners trying to see full-size screenshots and details. Based on team feedback from Nate Stephany, ALL images must open full-size in new tab.
   **Example violation**: `image::console-screenshot.png[OpenShift Console]`
   **Required fix**: `image::console-screenshot.png[OpenShift Console,link=self,window=blank]`
   ```

3. **Updated JSON Structure**: Added `non_clickable_images` check
   ```json
   "non_clickable_images": {
     "violations_found": 0,
     "severity": "error",
     "files_affected": []
   }
   ```

4. **Updated Scanning Order**: Added Check 3, renumbered subsequent checks 4-7
   - Check 1: External Link Caret Enforcement
   - Check 2: Clickable Image Link Caret Enforcement
   - **Check 3: Non-Clickable Images Check** (NEW)
   - Check 4: List Marker Semantic Validation
   - Check 5: List Blank Line Check
   - Check 6: Formatting Consistency Check
   - Check 7: Build Summary Table

5. **Updated Count Consistency Requirements**:
   ```
   - `validation_summary.checks_performed.non_clickable_images.violations_found` = count of non-clickable image errors in `issues`
   ```

**Critical Clarifications**:
- `link=self,window=blank` does **NOT** use caret `^`
- Only external URLs use caret: `link=https://example.com^`
- Screenshots/diagrams use: `link=self,window=blank` (opens full-size in new tab)

**Impact**:
- Verification agents will now flag non-clickable images as **CRITICAL errors**
- All existing content must be updated to make images clickable
- Applies to workshops and demos

**Commit**: `49db3d4` - Add non-clickable image validation to verification prompts

---

## Related Documentation

See also:
- `.claude/skills/create-lab/SKILL.md:798-850` - Clickable image requirements for workshops
- `.claude/skills/create-demo/SKILL.md:651-697` - Clickable image requirements for demos
- `.claude/docs/SKILL-COMMON-RULES.md:208-233` - Common image formatting rules
- `.claude/CHANGELOG-2026-01-12.md` - Previous clickable image requirements
